package channelhost

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"agent-vivy/internal/channelhost/fake"
	"agent-vivy/internal/config"
	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/sqlite"
	plugin "agent-vivy/sdk/port/channel"
)

// publishHello publishes one allowed hello from alice through the host env,
// mirroring what a live adapter does.
func publishHello(t *testing.T, host *Host, ch plugin.Channel, messageID string) {
	t.Helper()
	env := host.envFor(ch)
	if err := env.PublishInbound(context.Background(), plugin.InboundMessage{
		Channel: "fake", ChatID: "chat-1", Sender: "alice", MessageID: messageID,
		Parts: []plugin.Part{{Kind: plugin.PartText, Text: "hello"}},
	}); err != nil {
		t.Fatalf("publish inbound: %v", err)
	}
}

// seedReply appends the assistant row the delivery path reads back.
func seedReply(t *testing.T, backend *sqlite.Backend, sessionID domain.SessionID, runID domain.RunID) {
	t.Helper()
	if err := backend.AppendMessage(context.Background(), domain.Message{
		ID: "msg-" + string(runID), SessionID: sessionID, RunID: runID,
		Role: domain.RoleAssistant, CreatedAt: time.Now().UnixMilli(),
		Content: "channel reply",
	}); err != nil {
		t.Fatalf("seed assistant message: %v", err)
	}
}

// journalTerminal persists the run terminal the runtime would have written.
func journalTerminal(t *testing.T, backend *sqlite.Backend, runID domain.RunID, typ domain.EventType) {
	t.Helper()
	if _, err := backend.Append(context.Background(), storage.Commit{RunID: runID, Events: []domain.RunEvent{
		{Type: domain.EventRunStarted, CreatedAt: time.Now().UnixMilli(), PayloadVersion: 1, Payload: []byte(`{}`)},
		{Type: typ, CreatedAt: time.Now().UnixMilli(), PayloadVersion: 1, Payload: []byte(`{}`)},
	}}); err != nil {
		t.Fatalf("journal terminal: %v", err)
	}
}

// openIntent returns the open intent rows, failing the test on store errors.
func openIntent(t *testing.T, backend *sqlite.Backend) []storage.ChannelDelivery {
	t.Helper()
	open, err := backend.ListOpenChannelDeliveries(context.Background())
	if err != nil {
		t.Fatalf("list open intents: %v", err)
	}
	return open
}

// waitOpenState polls until the run's intent row leaves the open list or
// reaches the wanted state.
func waitOpenState(t *testing.T, backend *sqlite.Backend, runID domain.RunID, wantState string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		open := openIntent(t, backend)
		for _, d := range open {
			if d.RunID == runID && (wantState == "" || d.State == wantState) {
				return
			}
		}
		if wantState == "" && len(open) == 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("intent %s never reached state %q; open = %+v", runID, wantState, open)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// TestRestartRedeliversArmedIntentOfCompletedRun is the CH-C3-N1 defect
// shape: the run completed and the process died before the Send. A fresh
// host over the same store must reconcile the armed intent against the
// journal's terminal and deliver the reply.
func TestRestartRedeliversArmedIntentOfCompletedRun(t *testing.T) {
	backend := openBackend(t)
	runs := &runRecorder{messages: backend}
	chA := fake.New()
	chA.Publish = func(context.Context, plugin.ChannelEnv) error { return nil }
	hostA := New(Deps{
		Journal:    backend,
		Messages:   backend,
		Sessions:   backend,
		Deliveries: backend,
		Run:        runs.run,
		Channels:   []plugin.Channel{chA},
		Config:     config.Channels{"fake": {Enabled: true, AllowFrom: []string{"alice"}}},
		Logger:     testLogger(),
	})
	if err := hostA.StartAll(context.Background()); err != nil {
		t.Fatalf("start host a: %v", err)
	}
	publishHello(t, hostA, chA, "m-crash-1")
	call := runs.snapshot()[0]

	// "Crash": the terminal lands in the journal, no OnRunEvent fires.
	seedReply(t, backend, call.sessionID, call.runID)
	journalTerminal(t, backend, call.runID, domain.EventRunCompleted)

	chB := fake.New()
	chB.Publish = func(context.Context, plugin.ChannelEnv) error { return nil }
	runsB := &runRecorder{messages: backend}
	hostB := New(Deps{
		Journal:    backend,
		Messages:   backend,
		Sessions:   backend,
		Deliveries: backend,
		Run:        runsB.run,
		Channels:   []plugin.Channel{chB},
		Config:     config.Channels{"fake": {Enabled: true, AllowFrom: []string{"alice"}}},
		Logger:     testLogger(),
	})
	if err := hostB.StartAll(context.Background()); err != nil {
		t.Fatalf("start host b: %v", err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for len(chB.Snapshot()) == 0 {
		if time.Now().After(deadline) {
			t.Fatal("restart redelivery never reached the fake channel")
		}
		time.Sleep(5 * time.Millisecond)
	}
	sent := chB.Snapshot()
	if len(sent) != 1 || sent[0].Parts[0].Text != "channel reply" {
		t.Fatalf("redelivered envelopes = %+v, want the assistant reply", sent)
	}
	if calls := runsB.snapshot(); len(calls) != 0 {
		t.Fatalf("reconcile opened new runs: %+v", calls)
	}
	waitOpenState(t, backend, call.runID, "")
}

// TestRestartSettlesArmedIntentOfFailedRun: an armed intent whose run
// failed over the restart delivers nothing and leaves no open row.
func TestRestartSettlesArmedIntentOfFailedRun(t *testing.T) {
	backend := openBackend(t)
	runs := &runRecorder{messages: backend}
	chA := fake.New()
	chA.Publish = func(context.Context, plugin.ChannelEnv) error { return nil }
	hostA := New(Deps{
		Journal:    backend,
		Messages:   backend,
		Sessions:   backend,
		Deliveries: backend,
		Run:        runs.run,
		Channels:   []plugin.Channel{chA},
		Config:     config.Channels{"fake": {Enabled: true, AllowFrom: []string{"alice"}}},
		Logger:     testLogger(),
	})
	if err := hostA.StartAll(context.Background()); err != nil {
		t.Fatalf("start host a: %v", err)
	}
	publishHello(t, hostA, chA, "m-crash-2")
	call := runs.snapshot()[0]
	journalTerminal(t, backend, call.runID, domain.EventRunFailed)

	chB := fake.New()
	chB.Publish = func(context.Context, plugin.ChannelEnv) error { return nil }
	hostB := New(Deps{
		Journal:    backend,
		Messages:   backend,
		Sessions:   backend,
		Deliveries: backend,
		Run:        (&runRecorder{messages: backend}).run,
		Channels:   []plugin.Channel{chB},
		Config:     config.Channels{"fake": {Enabled: true, AllowFrom: []string{"alice"}}},
		Logger:     testLogger(),
	})
	if err := hostB.StartAll(context.Background()); err != nil {
		t.Fatalf("start host b: %v", err)
	}
	time.Sleep(100 * time.Millisecond)
	if got := chB.Snapshot(); len(got) != 0 {
		t.Fatalf("failed run redelivered: %+v", got)
	}
	if open := openIntent(t, backend); len(open) != 0 {
		t.Fatalf("failed-run intent survived the restart: %+v", open)
	}
}

// TestRestartRedeliversPendingIntent: a pending intent (completed observed,
// Send unconfirmed) redelivers on the next start — at-least-once.
func TestRestartRedeliversPendingIntent(t *testing.T) {
	backend := openBackend(t)
	sessionID := ChannelSessionID("fake", "chat-1", "")
	runID := domain.RunID("run-ch-pending")
	now := time.Now().UnixMilli()
	if err := backend.UpsertChannelDelivery(context.Background(), storage.ChannelDelivery{
		RunID: runID, SessionID: sessionID, Channel: "fake", ChatID: "chat-1",
		State: storage.ChannelDeliveryPending, Attempts: 1, CreatedAtMs: now, UpdatedAtMs: now,
	}); err != nil {
		t.Fatal(err)
	}
	seedReply(t, backend, sessionID, runID)

	ch := fake.New()
	ch.Publish = func(context.Context, plugin.ChannelEnv) error { return nil }
	host := New(Deps{
		Journal:    backend,
		Messages:   backend,
		Sessions:   backend,
		Deliveries: backend,
		Run:        (&runRecorder{messages: backend}).run,
		Channels:   []plugin.Channel{ch},
		Config:     config.Channels{"fake": {Enabled: true, AllowFrom: []string{"alice"}}},
		Logger:     testLogger(),
	})
	if err := host.StartAll(context.Background()); err != nil {
		t.Fatalf("start host: %v", err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for len(ch.Snapshot()) == 0 {
		if time.Now().After(deadline) {
			t.Fatal("pending intent was never redelivered")
		}
		time.Sleep(5 * time.Millisecond)
	}
	waitOpenState(t, backend, runID, "")
}

// TestRestartParksExhaustedPendingIntentAsFailed: a pending row whose
// attempt budget a previous process already spent settles to failed
// instead of retrying on every boot.
func TestRestartParksExhaustedPendingIntentAsFailed(t *testing.T) {
	backend := openBackend(t)
	sessionID := ChannelSessionID("fake", "chat-1", "")
	runID := domain.RunID("run-ch-exhausted")
	now := time.Now().UnixMilli()
	if err := backend.UpsertChannelDelivery(context.Background(), storage.ChannelDelivery{
		RunID: runID, SessionID: sessionID, Channel: "fake", ChatID: "chat-1",
		State: storage.ChannelDeliveryPending, Attempts: outboundDeliveryAttempts,
		CreatedAtMs: now, UpdatedAtMs: now,
	}); err != nil {
		t.Fatal(err)
	}
	seedReply(t, backend, sessionID, runID)

	ch := fake.New()
	ch.Publish = func(context.Context, plugin.ChannelEnv) error { return nil }
	host := New(Deps{
		Journal:    backend,
		Messages:   backend,
		Sessions:   backend,
		Deliveries: backend,
		Run:        (&runRecorder{messages: backend}).run,
		Channels:   []plugin.Channel{ch},
		Config:     config.Channels{"fake": {Enabled: true, AllowFrom: []string{"alice"}}},
		Logger:     testLogger(),
	})
	if err := host.StartAll(context.Background()); err != nil {
		t.Fatalf("start host: %v", err)
	}
	time.Sleep(100 * time.Millisecond)
	if got := ch.Snapshot(); len(got) != 0 {
		t.Fatalf("exhausted intent redelivered: %+v", got)
	}
	open, err := backend.ListOpenChannelDeliveries(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(open) != 0 {
		t.Fatalf("exhausted intent still open: %+v", open)
	}
}

// gatedChannel blocks the first Send until its gate opens, so the test can
// hold a delivery in flight across StopAll.
type gatedChannel struct {
	*fake.Channel
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func newGatedChannel() *gatedChannel {
	ch := &gatedChannel{
		Channel: fake.New(),
		entered: make(chan struct{}),
		release: make(chan struct{}),
	}
	ch.Publish = func(context.Context, plugin.ChannelEnv) error { return nil }
	return ch
}

func (g *gatedChannel) Send(ctx context.Context, msg plugin.OutboundMessage) ([]string, error) {
	g.once.Do(func() { close(g.entered) })
	<-g.release
	return g.Channel.Send(ctx, msg)
}

// TestStopAllWaitsForInFlightDelivery: StopAll joins the in-flight Send
// instead of dropping it to process exit; only the channel deadline bounds
// the wait.
func TestStopAllWaitsForInFlightDelivery(t *testing.T) {
	backend := openBackend(t)
	runs := &runRecorder{messages: backend}
	ch := newGatedChannel()
	host := New(Deps{
		Journal:    backend,
		Messages:   backend,
		Sessions:   backend,
		Deliveries: backend,
		Run:        runs.run,
		Channels:   []plugin.Channel{ch},
		Config:     config.Channels{"fake": {Enabled: true, AllowFrom: []string{"alice"}}},
		Logger:     testLogger(),
	})
	if err := host.StartAll(context.Background()); err != nil {
		t.Fatalf("start all: %v", err)
	}
	publishHello(t, host, ch, "m-drain-1")
	call := runs.snapshot()[0]
	seedReply(t, backend, call.sessionID, call.runID)

	host.OnRunEvent(context.Background(), domain.RunEvent{
		RunID: call.runID, Type: domain.EventRunCompleted,
		CreatedAt: time.Now().UnixMilli(), PayloadVersion: 1,
	})
	select {
	case <-ch.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("delivery never reached the gated Send")
	}

	stopDone := make(chan struct{})
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		host.StopAll(ctx)
		close(stopDone)
	}()
	// While the Send is held, StopAll must still be waiting.
	select {
	case <-stopDone:
		t.Fatal("StopAll returned while a Send was still in flight")
	case <-time.After(200 * time.Millisecond):
	}
	close(ch.release)
	select {
	case <-stopDone:
	case <-time.After(2 * time.Second):
		t.Fatal("StopAll never returned after the Send finished")
	}
	if got := ch.Channel.Snapshot(); len(got) != 1 {
		t.Fatalf("sent envelopes after drain = %d, want 1", len(got))
	}
	waitOpenState(t, backend, call.runID, "")
}

// TestOnRunEventDuringDrainKeepsPendingIntent: a run completing while
// StopAll drains must not spawn a delivery goroutine; its durable intent
// stays pending for the next start instead of racing the shutdown.
func TestOnRunEventDuringDrainKeepsPendingIntent(t *testing.T) {
	backend := openBackend(t)
	runs := &runRecorder{messages: backend}
	ch := newGatedChannel()
	host := New(Deps{
		Journal:    backend,
		Messages:   backend,
		Sessions:   backend,
		Deliveries: backend,
		Run:        runs.run,
		Channels:   []plugin.Channel{ch},
		Config:     config.Channels{"fake": {Enabled: true, AllowFrom: []string{"alice"}}},
		Logger:     testLogger(),
	})
	if err := host.StartAll(context.Background()); err != nil {
		t.Fatalf("start all: %v", err)
	}
	publishHello(t, host, ch, "m-drain-2")
	call := runs.snapshot()[0]
	seedReply(t, backend, call.sessionID, call.runID)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	host.StopAll(ctx)

	host.OnRunEvent(context.Background(), domain.RunEvent{
		RunID: call.runID, Type: domain.EventRunCompleted,
		CreatedAt: time.Now().UnixMilli(), PayloadVersion: 1,
	})
	time.Sleep(100 * time.Millisecond)
	if got := ch.Channel.Snapshot(); len(got) != 0 {
		t.Fatalf("drain-era terminal delivered: %+v", got)
	}
	waitOpenState(t, backend, call.runID, storage.ChannelDeliveryPending)
}

// failingChannel records the Publish face but refuses every Send, counting
// the attempts so tests can observe the retry budget without scraping rows.
type failingChannel struct {
	*fake.Channel
	mu        sync.Mutex
	sentTries int
}

func (f *failingChannel) Send(context.Context, plugin.OutboundMessage) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sentTries++
	return nil, errors.New("platform unreachable")
}

func (f *failingChannel) tries() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.sentTries
}

// TestDeliveryAttemptsExhaustedMarksFailed: a channel whose Send always
// fails burns its attempt budget, parks the intent as failed, and stops
// retrying instead of looping forever.
func TestDeliveryAttemptsExhaustedMarksFailed(t *testing.T) {
	oldDelay := outboundDeliveryRetryDelay
	outboundDeliveryRetryDelay = time.Millisecond
	defer func() { outboundDeliveryRetryDelay = oldDelay }()

	backend := openBackend(t)
	runs := &runRecorder{messages: backend}
	ch := &failingChannel{Channel: fake.New()}
	ch.Channel.Publish = func(context.Context, plugin.ChannelEnv) error { return nil }
	host := New(Deps{
		Journal:    backend,
		Messages:   backend,
		Sessions:   backend,
		Deliveries: backend,
		Run:        runs.run,
		Channels:   []plugin.Channel{ch},
		Config:     config.Channels{"fake": {Enabled: true, AllowFrom: []string{"alice"}}},
		Logger:     testLogger(),
	})
	if err := host.StartAll(context.Background()); err != nil {
		t.Fatalf("start all: %v", err)
	}
	publishHello(t, host, ch, "m-retry-1")
	call := runs.snapshot()[0]
	seedReply(t, backend, call.sessionID, call.runID)

	host.OnRunEvent(context.Background(), domain.RunEvent{
		RunID: call.runID, Type: domain.EventRunCompleted,
		CreatedAt: time.Now().UnixMilli(), PayloadVersion: 1,
	})
	// The budget is initial attempt + retries, all against the dead channel.
	deadline := time.Now().Add(2 * time.Second)
	for ch.tries() < outboundDeliveryAttempts {
		if time.Now().After(deadline) {
			t.Fatalf("delivery attempts = %d, want the full budget %d", ch.tries(), outboundDeliveryAttempts)
		}
		time.Sleep(5 * time.Millisecond)
	}
	waitOpenState(t, backend, call.runID, "")

	// A failed intent is terminal: the next start must not retry it.
	host2 := New(Deps{
		Journal:    backend,
		Messages:   backend,
		Sessions:   backend,
		Deliveries: backend,
		Run:        runs.run,
		Channels:   []plugin.Channel{ch},
		Config:     config.Channels{"fake": {Enabled: true, AllowFrom: []string{"alice"}}},
		Logger:     testLogger(),
	})
	if err := host2.StartAll(context.Background()); err != nil {
		t.Fatalf("restart: %v", err)
	}
	time.Sleep(100 * time.Millisecond)
	if got := ch.tries(); got != outboundDeliveryAttempts {
		t.Fatalf("failed intent retried after restart: %d attempts, want %d", got, outboundDeliveryAttempts)
	}
	if got := ch.Snapshot(); len(got) != 0 {
		t.Fatalf("failing channel recorded sends: %+v", got)
	}
}

// TestRecoveryLeavesPendingIntentWhenPluginMissing keeps the durable row
// untouched. A temporarily uninstalled plugin must not panic startup or
// consume the retry budget; reinstalling it on a later boot can recover.
func TestRecoveryLeavesPendingIntentWhenPluginMissing(t *testing.T) {
	backend := openBackend(t)
	sessionID := ChannelSessionID("removed", "chat-1", "")
	runID := domain.RunID("run-plugin-missing")
	now := time.Now().UnixMilli()
	if err := backend.UpsertChannelDelivery(context.Background(), storage.ChannelDelivery{
		RunID: runID, SessionID: sessionID, Channel: "removed", ChatID: "chat-1",
		State: storage.ChannelDeliveryPending, Attempts: outboundDeliveryAttempts,
		CreatedAtMs: now, UpdatedAtMs: now,
	}); err != nil {
		t.Fatal(err)
	}
	host := New(Deps{
		Journal:    backend,
		Messages:   backend,
		Sessions:   backend,
		Deliveries: backend,
		Run:        (&runRecorder{messages: backend}).run,
		Logger:     testLogger(),
	})
	if err := host.StartAll(context.Background()); err != nil {
		t.Fatalf("start host: %v", err)
	}
	open := openIntent(t, backend)
	if len(open) != 1 || open[0].RunID != runID ||
		open[0].State != storage.ChannelDeliveryPending ||
		open[0].Attempts != outboundDeliveryAttempts {
		t.Fatalf("missing-plugin recovery mutated intent: %+v", open)
	}
}

// TestRecoveryLeavesPendingIntentWhenChannelDisabled proves that compiled is
// not equivalent to running. Recovery waits for a later boot that actually
// starts the adapter instead of sending through a disabled instance.
func TestRecoveryLeavesPendingIntentWhenChannelDisabled(t *testing.T) {
	backend := openBackend(t)
	sessionID := ChannelSessionID("fake", "chat-1", "")
	runID := domain.RunID("run-channel-disabled")
	now := time.Now().UnixMilli()
	if err := backend.UpsertChannelDelivery(context.Background(), storage.ChannelDelivery{
		RunID: runID, SessionID: sessionID, Channel: "fake", ChatID: "chat-1",
		State: storage.ChannelDeliveryPending, Attempts: 1,
		CreatedAtMs: now, UpdatedAtMs: now,
	}); err != nil {
		t.Fatal(err)
	}
	seedReply(t, backend, sessionID, runID)
	ch := fake.New()
	host := New(Deps{
		Journal:    backend,
		Messages:   backend,
		Sessions:   backend,
		Deliveries: backend,
		Run:        (&runRecorder{messages: backend}).run,
		Channels:   []plugin.Channel{ch},
		Config:     config.Channels{"fake": {Enabled: false}},
		Logger:     testLogger(),
	})
	if err := host.StartAll(context.Background()); err != nil {
		t.Fatalf("start host: %v", err)
	}
	time.Sleep(50 * time.Millisecond)
	if sent := ch.Snapshot(); len(sent) != 0 {
		t.Fatalf("disabled channel sent recovered reply: %+v", sent)
	}
	open := openIntent(t, backend)
	if len(open) != 1 || open[0].RunID != runID ||
		open[0].State != storage.ChannelDeliveryPending || open[0].Attempts != 1 {
		t.Fatalf("disabled-channel recovery mutated intent: %+v", open)
	}
}

// TestFastTerminalBeforeRunReturnsIsDelivered reproduces the ordering race:
// a runtime may publish a terminal event before Run returns. The host must
// have registered and durably armed the target before that event is visible.
func TestFastTerminalBeforeRunReturnsIsDelivered(t *testing.T) {
	backend := openBackend(t)
	ch := fake.New()
	ch.Publish = func(context.Context, plugin.ChannelEnv) error { return nil }
	var host *Host
	run := func(ctx context.Context, sessionID domain.SessionID, _ string, _ []domain.Attachment, _ *domain.Provenance, prepare PrepareRunFunc) (domain.RunID, error) {
		runID := domain.RunID("run-fast-terminal")
		if err := prepare(runID); err != nil {
			return "", err
		}
		if err := backend.AppendMessage(ctx, domain.Message{
			ID: "msg-fast-terminal", SessionID: sessionID, RunID: runID,
			Role: domain.RoleAssistant, Content: "fast reply", CreatedAt: time.Now().UnixMilli(),
		}); err != nil {
			return "", err
		}
		host.OnRunEvent(ctx, domain.RunEvent{
			RunID: runID, Type: domain.EventRunCompleted,
			CreatedAt: time.Now().UnixMilli(), PayloadVersion: 1,
		})
		return runID, nil
	}
	host = New(Deps{
		Journal:     backend,
		Messages:    backend,
		Sessions:    backend,
		Deliveries:  backend,
		RunPrepared: run,
		Channels:    []plugin.Channel{ch},
		Config:      config.Channels{"fake": {Enabled: true, AllowFrom: []string{"alice"}}},
		Logger:      testLogger(),
	})
	if err := host.StartAll(context.Background()); err != nil {
		t.Fatalf("start host: %v", err)
	}
	publishHello(t, host, ch, "m-fast-terminal")
	deadline := time.Now().Add(time.Second)
	for len(ch.Snapshot()) == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	sent := ch.Snapshot()
	if len(sent) != 1 || sent[0].Parts[0].Text != "fast reply" {
		t.Fatalf("fast terminal delivery = %+v, want one reply", sent)
	}
	if open := openIntent(t, backend); len(open) != 0 {
		t.Fatalf("fast terminal left durable intent open: %+v", open)
	}
}

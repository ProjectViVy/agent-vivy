package channelhost

import (
	"context"
	"strings"
	"testing"
	"time"

	"agent-vivy/internal/channelhost/fake"
	"agent-vivy/internal/config"
	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/sqlite"
	plugin "agent-vivy/sdk/port/channel"
)

// seedFailedIntent writes one failed delivery row directly, the shape
// failDelivery parks after the attempt budget is spent.
func seedFailedIntent(t *testing.T, backend *sqlite.Backend, runID domain.RunID, attempts int) {
	t.Helper()
	sessionID := ChannelSessionID("fake", "chat-1", "")
	now := time.Now().UnixMilli()
	if err := backend.UpsertChannelDelivery(context.Background(), storage.ChannelDelivery{
		RunID: runID, SessionID: sessionID, Channel: "fake", ChatID: "chat-1",
		State: storage.ChannelDeliveryFailed, Attempts: attempts,
		CreatedAtMs: now, UpdatedAtMs: now,
	}); err != nil {
		t.Fatalf("seed failed intent: %v", err)
	}
}

// failedIntent polls the failed listing until the run appears with the
// wanted attempt count.
func failedIntent(t *testing.T, backend *sqlite.Backend, runID domain.RunID, wantAttempts int) storage.ChannelDelivery {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		failed, err := backend.ListFailedChannelDeliveries(context.Background())
		if err != nil {
			t.Fatalf("list failed intents: %v", err)
		}
		for _, d := range failed {
			if d.RunID == runID && (wantAttempts <= 0 || d.Attempts == wantAttempts) {
				return d
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("failed intent %s never reached attempts %d; failed = %+v", runID, wantAttempts, failed)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// TestRedeliverDeliverySendsAndSettles: the operator re-arms a failed
// intent after reviving the channel; the reply is delivered through the
// adapter and the row is deleted.
func TestRedeliverDeliverySendsAndSettles(t *testing.T) {
	backend := openBackend(t)
	runID := domain.RunID("run-redeliver-ok")
	seedFailedIntent(t, backend, runID, outboundDeliveryAttempts)
	seedReply(t, backend, ChannelSessionID("fake", "chat-1", ""), runID)

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
		t.Fatalf("start all: %v", err)
	}
	if err := host.RedeliverDelivery(context.Background(), runID); err != nil {
		t.Fatalf("redeliver: %v", err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for len(ch.Snapshot()) == 0 {
		if time.Now().After(deadline) {
			t.Fatal("redelivered intent never reached the fake channel")
		}
		time.Sleep(5 * time.Millisecond)
	}
	sent := ch.Snapshot()
	if len(sent) != 1 || sent[0].Parts[0].Text != "channel reply" {
		t.Fatalf("redelivered envelopes = %+v, want the assistant reply", sent)
	}
	waitOpenState(t, backend, runID, "")
	if _, err := backend.ListFailedChannelDeliveries(context.Background()); err != nil {
		t.Fatalf("list failed after settle: %v", err)
	}
	if failed := listFailed(t, backend); len(failed) != 0 {
		t.Fatalf("delivered intent still failed: %+v", failed)
	}
}

// listFailed is failedIntent's non-polling sibling for whole-list asserts.
func listFailed(t *testing.T, backend *sqlite.Backend) []storage.ChannelDelivery {
	t.Helper()
	failed, err := backend.ListFailedChannelDeliveries(context.Background())
	if err != nil {
		t.Fatalf("list failed intents: %v", err)
	}
	return failed
}

// TestRedeliverDeliveryKeepsCumulativeBudget: with the budget spent, a
// redeliver against a still-dead channel is exactly one Send attempt, and
// the row re-parks as failed with attempts+1 — no retry loops.
func TestRedeliverDeliveryKeepsCumulativeBudget(t *testing.T) {
	backend := openBackend(t)
	runID := domain.RunID("run-redeliver-dead")
	seedFailedIntent(t, backend, runID, outboundDeliveryAttempts)
	seedReply(t, backend, ChannelSessionID("fake", "chat-1", ""), runID)

	ch := &failingChannel{Channel: fake.New()}
	ch.Channel.Publish = func(context.Context, plugin.ChannelEnv) error { return nil }
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
		t.Fatalf("start all: %v", err)
	}
	if err := host.RedeliverDelivery(context.Background(), runID); err != nil {
		t.Fatalf("redeliver: %v", err)
	}
	failedIntent(t, backend, runID, outboundDeliveryAttempts+1)
	if got := ch.tries(); got != 1 {
		t.Fatalf("redeliver sent %d attempts, want exactly 1", got)
	}
}

// TestRedeliverDeliveryRejectsOperatorErrors: unknown runs, non-failed
// intents, channels that are not running, and a draining host are all
// reported instead of silently swallowed.
func TestRedeliverDeliveryRejectsOperatorErrors(t *testing.T) {
	backend := openBackend(t)
	runID := domain.RunID("run-redeliver-err")
	seedFailedIntent(t, backend, runID, outboundDeliveryAttempts)

	ch := fake.New()
	ch.Publish = func(context.Context, plugin.ChannelEnv) error { return nil }
	newHost := func() *Host {
		return New(Deps{
			Journal:    backend,
			Messages:   backend,
			Sessions:   backend,
			Deliveries: backend,
			Run:        (&runRecorder{messages: backend}).run,
			Channels:   []plugin.Channel{ch},
			Config:     config.Channels{"fake": {Enabled: true, AllowFrom: []string{"alice"}}},
			Logger:     testLogger(),
		})
	}
	host := newHost()
	if err := host.StartAll(context.Background()); err != nil {
		t.Fatalf("start all: %v", err)
	}

	if err := host.RedeliverDelivery(context.Background(), domain.RunID("run-unknown")); err == nil {
		t.Fatal("unknown run accepted")
	}
	// An armed row is open, not failed: not a redeliver candidate.
	sessionID := ChannelSessionID("fake", "chat-1", "")
	now := time.Now().UnixMilli()
	if err := backend.UpsertChannelDelivery(context.Background(), storage.ChannelDelivery{
		RunID: "run-redeliver-armed", SessionID: sessionID, Channel: "fake", ChatID: "chat-1",
		State: storage.ChannelDeliveryArmed, CreatedAtMs: now, UpdatedAtMs: now,
	}); err != nil {
		t.Fatal(err)
	}
	if err := host.RedeliverDelivery(context.Background(), "run-redeliver-armed"); err == nil {
		t.Fatal("armed intent accepted as failed")
	}

	// A host that never started its channels refuses the redeliver.
	if err := newHost().RedeliverDelivery(context.Background(), runID); err == nil {
		t.Fatal("redeliver accepted while the channel is not running")
	} else if want := "not running"; !strings.Contains(err.Error(), want) {
		t.Fatalf("error = %v, want it to mention %q", err, want)
	}

	// A draining host refuses: the operator retries after restart.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	draining := newHost()
	if err := draining.StartAll(context.Background()); err != nil {
		t.Fatalf("start draining host: %v", err)
	}
	draining.StopAll(ctx)
	if err := draining.RedeliverDelivery(context.Background(), runID); err == nil {
		t.Fatal("redeliver accepted during drain")
	} else if want := "shutting down"; !strings.Contains(err.Error(), want) {
		t.Fatalf("error = %v, want it to mention %q", err, want)
	}
}

// TestFailedDeliveriesListsOnlyFailed pins the Host read surface: failed
// rows are visible, open rows are not.
func TestFailedDeliveriesListsOnlyFailed(t *testing.T) {
	backend := openBackend(t)
	failedRun := domain.RunID("run-redeliver-list-failed")
	seedFailedIntent(t, backend, failedRun, 3)
	sessionID := ChannelSessionID("fake", "chat-1", "")
	now := time.Now().UnixMilli()
	if err := backend.UpsertChannelDelivery(context.Background(), storage.ChannelDelivery{
		RunID: "run-redeliver-list-armed", SessionID: sessionID, Channel: "fake", ChatID: "chat-1",
		State: storage.ChannelDeliveryArmed, CreatedAtMs: now, UpdatedAtMs: now,
	}); err != nil {
		t.Fatal(err)
	}

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
		t.Fatalf("start all: %v", err)
	}
	failed, err := host.FailedDeliveries(context.Background())
	if err != nil {
		t.Fatalf("failed deliveries: %v", err)
	}
	if len(failed) != 1 || failed[0].RunID != failedRun {
		t.Fatalf("failed deliveries = %+v, want only %s", failed, failedRun)
	}
}

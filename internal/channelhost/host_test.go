package channelhost

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"agent-vivy/internal/channelhost/fake"
	"agent-vivy/internal/config"
	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/sdk/plugin"
)

// recordingJournal wraps the backend journal and records every commit so
// the TCK can assert the channel.inbound append without scraping tables.
type recordingJournal struct {
	storage.Journal
	mu      sync.Mutex
	commits []storage.Commit
}

func (r *recordingJournal) Append(ctx context.Context, commit storage.Commit) (domain.EventSeq, error) {
	r.mu.Lock()
	r.commits = append(r.commits, commit)
	r.mu.Unlock()
	return r.Journal.Append(ctx, commit)
}

func (r *recordingJournal) snapshot() []storage.Commit {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]storage.Commit(nil), r.commits...)
}

// runCall records one RunFunc invocation.
type runCall struct {
	sessionID domain.SessionID
	text      string
	prov      *domain.Provenance
	runID     domain.RunID
}

// runRecorder fakes the app-injected RunFunc. It mirrors what
// *runtime.Service.RunWithOptions does to the message log: the user turn
// is persisted with its provenance before the run opens.
type runRecorder struct {
	messages storage.MessageStore
	mu       sync.Mutex
	calls    []runCall
	next     int
}

func (r *runRecorder) run(ctx context.Context, sessionID domain.SessionID, text string, prov *domain.Provenance) (domain.RunID, error) {
	r.mu.Lock()
	r.next++
	runID := domain.RunID(fmt.Sprintf("run-test-%d", r.next))
	call := runCall{sessionID: sessionID, text: text, runID: runID}
	if prov != nil {
		p := *prov
		call.prov = &p
	}
	r.calls = append(r.calls, call)
	r.mu.Unlock()

	if r.messages != nil {
		msg := domain.Message{
			ID:        fmt.Sprintf("msg-test-%d", r.next),
			SessionID: sessionID,
			RunID:     runID,
			Role:      domain.RoleUser,
			CreatedAt: time.Now().UnixMilli(),
			Content:   text,
		}
		if prov != nil {
			msg.Source = prov.Source
			msg.Channel = prov.Channel
			msg.ChatID = prov.ChatID
			msg.ChannelMessageID = prov.ChannelMessageID
		}
		if err := r.messages.AppendMessage(ctx, msg); err != nil {
			return "", fmt.Errorf("append user message: %w", err)
		}
	}
	return runID, nil
}

func (r *runRecorder) snapshot() []runCall {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]runCall(nil), r.calls...)
}

func openBackend(t *testing.T) *sqlite.Backend {
	t.Helper()
	backend, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "journal.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = backend.Close() })
	return backend
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// startAllowedHost builds a started host with the default fake publish:
// one hello text from sender "alice" in chat "chat-1". After StartAll the
// inbound has already been dispatched once.
func startAllowedHost(t *testing.T) (*sqlite.Backend, *recordingJournal, *runRecorder, *fake.Channel, *Host) {
	t.Helper()
	backend := openBackend(t)
	journal := &recordingJournal{Journal: backend}
	runs := &runRecorder{messages: backend}
	ch := fake.New()
	host := New(Deps{
		Journal:  journal,
		Messages: backend,
		Sessions: backend,
		Run:      runs.run,
		Channels: []plugin.Channel{ch},
		Config:   config.Channels{"fake": {Enabled: true, AllowFrom: []string{"alice"}}},
		Logger:   testLogger(),
	})
	if err := host.StartAll(context.Background()); err != nil {
		t.Fatalf("start all: %v", err)
	}
	return backend, journal, runs, ch, host
}

// TestStartAllRefusesEmptyAllowFrom: enabled with an empty allow_from
// never starts, and a later PublishInbound attempt is dropped (fail
// closed at dispatch too).
func TestStartAllRefusesEmptyAllowFrom(t *testing.T) {
	backend := openBackend(t)
	runs := &runRecorder{messages: backend}
	journal := &recordingJournal{Journal: backend}
	ch := fake.New()
	host := New(Deps{
		Journal:  journal,
		Messages: backend,
		Sessions: backend,
		Run:      runs.run,
		Channels: []plugin.Channel{ch},
		Config:   config.Channels{"fake": {Enabled: true}},
		Logger:   testLogger(),
	})
	if err := host.StartAll(context.Background()); err != nil {
		t.Fatalf("start all: %v", err)
	}
	if got := host.Started(); len(got) != 0 {
		t.Fatalf("started channels = %v, want none", got)
	}
	env := host.envFor(ch)
	err := env.PublishInbound(context.Background(), plugin.InboundMessage{
		Channel: "fake", ChatID: "chat-1", Sender: "alice", MessageID: "m-late",
		Parts: []plugin.Part{{Kind: plugin.PartText, Text: "late hello"}},
	})
	if err != nil {
		t.Fatalf("publish inbound: %v", err)
	}
	if calls := runs.snapshot(); len(calls) != 0 {
		t.Fatalf("run called %d times after refusal, want 0", len(calls))
	}
	if _, err := backend.GetSession(context.Background(), ChannelSessionID("fake", "chat-1", "")); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("dropped inbound created a session: err=%v", err)
	}
	if commits := journal.snapshot(); len(commits) != 0 {
		t.Fatalf("dropped inbound journaled commits: %+v", commits)
	}
}

// TestPublishInboundDropsSenderNotInAllowFrom: the allow-list match is
// exact; a non-listed sender never reaches the session, the journal, or
// the run — and PublishInbound still reports nil (dropping is policy).
func TestPublishInboundDropsSenderNotInAllowFrom(t *testing.T) {
	backend := openBackend(t)
	runs := &runRecorder{messages: backend}
	journal := &recordingJournal{Journal: backend}
	ch := fake.New()
	ch.Publish = func(ctx context.Context, env plugin.ChannelEnv) error {
		return env.PublishInbound(ctx, plugin.InboundMessage{
			Channel: "fake", ChatID: "chat-1", Sender: "eve", MessageID: "m-2",
			Parts: []plugin.Part{{Kind: plugin.PartText, Text: "intrusion"}},
		})
	}
	host := New(Deps{
		Journal:  journal,
		Messages: backend,
		Sessions: backend,
		Run:      runs.run,
		Channels: []plugin.Channel{ch},
		Config:   config.Channels{"fake": {Enabled: true, AllowFrom: []string{"alice"}}},
		Logger:   testLogger(),
	})
	if err := host.StartAll(context.Background()); err != nil {
		t.Fatalf("start all: %v", err)
	}
	if got := host.Started(); len(got) != 1 || got[0] != "fake" {
		t.Fatalf("started channels = %v, want [fake]", got)
	}
	if calls := runs.snapshot(); len(calls) != 0 {
		t.Fatalf("run called for non-allow-listed sender: %+v", calls)
	}
	if _, err := backend.GetSession(context.Background(), ChannelSessionID("fake", "chat-1", "")); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("non-allow-listed sender created a session: err=%v", err)
	}
	if commits := journal.snapshot(); len(commits) != 0 {
		t.Fatalf("non-allow-listed sender journaled commits: %+v", commits)
	}
}

// TestPublishInboundAllowedSenderJournalsAndRuns: the allowed inbound
// journals channel.inbound under its pseudo run, opens the run with
// channel provenance, and lands the user message with Source=channel.
func TestPublishInboundAllowedSenderJournalsAndRuns(t *testing.T) {
	backend, journal, runs, _, _ := startAllowedHost(t)
	ctx := context.Background()

	calls := runs.snapshot()
	if len(calls) != 1 {
		t.Fatalf("run calls = %d, want 1", len(calls))
	}
	call := calls[0]
	if call.sessionID != ChannelSessionID("fake", "chat-1", "") {
		t.Fatalf("run session = %q, want deterministic channel session", call.sessionID)
	}
	if call.text != "hello vivy" {
		t.Fatalf("run text = %q, want the inbound text", call.text)
	}
	if call.prov == nil || call.prov.Source != "channel" || call.prov.Channel != "fake" ||
		call.prov.ChatID != "chat-1" || call.prov.ChannelMessageID != "m-1" {
		t.Fatalf("run provenance = %+v, want channel/fake/chat-1/m-1", call.prov)
	}

	commits := journal.snapshot()
	if len(commits) != 1 {
		t.Fatalf("journal commits = %d, want 1 (the channel.inbound pseudo run)", len(commits))
	}
	commit := commits[0]
	if !strings.HasPrefix(string(commit.RunID), "chanin_") || len(string(commit.RunID)) != len("chanin_")+16 {
		t.Fatalf("pseudo run id = %q, want chanin_<16hex>", commit.RunID)
	}
	if len(commit.Events) != 1 || commit.Events[0].Type != domain.EventChannelInbound {
		t.Fatalf("commit events = %+v, want one channel.inbound", commit.Events)
	}
	ev := commit.Events[0]
	if ev.PayloadVersion != 1 || ev.RunID != commit.RunID {
		t.Fatalf("inbound event = version %d run %q, want version 1 under the pseudo run", ev.PayloadVersion, ev.RunID)
	}
	var payload map[string]any
	if err := json.Unmarshal(ev.Payload, &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	for key, want := range map[string]string{
		"channel":    "fake",
		"chat_id":    "chat-1",
		"sender":     "alice",
		"message_id": "m-1",
		"session_id": string(call.sessionID),
	} {
		if got, _ := payload[key].(string); got != want {
			t.Fatalf("payload[%s] = %v, want %q", key, payload[key], want)
		}
	}
	if _, present := payload["run_id"]; present {
		t.Fatalf("payload carries run_id this slice: %v", payload)
	}
	// The pseudo run replays through the ordinary Journal surface.
	it, err := backend.Replay(ctx, commit.RunID, 0)
	if err != nil {
		t.Fatalf("replay pseudo run: %v", err)
	}
	var replayed int
	for it.Next() {
		if it.Value().Event.Type != domain.EventChannelInbound {
			t.Fatalf("pseudo run holds non-inbound event %s", it.Value().Event.Type)
		}
		replayed++
	}
	if err := it.Err(); err != nil {
		t.Fatalf("replay iteration: %v", err)
	}
	if err := it.Close(); err != nil {
		t.Fatalf("close replay: %v", err)
	}
	if replayed != 1 {
		t.Fatalf("pseudo run replays %d events, want 1", replayed)
	}

	msgs, err := backend.ListMessages(ctx, call.sessionID)
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if len(msgs) != 1 || msgs[0].Role != domain.RoleUser {
		t.Fatalf("messages = %+v, want one user turn", msgs)
	}
	msg := msgs[0]
	if msg.Source != "channel" || msg.Channel != "fake" || msg.ChatID != "chat-1" || msg.ChannelMessageID != "m-1" {
		t.Fatalf("user message provenance = %+v, want channel/fake/chat-1/m-1", msg)
	}
	if msg.RunID != call.runID {
		t.Fatalf("user message RunID = %q, want %q", msg.RunID, call.runID)
	}
	sess, err := backend.GetSession(ctx, call.sessionID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if sess.Title != "channel/fake/chat-1" {
		t.Fatalf("session title = %q, want channel/fake/chat-1", sess.Title)
	}
}

// TestChannelSessionIDDeterministic pins the mapping: stable id, the
// sess_ch_ namespace, and topic-scoped divergence.
func TestChannelSessionIDDeterministic(t *testing.T) {
	a := ChannelSessionID("fake", "chat-1", "")
	b := ChannelSessionID("fake", "chat-1", "")
	if a != b {
		t.Fatalf("mapping is not deterministic: %q vs %q", a, b)
	}
	if !strings.HasPrefix(string(a), "sess_ch_") {
		t.Fatalf("session id %q lacks the sess_ch_ namespace", a)
	}
	suffix := strings.TrimPrefix(string(a), "sess_ch_")
	if len(suffix) != 16 {
		t.Fatalf("session id suffix len = %d, want 16", len(suffix))
	}
	for _, r := range suffix {
		if !strings.ContainsRune("0123456789abcdef", r) {
			t.Fatalf("session id suffix %q is not lowercase hex", suffix)
		}
	}
	if ChannelSessionID("fake", "chat-1", "topic-9") == a {
		t.Fatal("topic must be part of the mapping")
	}
	if ChannelSessionID("fake", "chat-2", "") == a {
		t.Fatal("chat id must be part of the mapping")
	}

	backend := openBackend(t)
	host := New(Deps{
		Journal:  backend,
		Messages: backend,
		Sessions: backend,
		Run:      (&runRecorder{messages: backend}).run,
		Logger:   testLogger(),
	})
	ctx := context.Background()
	first, err := host.EnsureSession(ctx, "fake", "chat-1", "")
	if err != nil {
		t.Fatalf("ensure session: %v", err)
	}
	second, err := host.EnsureSession(ctx, "fake", "chat-1", "")
	if err != nil {
		t.Fatalf("ensure session again: %v", err)
	}
	if first != second {
		t.Fatalf("EnsureSession drifted: %q vs %q", first, second)
	}
	sess, err := backend.GetSession(ctx, first)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if sess.Title != "channel/fake/chat-1" {
		t.Fatalf("session title = %q, want channel/fake/chat-1", sess.Title)
	}
	topicID, err := host.EnsureSession(ctx, "fake", "chat-1", "t9")
	if err != nil {
		t.Fatalf("ensure session with topic: %v", err)
	}
	if topicID != ChannelSessionID("fake", "chat-1", "t9") || topicID == first {
		t.Fatalf("topic session id = %q, want a distinct deterministic id", topicID)
	}
	topicSess, err := backend.GetSession(ctx, topicID)
	if err != nil {
		t.Fatalf("get topic session: %v", err)
	}
	if topicSess.Title != "channel/fake/chat-1#t9" {
		t.Fatalf("topic session title = %q, want channel/fake/chat-1#t9", topicSess.Title)
	}
}

// TestOnRunEventDeliversAssistantReply: a tracked run.completed delivers
// the run's last assistant message to the originating chat, exactly once.
func TestOnRunEventDeliversAssistantReply(t *testing.T) {
	backend, _, runs, ch, host := startAllowedHost(t)
	ctx := context.Background()
	call := runs.snapshot()[0]

	if err := backend.AppendMessage(ctx, domain.Message{
		ID:        "msg-assistant-1",
		SessionID: call.sessionID,
		RunID:     call.runID,
		Role:      domain.RoleAssistant,
		CreatedAt: time.Now().UnixMilli(),
		Content:   "channel reply",
	}); err != nil {
		t.Fatalf("seed assistant message: %v", err)
	}

	host.OnRunEvent(ctx, domain.RunEvent{
		RunID: call.runID, Type: domain.EventRunCompleted,
		CreatedAt: time.Now().UnixMilli(), PayloadVersion: 1,
	})

	// Delivery hops off the runtime goroutine; poll for it.
	deadline := time.Now().Add(2 * time.Second)
	for len(ch.Snapshot()) == 0 {
		if time.Now().After(deadline) {
			t.Fatal("assistant reply was never delivered to the fake channel")
		}
		time.Sleep(5 * time.Millisecond)
	}
	sent := ch.Snapshot()
	if len(sent) != 1 {
		t.Fatalf("sent envelopes = %d, want 1", len(sent))
	}
	if sent[0].ChatID != "chat-1" {
		t.Fatalf("outbound chat = %q, want chat-1", sent[0].ChatID)
	}
	if len(sent[0].Parts) != 1 || sent[0].Parts[0].Kind != plugin.PartText || sent[0].Parts[0].Text != "channel reply" {
		t.Fatalf("outbound parts = %+v, want one text part with the assistant content", sent[0].Parts)
	}

	// The tracking entry is consumed: a replayed terminal delivers nothing.
	host.OnRunEvent(ctx, domain.RunEvent{
		RunID: call.runID, Type: domain.EventRunCompleted,
		CreatedAt: time.Now().UnixMilli(), PayloadVersion: 1,
	})
	time.Sleep(100 * time.Millisecond)
	if got := len(ch.Snapshot()); got != 1 {
		t.Fatalf("replayed terminal delivered again: %d envelopes", got)
	}
}

// TestOnRunEventFailedDeliversNothing: failed and cancelled runs log and
// deliver nothing this slice, and still release the tracking entry.
func TestOnRunEventFailedDeliversNothing(t *testing.T) {
	_, _, runs, ch, host := startAllowedHost(t)
	call := runs.snapshot()[0]
	host.OnRunEvent(context.Background(), domain.RunEvent{
		RunID: call.runID, Type: domain.EventRunFailed,
		CreatedAt: time.Now().UnixMilli(), PayloadVersion: 1,
	})
	time.Sleep(100 * time.Millisecond)
	if got := ch.Snapshot(); len(got) != 0 {
		t.Fatalf("failed run delivered: %+v", got)
	}
	// The entry is consumed: the same terminal is not re-handled.
	host.OnRunEvent(context.Background(), domain.RunEvent{
		RunID: call.runID, Type: domain.EventRunCompleted,
		CreatedAt: time.Now().UnixMilli(), PayloadVersion: 1,
	})
	time.Sleep(100 * time.Millisecond)
	if got := ch.Snapshot(); len(got) != 0 {
		t.Fatalf("late completed delivered after failed terminal: %+v", got)
	}
}

// TestStartAllIgnoresConfigWithoutPlugin: an envelope naming a channel
// that is not compiled in is app-level (partitionChannels); StartAll
// itself ignores it without panicking.
func TestStartAllIgnoresConfigWithoutPlugin(t *testing.T) {
	backend := openBackend(t)
	runs := &runRecorder{messages: backend}
	ch := fake.New()
	host := New(Deps{
		Journal:  backend,
		Messages: backend,
		Sessions: backend,
		Run:      runs.run,
		Channels: []plugin.Channel{ch},
		Config: config.Channels{
			"ghost": {Enabled: true, AllowFrom: []string{"someone"}},
		},
		Logger: testLogger(),
	})
	if err := host.StartAll(context.Background()); err != nil {
		t.Fatalf("start all: %v", err)
	}
	if got := host.Started(); len(got) != 0 {
		t.Fatalf("started channels = %v, want none (fake is not configured)", got)
	}
	if calls := runs.snapshot(); len(calls) != 0 {
		t.Fatalf("run called without a started channel: %+v", calls)
	}
}

// TestDiscoverReportsOnlyImplementedCapabilities: the fake implements no
// optional interface; one stub flips exactly its own bit.
func TestDiscoverReportsOnlyImplementedCapabilities(t *testing.T) {
	if got := Discover(fake.New()); got != (Capabilities{}) {
		t.Fatalf("fake advertises capabilities: %+v", got)
	}
	got := Discover(&typingStub{Channel: fake.New()})
	if !got.Typing {
		t.Fatalf("typing stub did not advertise Typing: %+v", got)
	}
	if got.Edit || got.Delete || got.Reaction || got.Placeholder || got.Media ||
		got.MediaStore || got.Webhook || got.Listen || got.Stream || got.Health {
		t.Fatalf("typing stub advertised extra capabilities: %+v", got)
	}
}

// typingStub adds exactly one capability to the fake.
type typingStub struct {
	*fake.Channel
}

func (typingStub) Typing(_ context.Context, _ string) error { return nil }

// renamedChannel rebrands a fake adapter so one host can carry several
// channel names without triggering the duplicate-name guard.
type renamedChannel struct {
	*fake.Channel
	name string
}

func (c renamedChannel) Name() string { return c.name }

// TestInspectNotesRecordStartAllDecisions: every StartAll decision lands in
// the per-channel note — started stays empty, and skip/fail reasons are
// recorded for unconfigured, disabled, empty-allow-from, and start-failed
// channels. Inspect is deterministic (sorted by name) and reports every
// compiled-in channel regardless of configuration.
func TestInspectNotesRecordStartAllDecisions(t *testing.T) {
	backend := openBackend(t)
	runs := &runRecorder{messages: backend}
	ok := renamedChannel{fake.New(), "fake"}
	refused := renamedChannel{fake.New(), "refused"}
	refused.Publish = func(context.Context, plugin.ChannelEnv) error {
		return errors.New("boom: platform unreachable")
	}
	host := New(Deps{
		Journal:  backend,
		Messages: backend,
		Sessions: backend,
		Run:      runs.run,
		Channels: []plugin.Channel{refused, ok},
		Config: config.Channels{
			"fake":    {Enabled: true, AllowFrom: []string{"alice"}},
			"refused": {Enabled: true, AllowFrom: []string{"alice"}},
		},
		Logger: testLogger(),
	})
	// Inspect before StartAll: no notes yet, nothing started.
	for _, status := range host.Inspect() {
		if status.Note != "" {
			t.Fatalf("pre-start note for %q = %q, want empty", status.Name, status.Note)
		}
	}
	if err := host.StartAll(context.Background()); err != nil {
		t.Fatalf("start all: %v", err)
	}

	statuses := host.Inspect()
	if len(statuses) != 2 {
		t.Fatalf("inspect = %+v, want the two compiled-in channels", statuses)
	}
	if statuses[0].Name != "fake" || statuses[1].Name != "refused" {
		t.Fatalf("inspect order = [%s, %s], want deterministic name order", statuses[0].Name, statuses[1].Name)
	}
	started := statuses[0]
	if !started.Started || !started.Configured || !started.Enabled || started.Note != "" {
		t.Fatalf("started status = %+v, want configured+enabled+started with an empty note", started)
	}
	if started.Capabilities != (Capabilities{}) {
		t.Fatalf("fake capabilities = %+v, want none", started.Capabilities)
	}
	failed := statuses[1]
	if failed.Started || !failed.Configured || !failed.Enabled {
		t.Fatalf("failed status = %+v, want configured+enabled but not started", failed)
	}
	if failed.Note != "start failed: boom: platform unreachable" {
		t.Fatalf("failed note = %q, want the start-failed reason", failed.Note)
	}
}

// TestInspectNotesForSkips: unconfigured, disabled, and empty-allow-from
// channels carry their exact fail-closed notes.
func TestInspectNotesForSkips(t *testing.T) {
	backend := openBackend(t)
	runs := &runRecorder{messages: backend}
	ch := fake.New()
	host := New(Deps{
		Journal:  backend,
		Messages: backend,
		Sessions: backend,
		Run:      runs.run,
		Channels: []plugin.Channel{ch},
		Config:   config.Channels{}, // compiled-in but unconfigured
		Logger:   testLogger(),
	})
	if err := host.StartAll(context.Background()); err != nil {
		t.Fatalf("start all: %v", err)
	}
	statuses := host.Inspect()
	if len(statuses) != 1 {
		t.Fatalf("inspect = %+v, want [fake]", statuses)
	}
	if got := statuses[0].Note; got != "no config envelope" {
		t.Fatalf("unconfigured note = %q, want %q", got, "no config envelope")
	}
	if statuses[0].Started || statuses[0].Configured || statuses[0].Enabled {
		t.Fatalf("unconfigured status = %+v", statuses[0])
	}

	// Disabled envelope.
	host = New(Deps{
		Journal:  backend,
		Messages: backend,
		Sessions: backend,
		Run:      runs.run,
		Channels: []plugin.Channel{ch},
		Config:   config.Channels{"fake": {Enabled: false, AllowFrom: []string{"alice"}}},
		Logger:   testLogger(),
	})
	if err := host.StartAll(context.Background()); err != nil {
		t.Fatalf("start all: %v", err)
	}
	if got := host.Inspect()[0].Note; got != "disabled" {
		t.Fatalf("disabled note = %q, want %q", got, "disabled")
	}

	// Empty allow_from refusal.
	host = New(Deps{
		Journal:  backend,
		Messages: backend,
		Sessions: backend,
		Run:      runs.run,
		Channels: []plugin.Channel{ch},
		Config:   config.Channels{"fake": {Enabled: true}},
		Logger:   testLogger(),
	})
	if err := host.StartAll(context.Background()); err != nil {
		t.Fatalf("start all: %v", err)
	}
	if got := host.Inspect()[0].Note; got != "empty allow_from — start refused" {
		t.Fatalf("empty allow_from note = %q", got)
	}
}

// TestInspectTokenEnvSet: TokenEnv carries the envelope's env NAME only;
// TokenEnvSet resolves the variable via os.LookupEnv and is false when it
// is unset or empty. Values never cross the inspect surface.
func TestInspectTokenEnvSet(t *testing.T) {
	backend := openBackend(t)
	ch := fake.New()
	host := New(Deps{
		Journal:  backend,
		Messages: backend,
		Sessions: backend,
		Channels: []plugin.Channel{ch},
		Config: config.Channels{
			"fake": {Enabled: false, TokenEnv: "VIVY_TEST_CHANNEL_TOKEN_INSPECT"},
		},
		Logger: testLogger(),
	})
	t.Setenv("VIVY_TEST_CHANNEL_TOKEN_INSPECT", "")

	status := host.Inspect()[0]
	if status.TokenEnv != "VIVY_TEST_CHANNEL_TOKEN_INSPECT" {
		t.Fatalf("token_env = %q, want the declared env name", status.TokenEnv)
	}
	if status.TokenEnvSet {
		t.Fatal("TokenEnvSet = true for an unset env variable")
	}
	if status.Note != "" {
		// No StartAll ran yet; notes start empty.
		t.Fatalf("note before StartAll = %q, want empty", status.Note)
	}

	t.Setenv("VIVY_TEST_CHANNEL_TOKEN_INSPECT", "secret-value-never-inspected")
	status = host.Inspect()[0]
	if !status.TokenEnvSet {
		t.Fatal("TokenEnvSet = false for a set env variable")
	}
}

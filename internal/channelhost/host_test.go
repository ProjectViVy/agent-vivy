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
	"unicode/utf8"

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

// TestEnsureSessionConcurrentSameChat drives CH-C4-N2: a burst of
// concurrent inbound for one chat races EnsureSession's read→create→
// re-read path. Every caller must receive the same deterministic id
// with no error — the losers of the unique insert re-read the winner's
// row — and exactly one session lands in the store.
func TestEnsureSessionConcurrentSameChat(t *testing.T) {
	backend := openBackend(t)
	runs := &runRecorder{messages: backend}
	host := New(Deps{
		Journal:  backend,
		Messages: backend,
		Sessions: backend,
		Run:      runs.run,
		Logger:   testLogger(),
	})
	ctx := context.Background()

	const n = 32
	ids := make([]domain.SessionID, n)
	errs := make([]error, n)
	var ready sync.WaitGroup
	ready.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer ready.Done()
			ids[i], errs[i] = host.EnsureSession(ctx, "fake", "chat-1", "")
		}(i)
	}
	ready.Wait()

	want := ChannelSessionID("fake", "chat-1", "")
	for i := 0; i < n; i++ {
		if errs[i] != nil {
			t.Fatalf("EnsureSession caller %d: %v", i, errs[i])
		}
		if ids[i] != want {
			t.Fatalf("EnsureSession caller %d = %q, want %q", i, ids[i], want)
		}
	}
	sessions, err := backend.ListSessions(ctx)
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("sessions = %d rows, want exactly 1 (no duplicate create)", len(sessions))
	}
	if sessions[0].Title != "channel/fake/chat-1" {
		t.Fatalf("session title = %q, want channel/fake/chat-1", sessions[0].Title)
	}
}

// TestConcurrentInboundSameChatDispatch is the end-to-end face of the
// same burst: N allowed inbound messages for one chat dispatched
// concurrently through the full publish pipeline. All N runs must open
// on the one deterministic session with their own provenance, all N
// user turns and N journal commits must land, and the session must be
// created exactly once (run with -race).
func TestConcurrentInboundSameChatDispatch(t *testing.T) {
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
	ctx := context.Background()
	env := host.envFor(ch)

	const n = 8
	var ready sync.WaitGroup
	ready.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer ready.Done()
			_ = env.PublishInbound(ctx, plugin.InboundMessage{
				Channel: "fake", ChatID: "chat-1", Sender: "alice",
				MessageID: fmt.Sprintf("m-burst-%d", i),
				Parts:     []plugin.Part{{Kind: plugin.PartText, Text: fmt.Sprintf("burst %d", i)}},
			})
		}(i)
	}
	ready.Wait()

	want := ChannelSessionID("fake", "chat-1", "")
	calls := runs.snapshot()
	if len(calls) != n {
		t.Fatalf("run calls = %d, want %d", len(calls), n)
	}
	seen := map[domain.RunID]bool{}
	for _, call := range calls {
		if call.sessionID != want {
			t.Fatalf("run opened on %q, want the deterministic channel session %q", call.sessionID, want)
		}
		if call.prov == nil || call.prov.Source != "channel" || call.prov.ChatID != "chat-1" {
			t.Fatalf("run provenance = %+v, want channel provenance for chat-1", call.prov)
		}
		if seen[call.runID] {
			t.Fatalf("run id %q dispatched twice", call.runID)
		}
		seen[call.runID] = true
	}
	if commits := journal.snapshot(); len(commits) != n {
		t.Fatalf("journal commits = %d, want %d (one channel.inbound per message)", len(commits), n)
	}
	msgs, err := backend.ListMessages(ctx, want)
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if len(msgs) != n {
		t.Fatalf("user messages = %d, want %d", len(msgs), n)
	}
	sessions, err := backend.ListSessions(ctx)
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("sessions = %d rows, want exactly 1 under the concurrent burst", len(sessions))
	}
}

// runesLimited wraps a fake channel with the plugin.RunesLimiter
// capability so tests can drive the CH-C4-N1 splitting path without
// teaching the fake (which stays capability-free) about limits.
type runesLimited struct {
	plugin.Channel
	runes int
}

func (c runesLimited) MaxMessageRunes() int { return c.runes }

// TestSplitRunes pins the chunking rules: no limit passes through, exact
// fit stays whole, over-limit hard-breaks at the rune boundary, and a
// newline inside the window wins over a mid-word break. A newline at the
// window start never produces an empty chunk.
func TestSplitRunes(t *testing.T) {
	cases := []struct {
		name    string
		content string
		limit   int
		want    []string
	}{
		{"no limit", "abc", 0, []string{"abc"}},
		{"negative limit", "abc", -1, []string{"abc"}},
		{"exact fit", "abcd", 4, []string{"abcd"}},
		{"under limit", "abc", 4, []string{"abc"}},
		{"hard break", "abcdef", 4, []string{"abcd", "ef"}},
		{"newline preferred", "ab\ncdef", 4, []string{"ab\n", "cdef"}},
		{"newline at zero ignored", "\ncdef", 4, []string{"\ncde", "f"}},
		{"last newline in window", "ab\ncd\nefgh", 5, []string{"ab\n", "cd\n", "efgh"}},
		{"multibyte runes", "aé字字字", 3, []string{"aé字", "字字"}},
	}
	for _, tc := range cases {
		got := splitRunes(tc.content, tc.limit)
		if len(got) != len(tc.want) {
			t.Fatalf("%s: splitRunes(%q,%d) = %q, want %q", tc.name, tc.content, tc.limit, got, tc.want)
		}
		var joined strings.Builder
		for i, chunk := range got {
			if chunk == "" {
				t.Fatalf("%s: empty chunk at %d", tc.name, i)
			}
			joined.WriteString(chunk)
			if tc.limit > 0 && utf8.RuneCountInString(chunk) > tc.limit {
				t.Fatalf("%s: chunk %q exceeds limit %d", tc.name, chunk, tc.limit)
			}
		}
		if joined.String() != tc.content {
			t.Fatalf("%s: chunks %q reassemble to %q, want the original", tc.name, got, joined.String())
		}
	}
}

// TestDeliverySplitsAtAdapterRunesLimit drives CH-C4-N1 end to end: an
// adapter declaring a bound receives an over-limit reply as sequential
// in-order sends that reassemble to the original text; an adapter without
// the capability still gets the whole message.
func TestDeliverySplitsAtAdapterRunesLimit(t *testing.T) {
	ctx := context.Background()
	long := strings.Repeat("段落", 10) + "\n" + strings.Repeat("tail", 10) // 21 + 40 runes
	for _, tc := range []struct {
		name      string
		limit     int // 0 = the adapter does not implement RunesLimiter
		wantParts int
	}{
		{"limited adapter splits", 50, 2},
		{"unlimited adapter gets whole message", 0, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			backend := openBackend(t)
			runs := &runRecorder{messages: backend}
			fakeCh := fake.New()
			var ch plugin.Channel = fakeCh
			if tc.limit != 0 {
				ch = runesLimited{Channel: fakeCh, runes: tc.limit}
			}
			host := New(Deps{
				Journal:  backend,
				Messages: backend,
				Sessions: backend,
				Run:      runs.run,
				Channels: []plugin.Channel{ch},
				Config:   config.Channels{"fake": {Enabled: true, AllowFrom: []string{"alice"}}},
				Logger:   testLogger(),
			})
			env := host.envFor(ch)
			if err := env.PublishInbound(ctx, plugin.InboundMessage{
				Channel: "fake", ChatID: "chat-1", Sender: "alice", MessageID: "m-long",
				Parts: []plugin.Part{{Kind: plugin.PartText, Text: "long please"}},
			}); err != nil {
				t.Fatalf("publish inbound: %v", err)
			}
			call := runs.snapshot()[0]
			if err := backend.AppendMessage(ctx, domain.Message{
				ID: "msg-long", SessionID: call.sessionID, RunID: call.runID,
				Role: domain.RoleAssistant, CreatedAt: time.Now().UnixMilli(),
				Content: long,
			}); err != nil {
				t.Fatalf("seed assistant message: %v", err)
			}
			host.OnRunEvent(ctx, domain.RunEvent{
				RunID: call.runID, Type: domain.EventRunCompleted,
				CreatedAt: time.Now().UnixMilli(), PayloadVersion: 1,
			})
			// Delivery hops off the runtime goroutine; poll until every
			// expected envelope landed (sends are sequential, so a count
			// below the expectation just means the next chunk is in flight).
			var sent []plugin.OutboundMessage
			deadline := time.Now().Add(2 * time.Second)
			for {
				sent = fakeCh.Snapshot()
				if len(sent) >= tc.wantParts {
					break
				}
				if time.Now().After(deadline) {
					t.Fatalf("delivered %d envelopes, want %d", len(sent), tc.wantParts)
				}
				time.Sleep(5 * time.Millisecond)
			}
			var joined strings.Builder
			for i, msg := range sent {
				if msg.ChatID != "chat-1" {
					t.Fatalf("envelope %d chat = %q, want chat-1", i, msg.ChatID)
				}
				if len(msg.Parts) != 1 || msg.Parts[0].Kind != plugin.PartText {
					t.Fatalf("envelope %d parts = %+v, want one text part", i, msg.Parts)
				}
				joined.WriteString(msg.Parts[0].Text)
			}
			if joined.String() != long {
				t.Fatalf("reassembled reply %q, want the original text", joined.String())
			}
		})
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

// TestDeliverCompletedDropsUnregisteredChannel: an envelope naming a
// channel absent from Deps.Channels passes dispatch (StartAll ignores such
// envelopes, the pipeline does not), so the run is tracked with a nil
// channel. The completed terminal must drop the delivery with a warning —
// pre-guard this panicked the delivery goroutine on the nil interface.
func TestDeliverCompletedDropsUnregisteredChannel(t *testing.T) {
	backend := openBackend(t)
	runs := &runRecorder{messages: backend}
	ch := fake.New()
	host := New(Deps{
		Journal:  backend,
		Messages: backend,
		Sessions: backend,
		Run:      runs.run,
		Channels: []plugin.Channel{ch},
		Config:   config.Channels{"ghost": {Enabled: true, AllowFrom: []string{"alice"}}},
		Logger:   testLogger(),
	})
	env := host.envFor(ch)
	err := env.PublishInbound(context.Background(), plugin.InboundMessage{
		Channel: "ghost", ChatID: "chat-9", Sender: "alice", MessageID: "m-ghost",
		Parts: []plugin.Part{{Kind: plugin.PartText, Text: "hello ghost"}},
	})
	if err != nil {
		t.Fatalf("publish inbound: %v", err)
	}
	calls := runs.snapshot()
	if len(calls) != 1 {
		t.Fatalf("run calls = %d, want 1", len(calls))
	}
	host.OnRunEvent(context.Background(), domain.RunEvent{
		RunID: calls[0].runID, Type: domain.EventRunCompleted,
		CreatedAt: time.Now().UnixMilli(), PayloadVersion: 1,
	})
	// Delivery runs on a detached goroutine; a panic there would kill the
	// test process, so reaching here with nothing delivered is the pass.
	time.Sleep(100 * time.Millisecond)
	if got := ch.Snapshot(); len(got) != 0 {
		t.Fatalf("unregistered channel delivered: %+v", got)
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
	if got := started.AllowFrom; len(got) != 1 || got[0] != "alice" {
		t.Fatalf("started allow_from = %v, want the startup-effective summary [alice]", got)
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
	if statuses[0].AllowFrom != nil {
		t.Fatalf("unconfigured allow_from = %v, want nil", statuses[0].AllowFrom)
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

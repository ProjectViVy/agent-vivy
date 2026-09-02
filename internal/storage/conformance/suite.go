// Package conformance is the D-032 backend suite (CN-01..CN-17).
package conformance

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

// DualOpenMode is how CN-14 interprets a second Open on the same Journal.
type DualOpenMode int

const (
	// DualOpenShared is the SQLite file model: two handles may write; the
	// journal stays contiguous.
	DualOpenShared DualOpenMode = iota
	// DualOpenExclusive is the server model: the second Open must fail with
	// storage.ErrLeaseHeld.
	DualOpenExclusive
)

// Slot is one isolated database plus reopen/second-open hooks.
type Slot struct {
	Engine     storage.Engine
	Reopen     func() (storage.Engine, error)
	OpenSecond func() (storage.Engine, error)
}

// Harness binds the suite to one backend.
type Harness struct {
	DualOpen DualOpenMode
	Setup    func(t *testing.T) Slot
}

// Run executes CN-01..CN-17.
func Run(t *testing.T, h Harness) {
	t.Helper()
	cases := []struct {
		id   string
		name string
		run  func(*testing.T, Harness)
	}{
		{"CN-01", "atomic append", cnAtomicAppend},
		{"CN-02", "monotonic sequence", cnMonotonicSequence},
		{"CN-03", "expected-version conflict", cnExpectedVersionConflict},
		{"CN-04", "idempotent replay", cnIdempotentReplay},
		{"CN-05", "retry divergence refused", cnRetryDivergenceRefused},
		{"CN-06", "exactly one terminal", cnExactlyOneTerminal},
		{"CN-07", "first-writer-wins approval", cnFirstWriterWinsApproval},
		{"CN-08", "restart repair view", cnRestartRepairView},
		{"CN-09", "torn final write survives reopen", cnTornFinalWrite},
		{"CN-10", "malformed payload round-trip", cnMalformedPayload},
		{"CN-11", "orphan checkpoint generations", cnOrphanCheckpoint},
		{"CN-12", "approval decision after kill", cnApprovalAfterKill},
		{"CN-13", "payload byte fidelity (secret audit anchor)", cnPayloadByteFidelity},
		{"CN-14", "dual-handle process lock safety", cnDualHandleSafety},
		{"CN-15", "monotonic replay under concurrent writers", cnConcurrentWriters},
		{"CN-16", "replay after disconnect (after_seq tail)", cnReplayAfterDisconnect},
		{"CN-17", "message provenance round-trip", cnMessageProvenance},
		{"CN-18", "file version chain + stale-read tracker", cnFileVersionChain},
		{"CN-19", "runs listed by session", cnRunsBySession},
		{"CN-20", "compactions listed by session", cnCompactionsBySession},
	}
	if len(cases) != 20 {
		t.Fatalf("conformance suite must carry exactly 20 cases, got %d", len(cases))
	}
	for _, c := range cases {
		t.Run(c.id+" "+c.name, func(t *testing.T) { c.run(t, h) })
	}
}

func fresh(t *testing.T, h Harness) storage.Engine {
	t.Helper()
	slot := h.Setup(t)
	t.Cleanup(func() { _ = slot.Engine.Close() })
	return slot.Engine
}

func replayAll(t *testing.T, b storage.Journal, runID domain.RunID, after domain.EventSeq) []domain.RunEvent {
	t.Helper()
	it, err := b.Replay(context.Background(), runID, after)
	if err != nil {
		t.Fatalf("Replay: %v", err)
	}
	defer func() { _ = it.Close() }()
	var out []domain.RunEvent
	for it.Next() {
		out = append(out, it.Value().Event)
	}
	if it.Err() != nil {
		t.Fatalf("iterator: %v", it.Err())
	}
	return out
}

func ev(typ domain.EventType) domain.RunEvent {
	return domain.RunEvent{Type: typ, CreatedAt: 1, PayloadVersion: 1, Payload: []byte(`{}`)}
}

func cnAtomicAppend(t *testing.T, h Harness) {
	b := fresh(t, h)
	ctx := context.Background()
	if _, err := b.Append(ctx, storage.Commit{
		RunID: "run-1",
		Events: []domain.RunEvent{
			ev(domain.EventRunStarted), ev(domain.EventModelDelta), ev(domain.EventModelCompleted),
		},
	}); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if got := replayAll(t, b, "run-1", 0); len(got) != 3 {
		t.Fatalf("events = %d, want all 3 of the commit", len(got))
	}
	if _, err := b.Append(ctx, storage.Commit{
		RunID: "run-1",
		Events: []domain.RunEvent{
			ev(domain.EventRunCompleted), ev(domain.EventRunFailed),
		},
	}); !errors.Is(err, storage.ErrCommitInvalid) {
		t.Fatalf("two-terminal commit: err = %v, want ErrCommitInvalid", err)
	}
	if got := replayAll(t, b, "run-1", 0); len(got) != 3 {
		t.Fatalf("rejected commit leaked rows: %d events", len(got))
	}
}

func cnMonotonicSequence(t *testing.T, h Harness) {
	b := fresh(t, h)
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		seq, err := b.Append(ctx, storage.Commit{
			RunID:  "run-seq",
			Events: []domain.RunEvent{ev(domain.EventModelDelta)},
		})
		if err != nil {
			t.Fatalf("Append %d: %v", i, err)
		}
		if seq != domain.EventSeq(i+1) {
			t.Fatalf("seq = %d, want %d", seq, i+1)
		}
	}
	got := replayAll(t, b, "run-seq", 0)
	for i, e := range got {
		if e.Seq != domain.EventSeq(i+1) {
			t.Fatalf("replay seq[%d] = %d, want contiguous", i, e.Seq)
		}
	}
}

func cnExpectedVersionConflict(t *testing.T, h Harness) {
	b := fresh(t, h)
	ctx := context.Background()
	snap := b.Snapshot()
	if err := snap.Put(ctx, "k", []byte("v"), 1); !errors.Is(err, storage.ErrVersionConflict) {
		t.Fatalf("create with expectVersion=1: err = %v, want conflict", err)
	}
	if err := snap.Put(ctx, "k", []byte("v"), 0); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := snap.Put(ctx, "k", []byte("v2"), 0); !errors.Is(err, storage.ErrVersionConflict) {
		t.Fatalf("stale create: err = %v, want conflict", err)
	}
}

func cnIdempotentReplay(t *testing.T, h Harness) {
	b := fresh(t, h)
	ctx := context.Background()
	if _, err := b.Append(ctx, storage.Commit{
		RunID: "run-replay",
		Events: []domain.RunEvent{
			ev(domain.EventRunStarted), ev(domain.EventModelDelta), ev(domain.EventRunCompleted),
		},
	}); err != nil {
		t.Fatalf("Append: %v", err)
	}
	first := replayAll(t, b, "run-replay", 0)
	second := replayAll(t, b, "run-replay", 0)
	if len(first) != len(second) {
		t.Fatalf("replay lengths diverged: %d vs %d", len(first), len(second))
	}
	for i := range first {
		if first[i].Seq != second[i].Seq || first[i].Type != second[i].Type {
			t.Fatalf("replay pass diverged at %d", i)
		}
	}
}

func cnRetryDivergenceRefused(t *testing.T, h Harness) {
	b := fresh(t, h)
	ctx := context.Background()
	if _, err := b.Append(ctx, storage.Commit{
		RunID:  "run-retry",
		Events: []domain.RunEvent{ev(domain.EventRunStarted), ev(domain.EventRunCompleted)},
	}); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if _, err := b.Append(ctx, storage.Commit{
		RunID:  "run-retry",
		Events: []domain.RunEvent{ev(domain.EventModelDelta)},
	}); !errors.Is(err, storage.ErrRunClosed) {
		t.Fatalf("divergent retry: err = %v, want ErrRunClosed", err)
	}
	if got := replayAll(t, b, "run-retry", 0); len(got) != 2 {
		t.Fatalf("journal grew under a refused retry: %d events", len(got))
	}
}

func cnExactlyOneTerminal(t *testing.T, h Harness) {
	b := fresh(t, h)
	ctx := context.Background()
	if _, err := b.Append(ctx, storage.Commit{
		RunID: "run-one",
		Events: []domain.RunEvent{
			ev(domain.EventRunStarted), ev(domain.EventRunCompleted),
		},
	}); err != nil {
		t.Fatalf("Append: %v", err)
	}
	for _, typ := range []domain.EventType{
		domain.EventRunCompleted, domain.EventRunFailed, domain.EventRunCancelled, domain.EventModelDelta,
	} {
		if _, err := b.Append(ctx, storage.Commit{
			RunID: "run-one", Events: []domain.RunEvent{ev(typ)},
		}); !errors.Is(err, storage.ErrRunClosed) {
			t.Fatalf("append %s after terminal: err = %v, want ErrRunClosed", typ, err)
		}
	}
	terminals := 0
	for _, e := range replayAll(t, b, "run-one", 0) {
		if e.Type.Terminal() {
			terminals++
		}
	}
	if terminals != 1 {
		t.Fatalf("terminal events = %d, want exactly 1", terminals)
	}
}

func cnFirstWriterWinsApproval(t *testing.T, h Harness) {
	b := fresh(t, h)
	ctx := context.Background()
	apr := domain.Approval{
		ID: "apr-fw", RunID: "run-fw", ToolCallID: "tc-1",
		Decision: domain.ApprovalPending, ExpiresAt: time.Now().Add(time.Minute).UnixMilli(),
	}
	if err := b.CreateApproval(ctx, apr); err != nil {
		t.Fatalf("CreateApproval: %v", err)
	}

	var wg sync.WaitGroup
	results := make(chan bool, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok, err := b.DecideApproval(ctx, apr.ID, domain.ApprovalApproved)
			if err != nil {
				t.Errorf("DecideApproval: %v", err)
				return
			}
			results <- ok
		}()
	}
	wg.Wait()
	close(results)
	wins := 0
	for ok := range results {
		if ok {
			wins++
		}
	}
	if wins != 1 {
		t.Fatalf("concurrent decisions won = %d, want exactly 1", wins)
	}
	if pending, _ := b.ListPendingApprovals(ctx); len(pending) != 0 {
		t.Fatalf("pending after decision = %d, want 0", len(pending))
	}
}

func cnRestartRepairView(t *testing.T, h Harness) {
	slot := h.Setup(t)
	ctx := context.Background()
	b := slot.Engine
	if err := b.CreateSession(ctx, domain.Session{ID: "sess-rr", Title: "t", CreatedAt: 1}); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if err := b.CreateRun(ctx, domain.Run{ID: "run-rr", SessionID: "sess-rr", Status: domain.RunActive, CreatedAt: 2}); err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	if err := b.CreateApproval(ctx, domain.Approval{
		ID: "apr-rr", RunID: "run-rr", Decision: domain.ApprovalPending, ExpiresAt: 9999,
	}); err != nil {
		t.Fatalf("CreateApproval: %v", err)
	}
	if err := b.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	b, err := slot.Reopen()
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	t.Cleanup(func() { _ = b.Close() })
	active, err := b.ListActiveRuns(ctx)
	if err != nil || len(active) != 1 || active[0].ID != "run-rr" {
		t.Fatalf("active after reopen = %+v, %v", active, err)
	}
	pending, err := b.ListPendingApprovals(ctx)
	if err != nil || len(pending) != 1 || pending[0].ID != "apr-rr" {
		t.Fatalf("pending after reopen = %+v, %v", pending, err)
	}
}

func cnTornFinalWrite(t *testing.T, h Harness) {
	slot := h.Setup(t)
	ctx := context.Background()
	b := slot.Engine
	if _, err := b.Append(ctx, storage.Commit{
		RunID: "run-torn",
		Events: []domain.RunEvent{
			ev(domain.EventRunStarted), ev(domain.EventModelDelta), ev(domain.EventModelCompleted),
		},
	}); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if err := b.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	b, err := slot.Reopen()
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	t.Cleanup(func() { _ = b.Close() })
	got := replayAll(t, b, "run-torn", 0)
	if len(got) != 3 || got[2].Type != domain.EventModelCompleted {
		t.Fatalf("reopen replay = %d events, tail %v; want the committed stream", len(got), got[len(got)-1].Type)
	}
}

func cnMalformedPayload(t *testing.T, h Harness) {
	b := fresh(t, h)
	ctx := context.Background()
	malformed := []byte(`{"broken":`)
	if _, err := b.Append(ctx, storage.Commit{
		RunID: "run-bad",
		Events: []domain.RunEvent{
			{Type: domain.EventModelDelta, CreatedAt: 1, PayloadVersion: 1, Payload: malformed},
		},
	}); err != nil {
		t.Fatalf("Append malformed payload: %v", err)
	}
	got := replayAll(t, b, "run-bad", 0)
	if len(got) != 1 || !bytes.Equal(got[0].Payload, malformed) {
		t.Fatalf("malformed payload was rewritten: %q", got[0].Payload)
	}
}

func cnOrphanCheckpoint(t *testing.T, h Harness) {
	b := fresh(t, h)
	ctx := context.Background()
	if err := b.Blobs().Put(ctx, "ckpt-orphan", []byte("gen-1")); err != nil {
		t.Fatalf("Put: %v", err)
	}
	orphaner, ok := b.(storage.CheckpointOrphaner)
	if !ok {
		t.Fatal("engine does not implement CheckpointOrphaner")
	}
	if err := orphaner.DropCheckpointPointer(ctx, "ckpt-orphan"); err != nil {
		t.Fatalf("drop pointer: %v", err)
	}
	n, err := orphaner.CountCheckpointGenerations(ctx, "ckpt-orphan")
	if err != nil {
		t.Fatalf("count generations: %v", err)
	}
	if n != 1 {
		t.Fatalf("orphan generations = %d, want 1 visible for recovery", n)
	}
	if err := orphaner.DeleteCheckpointGenerations(ctx, "ckpt-orphan"); err != nil {
		t.Fatalf("cleanup orphan: %v", err)
	}
	n, err = orphaner.CountCheckpointGenerations(ctx, "ckpt-orphan")
	if err != nil {
		t.Fatalf("recount: %v", err)
	}
	if n != 0 {
		t.Fatalf("orphan cleanup left %d rows", n)
	}
}

func cnApprovalAfterKill(t *testing.T, h Harness) {
	slot := h.Setup(t)
	ctx := context.Background()
	b := slot.Engine
	if err := b.CreateApproval(ctx, domain.Approval{
		ID: "apr-kill", RunID: "run-kill", ToolCallID: "tc-1",
		Decision: domain.ApprovalPending, ExpiresAt: 9999, ResumeTarget: "rt",
	}); err != nil {
		t.Fatalf("CreateApproval: %v", err)
	}
	if err := b.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	b, err := slot.Reopen()
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	t.Cleanup(func() { _ = b.Close() })
	ok, err := b.DecideApproval(ctx, "apr-kill", domain.ApprovalApproved)
	if err != nil || !ok {
		t.Fatalf("decide after reopen = %v/%v, want granted", ok, err)
	}
	apr, err := b.GetApproval(ctx, "apr-kill")
	if err != nil || apr.Decision != domain.ApprovalApproved {
		t.Fatalf("approval after decide = %+v, %v", apr, err)
	}
}

func cnPayloadByteFidelity(t *testing.T, h Harness) {
	b := fresh(t, h)
	canary := []byte(`{"secret":"cn13-byte-fidelity-canary"}`)
	ctx := context.Background()
	if _, err := b.Append(ctx, storage.Commit{
		RunID: "run-canary",
		Events: []domain.RunEvent{
			{Type: domain.EventModelDelta, CreatedAt: 1, PayloadVersion: 1, Payload: canary},
		},
	}); err != nil {
		t.Fatalf("Append: %v", err)
	}
	got := replayAll(t, b, "run-canary", 0)
	if len(got) != 1 || !bytes.Equal(got[0].Payload, canary) {
		t.Fatalf("payload mutated in storage: %q", got[0].Payload)
	}
}

func cnDualHandleSafety(t *testing.T, h Harness) {
	slot := h.Setup(t)
	ctx := context.Background()
	first := slot.Engine
	t.Cleanup(func() { _ = first.Close() })
	second, err := slot.OpenSecond()
	switch h.DualOpen {
	case DualOpenExclusive:
		if !errors.Is(err, storage.ErrLeaseHeld) {
			if second != nil {
				_ = second.Close()
			}
			t.Fatalf("second Open = %v, want ErrLeaseHeld", err)
		}
		if _, err := first.Append(ctx, storage.Commit{
			RunID: "run-dual", Events: []domain.RunEvent{ev(domain.EventRunStarted)},
		}); err != nil {
			t.Fatalf("append via first handle: %v", err)
		}
		if got := replayAll(t, first, "run-dual", 0); len(got) != 1 {
			t.Fatalf("exclusive journal = %d events, want the first commit", len(got))
		}
		return
	default:
		if err != nil {
			t.Fatalf("second Open must not fail or corrupt: %v", err)
		}
		t.Cleanup(func() { _ = second.Close() })
		if _, err := first.Append(ctx, storage.Commit{
			RunID: "run-dual", Events: []domain.RunEvent{ev(domain.EventRunStarted)},
		}); err != nil {
			t.Fatalf("append via first handle: %v", err)
		}
		if _, err := second.Append(ctx, storage.Commit{
			RunID: "run-dual", Events: []domain.RunEvent{ev(domain.EventRunCompleted)},
		}); err != nil {
			t.Fatalf("append via second handle: %v", err)
		}
		got := replayAll(t, first, "run-dual", 0)
		if len(got) != 2 || got[1].Type != domain.EventRunCompleted {
			t.Fatalf("dual-handle journal = %d events, want both commits", len(got))
		}
	}
}

func cnConcurrentWriters(t *testing.T, h Harness) {
	b := fresh(t, h)
	ctx := context.Background()
	const writers, perRun = 8, 10

	var wg sync.WaitGroup
	errs := make(chan error, writers)
	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			runID := domain.RunID(fmt.Sprintf("run-cw-%d", w))
			for i := 0; i < perRun; i++ {
				if _, err := b.Append(ctx, storage.Commit{
					RunID:  runID,
					Events: []domain.RunEvent{ev(domain.EventModelDelta)},
				}); err != nil {
					errs <- fmt.Errorf("writer %d append %d: %w", w, i, err)
					return
				}
			}
		}(w)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	for w := 0; w < writers; w++ {
		got := replayAll(t, b, domain.RunID(fmt.Sprintf("run-cw-%d", w)), 0)
		if len(got) != perRun {
			t.Fatalf("run %d events = %d, want %d", w, len(got), perRun)
		}
		for i, e := range got {
			if e.Seq != domain.EventSeq(i+1) {
				t.Fatalf("run %d seq[%d] = %d, want contiguous", w, i, e.Seq)
			}
		}
	}
}

func cnReplayAfterDisconnect(t *testing.T, h Harness) {
	b := fresh(t, h)
	ctx := context.Background()
	if _, err := b.Append(ctx, storage.Commit{
		RunID: "run-tail",
		Events: []domain.RunEvent{
			ev(domain.EventRunStarted),
			ev(domain.EventModelDelta),
			ev(domain.EventModelDelta),
			ev(domain.EventRunCompleted),
		},
	}); err != nil {
		t.Fatalf("Append: %v", err)
	}
	tail := replayAll(t, b, "run-tail", 2)
	if len(tail) != 2 || tail[0].Seq != 3 || tail[1].Seq != 4 {
		t.Fatalf("tail = %+v, want exactly seq 3..4", tail)
	}
	if tail[1].Type != domain.EventRunCompleted {
		t.Fatalf("tail terminal = %s, want run.completed", tail[1].Type)
	}
}

func cnMessageProvenance(t *testing.T, h Harness) {
	b := fresh(t, h)
	ctx := context.Background()
	if err := b.CreateSession(ctx, domain.Session{ID: "sess-prov", Title: "t", CreatedAt: 1}); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	channelMsg := domain.Message{
		ID: "msg-prov-channel", SessionID: "sess-prov", Role: domain.RoleUser,
		CreatedAt: 2, Content: "hello from the world",
		Source: "channel", Channel: "telegram", ChatID: "chat-123", ChannelMessageID: "tg-456",
	}
	legacyMsg := domain.Message{
		ID: "msg-prov-legacy", SessionID: "sess-prov", Role: domain.RoleUser,
		CreatedAt: 3, Content: "hello from the ui",
	}
	for i, m := range []domain.Message{channelMsg, legacyMsg} {
		if err := b.AppendMessage(ctx, m); err != nil {
			t.Fatalf("AppendMessage %d: %v", i, err)
		}
	}
	got, err := b.ListMessages(ctx, "sess-prov")
	if err != nil {
		t.Fatalf("ListMessages: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("messages = %d, want 2", len(got))
	}
	c := got[0]
	if c.ID != channelMsg.ID || c.Role != domain.RoleUser || c.Content != channelMsg.Content {
		t.Fatalf("channel row base fields drifted: %+v", c)
	}
	if c.Source != "channel" || c.Channel != "telegram" || c.ChatID != "chat-123" || c.ChannelMessageID != "tg-456" {
		t.Fatalf("channel provenance did not round-trip: %+v", c)
	}
	if c.EffectiveSource() != "channel" {
		t.Fatalf("EffectiveSource = %q, want channel", c.EffectiveSource())
	}
	l := got[1]
	if l.ID != legacyMsg.ID || l.Role != domain.RoleUser || l.Content != legacyMsg.Content {
		t.Fatalf("legacy row base fields drifted: %+v", l)
	}
	if l.Source != "" {
		t.Fatalf("legacy row Source = %q, want empty", l.Source)
	}
	if l.EffectiveSource() != "ui" {
		t.Fatalf("legacy EffectiveSource = %q, want ui", l.EffectiveSource())
	}
}

func cnFileVersionChain(t *testing.T, h Harness) {
	b := fresh(t, h)
	ctx := context.Background()
	if err := b.CreateSession(ctx, domain.Session{ID: "sess-fv", Title: "t", CreatedAt: 1}); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	// Record mutations without error and keep the tracker round-trip
	// exact. Chain contents (baseline/intermediate/dedupe/retention) are
	// asserted by backend-local tests that can query the table directly;
	// the interface intentionally exposes no version reads until a restore
	// consumer exists (RB-L2-DEFER).
	mutations := []struct{ old, new string }{
		{"", "v1"},
		{"v1", "v2"},
		{"v2", "v3"},
	}
	for i, m := range mutations {
		if err := b.RecordFileMutation(ctx, "sess-fv", "run-fv", "a.go", []byte(m.old), []byte(m.new)); err != nil {
			t.Fatalf("RecordFileMutation %d: %v", i, err)
		}
	}

	if _, ok, err := b.LastFileAccess(ctx, "sess-fv", "a.go"); ok || err != nil {
		t.Fatalf("LastFileAccess before tracking = ok=%v err=%v, want ok=false err=nil", ok, err)
	}
	if err := b.TrackFileAccess(ctx, "sess-fv", "a.go", 100); err != nil {
		t.Fatalf("TrackFileAccess: %v", err)
	}
	if err := b.TrackFileAccess(ctx, "sess-fv", "a.go", 200); err != nil {
		t.Fatalf("TrackFileAccess (upsert): %v", err)
	}
	at, ok, err := b.LastFileAccess(ctx, "sess-fv", "a.go")
	if err != nil || !ok || at != 200 {
		t.Fatalf("LastFileAccess = (%d, %v, %v), want (200, true, nil)", at, ok, err)
	}
	if _, ok, _ := b.LastFileAccess(ctx, "sess-fv", "other.go"); ok {
		t.Fatalf("LastFileAccess for untracked path = ok, want not ok")
	}

	// Session deletion cascades both tables in one transaction.
	if err := b.DeleteSession(ctx, "sess-fv"); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}
	if _, ok, err := b.LastFileAccess(ctx, "sess-fv", "a.go"); ok || err != nil {
		t.Fatalf("LastFileAccess after DeleteSession = ok=%v err=%v, want ok=false err=nil", ok, err)
	}
}

func cnRunsBySession(t *testing.T, h Harness) {
	b := fresh(t, h)
	ctx := context.Background()
	runs := []domain.Run{
		{ID: "run-a", SessionID: "sess-pin", Status: domain.RunCompleted, CreatedAt: 1},
		{ID: "run-b", SessionID: "sess-pin", Status: domain.RunActive, CreatedAt: 2},
		{ID: "run-x", SessionID: "sess-other", Status: domain.RunActive, CreatedAt: 3},
	}
	for _, r := range runs {
		if err := b.CreateRun(ctx, r); err != nil {
			t.Fatalf("CreateRun %s: %v", r.ID, err)
		}
	}
	got, err := b.ListRunsBySession(ctx, "sess-pin")
	if err != nil {
		t.Fatalf("ListRunsBySession: %v", err)
	}
	if len(got) != 2 || got[0].ID != "run-a" || got[1].ID != "run-b" {
		t.Fatalf("ListRunsBySession(sess-pin) = %+v, want [run-a run-b] in creation order", got)
	}
	if got[1].Status != domain.RunActive || got[0].Status != domain.RunCompleted {
		t.Fatalf("ListRunsBySession must return all statuses, got %s then %s", got[0].Status, got[1].Status)
	}
	empty, err := b.ListRunsBySession(ctx, "sess-none")
	if err != nil || len(empty) != 0 {
		t.Fatalf("ListRunsBySession(unknown) = %+v, %v; want empty, nil", empty, err)
	}
}

func cnCompactionsBySession(t *testing.T, h Harness) {
	b := fresh(t, h)
	ctx := context.Background()
	records := []storage.SessionCompaction{
		{SessionID: "sess-cp", RunID: "run-c1", Summary: "older", TailFrom: 100, DroppedCount: 4, CreatedAt: 100},
		{SessionID: "sess-cp", RunID: "run-c2", Summary: "newer", TailFrom: 200, DroppedCount: 6, CreatedAt: 200},
		{SessionID: "sess-cp", RunID: "run-c9", Summary: "tie-newest", TailFrom: 300, DroppedCount: 2, CreatedAt: 200},
		{SessionID: "sess-other", RunID: "run-z", Summary: "elsewhere", TailFrom: 400, DroppedCount: 1, CreatedAt: 300},
	}
	for _, rec := range records {
		if err := b.SaveSessionCompaction(ctx, rec); err != nil {
			t.Fatalf("SaveSessionCompaction %s: %v", rec.RunID, err)
		}
	}
	got, err := b.ListSessionCompactions(ctx, "sess-cp", 10)
	if err != nil {
		t.Fatalf("ListSessionCompactions: %v", err)
	}
	wantOrder := []domain.RunID{"run-c9", "run-c2", "run-c1"}
	if len(got) != len(wantOrder) {
		t.Fatalf("ListSessionCompactions = %d rows, want %d", len(got), len(wantOrder))
	}
	for i, want := range wantOrder {
		if got[i].RunID != want {
			t.Fatalf("row %d = %s, want %s (newest first)", i, got[i].RunID, want)
		}
	}
	if got[0].Summary != "tie-newest" || got[0].DroppedCount != 2 || got[0].TailFrom != 300 {
		t.Fatalf("row 0 fields = %+v, want tie-newest record", got[0])
	}
	capped, err := b.ListSessionCompactions(ctx, "sess-cp", 2)
	if err != nil || len(capped) != 2 || capped[0].RunID != "run-c9" {
		t.Fatalf("limit=2 = %+v, %v; want top 2 newest", capped, err)
	}
	if none, err := b.ListSessionCompactions(ctx, "sess-none", 10); err != nil || len(none) != 0 {
		t.Fatalf("unknown session = %+v, %v; want empty, nil", none, err)
	}
	if zero, err := b.ListSessionCompactions(ctx, "sess-cp", 0); err != nil || len(zero) != 0 {
		t.Fatalf("limit=0 = %+v, %v; want empty, nil", zero, err)
	}
}

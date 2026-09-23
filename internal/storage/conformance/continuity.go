package conformance

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

// continuityStore unwraps the T4 store surface for the suite.
func continuityStore(t *testing.T, b storage.Engine) storage.ContinuityStore {
	t.Helper()
	c, ok := b.(storage.ContinuityStore)
	if !ok {
		t.Fatal("backend does not implement ContinuityStore")
	}
	return c
}

func continuityAdmission(sid domain.SessionID, runID domain.RunID, messageID, requestID, hash string) storage.ContinuityAdmission {
	scope, err := domain.NewAcceptedHistoryScope(sid, []domain.SessionID{sid})
	if err != nil {
		panic(err)
	}
	return storage.ContinuityAdmission{
		SessionID: sid,
		Message: domain.Message{
			ID: messageID, SessionID: sid, RunID: runID,
			Role: domain.RoleUser, CreatedAt: 7, Content: "continuity turn",
		},
		Run: domain.Run{ID: runID, SessionID: sid, Status: domain.RunActive, CreatedAt: 7},
		Events: []domain.RunEvent{
			{RunID: runID, Type: domain.EventRunStarted, CreatedAt: 7, PayloadVersion: 1, Payload: []byte(`{}`)},
			{RunID: runID, Type: domain.EventContextReferenceAttached, CreatedAt: 7, PayloadVersion: 1, Payload: []byte(`{}`)},
		},
		Scope:   scope,
		Receipt: domain.ContinuityReceipt{SessionID: sid, RunID: runID, Operation: storage.ContinuityOperationAdmission, RequestID: requestID, InputHash: hash},
	}
}

// AssertContinuityAtomic injects a fault immediately after each write step
// and asserts, after reopening the database, that either every admission
// record committed or none did — never a torn admission.
func AssertContinuityAtomic(t *testing.T, slot Slot) {
	t.Helper()
	b := slot.Engine
	ctx := context.Background()
	store := continuityStore(t, b)
	const sid = domain.SessionID("continuity-atomic")
	createHistorySession(t, b, sid)

	for _, step := range []string{"message", "run", "events", "receipt"} {
		step := step
		t.Run("fault after "+step, func(t *testing.T) {
			admission := continuityAdmission(sid, domain.RunID("r-"+step), "m-"+step, "req-"+step, "hash-"+step)
			admission.TestFaultHook = func(got string) error {
				if got == step {
					return fmt.Errorf("injected fault after %s", step)
				}
				return nil
			}
			if _, err := store.CommitContinuityRun(ctx, admission); err == nil {
				t.Fatalf("expected injected fault after %s", step)
			}

			reopened, err := slot.Reopen()
			if err != nil {
				t.Fatalf("reopen: %v", err)
			}
			defer func() { _ = reopened.Close() }()
			rstore := continuityStore(t, reopened)

			if _, err := reopened.GetRun(ctx, admission.Run.ID); !errors.Is(err, storage.ErrNotFound) {
				t.Fatalf("run row leaked after fault after %s: %v", step, err)
			}
			if _, found, err := rstore.FindContinuityReceipt(ctx, sid, storage.ContinuityOperationAdmission, admission.Receipt.RequestID); err != nil || found {
				t.Fatalf("receipt leaked after fault after %s (found=%v err=%v)", step, found, err)
			}
			if got := replayAll(t, reopened, admission.Run.ID, 0); len(got) != 0 {
				t.Fatalf("events leaked after fault after %s: %d", step, len(got))
			}
		})
	}

	// Clean commit: every record lands together.
	admission := continuityAdmission(sid, "r-ok", "m-ok", "req-ok", "hash-ok")
	result, err := store.CommitContinuityRun(ctx, admission)
	if err != nil {
		t.Fatalf("CommitContinuityRun: %v", err)
	}
	if !result.NewlyCommitted {
		t.Fatal("first admission must commit")
	}
	if _, err := b.GetRun(ctx, admission.Run.ID); err != nil {
		t.Fatalf("run row missing after commit: %v", err)
	}
	messages, err := b.ListMessages(ctx, sid)
	if err != nil {
		t.Fatalf("ListMessages: %v", err)
	}
	if len(messages) != 1 || messages[0].ID != admission.Message.ID {
		t.Fatalf("user row missing after commit: %+v", messages)
	}
	if got := replayAll(t, b, admission.Run.ID, 0); len(got) != 2 {
		t.Fatalf("startup events missing after commit: %d", len(got))
	}
}

// AssertContinuityRetry proves duplicate admission replays the original
// identity without launching a second engine and guarded operations replay
// their original result even after the run is terminal.
func AssertContinuityRetry(t *testing.T, slot Slot) {
	t.Helper()
	b := slot.Engine
	ctx := context.Background()
	store := continuityStore(t, b)
	const sid = domain.SessionID("continuity-retry")
	createHistorySession(t, b, sid)

	first, err := store.CommitContinuityRun(ctx, continuityAdmission(sid, "r-first", "m-first", "req-dup", "hash-dup"))
	if err != nil {
		t.Fatalf("first CommitContinuityRun: %v", err)
	}
	// A retry carries a fresh run/message identity but the same request id
	// and input hash — the queueing path reissues submission, not identity.
	retryAdmission := continuityAdmission(sid, "r-retry", "m-retry", "req-dup", "hash-dup")
	retry, err := store.CommitContinuityRun(ctx, retryAdmission)
	if err != nil {
		t.Fatalf("retry CommitContinuityRun: %v", err)
	}
	if first.RunID != retry.RunID {
		t.Fatal("retry admitted a second run")
	}
	if retry.NewlyCommitted {
		t.Fatal("retry must not launch engine")
	}
	countRunStarted, countReferences := 0, 0
	for _, e := range replayAll(t, b, first.RunID, 0) {
		switch e.Type {
		case domain.EventRunStarted:
			countRunStarted++
		case domain.EventContextReferenceAttached:
			countReferences++
		}
	}
	if countRunStarted != 1 || countReferences != 1 {
		t.Fatal("duplicate startup events")
	}
	if _, err := b.GetRun(ctx, "r-retry"); !errors.Is(err, storage.ErrNotFound) {
		t.Fatal("retry run row leaked")
	}
	messages, err := b.ListMessages(ctx, sid)
	if err != nil {
		t.Fatalf("ListMessages: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("duplicate user rows: %d", len(messages))
	}

	// Same request id with a different input hash is a conflict.
	conflict := continuityAdmission(sid, "r-conflict", "m-conflict", "req-dup", "hash-other")
	if _, err := store.CommitContinuityRun(ctx, conflict); !errors.Is(err, storage.ErrConflict) {
		t.Fatalf("different hash must be conflict: %v", err)
	}

	// Guarded operations: replay returns the original committed event even
	// after the run is terminal; a new mutation after terminal fails closed.
	mutation := storage.ContinuityMutation{
		SessionID: sid,
		RunID:     first.RunID,
		Event: domain.RunEvent{
			RunID: first.RunID, Type: domain.EventContextReferenceAttached,
			CreatedAt: 8, PayloadVersion: 1, Payload: []byte(`{"op":1}`),
		},
		InputHash: "op-hash-1",
		Receipt:   domain.ContinuityReceipt{SessionID: sid, Operation: "reference.attach", RequestID: "r-first:tc-1", InputHash: "op-hash-1"},
	}
	committed, err := store.CommitContinuityOperation(ctx, mutation)
	if err != nil {
		t.Fatalf("CommitContinuityOperation: %v", err)
	}
	if !committed.NewlyCommitted || len(committed.Events) != 1 {
		t.Fatalf("operation did not commit: %+v", committed)
	}
	// Terminal append through the plain journal wins the race with any later
	// guarded mutation.
	if _, err := b.Append(ctx, storage.Commit{RunID: first.RunID, Events: []domain.RunEvent{
		{RunID: first.RunID, Type: domain.EventRunCompleted, CreatedAt: 9, PayloadVersion: 1, Payload: []byte(`{"outcome":"ok"}`)},
	}}); err != nil {
		t.Fatalf("terminal append: %v", err)
	}
	replayed, err := store.CommitContinuityOperation(ctx, mutation)
	if err != nil {
		t.Fatalf("replay after terminal must return original result: %v", err)
	}
	if replayed.NewlyCommitted {
		t.Fatal("operation replay must not commit again")
	}
	if len(replayed.Events) != 1 || replayed.Events[0].Seq != committed.Events[0].Seq {
		t.Fatal("replay did not return the original event")
	}
	if _, err := store.CommitContinuityOperation(ctx, storage.ContinuityMutation{
		SessionID: sid,
		RunID:     first.RunID,
		Event: domain.RunEvent{
			RunID: first.RunID, Type: domain.EventContextReferenceAttached,
			CreatedAt: 10, PayloadVersion: 1, Payload: []byte(`{"op":2}`),
		},
		InputHash: "op-hash-2",
		Receipt:   domain.ContinuityReceipt{SessionID: sid, Operation: "reference.attach", RequestID: "r-first:tc-2", InputHash: "op-hash-2"},
	}); !errors.Is(err, storage.ErrRunClosed) {
		t.Fatalf("new mutation after terminal must fail run closed: %v", err)
	}
	// Same receipt identity with a changed hash is a conflict.
	if _, err := store.CommitContinuityOperation(ctx, storage.ContinuityMutation{
		SessionID: sid,
		RunID:     first.RunID,
		Event: domain.RunEvent{
			RunID: first.RunID, Type: domain.EventContextReferenceAttached,
			CreatedAt: 11, PayloadVersion: 1, Payload: []byte(`{"op":3}`),
		},
		InputHash: "op-hash-other",
		Receipt:   domain.ContinuityReceipt{SessionID: sid, Operation: "reference.attach", RequestID: "r-first:tc-1", InputHash: "op-hash-other"},
	}); !errors.Is(err, storage.ErrConflict) {
		t.Fatalf("changed operation hash must be conflict: %v", err)
	}
}

// AssertContinuityExpectations proves the commit transaction rechecks
// source visibility: a deleted or rewound source fails the admission
// instead of committing a stale snapshot.
func AssertContinuityExpectations(t *testing.T, slot Slot) {
	t.Helper()
	b := slot.Engine
	ctx := context.Background()
	store := continuityStore(t, b)
	const sid = domain.SessionID("continuity-dest")
	const src = domain.SessionID("continuity-src")
	createHistorySession(t, b, sid)
	createHistorySession(t, b, src)

	admission := continuityAdmission(sid, "r-exp", "m-exp", "req-exp", "hash-exp")
	scope, err := domain.NewAcceptedHistoryScope(sid, []domain.SessionID{sid, src})
	if err != nil {
		t.Fatalf("scope: %v", err)
	}
	admission.Scope = scope
	admission.Expectations = []storage.SourceExpectation{{SourceSessionID: src, TruncationMarkers: 0, Digest: "d1"}}
	if _, err := store.CommitContinuityRun(ctx, admission); err != nil {
		t.Fatalf("admission with valid expectations: %v", err)
	}

	// A source rewind between preview and commit moves the truncation
	// revision and must fail the admission.
	rewinder, ok := b.(storage.HistoryMutationStore)
	if !ok {
		t.Fatal("backend does not implement HistoryMutationStore")
	}
	marker := storage.SessionTruncation{SessionID: src, CutoffMessageID: "x", TailMessageID: "x", Reason: storage.TruncationRewind, CreatedAt: 1}
	if _, err := rewinder.CommitSessionRewind(ctx, marker, ev(domain.EventSessionTruncated)); err != nil {
		t.Fatalf("CommitSessionRewind: %v", err)
	}
	stale := continuityAdmission(sid, "r-exp2", "m-exp2", "req-exp2", "hash-exp2")
	stale.Scope = scope
	stale.Expectations = []storage.SourceExpectation{{SourceSessionID: src, TruncationMarkers: 0, Digest: "d1"}}
	if _, err := store.CommitContinuityRun(ctx, stale); !errors.Is(err, storage.ErrSourceChanged) {
		t.Fatalf("rewound source must fail expectation: %v", err)
	}

	// A deleted source fails admission outright.
	if err := b.DeleteSession(ctx, src); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}
	deleted := continuityAdmission(sid, "r-exp3", "m-exp3", "req-exp3", "hash-exp3")
	deleted.Scope = scope
	deleted.Expectations = []storage.SourceExpectation{{SourceSessionID: src, TruncationMarkers: 1, Digest: "d1"}}
	if _, err := store.CommitContinuityRun(ctx, deleted); !errors.Is(err, storage.ErrSourceChanged) {
		t.Fatalf("deleted source must fail expectation: %v", err)
	}
}

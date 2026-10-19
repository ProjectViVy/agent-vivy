package runtime

import (
	"context"
	"errors"
	"testing"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/sqlite"
)

var errCleanupWrite = errors.New("injected recovery write failure")

type cleanupFaultStore struct{ *sqlite.Backend }

func (f cleanupFaultStore) CancelApproval(context.Context, string, string, string) (bool, error) {
	return false, errCleanupWrite
}
func (f cleanupFaultStore) CancelQuestionWithMetadata(context.Context, string, string, string) (bool, error) {
	return false, errCleanupWrite
}

type cleanupFaultJournal struct {
	storage.Journal
	kind    domain.EventType
	ackLost bool
}

func (f cleanupFaultJournal) Append(ctx context.Context, c storage.Commit) (domain.EventSeq, error) {
	for _, e := range c.Events {
		if e.Type == f.kind {
			if f.ackLost {
				if _, err := f.Journal.Append(ctx, c); err != nil {
					return 0, err
				}
			}
			return 0, errCleanupWrite
		}
	}
	return f.Journal.Append(ctx, c)
}

type cleanupFaultRunStore struct{ storage.RunStore }

func (f cleanupFaultRunStore) SetRunStatus(context.Context, domain.RunID, domain.RunStatus) error {
	return errCleanupWrite
}

type cleanupFaultWork struct{ storage.WorkStore }

func (f cleanupFaultWork) CommitWork(ctx context.Context, m domain.WorkMutation) (storage.WorkCommitResult, error) {
	if m.Kind == domain.WorkEventPlanReviewCancelled {
		return storage.WorkCommitResult{}, errCleanupWrite
	}
	return f.WorkStore.CommitWork(ctx, m)
}

type cleanupFaultBlobs struct {
	storage.BlobStore
	storage.BlobPrefixLister
	failPut, failDelete bool
}

func (f cleanupFaultBlobs) Put(ctx context.Context, id string, data []byte) error {
	if f.failPut {
		return errCleanupWrite
	}
	return f.BlobStore.Put(ctx, id, data)
}
func (f cleanupFaultBlobs) Delete(ctx context.Context, id string) error {
	if f.failDelete {
		return errCleanupWrite
	}
	return f.BlobStore.Delete(ctx, id)
}

func TestRecoverCleanupIntentWriteFailures(t *testing.T) {
	for _, phase := range []string{"create", "delete"} {
		t.Run(phase, func(t *testing.T) {
			ctx := context.Background()
			initial, backend, _ := newApprovalService(t, 5*time.Minute)
			mustCreateSession(t, backend, "intent")
			runID, err := initial.Run(ctx, "intent", "note milk")
			if err != nil {
				t.Fatal(err)
			}
			waitForPendingApproval(t, backend, runID)
			waitForApprovalEvent(t, backend, runID)
			s, _ := restartServiceAt(t, backend, "test-engine-next")
			s.engine.cfg.Checkpoints.blobs = cleanupFaultBlobs{backend.Blobs(), backend.Blobs().(storage.BlobPrefixLister), phase == "create", phase == "delete"}
			if err = s.Recover(ctx); !errors.Is(err, errCleanupWrite) {
				t.Fatalf("recover = %v", err)
			}
			n := countTerminal(replayAll(t, backend, runID))
			if phase == "create" && n != 0 {
				t.Fatal("intent failure must precede all cleanup")
			}
			if phase == "delete" && n != 1 {
				t.Fatal("deletion failure must retain completed terminal")
			}
			s, _ = restartServiceAt(t, backend, "test-engine-next")
			for i := 0; i < 3; i++ {
				if err = s.Recover(ctx); err != nil {
					t.Fatal(err)
				}
			}
			assertEngineChangedFailure(t, backend, runID)
			n = 0
			for _, ev := range replayAll(t, backend, runID) {
				if ev.Type == domain.EventToolApprovalCancelled {
					n++
				}
			}
			if n != 1 {
				t.Fatalf("cancellations = %d", n)
			}
			ids, err := backend.Blobs().(storage.BlobPrefixLister).ListPrefix(ctx, checkpointCleanupPrefix)
			if err != nil || len(ids) != 0 {
				t.Fatalf("remaining intents = %v / %v", ids, err)
			}
		})
	}
}

// These faults straddle independent durable writes, not just model failures.
// A fresh Service must finish cleanup without losing or duplicating an event.
func TestRecoverCleanupRetriesInteractionWrites(t *testing.T) {
	for _, kind := range []string{"approval", "question"} {
		for _, fault := range []string{"row", "event", "event_ack", "terminal", "terminal_ack", "status"} {
			t.Run(kind+"/"+fault, func(t *testing.T) {
				ctx := context.Background()
				var backend *sqlite.Backend
				var runID domain.RunID
				var restart func() *Service
				var cancelType domain.EventType
				if kind == "approval" {
					initial, b, _ := newApprovalService(t, 5*time.Minute)
					backend = b
					mustCreateSession(t, b, "cleanup")
					var err error
					runID, err = initial.Run(ctx, "cleanup", "note milk")
					if err != nil {
						t.Fatal(err)
					}
					waitForPendingApproval(t, b, runID)
					waitForApprovalEvent(t, b, runID)
					restart = func() *Service { s, _ := restartServiceAt(t, b, "test-engine-next"); return s }
					cancelType = domain.EventToolApprovalCancelled
				} else {
					initial, b, _ := newQuestionService(t)
					backend = b
					mustCreateSession(t, b, "cleanup")
					var err error
					runID, err = initial.Run(ctx, "cleanup", "ask a color")
					if err != nil {
						t.Fatal(err)
					}
					waitForPendingQuestion(t, b, runID)
					restart = func() *Service { return restartQuestionServiceAt(t, b, "test-engine-next") }
					cancelType = domain.EventUserQuestionCancelled
				}
				s := restart()
				switch fault {
				case "row":
					s.deps.Approvals = cleanupFaultStore{backend}
					s.deps.Questions = cleanupFaultStore{backend}
				case "event":
					s.deps.Journal = cleanupFaultJournal{Journal: backend, kind: cancelType}
				case "event_ack":
					s.deps.Journal = cleanupFaultJournal{Journal: backend, kind: cancelType, ackLost: true}
				case "terminal_ack":
					s.deps.Journal = cleanupFaultJournal{Journal: backend, kind: domain.EventRunFailed, ackLost: true}
				case "status":
					s.deps.Runs = cleanupFaultRunStore{backend}
				case "terminal":
					s.deps.Journal = cleanupFaultJournal{Journal: backend, kind: domain.EventRunFailed}
				}
				if err := s.Recover(ctx); !errors.Is(err, errCleanupWrite) {
					t.Fatalf("recover error = %v, want cause preserved", err)
				}
				wantTerminals := 0
				if fault == "terminal_ack" || fault == "status" {
					wantTerminals = 1
				}
				if n := countTerminal(replayAll(t, backend, runID)); n != wantTerminals {
					t.Fatalf("terminal count after fault = %d, want %d", n, wantTerminals)
				}
				s = restart()
				for i := 0; i < 3; i++ {
					if err := s.Recover(ctx); err != nil {
						t.Fatal(err)
					}
				}
				assertEngineChangedFailure(t, backend, runID)
				events := replayAll(t, backend, runID)
				n := 0
				for _, e := range events {
					if e.Type == cancelType {
						n++
					}
				}
				if n != 1 {
					t.Fatalf("cancellation events = %d, want 1", n)
				}
				if a, err := backend.ListPendingApprovals(ctx); err != nil || len(a) != 0 {
					t.Fatalf("pending approvals = %v / %v", a, err)
				}
				if q, err := backend.ListPendingQuestions(ctx); err != nil || len(q) != 0 {
					t.Fatalf("pending questions = %v / %v", q, err)
				}
			})
		}
	}
}

func TestRecoverCleanupRetriesPlanAfterTerminal(t *testing.T) {
	backend, runID, sessionID, restart, reopen := newPendingPlanRecovery(t)
	ctx := context.Background()
	s := restart()
	s.deps.Work = cleanupFaultWork{backend}
	if err := s.Recover(ctx); !errors.Is(err, errCleanupWrite) {
		t.Fatalf("recover = %v", err)
	}
	assertEngineChangedFailure(t, backend, runID)
	state, err := backend.ReadWork(ctx, sessionID)
	if err != nil || state.Plan.ReviewStatus != domain.PlanReviewPending {
		t.Fatalf("Plan = %+v / %v", state.Plan, err)
	}
	// Close and reopen SQLite: only durable state survives this restart.
	backend = reopen()
	// No active runs remain: startup must still finish the durable intent.
	s = restart()
	for i := 0; i < 3; i++ {
		if err := s.Recover(ctx); err != nil {
			t.Fatal(err)
		}
	}
	state, err = backend.ReadWork(ctx, sessionID)
	if err != nil || state.Plan.ReviewStatus != domain.PlanReviewCancelled {
		t.Fatalf("Plan = %+v / %v", state.Plan, err)
	}
	if n := countWorkEvents(t, backend, sessionID, domain.WorkEventPlanReviewCancelled); n != 1 {
		t.Fatalf("Plan cancellation events = %d", n)
	}
	assertEngineChangedFailure(t, backend, runID)
}

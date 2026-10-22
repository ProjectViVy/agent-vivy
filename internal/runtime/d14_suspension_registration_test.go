package runtime

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/sqlite"
)

// Stop only the suspend owner's post-publication Err check. Unlike blocking
// the event sink, this leaves projectionMu free for the responding client.
type d14SuspensionGap struct {
	context.Context
	published              atomic.Bool
	reached, released      chan struct{}
	reachOnce, releaseOnce sync.Once
}

func (g *d14SuspensionGap) Err() error {
	if g.published.Load() {
		g.reachOnce.Do(func() { close(g.reached) })
		<-g.released
	}
	return g.Context.Err()
}

func (g *d14SuspensionGap) release() { g.releaseOnce.Do(func() { close(g.released) }) }

type d14SuspensionSink struct {
	inner *testSink
	gap   *d14SuspensionGap
}

func (s *d14SuspensionSink) Publish(event domain.RunEvent) {
	s.inner.Publish(event)
	if event.Type == domain.EventToolApprovalRequired || event.Type == domain.EventUserQuestionRequired {
		s.gap.published.Store(true)
	}
}

// Use the actual drive/typed Eino runner to create its native checkpoint. The
// fixture only supplies the same durable admission and process-owned maps as
// Run, so the post-release assertions exercise a real native continuation.
func d14NativeSuspensionGap(t *testing.T, question bool, expiration time.Duration) (*Service, *sqlite.Backend, *d14SuspensionGap, domain.RunID) {
	t.Helper()
	var svc *Service
	var backend *sqlite.Backend
	var sink *testSink
	if question {
		svc, backend, sink = newQuestionService(t)
	} else {
		svc, backend, sink = newApprovalService(t, 5*time.Minute)
	}
	svc.deps.ApprovalExpiration = expiration
	const sessionID domain.SessionID = "d14-suspension-session"
	const runID domain.RunID = "d14-suspension-run"
	const text = "complete the human interaction"
	mustCreateSession(t, backend, sessionID)
	policy, err := svc.engine.cfg.Policy.Snapshot(svc.defaultProfile)
	if err != nil {
		t.Fatal(err)
	}
	prompt, err := buildPromptSnapshot(PromptInput{Policy: policy, RunID: runID, GenerationID: "test-generation", Capture: promptCapture(nil), Face: domain.FaceWeb})
	if err != nil {
		t.Fatal(err)
	}
	mapper := newEventMapper(runID, 64<<10)
	mapper.setUsageRoutes("scripted", "scripted-v0", "")
	_, err = backend.CommitRunAdmission(context.Background(), storage.RunAdmission{
		Message: domain.Message{ID: "d14-suspension-message", SessionID: sessionID, RunID: runID, Role: domain.RoleUser, Content: text, CreatedAt: 1},
		Run:     domain.Run{ID: runID, SessionID: sessionID, Status: domain.RunActive, Kind: domain.RunKindPrimary, RootID: runID, CreatedAt: 1},
		Started: mapper.build(domain.EventRunStarted, payloadRunStarted{PromptSchema: prompt.SchemaVersion}), Prompt: &prompt,
	})
	if err != nil {
		t.Fatal(err)
	}
	ledger, err := NewBudgetLedger(svc.deps.Budget)
	if err != nil {
		t.Fatal(err)
	}
	if err := ledger.ReserveEvent(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	gap := &d14SuspensionGap{Context: ctx, reached: make(chan struct{}), released: make(chan struct{})}
	svc.deps.Sink = &d14SuspensionSink{inner: sink, gap: gap}
	selected := make(map[string]struct{})
	for _, name := range svc.engine.SelectTools().Names() {
		selected[name] = struct{}{}
	}
	svc.mu.Lock()
	svc.active[runID], svc.runSessions[runID] = cancel, sessionID
	svc.ledgers[runID], svc.snapshots[runID] = ledger, policy
	svc.runTools[runID], svc.workGates[runID] = selected, &sync.Mutex{}
	svc.mu.Unlock()
	t.Cleanup(func() {
		gap.release()
		svc.CancelAll()
		ctx, stop := context.WithTimeout(context.Background(), 10*time.Second)
		defer stop()
		if !svc.WaitIdle(ctx) {
			t.Error("native suspension did not drain")
		}
		cancel()
	})
	svc.wg.Add(1)
	go func() {
		defer svc.wg.Done()
		svc.drive(gap, mapper, sessionID, text, domain.RunModeNormal, svc.defaultProfile, domain.CollaborationModeNone, policy, domain.SandboxModeWorkspaceWrite, domain.ApprovalPolicyAsk, domain.FaceWeb, "", nil)
	}()
	select {
	case <-gap.reached:
	case <-time.After(5 * time.Second):
		t.Fatal("native drive did not reach the post-publication registration gap")
	}
	if !svc.projectionMu.TryLock() {
		t.Fatal("fixture blocked publication instead of the post-publication boundary")
	}
	svc.projectionMu.Unlock()
	svc.mu.Lock()
	_, pending := svc.pending[runID]
	_, active := svc.active[runID]
	svc.mu.Unlock()
	if pending || !active {
		t.Fatalf("gap owner state: pending=%v active=%v", pending, active)
	}
	return svc, backend, gap, runID
}

func TestD14NativeResponseWaitsForSuspensionRegistration(t *testing.T) {
	for _, response := range []string{domain.ApprovalApproved, domain.ApprovalDenied, "answer"} {
		t.Run(response, func(t *testing.T) {
			question := response == "answer"
			svc, backend, gap, runID := d14NativeSuspensionGap(t, question, 5*time.Minute)
			var respond func() error
			if question {
				q := waitForPendingQuestion(t, backend, runID)
				respond = func() error { return svc.AnswerQuestion(context.Background(), q.ID, "blue") }
			} else {
				a := waitForPendingApproval(t, backend, runID)
				if !svc.ApprovalRequiredDurable(context.Background(), runID, a.ID) {
					t.Fatal("approval was not durable")
				}
				respond = func() error { return svc.DecideApproval(context.Background(), a.ID, response) }
			}
			done := make(chan error, 1)
			go func() { done <- respond() }()
			select {
			case err := <-done:
				t.Fatalf("response returned before native suspension registration: %v", err)
			case <-time.After(100 * time.Millisecond):
			}
			gap.release()
			select {
			case err := <-done:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("response did not settle after registration")
			}
			waitForRunStatus(t, backend, runID, domain.RunCompleted)
			if err := respond(); !errors.Is(err, ErrApprovalAlreadyDecided) && !(question && errors.Is(err, ErrQuestionAlreadyAnswered)) {
				t.Fatalf("duplicate response = %v", err)
			}
			events := replayAll(t, backend, runID)
			counts := make(map[domain.EventType]int)
			for _, event := range events {
				counts[event.Type]++
			}
			if countTerminal(events) != 1 || counts[domain.EventModelRequest] != 2 || counts[domain.EventToolRequested] != 1 || counts[domain.EventToolFinished] != 1 {
				t.Fatalf("native continuation duplicated or lost work: %+v", counts)
			}
			notes, err := backend.ListNotes(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			wantNotes := 0
			if response == domain.ApprovalApproved {
				wantNotes = 1
			}
			if len(notes) != wantNotes {
				t.Fatalf("notes=%d, want %d", len(notes), wantNotes)
			}
		})
	}
}

func TestD14NativeResponseWithoutRegistrationKeepsDecisionSlot(t *testing.T) {
	for _, question := range []bool{false, true} {
		t.Run(map[bool]string{false: "approval", true: "question"}[question], func(t *testing.T) {
			svc, backend, gap, runID := d14NativeSuspensionGap(t, question, 5*time.Minute)
			if question {
				q := waitForPendingQuestion(t, backend, runID)
				if err := svc.AnswerQuestion(context.Background(), q.ID, "blue"); err == nil {
					t.Fatal("unregistered native question accepted an irreversible answer")
				}
				got, err := backend.GetQuestion(context.Background(), q.ID)
				if err != nil || got.Status != domain.QuestionPending {
					t.Fatalf("question slot was consumed: %+v err=%v", got, err)
				}
			} else {
				a := waitForPendingApproval(t, backend, runID)
				if err := svc.DecideApproval(context.Background(), a.ID, domain.ApprovalDenied); err == nil {
					t.Fatal("unregistered native approval accepted an irreversible decision")
				}
				got, err := backend.GetApproval(context.Background(), a.ID)
				if err != nil || got.Decision != domain.ApprovalPending {
					t.Fatalf("approval slot was consumed: %+v err=%v", got, err)
				}
			}
			gap.release()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if !svc.WaitIdle(ctx) {
				t.Fatal("native suspend owner did not finish registration")
			}
			if question {
				q := waitForPendingQuestion(t, backend, runID)
				if err := svc.AnswerQuestion(ctx, q.ID, "blue"); err != nil {
					t.Fatalf("pending answer slot was not reusable: %v", err)
				}
			} else {
				a := waitForPendingApproval(t, backend, runID)
				if err := svc.DecideApproval(ctx, a.ID, domain.ApprovalDenied); err != nil {
					t.Fatalf("pending decision slot was not reusable: %v", err)
				}
			}
			waitForRunStatus(t, backend, runID, domain.RunCompleted)
		})
	}
}

func TestD14CancelDuringNativeRegistrationCannotSettleResponse(t *testing.T) {
	for _, question := range []bool{false, true} {
		t.Run(map[bool]string{false: "approval", true: "question"}[question], func(t *testing.T) {
			svc, backend, gap, runID := d14NativeSuspensionGap(t, question, 5*time.Minute)
			response := make(chan error, 1)
			if question {
				q := waitForPendingQuestion(t, backend, runID)
				go func() { response <- svc.AnswerQuestion(context.Background(), q.ID, "blue") }()
			} else {
				a := waitForPendingApproval(t, backend, runID)
				go func() { response <- svc.DecideApproval(context.Background(), a.ID, domain.ApprovalApproved) }()
			}
			select {
			case err := <-response:
				t.Fatalf("response settled before registration: %v", err)
			case <-time.After(100 * time.Millisecond):
			}
			cancelled := make(chan bool, 1)
			go func() { cancelled <- svc.Cancel(runID) }()
			select {
			case <-gap.Done():
			case <-time.After(5 * time.Second):
				t.Fatal("Cancel did not reach the active native run")
			}
			gap.release()
			select {
			case ok := <-cancelled:
				if !ok {
					t.Fatal("active cancellation was rejected")
				}
			case <-time.After(5 * time.Second):
				t.Fatal("Cancel did not finish")
			}
			select {
			case err := <-response:
				if err == nil {
					t.Fatal("cancelled suspension accepted a response")
				}
			case <-time.After(5 * time.Second):
				t.Fatal("response readiness check did not finish")
			}
			waitForRunStatus(t, backend, runID, domain.RunCancelled)
			events := replayAll(t, backend, runID)
			requests := 0
			for _, event := range events {
				if event.Type == domain.EventToolApprovalDecided || event.Type == domain.EventUserQuestionAnswered {
					t.Fatalf("cancellation consumed the response slot: %s", event.Type)
				}
				if event.Type == domain.EventModelRequest {
					requests++
				}
			}
			if countTerminal(events) != 1 || requests != 1 {
				t.Fatalf("cancelled continuation ran again: terminals=%d model_calls=%d", countTerminal(events), requests)
			}
			notes, err := backend.ListNotes(context.Background())
			if err != nil || len(notes) != 0 {
				t.Fatalf("cancelled response performed an effect: notes=%d err=%v", len(notes), err)
			}
		})
	}
}

func TestD14NativeResponseExpiringDuringRegistrationKeepsDecisionSlot(t *testing.T) {
	for _, question := range []bool{false, true} {
		t.Run(map[bool]string{false: "approval", true: "question"}[question], func(t *testing.T) {
			svc, backend, gap, runID := d14NativeSuspensionGap(t, question, 500*time.Millisecond)
			response := make(chan error, 1)
			var expiresAt int64
			var id string
			wantErr := ErrApprovalExpired
			if question {
				q := waitForPendingQuestion(t, backend, runID)
				id, expiresAt = q.ID, q.ExpiresAt
				wantErr = ErrQuestionExpired
				go func() { response <- svc.AnswerQuestion(context.Background(), q.ID, "blue") }()
			} else {
				a := waitForPendingApproval(t, backend, runID)
				id, expiresAt = a.ID, a.ExpiresAt
				go func() { response <- svc.DecideApproval(context.Background(), a.ID, domain.ApprovalApproved) }()
			}
			if time.Now().UnixMilli() >= expiresAt {
				t.Fatal("fixture did not start the response before its deadline")
			}
			deadline := time.NewTimer(time.Until(time.UnixMilli(expiresAt)) + 25*time.Millisecond)
			defer deadline.Stop()
			select {
			case err := <-response:
				t.Fatalf("response returned before registration: %v", err)
			case <-deadline.C:
			}
			gap.release()
			select {
			case err := <-response:
				if !errors.Is(err, wantErr) {
					t.Fatalf("response after deadline = %v, want %v", err, wantErr)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("expired response did not return")
			}
			if question {
				q, err := backend.GetQuestion(context.Background(), id)
				if err != nil || q.Status != domain.QuestionPending {
					t.Fatalf("expired answer consumed its slot: %+v err=%v", q, err)
				}
			} else {
				a, err := backend.GetApproval(context.Background(), id)
				if err != nil || a.Decision != domain.ApprovalPending {
					t.Fatalf("expired decision consumed its slot: %+v err=%v", a, err)
				}
			}
		})
	}
}

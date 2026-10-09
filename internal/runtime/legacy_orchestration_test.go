package runtime

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/internal/testsupport"
	"agent-vivy/internal/tools"
)

func TestLegacyOrchestrationApprovalRejectedBeforeDecision(t *testing.T) {
	ctx := context.Background()
	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "legacy-approval.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = backend.Close() })
	service := newLegacyOrchestrationTestService(t, backend)
	approval := createLegacyOrchestrationApproval(t, backend, service, "legacy-approval-live", "legacy-run-live")

	err = service.DecideApprovalAsActor(ctx, approval.ID, domain.ApprovalApproved, "", "channel:test")
	if !errors.Is(err, ErrLegacyOrchestrationResumeUnsupported) {
		t.Fatalf("decision error = %v, want ErrLegacyOrchestrationResumeUnsupported", err)
	}
	assertLegacyApprovalStillPending(t, backend, approval.ID)
}

func TestLegacyOrchestrationApprovalAfterRestartRejected(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "legacy-restart.db")
	backend, err := sqlite.Open(ctx, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = backend.Close() })
	first := newLegacyOrchestrationTestService(t, backend)
	approval := createLegacyOrchestrationApproval(t, backend, first, "legacy-approval-restart", "legacy-run-restart")
	if err := backend.Close(); err != nil {
		t.Fatal(err)
	}
	backend, err = sqlite.Open(ctx, dbPath)
	if err != nil {
		t.Fatal(err)
	}

	// The second Service has no in-memory suspension; the durable historical
	// row must still fail closed without consuming its first-writer decision.
	restarted := newLegacyOrchestrationTestService(t, backend)
	err = restarted.DecideApprovalAsActor(ctx, approval.ID, domain.ApprovalApproved, "", "channel:test")
	if !errors.Is(err, ErrLegacyOrchestrationResumeUnsupported) {
		t.Fatalf("post-restart decision error = %v, want ErrLegacyOrchestrationResumeUnsupported", err)
	}
	assertLegacyApprovalStillPending(t, backend, approval.ID)

	systemApproval := createLegacyOrchestrationApproval(t, backend, restarted, "legacy-approval-system", "legacy-run-system", time.Now().Add(-time.Hour).UnixMilli())
	restarted.SetApprovalSettleTimeout(time.Second)
	if err := restarted.settleApprovalAsSystem(ctx, systemApproval); !errors.Is(err, ErrLegacyOrchestrationResumeUnsupported) {
		t.Fatalf("timed system settlement error = %v, want ErrLegacyOrchestrationResumeUnsupported", err)
	}
	assertLegacyApprovalStillPending(t, backend, systemApproval.ID)
}

func TestLegacyOrchestrationHistoryReadable(t *testing.T) {
	ctx := context.Background()
	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "legacy-history.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = backend.Close() })
	const runID = domain.RunID("legacy-history-run")
	if err := backend.CreateSession(ctx, domain.Session{ID: "legacy-history-session", Title: "legacy", CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	if err := backend.CreateRun(ctx, domain.Run{ID: runID, SessionID: "legacy-history-session", Status: domain.RunAccepted, CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	descriptor := []byte(`{"schema_version":1,"nodes":[{"key":"a","task":"alpha"},{"key":"b","task":"beta"},{"key":"join"},{"key":"audit"}]}`)
	if err := backend.AppendMessage(ctx, domain.Message{ID: "legacy-history-message", SessionID: "legacy-history-session", RunID: runID, Role: domain.RoleUser, CreatedAt: 2, Content: string(descriptor), Source: "workflow"}); err != nil {
		t.Fatal(err)
	}
	messages, err := backend.ListMessages(ctx, "legacy-history-session")
	if err != nil || len(messages) != 1 || messages[0].Content != string(descriptor) {
		t.Fatalf("legacy descriptor read = %+v err=%v", messages, err)
	}
	store, err := NewVersionedCheckpointStore(backend.Blobs(), "v0.9.13")
	if err != nil {
		t.Fatal(err)
	}
	checkpoint := []byte("opaque historical checkpoint bytes")
	if err := store.Set(ctx, checkpointIDFor(runID), checkpoint); err != nil {
		t.Fatal(err)
	}
	got, ok, err := store.Get(ctx, checkpointIDFor(runID))
	if err != nil || !ok || string(got) != string(checkpoint) {
		t.Fatalf("historical checkpoint read = %q present=%v err=%v", got, ok, err)
	}
}

func TestLegacyOrchestrationDoesNotBlockCurrentINOFY(t *testing.T) {
	ctx := context.Background()
	svc, backend := inofyExecService(t, &scriptedModel{replies: map[string]string{}})
	authoredSession := domain.SessionID("legacy-current-authored-session")
	authoredParent := domain.RunID("legacy-current-authored-parent")
	prepareChildSessionAuthorizer(t, svc, backend, authoredSession, authoredParent, []string{tools.EchoInfoName})
	authored, err := svc.StartINOFYWorkflow(ctx, authoredParent, "current-authored-operation", []byte(inofyTwoNodeDefinition))
	if err != nil {
		t.Fatalf("current authored workflow admission: %v", err)
	}
	waitForRunStatus(t, backend, authored.Run.ID, domain.RunCompleted)

	trustedSession := domain.SessionID("legacy-current-trusted-session")
	trustedParent := domain.RunID("legacy-current-trusted-parent")
	prepareChildSessionAuthorizer(t, svc, backend, trustedSession, trustedParent, nil)
	svc.deps.Cognitive = &CognitiveBinding{Domain: &fakeCognitiveDomain{}, Binding: cognitiveFixtureRunBinding()}
	trusted, err := svc.StartCognitiveWorkflow(ctx, trustedParent, "current-trusted-operation", TrustedStrategyDIVA, cognitiveInput(t))
	if err != nil {
		t.Fatalf("current trusted strategy admission: %v", err)
	}
	waitForRunStatus(t, backend, trusted.Run.ID, domain.RunCompleted)

	if !isNativeOrchestrationResumeTarget(nativeOrchestrationResumeTargetPrefix + "legacy") {
		t.Fatal("legacy marker helper does not recognize the historical prefix")
	}
	if isNativeOrchestrationResumeTarget("workflow:current") {
		t.Fatal("current workflow target was classified as a legacy orchestration")
	}
}

func newLegacyOrchestrationTestService(t *testing.T, backend *sqlite.Backend) *Service {
	t.Helper()
	ctx := context.Background()
	toolset, err := tools.Builtin(backend).Resolve([]string{tools.EchoInfoName})
	if err != nil {
		t.Fatal(err)
	}
	engine, err := NewEngine(ctx, WrapModel(testsupport.NewEchoModel()), toolset, EngineConfig{StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10})
	if err != nil {
		t.Fatal(err)
	}
	return NewService(engine, "legacy-test", "legacy-test-model", ServiceDeps{
		Journal: backend, Runs: backend, Messages: backend, Sessions: backend,
		Approvals: backend, Sink: newTestSink(),
	})
}

func createLegacyOrchestrationApproval(t *testing.T, backend *sqlite.Backend, service *Service, approvalID string, runID domain.RunID, expiresAt ...int64) domain.Approval {
	t.Helper()
	ctx := context.Background()
	sessionID := domain.SessionID("legacy-approval-session-" + approvalID)
	if err := backend.CreateSession(ctx, domain.Session{ID: sessionID, Title: "legacy", CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	if err := backend.CreateRun(ctx, domain.Run{ID: runID, SessionID: sessionID, Status: domain.RunActive, CreatedAt: 2}); err != nil {
		t.Fatal(err)
	}
	expires := time.Now().Add(time.Hour).UnixMilli()
	if len(expiresAt) > 0 {
		expires = expiresAt[0]
	}
	approval := domain.Approval{
		ID: approvalID, RunID: runID, ToolCallID: "legacy-node-a", ToolName: "orchestration.node",
		Decision: domain.ApprovalPending, ExpiresAt: expires,
		ResumeTarget: nativeOrchestrationResumeTargetPrefix + "runnable:vivy;node:a",
	}
	if err := backend.CreateApproval(ctx, approval); err != nil {
		t.Fatal(err)
	}
	if !service.journalReviewEvent(ctx, runID, domain.EventToolApprovalRequired, payloadToolApprovalRequired{ApprovalID: approval.ID}) {
		t.Fatal("failed to persist legacy approval visibility event")
	}
	return approval
}

func assertLegacyApprovalStillPending(t *testing.T, backend *sqlite.Backend, approvalID string) {
	t.Helper()
	got, err := backend.GetApproval(context.Background(), approvalID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Decision != domain.ApprovalPending || got.DecidedAt != 0 || got.Actor != "" {
		t.Fatalf("legacy approval was consumed: %+v", got)
	}
	if _, err := backend.GetRun(context.Background(), got.RunID); err != nil {
		t.Fatalf("legacy run history became unreadable: %v", err)
	}
	for _, event := range replayEvents(t, backend, got.RunID) {
		if event.Type != domain.EventToolApprovalRequired {
			t.Fatalf("legacy approval produced post-retirement work event %q", event.Type)
		}
	}
}

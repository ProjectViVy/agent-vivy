package runtime

import (
	"context"
	"testing"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/testsupport"
)

// Preserve the WIP branch's explicit-child outcome coverage alongside the
// snapshot's legacy-child and append-only session-view regression tests.
func TestMessageProjectorOneShotChildPreservesJournalOutcome(t *testing.T) {
	svc, backend, _ := newTestService(t, testsupport.NewEchoModel())
	ctx := context.Background()
	sessionID := domain.SessionID("sess-projector-oneshot-child")
	parentRunID := domain.RunID("run-projector-oneshot-parent")
	childRunID := domain.RunID("child-projector-oneshot")
	if err := backend.CreateSession(ctx, domain.Session{ID: sessionID, Title: "one-shot child", CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	if err := backend.CreateRun(ctx, domain.Run{ID: parentRunID, SessionID: sessionID, Status: domain.RunCompleted, Kind: domain.RunKindPrimary, CreatedAt: 2}); err != nil {
		t.Fatal(err)
	}
	if err := backend.CreateRun(ctx, domain.Run{
		ID: childRunID, SessionID: sessionID, Status: domain.RunCompleted, CreatedAt: 3,
		Kind: domain.RunKindChild, ChildMode: domain.ChildModeOneShot, ParentID: parentRunID,
	}); err != nil {
		t.Fatal(err)
	}
	trajCommit(t, ctx, svc.deps.Journal, parentRunID,
		trajEvent(domain.EventModelRequest, 10, payloadModelRequest{}),
		trajEvent(domain.EventModelDelta, 20, payloadModelDelta{Delta: "parent answer"}),
		trajEventVersion(domain.EventModelCompleted, 30, v2Completion("parent answer"), 2),
	)
	trajCommit(t, ctx, svc.deps.Journal, childRunID,
		trajEvent(domain.EventChildStarted, 40, map[string]string{"workspace_id": "ws-child"}),
		trajEvent(domain.EventModelRequest, 41, payloadModelRequest{}),
		trajEvent(domain.EventModelDelta, 42, payloadModelDelta{Delta: "child answer"}),
		trajEventVersion(domain.EventModelCompleted, 43, v2Completion("child answer"), 2),
		trajEvent(domain.EventChildCompleted, 44, map[string]string{"summary": "child summary"}),
	)
	for attempt := 0; attempt < 2; attempt++ {
		if err := svc.ReconcileSessionMessages(ctx, sessionID); err != nil {
			t.Fatal(err)
		}
	}
	got, err := backend.ListMessages(ctx, sessionID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].RunID != parentRunID || got[0].Content != "parent answer" {
		t.Fatalf("session projection = %+v, want only the parent run", got)
	}
	summary, message, _, err := svc.ChildRunDetails(ctx, childRunID)
	if err != nil || summary != "child summary" || message != "" {
		t.Fatalf("child journal = %q/%q %v, want the child outcome intact", summary, message, err)
	}
}

func TestMessageProjectorKeepsContinuableChildSessionHistory(t *testing.T) {
	svc, backend, _ := newTestService(t, testsupport.NewEchoModel())
	ctx := context.Background()
	childSessionID := domain.SessionID("sess-projector-continuable-child")
	childRunID := domain.RunID("run-projector-continuable-child")
	if err := backend.CreateSession(ctx, domain.Session{ID: childSessionID, Title: "continuable child", CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	if err := backend.CreateRun(ctx, domain.Run{
		ID: childRunID, SessionID: childSessionID, Status: domain.RunCompleted, CreatedAt: 2,
		Kind: domain.RunKindChild, ChildMode: domain.ChildModeContinuable,
	}); err != nil {
		t.Fatal(err)
	}
	trajCommit(t, ctx, svc.deps.Journal, childRunID,
		trajEvent(domain.EventModelRequest, 10, payloadModelRequest{}),
		trajEvent(domain.EventModelDelta, 20, payloadModelDelta{Delta: "continuable answer"}),
		trajEventVersion(domain.EventModelCompleted, 30, v2Completion("continuable answer"), 2),
	)
	if err := svc.ReconcileSessionMessages(ctx, childSessionID); err != nil {
		t.Fatal(err)
	}
	got, err := backend.ListMessages(ctx, childSessionID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].RunID != childRunID || got[0].Content != "continuable answer" {
		t.Fatalf("child session projection = %+v, want the continuable child transcript", got)
	}
}

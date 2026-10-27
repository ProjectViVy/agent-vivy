package runtime

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"agent-vivy/internal/domain"

	"agent-vivy/internal/testsupport"
)

func projectorRun(t *testing.T, svc *Service, sessionID domain.SessionID, runID domain.RunID, content string, completed payloadModelCompletedV3) {
	t.Helper()
	ctx := context.Background()
	if err := svc.deps.Sessions.CreateSession(ctx, domain.Session{ID: sessionID, Title: "projection", CreatedAt: 1}); err != nil {
		t.Fatalf("create session: %v", err)
	}
	if err := svc.deps.Runs.CreateRun(ctx, domain.Run{ID: runID, SessionID: sessionID, Status: domain.RunCompleted, CreatedAt: 1}); err != nil {
		t.Fatalf("create run: %v", err)
	}
	trajCommit(t, ctx, svc.deps.Journal, runID,
		trajEvent(domain.EventRunStarted, 10, payloadRunStarted{Provider: "test", Model: "test-model", Mode: "chat"}),
		trajEvent(domain.EventModelRequest, 20, payloadModelRequest{}),
		trajEvent(domain.EventModelDelta, 30, payloadModelDelta{Delta: content}),
		trajEventVersion(domain.EventModelCompleted, 40, completed, 2),
	)
}

func v2Completion(content string) payloadModelCompletedV3 {
	return payloadModelCompletedV3{ByteLen: len([]byte(content))}
}

func TestMessageProjectorReplaysAndRepairsMissingProjection(t *testing.T) {
	svc, backend, _ := newTestService(t, testsupport.NewEchoModel())
	ctx := context.Background()
	sessionID := domain.SessionID("sess-projector-repair")
	runID := domain.RunID("run-projector-repair")
	content := "durable answer"
	projectorRun(t, svc, sessionID, runID, content, v2Completion(content))

	before, err := backend.ListMessages(ctx, sessionID)
	if err != nil {
		t.Fatalf("list before repair: %v", err)
	}
	if len(before) != 0 {
		t.Fatalf("projection unexpectedly populated before repair: %+v", before)
	}
	if err := svc.ReconcileSessionMessages(ctx, sessionID); err != nil {
		t.Fatalf("reconcile missing projection: %v", err)
	}
	after, err := backend.ListMessages(ctx, sessionID)
	if err != nil {
		t.Fatalf("list after repair: %v", err)
	}
	if len(after) != 1 || after[0].Role != domain.RoleAssistant || after[0].Content != content || after[0].RunID != runID {
		t.Fatalf("repaired projection = %+v", after)
	}
}

func TestMessageProjectorReplayIsIdempotent(t *testing.T) {
	svc, backend, _ := newTestService(t, testsupport.NewEchoModel())
	ctx := context.Background()
	sessionID := domain.SessionID("sess-projector-idempotent")
	runID := domain.RunID("run-projector-idempotent")
	content := "same answer"
	projectorRun(t, svc, sessionID, runID, content, v2Completion(content))

	for i := 0; i < 2; i++ {
		if err := svc.ReconcileSessionMessages(ctx, sessionID); err != nil {
			t.Fatalf("reconcile %d: %v", i+1, err)
		}
	}
	got, err := backend.ListMessages(ctx, sessionID)
	if err != nil {
		t.Fatalf("list projected messages: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("idempotent replay produced %d rows: %+v", len(got), got)
	}
	if !strings.HasPrefix(got[0].ID, "msgp_") {
		t.Fatalf("projection id = %q, want deterministic msgp id", got[0].ID)
	}
}

func TestMessageProjectorAdoptsLegacyAssistantRow(t *testing.T) {
	svc, backend, _ := newTestService(t, testsupport.NewEchoModel())
	ctx := context.Background()
	sessionID := domain.SessionID("sess-projector-legacy")
	runID := domain.RunID("run-projector-legacy")
	content := "legacy answer"
	projectorRun(t, svc, sessionID, runID, content, v2Completion(content))
	legacy := domain.Message{
		ID: "legacy-random-id", SessionID: sessionID, RunID: runID,
		Role: domain.RoleAssistant, CreatedAt: 999, Content: content,
	}
	if err := backend.AppendMessage(ctx, legacy); err != nil {
		t.Fatalf("append legacy projection: %v", err)
	}

	if err := svc.ReconcileSessionMessages(ctx, sessionID); err != nil {
		t.Fatalf("adopt legacy projection: %v", err)
	}
	got, err := backend.ListMessages(ctx, sessionID)
	if err != nil {
		t.Fatalf("list adopted projection: %v", err)
	}
	if len(got) != 1 || got[0].ID != legacy.ID || got[0].Content != content {
		t.Fatalf("legacy adoption produced %+v", got)
	}
}

func TestMessageProjectorRejectsByteMismatch(t *testing.T) {
	svc, backend, _ := newTestService(t, testsupport.NewEchoModel())
	projectorRun(t, svc, "s-count", "r-count", "actual", payloadModelCompletedV3{ByteLen: 1})
	if err := svc.ReconcileSessionMessages(context.Background(), "s-count"); err == nil {
		t.Fatal("accepted wrong byte count")
	}
	got, err := backend.ListMessages(context.Background(), "s-count")
	if err != nil || len(got) != 0 {
		t.Fatalf("partial projection: %+v %v", got, err)
	}
}

func TestMessageProjectorRejectsNonCanonicalV2Metadata(t *testing.T) {
	for _, tc := range []struct {
		name    string
		payload map[string]any
	}{
		{name: "null byte count", payload: map[string]any{"byte_len": nil}},
		{name: "unknown field", payload: map[string]any{"content_sha256": "ignored historical digest", "byte_len": len("answer"), "extra": true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, backend, _ := newTestService(t, testsupport.NewEchoModel())
			ctx := context.Background()
			sessionID := domain.SessionID("sess-projector-strict-" + strings.ReplaceAll(tc.name, " ", "-"))
			runID := domain.RunID("run-projector-strict-" + strings.ReplaceAll(tc.name, " ", "-"))
			if err := backend.CreateSession(ctx, domain.Session{ID: sessionID, Title: "strict", CreatedAt: 1}); err != nil {
				t.Fatal(err)
			}
			if err := backend.CreateRun(ctx, domain.Run{ID: runID, SessionID: sessionID, Status: domain.RunCompleted, CreatedAt: 1}); err != nil {
				t.Fatal(err)
			}
			payload, err := json.Marshal(tc.payload)
			if err != nil {
				t.Fatal(err)
			}
			trajCommit(t, ctx, backend, runID,
				trajEvent(domain.EventModelDelta, 10, payloadModelDelta{Delta: "answer"}),
				domain.RunEvent{RunID: runID, Type: domain.EventModelCompleted, PayloadVersion: 2, Payload: payload, CreatedAt: 20},
			)
			if err := svc.ReconcileSessionMessages(ctx, sessionID); err == nil {
				t.Fatal("non-canonical v2 metadata was accepted")
			}
		})
	}
}

func TestMessageProjectorPreservesToolPreambleAndFinalOrder(t *testing.T) {
	svc, backend, _ := newTestService(t, testsupport.NewEchoModel())
	ctx := context.Background()
	sessionID := domain.SessionID("sess-projector-tools")
	runID := domain.RunID("run-projector-tools")
	if err := svc.deps.Sessions.CreateSession(ctx, domain.Session{ID: sessionID, Title: "tools", CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	if err := svc.deps.Runs.CreateRun(ctx, domain.Run{ID: runID, SessionID: sessionID, Status: domain.RunCompleted, CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	trajCommit(t, ctx, svc.deps.Journal, runID,
		trajEvent(domain.EventModelRequest, 10, payloadModelRequest{}),
		trajEvent(domain.EventModelDelta, 20, payloadModelDelta{Delta: "before tool"}),
		trajEvent(domain.EventToolRequested, 30, payloadToolRequested{ToolCallID: "call-1", ToolName: "echo_info", Args: map[string]any{"text": "hi"}}),
		trajEvent(domain.EventToolFinished, 40, payloadToolFinished{ToolCallID: "call-1", ToolName: "echo_info", Result: "tool result"}),
		trajEvent(domain.EventModelDelta, 50, payloadModelDelta{Delta: "final answer"}),
		trajEventVersion(domain.EventModelCompleted, 60, v2Completion("final answer"), 2),
	)
	for i := 0; i < 2; i++ {
		if err := svc.ReconcileSessionMessages(ctx, sessionID); err != nil {
			t.Fatalf("reconcile %d: %v", i, err)
		}
	}
	got, err := backend.ListMessages(ctx, sessionID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 4 {
		t.Fatalf("projected rows = %+v, want four", got)
	}
	if got[0].Role != domain.RoleAssistant || got[0].Content != "before tool" ||
		got[1].Role != domain.RoleAssistant || got[1].ToolCallID != "call-1" || got[1].ToolName != "echo_info" ||
		got[2].Role != domain.RoleTool || got[2].Content != "tool result" || got[2].ToolCallID != "call-1" ||
		got[3].Role != domain.RoleAssistant || got[3].Content != "final answer" {
		t.Fatalf("projected order/content = %+v", got)
	}
}

func TestMessageProjectorToleratesLegacyV1Run(t *testing.T) {
	svc, backend, _ := newTestService(t, testsupport.NewEchoModel())
	ctx := context.Background()
	sessionID := domain.SessionID("sess-projector-v1")
	runID := domain.RunID("run-projector-v1")
	if err := svc.deps.Sessions.CreateSession(ctx, domain.Session{ID: sessionID, Title: "v1", CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	if err := svc.deps.Runs.CreateRun(ctx, domain.Run{ID: runID, SessionID: sessionID, Status: domain.RunCompleted, CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	trajCommit(t, ctx, svc.deps.Journal, runID,
		trajEvent(domain.EventModelRequest, 10, payloadModelRequest{}),
		trajEventVersion(domain.EventModelCompleted, 20, map[string]string{"content": "legacy completion"}, 1),
	)
	// A projection persisted by the pre-v2 writer stays readable: reconcile
	// skips the legacy run instead of failing the whole session.
	persisted := domain.Message{
		ID: projectedMessageID(runID, 20, "0"), SessionID: sessionID, RunID: runID,
		Role: domain.RoleAssistant, CreatedAt: 1, Content: "old projected text",
	}
	if err := backend.AppendMessage(ctx, persisted); err != nil {
		t.Fatal(err)
	}
	if err := svc.ReconcileSessionMessages(ctx, sessionID); err != nil {
		t.Fatalf("legacy v1 run made the session unreadable: %v", err)
	}
	got, err := backend.ListMessages(ctx, sessionID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Content != "old projected text" {
		t.Fatalf("legacy run projection = %+v, want the persisted row untouched", got)
	}
}

func TestMessageProjectorSkipsOneShotChildRuns(t *testing.T) {
	svc, backend, _ := newTestService(t, testsupport.NewEchoModel())
	ctx := context.Background()
	sessionID := domain.SessionID("sess-projector-child")
	if err := svc.deps.Sessions.CreateSession(ctx, domain.Session{ID: sessionID, Title: "child", CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	parentID := domain.RunID("run-projector-parent")
	childID := domain.RunID("run-projector-child")
	for _, run := range []domain.Run{
		{ID: parentID, SessionID: sessionID, Status: domain.RunCompleted, CreatedAt: 1, Kind: domain.RunKindPrimary},
		{ID: childID, SessionID: sessionID, Status: domain.RunCompleted, CreatedAt: 2, Kind: domain.RunKindChild,
			ParentID: parentID, RootID: parentID, Depth: 1},
	} {
		if err := svc.deps.Runs.CreateRun(ctx, run); err != nil {
			t.Fatalf("create run %s: %v", run.ID, err)
		}
	}
	for _, spec := range []struct {
		runID   domain.RunID
		content string
	}{{parentID, "parent answer"}, {childID, "child answer"}} {
		trajCommit(t, ctx, svc.deps.Journal, spec.runID,
			trajEvent(domain.EventRunStarted, 10, payloadRunStarted{Provider: "test", Model: "test-model", Mode: "chat"}),
			trajEvent(domain.EventModelRequest, 20, payloadModelRequest{}),
			trajEvent(domain.EventModelDelta, 30, payloadModelDelta{Delta: spec.content}),
			trajEventVersion(domain.EventModelCompleted, 40, v2Completion(spec.content), 2),
		)
	}

	if err := svc.ReconcileSessionMessages(ctx, sessionID); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	got, err := backend.ListMessages(ctx, sessionID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].RunID != parentID {
		t.Fatalf("projected rows = %+v, want only the parent transcript", got)
	}
}

func TestSessionViewMessagesDropsOnlyOneShotChildRows(t *testing.T) {
	svc, backend, _ := newTestService(t, testsupport.NewEchoModel())
	ctx := context.Background()
	sessionID := domain.SessionID("sess-view-child")
	if err := svc.deps.Sessions.CreateSession(ctx, domain.Session{ID: sessionID, Title: "view", CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	parentID := domain.RunID("run-view-parent")
	oneShotID := domain.RunID("run-view-oneshot")
	continuableID := domain.RunID("run-view-continuable")
	for _, run := range []domain.Run{
		{ID: parentID, SessionID: sessionID, Status: domain.RunCompleted, CreatedAt: 1, Kind: domain.RunKindPrimary},
		{ID: oneShotID, SessionID: sessionID, Status: domain.RunCompleted, CreatedAt: 2, Kind: domain.RunKindChild,
			ParentID: parentID, RootID: parentID, Depth: 1},
		{ID: continuableID, SessionID: sessionID, Status: domain.RunCompleted, CreatedAt: 3, Kind: domain.RunKindChild,
			ChildMode: domain.ChildModeContinuable, ParentID: parentID, RootID: parentID, Depth: 1},
	} {
		if err := svc.deps.Runs.CreateRun(ctx, run); err != nil {
			t.Fatalf("create run %s: %v", run.ID, err)
		}
	}
	rows := []domain.Message{
		{ID: "msg-view-parent", SessionID: sessionID, RunID: parentID, Role: domain.RoleAssistant, CreatedAt: 1, Content: "parent"},
		{ID: "msg-view-oneshot", SessionID: sessionID, RunID: oneShotID, Role: domain.RoleAssistant, CreatedAt: 2, Content: "one-shot child"},
		{ID: "msg-view-continuable", SessionID: sessionID, RunID: continuableID, Role: domain.RoleAssistant, CreatedAt: 3, Content: "continuable child"},
		{ID: "msg-view-legacy", SessionID: sessionID, Role: domain.RoleUser, CreatedAt: 4, Content: "legacy"},
	}
	for _, msg := range rows {
		if err := backend.AppendMessage(ctx, msg); err != nil {
			t.Fatalf("append message %s: %v", msg.ID, err)
		}
	}

	view, err := svc.SessionViewMessages(ctx, sessionID)
	if err != nil {
		t.Fatalf("session view: %v", err)
	}
	var ids []string
	for _, msg := range view {
		ids = append(ids, msg.ID)
	}
	want := "msg-view-parent,msg-view-continuable,msg-view-legacy"
	if strings.Join(ids, ",") != want {
		t.Fatalf("session view = %v, want %s", ids, want)
	}
	stored, err := backend.ListMessages(ctx, sessionID)
	if err != nil {
		t.Fatal(err)
	}
	if len(stored) != 4 {
		t.Fatalf("stored rows = %d, want the append-only store untouched", len(stored))
	}
}

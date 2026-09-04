package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/testsupport"
)

func projectorRun(t *testing.T, svc *Service, sessionID domain.SessionID, runID domain.RunID, content string, completed payloadModelCompletedV2) {
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

func v2Completion(content string) payloadModelCompletedV2 {
	return payloadModelCompletedV2{ContentSHA256: sha256Hex([]byte(content)), ByteLen: len([]byte(content))}
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

func TestMessageProjectorRejectsV2HashMismatch(t *testing.T) {
	svc, backend, _ := newTestService(t, testsupport.NewEchoModel())
	ctx := context.Background()
	sessionID := domain.SessionID("sess-projector-mismatch")
	runID := domain.RunID("run-projector-mismatch")
	projectorRun(t, svc, sessionID, runID, "actual answer", v2Completion("actual anSwer"))

	err := svc.ReconcileSessionMessages(ctx, sessionID)
	if err == nil || !strings.Contains(err.Error(), "digest mismatch") {
		t.Fatalf("hash mismatch reconcile error = %v, want digest mismatch", err)
	}
	if errors.Is(err, storage.ErrProjectionConflict) {
		t.Fatalf("hash mismatch must fail before projection conflict: %v", err)
	}
	got, listErr := backend.ListMessages(ctx, sessionID)
	if listErr != nil {
		t.Fatalf("list after mismatch: %v", listErr)
	}
	if len(got) != 0 {
		t.Fatalf("hash mismatch partially projected rows: %+v", got)
	}
}

func TestMessageProjectorRejectsNonCanonicalV2Metadata(t *testing.T) {
	for _, tc := range []struct {
		name    string
		payload map[string]any
	}{
		{name: "uppercase digest", payload: map[string]any{"content_sha256": strings.ToUpper(sha256Hex([]byte("answer"))), "byte_len": len("answer")}},
		{name: "unknown field", payload: map[string]any{"content_sha256": sha256Hex([]byte("answer")), "byte_len": len("answer"), "extra": true}},
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

func TestMessageProjectorReadsLegacyV1CompletionOnly(t *testing.T) {
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
		trajEvent(domain.EventModelCompleted, 20, payloadModelCompleted{Content: "legacy completion"}),
	)
	if err := svc.ReconcileSessionMessages(ctx, sessionID); err != nil {
		t.Fatal(err)
	}
	got, err := backend.ListMessages(ctx, sessionID)
	if err != nil || len(got) != 1 || got[0].Content != "legacy completion" {
		t.Fatalf("legacy projection = %+v, err=%v", got, err)
	}
}

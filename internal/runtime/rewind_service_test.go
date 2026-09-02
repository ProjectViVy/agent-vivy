package runtime

import (
	"context"
	"strings"
	"testing"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/testsupport"
)

func appendRewindFixture(t *testing.T, svc *Service, sessionID domain.SessionID) {
	t.Helper()
	ctx := context.Background()
	messages := []domain.Message{
		{ID: "msg-1", SessionID: sessionID, Role: domain.RoleUser, Content: "one"},
		{ID: "msg-2", SessionID: sessionID, Role: domain.RoleAssistant, Content: "two"},
		{ID: "msg-3", SessionID: sessionID, Role: domain.RoleUser, Content: "three"},
		{ID: "msg-4", SessionID: sessionID, Role: domain.RoleAssistant, Content: "four"},
	}
	for _, m := range messages {
		if err := svc.deps.Messages.AppendMessage(ctx, m); err != nil {
			t.Fatalf("AppendMessage %s: %v", m.ID, err)
		}
	}
}

func TestRewindSessionMarksAndFilters(t *testing.T) {
	svc, backend, sink := newTestService(t, testsupport.NewEchoModel())
	ctx := context.Background()
	appendRewindFixture(t, svc, "sess-rw")

	result, err := svc.RewindSession(ctx, "sess-rw", "msg-3")
	if err != nil {
		t.Fatalf("RewindSession: %v", err)
	}
	if result.CutoffMessageID != "msg-3" || result.RemainingCount != 2 {
		t.Fatalf("RewindSession = %+v, want cutoff msg-3 with 2 remaining", result)
	}
	marker, ok, err := backend.LatestSessionTruncation(ctx, "sess-rw")
	if err != nil || !ok {
		t.Fatalf("LatestSessionTruncation = ok=%v, %v; want true, nil", ok, err)
	}
	if marker.CutoffMessageID != "msg-3" || marker.Reason != storage.TruncationRewind {
		t.Fatalf("marker = %+v, want msg-3 rewind", marker)
	}
	folded := svc.effectiveSessionMessages(ctx, "sess-rw", mustListMessages(t, svc, "sess-rw"))
	if len(folded) != 2 || folded[0].ID != "msg-1" || folded[1].ID != "msg-2" {
		t.Fatalf("effective view = %+v, want msg-1..msg-2", folded)
	}
	truncated := 0
	for _, ev := range sink.snapshot() {
		if ev.Type == domain.EventSessionTruncated {
			truncated++
			if !strings.HasPrefix(string(ev.RunID), "tr_") {
				t.Fatalf("session.truncated run id = %s, want tr_ prefix", ev.RunID)
			}
			if !strings.Contains(string(ev.Payload), `"cutoff_message_id":"msg-3"`) {
				t.Fatalf("session.truncated payload = %s, want cutoff msg-3", ev.Payload)
			}
		}
	}
	if truncated != 1 {
		t.Fatalf("published session.truncated events = %d, want 1", truncated)
	}
}

func TestRewindSessionValidation(t *testing.T) {
	svc, backend, _ := newTestService(t, testsupport.NewEchoModel())
	ctx := context.Background()
	appendRewindFixture(t, svc, "sess-rw")

	if _, err := svc.RewindSession(ctx, "sess-rw", "msg-gone"); err != ErrInvalidCutoff {
		t.Fatalf("unknown cutoff = %v, want ErrInvalidCutoff", err)
	}
	if err := backend.CreateRun(ctx, domain.Run{ID: "run-busy", SessionID: "sess-rw", Status: domain.RunActive, CreatedAt: 1}); err != nil {
		t.Fatalf("CreateRun busy: %v", err)
	}
	if _, err := svc.RewindSession(ctx, "sess-rw", "msg-2"); err != ErrSessionBusy {
		t.Fatalf("busy session = %v, want ErrSessionBusy", err)
	}
	if err := backend.SetRunStatus(ctx, "run-busy", domain.RunCompleted); err != nil {
		t.Fatalf("SetRunStatus: %v", err)
	}
	// A queued run on another session must not block this one.
	if err := backend.CreateRun(ctx, domain.Run{ID: "run-other", SessionID: "sess-other", Status: domain.RunQueued, CreatedAt: 2}); err != nil {
		t.Fatalf("CreateRun other: %v", err)
	}
	if _, err := svc.RewindSession(ctx, "sess-rw", "msg-2"); err != nil {
		t.Fatalf("rewind with foreign active run = %v, want nil", err)
	}
}

func TestRewindSessionNotWired(t *testing.T) {
	svc, _, _ := newTestService(t, testsupport.NewEchoModel())
	svc.deps.Truncations = nil
	appendRewindFixture(t, svc, "sess-rw")
	if _, err := svc.RewindSession(context.Background(), "sess-rw", "msg-1"); err != ErrRewindNotWired {
		t.Fatalf("unwired rewind = %v, want ErrRewindNotWired", err)
	}
}

func mustListMessages(t *testing.T, svc *Service, sessionID domain.SessionID) []domain.Message {
	t.Helper()
	messages, err := svc.deps.Messages.ListMessages(context.Background(), sessionID)
	if err != nil {
		t.Fatalf("ListMessages: %v", err)
	}
	return messages
}

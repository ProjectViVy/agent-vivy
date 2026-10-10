package runtime

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"agent-vivy/internal/domain"
	"agent-vivy/sdk/port/observer"
)

// Removing the admitted user-source projection would lose a fact which the
// assistant never repeats. The source is read from real durable message rows.
func TestMemoryLoopCaptureAdmittedUserSource(t *testing.T) {
	for _, terminal := range []string{"run.completed", "run.failed", "run.cancelled"} {
		t.Run(terminal, func(t *testing.T) {
			ctx := context.Background()
			_, store, _ := newTestService(t, cognitiveTestModel())
			mustCreateSession(t, store, "capture-session")
			run := domain.Run{ID: "capture-run", SessionID: "capture-session", Kind: domain.RunKindPrimary, Status: domain.RunCompleted}
			if err := store.CreateRun(ctx, run); err != nil {
				t.Fatal(err)
			}
			nonce := make([]byte, 16)
			if _, err := rand.Read(nonce); err != nil {
				t.Fatal(err)
			}
			fact := "user-only-" + hex.EncodeToString(nonce)
			body := strings.Repeat("长中文事实", 2200) + fact
			for _, msg := range []domain.Message{
				{ID: "admitted-user", SessionID: run.SessionID, RunID: run.ID, Role: domain.RoleUser, Content: body, CreatedAt: 1},
				{ID: "other-user", SessionID: run.SessionID, RunID: "foreign-run", Role: domain.RoleUser, Content: "foreign-fact", CreatedAt: 2},
				{ID: "tool-output", SessionID: run.SessionID, RunID: run.ID, Role: domain.RoleTool, Content: "tool-must-not-become-user", CreatedAt: 3},
			} {
				if err := store.AppendMessage(ctx, msg); err != nil {
					t.Fatal(err)
				}
			}
			sink := &fakeSink{}
			p := NewCognitiveCaptureProvider(store, sink, nil)
			event := observer.NewRunEvent(observer.NewEventID(string(run.ID), 7), terminal, 10, json.RawMessage(`{"summary":"收到","session_id":"capture-session"}`))
			receipt, err := p.ObserveRunWithReceipt(ctx, event)
			if err != nil {
				t.Fatal(err)
			}
			if len(sink.captures) != 1 {
				t.Fatalf("captures=%d", len(sink.captures))
			}
			if sink.captures[0].UserContent != body {
				t.Fatal("activity source differs from actual admitted user content")
			}
			content := sink.captures[0].Content
			var source struct {
				Schema    string `json:"schema"`
				RunID     string `json:"run_id"`
				SessionID string `json:"session_id"`
				Messages  []struct {
					ID       string `json:"id"`
					Role     string `json:"role"`
					Content  string `json:"content"`
					Complete bool   `json:"complete"`
				} `json:"messages"`
			}
			if err := json.Unmarshal([]byte(content), &source); err != nil {
				t.Fatalf("source is not a role-bearing durable envelope: %s (%v)", content, err)
			}
			if source.Schema != "vivy.conversation-source/v1" || source.RunID != string(run.ID) || source.SessionID != string(run.SessionID) || len(source.Messages) == 0 {
				t.Fatalf("source identity missing: %+v", source)
			}
			user := source.Messages[0]
			if user.ID != "admitted-user" || user.Role != "user" || user.Content != body || !user.Complete {
				t.Fatalf("admitted user text/role not retained; role=%s bytes=%d complete=%t", user.Role, len(user.Content), user.Complete)
			}
			if strings.Contains(content, "foreign-fact") || strings.Contains(content, "tool-must-not-become-user") {
				t.Fatal("foreign run or tool text admitted as user evidence")
			}
			again, err := p.ObserveRunWithReceipt(ctx, event)
			if err != nil || again.ReceiptID != receipt.ReceiptID || len(sink.captures) != 1 {
				t.Fatalf("replay changed identity: %+v %v", again, err)
			}
		})
	}
}

func TestMemoryLoopCaptureRefusesMissingOrForeignSource(t *testing.T) {
	ctx := context.Background()
	_, store, _ := newTestService(t, cognitiveTestModel())
	mustCreateSession(t, store, "capture-session")
	if err := store.CreateRun(ctx, domain.Run{ID: "capture-run", SessionID: "capture-session", Kind: domain.RunKindPrimary, Status: domain.RunCompleted}); err != nil {
		t.Fatal(err)
	}
	sink := &fakeSink{}
	p := NewCognitiveCaptureProvider(store, sink, nil)
	for _, payload := range []string{`{"summary":"收到"}`, `{"summary":"收到","session_id":"foreign-session"}`} {
		_, err := p.ObserveRunWithReceipt(ctx, observer.NewRunEvent(observer.NewEventID("capture-run", 7), "run.completed", 10, json.RawMessage(payload)))
		if err == nil || len(sink.captures) != 0 {
			t.Fatalf("missing/foreign durable user source was accepted: %v", err)
		}
	}
}

// The old payload may have been accepted before the observer cursor failed.
// Rejoin that receipt without changing its idempotency key or rewriting it.
type legacyAcceptedCaptureSink struct {
	receipt CognitiveCaptureReceipt
	writes  int
}

func (s *legacyAcceptedCaptureSink) Capture(context.Context, CognitiveCapture) (CognitiveCaptureReceipt, error) {
	s.writes++
	return CognitiveCaptureReceipt{}, fmt.Errorf("event_conflict: original assistant-only payload already accepted")
}
func (s *legacyAcceptedCaptureSink) LookupCapture(context.Context, CognitiveCapture) (CognitiveCaptureReceipt, bool, error) {
	return s.receipt, true, nil
}
func TestMemoryLoopCaptureRejoinsLegacyAcceptedReceipt(t *testing.T) {
	ctx := context.Background()
	_, store, _ := newTestService(t, cognitiveTestModel())
	mustCreateSession(t, store, "legacy-session")
	run := domain.Run{ID: "legacy-run", SessionID: "legacy-session", Kind: domain.RunKindPrimary, Status: domain.RunCompleted}
	if err := store.CreateRun(ctx, run); err != nil {
		t.Fatal(err)
	}
	if err := store.AppendMessage(ctx, domain.Message{ID: "legacy-user", SessionID: run.SessionID, RunID: run.ID, Role: domain.RoleUser, Content: "retained journal fact", CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	sink := &legacyAcceptedCaptureSink{receipt: CognitiveCaptureReceipt{IngestionID: "original-receipt", Seq: 5, Status: "accepted"}}
	var notified int
	provider := NewCognitiveCaptureProvider(store, sink, func(r CognitiveCaptureReceipt) {
		if r.IngestionID != sink.receipt.IngestionID {
			t.Errorf("wrong receipt %+v", r)
		}
		notified++
	})
	event := observer.NewRunEvent(observer.NewEventID(string(run.ID), 7), "run.completed", 10, json.RawMessage(`{"summary":"收到","session_id":"legacy-session"}`))
	for i := 0; i < 2; i++ {
		r, err := provider.ObserveRunWithReceipt(ctx, event)
		if err != nil || r.ReceiptID != "original-receipt" {
			t.Fatalf("legacy receipt lost: %+v %v", r, err)
		}
	}
	if sink.writes != 0 || notified != 2 {
		t.Fatalf("rewrote accepted event or missed durable notification: writes=%d notified=%d", sink.writes, notified)
	}
}

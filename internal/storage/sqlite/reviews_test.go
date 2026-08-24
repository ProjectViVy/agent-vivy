package sqlite

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

func TestReviewProjectionRedactsArgumentsAndKeepsMetadata(t *testing.T) {
	ctx := context.Background()
	b, err := Open(ctx, filepath.Join(t.TempDir(), "reviews.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = b.Close() }()
	if err := b.CreateSession(ctx, domain.Session{ID: "sess-review", Title: "Review session", CreatedAt: time.Now().UnixMilli()}); err != nil {
		t.Fatal(err)
	}
	if err := b.CreateRun(ctx, domain.Run{ID: "run-review", SessionID: "sess-review", Status: domain.RunActive, CreatedAt: time.Now().UnixMilli()}); err != nil {
		t.Fatal(err)
	}
	approval := domain.Approval{
		ID: "apr_review", RunID: "run-review", ToolCallID: "call-review", ToolName: "http_request",
		Decision: domain.ApprovalPending, ExpiresAt: time.Now().Add(time.Minute).UnixMilli(), CreatedAt: time.Now().UnixMilli(),
		Action: "http_request", Target: "api.example.test", Preview: "POST /v1/items", RiskFindings: []string{"network mutation"},
	}
	if err := b.CreateApproval(ctx, approval); err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(map[string]any{
		"approval_id": "apr_review", "tool_call_id": "call-review", "tool_name": "http_request",
		"args": map[string]any{"url": "https://api.example.test", "api_key": "super-secret", "body": "hello"},
	})
	if _, err := b.Append(ctx, storage.Commit{RunID: "run-review", Events: []domain.RunEvent{{Type: domain.EventToolApprovalRequired, CreatedAt: time.Now().UnixMilli(), PayloadVersion: 1, Payload: payload}}}); err != nil {
		t.Fatal(err)
	}
	items, err := b.ListReviews(ctx, storage.ReviewFilter{Status: domain.ReviewPending})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != "apr_review" {
		t.Fatalf("reviews = %+v", items)
	}
	if items[0].SessionTitle != "Review session" || items[0].ToolName != "http_request" || items[0].Effect == "" {
		t.Fatalf("review metadata = %+v", items[0])
	}
	args := string(items[0].Arguments)
	if !strings.Contains(args, "[REDACTED]") || strings.Contains(args, "super-secret") {
		t.Fatalf("review arguments were not redacted: %s", args)
	}
	decided, err := b.DecideApprovalWithMetadata(ctx, "apr_review", domain.ApprovalDenied, "reviewer", "scope is too broad")
	if err != nil || !decided {
		t.Fatalf("decide approval: decided=%v err=%v", decided, err)
	}
	stored, err := b.GetApproval(ctx, "apr_review")
	if err != nil {
		t.Fatal(err)
	}
	if stored.Actor != "reviewer" || stored.DecisionReason != "scope is too broad" || stored.DecidedAt == 0 {
		t.Fatalf("approval metadata = %+v", stored)
	}
}

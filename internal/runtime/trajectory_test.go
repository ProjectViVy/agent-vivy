package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/testsupport"
)

func trajEvent(eventType domain.EventType, at int64, payload any) domain.RunEvent {
	data, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}
	return domain.RunEvent{Type: eventType, CreatedAt: at, Payload: data, PayloadVersion: 1}
}

func trajCommit(t *testing.T, ctx context.Context, journal storage.Journal, runID domain.RunID, events ...domain.RunEvent) {
	t.Helper()
	if _, err := journal.Append(ctx, storage.Commit{RunID: runID, Events: events}); err != nil {
		t.Fatalf("journal append: %v", err)
	}
}

// TestSessionTrajectoryProjection folds a hand-crafted two-run session over
// the real sqlite stores and asserts the record/request structure.
func TestSessionTrajectoryProjection(t *testing.T) {
	ctx := context.Background()
	svc, backend, _ := newTestService(t, testsupport.NewEchoModel())
	sessionID := domain.SessionID("sess-traj")

	run1, run2 := domain.RunID("run-traj-1"), domain.RunID("run-traj-2")
	if err := backend.CreateRun(ctx, domain.Run{ID: run1, SessionID: sessionID, Status: domain.RunCompleted, CreatedAt: 1000}); err != nil {
		t.Fatal(err)
	}
	if err := backend.CreateRun(ctx, domain.Run{ID: run2, SessionID: sessionID, Status: domain.RunFailed, CreatedAt: 9000}); err != nil {
		t.Fatal(err)
	}
	if err := backend.AppendMessage(ctx, domain.Message{ID: "m1", SessionID: sessionID, RunID: run1,
		Role: domain.RoleUser, CreatedAt: 1100, Content: "fix the readme"}); err != nil {
		t.Fatal(err)
	}
	if err := backend.AppendMessage(ctx, domain.Message{ID: "m2", SessionID: sessionID, RunID: run2,
		Role: domain.RoleUser, CreatedAt: 9100, Content: "second turn"}); err != nil {
		t.Fatal(err)
	}
	trajCommit(t, ctx, backend, run1,
		trajEvent(domain.EventRunStarted, 1200, payloadRunStarted{Provider: "test", Model: "m1", Mode: "chat"}),
		trajEvent(domain.EventModelRequest, 1300, payloadModelRequest{PreambleBytes: 128, Messages: []payloadModelRequestMessage{
			{Role: "system", ContentSHA256: "abc", ByteLen: 128},
			{Role: "user", ContentSHA256: "def", ByteLen: 30},
		}}),
		trajEvent(domain.EventModelUsage, 1400, payloadModelUsage{PromptTokens: 100, CompletionTokens: 20, TotalTokens: 120, ReasoningTokens: 5, CachedTokens: 30}),
		trajEvent(domain.EventModelCompleted, 1500, payloadModelCompleted{Content: "step one text"}),
		trajEvent(domain.EventToolRequested, 1550, payloadToolRequested{ToolCallID: "call-1", ToolName: "read_file", Args: map[string]any{"path": "README.md"}}),
		trajEvent(domain.EventToolStarted, 1600, payloadToolStarted{ToolCallID: "call-1", ToolName: "read_file"}),
		trajEvent(domain.EventToolFinished, 1900, payloadToolFinished{ToolCallID: "call-1", ToolName: "read_file", Result: "21 lines"}),
		trajEvent(domain.EventModelRequest, 2000, payloadModelRequest{PreambleBytes: 128}),
		trajEvent(domain.EventModelUsage, 2100, payloadModelUsage{PromptTokens: 200, CompletionTokens: 10, TotalTokens: 210}),
		trajEvent(domain.EventModelCompleted, 2200, payloadModelCompleted{Content: "final"}),
		trajEvent(domain.EventContextCompacted, 2300, payloadContextCompacted{Mode: "reduction", BeforeTokens: 900, AfterTokens: 300}),
	)
	trajCommit(t, ctx, backend, run2,
		trajEvent(domain.EventRunStarted, 9200, payloadRunStarted{Provider: "test", Model: "m1", Mode: "chat"}),
		trajEvent(domain.EventModelRequest, 9300, payloadModelRequest{PreambleBytes: 64}),
		trajEvent(domain.EventModelUsage, 9400, payloadModelUsage{PromptTokens: 50, CompletionTokens: 5, TotalTokens: 55}),
		trajEvent(domain.EventRunFailed, 9500, payloadRunFailed{CauseCategory: causeToolError, Message: "boom"}),
	)

	session, err := svc.SessionTrajectory(ctx, sessionID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if session.Turns != 2 || session.SessionID != string(sessionID) {
		t.Fatalf("turns = %d session = %s, want 2 / sess-traj", session.Turns, session.SessionID)
	}
	kinds := make([]string, 0, len(session.Records))
	for _, record := range session.Records {
		kinds = append(kinds, record.Kind)
	}
	wantOrder := "system,user,message,tool,message,compacted,user,message"
	if got := strings.Join(kinds, ","); got != wantOrder {
		t.Fatalf("record kinds = %s, want %s", got, wantOrder)
	}
	for i, record := range session.Records {
		if record.Index != i+1 || record.ID != fmt.Sprintf("rec-%d", i+1) {
			t.Fatalf("record %d has index=%d id=%s", i, record.Index, record.ID)
		}
	}
	first := session.Records[0]
	if first.Turn != nil || first.Group != "Session" {
		t.Fatalf("system row = %+v, want turn null group Session", first)
	}
	user1 := session.Records[1]
	if *user1.Turn != 1 || !user1.OpensTurn || user1.Text != "fix the readme" {
		t.Fatalf("user row = %+v", user1)
	}
	step1 := session.Records[2]
	if step1.Group != "Step 1" || step1.Text != "step one text" || step1.Tokens == nil ||
		step1.Tokens.Input != 100 || step1.Tokens.Output != 20 || step1.Tokens.Think != 5 || step1.Tokens.CacheRead != 30 {
		t.Fatalf("step1 message row = %+v", step1)
	}
	if step1.TimeSeconds == nil || *step1.TimeSeconds < 0 {
		t.Fatalf("step1 timing = %+v", step1)
	}
	tool := session.Records[3]
	if tool.Text != "read_file" || tool.CallID != "call-1" || tool.Result != "21 lines" ||
		tool.IsError || !strings.Contains(tool.InputDetail, "README.md") {
		t.Fatalf("tool row = %+v", tool)
	}
	compacted := session.Records[5]
	if compacted.Turn != nil || !strings.Contains(compacted.Text, "reduction") {
		t.Fatalf("compacted row = %+v", compacted)
	}
	user2 := session.Records[6]
	if *user2.Turn != 2 || user2.Text != "second turn" {
		t.Fatalf("user2 row = %+v", user2)
	}
	failureRow := session.Records[7]
	if !failureRow.IsError || failureRow.Text != "boom" || failureRow.Group != "Run" {
		t.Fatalf("failure row = %+v", failureRow)
	}
	if len(session.Requests) != 3 {
		t.Fatalf("requests = %d, want 3", len(session.Requests))
	}
	for i, request := range session.Requests {
		if request.Number != i+1 {
			t.Fatalf("request %d number = %d", i, request.Number)
		}
	}
	if session.Requests[0].Status != "complete" || session.Requests[0].Usage.Input != 100 ||
		session.Requests[0].Messages != 2 || session.Requests[0].PreambleBytes != 128 {
		t.Fatalf("request 1 = %+v", session.Requests[0])
	}
	if session.Requests[2].Status != "error" || session.Requests[2].Error != "boom" {
		t.Fatalf("request 3 = %+v", session.Requests[2])
	}
	if session.Requests[0].Provider != "test" || session.Requests[0].Model != "m1" {
		t.Fatalf("request provider/model = %+v", session.Requests[0])
	}
}

// TestSessionTrajectoryLimitClamp keeps the projection bounded: only the
// most recent runs survive a small limit.
func TestSessionTrajectoryLimitClamp(t *testing.T) {
	ctx := context.Background()
	svc, backend, _ := newTestService(t, testsupport.NewEchoModel())
	sessionID := domain.SessionID("sess-traj-limit")
	models := []string{"m-alpha", "m-beta", "m-gamma"}
	for i := 0; i < 3; i++ {
		runID := domain.RunID(fmt.Sprintf("run-l-%d", i))
		if err := backend.CreateRun(ctx, domain.Run{ID: runID, SessionID: sessionID, Status: domain.RunCompleted, CreatedAt: int64(1000 + i)}); err != nil {
			t.Fatal(err)
		}
		trajCommit(t, ctx, backend, runID,
			trajEvent(domain.EventRunStarted, int64(1100+i), payloadRunStarted{Provider: "test", Model: models[i], Mode: "chat"}))
	}
	session, err := svc.SessionTrajectory(ctx, sessionID, 2)
	if err != nil {
		t.Fatal(err)
	}
	if session.Turns != 2 {
		t.Fatalf("turns = %d, want 2", session.Turns)
	}
	for _, record := range session.Records {
		if strings.Contains(record.Text, "m-alpha") {
			t.Fatalf("stale run leaked: %+v", record)
		}
	}
}

// TestSessionTrajectoryRealRun drives a real echo run through the service
// and projects it: the user text and final answer land as records.
func TestSessionTrajectoryRealRun(t *testing.T) {
	ctx := context.Background()
	svc, _, _ := newTestService(t, testsupport.NewEchoModel())
	sessionID := domain.SessionID("sess-traj-real")
	runID, err := svc.Run(ctx, sessionID, "hello trajectory")
	if err != nil {
		t.Fatal(err)
	}
	waitForRunStatus(t, svc.deps.Runs, runID, domain.RunCompleted)
	session, err := svc.SessionTrajectory(ctx, sessionID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if session.Turns != 1 {
		t.Fatalf("turns = %d, want 1", session.Turns)
	}
	var sawUser, sawMessage bool
	for _, record := range session.Records {
		if record.Kind == "user" && record.Text == "hello trajectory" && record.OpensTurn {
			sawUser = true
		}
		if record.Kind == "message" && strings.Contains(record.Text, "test response to: hello trajectory") {
			sawMessage = true
		}
	}
	if !sawUser || !sawMessage {
		t.Fatalf("user=%v message=%v records=%+v", sawUser, sawMessage, session.Records)
	}
	if len(session.Requests) != 1 || session.Requests[0].Status != "complete" {
		t.Fatalf("requests = %+v", session.Requests)
	}
}

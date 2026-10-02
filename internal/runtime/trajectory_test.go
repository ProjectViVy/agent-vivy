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
	return trajEventVersion(eventType, at, payload, 1)
}

func trajEventVersion(eventType domain.EventType, at int64, payload any, version int) domain.RunEvent {
	data, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}
	return domain.RunEvent{Type: eventType, CreatedAt: at, Payload: data, PayloadVersion: version}
}

func trajCompletedV2(at int64, content string) domain.RunEvent {
	return trajEventVersion(domain.EventModelCompleted, at, payloadModelCompletedV2{
		ContentSHA256: sha256Hex([]byte(content)), ByteLen: len([]byte(content)),
	}, 2)
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
	mustCreateSession(t, backend, sessionID)

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
		trajEvent(domain.EventModelDelta, 1450, payloadModelDelta{Delta: "step one text"}),
		trajCompletedV2(1500, "step one text"),
		trajEvent(domain.EventToolRequested, 1550, payloadToolRequested{ToolCallID: "call-1", ToolName: "read_file", Args: map[string]any{"path": "README.md"}}),
		trajEvent(domain.EventToolStarted, 1600, payloadToolStarted{ToolCallID: "call-1", ToolName: "read_file"}),
		trajEvent(domain.EventToolFinished, 1900, payloadToolFinished{ToolCallID: "call-1", ToolName: "read_file", Result: "21 lines"}),
		trajEvent(domain.EventModelRequest, 2000, payloadModelRequest{PreambleBytes: 128}),
		trajEvent(domain.EventModelUsage, 2100, payloadModelUsage{PromptTokens: 200, CompletionTokens: 10, TotalTokens: 210}),
		trajEvent(domain.EventModelDelta, 2150, payloadModelDelta{Delta: "final"}),
		trajCompletedV2(2200, "final"),
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
		if record.Index != i+1 || record.ID == "" {
			t.Fatalf("record %d has index=%d id=%q", i, record.Index, record.ID)
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

// TestTrajectoryMultipleCallsStableIdentity folds a run with two v3 calls and
// a tool row: calls keep separate stable request_ids keyed by (run_id,
// call_id), the latest usage sample replaces its own attempt, and a rebuild
// reproduces identical IDs.
func TestTrajectoryMultipleCallsStableIdentity(t *testing.T) {
	ctx := context.Background()
	svc, backend, _ := newTestService(t, testsupport.NewEchoModel())
	sessionID := domain.SessionID("sess-traj-ids")
	mustCreateSession(t, backend, sessionID)

	runID := domain.RunID("run-traj-ids")
	if err := backend.CreateRun(ctx, domain.Run{ID: runID, SessionID: sessionID, Status: domain.RunCompleted, CreatedAt: 1000}); err != nil {
		t.Fatal(err)
	}
	reasoning := 7
	cached := 30
	trajCommit(t, ctx, backend, runID,
		trajEvent(domain.EventRunStarted, 1200, payloadRunStarted{Provider: "test", Model: "m1", Mode: "chat"}),
		trajEvent(domain.EventModelRequest, 1300, payloadModelRequestV3{
			payloadModelRequest: payloadModelRequest{PreambleBytes: 10, Messages: []payloadModelRequestMessage{{Role: "user"}}},
			CallID:              "call-a", Mode: "stream", Provider: "test", Model: "m1", Source: "main",
		}),
		trajEvent(domain.EventModelUsage, 1350, payloadModelUsageV2{
			CallID: "call-a", Provider: "test", Model: "m1", Source: "main", UsageKind: "cumulative",
			PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15,
		}),
		trajEvent(domain.EventModelUsage, 1400, payloadModelUsageV2{
			CallID: "call-a", Provider: "test", Model: "m1", Source: "main", UsageKind: "cumulative",
			PromptTokens: 100, CompletionTokens: 20, TotalTokens: 120,
			ReasoningTokens: &reasoning, CachedTokens: &cached, Settlement: boolPtr(true),
		}),
		trajEvent(domain.EventModelCallFinished, 1500, payloadModelCallFinished{
			CallID: "call-a", Mode: "stream", Provider: "test", Model: "m1", Source: "main",
			Status: "completed", ResponseComplete: true,
		}),
		trajEvent(domain.EventModelRequest, 1600, payloadModelRequestV3{
			payloadModelRequest: payloadModelRequest{PreambleBytes: 20},
			CallID:              "call-b", Mode: "stream", Provider: "test", Model: "m1", Source: "main",
		}),
		trajEvent(domain.EventToolRequested, 1650, payloadToolRequested{ToolCallID: "tc-1", ToolName: "read_file", Args: map[string]any{"path": "x"}}),
		trajEvent(domain.EventToolFinished, 1700, payloadToolFinished{ToolCallID: "tc-1", ToolName: "read_file", Result: "ok"}),
		trajEvent(domain.EventModelCallFinished, 1800, payloadModelCallFinished{
			CallID: "call-b", Mode: "stream", Provider: "test", Model: "m1", Source: "main",
			Status: "completed", ResponseComplete: true,
		}),
		trajEvent(domain.EventRunCompleted, 1900, payloadRunCompleted{Outcome: "completed"}),
	)

	first, err := svc.SessionTrajectory(ctx, sessionID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Requests) != 2 {
		t.Fatalf("requests = %d, want 2", len(first.Requests))
	}
	a, b := first.Requests[0], first.Requests[1]
	if a.RequestID != "run-traj-ids:call-a" || a.CallID != "call-a" || a.CallStatus != TrajCallCompleted {
		t.Fatalf("call a = %+v", a)
	}
	if b.RequestID != "run-traj-ids:call-b" || b.CallID != "call-b" || b.CallStatus != TrajCallCompleted {
		t.Fatalf("call b = %+v", b)
	}
	// Latest sample replaces the earlier one on the same attempt.
	if a.UsageState != TrajUsageReported || a.UsageEvidence == nil ||
		a.UsageEvidence.PromptTokens != 100 || a.UsageEvidence.TotalTokens != 120 {
		t.Fatalf("call a evidence = %+v", a.UsageEvidence)
	}
	if a.UsageEvidence.ReasoningTokens == nil || *a.UsageEvidence.ReasoningTokens != 7 ||
		a.UsageEvidence.CachedTokens == nil || *a.UsageEvidence.CachedTokens != 30 {
		t.Fatalf("call a optional buckets = %+v", a.UsageEvidence)
	}
	if b.UsageState != TrajUsageMissing || b.UsageEvidence != nil {
		t.Fatalf("call b usage = %s %+v", b.UsageState, b.UsageEvidence)
	}
	if a.Provider != "test" || a.Model != "m1" || a.StartedAt != 1300 || a.FinishedAt == nil || *a.FinishedAt != 1500 {
		t.Fatalf("call a lifecycle = %+v", a)
	}
	// Record IDs carry the persisted (run_id, seq, kind) triple.
	var toolRec *TrajectoryRecord
	for i := range first.Records {
		if first.Records[i].Kind == "tool" {
			toolRec = &first.Records[i]
		}
	}
	if toolRec == nil || toolRec.ID != "run-traj-ids:8:tool" || toolRec.CallID != "tc-1" {
		t.Fatalf("tool record = %+v", toolRec)
	}
	if first.Watermarks[string(runID)] != 10 {
		t.Fatalf("watermark = %v", first.Watermarks)
	}
	if first.HasOlderRuns {
		t.Fatal("has_older_runs = true on single-run session")
	}
	// Rebuild is a pure fold: identical IDs and rows.
	second, err := svc.SessionTrajectory(ctx, sessionID, 0)
	if err != nil {
		t.Fatal(err)
	}
	for i := range first.Requests {
		if first.Requests[i].RequestID != second.Requests[i].RequestID {
			t.Fatalf("request %d id drifted: %s != %s", i, first.Requests[i].RequestID, second.Requests[i].RequestID)
		}
	}
	for i := range first.Records {
		if first.Records[i].ID != second.Records[i].ID {
			t.Fatalf("record %d id drifted: %s != %s", i, first.Records[i].ID, second.Records[i].ID)
		}
	}
}

// TestTrajectoryActiveWaitCancelAndInterrupted pins the lifecycle truth:
// active calls are neither failed nor completed, waiting approvals surface
// on run_activity (never on the call), and terminal runs interrupt open
// calls without inventing usage.
func TestTrajectoryActiveWaitCancelAndInterrupted(t *testing.T) {
	ctx := context.Background()
	svc, backend, _ := newTestService(t, testsupport.NewEchoModel())
	sessionID := domain.SessionID("sess-traj-life")
	mustCreateSession(t, backend, sessionID)

	liveRun := domain.RunID("run-live")
	if err := backend.CreateRun(ctx, domain.Run{ID: liveRun, SessionID: sessionID, Status: domain.RunActive, CreatedAt: 1000}); err != nil {
		t.Fatal(err)
	}
	trajCommit(t, ctx, backend, liveRun,
		trajEvent(domain.EventRunStarted, 1100, payloadRunStarted{Provider: "test", Model: "m1", Mode: "chat"}),
		trajEvent(domain.EventModelRequest, 1200, payloadModelRequestV3{
			CallID: "call-live", Mode: "stream", Provider: "test", Model: "m1", Source: "main",
		}),
		trajEvent(domain.EventToolApprovalRequired, 1300, payloadToolApprovalRequired{
			ApprovalID: "appr-1", ToolCallID: "tc-9", ToolName: "shell", Face: "web",
		}),
	)
	session, err := svc.SessionTrajectory(ctx, sessionID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(session.Requests) != 1 {
		t.Fatalf("requests = %+v", session.Requests)
	}
	live := session.Requests[0]
	if live.CallStatus != TrajCallActive || live.Status == "complete" || live.Status == "error" || live.FinishedAt != nil {
		t.Fatalf("active call = %+v", live)
	}
	if live.UsageState != TrajUsageActive || live.UsageEvidence != nil {
		t.Fatalf("active usage = %s %+v", live.UsageState, live.UsageEvidence)
	}
	if len(session.RunActivity) != 1 || session.RunActivity[0].ActivityState != TrajActivityWaiting ||
		session.RunActivity[0].WaitKind != TrajWaitApproval || session.RunActivity[0].Status != string(domain.RunActive) {
		t.Fatalf("run activity = %+v", session.RunActivity)
	}

	// Second run: cancelled mid-call — interrupted, never complete.
	deadRun := domain.RunID("run-dead")
	if err := backend.CreateRun(ctx, domain.Run{ID: deadRun, SessionID: sessionID, Status: domain.RunCancelled, CreatedAt: 2000}); err != nil {
		t.Fatal(err)
	}
	trajCommit(t, ctx, backend, deadRun,
		trajEvent(domain.EventRunStarted, 2100, payloadRunStarted{Provider: "test", Model: "m1", Mode: "chat"}),
		trajEvent(domain.EventModelRequest, 2200, payloadModelRequestV3{
			CallID: "call-dead", Mode: "stream", Provider: "test", Model: "m1", Source: "main",
		}),
		trajEvent(domain.EventRunCancelled, 2300, payloadRunCancelled{Reason: reasonUserRequested, Outcome: "cancelled"}),
	)
	session, err = svc.SessionTrajectory(ctx, sessionID, 0)
	if err != nil {
		t.Fatal(err)
	}
	var dead *TrajectoryRequest
	for i := range session.Requests {
		if session.Requests[i].CallID == "call-dead" {
			dead = &session.Requests[i]
		}
	}
	if dead == nil || dead.CallStatus != TrajCallInterrupted || dead.UsageState != TrajUsageMissing || dead.UsageEvidence != nil {
		t.Fatalf("dead call = %+v", dead)
	}
	var deadActivity *TrajectoryRunActivity
	for i := range session.RunActivity {
		if session.RunActivity[i].RunID == string(deadRun) {
			deadActivity = &session.RunActivity[i]
		}
	}
	if deadActivity == nil || deadActivity.ActivityState != TrajActivityCancelled || deadActivity.WaitKind != "" {
		t.Fatalf("dead activity = %+v", deadActivity)
	}
}

// TestTrajectoryWatermarkWindow asserts watermarks come from the folded
// prefix and has_older_runs reflects the window boundary.
func TestTrajectoryWatermarkWindow(t *testing.T) {
	ctx := context.Background()
	svc, backend, _ := newTestService(t, testsupport.NewEchoModel())
	sessionID := domain.SessionID("sess-traj-wm")
	mustCreateSession(t, backend, sessionID)
	for i := 0; i < 3; i++ {
		runID := domain.RunID(fmt.Sprintf("run-wm-%d", i))
		if err := backend.CreateRun(ctx, domain.Run{ID: runID, SessionID: sessionID, Status: domain.RunCompleted, CreatedAt: int64(1000 + i)}); err != nil {
			t.Fatal(err)
		}
		trajCommit(t, ctx, backend, runID,
			trajEvent(domain.EventRunStarted, int64(1100+i), payloadRunStarted{Provider: "test", Model: "m", Mode: "chat"}),
			trajEvent(domain.EventModelRequest, int64(1200+i), payloadModelRequest{PreambleBytes: 1}),
			trajEvent(domain.EventRunCompleted, int64(1300+i), payloadRunCompleted{Outcome: "completed"}),
		)
	}
	session, err := svc.SessionTrajectory(ctx, sessionID, 2)
	if err != nil {
		t.Fatal(err)
	}
	if !session.HasOlderRuns || len(session.Watermarks) != 2 || session.Watermarks["run-wm-1"] != 3 || session.Watermarks["run-wm-2"] != 3 {
		t.Fatalf("window = older %v watermarks %v", session.HasOlderRuns, session.Watermarks)
	}
	if session.ProjectionVersion != 2 {
		t.Fatalf("projection_version = %d", session.ProjectionVersion)
	}
}

func boolPtr(v bool) *bool { return &v }

// TestSessionTrajectoryRealRun drives a real echo run through the service
// and projects it: the user text and final answer land as records.
func TestSessionTrajectoryRealRun(t *testing.T) {
	ctx := context.Background()
	svc, _, _ := newTestService(t, testsupport.NewEchoModel())
	sessionID := domain.SessionID("sess-traj-real")
	mustCreateSession(t, svc.deps.Sessions, sessionID)
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

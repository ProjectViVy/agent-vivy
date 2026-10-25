package runtime

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/internal/tools"
)

// feedCaptureModel records each model input before replaying the scripted
// answer, so a test can assert exactly what the runtime fed the provider.
type feedCaptureModel struct {
	*ScriptedModel
	mu     sync.Mutex
	inputs [][]*schema.AgenticMessage
}

func (m *feedCaptureModel) record(input []*schema.AgenticMessage) {
	m.mu.Lock()
	m.inputs = append(m.inputs, append([]*schema.AgenticMessage(nil), input...))
	m.mu.Unlock()
}

func (m *feedCaptureModel) Generate(ctx context.Context, input []*schema.AgenticMessage, opts ...model.Option) (*schema.AgenticMessage, error) {
	m.record(input)
	return m.ScriptedModel.Generate(ctx, input, opts...)
}

func (m *feedCaptureModel) Stream(ctx context.Context, input []*schema.AgenticMessage, opts ...model.Option) (*schema.StreamReader[*schema.AgenticMessage], error) {
	m.record(input)
	return m.ScriptedModel.Stream(ctx, input, opts...)
}

func (m *feedCaptureModel) firstInput(t *testing.T) []*schema.AgenticMessage {
	t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.inputs) == 0 {
		t.Fatal("model was never called")
	}
	return m.inputs[0]
}

func seedChildTranscript(t *testing.T, ctx context.Context, backend *sqlite.Backend, sessionID domain.SessionID) {
	t.Helper()
	runs := []domain.Run{
		{ID: "run-parent-history", SessionID: sessionID, Status: domain.RunCompleted, CreatedAt: 10, Kind: domain.RunKindPrimary},
		{ID: "run-child-history", SessionID: sessionID, Status: domain.RunCompleted, CreatedAt: 20, Kind: domain.RunKindChild,
			ParentID: "run-parent-history", RootID: "run-parent-history", Depth: 1},
	}
	for _, run := range runs {
		if err := backend.CreateRun(ctx, run); err != nil {
			t.Fatalf("create run %s: %v", run.ID, err)
		}
	}
	messages := []domain.Message{
		{ID: "msg-parent-user", SessionID: sessionID, RunID: "run-parent-history", Role: domain.RoleUser, CreatedAt: 11, Content: "parent-history-marker"},
		{ID: "msg-parent-assistant", SessionID: sessionID, RunID: "run-parent-history", Role: domain.RoleAssistant, CreatedAt: 12, Content: "primary-answer-marker"},
		{ID: "msg-child-assistant", SessionID: sessionID, RunID: "run-child-history", Role: domain.RoleAssistant, CreatedAt: 21, Content: "child-answer-marker"},
		{ID: "msg-child-tool", SessionID: sessionID, RunID: "run-child-history", Role: domain.RoleTool, CreatedAt: 22,
			Content: "child-transcript-marker", ToolCallID: "call-child-1", ToolName: "read_file"},
		{ID: "msg-legacy", SessionID: sessionID, Role: domain.RoleUser, CreatedAt: 30, Content: "legacy-row-marker"},
	}
	for _, msg := range messages {
		if err := backend.AppendMessage(ctx, msg); err != nil {
			t.Fatalf("append message %s: %v", msg.ID, err)
		}
	}
}

func TestParentFeedMessagesDropsChildTranscriptsOnly(t *testing.T) {
	ctx := context.Background()
	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "child-feed.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = backend.Close() })
	sessionID := domain.SessionID("sess-child-feed")
	if err := backend.CreateSession(ctx, domain.Session{ID: sessionID, Title: "child feed", CreatedAt: 1}); err != nil {
		t.Fatalf("create session: %v", err)
	}
	seedChildTranscript(t, ctx, backend, sessionID)

	svc := NewService(nil, "test", "test-model", ServiceDeps{Runs: backend, Messages: backend})
	stored, err := backend.ListMessages(ctx, sessionID)
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	kept := svc.parentFeedMessages(ctx, sessionID, stored)
	var keptIDs []string
	for _, msg := range kept {
		keptIDs = append(keptIDs, msg.ID)
	}
	want := []string{"msg-parent-user", "msg-parent-assistant", "msg-legacy"}
	if strings.Join(keptIDs, ",") != strings.Join(want, ",") {
		t.Fatalf("feed rows = %v, want %v (child transcript stays in the store, not in the feed)", keptIDs, want)
	}
	if len(stored) != 5 {
		t.Fatalf("store rows = %d, want the child rows preserved for the transcript", len(stored))
	}
}

func TestContextStatusCountsOnlyParentFeed(t *testing.T) {
	ctx := context.Background()
	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "child-feed-status.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = backend.Close() })
	sessionID := domain.SessionID("sess-child-feed-status")
	if err := backend.CreateSession(ctx, domain.Session{ID: sessionID, Title: "child feed status", CreatedAt: 1}); err != nil {
		t.Fatalf("create session: %v", err)
	}
	seedChildTranscript(t, ctx, backend, sessionID)

	eng, err := NewEngine(ctx, NewScriptedModel(testAssistant("ok")), nil, EngineConfig{StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	svc := NewService(eng, "test", "test-model", ServiceDeps{
		Journal: backend, Runs: backend, Messages: backend, Sessions: backend, Sink: newTestSink(),
	})
	status, err := svc.ContextStatus(ctx, sessionID)
	if err != nil {
		t.Fatalf("context status: %v", err)
	}
	if status.TotalMessages != 5 {
		t.Fatalf("total messages = %d, want the full stored transcript (5)", status.TotalMessages)
	}
	if status.FeedMessages != 3 {
		t.Fatalf("feed messages = %d, want 3 parent/legacy rows", status.FeedMessages)
	}
	stored, err := backend.ListMessages(ctx, sessionID)
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	_, unfilteredTokens := historyBytesTokens(feedableMessages(stored))
	if status.FeedTokens >= unfilteredTokens {
		t.Fatalf("feed tokens = %d, must exclude the child transcript (%d tokens with it)", status.FeedTokens, unfilteredTokens)
	}
}

func TestRunFeedExcludesChildTranscripts(t *testing.T) {
	ctx := context.Background()
	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "child-feed-run.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = backend.Close() })
	const sessionID = domain.SessionID("sess-child-feed-run")
	if err := backend.CreateSession(ctx, domain.Session{ID: sessionID, Title: "child feed run", CreatedAt: 1,
		SandboxMode: string(domain.SandboxModeWorkspaceWrite), ApprovalPolicy: string(domain.ApprovalPolicyAuto)}); err != nil {
		t.Fatalf("create session: %v", err)
	}
	seedChildTranscript(t, ctx, backend, sessionID)

	toolset, err := tools.NewRegistry(tools.NewEnterPlanMode()).Resolve([]string{tools.EnterPlanModeName})
	if err != nil {
		t.Fatalf("resolve tools: %v", err)
	}
	scripted := NewScriptedModel(testAssistant("done"))
	capture := &feedCaptureModel{ScriptedModel: scripted}
	engine, err := NewEngine(ctx, capture, toolset, EngineConfig{StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	svc := NewService(engine, "scripted", "scripted-v0", ServiceDeps{
		Journal: backend, Runs: backend, Messages: backend, Sessions: backend,
		PrimaryRuns: backend, Sink: newTestSink(), PolicyDefaultProfile: domain.PolicyProfileFullAuto,
	})
	runID, err := svc.Run(ctx, sessionID, "continue the conversation")
	if err != nil {
		t.Fatalf("start run: %v", err)
	}
	waitForTerminalRun(t, backend, runID)

	var fed strings.Builder
	for _, msg := range capture.firstInput(t) {
		fed.WriteString(agenticMessageText(msg))
		fed.WriteString("\n")
	}
	got := fed.String()
	if strings.Contains(got, "child-answer-marker") || strings.Contains(got, "child-transcript-marker") {
		t.Fatalf("child transcript reached the parent model input:\n%s", got)
	}
	if !strings.Contains(got, "primary-answer-marker") || !strings.Contains(got, "legacy-row-marker") {
		t.Fatalf("parent history missing from the model input:\n%s", got)
	}
}

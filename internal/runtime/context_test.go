package runtime

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/contexthost"
	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/internal/testsupport"
	"agent-vivy/internal/tools"
	"agent-vivy/sdk/port/contextsource"
)

type runtimeContextFixtureSource struct{}

func (runtimeContextFixtureSource) ID() string { return "fixture.docs" }
func (runtimeContextFixtureSource) Query(context.Context, contextsource.Request) (contextsource.Page, error) {
	return contextsource.NewPage([]contextsource.Candidate{{
		SourceID: "fixture.docs", ContentID: "guide", MediaType: "text/plain", Content: "generic source body", Confidence: 0.8,
	}}, ""), nil
}

type workspaceRecordingSource struct {
	workspaceID chan string
}

func (source workspaceRecordingSource) ID() string { return "fixture.workspace" }
func (source workspaceRecordingSource) Query(_ context.Context, request contextsource.Request) (contextsource.Page, error) {
	source.workspaceID <- request.WorkspaceID
	return contextsource.NewPage(nil, ""), nil
}

type fixedWorkspaceAllocator struct{}

func (fixedWorkspaceAllocator) Ensure(context.Context, domain.RunID) (Workspace, error) {
	return Workspace{ID: "workspace-actual", Path: "/workspace/actual"}, nil
}

func TestBuildRunContextKeepsCurrentAndRecentHistory(t *testing.T) {
	stored := []domain.Message{
		{Role: domain.RoleUser, Content: "old question"},
		{Role: domain.RoleAssistant, Content: "old answer"},
		{Role: domain.RoleUser, Content: "recent question"},
		{Role: domain.RoleAssistant, Content: "recent answer"},
		{Role: domain.RoleUser, Content: "current"},
	}

	msgs, stats, err := buildRunContext(ContextPolicy{MaxHistoryMessages: 2}, "preamble", stored, "current")
	if err != nil {
		t.Fatalf("build context: %v", err)
	}
	if stats.OriginalHistoryMessages != 4 {
		t.Fatalf("original history = %d, want 4 feedable history rows", stats.OriginalHistoryMessages)
	}
	if stats.IncludedHistoryMessages != 2 || stats.DroppedHistoryMessages != 2 {
		t.Fatalf("context stats = %+v, want two included and two dropped", stats)
	}
	if len(msgs) != 4 {
		t.Fatalf("message count = %d, want preamble + two history + current", len(msgs))
	}
	if msgs[0].Role != "system" || msgs[0].Content != "preamble" {
		t.Fatalf("preamble = %+v", msgs[0])
	}
	if msgs[1].Content != "recent question" || msgs[2].Content != "recent answer" || msgs[3].Content != "current" {
		t.Fatalf("bounded context contents = %+v", msgs)
	}
}

func TestBuildRunContextKeepsPairedToolTurns(t *testing.T) {
	stored := []domain.Message{
		{Role: domain.RoleUser, Content: "look up"},
		{Role: domain.RoleAssistant, ToolCallID: "c1", ToolName: "echo_info", ToolArgs: []byte(`{"text":"hi"}`)},
		{Role: domain.RoleTool, Content: "hi", ToolCallID: "c1", ToolName: "echo_info"},
		{Role: domain.RoleAssistant, Content: "done"},
		{Role: domain.RoleUser, Content: "again"},
	}
	msgs, stats, err := buildRunContext(ContextPolicy{}, "preamble", stored, "again")
	if err != nil {
		t.Fatalf("build context: %v", err)
	}
	if stats.OriginalHistoryMessages != 4 {
		t.Fatalf("original history = %d, want 4", stats.OriginalHistoryMessages)
	}
	if len(msgs) != 6 {
		t.Fatalf("message count = %d, want preamble + user + assistant-call + tool + assistant + current", len(msgs))
	}
	if msgs[2].Role != "assistant" || len(msgs[2].ToolCalls) != 1 || msgs[2].ToolCalls[0].ID != "c1" {
		t.Fatalf("assistant tool call = %+v", msgs[2])
	}
	if msgs[3].Role != "tool" || msgs[3].ToolCallID != "c1" || msgs[3].Content != "hi" {
		t.Fatalf("tool result = %+v", msgs[3])
	}
}

func TestBuildRunContextDropsUnpairedToolRows(t *testing.T) {
	stored := []domain.Message{
		{Role: domain.RoleUser, Content: "q"},
		{Role: domain.RoleAssistant, ToolCallID: "orphan", ToolName: "echo_info", ToolArgs: []byte(`{}`)},
		{Role: domain.RoleUser, Content: "next"},
	}
	msgs, _, err := buildRunContext(ContextPolicy{}, "preamble", stored, "next")
	if err != nil {
		t.Fatalf("build context: %v", err)
	}
	if len(msgs) != 3 {
		t.Fatalf("message count = %d, want preamble + user + current (orphan call dropped)", len(msgs))
	}
}

func TestBuildRunContextEnforcesByteBudget(t *testing.T) {
	base := messageCost("preamble", "system") + messageCost("current", string(domain.RoleUser))
	latest := messageCost("latest", string(domain.RoleAssistant))
	stored := []domain.Message{
		{Role: domain.RoleUser, Content: "old"},
		{Role: domain.RoleAssistant, Content: "latest"},
		{Role: domain.RoleUser, Content: "current"},
	}

	msgs, stats, err := buildRunContext(ContextPolicy{MaxBytes: base + latest}, "preamble", stored, "current")
	if err != nil {
		t.Fatalf("build context: %v", err)
	}
	if stats.Bytes != base+latest || stats.DroppedHistoryMessages != 1 {
		t.Fatalf("context stats = %+v, want latest history only", stats)
	}
	if len(msgs) != 3 || msgs[1].Content != "latest" || msgs[2].Content != "current" {
		t.Fatalf("byte-bounded context = %+v", msgs)
	}
}

func TestBuildRunContextRejectsMandatoryOverflow(t *testing.T) {
	_, _, err := buildRunContext(ContextPolicy{MaxBytes: 1}, "preamble", nil, "current")
	if err == nil || !strings.Contains(err.Error(), ErrContextBudgetExceeded.Error()) {
		t.Fatalf("error = %v, want context budget error", err)
	}
}

func TestServiceContextBudgetFailureIsTerminal(t *testing.T) {
	ctx := context.Background()
	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "journal.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = backend.Close() })

	ts, err := tools.Builtin(backend).Resolve([]string{tools.EchoInfoName})
	if err != nil {
		t.Fatalf("resolve tools: %v", err)
	}
	eng, err := NewEngine(ctx, WrapModel(testsupport.NewEchoModel()), ts, EngineConfig{
		StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10, MaxContextBytes: 64, MaxHistoryMessages: 2,
	})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	sink := newTestSink()
	svc := NewService(eng, "test", "test-model", ServiceDeps{
		Journal: backend, Runs: backend, Messages: backend, Notes: backend, Sink: sink,
	})

	runID, err := svc.Run(ctx, "session-context", strings.Repeat("x", 200))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	waitForRunStatus(t, backend, runID, domain.RunFailed)
	events := replayAll(t, backend, runID)
	last := events[len(events)-1]
	if last.Type != domain.EventRunFailed {
		t.Fatalf("last event = %s, want run.failed", last.Type)
	}
	category, message := payloadFailureOf(t, last.Payload)
	if category != causeInternalError || !strings.Contains(message, "context") {
		t.Fatalf("failure = (%q, %q), want bounded context failure", category, message)
	}
	if countTerminal(events) != 1 {
		t.Fatalf("terminal events = %d, want one", countTerminal(events))
	}
}

func TestServiceRunProjectsGenericContextHostIntoModelInput(t *testing.T) {
	ctx := context.Background()
	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "journal.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = backend.Close() })
	ts, err := tools.Builtin(backend).Resolve([]string{tools.EchoInfoName})
	if err != nil {
		t.Fatalf("resolve tools: %v", err)
	}
	contextHost, err := contexthost.New(contexthost.Config{Sources: []contextsource.Provider{runtimeContextFixtureSource{}}})
	if err != nil {
		t.Fatal(err)
	}
	recorder := &recordingChatModel{inner: NewScriptedModel(schema.AssistantMessage("ok", nil))}
	eng, err := NewEngine(ctx, recorder, ts, EngineConfig{
		ContextHost: contextHost, StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10, MaxContextBytes: 4096,
	})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	svc := NewService(eng, "test", "test-model", ServiceDeps{
		Journal: backend, Runs: backend, Messages: backend, Sink: newTestSink(), Truncations: backend,
	})
	runID, err := svc.Run(ctx, "sess-context-host", "find docs")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	waitForRunStatus(t, backend, runID, domain.RunCompleted)
	input := recorder.lastInput()
	found := false
	for _, message := range input {
		for _, part := range message.UserInputMultiContent {
			if strings.Contains(part.Text, "generic source body") && strings.Contains(part.Text, "fixture.docs/guide") {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("generic ContextHost candidate did not reach model input: %+v", input)
	}
}

func TestServiceRunPassesEnsuredWorkspaceIdentityToContextHost(t *testing.T) {
	ctx := context.Background()
	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "journal.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = backend.Close() })
	ts, err := tools.Builtin(backend).Resolve([]string{tools.EchoInfoName})
	if err != nil {
		t.Fatal(err)
	}
	workspaceID := make(chan string, 1)
	contextHost, err := contexthost.New(contexthost.Config{Sources: []contextsource.Provider{workspaceRecordingSource{workspaceID: workspaceID}}})
	if err != nil {
		t.Fatal(err)
	}
	eng, err := NewEngine(ctx, &recordingChatModel{inner: NewScriptedModel(schema.AssistantMessage("ok", nil))}, ts, EngineConfig{
		ContextHost: contextHost, StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10, MaxContextBytes: 4096,
	})
	if err != nil {
		t.Fatal(err)
	}
	svc := NewService(eng, "test", "test-model", ServiceDeps{
		Journal: backend, Runs: backend, Messages: backend, Sink: newTestSink(), Workspaces: fixedWorkspaceAllocator{},
	})
	runID, err := svc.Run(ctx, "sess-workspace", "workspace")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	waitForRunStatus(t, backend, runID, domain.RunCompleted)
	select {
	case got := <-workspaceID:
		if got != "workspace-actual" {
			t.Fatalf("ContextHost workspace id = %q, want ensured identity", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("ContextHost did not receive a workspace request")
	}
}

func TestBuildRunContextProjectsImageAttachments(t *testing.T) {
	stored := []domain.Message{
		{Role: domain.RoleUser, Content: "earlier", Attachments: []domain.Attachment{
			{Name: "old.png", MimeType: "image/png", Data: []byte{0xAA, 0xBB}},
		}},
		{Role: domain.RoleAssistant, Content: "sure"},
		{Role: domain.RoleUser, Content: "what is this?", Attachments: []domain.Attachment{
			{Name: "new.png", MimeType: "image/png", Data: []byte{0x01, 0x02, 0x03}},
		}},
	}
	msgs, stats, err := buildRunContext(ContextPolicy{}, "preamble", stored, "what is this?")
	if err != nil {
		t.Fatalf("buildRunContext: %v", err)
	}
	if len(msgs) != 4 {
		t.Fatalf("messages = %d, want 4 (system, history user, assistant, current user)", len(msgs))
	}
	for _, index := range []int{1, 3} {
		msg := msgs[index]
		if msg.Role != schema.User {
			t.Fatalf("message %d role = %s, want user", index, msg.Role)
		}
		if len(msg.UserInputMultiContent) != 2 {
			t.Fatalf("message %d parts = %d, want 2 (text + image)", index, len(msg.UserInputMultiContent))
		}
		textPart, imagePart := msg.UserInputMultiContent[0], msg.UserInputMultiContent[1]
		if textPart.Type != schema.ChatMessagePartTypeText || textPart.Text == "" {
			t.Fatalf("message %d text part = %+v", index, textPart)
		}
		if imagePart.Type != schema.ChatMessagePartTypeImageURL || imagePart.Image == nil {
			t.Fatalf("message %d image part = %+v", index, imagePart)
		}
		if imagePart.Image.MIMEType != "image/png" || imagePart.Image.Base64Data == nil || *imagePart.Image.Base64Data == "" {
			t.Fatalf("message %d image payload = %+v", index, imagePart.Image)
		}
		if msgs[index].Content != "" {
			t.Fatalf("message %d Content = %q, want empty (content lives in parts)", index, msgs[index].Content)
		}
	}
	// The plain assistant row stays text-only.
	if msgs[2].Content != "sure" || len(msgs[2].UserInputMultiContent) != 0 {
		t.Fatalf("assistant row = %+v", msgs[2])
	}
	// Image bytes must not count toward the text byte budget.
	if stats.Bytes > 200 {
		t.Fatalf("stats.Bytes = %d, image bytes leaked into the text budget", stats.Bytes)
	}
}

func TestBuildRunContextProjectsFileContextSnapshotsAsText(t *testing.T) {
	body := []byte("package main\n")
	stored := []domain.Message{
		{Role: domain.RoleUser, Content: "inspect", FileContexts: []domain.FileContext{{Path: "main.go", Name: "main.go", Size: int64(len(body)), Content: body}}},
		{Role: domain.RoleAssistant, Content: "found it"},
	}
	msgs, stats, err := buildRunContext(ContextPolicy{}, "preamble", stored, "found it")
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 4 || len(msgs[1].UserInputMultiContent) != 2 {
		t.Fatalf("file context projection = %+v", msgs)
	}
	part := msgs[1].UserInputMultiContent[1]
	if part.Type != schema.ChatMessagePartTypeText || !strings.Contains(part.Text, "[project file: main.go]") || !strings.Contains(part.Text, string(body)) {
		t.Fatalf("file text part = %+v", part)
	}
	if stats.Bytes < len(body) {
		t.Fatalf("file snapshot omitted from context budget: %+v", stats)
	}
}

func TestBuildRunContextRejectsFileProjectionBeyondFinalBudget(t *testing.T) {
	body := []byte("package main\n")
	stored := []domain.Message{{
		Role: domain.RoleUser, Content: "inspect",
		FileContexts: []domain.FileContext{{Path: "main.go", Name: "main.go", Size: int64(len(body)), Content: body}},
	}}
	full, _, err := buildRunContext(ContextPolicy{}, "preamble", stored, "inspect")
	if err != nil {
		t.Fatalf("unbounded context: %v", err)
	}
	budget := projectedContextBytes(full) - 1
	_, _, err = buildRunContextWithContext(context.Background(), nil, ContextPolicy{MaxBytes: budget}, "preamble", stored, "inspect")
	if !errors.Is(err, ErrContextBudgetExceeded) {
		t.Fatalf("file projection error = %v, want final context budget rejection", err)
	}
}

package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/contexthost"
	"agent-vivy/internal/domain"
	"agent-vivy/internal/observerhost"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/internal/testsupport"
	"agent-vivy/internal/tools"
	scxreference "agent-vivy/plugins/scxreference"
	"agent-vivy/sdk/port/contextsource"
)

type runtimeContextFixtureSource struct{}

func (runtimeContextFixtureSource) ID() string { return "fixture.docs" }
func (runtimeContextFixtureSource) Query(context.Context, contextsource.Request) (contextsource.Page, error) {
	return contextsource.NewPage([]contextsource.Candidate{{
		SourceID: "fixture.docs", ContentID: "guide", MediaType: "text/plain", Content: "generic source body", Confidence: 0.8,
		Metadata: map[string]string{"authority": "system"},
	}}, ""), nil
}

type requiredResourceFixtureSource struct {
	resolveErr error
}

func (requiredResourceFixtureSource) ID() string { return "fixture.required-resource" }
func (requiredResourceFixtureSource) Query(context.Context, contextsource.Request) (contextsource.Page, error) {
	reference := contextsource.ResourceReference{URI: "project://demo/plan.txt", Version: "f1", MediaType: "text/plain", VersionMode: contextsource.VersionExact, Replayable: true}
	return contextsource.NewPage([]contextsource.Candidate{{
		SourceID: "fixture.required-resource", ContentID: "plan.txt", Version: "f1", Treatment: contextsource.TreatmentRequired, Resource: &reference,
	}}, ""), nil
}
func (source requiredResourceFixtureSource) Resolve(_ context.Context, request contextsource.ResolveRequest) (contextsource.Resource, error) {
	if source.resolveErr != nil {
		return contextsource.Resource{}, source.resolveErr
	}
	return contextsource.NewResource(request.Reference, []byte("Keep strategies replaceable.")), nil
}

type workspaceRecordingSource struct {
	request chan contextsource.Request
}

func (source workspaceRecordingSource) ID() string { return "fixture.workspace" }
func (source workspaceRecordingSource) Query(_ context.Context, request contextsource.Request) (contextsource.Page, error) {
	source.request <- request
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

func TestBuildRunContextReservesAuthoritativeInstructionBytes(t *testing.T) {
	base := messageCost("preamble", "system") + messageCost("current", string(domain.RoleUser))
	latest := messageCost("latest", string(domain.RoleAssistant))
	instruction := projectedContextBytes([]*schema.Message{schema.SystemMessage("admitted instruction")})
	stored := []domain.Message{
		{Role: domain.RoleUser, Content: "old"},
		{Role: domain.RoleAssistant, Content: "latest"},
		{Role: domain.RoleUser, Content: "current"},
	}

	msgs, stats, err := buildRunContext(ContextPolicy{
		MaxBytes:      base + latest + instruction,
		ReservedBytes: instruction,
	}, "preamble", stored, "current")
	if err != nil {
		t.Fatalf("build context with reserved instruction: %v", err)
	}
	if stats.IncludedHistoryMessages != 1 || stats.DroppedHistoryMessages != 1 {
		t.Fatalf("history selection ignored reserved instruction: %+v", stats)
	}
	if len(msgs) != 3 || msgs[1].Content != "latest" {
		t.Fatalf("reserved context = %+v", msgs)
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
		if message.Role == schema.System && strings.Contains(message.Content, "generic source body") {
			t.Fatalf("Context Source metadata elevated candidate authority: %+v", message)
		}
		for _, part := range message.UserInputMultiContent {
			if strings.Contains(part.Text, "generic source body") && strings.Contains(part.Text, "fixture.docs/guide") {
				if message.Role != schema.User {
					t.Fatalf("Context Source candidate role = %s, want user data", message.Role)
				}
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("generic ContextHost candidate did not reach model input: %+v", input)
	}
}

func TestServiceRunFailsWhenRequiredContextVersionIsUnavailable(t *testing.T) {
	ctx := context.Background()
	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "journal.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = backend.Close() })
	ts, err := tools.Builtin(backend).Resolve([]string{tools.EchoInfoName})
	if err != nil {
		t.Fatal(err)
	}
	contextHost, err := contexthost.New(contexthost.Config{Sources: []contextsource.Provider{requiredResourceFixtureSource{resolveErr: contextsource.ErrVersionUnavailable}}})
	if err != nil {
		t.Fatal(err)
	}
	eng, err := NewEngine(ctx, &recordingChatModel{inner: NewScriptedModel(schema.AssistantMessage("must not run", nil))}, ts, EngineConfig{
		ContextHost: contextHost, StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10, MaxContextBytes: 4096,
	})
	if err != nil {
		t.Fatal(err)
	}
	svc := NewService(eng, "test", "test-model", ServiceDeps{Journal: backend, Runs: backend, Messages: backend, Sink: newTestSink()})
	runID, err := svc.Run(ctx, "sess-required-context", "read exact plan")
	if err != nil {
		t.Fatal(err)
	}
	waitForRunStatus(t, backend, runID, domain.RunFailed)
	events := replayAll(t, backend, runID)
	if events[len(events)-1].Type != domain.EventRunFailed || countTerminal(events) != 1 {
		t.Fatalf("events = %#v, want one run.failed terminal", events)
	}
}

func TestRunMessagesFailsRequiredSourceWhenBaseInputConsumesBudget(t *testing.T) {
	ctx := context.Background()
	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "journal.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = backend.Close() })
	ts, err := tools.Builtin(backend).Resolve([]string{tools.EchoInfoName})
	if err != nil {
		t.Fatal(err)
	}
	contextHost, err := contexthost.New(contexthost.Config{Sources: []contextsource.Provider{requiredResourceFixtureSource{}}})
	if err != nil {
		t.Fatal(err)
	}
	const userText = "fill the base input"
	preamble := composeRunPreamble(time.Now(), "", len(ts) > 0, domain.FaceWeb)
	base, _, err := buildRunContext(ContextPolicy{}, preamble, nil, userText)
	if err != nil {
		t.Fatal(err)
	}
	reserved, err := promptInstructionReservation(ctx)
	if err != nil {
		t.Fatal(err)
	}
	eng, err := NewEngine(ctx, NewScriptedModel(schema.AssistantMessage("must not run", nil)), ts, EngineConfig{
		ContextHost: contextHost, StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10, MaxContextBytes: projectedContextBytes(base) + reserved,
	})
	if err != nil {
		t.Fatal(err)
	}
	svc := NewService(eng, "test", "test-model", ServiceDeps{Journal: backend, Runs: backend, Messages: backend, Sink: newTestSink()})
	_, _, _, err = svc.runMessagesForRun(ctx, "sess-full-budget", userText, eng, domain.FaceWeb, "workspace")
	if !errors.Is(err, contexthost.ErrRequiredContextBudget) {
		t.Fatalf("required Source at zero remaining budget error = %v", err)
	}
}

func TestResumeMapperRecoversContextViewFromCommittedModelRequest(t *testing.T) {
	ctx := context.Background()
	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "journal.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = backend.Close() })
	const runID domain.RunID = "run-view-recovery"
	_, err = backend.Append(ctx, storage.Commit{RunID: runID, Events: []domain.RunEvent{{
		Type: domain.EventModelRequest, Payload: json.RawMessage(`{"context_view":"view-committed"}`),
	}}})
	if err != nil {
		t.Fatal(err)
	}
	eng := &Engine{cfg: EngineConfig{MaxEventPayloadBytes: 64 << 10}}
	svc := NewService(eng, "test", "test-model", ServiceDeps{Journal: backend})
	m := svc.newResumeEventMapper(ctx, runID)
	if m.contextViewID != "view-committed" {
		t.Fatalf("recovered Context View = %q, want view-committed", m.contextViewID)
	}
}

func TestContextViewRecoveryIsCachedPerRun(t *testing.T) {
	ctx := context.Background()
	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "journal.db"))
	if err != nil {
		t.Fatal(err)
	}
	const runID domain.RunID = "run-view-cache"
	_, err = backend.Append(ctx, storage.Commit{RunID: runID, Events: []domain.RunEvent{{
		Type: domain.EventModelRequest, Payload: json.RawMessage(`{"context_view":"view-cached"}`),
	}}})
	if err != nil {
		t.Fatal(err)
	}
	eng := &Engine{cfg: EngineConfig{MaxEventPayloadBytes: 64 << 10}}
	svc := NewService(eng, "test", "test-model", ServiceDeps{Journal: backend})
	if got := svc.contextViewForRun(ctx, runID); got != "view-cached" {
		t.Fatalf("first recovery = %q, want view-cached", got)
	}
	if err := backend.Close(); err != nil {
		t.Fatal(err)
	}
	// The cached View must survive a backend that can no longer serve
	// replays; a resume must not re-read the Journal.
	if got := svc.contextViewForRun(ctx, runID); got != "view-cached" {
		t.Fatalf("cached recovery = %q, want view-cached", got)
	}
	// A miss on the closed backend degrades to no View instead of failing.
	if got := svc.contextViewForRun(ctx, "run-view-uncached"); got != "" {
		t.Fatalf("uncached recovery = %q, want empty", got)
	}
}

type failingReplayJournal struct{}

func (failingReplayJournal) Append(context.Context, storage.Commit) (domain.EventSeq, error) {
	return 0, errors.New("append unused")
}

type failingReplayIterator struct{}

func (failingReplayIterator) Next() bool           { return false }
func (failingReplayIterator) Value() storage.Entry { return storage.Entry{} }
func (failingReplayIterator) Err() error           { return errors.New("replay failed") }
func (failingReplayIterator) Close() error         { return nil }
func (failingReplayJournal) Replay(context.Context, domain.RunID, domain.EventSeq) (storage.Iterator[storage.Entry], error) {
	return failingReplayIterator{}, nil
}

func TestContextViewRecoveryToleratesIteratorFailure(t *testing.T) {
	eng := &Engine{cfg: EngineConfig{MaxEventPayloadBytes: 64 << 10}}
	svc := NewService(eng, "test", "test-model", ServiceDeps{Journal: failingReplayJournal{}})
	if got := svc.contextViewForRun(context.Background(), "run-view-iterfail"); got != "" {
		t.Fatalf("iterator-failure recovery = %q, want empty", got)
	}
	svc.mu.Lock()
	cached := len(svc.contextViews)
	svc.mu.Unlock()
	if cached != 0 {
		t.Fatalf("iterator failure cached %d entries, want none", cached)
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
	requests := make(chan contextsource.Request, 1)
	contextHost, err := contexthost.New(contexthost.Config{Sources: []contextsource.Provider{workspaceRecordingSource{request: requests}}})
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
		Journal: backend, Runs: backend, Messages: backend, Sink: newTestSink(), Workspaces: fixedWorkspaceAllocator{}, TenantID: "tenant-actual",
	})
	runID, err := svc.Run(ctx, "sess-workspace", "workspace")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	waitForRunStatus(t, backend, runID, domain.RunCompleted)
	select {
	case got := <-requests:
		if got.TenantID != "tenant-actual" || got.WorkspaceID != "workspace-actual" || got.SessionID != "sess-workspace" {
			t.Fatalf("ContextHost scope = %#v, want tenant/workspace/session identity", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("ContextHost did not receive a workspace request")
	}
}

func TestServiceRunReturnsContextViewAndBoundedSummaryThroughCommittedObserverPath(t *testing.T) {
	ctx := context.Background()
	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "journal.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = backend.Close() })
	provider := scxreference.NewProvider()
	contextHost, err := contexthost.New(contexthost.Config{
		Sources: []contextsource.Provider{provider}, RequiredSourceIDs: []string{provider.ID()},
	})
	if err != nil {
		t.Fatal(err)
	}
	observerHost, err := observerhost.New(observerhost.Config{
		Journal: backend, Cursors: backend.Snapshot(), RetryDelay: 5 * time.Millisecond,
		RunSubscriptions: []observerhost.RunSubscription{{
			Provider: provider, EventTypes: []string{"run.completed"},
			AllowedPayloadFields: []string{"outcome", "summary", "view", "tenant_id", "workspace_id", "session_id"},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	observerHost.Start(ctx)
	t.Cleanup(observerHost.Close)
	ts, err := tools.Builtin(backend).Resolve([]string{tools.EchoInfoName})
	if err != nil {
		t.Fatal(err)
	}
	model := NewScriptedModel(schema.AssistantMessage("SCX terminal summary token sk-test-12345678901234567890", nil))
	eng, err := NewEngine(ctx, model, ts, EngineConfig{ContextHost: contextHost, StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10, MaxContextBytes: 4096})
	if err != nil {
		t.Fatal(err)
	}
	svc := NewService(eng, "test", "test-model", ServiceDeps{
		Journal: backend, Runs: backend, Messages: backend, Sink: newTestSink(), Workspaces: fixedWorkspaceAllocator{},
		TenantID: "tenant-actual", Hooks: []RunHook{observerHost},
	})
	runID, err := svc.Run(ctx, "sess-terminal-projection", "read exact plan")
	if err != nil {
		t.Fatal(err)
	}
	waitForRunStatus(t, backend, runID, domain.RunCompleted)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		updates := provider.Updates()
		if len(updates) == 0 {
			time.Sleep(5 * time.Millisecond)
			continue
		}
		if len(updates) != 1 {
			t.Fatalf("logical SCX memory updates = %d, want 1", len(updates))
		}
		var payload map[string]string
		if err := json.Unmarshal(updates[0].Payload, &payload); err != nil {
			t.Fatal(err)
		}
		if payload["outcome"] != "completed" || payload["view"] == "" || payload["tenant_id"] != "tenant-actual" || payload["workspace_id"] != "workspace-actual" || payload["session_id"] != "sess-terminal-projection" {
			t.Fatalf("terminal SCX projection = %#v", payload)
		}
		if payload["summary"] == "" || strings.Contains(payload["summary"], "sk-test-") {
			t.Fatalf("terminal summary was absent or unredacted: %q", payload["summary"])
		}
		return
	}
	t.Fatal("committed run.completed did not reach the SCX reference provider")
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

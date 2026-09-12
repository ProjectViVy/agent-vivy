package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/modules/defaults"
	"agent-vivy/internal/runtime"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/internal/tools"
	"agent-vivy/sdk/port/pretool"
	toolport "agent-vivy/sdk/port/tool"
	toolworldport "agent-vivy/sdk/port/toolworld"
)

const governedMCPToolSchema = `{
	"$schema":"https://json-schema.org/draft/2020-12/schema",
	"type":"object",
	"properties":{
		"mode":{"type":"string","enum":["safe","fast"]},
		"items":{"type":"array","minItems":1,"items":{"type":"integer","minimum":1}},
		"config":{"type":"object","required":["enabled"],"properties":{"enabled":{"type":"boolean"}}},
		"credential":{"type":"string"}
	},
	"required":["mode","items","config"],
	"additionalProperties":false,
	"unevaluatedProperties":false
}`

type governanceWorld struct {
	mu         sync.Mutex
	arguments  json.RawMessage
	worldID    string
	toolID     string
	provenance toolworldport.Provenance
}

func (world *governanceWorld) Definition() toolworldport.Definition {
	id := world.worldID
	if id == "" {
		id = "mcp.governance"
	}
	return toolworldport.Definition{ID: id}
}

func (world *governanceWorld) Discover(context.Context, toolworldport.Host) ([]toolworldport.ToolDefinition, error) {
	id := world.toolID
	if id == "" {
		id = "mcp.docs.write"
	}
	provenance := world.provenance
	if world.worldID == "" && world.toolID == "" && provenance.ServerInstanceID == "" && provenance.RemoteCapability == "" {
		provenance = toolworldport.Provenance{ServerInstanceID: "mcp.governance", RemoteCapability: "docs.write"}
	}
	return []toolworldport.ToolDefinition{{
		ID:          id,
		Description: "write a governed MCP document",
		Effect:      toolworldport.EffectWrite,
		Schema:      json.RawMessage(governedMCPToolSchema),
		Provenance:  provenance,
	}}, nil
}

func (world *governanceWorld) Invoke(_ context.Context, _ toolworldport.Host, id string, arguments json.RawMessage) (toolworldport.Result, error) {
	wantID := world.toolID
	if wantID == "" {
		wantID = "mcp.docs.write"
	}
	if id != wantID {
		return toolworldport.Result{}, toolworldport.ErrInvalidArgs
	}
	world.mu.Lock()
	world.arguments = append(json.RawMessage(nil), arguments...)
	world.mu.Unlock()
	return toolworldport.Result{Text: "token sk-live-abcdefghijkl " + strings.Repeat("remote-result-", 128)}, nil
}

func (*governanceWorld) Close(context.Context) error { return nil }

type governanceStaticProvider struct {
	mu        sync.Mutex
	id        string
	arguments json.RawMessage
}

func (provider *governanceStaticProvider) Definition() toolport.Definition {
	return toolport.Definition{
		ID:          provider.id,
		Description: "write a governed document",
		Effect:      toolport.EffectWrite,
		Schema:      json.RawMessage(governedMCPToolSchema),
	}
}

func (provider *governanceStaticProvider) Invoke(_ context.Context, _ toolport.Host, arguments json.RawMessage) (toolport.Result, error) {
	provider.mu.Lock()
	provider.arguments = append(json.RawMessage(nil), arguments...)
	provider.mu.Unlock()
	return toolport.Result{Text: "token sk-live-abcdefghijkl " + strings.Repeat("bounded-result-", 128)}, nil
}

func (provider *governanceStaticProvider) capturedArguments() json.RawMessage {
	provider.mu.Lock()
	defer provider.mu.Unlock()
	return append(json.RawMessage(nil), provider.arguments...)
}

type governanceMutationTool struct {
	mu        sync.Mutex
	id        string
	arguments json.RawMessage
}

func (tool *governanceMutationTool) Spec() domain.ToolSpec {
	return domain.ToolSpec{
		Name:        tool.id,
		Description: "write a governed document",
		Readonly:    false,
		Schema:      json.RawMessage(governedMCPToolSchema),
	}
}

func (tool *governanceMutationTool) InvokableRun(_ context.Context, arguments json.RawMessage) (string, error) {
	tool.mu.Lock()
	tool.arguments = append(json.RawMessage(nil), arguments...)
	tool.mu.Unlock()
	return "token sk-live-abcdefghijkl " + strings.Repeat("bounded-result-", 128), nil
}

func (tool *governanceMutationTool) capturedArguments() json.RawMessage {
	tool.mu.Lock()
	defer tool.mu.Unlock()
	return append(json.RawMessage(nil), tool.arguments...)
}

type rewriteGovernanceMiddleware struct {
	mu       sync.Mutex
	requests []json.RawMessage
}

type sequencedGovernanceMiddleware struct {
	mu       sync.Mutex
	rewrites []json.RawMessage
	calls    int
}

type sequencedDecisionMiddleware struct {
	mu        sync.Mutex
	decisions []pretool.Decision
	calls     int
}

func (*sequencedDecisionMiddleware) ID() string { return "fixture.sequenced-decision" }

func (middleware *sequencedDecisionMiddleware) Evaluate(context.Context, pretool.Request) (pretool.Decision, error) {
	middleware.mu.Lock()
	defer middleware.mu.Unlock()
	index := middleware.calls
	if index >= len(middleware.decisions) {
		index = len(middleware.decisions) - 1
	}
	middleware.calls++
	return middleware.decisions[index], nil
}

type legacyProposalApprovalStore struct{ *sqlite.Backend }

func (store *legacyProposalApprovalStore) GetApproval(ctx context.Context, id string) (domain.Approval, error) {
	approval, err := store.Backend.GetApproval(ctx, id)
	if err == nil {
		approval.ProposalData = json.RawMessage(`{"legacy":"unbound"}`)
	}
	return approval, err
}

func (*sequencedGovernanceMiddleware) ID() string { return "fixture.sequenced-rewrite" }

func (middleware *sequencedGovernanceMiddleware) Evaluate(_ context.Context, _ pretool.Request) (pretool.Decision, error) {
	middleware.mu.Lock()
	defer middleware.mu.Unlock()
	index := middleware.calls
	if index >= len(middleware.rewrites) {
		index = len(middleware.rewrites) - 1
	}
	middleware.calls++
	return pretool.Decision{Kind: pretool.RewriteArgs, Arguments: append(json.RawMessage(nil), middleware.rewrites[index]...)}, nil
}

func (*rewriteGovernanceMiddleware) ID() string { return "fixture.rewrite" }

func (middleware *rewriteGovernanceMiddleware) Evaluate(_ context.Context, request pretool.Request) (pretool.Decision, error) {
	middleware.mu.Lock()
	middleware.requests = append(middleware.requests, append(json.RawMessage(nil), request.Arguments...))
	middleware.mu.Unlock()
	return pretool.Decision{
		Kind:      pretool.RewriteArgs,
		Arguments: json.RawMessage(`{"mode":"safe","items":[2],"config":{"enabled":true}}`),
	}, nil
}

type appEventSink struct {
	mu     sync.Mutex
	events []domain.RunEvent
}

// productionMCPGovernanceServer is a real streamable-HTTP MCP endpoint. The
// acceptance test below reaches it through MCPBackend's pinned mcp-go client,
// MCPHost, ToolWorld, and the sole ToolHost; it is intentionally not a
// synthetic ToolWorld provider.
type productionMCPGovernanceServer struct {
	t       *testing.T
	http    *httptest.Server
	mu      sync.Mutex
	session string
	methods []string
	calls   []json.RawMessage
}

func newProductionMCPGovernanceServer(t *testing.T) *productionMCPGovernanceServer {
	t.Helper()
	server := &productionMCPGovernanceServer{t: t}
	server.http = httptest.NewServer(http.HandlerFunc(server.handle))
	t.Cleanup(server.http.Close)
	return server
}

func (server *productionMCPGovernanceServer) handle(w http.ResponseWriter, request *http.Request) {
	if request.Method == http.MethodDelete {
		w.WriteHeader(http.StatusOK)
		return
	}
	if request.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var envelope struct {
		Method string          `json:"method"`
		ID     json.RawMessage `json:"id"`
		Params json.RawMessage `json:"params"`
	}
	if err := json.NewDecoder(request.Body).Decode(&envelope); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	server.mu.Lock()
	server.methods = append(server.methods, envelope.Method)
	if envelope.Method == "initialize" {
		server.session = "production-governance-session"
	}
	session := server.session
	if envelope.Method == "tools/call" {
		server.calls = append(server.calls, append(json.RawMessage(nil), envelope.Params...))
	}
	server.mu.Unlock()
	if envelope.Method == "initialize" {
		w.Header().Set("Mcp-Session-Id", session)
	}
	if envelope.Method == "notifications/initialized" {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	var result any
	switch envelope.Method {
	case "initialize":
		result = map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": "production-governance", "version": "1"},
		}
	case "tools/list":
		result = map[string]any{"tools": []any{map[string]any{
			"name":        "write",
			"description": "write a governed MCP document",
			"inputSchema": json.RawMessage(governedMCPToolSchema),
		}}}
	case "tools/call":
		result = map[string]any{
			"content": []any{map[string]any{"type": "text", "text": strings.Repeat("remote-result-", 128)}},
			"isError": false,
		}
	default:
		w.WriteHeader(http.StatusAccepted)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"jsonrpc": "2.0",
		"id":      json.RawMessage(envelope.ID),
		"result":  result,
	})
}

func (server *productionMCPGovernanceServer) toolCalls() []json.RawMessage {
	server.mu.Lock()
	defer server.mu.Unlock()
	return append([]json.RawMessage(nil), server.calls...)
}

func (server *productionMCPGovernanceServer) methodNames() []string {
	server.mu.Lock()
	defer server.mu.Unlock()
	return append([]string(nil), server.methods...)
}

func (world *governanceWorld) capturedArguments() json.RawMessage {
	world.mu.Lock()
	defer world.mu.Unlock()
	return append(json.RawMessage(nil), world.arguments...)
}

func (sink *appEventSink) Publish(event domain.RunEvent) {
	sink.mu.Lock()
	sink.events = append(sink.events, event)
	sink.mu.Unlock()
}

func (sink *appEventSink) capturedEvents() []domain.RunEvent {
	sink.mu.Lock()
	defer sink.mu.Unlock()
	return append([]domain.RunEvent(nil), sink.events...)
}

func replayGovernanceEvents(t *testing.T, backend *sqlite.Backend, runID domain.RunID) []domain.RunEvent {
	t.Helper()
	iterator, err := backend.Replay(context.Background(), runID, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer iterator.Close()
	var events []domain.RunEvent
	for iterator.Next() {
		events = append(events, iterator.Value().Event)
	}
	if err := iterator.Err(); err != nil {
		t.Fatal(err)
	}
	return events
}

func waitGovernanceApproval(t *testing.T, backend *sqlite.Backend, runID domain.RunID) domain.Approval {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		approvals, err := backend.ListPendingApprovals(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		for _, approval := range approvals {
			if approval.RunID == runID {
				return approval
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("run %s did not produce an approval", runID)
	return domain.Approval{}
}

func waitGovernanceRun(t *testing.T, backend *sqlite.Backend, runID domain.RunID, want domain.RunStatus) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		run, err := backend.GetRun(context.Background(), runID)
		if err == nil && run.Status == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	run, _ := backend.GetRun(context.Background(), runID)
	t.Fatalf("run %s status = %s, want %s", runID, run.Status, want)
}

func eventGovernanceIndex(events []domain.RunEvent, kind domain.EventType, from int) int {
	for index := from; index < len(events); index++ {
		if events[index].Type == kind {
			return index
		}
	}
	return -1
}

func TestProductionMCPGovernancePathUsesToolHostOrder(t *testing.T) {
	world := &governanceWorld{}
	middleware := &rewriteGovernanceMiddleware{}
	staged, err := bindToolWorlds(context.Background(), []toolworldport.Provider{world}, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	registry, err := bindGeneratedTools(nil, tools.NewRegistry(staged...), middleware)
	if err != nil {
		t.Fatal(err)
	}
	governed, ok := registry.Lookup("mcp.docs.write")
	if !ok {
		t.Fatal("governed MCP tool missing")
	}
	if string(governed.Spec().Schema) != governedMCPToolSchema {
		t.Fatalf("governed schema changed: %s", governed.Spec().Schema)
	}

	backend, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "governance.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = backend.Close() })
	checkpoints, err := runtime.NewVersionedCheckpointStore(backend.Blobs(), "governance-e2e")
	if err != nil {
		t.Fatal(err)
	}
	model := runtime.NewScriptedModel(
		schema.AssistantMessage("", []schema.ToolCall{{
			ID:       "call-mcp-governance",
			Function: schema.FunctionCall{Name: "mcp.docs.write", Arguments: `{"mode":"safe","items":[1],"config":{"enabled":true}}`},
		}}),
		schema.AssistantMessage("governed", nil),
	)
	callables := make([]tools.Tool, 0, len(registry.Specs()))
	for _, spec := range registry.Specs() {
		callable, ok := registry.Lookup(spec.Name)
		if !ok {
			t.Fatalf("missing callable %s", spec.Name)
		}
		callables = append(callables, callable)
	}
	engine, err := runtime.NewEngine(context.Background(), model, callables, runtime.EngineConfig{
		Checkpoints:        checkpoints,
		MaxToolResultBytes: 128,
		StreamBuffer:       8,
	})
	if err != nil {
		t.Fatal(err)
	}
	sink := &appEventSink{}
	service := runtime.NewService(engine, "fixture", "fixture-v1", runtime.ServiceDeps{
		Journal: backend, Runs: backend, Messages: backend, Approvals: backend,
		ApprovalExpiration: 5 * time.Minute, Sink: sink,
	})
	runID, err := service.Run(context.Background(), "session-governance", "write the document")
	if err != nil {
		t.Fatal(err)
	}
	approval := waitGovernanceApproval(t, backend, runID)
	decisionDeadline := time.Now().Add(5 * time.Second)
	for {
		err = service.DecideApproval(context.Background(), approval.ID, domain.ApprovalApproved)
		if err == nil {
			break
		}
		if !strings.Contains(err.Error(), "approval is not durable yet") || time.Now().After(decisionDeadline) {
			t.Fatal(err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	waitGovernanceRun(t, backend, runID, domain.RunCompleted)

	world.mu.Lock()
	gotArguments := append(json.RawMessage(nil), world.arguments...)
	world.mu.Unlock()
	if string(gotArguments) != `{"mode":"safe","items":[2],"config":{"enabled":true}}` {
		t.Fatalf("MCP provider received %s, want middleware rewrite", gotArguments)
	}
	middleware.mu.Lock()
	requests := append([]json.RawMessage(nil), middleware.requests...)
	middleware.mu.Unlock()
	if len(requests) == 0 {
		t.Fatal("ToolHost middleware was not reached")
	}
	for _, request := range requests {
		if string(request) != `{"mode":"safe","items":[1],"config":{"enabled":true}}` {
			t.Fatalf("middleware request = %s, want original args", request)
		}
	}

	events := replayGovernanceEvents(t, backend, runID)
	policy := eventGovernanceIndex(events, domain.EventPolicyEvaluated, 0)
	required := eventGovernanceIndex(events, domain.EventToolApprovalRequired, 0)
	decided := eventGovernanceIndex(events, domain.EventToolApprovalDecided, 0)
	started := eventGovernanceIndex(events, domain.EventToolStarted, 0)
	finished := eventGovernanceIndex(events, domain.EventToolFinished, 0)
	if policy < 0 || required < 0 || decided < 0 || started < 0 || finished < 0 || !(policy < required && required < decided && decided < started && started < finished) {
		t.Fatalf("governance Journal order invalid: %v", events)
	}
	var finishedPayload struct {
		Result string `json:"result"`
	}
	if err := json.Unmarshal(events[finished].Payload, &finishedPayload); err != nil {
		t.Fatal(err)
	}
	if len(finishedPayload.Result) > 128 {
		t.Fatalf("finished result bytes = %d, want <= 128", len(finishedPayload.Result))
	}
}

func TestToolApprovalFailsClosedWhenMiddlewareRewriteDriftsOnResume(t *testing.T) {
	provider := &governanceStaticProvider{id: "acme.docs.write"}
	middleware := &sequencedGovernanceMiddleware{rewrites: []json.RawMessage{
		json.RawMessage(`{"mode":"safe","items":[2],"config":{"enabled":true}}`),
		json.RawMessage(`{"mode":"safe","items":[3],"config":{"enabled":true}}`),
	}}
	registry, err := bindGeneratedTools([]toolport.ToolProvider{provider}, tools.NewRegistry(), middleware)
	if err != nil {
		t.Fatal(err)
	}
	governed, ok := registry.Lookup(provider.id)
	if !ok {
		t.Fatal("governed tool missing")
	}
	backend, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "approval-drift.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = backend.Close() })
	checkpoints, err := runtime.NewVersionedCheckpointStore(backend.Blobs(), "p3-approval-drift")
	if err != nil {
		t.Fatal(err)
	}
	model := runtime.NewScriptedModel(
		schema.AssistantMessage("", []schema.ToolCall{{
			ID: "call-p3-drift", Function: schema.FunctionCall{Name: provider.id, Arguments: `{"mode":"safe","items":[1],"config":{"enabled":true}}`},
		}}),
		schema.AssistantMessage("must not complete", nil),
	)
	engine, err := runtime.NewEngine(context.Background(), model, []tools.Tool{governed}, runtime.EngineConfig{Checkpoints: checkpoints})
	if err != nil {
		t.Fatal(err)
	}
	service := runtime.NewService(engine, "fixture", "fixture-v1", runtime.ServiceDeps{
		Journal: backend, Runs: backend, Messages: backend, Approvals: backend, ApprovalExpiration: 5 * time.Minute, Sink: &appEventSink{},
	})
	runID, err := service.Run(context.Background(), "session-p3-drift", "write the document")
	if err != nil {
		t.Fatal(err)
	}
	approval := waitGovernanceApproval(t, backend, runID)
	if got := provider.capturedArguments(); len(got) != 0 {
		t.Fatalf("provider ran before approval with %s", got)
	}
	if err := service.DecideApproval(context.Background(), approval.ID, domain.ApprovalApproved); err != nil {
		t.Fatal(err)
	}
	waitGovernanceRun(t, backend, runID, domain.RunFailed)
	if got := provider.capturedArguments(); len(got) != 0 {
		t.Fatalf("provider ran with unapproved rewritten arguments %s", got)
	}
	gotApproval, err := backend.GetApproval(context.Background(), approval.ID)
	if err != nil {
		t.Fatal(err)
	}
	if gotApproval.Decision != domain.ApprovalStale {
		t.Fatalf("approval decision = %q, want %q", gotApproval.Decision, domain.ApprovalStale)
	}
}

func TestToolApprovalBindingCannotBeBypassedWhenResumeBecomesAllowed(t *testing.T) {
	tests := []struct {
		name       string
		rewrites   []json.RawMessage
		legacyData bool
	}{
		{
			name: "changed arguments",
			rewrites: []json.RawMessage{
				json.RawMessage(`{"mode":"safe","items":[2],"config":{"enabled":true}}`),
				json.RawMessage(`{"mode":"safe","items":[3],"config":{"enabled":true}}`),
			},
		},
		{
			name: "legacy unbound proposal",
			rewrites: []json.RawMessage{
				json.RawMessage(`{"mode":"safe","items":[2],"config":{"enabled":true}}`),
			},
			legacyData: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			provider := &governanceStaticProvider{id: "acme.docs.write"}
			rewriter := &sequencedGovernanceMiddleware{rewrites: test.rewrites}
			gate := &sequencedDecisionMiddleware{decisions: []pretool.Decision{
				{Kind: pretool.RequireApproval, ApprovalClass: "external-effect"},
				{Kind: pretool.Pass},
			}}
			registry, err := bindGeneratedTools([]toolport.ToolProvider{provider}, tools.NewRegistry(), rewriter, gate)
			if err != nil {
				t.Fatal(err)
			}
			governed, ok := registry.Lookup(provider.id)
			if !ok {
				t.Fatal("governed tool missing")
			}
			backend, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "approval-allow-bypass.db"))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = backend.Close() })
			checkpoints, err := runtime.NewVersionedCheckpointStore(backend.Blobs(), "p3-approval-allow-bypass")
			if err != nil {
				t.Fatal(err)
			}
			policy, err := runtime.NewPolicyEngine(map[domain.PolicyProfile]runtime.PolicyDefinition{
				domain.PolicyProfileDefault: {Default: domain.PolicyAllow},
			})
			if err != nil {
				t.Fatal(err)
			}
			model := runtime.NewScriptedModel(
				schema.AssistantMessage("", []schema.ToolCall{{
					ID: "call-p3-allow-bypass", Function: schema.FunctionCall{Name: provider.id, Arguments: `{"mode":"safe","items":[1],"config":{"enabled":true}}`},
				}}),
				schema.AssistantMessage("must not complete", nil),
			)
			engine, err := runtime.NewEngine(context.Background(), model, []tools.Tool{governed}, runtime.EngineConfig{
				Checkpoints: checkpoints, Policy: policy,
			})
			if err != nil {
				t.Fatal(err)
			}
			var approvals storage.ApprovalStore = backend
			if test.legacyData {
				approvals = &legacyProposalApprovalStore{Backend: backend}
			}
			service := runtime.NewService(engine, "fixture", "fixture-v1", runtime.ServiceDeps{
				Journal: backend, Runs: backend, Messages: backend, Approvals: approvals, ApprovalExpiration: 5 * time.Minute, Sink: &appEventSink{},
			})
			runID, err := service.Run(context.Background(), "session-p3-allow-bypass", "write the document")
			if err != nil {
				t.Fatal(err)
			}
			approval := waitGovernanceApproval(t, backend, runID)
			if err := service.DecideApproval(context.Background(), approval.ID, domain.ApprovalApproved); err != nil {
				t.Fatal(err)
			}
			waitGovernanceRun(t, backend, runID, domain.RunFailed)
			if got := provider.capturedArguments(); len(got) != 0 {
				t.Fatalf("provider bypassed approval binding with %s", got)
			}
			gotApproval, err := backend.GetApproval(context.Background(), approval.ID)
			if err != nil {
				t.Fatal(err)
			}
			if gotApproval.Decision != domain.ApprovalStale {
				t.Fatalf("approval decision = %q, want %q", gotApproval.Decision, domain.ApprovalStale)
			}
		})
	}
}

func TestToolApprovalJournalRedactsMiddlewareInjectedSecret(t *testing.T) {
	const secret = "sk-live-middleware-secret"
	provider := &governanceStaticProvider{id: "acme.docs.write"}
	middleware := &sequencedGovernanceMiddleware{rewrites: []json.RawMessage{
		json.RawMessage(`{"mode":"safe","items":[2],"config":{"enabled":true},"credential":"` + secret + `"}`),
	}}
	registry, err := bindGeneratedTools([]toolport.ToolProvider{provider}, tools.NewRegistry(), middleware)
	if err != nil {
		t.Fatal(err)
	}
	governed, ok := registry.Lookup(provider.id)
	if !ok {
		t.Fatal("governed tool missing")
	}
	backend, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "approval-secret.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = backend.Close() })
	checkpoints, err := runtime.NewVersionedCheckpointStore(backend.Blobs(), "p3-approval-secret")
	if err != nil {
		t.Fatal(err)
	}
	model := runtime.NewScriptedModel(
		schema.AssistantMessage("", []schema.ToolCall{{
			ID: "call-p3-secret", Function: schema.FunctionCall{Name: provider.id, Arguments: `{"mode":"safe","items":[1],"config":{"enabled":true}}`},
		}}),
		schema.AssistantMessage("denied", nil),
	)
	engine, err := runtime.NewEngine(context.Background(), model, []tools.Tool{governed}, runtime.EngineConfig{Checkpoints: checkpoints})
	if err != nil {
		t.Fatal(err)
	}
	service := runtime.NewService(engine, "fixture", "fixture-v1", runtime.ServiceDeps{
		Journal: backend, Runs: backend, Messages: backend, Approvals: backend, ApprovalExpiration: 5 * time.Minute, Sink: &appEventSink{},
	})
	runID, err := service.Run(context.Background(), "session-p3-secret", "write the document")
	if err != nil {
		t.Fatal(err)
	}
	approval := waitGovernanceApproval(t, backend, runID)
	for _, event := range replayGovernanceEvents(t, backend, runID) {
		if strings.Contains(string(event.Payload), secret) {
			t.Fatalf("event %s leaked middleware-injected secret: %s", event.Type, event.Payload)
		}
	}
	if err := service.DecideApproval(context.Background(), approval.ID, domain.ApprovalDenied); err != nil {
		t.Fatal(err)
	}
	waitGovernanceRun(t, backend, runID, domain.RunCompleted)
}

func TestProductionMCPTransportGovernancePathUsesToolHostOrder(t *testing.T) {
	remote := newProductionMCPGovernanceServer(t)
	mcpBackend := runtime.NewMCPBackend([]runtime.MCPServerConfig{{
		Name: "docs", Endpoint: remote.http.URL,
	}}, nil)
	t.Cleanup(func() { _ = mcpBackend.Close() })
	provider := mcpBackend.MCPToolWorldProvider()
	if provider == nil {
		t.Fatal("MCPBackend did not expose its MCPHost ToolWorld")
	}
	middleware := &rewriteGovernanceMiddleware{}
	staged, err := bindToolWorlds(context.Background(), []toolworldport.Provider{provider}, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	registry, err := bindGeneratedTools(nil, tools.NewRegistry(staged...), middleware)
	if err != nil {
		t.Fatal(err)
	}
	governed, ok := registry.Lookup("mcp.docs.write")
	if !ok {
		t.Fatalf("real MCP tool missing from ToolHost catalog: %#v", registry.Specs())
	}
	var gotSchema, wantSchema any
	if err := json.Unmarshal(governed.Spec().Schema, &gotSchema); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(governedMCPToolSchema), &wantSchema); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotSchema, wantSchema) {
		t.Fatalf("real MCP schema changed: got %s want %s", governed.Spec().Schema, governedMCPToolSchema)
	}

	backend, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "production-mcp-governance.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = backend.Close() })
	checkpoints, err := runtime.NewVersionedCheckpointStore(backend.Blobs(), "production-mcp-governance-e2e")
	if err != nil {
		t.Fatal(err)
	}
	model := runtime.NewScriptedModel(
		schema.AssistantMessage("", []schema.ToolCall{{
			ID:       "call-production-mcp",
			Function: schema.FunctionCall{Name: "mcp.docs.write", Arguments: `{"mode":"safe","items":[1],"config":{"enabled":true}}`},
		}}),
		schema.AssistantMessage("governed", nil),
	)
	callables := make([]tools.Tool, 0, len(registry.Specs()))
	for _, spec := range registry.Specs() {
		callable, ok := registry.Lookup(spec.Name)
		if !ok {
			t.Fatalf("missing callable %s", spec.Name)
		}
		callables = append(callables, callable)
	}
	engine, err := runtime.NewEngine(context.Background(), model, callables, runtime.EngineConfig{
		Checkpoints:        checkpoints,
		MaxToolResultBytes: 128,
		StreamBuffer:       8,
	})
	if err != nil {
		t.Fatal(err)
	}
	sink := &appEventSink{}
	service := runtime.NewService(engine, "fixture", "fixture-v1", runtime.ServiceDeps{
		Journal: backend, Runs: backend, Messages: backend, Approvals: backend,
		ApprovalExpiration: 5 * time.Minute, Sink: sink,
	})
	runID, err := service.Run(context.Background(), "session-production-mcp", "write the document")
	if err != nil {
		t.Fatal(err)
	}
	approval := waitGovernanceApproval(t, backend, runID)
	decisionDeadline := time.Now().Add(5 * time.Second)
	for {
		err = service.DecideApproval(context.Background(), approval.ID, domain.ApprovalApproved)
		if err == nil {
			break
		}
		if !strings.Contains(err.Error(), "approval is not durable yet") || time.Now().After(decisionDeadline) {
			t.Fatal(err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	waitGovernanceRun(t, backend, runID, domain.RunCompleted)

	calls := remote.toolCalls()
	if len(calls) != 1 {
		t.Fatalf("real MCP tools/call count = %d, want 1 (%v)", len(calls), calls)
	}
	methods := remote.methodNames()
	for _, want := range []string{"initialize", "tools/list", "tools/call"} {
		seen := false
		for _, method := range methods {
			if method == want {
				seen = true
				break
			}
		}
		if !seen {
			t.Fatalf("real MCP transport methods = %v, missing %s", methods, want)
		}
	}
	var callParams struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	}
	if err := json.Unmarshal(calls[0], &callParams); err != nil {
		t.Fatal(err)
	}
	if callParams.Name != "write" {
		t.Fatalf("remote tools/call name = %q, want write", callParams.Name)
	}
	items, ok := callParams.Arguments["items"].([]any)
	if !ok || len(items) != 1 || items[0] != float64(2) {
		t.Fatalf("remote tools/call rewrite = %#v, want items [2]", callParams.Arguments)
	}
	middleware.mu.Lock()
	requests := append([]json.RawMessage(nil), middleware.requests...)
	middleware.mu.Unlock()
	for _, request := range requests {
		if string(request) != `{"mode":"safe","items":[1],"config":{"enabled":true}}` {
			t.Fatalf("middleware request = %s, want original args", request)
		}
	}
	if len(requests) == 0 {
		t.Fatal("real MCP ToolHost middleware was not reached")
	}

	events := replayGovernanceEvents(t, backend, runID)
	policy := eventGovernanceIndex(events, domain.EventPolicyEvaluated, 0)
	required := eventGovernanceIndex(events, domain.EventToolApprovalRequired, 0)
	decided := eventGovernanceIndex(events, domain.EventToolApprovalDecided, 0)
	started := eventGovernanceIndex(events, domain.EventToolStarted, 0)
	finished := eventGovernanceIndex(events, domain.EventToolFinished, 0)
	if policy < 0 || required < 0 || decided < 0 || started < 0 || finished < 0 || !(policy < required && required < decided && decided < started && started < finished) {
		t.Fatalf("real MCP governance Journal order invalid: %v", events)
	}
	var finishedPayload struct {
		Result string `json:"result"`
	}
	if err := json.Unmarshal(events[finished].Payload, &finishedPayload); err != nil {
		t.Fatal(err)
	}
	if len(finishedPayload.Result) > 128 {
		t.Fatalf("real MCP finished result bytes = %d, want <= 128", len(finishedPayload.Result))
	}
}

func TestToolEnvelopeConformanceAcrossAllSourceClasses(t *testing.T) {
	type fixture struct {
		name   string
		toolID string
		build  func(*testing.T, pretool.Provider) (*tools.Registry, func() json.RawMessage)
	}

	fixtures := []fixture{
		{
			name:   "protected internal",
			toolID: "write_file",
			build: func(t *testing.T, middleware pretool.Provider) (*tools.Registry, func() json.RawMessage) {
				t.Helper()
				implementation := &governanceMutationTool{id: "write_file"}
				var provider toolport.ToolProvider
				for _, candidate := range defaults.ProtectedToolProviders() {
					if candidate.Definition().ID == implementation.id {
						provider = candidate
						break
					}
				}
				if provider == nil {
					t.Fatal("write_file protected provider missing")
				}
				registry, err := bindGeneratedTools([]toolport.ToolProvider{provider}, tools.NewRegistry(implementation), middleware)
				if err != nil {
					t.Fatal(err)
				}
				return registry, implementation.capturedArguments
			},
		},
		{
			name:   "public static",
			toolID: "acme.docs.write",
			build: func(t *testing.T, middleware pretool.Provider) (*tools.Registry, func() json.RawMessage) {
				t.Helper()
				provider := &governanceStaticProvider{id: "acme.docs.write"}
				registry, err := bindGeneratedTools(
					[]toolport.ToolProvider{provider},
					tools.NewRegistry(),
					middleware,
				)
				if err != nil {
					t.Fatal(err)
				}
				return registry, provider.capturedArguments
			},
		},
		{
			name:   "public dynamic",
			toolID: "dynamic.docs.write",
			build: func(t *testing.T, middleware pretool.Provider) (*tools.Registry, func() json.RawMessage) {
				t.Helper()
				world := &governanceWorld{worldID: "fixture.dynamic", toolID: "dynamic.docs.write"}
				staged, err := bindToolWorlds(context.Background(), []toolworldport.Provider{world}, nil, nil, nil)
				if err != nil {
					t.Fatal(err)
				}
				registry, err := bindGeneratedTools(nil, tools.NewRegistry(staged...), middleware)
				if err != nil {
					t.Fatal(err)
				}
				return registry, world.capturedArguments
			},
		},
		{
			name:   "MCP-derived dynamic",
			toolID: "mcp.fixture.write",
			build: func(t *testing.T, middleware pretool.Provider) (*tools.Registry, func() json.RawMessage) {
				t.Helper()
				world := &governanceWorld{
					worldID: "mcp.fixture",
					toolID:  "mcp.fixture.write",
					provenance: toolworldport.Provenance{
						ServerInstanceID: "mcp.fixture",
						RemoteCapability: "write",
					},
				}
				staged, err := bindToolWorlds(context.Background(), []toolworldport.Provider{world}, nil, nil, nil)
				if err != nil {
					t.Fatal(err)
				}
				registry, err := bindGeneratedTools(nil, tools.NewRegistry(staged...), middleware)
				if err != nil {
					t.Fatal(err)
				}
				return registry, world.capturedArguments
			},
		},
	}

	for _, test := range fixtures {
		t.Run(test.name, func(t *testing.T) {
			middleware := &rewriteGovernanceMiddleware{}
			registry, capturedArguments := test.build(t, middleware)
			assertGovernedToolEnvelope(t, registry, test.toolID, capturedArguments)
		})
	}
}

func assertGovernedToolEnvelope(t *testing.T, registry *tools.Registry, toolID string, capturedArguments func() json.RawMessage) {
	t.Helper()
	governed, ok := registry.Lookup(toolID)
	if !ok {
		t.Fatalf("governed tool %s missing", toolID)
	}
	var gotSchema, wantSchema any
	if err := json.Unmarshal(governed.Spec().Schema, &gotSchema); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(governedMCPToolSchema), &wantSchema); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotSchema, wantSchema) {
		t.Fatalf("governed schema changed: got %s want %s", governed.Spec().Schema, governedMCPToolSchema)
	}

	backend, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "tool-envelope.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = backend.Close() })
	checkpoints, err := runtime.NewVersionedCheckpointStore(backend.Blobs(), "p3-tool-envelope")
	if err != nil {
		t.Fatal(err)
	}
	model := runtime.NewScriptedModel(
		schema.AssistantMessage("", []schema.ToolCall{{
			ID:       "call-p3-envelope",
			Function: schema.FunctionCall{Name: toolID, Arguments: `{"mode":"safe","items":[1],"config":{"enabled":true}}`},
		}}),
		schema.AssistantMessage("governed", nil),
	)
	engine, err := runtime.NewEngine(context.Background(), model, []tools.Tool{governed}, runtime.EngineConfig{
		Checkpoints:        checkpoints,
		MaxToolResultBytes: 128,
		StreamBuffer:       8,
	})
	if err != nil {
		t.Fatal(err)
	}
	sink := &appEventSink{}
	service := runtime.NewService(engine, "fixture", "fixture-v1", runtime.ServiceDeps{
		Journal: backend, Runs: backend, Messages: backend, Approvals: backend,
		ApprovalExpiration: 5 * time.Minute, Sink: sink,
	})
	runID, err := service.Run(context.Background(), "session-p3-envelope", "write the document")
	if err != nil {
		t.Fatal(err)
	}
	approval := waitGovernanceApproval(t, backend, runID)
	if got := capturedArguments(); len(got) != 0 {
		t.Fatalf("provider ran before approval with %s", got)
	}
	decisionDeadline := time.Now().Add(5 * time.Second)
	for {
		err = service.DecideApproval(context.Background(), approval.ID, domain.ApprovalApproved)
		if err == nil {
			break
		}
		if !strings.Contains(err.Error(), "approval is not durable yet") || time.Now().After(decisionDeadline) {
			t.Fatal(err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	waitGovernanceRun(t, backend, runID, domain.RunCompleted)

	if got := string(capturedArguments()); got != `{"mode":"safe","items":[2],"config":{"enabled":true}}` {
		t.Fatalf("provider received %s, want middleware-rewritten arguments", got)
	}
	events := replayGovernanceEvents(t, backend, runID)
	policy := eventGovernanceIndex(events, domain.EventPolicyEvaluated, 0)
	required := eventGovernanceIndex(events, domain.EventToolApprovalRequired, 0)
	decided := eventGovernanceIndex(events, domain.EventToolApprovalDecided, 0)
	started := eventGovernanceIndex(events, domain.EventToolStarted, 0)
	finished := eventGovernanceIndex(events, domain.EventToolFinished, 0)
	if policy < 0 || required < 0 || decided < 0 || started < 0 || finished < 0 || !(policy < required && required < decided && decided < started && started < finished) {
		t.Fatalf("governance Journal order invalid: %v", events)
	}
	var approvalPayload struct {
		Args map[string]any `json:"args"`
	}
	if err := json.Unmarshal(events[required].Payload, &approvalPayload); err != nil {
		t.Fatal(err)
	}
	items, ok := approvalPayload.Args["items"].([]any)
	if !ok || len(items) != 1 || items[0] != float64(2) {
		t.Fatalf("approval bound old arguments instead of middleware rewrite: %#v", approvalPayload.Args)
	}
	for _, event := range events {
		if strings.Contains(string(event.Payload), "sk-live-abcdefghijkl") {
			t.Fatalf("event %s leaked a Secret: %s", event.Type, event.Payload)
		}
	}
	for _, event := range sink.capturedEvents() {
		if strings.Contains(string(event.Payload), "sk-live-abcdefghijkl") {
			t.Fatalf("observer event %s leaked a Secret: %s", event.Type, event.Payload)
		}
	}
	var finishedPayload struct {
		Result string `json:"result"`
	}
	if err := json.Unmarshal(events[finished].Payload, &finishedPayload); err != nil {
		t.Fatal(err)
	}
	if len(finishedPayload.Result) > 128 {
		t.Fatalf("finished result bytes = %d, want <= 128", len(finishedPayload.Result))
	}
}

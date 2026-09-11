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
	"agent-vivy/internal/runtime"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/internal/tools"
	"agent-vivy/sdk/port/pretool"
	toolworldport "agent-vivy/sdk/port/toolworld"
)

const governedMCPToolSchema = `{
	"$schema":"https://json-schema.org/draft/2020-12/schema",
	"type":"object",
	"properties":{
		"mode":{"type":"string","enum":["safe","fast"]},
		"items":{"type":"array","minItems":1,"items":{"type":"integer","minimum":1}},
		"config":{"type":"object","required":["enabled"],"properties":{"enabled":{"type":"boolean"}}}
	},
	"required":["mode","items","config"],
	"additionalProperties":false,
	"unevaluatedProperties":false
}`

type governanceWorld struct {
	mu        sync.Mutex
	arguments json.RawMessage
}

func (*governanceWorld) Definition() toolworldport.Definition {
	return toolworldport.Definition{ID: "mcp.governance"}
}

func (*governanceWorld) Discover(context.Context, toolworldport.Host) ([]toolworldport.ToolDefinition, error) {
	return []toolworldport.ToolDefinition{{
		ID:          "mcp.docs.write",
		Description: "write a governed MCP document",
		Effect:      toolworldport.EffectWrite,
		Schema:      json.RawMessage(governedMCPToolSchema),
		Provenance: toolworldport.Provenance{
			ServerInstanceID: "mcp.governance",
			RemoteCapability: "docs.write",
		},
	}}, nil
}

func (world *governanceWorld) Invoke(_ context.Context, _ toolworldport.Host, id string, arguments json.RawMessage) (toolworldport.Result, error) {
	if id != "mcp.docs.write" {
		return toolworldport.Result{}, toolworldport.ErrInvalidArgs
	}
	world.mu.Lock()
	world.arguments = append(json.RawMessage(nil), arguments...)
	world.mu.Unlock()
	return toolworldport.Result{Text: strings.Repeat("remote-result-", 128)}, nil
}

func (*governanceWorld) Close(context.Context) error { return nil }

type rewriteGovernanceMiddleware struct {
	mu       sync.Mutex
	requests []json.RawMessage
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

func (sink *appEventSink) Publish(event domain.RunEvent) {
	sink.mu.Lock()
	sink.events = append(sink.events, event)
	sink.mu.Unlock()
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

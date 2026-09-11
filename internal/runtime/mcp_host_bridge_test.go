package runtime

import (
	"context"
	"errors"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/contexthost"
	"agent-vivy/internal/domain"
	"agent-vivy/internal/mcphost"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/internal/tools"
)

func waitForMCPDeletes(t *testing.T, server *testMCPServer, want int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		deletes, _, _ := server.counts()
		if deletes == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	deletes, _, _ := server.counts()
	t.Fatalf("MCP transport DELETE count=%d, want %d", deletes, want)
}

func TestMCPHostCloseOwnsInitializedHTTPTransport(t *testing.T) {
	server := newTestMCPServer(t, simpleToolsDispatch("owned"))
	backend := NewMCPBackend([]MCPServerConfig{{Name: "docs", Endpoint: server.http.URL}}, server.http.Client())
	provider := backend.MCPToolWorldProvider()
	if provider == nil {
		t.Fatal("MCP ToolWorld provider is nil")
	}
	if _, err := provider.Discover(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if err := provider.Close(context.Background()); err != nil {
		t.Fatalf("close MCPHost provider: %v", err)
	}
	waitForMCPDeletes(t, server, 1)
	if err := provider.Close(context.Background()); err != nil {
		t.Fatalf("second close MCPHost provider: %v", err)
	}
	waitForMCPDeletes(t, server, 1)
	if err := backend.Close(); err != nil {
		t.Fatalf("close MCP configuration facade: %v", err)
	}
	waitForMCPDeletes(t, server, 1)
}

func TestMCPHostReplaceOwnsRetiredHTTPTransport(t *testing.T) {
	oldServer := newTestMCPServer(t, simpleToolsDispatch("old"))
	newServer := newTestMCPServer(t, simpleToolsDispatch("new"))
	backend := NewMCPBackend([]MCPServerConfig{{Name: "docs", Endpoint: oldServer.http.URL}}, oldServer.http.Client())
	provider := backend.MCPToolWorldProvider()
	if provider == nil {
		t.Fatal("MCP ToolWorld provider is nil")
	}
	if _, err := provider.Discover(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	backend.ReplaceServers([]MCPServerConfig{{Name: "docs", Endpoint: newServer.http.URL}})
	waitForMCPDeletes(t, oldServer, 1)
	if old, _, _ := oldServer.counts(); old != 1 {
		t.Fatalf("retired HTTP transport DELETE count=%d, want exactly once", old)
	}
	if _, err := provider.Discover(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if err := provider.Close(context.Background()); err != nil {
		t.Fatalf("close replacement MCPHost provider: %v", err)
	}
	waitForMCPDeletes(t, newServer, 1)
	if err := backend.Close(); err != nil {
		t.Fatalf("close MCP configuration facade: %v", err)
	}
	waitForMCPDeletes(t, oldServer, 1)
	waitForMCPDeletes(t, newServer, 1)
}

func TestMCPResourceContextHostRequiresExplicitInstanceOptIn(t *testing.T) {
	without := NewMCPBackend([]MCPServerConfig{{Name: "docs", Endpoint: "https://example.invalid/mcp"}}, nil)
	t.Cleanup(func() { _ = without.Close() })
	contextHost, err := without.MCPResourceContextHost()
	if err != nil {
		t.Fatal(err)
	}
	if contextHost != nil {
		t.Fatal("MCP resources unexpectedly activated ContextHost without resource_bridge")
	}

	with := NewMCPBackend([]MCPServerConfig{{Name: "docs", Endpoint: "https://example.invalid/mcp", ResourceBridge: true}}, nil)
	t.Cleanup(func() { _ = with.Close() })
	if contextHost, err := with.MCPResourceContextHost(); err != nil || contextHost != nil {
		t.Fatalf("resource bridge bypassed missing compiled-host signal: host=%v err=%v", contextHost, err)
	}
	contextHost, err = with.MCPResourceContextHost(true)
	if err != nil {
		t.Fatal(err)
	}
	if contextHost == nil {
		t.Fatal("explicit resource_bridge did not instantiate ContextHost")
	}
}

func TestMCPToolWorldProviderTracksLiveBackendReplacement(t *testing.T) {
	oldServer := newTestMCPServer(t, simpleToolsDispatch("old"))
	newServer := newTestMCPServer(t, simpleToolsDispatch("new"))
	backend := NewMCPBackend([]MCPServerConfig{{Name: "docs", Endpoint: oldServer.http.URL}}, oldServer.http.Client())
	t.Cleanup(func() { _ = backend.Close() })

	provider := backend.MCPToolWorldProvider()
	if provider == nil {
		t.Fatal("MCP ToolWorld provider is nil")
	}
	first, err := provider.Discover(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 1 || first[0].ID != "mcp.docs.old" {
		t.Fatalf("initial MCP projection = %#v", first)
	}

	backend.ReplaceServers([]MCPServerConfig{{Name: "docs", Endpoint: newServer.http.URL}})
	second, err := provider.Discover(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(second) != 1 || second[0].ID != "mcp.docs.new" {
		t.Fatalf("replacement MCP projection = %#v", second)
	}
	if _, err := provider.Invoke(context.Background(), nil, "mcp.docs.old", nil); !errors.Is(err, mcphost.ErrUnknownTool) {
		t.Fatalf("old MCP projection invoke error = %v, want ErrUnknownTool", err)
	}
}

func TestMCPHostHTTPSessionTerminationRecoversUnderHostRetry(t *testing.T) {
	var listCalls int
	server := newTestMCPServer(t, func(request testMCPRequest) testMCPResponse {
		switch request.Method {
		case "initialize":
			return testMCPResponse{Status: http.StatusAccepted, Result: initializeResult(map[string]any{"tools": map[string]any{}})}
		case "tools/list":
			listCalls++
			if listCalls == 1 {
				// mcp-go maps this HTTP 404 to ErrSessionTerminated. The
				// adapter must return that one failed operation; MCPHost owns
				// the bounded retry and asks the shared backend for a fresh
				// HTTP session before trying discovery again.
				return testMCPResponse{Status: http.StatusNotFound}
			}
			return testMCPResponse{Result: map[string]any{"tools": []map[string]any{{
				"name": "recovered", "inputSchema": map[string]any{"type": "object"},
			}}}}
		default:
			return testMCPResponse{NoBody: true}
		}
	})
	backend := NewMCPBackend([]MCPServerConfig{{Name: "docs", Endpoint: server.http.URL}}, server.http.Client())
	t.Cleanup(func() { _ = backend.Close() })
	provider := backend.MCPToolWorldProvider()
	if provider == nil {
		t.Fatal("MCP ToolWorld provider is nil")
	}
	projected, err := provider.Discover(context.Background(), nil)
	if err != nil {
		t.Fatalf("MCPHost discovery did not recover from HTTP session termination: %v", err)
	}
	if len(projected) != 1 || projected[0].ID != "mcp.docs.recovered" {
		t.Fatalf("recovered projection = %#v", projected)
	}
	if listCalls != 2 {
		t.Fatalf("tools/list calls = %d, want one failed operation plus one MCPHost retry", listCalls)
	}
	methods := serverMethods(server)
	initializeCalls := 0
	for _, method := range methods {
		if method == "initialize" {
			initializeCalls++
		}
	}
	if initializeCalls != 2 {
		t.Fatalf("initialize calls = %d, want a fresh HTTP session after 404", initializeCalls)
	}
}

func TestRetiredMCPHostGenerationCannotMarkReplacementReady(t *testing.T) {
	oldServer := newTestMCPServer(t, simpleToolsDispatch("old"))
	newServer := newTestMCPServer(t, simpleToolsDispatch("new"))
	backend := NewMCPBackend([]MCPServerConfig{{Name: "docs", Endpoint: oldServer.http.URL}}, oldServer.http.Client())
	factory := newMCPHostSessionFactory(backend)
	statusForConfig := factory.status
	statusLookupStarted := make(chan struct{})
	releaseStatusLookup := make(chan struct{})
	factory.status = func(config MCPServerConfig) *mcpServer {
		close(statusLookupStarted)
		<-releaseStatusLookup
		return statusForConfig(config)
	}
	host, err := mcphost.New(mcphost.Config{
		Factory:           factory,
		Instances:         []mcphost.InstanceConfig{{ID: "docs", Endpoint: oldServer.http.URL}},
		MaxSafeRetries:    0,
		MaxSafeRetriesSet: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	result := make(chan error, 1)
	go func() {
		_, discoverErr := host.DiscoverTools(context.Background(), "docs")
		result <- discoverErr
	}()
	<-statusLookupStarted

	// ReplaceServers publishes the new status record before the old Host
	// generation is retired. The blocked factory.Open is the exact interleaving
	// where an ID-only lookup would attach the old transport to the new record.
	backend.ReplaceServers([]MCPServerConfig{{Name: "docs", Endpoint: newServer.http.URL}})
	close(releaseStatusLookup)
	if err := <-result; err != nil {
		t.Fatalf("old-generation discovery error = %v", err)
	}
	statuses := backend.ServerStatuses()
	if len(statuses) != 1 || statuses[0].Initialized || statuses[0].ToolCount != -1 {
		t.Fatalf("replacement status was marked by retired transport: %+v", statuses)
	}
	if err := host.Close(); err != nil {
		t.Fatalf("close old MCPHost generation: %v", err)
	}
	if err := backend.Close(); err != nil {
		t.Fatalf("close MCP backend: %v", err)
	}
}

func serverMethods(server *testMCPServer) []string {
	server.mu.Lock()
	defer server.mu.Unlock()
	return append([]string(nil), server.methods...)
}

func TestMCPResourceBridgeReachesProductionServiceModelInput(t *testing.T) {
	server := newTestMCPServer(t, func(request testMCPRequest) testMCPResponse {
		switch request.Method {
		case "initialize":
			return testMCPResponse{Status: http.StatusAccepted, Result: initializeResult(map[string]any{"resources": map[string]any{}})}
		case "resources/list":
			return testMCPResponse{Result: map[string]any{"resources": []map[string]any{{"uri": "docs://guide", "name": "guide", "description": "remote guide", "mimeType": "text/plain"}}}}
		case "resources/read":
			return testMCPResponse{Result: map[string]any{"contents": []map[string]any{{"uri": "docs://guide", "mimeType": "text/plain", "text": "production MCP resource body"}}}}
		default:
			return testMCPResponse{NoBody: true}
		}
	})
	backend := NewMCPBackend([]MCPServerConfig{{Name: "docs", Endpoint: server.http.URL, ResourceBridge: true}}, server.http.Client())
	t.Cleanup(func() { _ = backend.Close() })
	contextHost, err := backend.MCPResourceContextHost(true)
	if err != nil {
		t.Fatal(err)
	}
	if contextHost == nil {
		t.Fatal("explicit resource bridge did not produce ContextHost")
	}
	direct, err := contextHost.Query(context.Background(), contexthost.Request{Query: "guide"})
	if err != nil || len(direct.Candidates) != 1 {
		t.Fatalf("direct MCP ContextHost query candidates=%#v failures=%#v err=%v", direct.Candidates, direct.Failures, err)
	}
	if parts := contextCandidatesToUserParts(direct.Candidates); len(parts) != 1 {
		t.Fatalf("MCP candidates failed runtime projection: %#v", direct.Candidates)
	}

	journal, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "journal.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = journal.Close() })
	toolsForRun, err := tools.Builtin(journal).Resolve([]string{tools.EchoInfoName})
	if err != nil {
		t.Fatal(err)
	}
	recorder := &recordingChatModel{inner: NewScriptedModel(schema.AssistantMessage("ok", nil))}
	engine, err := NewEngine(context.Background(), recorder, toolsForRun, EngineConfig{
		ContextHost: contextHost, StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10, MaxContextBytes: 4096,
	})
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(engine, "test", "test-model", ServiceDeps{
		Journal: journal, Runs: journal, Messages: journal, Sink: newTestSink(), Truncations: journal,
	})
	runID, err := service.Run(context.Background(), "mcp-resource-session", "guide")
	if err != nil {
		t.Fatal(err)
	}
	waitForRunStatus(t, journal, runID, domain.RunCompleted)
	found := false
	for _, message := range recorder.lastInput() {
		for _, part := range message.UserInputMultiContent {
			if strings.Contains(part.Text, "production MCP resource body") && strings.Contains(part.Text, "docs://guide") {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("MCP Resource candidate did not reach model input: %+v", recorder.lastInput())
	}
}

func TestMCPResourceBridgeDisabledAndLiveToggle(t *testing.T) {
	backend := NewMCPBackend([]MCPServerConfig{{Name: "docs", Endpoint: "https://example.invalid/mcp"}}, nil)
	t.Cleanup(func() { _ = backend.Close() })
	without, err := backend.MCPResourceContextHost()
	if err != nil {
		t.Fatal(err)
	}
	if without != nil {
		t.Fatal("default MCP config unexpectedly produced a ContextHost")
	}

	backend.ReplaceServers([]MCPServerConfig{{Name: "docs", Endpoint: "https://example.invalid/mcp", ResourceBridge: true}})
	with, err := backend.MCPResourceContextHost(true)
	if err != nil {
		t.Fatal(err)
	}
	if with == nil {
		t.Fatal("resource_bridge change did not produce a ContextHost")
	}

	backend.ReplaceServers([]MCPServerConfig{{Name: "docs", Endpoint: "https://example.invalid/mcp"}})
	withoutAgain, err := backend.MCPResourceContextHost(true)
	if err != nil {
		t.Fatal(err)
	}
	if withoutAgain != nil {
		t.Fatal("disabling resource bridge left ContextHost active")
	}
}

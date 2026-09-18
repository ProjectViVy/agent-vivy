package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"agent-vivy/internal/config"
	"agent-vivy/internal/runtime"
)

type liveMCPServer struct {
	t       *testing.T
	http    *httptest.Server
	mu      sync.Mutex
	session string
	seq     int
	tool    string
}

func newLiveMCPServer(t *testing.T, tool ...string) *liveMCPServer {
	t.Helper()
	server := &liveMCPServer{t: t}
	if len(tool) > 0 {
		server.tool = tool[0]
	}
	server.http = httptest.NewServer(http.HandlerFunc(server.handle))
	t.Cleanup(server.http.Close)
	return server
}

func (server *liveMCPServer) handle(w http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var envelope struct {
		Method string          `json:"method"`
		ID     json.RawMessage `json:"id"`
	}
	if err := json.NewDecoder(request.Body).Decode(&envelope); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	server.mu.Lock()
	if envelope.Method == "initialize" {
		server.seq++
		server.session = fmt.Sprintf("live-session-%d", server.seq)
	}
	session := server.session
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
			"capabilities":    map[string]any{"tools": map[string]any{}, "resources": map[string]any{}},
			"serverInfo":      map[string]any{"name": "live-fixture", "version": "1"},
		}
	case "tools/list":
		tools := []any{}
		if server.tool != "" {
			tools = append(tools, map[string]any{
				"name":        server.tool,
				"description": "live replacement tool",
				"inputSchema": map[string]any{"type": "object"},
			})
		}
		result = map[string]any{"tools": tools}
	case "resources/list":
		result = map[string]any{"resources": []map[string]any{{
			"uri": "docs://guide", "name": "guide", "description": "production guide", "mimeType": "text/plain",
		}}}
	case "resources/read":
		result = map[string]any{"contents": []map[string]any{{
			"uri": "docs://guide", "mimeType": "text/plain", "text": "production MCP resource body",
		}}}
	default:
		w.WriteHeader(http.StatusAccepted)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": json.RawMessage(envelope.ID), "result": result})
}

type deepseekInputServer struct {
	http   *httptest.Server
	mu     sync.Mutex
	bodies []string
}

func newDeepSeekInputServer(t *testing.T) *deepseekInputServer {
	t.Helper()
	server := &deepseekInputServer{}
	server.http = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		raw, _ := io.ReadAll(request.Body)
		server.mu.Lock()
		server.bodies = append(server.bodies, string(raw))
		server.mu.Unlock()
		if strings.Contains(string(raw), `"stream":true`) {
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = fmt.Fprint(w, "data: {\"id\":\"chatcmpl-live\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"deepseek-flash\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"ok\"},\"finish_reason\":null}]}\n\n")
			_, _ = fmt.Fprint(w, "data: {\"id\":\"chatcmpl-live\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"deepseek-flash\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":1,\"completion_tokens\":1,\"total_tokens\":2}}\n\n")
			_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"id":"chatcmpl-live","object":"chat.completion","created":1,"model":"deepseek-flash","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`)
	}))
	t.Cleanup(server.http.Close)
	return server
}

func (server *deepseekInputServer) containsBody(want string) bool {
	server.mu.Lock()
	defer server.mu.Unlock()
	for _, body := range server.bodies {
		if strings.Contains(body, want) {
			return true
		}
	}
	return false
}

func (server *deepseekInputServer) containsBodyAfter(want string, offset int) bool {
	server.mu.Lock()
	defer server.mu.Unlock()
	if offset < 0 {
		offset = 0
	}
	if offset >= len(server.bodies) {
		return false
	}
	for _, body := range server.bodies[offset:] {
		if strings.Contains(body, want) {
			return true
		}
	}
	return false
}

func (server *deepseekInputServer) containsToolName(want string) bool {
	server.mu.Lock()
	defer server.mu.Unlock()
	for _, body := range server.bodies {
		var envelope struct {
			Tools []struct {
				Function struct {
					Name string `json:"name"`
				} `json:"function"`
			} `json:"tools"`
		}
		if json.Unmarshal([]byte(body), &envelope) != nil {
			continue
		}
		for _, tool := range envelope.Tools {
			if tool.Function.Name == want {
				return true
			}
		}
	}
	return false
}

func (server *deepseekInputServer) containsToolNameAfter(want string, offset int) bool {
	server.mu.Lock()
	defer server.mu.Unlock()
	if offset < 0 {
		offset = 0
	}
	if offset >= len(server.bodies) {
		return false
	}
	for _, body := range server.bodies[offset:] {
		var envelope struct {
			Tools []struct {
				Function struct {
					Name string `json:"name"`
				} `json:"function"`
			} `json:"tools"`
		}
		if json.Unmarshal([]byte(body), &envelope) != nil {
			continue
		}
		for _, tool := range envelope.Tools {
			if tool.Function.Name == want {
				return true
			}
		}
	}
	return false
}

func (server *deepseekInputServer) bodyCount() int {
	server.mu.Lock()
	defer server.mu.Unlock()
	return len(server.bodies)
}

func TestProductionMCPResourceBridgeSettingsRebuildsModelInput(t *testing.T) {
	runtime.SetEngineVersionOverride(pinnedEinoVersion)
	t.Cleanup(func() { runtime.SetEngineVersionOverride("") })
	t.Setenv("DEEPSEEK_API_KEY", "mcp-live-test-key")
	mcpServer := newLiveMCPServer(t)
	modelServer := newDeepSeekInputServer(t)
	t.Setenv("VIVY_API_BASE", modelServer.http.URL)

	cfg := newDeepSeekTestConfig(t)
	cfg.Runtime.MCPServers = []config.MCPServer{{Name: "docs", Endpoint: mcpServer.http.URL}}
	cfg.Runtime.MaxContextBytes = 16 << 10
	a, err := New(context.Background(), cfg, WithoutEars(), WithoutGateway(), WithSettingsPath(filepath.Join(t.TempDir(), "settings.yaml")))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = a.Close() })
	if contextHost, err := a.mcpBackend.MCPResourceContextHost(); err != nil || contextHost != nil {
		t.Fatalf("default resource bridge context host=%v err=%v, want absent", contextHost, err)
	}

	client, err := a.DialControl(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	session := callControl(t, client, "session/create", map[string]any{"title": "mcp-live"})
	sessionID, _ := session["id"].(string)
	if sessionID == "" {
		t.Fatalf("session/create = %v", session)
	}

	callControl(t, client, "settings/mcp/upsert", map[string]any{
		"name": "docs", "endpoint": mcpServer.http.URL, "resource_bridge": true,
	})
	waitFor(t, 5*time.Second, func() bool {
		contextHost, err := a.mcpBackend.MCPResourceContextHost(true)
		return err == nil && contextHost != nil
	})
	turn := callControl(t, client, "turn/start", map[string]any{"session_id": sessionID, "text": "guide"})
	waitForAppRun(t, client, turn)
	if !modelServer.containsBody("production MCP resource body") {
		t.Fatal("bridge-only MCP settings change did not rebuild ContextHost model input")
	}
}

func TestProductionMCPReplacementRebuildsEngineWithoutStaleTool(t *testing.T) {
	runtime.SetEngineVersionOverride(pinnedEinoVersion)
	t.Cleanup(func() { runtime.SetEngineVersionOverride("") })
	t.Setenv("DEEPSEEK_API_KEY", "mcp-live-test-key")
	oldMCP := newLiveMCPServer(t, "old")
	newMCP := newLiveMCPServer(t, "new")
	modelServer := newDeepSeekInputServer(t)
	t.Setenv("VIVY_API_BASE", modelServer.http.URL)

	cfg := newDeepSeekTestConfig(t)
	cfg.Runtime.MCPServers = []config.MCPServer{{Name: "docs", Endpoint: oldMCP.http.URL}}
	cfg.Tools.Enabled = []string{"mcp.docs.old"}
	a, err := New(context.Background(), cfg, WithoutEars(), WithoutGateway(), WithSettingsPath(filepath.Join(t.TempDir(), "settings.yaml")))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = a.Close() })
	client, err := a.DialControl(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	session := callControl(t, client, "session/create", map[string]any{"title": "mcp-replace"})
	sessionID, _ := session["id"].(string)
	if sessionID == "" {
		t.Fatalf("session/create = %v", session)
	}
	first := callControl(t, client, "turn/start", map[string]any{"session_id": sessionID, "text": "old"})
	waitForAppRun(t, client, first)
	if !modelServer.containsBody("mcp.docs.old") {
		t.Fatal("initial model request did not expose the discovered old MCP tool")
	}
	baselineBodies := modelServer.bodyCount()

	callControl(t, client, "settings/mcp/upsert", map[string]any{
		"name": "docs", "endpoint": newMCP.http.URL,
	})
	second := callControl(t, client, "turn/start", map[string]any{"session_id": sessionID, "text": "new"})
	waitForAppRun(t, client, second)
	waitFor(t, 5*time.Second, func() bool {
		return modelServer.containsBodyAfter("mcp.docs.new", baselineBodies)
	})
	if modelServer.containsBodyAfter("mcp.docs.old", baselineBodies) {
		t.Fatal("replacement left the old MCP tool executable in the rebuilt engine")
	}
	listed := callControl(t, client, "tools/list", nil)
	encoded, err := json.Marshal(listed)
	if err != nil {
		t.Fatal(err)
	}
	var view struct {
		Active []string `json:"active"`
	}
	if err := json.Unmarshal(encoded, &view); err != nil {
		t.Fatalf("decode tools/list result: %v", err)
	}
	for _, name := range view.Active {
		if name == "mcp.docs.old" {
			t.Fatal("tools/list retained retired MCP tool as active")
		}
	}
	foundNew := false
	for _, name := range view.Active {
		if name == "mcp.docs.new" {
			foundNew = true
		}
	}
	if !foundNew {
		t.Fatalf("tools/list did not reconcile active replacement: %#v", view.Active)
	}
}

func TestProductionMCPToolCatalogRefreshIsWholeDuringConcurrentRPC(t *testing.T) {
	runtime.SetEngineVersionOverride(pinnedEinoVersion)
	t.Cleanup(func() { runtime.SetEngineVersionOverride("") })
	t.Setenv("DEEPSEEK_API_KEY", "mcp-live-test-key")
	oldMCP := newLiveMCPServer(t, "old")
	newMCP := newLiveMCPServer(t, "new")
	modelServer := newDeepSeekInputServer(t)
	t.Setenv("VIVY_API_BASE", modelServer.http.URL)

	cfg := newDeepSeekTestConfig(t)
	cfg.Runtime.MCPServers = []config.MCPServer{{Name: "docs", Endpoint: oldMCP.http.URL}}
	a, err := New(context.Background(), cfg, WithoutEars(), WithoutGateway(), WithSettingsPath(filepath.Join(t.TempDir(), "settings.yaml")))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = a.Close() })
	client, err := a.DialControl(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })

	failures := make(chan error, 128)
	var group sync.WaitGroup
	group.Add(1)
	go func() {
		defer group.Done()
		for index := 0; index < 80; index++ {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			raw, callErr := client.Call(ctx, "tools/list", nil)
			cancel()
			if callErr != nil {
				failures <- callErr
				continue
			}
			var view struct {
				Tools []struct {
					Name string `json:"name"`
				} `json:"tools"`
			}
			if decodeErr := json.Unmarshal(raw, &view); decodeErr != nil {
				failures <- decodeErr
				continue
			}
			oldSeen, newSeen := false, false
			for _, tool := range view.Tools {
				switch tool.Name {
				case "mcp.docs.old":
					oldSeen = true
				case "mcp.docs.new":
					newSeen = true
				}
			}
			if oldSeen && newSeen {
				failures <- fmt.Errorf("tools/list mixed old and new MCP snapshots")
			}
		}
	}()
	group.Add(1)
	go func() {
		defer group.Done()
		for index := 0; index < 8; index++ {
			endpoint := oldMCP.http.URL
			if index%2 == 1 {
				endpoint = newMCP.http.URL
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			_, callErr := client.Call(ctx, "settings/mcp/upsert", map[string]any{
				"name": "docs", "endpoint": endpoint,
			})
			cancel()
			if callErr != nil {
				failures <- callErr
			}
		}
	}()
	group.Wait()
	close(failures)
	var observed []error
	for err := range failures {
		observed = append(observed, err)
	}
	if len(observed) > 0 {
		t.Fatalf("concurrent MCP catalog refresh produced %d errors; first=%v", len(observed), observed[0])
	}
	final := callControl(t, client, "tools/list", nil)
	finalTools, ok := final["tools"].([]any)
	if !ok {
		t.Fatalf("final tools/list = %#v; tools array missing", final)
	}
	oldSeen, newSeen := false, false
	for _, rawTool := range finalTools {
		tool, ok := rawTool.(map[string]any)
		if !ok {
			continue
		}
		switch tool["name"] {
		case "mcp.docs.old":
			oldSeen = true
		case "mcp.docs.new":
			newSeen = true
		}
	}
	if !newSeen || oldSeen {
		t.Fatalf("final tools/list MCP projection old=%v new=%v; want only new", oldSeen, newSeen)
	}
}

func waitForAppRun(t *testing.T, client interface {
	Call(context.Context, string, any) (json.RawMessage, error)
}, result map[string]any) {
	t.Helper()
	runID, _ := result["run_id"].(string)
	if runID == "" {
		t.Fatalf("turn/start = %v", result)
	}
	waitFor(t, 10*time.Second, func() bool {
		run, err := client.Call(context.Background(), "run/get", map[string]any{"run_id": runID})
		if err != nil {
			return false
		}
		var view map[string]any
		if json.Unmarshal(run, &view) != nil {
			return false
		}
		status, _ := view["status"].(string)
		return status == "completed"
	})
}

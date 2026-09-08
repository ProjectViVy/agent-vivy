package runtime

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"agent-vivy/internal/tools"
	"github.com/mark3labs/mcp-go/mcp"
)

// TestMCPStdioHelperProcess is a deterministic local MCP child used by the
// stdio integration tests below. It is launched as the current test binary,
// so the fixture needs no network, compiler, or external executable.
func TestMCPStdioHelperProcess(t *testing.T) {
	if os.Getenv("MCP_HELPER") != "1" {
		return
	}
	if marker := os.Getenv("MCP_CWD_MARKER"); marker != "" {
		_ = os.WriteFile(marker, []byte(mustMCPHelperWorkingDirectory()), 0o600)
	}
	if marker := os.Getenv("MCP_MARKER"); marker != "" {
		current, _ := os.ReadFile(marker)
		_ = os.WriteFile(marker, []byte(fmt.Sprintf("%d", len(strings.TrimSpace(string(current)))+1)), 0o600)
	}
	reader := bufio.NewScanner(os.Stdin)
	writer := bufio.NewWriter(os.Stdout)
	for reader.Scan() {
		var request struct {
			JSONRPC string          `json:"jsonrpc"`
			ID      json.RawMessage `json:"id"`
			Method  string          `json:"method"`
		}
		if json.Unmarshal(reader.Bytes(), &request) != nil {
			continue
		}
		if request.ID == nil {
			continue
		}
		var result any
		var rpcError map[string]any
		switch request.Method {
		case "server/discover":
			rpcError = map[string]any{"code": -32601, "message": "method not found"}
		case "initialize":
			if os.Getenv("MCP_HANDSHAKE_FAIL") == "1" {
				rpcError = map[string]any{"code": -32000, "message": "fixture handshake failed"}
				break
			}
			result = initializeResult(map[string]any{
				"tools":     map[string]any{},
				"resources": map[string]any{},
				"prompts":   map[string]any{},
			})
		case "tools/list":
			result = map[string]any{"tools": []any{map[string]any{"name": "echo", "description": "stdio echo", "inputSchema": map[string]any{"type": "object"}}}}
		case "tools/call":
			result = map[string]any{"content": []any{map[string]any{"type": "text", "text": "stdio output"}}, "isError": false}
		case "resources/list":
			result = map[string]any{"resources": []any{map[string]any{"uri": "stdio://resource", "name": "resource", "description": "stdio resource", "mimeType": "text/plain"}}}
		case "resources/read":
			result = map[string]any{"contents": []any{map[string]any{"uri": "stdio://resource", "mimeType": "text/plain", "text": "stdio resource output"}}}
		case "prompts/list":
			result = map[string]any{"prompts": []any{map[string]any{"name": "review", "description": "stdio review", "arguments": []any{map[string]any{"name": "focus", "required": true}}}}}
		case "prompts/get":
			result = map[string]any{"description": "stdio prompt", "messages": []any{map[string]any{"role": "user", "content": map[string]any{"type": "text", "text": "stdio prompt output"}}}}
		default:
			rpcError = map[string]any{"code": -32601, "message": "method not found"}
		}
		envelope := map[string]any{"jsonrpc": "2.0", "id": json.RawMessage(request.ID)}
		if rpcError != nil {
			envelope["error"] = rpcError
		} else {
			envelope["result"] = result
		}
		if err := json.NewEncoder(writer).Encode(envelope); err != nil {
			return
		}
		if err := writer.Flush(); err != nil {
			return
		}
		if request.Method == "tools/list" && os.Getenv("MCP_EXIT_AFTER_LIST") == "1" {
			os.Exit(0)
		}
	}
}

func mustMCPHelperWorkingDirectory() string {
	workingDirectory, err := os.Getwd()
	if err != nil {
		return ""
	}
	return workingDirectory
}

type testMCPRequest struct {
	Method  string
	ID      json.RawMessage
	Params  map[string]any
	Headers http.Header
}

type testMCPResponse struct {
	Status   int
	Result   any
	RPCError map[string]any
	SSE      bool
	Headers  map[string]string
	NoBody   bool
}

type testMCPServer struct {
	t        *testing.T
	dispatch func(testMCPRequest) testMCPResponse
	http     *httptest.Server
	mu       sync.Mutex
	session  string
	sequence int
	deletes  int
	auth     []string
	sessions []string
	methods  []string
}

func newTestMCPServer(t *testing.T, dispatch func(testMCPRequest) testMCPResponse) *testMCPServer {
	t.Helper()
	fixture := &testMCPServer{t: t, dispatch: dispatch}
	fixture.http = httptest.NewServer(http.HandlerFunc(fixture.serveHTTP))
	t.Cleanup(func() { fixture.http.Close() })
	return fixture
}

func (s *testMCPServer) serveHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodDelete {
		s.mu.Lock()
		s.deletes++
		s.mu.Unlock()
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var envelope struct {
		Method string          `json:"method"`
		ID     json.RawMessage `json:"id"`
		Params map[string]any  `json:"params"`
	}
	if err := json.NewDecoder(r.Body).Decode(&envelope); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	req := testMCPRequest{Method: envelope.Method, ID: envelope.ID, Params: envelope.Params, Headers: r.Header.Clone()}
	s.mu.Lock()
	s.methods = append(s.methods, req.Method)
	s.auth = append(s.auth, req.Headers.Get("Authorization"))
	s.sessions = append(s.sessions, req.Headers.Get("Mcp-Session-Id"))
	current := s.session
	s.mu.Unlock()
	if req.Method != "initialize" && req.Method != "notifications/initialized" && current != "" && req.Headers.Get("Mcp-Session-Id") != current {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if req.Method == "initialize" {
		s.mu.Lock()
		s.sequence++
		s.session = fmt.Sprintf("session-%d", s.sequence)
		current = s.session
		s.mu.Unlock()
		w.Header().Set("Mcp-Session-Id", current)
	}
	response := s.dispatch(req)
	for key, value := range response.Headers {
		w.Header().Set(key, value)
	}
	if response.NoBody || req.Method == "notifications/initialized" {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	status := response.Status
	if status == 0 {
		status = http.StatusOK
	}
	if status == http.StatusAccepted && response.Result != nil {
		status = http.StatusOK
	}
	if status != http.StatusOK && status != http.StatusAccepted {
		w.WriteHeader(status)
		return
	}
	envelopeOut := map[string]any{"jsonrpc": "2.0", "id": json.RawMessage(req.ID)}
	if response.RPCError != nil {
		envelopeOut["error"] = response.RPCError
	} else {
		envelopeOut["result"] = response.Result
	}
	body, err := json.Marshal(envelopeOut)
	if err != nil {
		s.t.Errorf("marshal test MCP response: %v", err)
		return
	}
	if response.SSE {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprintf(w, "event: message\ndata: %s\n\n", body)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(body)
}

func (s *testMCPServer) backend(t *testing.T, name string) *MCPBackend {
	t.Helper()
	backend := NewMCPBackend([]MCPServerConfig{{Name: name, Endpoint: s.http.URL}}, s.http.Client())
	t.Cleanup(func() { _ = backend.Close() })
	return backend
}

func (s *testMCPServer) counts() (deletes int, methods []string, auth []string) {
	s.mu.Lock()
	deletes = s.deletes
	methods = append([]string(nil), s.methods...)
	auth = append([]string(nil), s.auth...)
	s.mu.Unlock()
	return deletes, methods, auth
}

func (s *testMCPServer) sessionHeaders() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.sessions...)
}

func initializeResult(capabilities map[string]any) map[string]any {
	return map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    capabilities,
		"serverInfo":      map[string]any{"name": "fixture", "version": "1"},
	}
}

func TestMCPBackendStreamableHTTPAndEinoProjection(t *testing.T) {
	server := newTestMCPServer(t, func(request testMCPRequest) testMCPResponse {
		switch request.Method {
		case "initialize":
			return testMCPResponse{Status: http.StatusAccepted, Result: initializeResult(map[string]any{"tools": map[string]any{}})}
		case "tools/list":
			return testMCPResponse{SSE: true, Result: map[string]any{"tools": []any{
				map[string]any{"name": "browser_use", "description": "hidden", "inputSchema": map[string]any{"type": "object"}},
				map[string]any{"name": "echo", "description": "remote echo", "inputSchema": map[string]any{
					"type": "object", "properties": map[string]any{"text": map[string]any{"type": "string"}}, "required": []string{"text"},
				}},
			}}}
		case "tools/call":
			return testMCPResponse{SSE: true, Result: map[string]any{"content": []any{
				map[string]any{"type": "text", "text": "remote output"},
			}, "isError": true}}
		default:
			return testMCPResponse{NoBody: true}
		}
	})
	backend := server.backend(t, "local")
	listed, err := backend.ListTools(context.Background(), "", "local")
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}
	if len(listed.Tools) != 1 || listed.Tools[0].Name != "echo" || !listed.Untrusted {
		t.Fatalf("projected tools = %#v", listed)
	}
	if !strings.Contains(string(listed.Tools[0].InputSchema), `"properties"`) || !strings.Contains(string(listed.Tools[0].InputSchema), `"text"`) {
		t.Fatalf("Eino schema conversion lost fields: %s", listed.Tools[0].InputSchema)
	}
	statuses := backend.ServerStatuses()
	if len(statuses) != 1 || !statuses[0].Initialized || statuses[0].Error != "" || statuses[0].ToolCount != 1 {
		t.Fatalf("MCP status after listing = %+v", statuses)
	}
	called, err := backend.CallTool(context.Background(), "", tools.MCPCallRequest{Server: "local", Tool: "echo", Arguments: map[string]any{"text": "hi"}})
	if err != nil {
		t.Fatalf("call tool: %v", err)
	}
	if !called.IsError || len(called.Content) != 1 || called.Content[0].Text != "remote output" || !called.Untrusted {
		t.Fatalf("call result = %#v", called)
	}
}

func TestMCPBackendModernDiscoverAvoidsLegacySession(t *testing.T) {
	server := newTestMCPServer(t, func(request testMCPRequest) testMCPResponse {
		switch request.Method {
		case "server/discover":
			return testMCPResponse{Result: map[string]any{
				"supportedVersions": []string{mcp.LATEST_PROTOCOL_VERSION},
				"capabilities":      map[string]any{"tools": map[string]any{}},
			}}
		case "tools/list":
			return testMCPResponse{Result: map[string]any{"tools": []map[string]any{{"name": "modern", "inputSchema": map[string]any{"type": "object"}}}}}
		default:
			return testMCPResponse{NoBody: true}
		}
	})
	backend := server.backend(t, "modern")
	listed, err := backend.ListTools(context.Background(), "", "modern")
	if err != nil || len(listed.Tools) != 1 || listed.Tools[0].Name != "modern" {
		t.Fatalf("modern discover listed=%#v err=%v", listed, err)
	}
	if err := backend.Close(); err != nil {
		t.Fatalf("close modern backend: %v", err)
	}
	deletes, methods, _ := server.counts()
	discoverCount := 0
	for _, method := range methods {
		if method == "server/discover" {
			discoverCount++
		}
	}
	if discoverCount != 1 || deletes != 0 || containsStringMCP(methods, "initialize") || containsStringMCP(methods, "notifications/initialized") {
		t.Fatalf("modern client used legacy lifecycle: deletes=%d methods=%v", deletes, methods)
	}
	for _, sessionID := range server.sessionHeaders() {
		if sessionID != "" {
			t.Fatalf("modern client sent legacy session header %q", sessionID)
		}
	}
}

func TestMCPBackendAuthEnvRotatesPerRequest(t *testing.T) {
	const env = "VIVY_TEST_MCP_TOKEN"
	if err := os.Setenv(env, "first-token"); err != nil {
		t.Fatal(err)
	}
	defer os.Unsetenv(env)
	server := newTestMCPServer(t, func(request testMCPRequest) testMCPResponse {
		switch request.Method {
		case "initialize":
			return testMCPResponse{Status: http.StatusAccepted, Result: initializeResult(map[string]any{"tools": map[string]any{}})}
		case "tools/list":
			return testMCPResponse{Result: map[string]any{"tools": []map[string]any{{"name": "echo", "inputSchema": map[string]any{"type": "object"}}}}}
		default:
			return testMCPResponse{NoBody: true}
		}
	})
	backend := NewMCPBackend([]MCPServerConfig{{Name: "auth", Endpoint: server.http.URL, AuthEnv: env}}, server.http.Client())
	t.Cleanup(func() { _ = backend.Close() })
	if _, err := backend.ListTools(context.Background(), "", "auth"); err != nil {
		t.Fatal(err)
	}
	if err := os.Setenv(env, "rotated-token"); err != nil {
		t.Fatal(err)
	}
	if _, err := backend.ListTools(context.Background(), "", "auth"); err != nil {
		t.Fatal(err)
	}
	_, _, auth := server.counts()
	if !containsStringMCP(auth, "Bearer first-token") || !containsStringMCP(auth, "Bearer rotated-token") {
		t.Fatalf("authorization headers did not rotate: %v", auth)
	}
	statuses := backend.ServerStatuses()
	if len(statuses) != 1 || !statuses[0].Initialized || statuses[0].AuthMissing || statuses[0].ToolCount != 1 {
		t.Fatalf("auth status with token = %+v", statuses)
	}
	if err := os.Unsetenv(env); err != nil {
		t.Fatal(err)
	}
	statuses = backend.ServerStatuses()
	if len(statuses) != 1 || !statuses[0].AuthMissing {
		t.Fatalf("missing auth status = %+v", statuses)
	}
}

func TestMCPBackendConcurrentInitializeOnce(t *testing.T) {
	var initializes atomic.Int32
	server := newTestMCPServer(t, func(request testMCPRequest) testMCPResponse {
		switch request.Method {
		case "initialize":
			initializes.Add(1)
			time.Sleep(20 * time.Millisecond)
			return testMCPResponse{Status: http.StatusAccepted, Result: initializeResult(map[string]any{"tools": map[string]any{}})}
		case "tools/list":
			return testMCPResponse{Result: map[string]any{"tools": []map[string]any{{"name": "echo", "inputSchema": map[string]any{"type": "object"}}}}}
		default:
			return testMCPResponse{NoBody: true}
		}
	})
	backend := server.backend(t, "concurrent")
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := backend.ListTools(context.Background(), "", "concurrent")
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if got := initializes.Load(); got != 1 {
		t.Fatalf("initialize count = %d, want one", got)
	}
}

func TestMCPBackendFailedInitializeRecoversAndClosesOldClient(t *testing.T) {
	var initializes atomic.Int32
	server := newTestMCPServer(t, func(request testMCPRequest) testMCPResponse {
		switch request.Method {
		case "initialize":
			if initializes.Add(1) == 1 {
				return testMCPResponse{Status: http.StatusInternalServerError}
			}
			return testMCPResponse{Status: http.StatusAccepted, Result: initializeResult(map[string]any{"tools": map[string]any{}})}
		case "tools/list":
			return testMCPResponse{Result: map[string]any{"tools": []map[string]any{{"name": "recovered", "inputSchema": map[string]any{"type": "object"}}}}}
		default:
			return testMCPResponse{NoBody: true}
		}
	})
	backend := server.backend(t, "recover")
	backend.mu.RLock()
	old := backend.servers["recover"]
	backend.mu.RUnlock()
	if _, err := backend.ListTools(context.Background(), "", "recover"); err == nil {
		t.Fatal("failed initialize must be returned")
	}
	failedStatus := backend.ServerStatuses()
	if len(failedStatus) != 1 || failedStatus[0].Initialized || failedStatus[0].Error == "" || failedStatus[0].ToolCount != -1 {
		t.Fatalf("failed initialize status = %+v", failedStatus)
	}
	select {
	case <-old.closeDone:
	case <-time.After(time.Second):
		t.Fatal("failed initialize client was not closed")
	}
	listed, err := backend.ListTools(context.Background(), "", "recover")
	if err != nil || len(listed.Tools) != 1 || listed.Tools[0].Name != "recovered" {
		t.Fatalf("recovery listed=%#v err=%v", listed, err)
	}
	if got := initializes.Load(); got != 2 {
		t.Fatalf("initialize count=%d, want failed attempt plus recovery", got)
	}
	recoveredStatus := backend.ServerStatuses()
	if len(recoveredStatus) != 1 || !recoveredStatus[0].Initialized || recoveredStatus[0].Error != "" || recoveredStatus[0].ToolCount != 1 {
		t.Fatalf("recovered status = %+v", recoveredStatus)
	}
}

func TestMCPBackendStatusSanitizesAndRejectsRetiredWrites(t *testing.T) {
	endpoint := "https://user:secret@example.invalid/mcp"
	backend := NewMCPBackend([]MCPServerConfig{{Name: "same", Endpoint: endpoint}}, nil)
	t.Cleanup(func() { _ = backend.Close() })
	backend.mu.RLock()
	old := backend.servers["same"]
	backend.mu.RUnlock()
	backend.noteInitResult(old, fmt.Errorf("%s\n\tconnection\u202Ebad\u2066", endpoint))
	backend.noteToolCount(old, 7)
	status := backend.ServerStatuses()
	if len(status) != 1 || status[0].Error == "" || status[0].ToolCount != 7 {
		t.Fatalf("initial status = %+v", status)
	}
	if strings.Contains(status[0].Error, endpoint) || strings.ContainsAny(status[0].Error, "\r\n\t") || strings.ContainsAny(status[0].Error, "\u202E\u2066") {
		t.Fatalf("status error was not sanitized: %q", status[0].Error)
	}

	backend.ReplaceServers([]MCPServerConfig{{Name: "same", Endpoint: "https://new.example.invalid/mcp"}})
	status = backend.ServerStatuses()
	if len(status) != 1 || status[0].Error != "" || status[0].ToolCount != -1 {
		t.Fatalf("replacement inherited old status = %+v", status)
	}
	backend.noteInitResult(old, errors.New("retired failure"))
	backend.noteToolCount(old, 99)
	status = backend.ServerStatuses()
	if len(status) != 1 || status[0].Error != "" || status[0].ToolCount != -1 {
		t.Fatalf("retired entry wrote current status = %+v", status)
	}
}

func TestBoundedMCPStatusErrorRemovesControlsAndBoundsRunes(t *testing.T) {
	endpoint := "https://secret.example.invalid/mcp"
	message := endpoint + "\x00\n\t\u061c\u200e\u200f\u202A\u202E\u2066" + strings.Repeat("x", 200)
	got := boundedMCPStatusError(message, endpoint)
	if strings.Contains(got, endpoint) || strings.ContainsAny(got, "\x00\r\n\t\u061c\u200e\u200f\u202A\u202E\u2066") {
		t.Fatalf("bounded status retained secret/control: %q", got)
	}
	if gotRunes := len([]rune(got)); gotRunes != 160 {
		t.Fatalf("bounded status rune length = %d, want 160", gotRunes)
	}
}

func TestMCPBackendSessionRetryAndCallNoRetry(t *testing.T) {
	var listCalls atomic.Int32
	var callCalls atomic.Int32
	server := newTestMCPServer(t, func(request testMCPRequest) testMCPResponse {
		switch request.Method {
		case "initialize":
			return testMCPResponse{Status: http.StatusAccepted, Result: initializeResult(map[string]any{"tools": map[string]any{}})}
		case "tools/list":
			if listCalls.Add(1) == 1 {
				return testMCPResponse{Status: http.StatusNotFound}
			}
			return testMCPResponse{Result: map[string]any{"tools": []map[string]any{{"name": "echo", "inputSchema": map[string]any{"type": "object"}}}}}
		case "tools/call":
			callCalls.Add(1)
			return testMCPResponse{Status: http.StatusNotFound}
		default:
			return testMCPResponse{NoBody: true}
		}
	})
	backend := server.backend(t, "retry")
	if listed, err := backend.ListTools(context.Background(), "", "retry"); err != nil || len(listed.Tools) != 1 {
		t.Fatalf("idempotent retry listed=%#v err=%v", listed, err)
	}
	if got := listCalls.Load(); got != 2 {
		t.Fatalf("list calls = %d, want one failed + one retry", got)
	}
	if _, err := backend.CallTool(context.Background(), "", tools.MCPCallRequest{Server: "retry", Tool: "echo"}); err == nil {
		t.Fatal("tools/call session failure must be returned")
	}
	if got := callCalls.Load(); got != 1 {
		t.Fatalf("call retried %d times", got)
	}
}

func TestMCPBackendReplaceRemoveAndCloseClients(t *testing.T) {
	first := newTestMCPServer(t, simpleToolsDispatch("old"))
	second := newTestMCPServer(t, simpleToolsDispatch("new"))
	backend := NewMCPBackend([]MCPServerConfig{
		{Name: "b", Endpoint: second.http.URL}, {Name: "a", Endpoint: first.http.URL},
	}, first.http.Client())
	t.Cleanup(func() { _ = backend.Close() })
	if _, err := backend.ListTools(context.Background(), "", "a"); err != nil {
		t.Fatal(err)
	}
	backend.ReplaceServers([]MCPServerConfig{{Name: "b", Endpoint: second.http.URL}, {Name: "a", Endpoint: second.http.URL}})
	if configured := backend.ConfiguredServers(); len(configured) != 2 || configured[0].Name != "a" || configured[1].Name != "b" {
		t.Fatalf("configured order = %+v", configured)
	}
	if listed, err := backend.ListTools(context.Background(), "", "a"); err != nil || listed.Tools[0].Name != "new" {
		t.Fatalf("replaced catalog=%#v err=%v", listed, err)
	}
	backend.ReplaceServers([]MCPServerConfig{{Name: "b", Endpoint: second.http.URL}})
	if _, err := backend.ListTools(context.Background(), "", "a"); err == nil {
		t.Fatal("removed server must fail closed")
	}
	if err := backend.Close(); err != nil {
		t.Fatal(err)
	}
	if err := backend.Close(); err != nil {
		t.Fatal(err)
	}
	if deletes, _, _ := first.counts(); deletes == 0 {
		t.Fatal("initialized removed client was not closed")
	}
}

func TestMCPBackendResourcesPromptsAndFailClosed(t *testing.T) {
	var methods []string
	server := newTestMCPServer(t, func(request testMCPRequest) testMCPResponse {
		methods = append(methods, request.Method)
		switch request.Method {
		case "initialize":
			return testMCPResponse{Status: http.StatusAccepted, Result: initializeResult(map[string]any{"resources": map[string]any{}, "prompts": map[string]any{}})}
		case "resources/list":
			if request.Params["cursor"] == nil {
				return testMCPResponse{Result: map[string]any{"resources": []map[string]any{{"uri": "docs://guide", "name": "guide", "title": "Guide", "description": "remote guide", "mimeType": "text/markdown", "size": 42, "annotations": map[string]any{"audience": []string{"user"}}, "_meta": map[string]any{"origin": "remote"}}}, "nextCursor": "page-2"}}
			}
			return testMCPResponse{Result: map[string]any{"resources": []map[string]any{{"uri": "docs://faq", "name": "faq"}}}, SSE: true}
		case "resources/read":
			return testMCPResponse{Result: map[string]any{"contents": []map[string]any{{"uri": "docs://guide", "mimeType": "text/markdown", "text": "# Hello"}, {"uri": "docs://guide", "mimeType": "application/octet-stream", "blob": "AQI="}}}}
		case "prompts/list":
			return testMCPResponse{Result: map[string]any{"prompts": []map[string]any{{"name": "review", "title": "Review", "description": "Review a change", "arguments": []map[string]any{{"name": "focus", "description": "review focus", "required": true}}}}}}
		case "prompts/get":
			return testMCPResponse{Result: map[string]any{"description": "expanded review", "messages": []map[string]any{{"role": "user", "content": map[string]any{"type": "text", "text": "Review security"}}, {"role": "assistant", "content": map[string]any{"type": "text", "text": "ignored"}}}}}
		default:
			return testMCPResponse{NoBody: true}
		}
	})
	backend := server.backend(t, "docs")
	resources, err := backend.ListResources(context.Background(), "", "docs")
	if err != nil || len(resources.Resources) != 2 {
		t.Fatalf("resources=%#v err=%v", resources, err)
	}
	if resources.Resources[0].Title != "Guide" || resources.Resources[0].Size != 42 || string(resources.Resources[0].Meta) != `{"origin":"remote"}` {
		t.Fatalf("resource fields lost: %#v", resources.Resources[0])
	}
	read, err := backend.ReadResource(context.Background(), "", tools.MCPReadResourceRequest{Server: "docs", URI: "docs://guide"})
	if err != nil || len(read.Contents) != 2 || read.Contents[0].Text == nil || read.Contents[1].Blob == nil {
		t.Fatalf("read=%#v err=%v", read, err)
	}
	prompts, err := backend.ListPrompts(context.Background(), "", "docs")
	if err != nil || len(prompts.Prompts) != 1 || !prompts.Prompts[0].Arguments[0].Required {
		t.Fatalf("prompts=%#v err=%v", prompts, err)
	}
	got, err := backend.GetPrompt(context.Background(), "", tools.MCPGetPromptRequest{Server: "docs", Name: "review", Arguments: map[string]string{"focus": "security"}})
	if err != nil || got.Text != "Review security" || got.Description != "expanded review" {
		t.Fatalf("prompt=%#v err=%v", got, err)
	}
	if !containsStringMCP(methods, "resources/list") || !containsStringMCP(methods, "prompts/get") {
		t.Fatalf("methods=%v", methods)
	}

	noPrompt := newTestMCPServer(t, func(request testMCPRequest) testMCPResponse {
		if request.Method == "initialize" {
			return testMCPResponse{Status: http.StatusAccepted, Result: initializeResult(map[string]any{"tools": map[string]any{}})}
		}
		return testMCPResponse{Status: http.StatusInternalServerError}
	})
	withoutPrompts := noPrompt.backend(t, "tools-only")
	listed, err := withoutPrompts.ListPrompts(context.Background(), "", "tools-only")
	if err != nil || len(listed.Prompts) != 0 {
		t.Fatalf("missing prompt capability must fail closed: %#v err=%v", listed, err)
	}

	nonText := newTestMCPServer(t, func(request testMCPRequest) testMCPResponse {
		switch request.Method {
		case "initialize":
			return testMCPResponse{Status: http.StatusAccepted, Result: initializeResult(map[string]any{"prompts": map[string]any{}})}
		case "prompts/get":
			return testMCPResponse{Result: map[string]any{"messages": []map[string]any{{"role": "user", "content": map[string]any{"type": "image", "data": "AQI=", "mimeType": "image/png"}}}}}
		default:
			return testMCPResponse{NoBody: true}
		}
	})
	nonTextBackend := nonText.backend(t, "non-text")
	if _, err := nonTextBackend.GetPrompt(context.Background(), "", tools.MCPGetPromptRequest{Server: "non-text", Name: "review"}); err == nil || !strings.Contains(err.Error(), "no user text") {
		t.Fatalf("non-text user prompt must fail closed: %v", err)
	}
}

func TestMCPBackendBoundsPagesContentAndRepeatedCursor(t *testing.T) {
	var listCalls atomic.Int32
	server := newTestMCPServer(t, func(request testMCPRequest) testMCPResponse {
		if request.Method == "initialize" {
			return testMCPResponse{Status: http.StatusAccepted, Result: initializeResult(map[string]any{"tools": map[string]any{}})}
		}
		if request.Method == "tools/list" {
			page := listCalls.Add(1)
			cursor, _ := request.Params["cursor"].(string)
			if cursor == "repeat" {
				return testMCPResponse{Result: map[string]any{"tools": []map[string]any{{"name": "ignored", "inputSchema": map[string]any{"type": "object"}}}, "nextCursor": "repeat"}}
			}
			return testMCPResponse{Result: map[string]any{"tools": []map[string]any{{"name": fmt.Sprintf("tool-%d", page), "description": strings.Repeat("x", 20<<10), "inputSchema": map[string]any{"type": "object"}}}, "nextCursor": "repeat"}}
		}
		return testMCPResponse{NoBody: true}
	})
	backend := server.backend(t, "bounds")
	listed, err := backend.ListTools(context.Background(), "", "bounds")
	if err == nil || !strings.Contains(err.Error(), "repeated cursor") {
		t.Fatalf("repeated cursor error=%v listed=%#v", err, listed)
	}
	if got := listCalls.Load(); got != 2 {
		t.Fatalf("repeated cursor calls=%d, want 2", got)
	}

	var pageCalls atomic.Int32
	pageServer := newTestMCPServer(t, func(request testMCPRequest) testMCPResponse {
		if request.Method == "initialize" {
			return testMCPResponse{Status: http.StatusAccepted, Result: initializeResult(map[string]any{"tools": map[string]any{}})}
		}
		if request.Method == "tools/list" {
			page := pageCalls.Add(1)
			return testMCPResponse{Result: map[string]any{"tools": []map[string]any{{"name": fmt.Sprintf("page-%d", page), "inputSchema": map[string]any{"type": "object"}}}, "nextCursor": fmt.Sprintf("cursor-%d", page)}}
		}
		return testMCPResponse{NoBody: true}
	})
	pageBackend := pageServer.backend(t, "pages")
	if _, err := pageBackend.ListTools(context.Background(), "", "pages"); err == nil || !strings.Contains(err.Error(), "exceeded 32 pages") {
		t.Fatalf("page bound error=%v", err)
	}
	if got := pageCalls.Load(); got != maxMCPResourcePages {
		t.Fatalf("page calls=%d, want %d", got, maxMCPResourcePages)
	}

	var resourcePages atomic.Int32
	resourceServer := newTestMCPServer(t, func(request testMCPRequest) testMCPResponse {
		switch request.Method {
		case "initialize":
			return testMCPResponse{Status: http.StatusAccepted, Result: initializeResult(map[string]any{"resources": map[string]any{}})}
		case "resources/list":
			page := resourcePages.Add(1)
			return testMCPResponse{Result: map[string]any{"resources": []map[string]any{{"uri": fmt.Sprintf("docs://page-%d", page), "name": fmt.Sprintf("page-%d", page)}}, "nextCursor": fmt.Sprintf("cursor-%d", page)}}
		default:
			return testMCPResponse{NoBody: true}
		}
	})
	resourceBackend := resourceServer.backend(t, "resource-pages")
	if _, err := resourceBackend.ListResources(context.Background(), "", "resource-pages"); err == nil || !strings.Contains(err.Error(), "exceeded 32 pages") {
		t.Fatalf("resource page bound error=%v", err)
	}
	if got := resourcePages.Load(); got != maxMCPResourcePages {
		t.Fatalf("resource page calls=%d, want %d", got, maxMCPResourcePages)
	}

	var budgetPage atomic.Int32
	budgetServer := newTestMCPServer(t, func(request testMCPRequest) testMCPResponse {
		if request.Method == "initialize" {
			return testMCPResponse{Status: http.StatusAccepted, Result: initializeResult(map[string]any{"tools": map[string]any{}})}
		}
		if request.Method == "tools/list" {
			page := budgetPage.Add(1)
			return testMCPResponse{Result: map[string]any{"tools": []map[string]any{{"name": fmt.Sprintf("budget-%d", page), "description": strings.Repeat("b", 16<<10), "inputSchema": map[string]any{"type": "object"}}}, "nextCursor": fmt.Sprintf("budget-%d", page)}}
		}
		return testMCPResponse{NoBody: true}
	})
	budgetBackend := budgetServer.backend(t, "budget")
	budgetCatalog, err := budgetBackend.ListTools(context.Background(), "", "budget")
	if err != nil {
		t.Fatalf("aggregate tool budget: %v", err)
	}
	used := 0
	for _, item := range budgetCatalog.Tools {
		used += mcpToolProjectedSize(item)
	}
	if used > maxMCPContentBytes || budgetPage.Load() >= maxMCPResourcePages {
		t.Fatalf("aggregate tool budget used=%d pages=%d", used, budgetPage.Load())
	}

	large := newTestMCPServer(t, func(request testMCPRequest) testMCPResponse {
		if request.Method == "initialize" {
			return testMCPResponse{Status: http.StatusAccepted, Result: initializeResult(map[string]any{"tools": map[string]any{}})}
		}
		if request.Method == "tools/call" {
			return testMCPResponse{Result: map[string]any{"content": []map[string]any{{"type": "text", "text": strings.Repeat("x", maxMCPContentBytes+1024)}}}}
		}
		return testMCPResponse{NoBody: true}
	})
	bounded := large.backend(t, "large")
	called, err := bounded.CallTool(context.Background(), "", tools.MCPCallRequest{Server: "large", Tool: "big"})
	if err != nil || len(called.Content) != 1 || len(called.Content[0].Text) != maxMCPContentBytes-len(called.Content[0].Type) {
		t.Fatalf("bounded call=%#v err=%v", called, err)
	}
	bounded.maxResponseBytes = 128
	if _, err := bounded.CallTool(context.Background(), "", tools.MCPCallRequest{Server: "large", Tool: "big"}); err == nil || !strings.Contains(err.Error(), "size limit") {
		t.Fatalf("raw response bound error=%v", err)
	}
}

func TestMCPBackendCloseRejectsOperations(t *testing.T) {
	backend := NewMCPBackend(nil, nil)
	if err := backend.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := backend.ListTools(context.Background(), "", "missing"); err == nil || !strings.Contains(err.Error(), "backend is closed") {
		t.Fatalf("closed backend accepted operation: %v", err)
	}
}

func TestBoundedMCPBodyReaderSemantics(t *testing.T) {
	t.Run("zero length does not touch source", func(t *testing.T) {
		source := &scriptedMCPReadCloser{reads: []scriptedMCPRead{{data: []byte("x")}}}
		body := &boundedMCPBody{ReadCloser: source, limit: 1}
		n, err := body.Read(nil)
		if n != 0 || err != nil || source.calls != 0 {
			t.Fatalf("zero-length read = (%d, %v), source calls=%d", n, err, source.calls)
		}
	})

	t.Run("exact limit then terminal error", func(t *testing.T) {
		source := &scriptedMCPReadCloser{reads: []scriptedMCPRead{
			{data: []byte("abc")},
			{err: io.EOF},
		}}
		body := &boundedMCPBody{ReadCloser: source, limit: 3}
		got := make([]byte, 8)
		n, err := body.Read(got)
		if n != 3 || err != nil || string(got[:n]) != "abc" {
			t.Fatalf("exact-limit read = (%d, %v, %q)", n, err, got[:n])
		}
		n, err = body.Read(got)
		if n != 0 || !errors.Is(err, io.EOF) {
			t.Fatalf("terminal probe = (%d, %v), want EOF", n, err)
		}
	})

	t.Run("zero nil probe remains retryable", func(t *testing.T) {
		source := &scriptedMCPReadCloser{reads: []scriptedMCPRead{
			{data: []byte("abc")},
			{},
			{data: []byte("x")},
		}}
		body := &boundedMCPBody{ReadCloser: source, limit: 3}
		got := make([]byte, 8)
		if n, err := body.Read(got); n != 3 || err != nil {
			t.Fatalf("initial read = (%d, %v), want exact limit", n, err)
		}
		if n, err := body.Read(got); n != 0 || err != nil {
			t.Fatalf("empty probe = (%d, %v), want 0,nil", n, err)
		}
		if n, err := body.Read(got); n != 0 || err == nil || !strings.Contains(err.Error(), "size limit") {
			t.Fatalf("delayed over-limit probe = (%d, %v), want size error", n, err)
		}
		if n, err := body.Read(got); n != 0 || !errors.Is(err, io.EOF) {
			t.Fatalf("post-error read = (%d, %v), want EOF", n, err)
		}
	})
}

type scriptedMCPRead struct {
	data []byte
	err  error
}

type scriptedMCPReadCloser struct {
	reads []scriptedMCPRead
	calls int
}

func (s *scriptedMCPReadCloser) Read(p []byte) (int, error) {
	s.calls++
	if len(s.reads) == 0 {
		return 0, io.EOF
	}
	read := s.reads[0]
	s.reads = s.reads[1:]
	if len(read.data) > len(p) {
		panic("scriptedMCPRead data exceeds destination")
	}
	copy(p, read.data)
	return len(read.data), read.err
}

func (s *scriptedMCPReadCloser) Close() error { return nil }

func TestMCPBackendMapsErrorsFiltersBrowserUseAndSortsServers(t *testing.T) {
	first := newTestMCPServer(t, func(request testMCPRequest) testMCPResponse {
		switch request.Method {
		case "initialize":
			return testMCPResponse{Status: http.StatusAccepted, Result: initializeResult(map[string]any{"tools": map[string]any{}})}
		case "tools/list":
			return testMCPResponse{Result: map[string]any{"tools": []map[string]any{{"name": "browser-use", "inputSchema": map[string]any{"type": "object"}}, {"name": "zeta", "inputSchema": map[string]any{"type": "object"}}}}}
		case "tools/call":
			return testMCPResponse{RPCError: map[string]any{"code": -32602, "message": "bad arguments"}}
		default:
			return testMCPResponse{NoBody: true}
		}
	})
	second := newTestMCPServer(t, simpleToolsDispatch("alpha"))
	backend := NewMCPBackend([]MCPServerConfig{{Name: "z", Endpoint: second.http.URL}, {Name: "a", Endpoint: first.http.URL}}, first.http.Client())
	t.Cleanup(func() { _ = backend.Close() })
	listed, err := backend.ListTools(context.Background(), "", "")
	if err != nil || len(listed.Tools) != 2 || listed.Tools[0].Server != "a" || listed.Tools[1].Server != "z" || listed.Tools[0].Name != "zeta" {
		t.Fatalf("sorted/filter catalog=%#v err=%v", listed, err)
	}
	_, err = backend.CallTool(context.Background(), "", tools.MCPCallRequest{Server: "a", Tool: "zeta"})
	var remote *tools.MCPRemoteError
	if !errors.As(err, &remote) || remote.Code != -32602 {
		t.Fatalf("remote error=%T %v", err, err)
	}
}

func TestMCPBackendStdioLazyProjectionAndCall(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "spawn-count")
	root := t.TempDir()
	t.Setenv("VIVY_MCP_HELPER", "1")
	t.Setenv("VIVY_MCP_MARKER", marker)
	t.Setenv("VIVY_MCP_NO_EXIT", "0")
	backend := NewMCPBackendWithOptions([]MCPServerConfig{{
		Name:    "local",
		Command: mcpHelperCommand(t),
		Args:    []string{"-test.run=TestMCPStdioHelperProcess"},
		EnvFrom: map[string]string{"MCP_HELPER": "VIVY_MCP_HELPER", "MCP_MARKER": "VIVY_MCP_MARKER", "MCP_EXIT_AFTER_LIST": "VIVY_MCP_NO_EXIT"},
	}}, nil, MCPBackendOptions{ProcessRoot: root})
	t.Cleanup(func() { _ = backend.Close() })
	if statuses := backend.ServerStatuses(); len(statuses) != 1 || statuses[0].Initialized || statuses[0].Error != "" {
		t.Fatalf("stdio backend opened before first operation: %+v", statuses)
	}
	listed, err := backend.ListTools(context.Background(), "", "local")
	if err != nil || len(listed.Tools) != 1 || listed.Tools[0].Name != "echo" {
		t.Fatalf("stdio tools=%#v err=%v", listed, err)
	}
	if count, _ := os.ReadFile(marker); string(count) != "1" {
		t.Fatalf("stdio spawn count=%q, want 1", count)
	}
	called, err := backend.CallTool(context.Background(), "", tools.MCPCallRequest{Server: "local", Tool: "echo", Arguments: map[string]any{"text": "hi"}})
	if err != nil || len(called.Content) != 1 || called.Content[0].Text != "stdio output" {
		t.Fatalf("stdio call=%#v err=%v", called, err)
	}
	statuses := backend.ServerStatuses()
	if len(statuses) != 1 || !statuses[0].Initialized || statuses[0].ToolCount != 1 {
		t.Fatalf("stdio status=%+v", statuses)
	}
}

func TestMCPBackendStdioResourcesPromptsEnvAndCwd(t *testing.T) {
	root := t.TempDir()
	workingDirectory := filepath.Join(root, "nested")
	if err := os.Mkdir(workingDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	cwdMarker := filepath.Join(t.TempDir(), "cwd")
	t.Setenv("VIVY_MCP_HELPER", "1")
	t.Setenv("VIVY_MCP_CWD_MARKER", cwdMarker)
	backend := NewMCPBackendWithOptions([]MCPServerConfig{{
		Name:    "local",
		Command: mcpHelperCommand(t),
		Args:    []string{"-test.run=TestMCPStdioHelperProcess", "", "  keep  "},
		EnvFrom: map[string]string{"MCP_HELPER": "VIVY_MCP_HELPER", "MCP_CWD_MARKER": "VIVY_MCP_CWD_MARKER"},
		Cwd:     "nested",
	}}, nil, MCPBackendOptions{ProcessRoot: root})
	t.Cleanup(func() { _ = backend.Close() })

	resources, err := backend.ListResources(context.Background(), "", "local")
	if err != nil || len(resources.Resources) != 1 || resources.Resources[0].URI != "stdio://resource" {
		t.Fatalf("stdio resources=%#v err=%v", resources, err)
	}
	read, err := backend.ReadResource(context.Background(), "", tools.MCPReadResourceRequest{Server: "local", URI: "stdio://resource"})
	if err != nil || len(read.Contents) != 1 || read.Contents[0].Text == nil || *read.Contents[0].Text != "stdio resource output" {
		t.Fatalf("stdio resource read=%#v err=%v", read, err)
	}
	prompts, err := backend.ListPrompts(context.Background(), "", "local")
	if err != nil || len(prompts.Prompts) != 1 || prompts.Prompts[0].Name != "review" || len(prompts.Prompts[0].Arguments) != 1 || !prompts.Prompts[0].Arguments[0].Required {
		t.Fatalf("stdio prompts=%#v err=%v", prompts, err)
	}
	got, err := backend.GetPrompt(context.Background(), "", tools.MCPGetPromptRequest{Server: "local", Name: "review", Arguments: map[string]string{"focus": "security"}})
	if err != nil || got.Text != "stdio prompt output" || got.Description != "stdio prompt" {
		t.Fatalf("stdio prompt=%#v err=%v", got, err)
	}
	actualCWD, err := os.ReadFile(cwdMarker)
	if err != nil {
		t.Fatal(err)
	}
	wantCWD, err := filepath.EvalSymlinks(workingDirectory)
	if err != nil {
		t.Fatal(err)
	}
	if string(actualCWD) != wantCWD {
		t.Fatalf("stdio cwd=%q, want %q", actualCWD, wantCWD)
	}
}

func TestMCPBackendStdioSpawnAndHandshakeFailuresStayFailClosed(t *testing.T) {
	t.Run("spawn", func(t *testing.T) {
		command := filepath.Join(t.TempDir(), "missing-mcp.exe")
		backend := NewMCPBackendWithOptions([]MCPServerConfig{{Name: "missing", Command: command}}, nil, MCPBackendOptions{ProcessRoot: t.TempDir()})
		t.Cleanup(func() { _ = backend.Close() })
		_, firstErr := backend.ListTools(context.Background(), "", "missing")
		if firstErr == nil {
			t.Fatal("missing executable unexpectedly started")
		}
		_, secondErr := backend.ListTools(context.Background(), "", "missing")
		if secondErr == nil || secondErr.Error() != firstErr.Error() {
			t.Fatalf("spawn failure was not cached: first=%v second=%v", firstErr, secondErr)
		}
		statuses := backend.ServerStatuses()
		if len(statuses) != 1 || statuses[0].Transport != "stdio" || statuses[0].Error == "" || strings.Contains(statuses[0].Error, command) || len([]rune(statuses[0].Error)) > 160 {
			t.Fatalf("spawn failure status=%+v", statuses)
		}
	})

	t.Run("handshake", func(t *testing.T) {
		marker := filepath.Join(t.TempDir(), "spawn-count")
		t.Setenv("VIVY_MCP_HELPER", "1")
		t.Setenv("VIVY_MCP_MARKER", marker)
		t.Setenv("VIVY_MCP_HANDSHAKE_FAIL", "1")
		backend := NewMCPBackendWithOptions([]MCPServerConfig{{
			Name:    "handshake",
			Command: mcpHelperCommand(t),
			Args:    []string{"-test.run=TestMCPStdioHelperProcess"},
			EnvFrom: map[string]string{"MCP_HELPER": "VIVY_MCP_HELPER", "MCP_MARKER": "VIVY_MCP_MARKER", "MCP_HANDSHAKE_FAIL": "VIVY_MCP_HANDSHAKE_FAIL"},
		}}, nil, MCPBackendOptions{ProcessRoot: t.TempDir()})
		t.Cleanup(func() { _ = backend.Close() })
		_, firstErr := backend.ListTools(context.Background(), "", "handshake")
		if firstErr == nil || !strings.Contains(firstErr.Error(), "initialize") {
			t.Fatalf("handshake error=%v", firstErr)
		}
		_, secondErr := backend.ListTools(context.Background(), "", "handshake")
		if secondErr == nil || secondErr.Error() != firstErr.Error() {
			t.Fatalf("handshake failure was not cached: first=%v second=%v", firstErr, secondErr)
		}
		if count, _ := os.ReadFile(marker); string(count) != "1" {
			t.Fatalf("handshake failure restarted process, spawn count=%q", count)
		}
		statuses := backend.ServerStatuses()
		if len(statuses) != 1 || statuses[0].Initialized || statuses[0].Error == "" || len([]rune(statuses[0].Error)) > 160 {
			t.Fatalf("handshake failure status=%+v", statuses)
		}
	})
}

func TestMCPBackendStdioReplaceRebuildsAndCloseIsIdempotent(t *testing.T) {
	root := t.TempDir()
	firstCWD := filepath.Join(root, "one")
	secondCWD := filepath.Join(root, "two")
	if err := os.MkdirAll(firstCWD, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(secondCWD, 0o700); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(t.TempDir(), "spawn-count")
	cwdMarker := filepath.Join(t.TempDir(), "cwd")
	t.Setenv("VIVY_MCP_HELPER", "1")
	t.Setenv("VIVY_MCP_MARKER", marker)
	t.Setenv("VIVY_MCP_CWD_MARKER", cwdMarker)
	config := func(cwd string) MCPServerConfig {
		return MCPServerConfig{
			Name:    "replaceable",
			Command: mcpHelperCommand(t),
			Args:    []string{"-test.run=TestMCPStdioHelperProcess"},
			EnvFrom: map[string]string{"MCP_HELPER": "VIVY_MCP_HELPER", "MCP_MARKER": "VIVY_MCP_MARKER", "MCP_CWD_MARKER": "VIVY_MCP_CWD_MARKER"},
			Cwd:     cwd,
		}
	}
	backend := NewMCPBackendWithOptions([]MCPServerConfig{config("one")}, nil, MCPBackendOptions{ProcessRoot: root})
	if _, err := backend.ListTools(context.Background(), "", "replaceable"); err != nil {
		t.Fatal(err)
	}
	backend.ReplaceServers([]MCPServerConfig{config("two")})
	if _, err := backend.ListTools(context.Background(), "", "replaceable"); err != nil {
		t.Fatal(err)
	}
	if count, _ := os.ReadFile(marker); string(count) != "2" {
		t.Fatalf("replacement spawn count=%q, want 2", count)
	}
	actualCWD, err := os.ReadFile(cwdMarker)
	if err != nil {
		t.Fatal(err)
	}
	wantCWD, err := filepath.EvalSymlinks(secondCWD)
	if err != nil {
		t.Fatal(err)
	}
	if string(actualCWD) != wantCWD {
		t.Fatalf("replacement cwd=%q, want %q", actualCWD, wantCWD)
	}
	if err := backend.Close(); err != nil {
		t.Fatal(err)
	}
	if err := backend.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := backend.ListTools(context.Background(), "", "replaceable"); err == nil || !strings.Contains(err.Error(), "backend is closed") {
		t.Fatalf("closed backend operation error=%v", err)
	}
}

func mcpHelperCommand(t *testing.T) string {
	t.Helper()
	path, err := filepath.Abs(os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	return path
}

func TestMCPBackendStdioMissingEnvIsVisibleAndFailClosed(t *testing.T) {
	t.Setenv("VIVY_MCP_REQUIRED", "")
	backend := NewMCPBackendWithOptions([]MCPServerConfig{{
		Name:    "missing-env",
		Command: mcpHelperCommand(t),
		Args:    []string{"-test.run=TestMCPStdioHelperProcess"},
		EnvFrom: map[string]string{"MCP_HELPER": "VIVY_MCP_REQUIRED"},
	}}, nil, MCPBackendOptions{ProcessRoot: t.TempDir()})
	t.Cleanup(func() { _ = backend.Close() })
	if _, err := backend.ListTools(context.Background(), "", "missing-env"); err == nil || !strings.Contains(err.Error(), "required environment variable") {
		t.Fatalf("missing env error=%v", err)
	}
	statuses := backend.ServerStatuses()
	if len(statuses) != 1 || statuses[0].Transport != "stdio" || statuses[0].Error == "" || !strings.Contains(statuses[0].Error, "required environment variables") || len(statuses[0].EnvMissing) != 1 || statuses[0].EnvMissing[0] != "MCP_HELPER" || strings.Contains(statuses[0].Error, "VIVY_MCP_REQUIRED") {
		t.Fatalf("missing env status=%+v", statuses)
	}
	if _, err := backend.ListTools(context.Background(), "", "missing-env"); err == nil {
		t.Fatal("missing env server should remain fail-closed")
	}
}

func TestMCPStatusErrorSanitizesPathsAndEnvironmentValues(t *testing.T) {
	root := filepath.Join(t.TempDir(), "workspace-root")
	command := filepath.Join(root, "trusted", "mcp-server.exe")
	t.Setenv("VIVY_MCP_SECRET", "resolved-secret-value")
	backend := NewMCPBackendWithOptions([]MCPServerConfig{{
		Name: "local", Command: command, Cwd: "trusted", EnvFrom: map[string]string{"TOKEN": "VIVY_MCP_SECRET"},
	}}, nil, MCPBackendOptions{ProcessRoot: root})
	t.Cleanup(func() { _ = backend.Close() })
	raw := fmt.Sprintf("open %s cwd=%s root=%s env=resolved-secret-value\x1b[31m\u202e", command, filepath.Join(root, "trusted"), root)
	got := backend.SanitizeMCPError("local", errors.New(raw))
	if len([]rune(got)) > 160 || strings.Contains(got, command) || strings.Contains(got, root) || strings.Contains(got, "trusted") || strings.Contains(got, "resolved-secret-value") || strings.ContainsAny(got, "\x1b\u202e") {
		t.Fatalf("unsafe MCP status error=%q", got)
	}
}

func TestMCPBackendStdioDeathDoesNotRestart(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "spawn-count")
	t.Setenv("VIVY_MCP_HELPER", "1")
	t.Setenv("VIVY_MCP_MARKER", marker)
	t.Setenv("VIVY_MCP_EXIT_AFTER_LIST", "1")
	backend := NewMCPBackendWithOptions([]MCPServerConfig{{
		Name:    "dies",
		Command: mcpHelperCommand(t),
		Args:    []string{"-test.run=TestMCPStdioHelperProcess"},
		EnvFrom: map[string]string{"MCP_HELPER": "VIVY_MCP_HELPER", "MCP_MARKER": "VIVY_MCP_MARKER", "MCP_EXIT_AFTER_LIST": "VIVY_MCP_EXIT_AFTER_LIST"},
	}}, nil, MCPBackendOptions{ProcessRoot: t.TempDir()})
	t.Cleanup(func() { _ = backend.Close() })
	if _, err := backend.ListTools(context.Background(), "", "dies"); err != nil {
		t.Fatalf("first stdio list: %v", err)
	}
	if _, err := backend.ListTools(context.Background(), "", "dies"); err == nil || !strings.Contains(strings.ToLower(err.Error()), "stdio process exited") {
		t.Fatalf("second stdio list error=%v", err)
	}
	if count, _ := os.ReadFile(marker); string(count) != "1" {
		t.Fatalf("stdio restarted after death, spawn count=%q", count)
	}
	statuses := backend.ServerStatuses()
	if len(statuses) != 1 || statuses[0].Initialized || !strings.Contains(statuses[0].Error, "stdio process exited") {
		t.Fatalf("dead stdio status=%+v", statuses)
	}
}

func simpleToolsDispatch(name string) func(testMCPRequest) testMCPResponse {
	return func(request testMCPRequest) testMCPResponse {
		switch request.Method {
		case "initialize":
			return testMCPResponse{Status: http.StatusAccepted, Result: initializeResult(map[string]any{"tools": map[string]any{}})}
		case "tools/list":
			return testMCPResponse{Result: map[string]any{"tools": []map[string]any{{"name": name, "inputSchema": map[string]any{"type": "object"}}}}}
		default:
			return testMCPResponse{NoBody: true}
		}
	}
}

func containsStringMCP(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

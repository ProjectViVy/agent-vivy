package runtime

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"agent-vivy/internal/tools"
)

func TestMCPBackendListsCallsAndReconnects(t *testing.T) {
	var initializes atomic.Int32
	var listCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Method string          `json:"method"`
			ID     json.RawMessage `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch request.Method {
		case "initialize":
			initializes.Add(1)
			w.Header().Set("Mcp-Session-Id", "session")
			writeMCPResult(w, request.ID, `{"protocolVersion":"2024-11-05"}`)
		case "notifications/initialized":
			_, _ = w.Write([]byte(`{}`))
		case "tools/list":
			if listCalls.Add(1) == 1 {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			writeMCPResult(w, request.ID, `{"tools":[{"name":"echo","description":"remote echo","inputSchema":{"type":"object"}}]}`)
		case "tools/call":
			writeMCPResult(w, request.ID, `{"content":[{"type":"text","text":"remote output"}]}`)
		default:
			t.Errorf("unexpected MCP method %q", request.Method)
		}
	}))
	defer server.Close()

	backend := NewMCPBackend([]MCPServerConfig{{Name: "local", Endpoint: server.URL}}, server.Client())
	if statuses := backend.ServerStatuses(); len(statuses) != 1 || statuses[0].Name != "local" || statuses[0].Initialized {
		t.Fatalf("initial statuses = %+v", statuses)
	}
	listed, err := backend.ListTools(context.Background(), "", "local")
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}
	if len(listed.Tools) != 1 || listed.Tools[0].Server != "local" || listed.Tools[0].Name != "echo" || !listed.Untrusted {
		t.Fatalf("listed = %#v", listed)
	}
	if initializes.Load() != 2 || listCalls.Load() != 2 {
		t.Fatalf("reconnect counts initialize=%d list=%d", initializes.Load(), listCalls.Load())
	}
	if statuses := backend.ServerStatuses(); len(statuses) != 1 || !statuses[0].Initialized {
		t.Fatalf("connected statuses = %+v", statuses)
	}

	called, err := backend.CallTool(context.Background(), "", tools.MCPCallRequest{Server: "local", Tool: "echo", Arguments: map[string]any{"text": "hi"}})
	if err != nil {
		t.Fatalf("call tool: %v", err)
	}
	if called.Server != "local" || called.Tool != "echo" || len(called.Content) != 1 || called.Content[0].Text != "remote output" || !called.Untrusted {
		t.Fatalf("called = %#v", called)
	}
}
func TestMCPBackendBoundsRemoteOutputAndRejectsUnknownServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Method string          `json:"method"`
			ID     json.RawMessage `json:"id"`
		}
		_ = json.NewDecoder(r.Body).Decode(&request)
		w.Header().Set("Content-Type", "application/json")
		switch request.Method {
		case "initialize":
			writeMCPResult(w, request.ID, `{"protocolVersion":"2024-11-05"}`)
		case "notifications/initialized":
			_, _ = w.Write([]byte(`{}`))
		case "tools/call":
			writeMCPResult(w, request.ID, `{"content":[{"type":"text","text":"`+strings.Repeat("x", maxMCPContentBytes+1024)+`"}]}`)
		}
	}))
	defer server.Close()
	backend := NewMCPBackend([]MCPServerConfig{{Name: "local", Endpoint: server.URL}}, server.Client())
	called, err := backend.CallTool(context.Background(), "", tools.MCPCallRequest{Server: "local", Tool: "big"})
	if err != nil {
		t.Fatalf("call tool: %v", err)
	}
	if len(called.Content) != 1 || len(called.Content[0].Text) != maxMCPContentBytes {
		t.Fatalf("bounded content len = %d", len(called.Content[0].Text))
	}
	if _, err := backend.ListTools(context.Background(), "", "missing"); err == nil || !strings.Contains(err.Error(), "not configured") {
		t.Fatalf("unknown server error = %v", err)
	}
}

func TestMCPBackendReplaceServersSwapsCatalog(t *testing.T) {
	first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeMCPJSON(w, r, `{"tools":[{"name":"old","description":"first"}]}`)
	}))
	defer first.Close()
	second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeMCPJSON(w, r, `{"tools":[{"name":"new","description":"second"}]}`)
	}))
	defer second.Close()

	backend := NewMCPBackend([]MCPServerConfig{{Name: "local", Endpoint: first.URL}}, first.Client())
	listed, err := backend.ListTools(context.Background(), "", "local")
	if err != nil {
		t.Fatalf("list first: %v", err)
	}
	if len(listed.Tools) != 1 || listed.Tools[0].Name != "old" {
		t.Fatalf("first catalog = %#v", listed)
	}

	backend.ReplaceServers([]MCPServerConfig{{Name: "local", Endpoint: second.URL}})
	listed, err = backend.ListTools(context.Background(), "", "local")
	if err != nil {
		t.Fatalf("list second: %v", err)
	}
	if len(listed.Tools) != 1 || listed.Tools[0].Name != "new" {
		t.Fatalf("replaced catalog = %#v", listed)
	}
	if _, err := backend.ListTools(context.Background(), "", "gone"); err == nil {
		t.Fatal("removed server must fail")
	}
}

func TestMCPBackendParsesSSEJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Method string `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&request)
		w.Header().Set("Content-Type", "text/event-stream")
		switch request.Method {
		case "initialize":
			_, _ = w.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"protocolVersion\":\"2024-11-05\"}}\n\n"))
		case "notifications/initialized":
			_, _ = w.Write([]byte("event: message\ndata: {}\n\n"))
		case "tools/list":
			_, _ = w.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":2,\"result\":{\"tools\":[{\"name\":\"sse_echo\"}]}}\n\n"))
		default:
			t.Errorf("unexpected MCP method %q", request.Method)
		}
	}))
	defer server.Close()
	backend := NewMCPBackend([]MCPServerConfig{{Name: "sse", Endpoint: server.URL}}, server.Client())
	listed, err := backend.ListTools(context.Background(), "", "sse")
	if err != nil {
		t.Fatalf("list sse: %v", err)
	}
	if len(listed.Tools) != 1 || listed.Tools[0].Name != "sse_echo" {
		t.Fatalf("sse catalog = %#v", listed)
	}
}

func TestMCPBackendListsAndReadsResourcesJSON(t *testing.T) {
	var methods []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Method string `json:"method"`
			Params struct {
				Cursor string `json:"cursor"`
			} `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		methods = append(methods, request.Method)
		w.Header().Set("Content-Type", "application/json")
		if request.Method != "initialize" && r.Header.Get("MCP-Protocol-Version") != "2024-11-05" {
			t.Errorf("%s protocol header = %q", request.Method, r.Header.Get("MCP-Protocol-Version"))
		}
		switch request.Method {
		case "initialize":
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2024-11-05","capabilities":{"resources":{"listChanged":true}}}}`))
		case "notifications/initialized":
			_, _ = w.Write([]byte(`{}`))
		case "resources/list":
			if request.Params.Cursor == "" {
				_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":2,"result":{"resources":[{"uri":"docs://guide","name":"guide","title":"Guide","description":"remote guide","mimeType":"text/markdown","size":42,"annotations":{"audience":["user"]},"_meta":{"origin":"remote"}}],"nextCursor":" page-2 "}}`))
			} else if request.Params.Cursor == " page-2 " {
				_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":3,"result":{"resources":[{"uri":"docs://faq","name":"faq"}]}}`))
			} else {
				t.Errorf("unexpected resources cursor %q", request.Params.Cursor)
			}
		case "resources/read":
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":4,"result":{"contents":[{"uri":"docs://guide","mimeType":"text/markdown","text":"# Hello"},{"uri":"docs://guide","mimeType":"application/octet-stream","blob":"AQI="}]}}`))
		default:
			t.Errorf("unexpected MCP method %q", request.Method)
		}
	}))
	defer server.Close()

	backend := NewMCPBackend([]MCPServerConfig{{Name: "docs", Endpoint: server.URL}}, server.Client())
	listed, err := backend.ListResources(context.Background(), "", "docs")
	if err != nil {
		t.Fatalf("list resources: %v", err)
	}
	if listed.Server != "docs" || !listed.Untrusted || len(listed.Resources) != 2 {
		t.Fatalf("listed = %#v", listed)
	}
	resource := listed.Resources[0]
	if resource.Server != "docs" || resource.URI != "docs://guide" || resource.Name != "guide" || resource.Title != "Guide" || resource.Description != "remote guide" || resource.MIME != "text/markdown" || resource.Size != 42 {
		t.Fatalf("resource = %#v", resource)
	}
	if string(resource.Annotations) != `{"audience":["user"]}` || string(resource.Meta) != `{"origin":"remote"}` {
		t.Fatalf("resource metadata = annotations %s meta %s", resource.Annotations, resource.Meta)
	}

	read, err := backend.ReadResource(context.Background(), "", tools.MCPReadResourceRequest{Server: "docs", URI: "docs://guide"})
	if err != nil {
		t.Fatalf("read resource: %v", err)
	}
	if read.Server != "docs" || read.URI != "docs://guide" || !read.Untrusted || len(read.Contents) != 2 {
		t.Fatalf("read = %#v", read)
	}
	if got := read.Contents[0]; got.URI != "docs://guide" || got.MIME != "text/markdown" || got.Text == nil || *got.Text != "# Hello" || got.Blob != nil {
		t.Fatalf("text content = %#v", got)
	}
	if got := read.Contents[1]; got.MIME != "application/octet-stream" || got.Blob == nil || *got.Blob != "AQI=" || got.Text != nil {
		t.Fatalf("blob content = %#v", got)
	}
	if strings.Join(methods, ",") != "initialize,notifications/initialized,resources/list,resources/list,resources/read" {
		t.Fatalf("methods = %v", methods)
	}

	if _, err := backend.ListResources(context.Background(), "", "missing"); err == nil || !strings.Contains(err.Error(), "not configured") {
		t.Fatalf("unknown list error = %v", err)
	}
	if _, err := backend.ReadResource(context.Background(), "", tools.MCPReadResourceRequest{Server: "missing", URI: "docs://guide"}); err == nil || !strings.Contains(err.Error(), "not configured") {
		t.Fatalf("unknown read error = %v", err)
	}
}

func TestMCPBackendListsAndGetsPrompts(t *testing.T) {
	var methods []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Method string `json:"method"`
			Params struct {
				Name      string            `json:"name"`
				Arguments map[string]string `json:"arguments"`
			} `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		methods = append(methods, request.Method)
		w.Header().Set("Content-Type", "application/json")
		switch request.Method {
		case "initialize":
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2024-11-05","capabilities":{"prompts":{"listChanged":true}}}}`))
		case "notifications/initialized":
			_, _ = w.Write([]byte(`{}`))
		case "prompts/list":
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":2,"result":{"prompts":[{"name":"review","title":"Review","description":"Review a change","arguments":[{"name":"focus","description":"review focus","required":true}]}]}}`))
		case "prompts/get":
			if request.Params.Name != "review" || request.Params.Arguments["focus"] != "security" {
				t.Errorf("get params = %#v", request.Params)
			}
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":3,"result":{"description":"expanded review","messages":[{"role":"user","content":{"type":"text","text":"Review security"}},{"role":"assistant","content":{"type":"text","text":"Check boundaries"}}]}}`))
		default:
			t.Errorf("unexpected MCP method %q", request.Method)
		}
	}))
	defer server.Close()

	backend := NewMCPBackend([]MCPServerConfig{{Name: "docs", Endpoint: server.URL}}, server.Client())
	listed, err := backend.ListPrompts(context.Background(), "", "docs")
	if err != nil {
		t.Fatalf("list prompts: %v", err)
	}
	if !listed.Untrusted || len(listed.Prompts) != 1 {
		t.Fatalf("listed = %#v", listed)
	}
	prompt := listed.Prompts[0]
	if prompt.Server != "docs" || prompt.Name != "review" || prompt.Title != "Review" || len(prompt.Arguments) != 1 || !prompt.Arguments[0].Required {
		t.Fatalf("prompt = %#v", prompt)
	}
	expanded, err := backend.GetPrompt(context.Background(), "", tools.MCPGetPromptRequest{Server: "docs", Name: "review", Arguments: map[string]string{"focus": "security"}})
	if err != nil {
		t.Fatalf("get prompt: %v", err)
	}
	if !expanded.Untrusted || expanded.Text != "Review security" {
		t.Fatalf("expanded = %#v", expanded)
	}
	if strings.Join(methods, ",") != "initialize,notifications/initialized,prompts/list,prompts/get" {
		t.Fatalf("methods = %v", methods)
	}
}

func TestMCPBackendIgnoresNonUserPromptContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Method string `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&request)
		w.Header().Set("Content-Type", "application/json")
		switch request.Method {
		case "initialize":
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2024-11-05","capabilities":{"prompts":{}}}}`))
		case "notifications/initialized":
			_, _ = w.Write([]byte(`{}`))
		case "prompts/get":
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":2,"result":{"messages":[{"role":"user","content":{"type":"image","data":"secret"}}]}}`))
		}
	}))
	defer server.Close()
	backend := NewMCPBackend([]MCPServerConfig{{Name: "docs", Endpoint: server.URL}}, server.Client())
	_, err := backend.GetPrompt(context.Background(), "", tools.MCPGetPromptRequest{Server: "docs", Name: "image"})
	if err == nil || !strings.Contains(err.Error(), "no user text") {
		t.Fatalf("non-text error = %v", err)
	}
}

func TestMCPBackendSkipsServerWithoutPromptCapability(t *testing.T) {
	var methods []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Method string `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&request)
		methods = append(methods, request.Method)
		w.Header().Set("Content-Type", "application/json")
		switch request.Method {
		case "initialize":
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2024-11-05","capabilities":{"tools":{}}}}`))
		case "notifications/initialized":
			_, _ = w.Write([]byte(`{}`))
		default:
			t.Errorf("unsupported server received %q", request.Method)
		}
	}))
	defer server.Close()
	backend := NewMCPBackend([]MCPServerConfig{{Name: "tools-only", Endpoint: server.URL}}, server.Client())
	listed, err := backend.ListPrompts(context.Background(), "", "")
	if err != nil || len(listed.Prompts) != 0 || strings.Join(methods, ",") != "initialize,notifications/initialized" {
		t.Fatalf("unsupported prompt catalog = %+v methods=%v err=%v", listed, methods, err)
	}
}

func TestMCPBackendParsesSSEResources(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Method string `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&request)
		w.Header().Set("Content-Type", "text/event-stream")
		switch request.Method {
		case "initialize":
			_, _ = w.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"protocolVersion\":\"2024-11-05\"}}\n\n"))
		case "notifications/initialized":
			_, _ = w.Write([]byte("event: message\ndata: {}\n\n"))
		case "resources/list":
			_, _ = w.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"method\":\"notifications/progress\",\"params\":{}}\n\nevent: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":2,\"result\":{\"resources\":[{\"uri\":\"sse://one\",\"name\":\"one\"}]}}\n\n"))
		case "resources/read":
			_, _ = w.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":3,\"result\":{\"contents\":[{\"uri\":\"sse://one\",\"text\":\"sse text\"}]}}\n\n"))
		default:
			t.Errorf("unexpected MCP method %q", request.Method)
		}
	}))
	defer server.Close()

	backend := NewMCPBackend([]MCPServerConfig{{Name: "sse", Endpoint: server.URL}}, server.Client())
	listed, err := backend.ListResources(context.Background(), "", "sse")
	if err != nil || len(listed.Resources) != 1 || listed.Resources[0].URI != "sse://one" {
		t.Fatalf("sse resources = %#v err=%v", listed, err)
	}
	read, err := backend.ReadResource(context.Background(), "", tools.MCPReadResourceRequest{Server: "sse", URI: "sse://one"})
	if err != nil || len(read.Contents) != 1 || read.Contents[0].Text == nil || *read.Contents[0].Text != "sse text" {
		t.Fatalf("sse read = %#v err=%v", read, err)
	}
}

func TestMCPBackendBoundsResourceContentAndResponses(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Method string `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&request)
		w.Header().Set("Content-Type", "application/json")
		switch request.Method {
		case "initialize":
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2024-11-05"}}`))
		case "notifications/initialized":
			_, _ = w.Write([]byte(`{}`))
		case "resources/read":
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":2,"result":{"contents":[{"uri":"docs://big","text":"` + strings.Repeat("x", maxMCPContentBytes+1024) + `"}]}}`))
		case "resources/list":
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":3,"result":{"resources":[{"uri":"docs://big","description":"` + strings.Repeat("d", maxMCPContentBytes+1024) + `","annotations":{"large":"` + strings.Repeat("a", 1024) + `"}}]}}`))
		}
	}))
	defer server.Close()

	backend := NewMCPBackend([]MCPServerConfig{{Name: "docs", Endpoint: server.URL}}, server.Client())
	read, err := backend.ReadResource(context.Background(), "", tools.MCPReadResourceRequest{Server: "docs", URI: "docs://big"})
	if err != nil {
		t.Fatalf("read bounded resource: %v", err)
	}
	if len(read.Contents) != 1 || read.Contents[0].Text == nil || len(*read.Contents[0].Text) == 0 || read.Contents[0].Blob != nil || mcpResourceContentSize(read.Contents[0]) > maxMCPContentBytes {
		t.Fatalf("bounded content = %#v", read.Contents[0])
	}
	listed, err := backend.ListResources(context.Background(), "", "docs")
	if err != nil {
		t.Fatalf("list bounded resources: %v", err)
	}
	if len(listed.Resources) != 1 || mcpResourceSize(listed.Resources[0]) > maxMCPContentBytes {
		t.Fatalf("bounded list = %#v", listed)
	}

	backend.maxResponseBytes = 128
	if _, err := backend.ListResources(context.Background(), "", "docs"); err == nil || !strings.Contains(err.Error(), "size limit") {
		t.Fatalf("oversize response error = %v", err)
	}
	if _, err := backend.ReadResource(context.Background(), "", tools.MCPReadResourceRequest{Server: "docs", URI: strings.Repeat("u", maxMCPContentBytes+1)}); err == nil || !strings.Contains(err.Error(), "URI exceeds size limit") {
		t.Fatalf("oversize URI error = %v", err)
	}
}

func TestMCPBackendRejectsMalformedResourceContentAndPreservesURI(t *testing.T) {
	var response string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Method string          `json:"method"`
			ID     json.RawMessage `json:"id"`
			Params struct {
				URI string `json:"uri"`
			} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&request)
		w.Header().Set("Content-Type", "application/json")
		switch request.Method {
		case "initialize":
			writeMCPResult(w, request.ID, `{"protocolVersion":"2024-11-05"}`)
		case "notifications/initialized":
			_, _ = w.Write([]byte(`{}`))
		case "resources/read":
			if request.Params.URI != " docs://opaque " {
				t.Errorf("URI changed in transit: %q", request.Params.URI)
			}
			writeMCPResult(w, request.ID, response)
		}
	}))
	defer server.Close()

	backend := NewMCPBackend([]MCPServerConfig{{Name: "docs", Endpoint: server.URL}}, server.Client())
	for _, malformed := range []string{
		`{"contents":[{"text":"missing uri"}]}`,
		`{"contents":[{"uri":"docs://bad"}]}`,
		`{"contents":[{"uri":"docs://bad","text":"x","blob":"eA=="}]}`,
	} {
		response = malformed
		if _, err := backend.ReadResource(context.Background(), "", tools.MCPReadResourceRequest{Server: "docs", URI: " docs://opaque "}); err == nil {
			t.Fatalf("malformed resource content accepted: %s", malformed)
		}
	}
	for _, valid := range []struct {
		response string
		text     bool
	}{
		{response: `{"contents":[{"uri":"docs://empty","text":""}]}`, text: true},
		{response: `{"contents":[{"uri":"docs://empty","blob":""}]}`},
	} {
		response = valid.response
		read, err := backend.ReadResource(context.Background(), "", tools.MCPReadResourceRequest{Server: "docs", URI: " docs://opaque "})
		if err != nil || len(read.Contents) != 1 {
			t.Fatalf("valid empty resource rejected: response=%s read=%#v err=%v", valid.response, read, err)
		}
		if valid.text && (read.Contents[0].Text == nil || read.Contents[0].Blob != nil) {
			t.Fatalf("empty text representation lost: %#v", read.Contents[0])
		}
		if !valid.text && (read.Contents[0].Blob == nil || read.Contents[0].Text != nil) {
			t.Fatalf("empty blob representation lost: %#v", read.Contents[0])
		}
	}
}

func TestMCPBackendBoundsMultiServerResourceFanout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Method string          `json:"method"`
			ID     json.RawMessage `json:"id"`
		}
		_ = json.NewDecoder(r.Body).Decode(&request)
		w.Header().Set("Content-Type", "application/json")
		switch request.Method {
		case "initialize":
			writeMCPResult(w, request.ID, `{"protocolVersion":"2024-11-05"}`)
		case "notifications/initialized":
			_, _ = w.Write([]byte(`{}`))
		case "resources/list":
			writeMCPResult(w, request.ID, `{"resources":[{"uri":"docs://large","description":"`+strings.Repeat("x", 200<<10)+`"}]}`)
		}
	}))
	defer server.Close()

	backend := NewMCPBackend([]MCPServerConfig{{Name: "one", Endpoint: server.URL}, {Name: "two", Endpoint: server.URL}}, server.Client())
	listed, err := backend.ListResources(context.Background(), "", "")
	if err != nil {
		t.Fatal(err)
	}
	total := 0
	for _, resource := range listed.Resources {
		total += mcpResourceSize(resource)
	}
	if total > maxMCPContentBytes {
		t.Fatalf("fanout projection = %d bytes, limit = %d", total, maxMCPContentBytes)
	}
}

func mcpResourceSize(resource tools.MCPResource) int {
	return len(resource.Server) + len(resource.URI) + len(resource.Name) + len(resource.Title) + len(resource.Description) + len(resource.MIME) + len(resource.Annotations) + len(resource.Meta)
}

func mcpResourceContentSize(content tools.MCPResourceContent) int {
	size := len(content.URI) + len(content.MIME) + len(content.Annotations) + len(content.Meta)
	if content.Text != nil {
		size += len(*content.Text)
	}
	if content.Blob != nil {
		size += len(*content.Blob)
	}
	return size
}

func writeMCPJSON(w http.ResponseWriter, r *http.Request, result string) {
	var request struct {
		Method string          `json:"method"`
		ID     json.RawMessage `json:"id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&request)
	w.Header().Set("Content-Type", "application/json")
	switch request.Method {
	case "initialize":
		writeMCPResult(w, request.ID, `{"protocolVersion":"2024-11-05"}`)
	case "notifications/initialized":
		_, _ = w.Write([]byte(`{}`))
	case "tools/list":
		writeMCPResult(w, request.ID, result)
	}
}

func writeMCPResult(w http.ResponseWriter, id json.RawMessage, result string) {
	_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":` + string(id) + `,"result":` + result + `}`))
}

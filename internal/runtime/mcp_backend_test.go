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

func TestEinoMCPBackendListsCallsAndReconnects(t *testing.T) {
	var initializes atomic.Int32
	var listCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Method string `json:"method"`
			ID     any    `json:"id"`
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
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2024-11-05"}}`))
		case "notifications/initialized":
			_, _ = w.Write([]byte(`{}`))
		case "tools/list":
			if listCalls.Add(1) == 1 {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"tools":[{"name":"echo","description":"remote echo","inputSchema":{"type":"object"}}]}}`))
		case "tools/call":
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"remote output"}]}}`))
		default:
			t.Errorf("unexpected MCP method %q", request.Method)
		}
	}))
	defer server.Close()

	backend := NewEinoMCPBackend([]MCPServerConfig{{Name: "local", Endpoint: server.URL}}, server.Client())
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

	called, err := backend.CallTool(context.Background(), "", tools.MCPCallRequest{Server: "local", Tool: "echo", Arguments: map[string]any{"text": "hi"}})
	if err != nil {
		t.Fatalf("call tool: %v", err)
	}
	if called.Server != "local" || called.Tool != "echo" || len(called.Content) != 1 || called.Content[0].Text != "remote output" || !called.Untrusted {
		t.Fatalf("called = %#v", called)
	}
}
func TestEinoMCPBackendBoundsRemoteOutputAndRejectsUnknownServer(t *testing.T) {
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
		case "tools/call":
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"` + strings.Repeat("x", maxMCPContentBytes+1024) + `"}]}}`))
		}
	}))
	defer server.Close()
	backend := NewEinoMCPBackend([]MCPServerConfig{{Name: "local", Endpoint: server.URL}}, server.Client())
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

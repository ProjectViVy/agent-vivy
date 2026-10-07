package probe

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/a2aproject/a2a-go/v2/a2a"
)

// rawRPC posts one JSON-RPC payload and returns the decoded response.
func rawRPC(t *testing.T, url string, headers map[string]string, body any) (int, map[string]any) {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("response is not JSONRPC JSON: %v body=%q", err, data)
	}
	return resp.StatusCode, decoded
}

func rpcErrorCode(t *testing.T, resp map[string]any) float64 {
	t.Helper()
	e, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error payload, got %v", resp)
	}
	code, ok := e["code"].(float64)
	if !ok {
		t.Fatalf("error without numeric code: %v", e)
	}
	return code
}

// TestA2AJSONRPCWireCompatibility pins transport framing independent of the
// typed client: malformed payloads, unknown methods and service parameters are
// rejected or carried exactly as the protocol declares.
func TestA2AJSONRPCWireCompatibility(t *testing.T) {
	h := &probeHandler{sendResult: &a2a.Task{ID: "task-w", ContextID: "ctx-w", Status: a2a.TaskStatus{State: a2a.TaskStateSubmitted}}}
	srv := newProbeServer(t, h)

	// Note: the v2.6.0 SDK's JSONRPC binding uses Go-style method names
	// ("SendMessage", "GetTask", ...), not spec-style "message/send" paths.
	call := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "SendMessage",
		"params": map[string]any{
			"message": map[string]any{
				"messageId": "m-1",
				"role":      "ROLE_USER",
				"parts":     []map[string]any{{"text": "hi"}},
			},
		},
	}

	t.Run("unknownMethod", func(t *testing.T) {
		bad := map[string]any{"jsonrpc": "2.0", "id": 2, "method": "tasks/frobnicate"}
		_, resp := rawRPC(t, srv.URL, nil, bad)
		if code := rpcErrorCode(t, resp); code != -32601 {
			t.Fatalf("unknown method code = %v, want -32601", code)
		}
	})

	t.Run("badJSONRPCVersion", func(t *testing.T) {
		bad := map[string]any{"jsonrpc": "1.0", "id": 3, "method": "message/send"}
		_, resp := rawRPC(t, srv.URL, nil, bad)
		if code := rpcErrorCode(t, resp); code != -32600 {
			t.Fatalf("bad jsonrpc version code = %v, want -32600", code)
		}
	})

	t.Run("malformedBody", func(t *testing.T) {
		resp, err := http.Post(srv.URL, "application/json", bytes.NewReader([]byte("{not json")))
		if err != nil {
			t.Fatalf("post: %v", err)
		}
		defer resp.Body.Close()
		data, _ := io.ReadAll(resp.Body)
		var decoded map[string]any
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("error response is not JSONRPC JSON: %v", err)
		}
		if code := rpcErrorCode(t, decoded); code != -32700 {
			t.Fatalf("malformed body code = %v, want -32700", code)
		}
	})

	t.Run("nonPostRejected", func(t *testing.T) {
		resp, err := http.Get(srv.URL)
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		defer resp.Body.Close()
		data, _ := io.ReadAll(resp.Body)
		var decoded map[string]any
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("GET rejection is not JSONRPC JSON: %v", err)
		}
		if _, ok := decoded["error"]; !ok {
			t.Fatalf("GET did not produce JSONRPC error: %v", decoded)
		}
	})

	t.Run("serviceParamsReachHandler", func(t *testing.T) {
		_, resp := rawRPC(t, srv.URL, map[string]string{
			"A2A-Version":    "1.0",
			"A2A-Extensions": "urn:probe:ext-a,urn:probe:ext-b",
		}, call)
		if _, isErr := resp["error"]; isErr {
			t.Fatalf("valid call rejected: %v", resp)
		}
		obs := h.last()
		if len(obs.versionParams) != 1 || obs.versionParams[0] != "1.0" {
			t.Fatalf("A2A-Version not carried to handler: %v", obs.versionParams)
		}
		if len(obs.extensionParams) != 1 || obs.extensionParams[0] != "urn:probe:ext-a,urn:probe:ext-b" {
			t.Fatalf("A2A-Extensions not carried to handler: %v", obs.extensionParams)
		}
	})

	t.Run("tenantPassthrough", func(t *testing.T) {
		withTenant := map[string]any{
			"jsonrpc": "2.0", "id": 9, "method": "SendMessage",
			"params": map[string]any{
				"tenant": "tenant-wire",
				"message": map[string]any{
					"messageId": "m-9", "role": "ROLE_USER",
					"parts": []map[string]any{{"text": "hi"}},
				},
			},
		}
		_, resp := rawRPC(t, srv.URL, nil, withTenant)
		if _, isErr := resp["error"]; isErr {
			t.Fatalf("tenant call rejected at transport: %v", resp)
		}
		if h.last().tenant != "tenant-wire" {
			t.Fatalf("tenant not passed to handler verbatim: %q", h.last().tenant)
		}
	})
}

// TestA2AEventSurfaceContract records which fields exist for stream replay:
// the standard events carry no resume cursor or sequence number.
func TestA2AEventSurfaceContract(t *testing.T) {
	for _, ev := range []a2a.Event{
		&a2a.TaskStatusUpdateEvent{},
		&a2a.TaskArtifactUpdateEvent{},
		&a2a.Task{},
		&a2a.Message{},
	} {
		raw, err := json.Marshal(ev)
		if err != nil {
			t.Fatalf("marshal %T: %v", ev, err)
		}
		var flat map[string]any
		if err := json.Unmarshal(raw, &flat); err != nil {
			t.Fatalf("unmarshal %T: %v", ev, err)
		}
		for _, banned := range []string{"cursor", "seq", "sequence", "resumeToken", "eventId"} {
			if _, present := flat[banned]; present {
				t.Fatalf("%T unexpectedly carries durable cursor field %q", ev, banned)
			}
		}
	}
	_ = context.Background()
	_ = time.Second
}

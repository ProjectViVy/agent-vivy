package facerun

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

type streamErrorHost struct {
	handler func(string, json.RawMessage)
	calls   []string
	stale   bool
}

func (*streamErrorHost) ModuleID() string                           { return "test/headless" }
func (h *streamErrorHost) OnEvent(fn func(string, json.RawMessage)) { h.handler = fn }
func (h *streamErrorHost) Call(_ context.Context, method string, _ any) (json.RawMessage, error) {
	h.calls = append(h.calls, method)
	switch method {
	case "session/create":
		return json.RawMessage(`{"id":"s"}`), nil
	case "turn/start":
		return json.RawMessage(`{"run_id":"r"}`), nil
	case "run/subscribe":
		h.handler("run/event", json.RawMessage(`{"subscription_id":"sub","event":{"run_id":"r","seq":1,"type":"model.delta","payload":{"delta":"kept"}}}`))
		id := "sub"
		if h.stale {
			id = "other"
		}
		raw, _ := json.Marshal(map[string]any{"subscription_id": id, "message": "malformed completion", "retryable": false})
		h.handler("run/stream_error", raw)
		if h.stale {
			h.handler("run/event", json.RawMessage(`{"subscription_id":"sub","event":{"run_id":"r","seq":2,"type":"run.completed","payload":{}}}`))
		}
		return json.RawMessage(`{"subscription_id":"sub"}`), nil
	}
	return json.RawMessage(`{}`), nil
}

func TestRunReportsSubscriptionProtocolError(t *testing.T) {
	for _, stale := range []bool{false, true} {
		host := &streamErrorHost{stale: stale}
		var out, diagnostics bytes.Buffer
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		result, err := Run(ctx, host, Options{Prompt: "test", Face: "test"}, &TextSink{Out: &out, Err: &diagnostics})
		cancel()
		if stale {
			if err != nil || result.Status != "completed" {
				t.Fatalf("stale error poisoned run: %+v %v", result, err)
			}
		} else {
			if err == nil || !strings.Contains(err.Error(), "malformed completion") || !strings.Contains(diagnostics.String(), "malformed completion") {
				t.Fatalf("stream failure: %+v %v %s", result, err, diagnostics.String())
			}
		}
		if !strings.HasPrefix(out.String(), "kept") {
			t.Fatalf("lost text: %q", out.String())
		}
		for _, call := range host.calls {
			if call == "run/cancel" {
				t.Fatal("protocol error changed backend run")
			}
		}
	}
}

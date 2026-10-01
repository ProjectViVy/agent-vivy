package embedded_test

// Host contract tests for the embedded owner (agent-diva DN-L).

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
	"sync/atomic"
	"testing"
	"time"

	"agent-vivy/internal/app"
	"agent-vivy/internal/config"
	"agent-vivy/internal/domain"
	"agent-vivy/internal/embedded"
	"agent-vivy/internal/runtime"
	"agent-vivy/internal/tools"
)

func newTestConfig(t *testing.T) config.Config {
	t.Helper()
	return config.Config{
		Server:  config.Server{Addr: "127.0.0.1:0"},
		Storage: config.Storage{Backend: "sqlite", SQLite: config.SQLite{Path: filepath.Join(t.TempDir(), "embedded.db")}},
		Providers: config.Providers{
			Active: "deepseek",
		},
		Runtime: config.Runtime{StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10, WorkspaceRoot: filepath.Join(t.TempDir(), "workspace")},
		Tools:   config.Tools{Enabled: []string{tools.WriteNoteName}, Approval: config.Approval{Expiration: 5 * time.Minute}},
		Governance: config.Governance{
			Profile: string(domain.PolicyProfileDefault),
			Profiles: map[string]config.GovernanceProfile{
				string(domain.PolicyProfileDefault): {Default: "allow", Rules: []config.GovernanceRule{{Tool: tools.WriteNoteName, Decision: "prompt"}}},
			},
		},
	}
}

// scriptedProvider answers the first call with a write_note tool call (which
// the policy profile holds for approval) and a plain text answer afterwards.
func scriptedProvider(t *testing.T) *httptest.Server {
	t.Helper()
	var calls atomic.Int64
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		streaming := strings.Contains(string(raw), `"stream":true`)
		if streaming {
			w.Header().Set("Content-Type", "text/event-stream")
		} else {
			w.Header().Set("Content-Type", "application/json")
		}
		flusher, _ := w.(http.Flusher)
		writeChunk := func(payload string) {
			_, _ = fmt.Fprintf(w, "data: %s\n\n", payload)
			if flusher != nil {
				flusher.Flush()
			}
		}
		if calls.Add(1) == 1 {
			if !streaming {
				_, _ = io.WriteString(w, `{"id":"chatcmpl-tool","object":"chat.completion","created":1,"model":"deepseek-flash","choices":[{"index":0,"message":{"role":"assistant","content":null,"tool_calls":[{"id":"call_1","type":"function","function":{"name":"`+tools.WriteNoteName+`","arguments":"{\"content\":\"hi\"}"}}]},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`)
				return
			}
			writeChunk(`{"id":"c1","object":"chat.completion.chunk","created":1,"model":"deepseek-flash","choices":[{"index":0,"delta":{"role":"assistant","tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"` + tools.WriteNoteName + `","arguments":""}}]},"finish_reason":null}]}`)
			writeChunk(`{"id":"c1","object":"chat.completion.chunk","created":1,"model":"deepseek-flash","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"content\":\"hi\"}"}}]},"finish_reason":null}]}`)
			writeChunk(`{"id":"c1","object":"chat.completion.chunk","created":1,"model":"deepseek-flash","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`)
			_, _ = io.WriteString(w, "data: [DONE]\n\n")
			return
		}
		if !streaming {
			_, _ = io.WriteString(w, `{"id":"chatcmpl-final","object":"chat.completion","created":1,"model":"deepseek-flash","choices":[{"index":0,"message":{"role":"assistant","content":"done"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`)
			return
		}
		writeChunk(`{"id":"c2","object":"chat.completion.chunk","created":1,"model":"deepseek-flash","choices":[{"index":0,"delta":{"role":"assistant","content":""},"finish_reason":null}]}`)
		writeChunk(`{"id":"c2","object":"chat.completion.chunk","created":1,"model":"deepseek-flash","choices":[{"index":0,"delta":{"content":"done"},"finish_reason":null}]}`)
		writeChunk(`{"id":"c2","object":"chat.completion.chunk","created":1,"model":"deepseek-flash","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`)
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
	}))
}

func openHost(t *testing.T, o embedded.Options) *embedded.Host {
	t.Helper()
	runtime.SetEngineVersionOverride(pinnedEinoVersionValue)
	t.Cleanup(func() { runtime.SetEngineVersionOverride("") })
	t.Setenv("DEEPSEEK_API_KEY", "embedded-test-key")
	t.Setenv("VIVY_PROVIDER", "deepseek")
	srv := scriptedProvider(t)
	t.Setenv("VIVY_API_BASE", srv.URL)
	t.Cleanup(srv.Close)

	h, err := embedded.Open(context.Background(), newTestConfig(t), o)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = h.Close() })
	return h
}

// pinnedEinoVersion mirrors the pin used by internal/app tests
// (realsmoke_test.go): the repository-pinned Eino version.
const pinnedEinoVersionValue = "v0.9.13"

func mustCall(t *testing.T, h *embedded.Host, method string, params any) map[string]any {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	raw, err := h.Call(ctx, method, params)
	if err != nil {
		t.Fatalf("%s: %v", method, err)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("%s: decode %s: %v", method, raw, err)
	}
	return out
}

func TestOpenCallInitialize(t *testing.T) {
	h := openHost(t, embedded.Options{AppOptions: []app.AppOption{app.WithoutEars()}})
	init := mustCall(t, h, "initialize", nil)
	if init["protocol_version"] != "vivy.rpc.v1" {
		t.Fatalf("protocol_version = %v", init["protocol_version"])
	}
	session := mustCall(t, h, "session/create", map[string]any{"title": "embedded"})
	if session["id"] == "" {
		t.Fatalf("session/create = %v", session)
	}
}

func TestPollCollectsRunEvents(t *testing.T) {
	h := openHost(t, embedded.Options{AppOptions: []app.AppOption{app.WithoutEars()}})
	session := mustCall(t, h, "session/create", map[string]any{"title": "poll"})
	sessionID, _ := session["id"].(string)
	turn := mustCall(t, h, "turn/start", map[string]any{"session_id": sessionID, "text": "hi"})
	runID, _ := turn["run_id"].(string)
	sub := mustCall(t, h, "run/subscribe", map[string]any{"run_id": runID})
	if sub["subscription_id"] == "" {
		t.Fatalf("run/subscribe = %v", sub)
	}

	deadline := time.Now().Add(15 * time.Second)
	seenStarted := false
	for time.Now().Before(deadline) && !seenStarted {
		res, err := h.Poll(context.Background(), 0)
		if err != nil {
			t.Fatalf("Poll: %v", err)
		}
		for _, ev := range res.Events {
			if ev.Method != "run/event" {
				continue
			}
			var params struct {
				Event struct {
					Type string `json:"type"`
				} `json:"event"`
			}
			_ = json.Unmarshal(ev.Params, &params)
			if params.Event.Type == "run.started" || params.Event.Type == "tool.approval_required" {
				seenStarted = true
			}
		}
		if !seenStarted {
			time.Sleep(25 * time.Millisecond)
		}
	}
	if !seenStarted {
		t.Fatal("no run events reached the embedded queue")
	}
}

func TestPollOverflowReportsGap(t *testing.T) {
	h := openHost(t, embedded.Options{QueueCapacity: 2, AppOptions: []app.AppOption{app.WithoutEars()}})
	session := mustCall(t, h, "session/create", map[string]any{"title": "overflow"})
	sessionID, _ := session["id"].(string)
	turn := mustCall(t, h, "turn/start", map[string]any{"session_id": sessionID, "text": "hi"})
	runID, _ := turn["run_id"].(string)
	mustCall(t, h, "run/subscribe", map[string]any{"run_id": runID})

	// Let events accumulate past capacity before polling.
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		// The run emits >2 events quickly (run.started, model.request, ...).
		time.Sleep(300 * time.Millisecond)
		res, err := h.Poll(context.Background(), 0)
		if err != nil {
			t.Fatalf("Poll: %v", err)
		}
		if len(res.Events) > 2 {
			t.Fatalf("poll drained %d events, capacity was 2", len(res.Events))
		}
		if res.Gap {
			return
		}
	}
	t.Fatal("overflow never reported a gap")
}

func TestCloseIsIdempotentAndRejectsCalls(t *testing.T) {
	h := openHost(t, embedded.Options{AppOptions: []app.AppOption{app.WithoutEars()}})
	if err := h.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := h.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
	if _, err := h.Call(context.Background(), "initialize", nil); err == nil {
		t.Fatal("Call after Close succeeded")
	}
	if _, err := h.Poll(context.Background(), 0); err == nil {
		t.Fatal("Poll after Close succeeded")
	}
}

func TestConcurrentCallPollClose(t *testing.T) {
	h := openHost(t, embedded.Options{AppOptions: []app.AppOption{app.WithoutEars()}})
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				_, _ = h.Call(context.Background(), "session/list", nil)
			}
		}()
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				_, _ = h.Poll(context.Background(), 0)
			}
		}()
	}
	wg.Wait()
	if err := h.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

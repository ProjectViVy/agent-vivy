package app

import (
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
	"sync/atomic"
	"testing"
	"time"

	"agent-vivy/internal/config"
	"agent-vivy/internal/domain"
	controlrpc "agent-vivy/internal/rpc"
	"agent-vivy/internal/runtime"
	"agent-vivy/internal/tools"
	"agent-vivy/sdk/plugin"
)

// The gateway-less assembly (VIVY-FACE-PACK §7, F1) drives the same control
// plane the web face reaches over WebSocket, but through a memory pipe. The
// model side is a local Anthropic-shaped server; the frozen ENV session
// points the composed resolver at it.

// The kernel composition unconditionally loads both provider bundles from
// BundleDir, so the temp dir needs both fixtures even though only the
// anthropic side is exercised.
func newAnthropicTestConfig(t *testing.T) config.Config {
	t.Helper()
	bundleDir := filepath.Join(t.TempDir(), "bundles")
	if err := os.MkdirAll(bundleDir, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"anthropic", "openai"} {
		raw, err := os.ReadFile(filepath.Join("..", "..", "fixtures", "provider", name+".yaml"))
		if err != nil {
			t.Fatalf("read %s fixture bundle: %v", name, err)
		}
		if err := os.WriteFile(filepath.Join(bundleDir, name+".yaml"), raw, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return config.Config{
		Server:    config.Server{Addr: "127.0.0.1:0"},
		Storage:   config.Storage{Backend: "sqlite", SQLite: config.SQLite{Path: filepath.Join(t.TempDir(), "facehost.db")}},
		Providers: config.Providers{Active: "anthropic", BundleDir: bundleDir, Anthropic: config.Provider{EnvKey: "ANTHROPIC_API_KEY", DefaultModel: "claude-sonnet-4-5"}},
		Runtime:   config.Runtime{StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10, WorkspaceRoot: filepath.Join(t.TempDir(), "workspace")},
		// write_note is non-readonly, so the ask approval policy holds the run
		// for a human decision; the governance rule additionally pins the
		// policy decision to prompt for determinism. The zero-valued
		// hand-built config needs the expiration spelled out.
		Tools: config.Tools{Enabled: []string{tools.WriteNoteName}, Approval: config.Approval{Expiration: 5 * time.Minute}},
		Governance: config.Governance{
			Profile: string(domain.PolicyProfileDefault),
			Profiles: map[string]config.GovernanceProfile{
				string(domain.PolicyProfileDefault): {Default: "allow", Rules: []config.GovernanceRule{{Tool: tools.WriteNoteName, Decision: "prompt"}}},
			},
		},
	}
}

// scriptedAnthropicServer answers the first model call with a tool_use
// block and a plain text answer afterwards. The engine streams, so
// streaming requests get an Anthropic SSE transcript; a non-streaming
// request (protocol-level tests) still gets plain JSON.
func scriptedAnthropicServer(t *testing.T, toolText, finalText string) *httptest.Server {
	t.Helper()
	var calls atomic.Int64
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		streaming := strings.Contains(string(raw), `"stream":true`)
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		writeEvent := func(event string, payload string) {
			_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, payload)
			if flusher != nil {
				flusher.Flush()
			}
		}
		messageStart := `{"type":"message_start","message":{"id":"msg_s","type":"message","role":"assistant","model":"claude-sonnet-4-5","content":[],"stop_reason":null,"usage":{"input_tokens":1,"output_tokens":1}}}`
		if calls.Add(1) == 1 {
			if !streaming {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"id":"msg_tool","type":"message","role":"assistant","model":"claude-sonnet-4-5","content":[{"type":"tool_use","id":"toolu_1","name":"` + tools.WriteNoteName + `","input":{"content":"` + toolText + `"}}],"stop_reason":"tool_use","stop_sequence":null,"usage":{"input_tokens":1,"output_tokens":1}}`))
				return
			}
			writeEvent("message_start", messageStart)
			writeEvent("content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"toolu_1","name":"`+tools.WriteNoteName+`","input":{}}}`)
			writeEvent("content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"content\":\"`+toolText+`\"}"}}`)
			writeEvent("content_block_stop", `{"type":"content_block_stop","index":0}`)
			writeEvent("message_delta", `{"type":"message_delta","delta":{"stop_reason":"tool_use","stop_sequence":null},"usage":{"output_tokens":1}}`)
			writeEvent("message_stop", `{"type":"message_stop"}`)
			return
		}
		if !streaming {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"msg_final","type":"message","role":"assistant","model":"claude-sonnet-4-5","content":[{"type":"text","text":"` + finalText + `"}],"stop_reason":"end_turn","stop_sequence":null,"usage":{"input_tokens":1,"output_tokens":1}}`))
			return
		}
		writeEvent("message_start", messageStart)
		writeEvent("content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`)
		writeEvent("content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"`+finalText+`"}}`)
		writeEvent("content_block_stop", `{"type":"content_block_stop","index":0}`)
		writeEvent("message_delta", `{"type":"message_delta","delta":{"stop_reason":"end_turn","stop_sequence":null},"usage":{"output_tokens":1}}`)
		writeEvent("message_stop", `{"type":"message_stop"}`)
	}))
}

func composeGatewayless(t *testing.T) (*App, *controlrpc.Peer) {
	t.Helper()
	runtime.SetEngineVersionOverride(pinnedEinoVersion)
	t.Cleanup(func() { runtime.SetEngineVersionOverride("") })
	t.Setenv("ANTHROPIC_API_KEY", "facehost-test-key")
	t.Setenv("VIVY_PROVIDER", "anthropic")
	srv := scriptedAnthropicServer(t, "loopback note", "loopback done")
	t.Setenv("VIVY_API_BASE", srv.URL)

	a, err := New(context.Background(), newAnthropicTestConfig(t), WithoutEars(), WithoutGateway())
	if err != nil {
		t.Fatalf("compose gateway-less: %v", err)
	}
	t.Cleanup(func() { _ = a.backend.Close() })
	if a.httpServer != nil {
		t.Fatalf("gateway-less composition built an HTTP server")
	}
	dialCtx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	client, err := a.DialControl(dialCtx, nil)
	if err != nil {
		t.Fatalf("dial control: %v", err)
	}
	return a, client
}

func callControl(t *testing.T, client *controlrpc.Peer, method string, params any) map[string]any {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result, err := client.Call(ctx, method, params)
	if err != nil {
		t.Fatalf("%s: %v", method, err)
	}
	var out map[string]any
	if err := json.Unmarshal(result, &out); err != nil {
		t.Fatalf("%s: decode result %s: %v", method, result, err)
	}
	return out
}

// waitFor polls until fn passes or the timeout elapses, reporting the last
// observed value through a map the caller reads for failure context.
func waitFor(t *testing.T, timeout time.Duration, fn func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("condition not met within %s", timeout)
}

// runEvents returns the journal entries of one run as decoded maps, for
// diagnostics inside poll failures.
func runEvents(t *testing.T, client *controlrpc.Peer, runID string) []map[string]any {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	raw, err := client.Call(ctx, "run/log", map[string]any{"run_id": runID})
	if err != nil {
		t.Logf("run/log: %v", err)
		return nil
	}
	var out struct {
		Events []map[string]any `json:"events"`
	}
	_ = json.Unmarshal(raw, &out)
	return out.Events
}

// TestLoopbackControlCompletesApprovedConversation pins the F1 success
// criterion: with no embed and no listener, an in-process JSON-RPC client
// drives one conversation through the exact same control-plane methods the
// web face uses, including the human approval round.
func TestLoopbackControlCompletesApprovedConversation(t *testing.T) {
	_, client := composeGatewayless(t)

	init := callControl(t, client, "initialize", nil)
	if init["protocol_version"] == "" {
		t.Fatalf("initialize result = %v", init)
	}
	session := callControl(t, client, "session/create", map[string]any{"title": "facehost"})
	sessionID, _ := session["id"].(string)
	if sessionID == "" {
		t.Fatalf("session/create result = %v", session)
	}
	turn := callControl(t, client, "turn/start", map[string]any{"session_id": sessionID, "text": "echo something"})
	runID, _ := turn["run_id"].(string)
	if runID == "" {
		t.Fatalf("turn/start result = %v", turn)
	}

	var approvalID string
	approvalDeadline := time.Now().Add(10 * time.Second)
	for {
		list := callControl(t, client, "approval/list", nil)
		approvals, _ := list["approvals"].([]any)
		if len(approvals) > 0 {
			first, _ := approvals[0].(map[string]any)
			approvalID, _ = first["id"].(string)
		}
		if approvalID != "" {
			break
		}
		if time.Now().After(approvalDeadline) {
			for _, ev := range runEvents(t, client, runID) {
				t.Logf("run event: type=%v payload=%v", ev["type"], ev["payload"])
			}
			t.Fatalf("no approval surfaced; approval/list = %v", list)
		}
		time.Sleep(25 * time.Millisecond)
	}
	callControl(t, client, "approval/respond", map[string]any{"approval_id": approvalID, "decision": "approved"})

	var status string
	waitFor(t, 30*time.Second, func() bool {
		run := callControl(t, client, "run/get", map[string]any{"run_id": runID})
		status, _ = run["status"].(string)
		return status == "completed"
	})

	log := callControl(t, client, "run/log", map[string]any{"run_id": runID})
	entries, _ := log["events"].([]any)
	noteFinished := false
	for _, entry := range entries {
		event, _ := entry.(map[string]any)
		if event["type"] != "tool.finished" {
			continue
		}
		payload, _ := event["payload"].(map[string]any)
		// tool.finished omits the error key when omitempty clears it.
		errValue, hasError := payload["error"]
		if payload["tool_name"] == tools.WriteNoteName && (!hasError || errValue == nil || errValue == "") {
			noteFinished = true
		}
	}
	if !noteFinished {
		for _, ev := range runEvents(t, client, runID) {
			if ev["type"] == "tool.finished" {
				t.Logf("tool.finished payload = %v", ev["payload"])
			}
		}
		t.Fatalf("write_note did not finish successfully over the loopback control plane")
	}
	messages := callControl(t, client, "session/messages", map[string]any{"session_id": sessionID})
	items, _ := messages["messages"].([]any)
	finalText := false
	for _, item := range items {
		message, _ := item.(map[string]any)
		if message["role"] == "assistant" && message["content"] == "loopback done" {
			finalText = true
		}
	}
	if !finalText {
		t.Fatalf("final assistant text missing after the approved round: %v", messages)
	}
}

// TestRunFaceWithoutOrganFails pins the launcher contract: composing run
// through a face with no organ compiled in fails loudly instead of
// silently falling back.
func TestRunFaceWithoutOrganFails(t *testing.T) {
	runtime.SetEngineVersionOverride(pinnedEinoVersion)
	t.Cleanup(func() { runtime.SetEngineVersionOverride("") })
	t.Setenv("ANTHROPIC_API_KEY", "facehost-test-key")
	t.Setenv("VIVY_PROVIDER", "anthropic")
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	_, err := RunFace(ctx, newAnthropicTestConfig(t), nil, plugin.FaceOptions{
		Prompt: "x", Out: io.Discard, Err: io.Discard,
	})
	if err == nil || !strings.Contains(err.Error(), "no face organ") {
		t.Fatalf("RunFace(nil ctor) = %v, want the no-organ error", err)
	}
}

// stubFace is a minimal seam-face organ: it proves the launcher hands the
// invocation payload through and the FaceEnv reaches the real control
// plane, without owning the conversation flow (that is faces/headless's).
type stubFace struct {
	opts plugin.FaceOptions
	kind string
}

func (f *stubFace) Kind() string { return f.kind }

func (f *stubFace) Run(ctx context.Context, env plugin.FaceEnv) (plugin.FaceResult, error) {
	if f.opts.Out == nil || f.opts.Err == nil {
		return plugin.FaceResult{}, errors.New("stub: launcher dropped the writers")
	}
	env.OnEvent(func(string, json.RawMessage) {})
	if _, err := env.Call(ctx, "initialize", nil); err != nil {
		return plugin.FaceResult{}, err
	}
	session, err := env.Call(ctx, "session/create", map[string]any{"title": "runface-stub"})
	if err != nil {
		return plugin.FaceResult{}, err
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(session, &created); err != nil || created.ID == "" {
		return plugin.FaceResult{}, fmt.Errorf("stub: session/create returned %s", session)
	}
	return plugin.FaceResult{Status: "completed"}, nil
}

// TestRunFaceServesGatewaylessControlPlane pins the F2 launcher half: a
// face organ drives the real control plane through FaceEnv with no
// listener, and the kind/kind-match plumbing stays out of its way.
func TestRunFaceServesGatewaylessControlPlane(t *testing.T) {
	runtime.SetEngineVersionOverride(pinnedEinoVersion)
	t.Cleanup(func() { runtime.SetEngineVersionOverride("") })
	t.Setenv("ANTHROPIC_API_KEY", "facehost-test-key")
	t.Setenv("VIVY_PROVIDER", "anthropic")
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	var captured plugin.FaceEnv
	ctor := func(opts plugin.FaceOptions) plugin.Face {
		return &capturingFace{stubFace: stubFace{opts: opts, kind: "stub"}, captured: &captured}
	}
	result, err := RunFace(ctx, newAnthropicTestConfig(t), ctor, plugin.FaceOptions{
		Prompt: "hello", Out: io.Discard, Err: io.Discard,
	})
	if err != nil {
		t.Fatalf("RunFace: %v", err)
	}
	if result.Status != "completed" {
		t.Fatalf("status = %q, want completed", result.Status)
	}
	if _, err := captured.Call(context.Background(), "initialize", nil); err == nil {
		t.Fatal("face control peer remained usable after RunFace returned")
	}
}

type capturingFace struct {
	stubFace
	captured *plugin.FaceEnv
}

func (f *capturingFace) Run(ctx context.Context, env plugin.FaceEnv) (plugin.FaceResult, error) {
	*f.captured = env
	return f.stubFace.Run(ctx, env)
}

// TestGatewaylessRunWithoutFaceCancelsDurably pins the no-UI approval fail
// path at the composition level: a run suspended on an approval with no
// face attached never resolves on its own, and a cancel through the same
// in-process control plane closes it durably; the gateway-less App.Run
// returns cleanly once its context is cancelled.
func TestGatewaylessRunWithoutFaceCancelsDurably(t *testing.T) {
	a, client := composeGatewayless(t)

	session := callControl(t, client, "session/create", map[string]any{"title": "facehost-cancel"})
	sessionID, _ := session["id"].(string)
	turn := callControl(t, client, "turn/start", map[string]any{"session_id": sessionID, "text": "needs approval"})
	runID, _ := turn["run_id"].(string)

	waitFor(t, 10*time.Second, func() bool {
		list := callControl(t, client, "approval/list", nil)
		approvals, _ := list["approvals"].([]any)
		return len(approvals) > 0
	})

	runCtx, cancelRun := context.WithCancel(context.Background())
	runDone := make(chan error, 1)
	go func() { runDone <- a.Run(runCtx) }()

	callControl(t, client, "run/cancel", map[string]any{"run_id": runID})
	var status string
	waitFor(t, 10*time.Second, func() bool {
		run := callControl(t, client, "run/get", map[string]any{"run_id": runID})
		status, _ = run["status"].(string)
		return status == "cancelled"
	})

	cancelRun()
	select {
	case err := <-runDone:
		if err != nil {
			t.Fatalf("gateway-less Run returned %v, want nil", err)
		}
	case <-time.After(shutdownGrace + 10*time.Second):
		t.Fatalf("gateway-less Run did not return after cancellation")
	}
}

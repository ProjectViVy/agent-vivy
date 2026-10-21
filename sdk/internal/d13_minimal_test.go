package sdk

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/tui"
)

// A packed minimal Generation must perform a real protected-tool causal turn,
// without constructing optional Modules or depending on the authoring Assembly.
func TestMinimalCausalExchange(t *testing.T) {
	for _, scenario := range []struct{ name, recipe, tool string }{
		{"minimal", "recipes/minimal.vivy.yml", "read_file"},
		{"minimal-plus-hello-fs", "examples/minimal-plus-hello-fs/recipe.vivy.yml", "hello_stat"},
	} {
		t.Run(scenario.name, func(t *testing.T) { testMinimalCausalExchange(t, scenario.recipe, scenario.tool) })
	}
}

func testMinimalCausalExchange(t *testing.T, recipe, toolName string) {
	t.Helper()
	root := t.TempDir()
	artifact, err := Pack(context.Background(), packOptions{Recipe: filepath.Join("..", "..", filepath.FromSlash(recipe)), Output: filepath.Join(root, "minimal")})
	if err != nil {
		t.Fatal(err)
	}
	inspected, err := InspectArtifact(artifact.Directory)
	if err != nil {
		t.Fatal(err)
	}
	selected := map[string]bool{}
	for _, m := range inspected.Manifest.Modules {
		selected[m.ID] = true
	}
	for _, id := range []string{"vivy/protected-tools", "vivy/provider-profiles"} {
		if !selected[id] {
			t.Fatalf("minimal cannot execute its real causal turn without %s", id)
		}
	}
	workspace := filepath.Join(root, "workspaces", "project")
	if err := os.MkdirAll(workspace, 0700); err != nil {
		t.Fatal(err)
	}
	nonce := rand.Text()
	if err := os.WriteFile(filepath.Join(workspace, "nonce.txt"), []byte(nonce), 0600); err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var requests [][]byte
	const finalText = "Minimal read complete."
	model := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		var request struct {
			Stream bool `json:"stream"`
		}
		if err := json.Unmarshal(body, &request); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		mu.Lock()
		requests = append(requests, body)
		seq := len(requests)
		mu.Unlock()
		if seq > 2 {
			http.Error(w, "unexpected model request", 500)
			return
		}
		message := map[string]any{"role": "assistant", "content": finalText}
		finish := "stop"
		if seq == 1 {
			call := map[string]any{"id": "minimal-read", "type": "function", "function": map[string]any{"name": toolName, "arguments": `{"path":"nonce.txt"}`}}
			if request.Stream {
				call["index"] = 0
			}
			message = map[string]any{"role": "assistant", "tool_calls": []any{call}}
			finish = "tool_calls"
		}
		choice := map[string]any{"index": 0, "finish_reason": finish}
		if request.Stream {
			choice["delta"] = message
		} else {
			choice["message"] = message
		}
		response := map[string]any{"id": fmt.Sprint("minimal-", seq), "object": "chat.completion", "created": 1, "model": "deepseek-flash", "choices": []any{choice}, "usage": map[string]any{"prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2}}
		raw, _ := json.Marshal(response)
		if request.Stream {
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", raw)
		} else {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(raw)
		}
	}))
	defer model.Close()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.Addr().String()
	_ = listener.Close()
	configFile := filepath.Join(root, "config.json")
	enabledTools := []string{"read_file", "list_dir"}
	if toolName != "read_file" {
		enabledTools = append(enabledTools, toolName)
	}
	cfg := map[string]any{"server": map[string]any{"addr": addr}, "storage": map[string]any{"backend": "sqlite", "sqlite": map[string]any{"path": filepath.Join(root, "minimal.db")}}, "runtime": map[string]any{"workspace_root": filepath.Dir(workspace), "cache_warming": "off"}, "tools": map[string]any{"enabled": enabledTools}}
	configRaw, _ := json.Marshal(cfg)
	if err := os.WriteFile(configFile, configRaw, 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	logs, err := os.Create(filepath.Join(root, "backend.log"))
	if err != nil {
		t.Fatal(err)
	}
	command := exec.CommandContext(ctx, artifact.Binary)
	command.Dir = root
	command.Stdout = logs
	command.Stderr = logs
	command.Env = append(os.Environ(), "VIVY_USER_HOME="+filepath.Join(root, "home"), "VIVY_CONFIG="+configFile, "VIVY_ADDR="+addr, "VIVY_PROVIDER=deepseek", "VIVY_API_BASE="+model.URL, "VIVY_MODEL=deepseek-flash", "DEEPSEEK_API_KEY=minimal-local-dummy")
	if err := command.Start(); err != nil {
		cancel()
		_ = logs.Close()
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("minimal backend did not stop")
		}
		_ = logs.Close()
	})
	ready := false
	httpClient := &http.Client{Timeout: time.Second}
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		response, err := httpClient.Get("http://" + addr + "/healthz")
		if err == nil {
			_ = response.Body.Close()
			if response.StatusCode == 200 {
				ready = true
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !ready {
		raw, _ := os.ReadFile(logs.Name())
		t.Fatalf("minimal did not become ready: %s", raw)
	}
	client, err := tui.Dial(ctx, addr, "")
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	call := func(method string, params any) json.RawMessage {
		t.Helper()
		raw, err := client.Call(ctx, method, params)
		if err != nil {
			t.Fatalf("%s: %v", method, err)
		}
		return raw
	}
	// Exercise the documented bootstrap command against the packed gateway.
	binding := exec.CommandContext(ctx, "go", "run", "../../examples/minimal/bind-workspace", "--addr", addr, "--workspace", workspace)
	bound, err := binding.CombinedOutput()
	if err != nil {
		t.Fatalf("documented workspace binding: %v: %s", err, bound)
	}
	var session struct {
		ID        string `json:"id"`
		Workspace string `json:"workspace_path"`
	}
	if err := json.Unmarshal(bound, &session); err != nil || session.ID == "" || session.Workspace != workspace {
		t.Fatalf("documented bootstrap lost workspace: %s (%v)", bound, err)
	}
	var turn struct {
		RunID string `json:"run_id"`
	}
	if err := json.Unmarshal(call("turn/start", map[string]any{"session_id": session.ID, "face": "code", "text": "Use " + toolName + " on nonce.txt and finish."}), &turn); err != nil {
		t.Fatal(err)
	}
	completed := false
	for ctx.Err() == nil {
		var run domain.Run
		if err := json.Unmarshal(call("run/get", map[string]any{"run_id": turn.RunID}), &run); err != nil {
			t.Fatal(err)
		}
		if run.Status.Terminal() {
			if run.Status != domain.RunCompleted {
				raw, _ := os.ReadFile(logs.Name())
				t.Fatalf("minimal failed: %s", raw)
			}
			completed = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !completed {
		t.Fatal("minimal did not complete")
	}
	var log struct {
		Events []struct {
			Type    domain.EventType `json:"type"`
			Payload json.RawMessage  `json:"payload"`
		} `json:"events"`
	}
	if err := json.Unmarshal(call("run/log", map[string]any{"run_id": turn.RunID}), &log); err != nil {
		t.Fatal(err)
	}
	terminals := 0
	result := ""
	for _, event := range log.Events {
		if event.Type.Terminal() {
			terminals++
		}
		if event.Type == domain.EventToolFinished {
			var p struct {
				ID     string `json:"tool_call_id"`
				Result string `json:"result"`
				Error  string `json:"error"`
			}
			if err := json.Unmarshal(event.Payload, &p); err != nil {
				t.Fatal(err)
			}
			if p.ID == "minimal-read" && p.Error == "" {
				result = p.Result
			}
		}
	}
	expected := nonce
	if toolName == "hello_stat" {
		expected = fmt.Sprintf("nonce.txt %d", len(nonce))
	}
	if terminals != 1 || !strings.Contains(result, expected) {
		t.Fatalf("lost durable causal result: terminals=%d, result=%q", terminals, result)
	}
	mu.Lock()
	captured := append([][]byte(nil), requests...)
	mu.Unlock()
	if len(captured) != 2 || bytes.Contains(captured[0], []byte(nonce)) {
		t.Fatalf("model request count/provenance: %d", len(captured))
	}
	var continuation struct {
		Messages []struct {
			Role    string          `json:"role"`
			CallID  string          `json:"tool_call_id"`
			Content json.RawMessage `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(captured[1], &continuation); err != nil {
		t.Fatal(err)
	}
	matches := 0
	for _, message := range continuation.Messages {
		if message.Role == "tool" && message.CallID == "minimal-read" {
			var content string
			if err := json.Unmarshal(message.Content, &content); err != nil || content != result {
				t.Fatalf("request2 lost exact result: %s (%v)", message.Content, err)
			}
			matches++
		}
	}
	if matches != 1 {
		t.Fatalf("request2 exact result/call ID matches: %d", matches)
	}
	var transcript struct {
		Messages []domain.Message `json:"messages"`
	}
	if err := json.Unmarshal(call("session/messages", map[string]any{"session_id": session.ID}), &transcript); err != nil {
		t.Fatal(err)
	}
	if len(transcript.Messages) == 0 || transcript.Messages[len(transcript.Messages)-1].Content != finalText {
		t.Fatal("minimal final answer was not persisted")
	}
	var active struct {
		Runs []domain.Run `json:"runs"`
	}
	if err := json.Unmarshal(call("background/list", nil), &active); err != nil {
		t.Fatal(err)
	}
	if len(active.Runs) != 0 {
		t.Fatalf("minimal did not return idle: %+v", active.Runs)
	}
}

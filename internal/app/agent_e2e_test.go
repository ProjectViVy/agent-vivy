package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"agent-vivy/internal/runtime"
	"agent-vivy/internal/tools"
)

// agentScriptServer serves the parent's two turns and fails every child model
// request. Parent and child share one provider; the admitted instruction is
// the child's only system text, so a prefix match tells them apart.
type agentScriptServer struct {
	server *httptest.Server
	mu     sync.Mutex
	parent int
	child  int
}

func newAgentScriptServer(t *testing.T) *agentScriptServer {
	t.Helper()
	s := &agentScriptServer{}
	s.server = httptest.NewServer(http.HandlerFunc(s.handle))
	t.Cleanup(s.server.Close)
	return s
}

func (s *agentScriptServer) counts() (parent, child int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.parent, s.child
}

func (s *agentScriptServer) handle(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	var request struct {
		Stream   bool `json:"stream"`
		Messages []struct {
			Role    string          `json:"role"`
			Content json.RawMessage `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(body, &request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	system := ""
	for _, message := range request.Messages {
		if message.Role != "system" {
			continue
		}
		var text string
		if json.Unmarshal(message.Content, &text) == nil {
			system += text
		}
	}
	if strings.HasPrefix(strings.TrimSpace(system), "Execute the assigned task") {
		s.mu.Lock()
		s.child++
		s.mu.Unlock()
		http.Error(w, "child provider exploded", http.StatusInternalServerError)
		return
	}
	s.mu.Lock()
	s.parent++
	call := s.parent
	s.mu.Unlock()
	if !request.Stream {
		http.Error(w, "expected streamed model request", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	write := func(delta any, finish any) {
		chunk := map[string]any{"id": "chatcmpl-agent-e2e", "object": "chat.completion.chunk", "created": 1, "model": "deepseek-flash", "choices": []any{map[string]any{"index": 0, "delta": delta, "finish_reason": finish}}}
		encoded, _ := json.Marshal(chunk)
		_, _ = fmt.Fprintf(w, "data: %s\n\n", encoded)
	}
	if call == 1 {
		write(map[string]any{"role": "assistant", "tool_calls": []any{map[string]any{"index": 0, "id": "agent-call-1", "type": "function", "function": map[string]any{"name": tools.AgentName, "arguments": ""}}}}, nil)
		write(map[string]any{"tool_calls": []any{map[string]any{"index": 0, "function": map[string]any{"arguments": `{"task":"Reply with exactly one word: OK."}`}}}}, nil)
		write(map[string]any{}, "tool_calls")
	} else {
		write(map[string]any{"role": "assistant", "content": "Parent recovered after the child failed."}, nil)
		write(map[string]any{}, "stop")
	}
	_, _ = io.WriteString(w, "data: [DONE]\n\n")
}

// A delegated child that fails is a tool outcome for the parent, never a
// parent terminal: the parent run completes with the child's diagnostic in the
// tool result instead of failing the conversation.
func TestAgentChildFailureKeepsParentRunAlive(t *testing.T) {
	runtime.SetEngineVersionOverride(pinnedEinoVersion)
	t.Cleanup(func() { runtime.SetEngineVersionOverride("") })
	t.Setenv("DEEPSEEK_API_KEY", "agent-child-e2e-key")
	t.Setenv("VIVY_PROVIDER", "deepseek")
	model := newAgentScriptServer(t)
	t.Setenv("VIVY_API_BASE", model.server.URL)

	cfg, workspace := planGoalTestConfig(t, []string{tools.AgentName})
	a := openPlanGoalTestApp(t, cfg)
	client, err := a.DialControl(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })

	session := callControl(t, client, "session/create", map[string]any{"title": "agent child failure", "workspace_path": workspace})
	sessionID, _ := session["id"].(string)
	turn := callControl(t, client, "turn/start", map[string]any{"session_id": sessionID, "text": "Delegate a one-word check."})
	runID, _ := turn["run_id"].(string)

	var status string
	waitFor(t, 60*time.Second, func() bool {
		run := callControl(t, client, "run/get", map[string]any{"run_id": runID})
		status, _ = run["status"].(string)
		return status == "completed" || status == "failed"
	})
	if status != "completed" {
		t.Fatalf("parent run status = %q, want completed", status)
	}

	var agentResult, agentOutcome, agentReason, agentEffects string
	for _, event := range runEvents(t, client, runID) {
		switch event["type"] {
		case "run.failed":
			t.Fatalf("parent run failed instead of recovering: %v", event["payload"])
		case "tool.finished":
			payload, _ := event["payload"].(map[string]any)
			if payload["tool_name"] == tools.AgentName {
				agentResult, _ = payload["result"].(string)
				agentOutcome, _ = payload["outcome"].(string)
				agentReason, _ = payload["reason"].(string)
				agentEffects, _ = payload["effects"].(string)
			}
		}
	}
	if !strings.Contains(agentResult, "sub-agent failed") {
		t.Fatalf("agent tool result = %q, want the bounded child diagnostic", agentResult)
	}
	if agentOutcome != "recoverable" || agentReason != "child_failed" || agentEffects != "unknown" {
		t.Fatalf("agent outcome = %q/%q/%q, want recoverable/child_failed/unknown", agentOutcome, agentReason, agentEffects)
	}
	if parent, child := model.counts(); parent < 2 || child == 0 {
		t.Fatalf("provider requests parent=%d child=%d, want a completed second parent turn and at least one child request", parent, child)
	}
}

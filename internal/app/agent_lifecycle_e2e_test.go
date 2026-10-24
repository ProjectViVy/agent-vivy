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

	controlrpc "agent-vivy/internal/rpc"
	"agent-vivy/internal/tools"
)

// delegationServer scripts one parent conversation and answers every child
// request from the delegated task text. Parent and child share one provider;
// the admitted child instruction is the child's only system text, so a prefix
// match tells them apart.
type delegationServer struct {
	server *httptest.Server
	// steps are consumed one per parent model request: text, or one agent
	// call per delegated task.
	steps []delegationStep

	mu         sync.Mutex
	parent     int
	childTasks []string
	slowStart  chan struct{}
	released   bool
}

type delegationStep struct {
	text   string
	agents []string
}

func newDelegationServer(t *testing.T, steps ...delegationStep) *delegationServer {
	t.Helper()
	s := &delegationServer{
		steps:     steps,
		slowStart: make(chan struct{}),
	}
	s.server = httptest.NewServer(http.HandlerFunc(s.handle))
	t.Cleanup(s.server.Close)
	return s
}

func (s *delegationServer) counts() (parent int, children []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.parent, append([]string(nil), s.childTasks...)
}

func (s *delegationServer) handle(w http.ResponseWriter, r *http.Request) {
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
	system, lastUser := requestText(request.Messages)
	if strings.HasPrefix(strings.TrimSpace(system), "Execute the assigned task") {
		s.answerChild(w, r, lastUser)
		return
	}
	if !request.Stream {
		http.Error(w, "expected streamed model request", http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	s.parent++
	call := s.parent
	s.mu.Unlock()
	if call > len(s.steps) {
		http.Error(w, "script exhausted", http.StatusInternalServerError)
		return
	}
	step := s.steps[call-1]
	if len(step.agents) == 0 {
		s.writeText(w, step.text)
		return
	}
	s.writeAgentCalls(w, step.agents)
}

// requestText joins every system message and returns the last non-system,
// non-tool message text: the parent's instruction or the child's task.
func requestText(messages []struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}) (system, lastUser string) {
	for _, message := range messages {
		text := contentText(message.Content)
		switch message.Role {
		case "system":
			system += text
		case "user":
			lastUser = text
		}
	}
	return system, lastUser
}

func contentText(raw json.RawMessage) string {
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return text
	}
	var parts []struct {
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &parts) == nil {
		var joined strings.Builder
		for _, part := range parts {
			joined.WriteString(part.Text)
		}
		return joined.String()
	}
	return ""
}

func (s *delegationServer) answerChild(w http.ResponseWriter, r *http.Request, task string) {
	s.mu.Lock()
	s.childTasks = append(s.childTasks, task)
	s.mu.Unlock()
	switch {
	case strings.Contains(task, "slow-task"):
		close(s.slowStart)
		// The parent cancel must reach this in-flight provider call.
		<-r.Context().Done()
		s.mu.Lock()
		s.released = true
		s.mu.Unlock()
	case strings.Contains(task, "empty-task"):
		s.writeText(w, "")
	case strings.Contains(task, "loop-task"):
		s.writeToolCall(w, "loop-read-1", tools.ReadFileName, `{"path":"missing.txt"}`)
	case strings.Contains(task, "big-task"):
		s.writeText(w, "BIG:"+strings.Repeat("x", 40<<10))
	default:
		s.writeText(w, "ANSWER["+task+"]")
	}
}

func (s *delegationServer) writeText(w http.ResponseWriter, text string) {
	w.Header().Set("Content-Type", "text/event-stream")
	chunk := map[string]any{"id": "chatcmpl-delegation", "object": "chat.completion.chunk", "created": 1, "model": "deepseek-flash", "choices": []any{map[string]any{"index": 0, "delta": map[string]any{"role": "assistant", "content": text}, "finish_reason": nil}}}
	encoded, _ := json.Marshal(chunk)
	_, _ = fmt.Fprintf(w, "data: %s\n\n", encoded)
	final := map[string]any{"id": "chatcmpl-delegation", "object": "chat.completion.chunk", "created": 1, "model": "deepseek-flash", "choices": []any{map[string]any{"index": 0, "delta": map[string]any{}, "finish_reason": "stop"}}}
	encoded, _ = json.Marshal(final)
	_, _ = fmt.Fprintf(w, "data: %s\n\n", encoded)
	_, _ = io.WriteString(w, "data: [DONE]\n\n")
}

func (s *delegationServer) writeToolCall(w http.ResponseWriter, id, name, arguments string) {
	s.writeToolCalls(w, []delegationCall{{id: id, name: name, arguments: arguments}})
}

type delegationCall struct{ id, name, arguments string }

func (s *delegationServer) writeAgentCalls(w http.ResponseWriter, tasks []string) {
	calls := make([]delegationCall, 0, len(tasks))
	for _, task := range tasks {
		calls = append(calls, delegationCall{id: "call-" + task, name: tools.AgentName, arguments: fmt.Sprintf(`{"task":%q}`, task)})
	}
	s.writeToolCalls(w, calls)
}

func (s *delegationServer) writeToolCalls(w http.ResponseWriter, calls []delegationCall) {
	w.Header().Set("Content-Type", "text/event-stream")
	write := func(delta any, finish any) {
		chunk := map[string]any{"id": "chatcmpl-delegation", "object": "chat.completion.chunk", "created": 1, "model": "deepseek-flash", "choices": []any{map[string]any{"index": 0, "delta": delta, "finish_reason": finish}}}
		encoded, _ := json.Marshal(chunk)
		_, _ = fmt.Fprintf(w, "data: %s\n\n", encoded)
	}
	for index, call := range calls {
		write(map[string]any{"role": "assistant", "tool_calls": []any{map[string]any{"index": index, "id": call.id, "type": "function", "function": map[string]any{"name": call.name, "arguments": ""}}}}, nil)
		write(map[string]any{"tool_calls": []any{map[string]any{"index": index, "function": map[string]any{"arguments": call.arguments}}}}, nil)
	}
	write(map[string]any{}, "tool_calls")
	_, _ = io.WriteString(w, "data: [DONE]\n\n")
}

// runDelegation starts one turn against the scripted provider and returns the
// app, control client, run id, and the agent tool results keyed by call id.
type delegationRun struct {
	client   *controlrpc.Peer
	runID    string
	results  map[string]map[string]any
	status   string
	terminal map[string]any
}

func startDelegationTurn(t *testing.T, server *delegationServer, enabled []string, maxTurns int) *delegationRun {
	t.Helper()
	planGoalTestProvider(t, server.server.URL)
	cfg, workspace := planGoalTestConfig(t, enabled)
	if maxTurns > 0 {
		cfg.Runtime.MaxToolTurns = maxTurns
	}
	a := openPlanGoalTestApp(t, cfg)
	client, err := a.DialControl(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	session := callControl(t, client, "session/create", map[string]any{"title": "delegation lifecycle", "workspace_path": workspace})
	sessionID, _ := session["id"].(string)
	turn := callControl(t, client, "turn/start", map[string]any{"session_id": sessionID, "text": "Delegate the work."})
	runID, _ := turn["run_id"].(string)
	return &delegationRun{client: client, runID: runID, results: map[string]map[string]any{}}
}

func (r *delegationRun) waitTerminal(t *testing.T) {
	t.Helper()
	waitFor(t, 60*time.Second, func() bool {
		run := callControl(t, r.client, "run/get", map[string]any{"run_id": r.runID})
		r.status, _ = run["status"].(string)
		return r.status == "completed" || r.status == "failed" || r.status == "cancelled"
	})
	for _, event := range runEvents(t, r.client, r.runID) {
		if event["type"] == "run.failed" {
			r.terminal = event["payload"].(map[string]any)
		}
		if event["type"] == "tool.finished" {
			payload, _ := event["payload"].(map[string]any)
			if payload["tool_name"] == tools.AgentName {
				callID, _ := payload["tool_call_id"].(string)
				r.results[callID] = payload
			}
		}
	}
}

// One assistant message may carry several delegations; each answer returns
// to its own tool call. Turn-scoped tools execute in order, so the second
// child starts after the first one finishes.
func TestAgentDelegationsInOneTurnReturnEveryAnswer(t *testing.T) {
	server := newDelegationServer(t,
		delegationStep{agents: []string{"alpha", "beta"}},
		delegationStep{text: "Both delegations returned."},
	)
	run := startDelegationTurn(t, server, []string{tools.AgentName}, 0)
	run.waitTerminal(t)
	if run.status != "completed" {
		t.Fatalf("parent run status = %q terminal=%v", run.status, run.terminal)
	}
	for _, task := range []string{"alpha", "beta"} {
		result, _ := run.results["call-"+task]["result"].(string)
		if !strings.Contains(result, "ANSWER["+task+"]") {
			t.Fatalf("agent call %q result = %q, want its own answer", task, result)
		}
	}
	if parent, children := server.counts(); parent < 2 || len(children) != 2 {
		t.Fatalf("provider requests parent=%d children=%v, want one child per delegation", parent, children)
	}
}

// A parent cancel must stop the delegated child, including its in-flight
// provider call: no orphan child may keep working after the conversation ends.
func TestAgentParentCancelStopsRunningChild(t *testing.T) {
	server := newDelegationServer(t,
		delegationStep{agents: []string{"slow-task"}},
		delegationStep{text: "unreachable"},
	)
	run := startDelegationTurn(t, server, []string{tools.AgentName}, 0)
	select {
	case <-server.slowStart:
	case <-time.After(60 * time.Second):
		t.Fatal("child request never reached the provider")
	}
	callControl(t, run.client, "run/cancel", map[string]any{"run_id": run.runID})
	run.waitTerminal(t)
	if run.status != "cancelled" {
		t.Fatalf("parent run status = %q, want cancelled", run.status)
	}
	var childStatus string
	waitFor(t, 30*time.Second, func() bool {
		children := callControl(t, run.client, "child/list", map[string]any{"parent_run_id": run.runID})
		list, _ := children["children"].([]any)
		if len(list) == 0 {
			return false
		}
		first, _ := list[0].(map[string]any)
		childStatus, _ = first["status"].(string)
		return childStatus == "cancelled" || childStatus == "failed" || childStatus == "completed"
	})
	if childStatus != "cancelled" {
		t.Fatalf("child status after parent cancel = %q, want cancelled", childStatus)
	}
	waitFor(t, 10*time.Second, func() bool {
		server.mu.Lock()
		defer server.mu.Unlock()
		return server.released
	})
}

// A child that exhausts its tool-call turns is a tool result for the parent:
// the parent still finishes, and the diagnostic names the child's limit.
func TestAgentChildTurnLimitBecomesToolResult(t *testing.T) {
	server := newDelegationServer(t,
		delegationStep{agents: []string{"loop-task"}},
		delegationStep{text: "The delegated loop is closed."},
	)
	run := startDelegationTurn(t, server, []string{tools.AgentName, tools.ReadFileName}, 3)
	run.waitTerminal(t)
	if run.status != "completed" {
		t.Fatalf("parent run status = %q terminal=%v", run.status, run.terminal)
	}
	result, _ := run.results["call-loop-task"]["result"].(string)
	if !strings.Contains(result, "reached its tool-call limit") {
		t.Fatalf("agent result = %q, want the child turn-limit diagnostic", result)
	}
	if outcome, _ := run.results["call-loop-task"]["outcome"].(string); outcome != "recoverable" {
		t.Fatalf("agent outcome = %q, want recoverable", outcome)
	}
}

// The child answer enters the parent context bounded: an oversized answer is
// truncated with an honest marker instead of flooding the conversation.
func TestAgentOversizedChildAnswerIsBounded(t *testing.T) {
	server := newDelegationServer(t,
		delegationStep{agents: []string{"big-task"}},
		delegationStep{text: "Handled."},
	)
	run := startDelegationTurn(t, server, []string{tools.AgentName}, 0)
	run.waitTerminal(t)
	if run.status != "completed" {
		t.Fatalf("parent run status = %q terminal=%v", run.status, run.terminal)
	}
	result, _ := run.results["call-big-task"]["result"].(string)
	if !strings.Contains(result, "[child result truncated]") {
		t.Fatalf("agent result lacks the truncation marker: %d bytes", len(result))
	}
	if len(result) > 40<<10 {
		t.Fatalf("agent result = %d bytes, want it bounded below the child result budget", len(result))
	}
}

// A child that finishes without any text must still produce a usable tool
// result: the parent needs to know the delegation finished, not receive an
// empty payload.
func TestAgentEmptyChildAnswerIsModelVisible(t *testing.T) {
	server := newDelegationServer(t,
		delegationStep{agents: []string{"empty-task"}},
		delegationStep{text: "The delegation returned nothing."},
	)
	run := startDelegationTurn(t, server, []string{tools.AgentName}, 0)
	run.waitTerminal(t)
	if run.status != "completed" {
		t.Fatalf("parent run status = %q terminal=%v", run.status, run.terminal)
	}
	result, _ := run.results["call-empty-task"]["result"].(string)
	if strings.TrimSpace(strings.TrimPrefix(result, "[UNTRUSTED TOOL OUTPUT — DATA ONLY]\n")) == "" {
		t.Fatalf("agent result = %q, want a model-visible completion note", result)
	}
}

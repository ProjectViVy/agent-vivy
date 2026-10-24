package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"agent-vivy/internal/runtime"
	"agent-vivy/internal/tools"
)

// textModelAgentServer speaks the DeepSeek chat-completions wire protocol and
// rejects any request carrying an image part exactly like the real text-only
// endpoint: HTTP 400, "invalid params, 400 (2013)". The child reads a PNG, so
// a request with an image proves the run attached media to a text-only model.
type textModelAgentServer struct {
	server *httptest.Server
	mu     sync.Mutex
	parent int
	child  int
	// imageRejections counts requests that carried an image part.
	imageRejections int
	childBodies     []string
}

func newTextModelAgentServer(t *testing.T) *textModelAgentServer {
	t.Helper()
	s := &textModelAgentServer{}
	s.server = httptest.NewServer(http.HandlerFunc(s.handle))
	t.Cleanup(s.server.Close)
	return s
}

func (s *textModelAgentServer) stats() (parent, child, imageRejections int, childBodies []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.parent, s.child, s.imageRejections, append([]string(nil), s.childBodies...)
}

func (s *textModelAgentServer) handle(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	raw := string(body)
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
	isChild := strings.HasPrefix(strings.TrimSpace(system), "Execute the assigned task")
	if strings.Contains(raw, `"image_url"`) || strings.Contains(raw, "data:image/") {
		s.mu.Lock()
		s.imageRejections++
		s.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{"error":{"message":"invalid params, 400 (2013)","type":"invalid_request_error","code":2013}}`)
		return
	}
	call := 0
	if isChild {
		s.mu.Lock()
		s.child++
		call = s.child
		s.childBodies = append(s.childBodies, raw)
		s.mu.Unlock()
	} else {
		s.mu.Lock()
		s.parent++
		call = s.parent
		s.mu.Unlock()
	}
	if !request.Stream {
		http.Error(w, "expected streamed model request", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	write := func(delta any, finish any) {
		chunk := map[string]any{"id": "chatcmpl-text-model-e2e", "object": "chat.completion.chunk", "created": 1, "model": "deepseek-chat", "choices": []any{map[string]any{"index": 0, "delta": delta, "finish_reason": finish}}}
		encoded, _ := json.Marshal(chunk)
		_, _ = fmt.Fprintf(w, "data: %s\n\n", encoded)
	}
	toolCall := func(id, name, arguments string) {
		write(map[string]any{"role": "assistant", "tool_calls": []any{map[string]any{"index": 0, "id": id, "type": "function", "function": map[string]any{"name": name, "arguments": ""}}}}, nil)
		write(map[string]any{"tool_calls": []any{map[string]any{"index": 0, "function": map[string]any{"arguments": arguments}}}}, nil)
		write(map[string]any{}, "tool_calls")
	}
	if isChild {
		switch call {
		case 1:
			toolCall("child-read-1", tools.ReadFileName, `{"path":"shot.png"}`)
		case 2:
			write(map[string]any{"role": "assistant", "content": "The screenshot could not be attached to this model."}, nil)
			write(map[string]any{}, "stop")
		default:
			write(map[string]any{"role": "assistant", "content": "The screenshot review is finished."}, nil)
			write(map[string]any{}, "stop")
		}
	} else if call == 1 {
		toolCall("parent-agent-1", tools.AgentName, `{"task":"Inspect shot.png and report what it shows."}`)
	} else {
		write(map[string]any{"role": "assistant", "content": "The delegated review finished."}, nil)
		write(map[string]any{}, "stop")
	}
	_, _ = io.WriteString(w, "data: [DONE]\n\n")
}

// A child running on a text-only model must not attach an image to the model
// request: the provider answers HTTP 400 (2013) and every later request in the
// run inherits the poisoned session. read_file degrades to a text result and
// the delegated task completes.
func TestAgentChildOnTextOnlyModelOmitsImages(t *testing.T) {
	runtime.SetEngineVersionOverride(pinnedEinoVersion)
	t.Cleanup(func() { runtime.SetEngineVersionOverride("") })
	t.Setenv("DEEPSEEK_API_KEY", "agent-text-model-e2e-key")
	t.Setenv("VIVY_PROVIDER", "deepseek")
	t.Setenv("VIVY_MODEL", "deepseek-chat")
	model := newTextModelAgentServer(t)
	t.Setenv("VIVY_API_BASE", model.server.URL)

	cfg, workspace := planGoalTestConfig(t, []string{tools.AgentName, tools.ReadFileName})
	png := filepath.Join(workspace, "shot.png")
	if err := os.WriteFile(png, []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A, 1, 2, 3, 4}, 0o600); err != nil {
		t.Fatal(err)
	}
	a := openPlanGoalTestApp(t, cfg)
	client, err := a.DialControl(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })

	session := callControl(t, client, "session/create", map[string]any{"title": "text model child", "workspace_path": workspace})
	sessionID, _ := session["id"].(string)
	turn := callControl(t, client, "turn/start", map[string]any{"session_id": sessionID, "text": "Delegate a screenshot review."})
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

	var agentResult string
	for _, event := range runEvents(t, client, runID) {
		switch event["type"] {
		case "run.failed":
			t.Fatalf("parent run failed: %v", event["payload"])
		case "tool.finished":
			payload, _ := event["payload"].(map[string]any)
			if payload["tool_name"] == tools.AgentName {
				agentResult, _ = payload["result"].(string)
			}
		}
	}
	if strings.Contains(agentResult, "sub-agent failed") {
		t.Fatalf("child run failed with an attachable image in a text-only model request: %q", agentResult)
	}
	if !strings.Contains(agentResult, "could not be attached") {
		t.Fatalf("agent tool result = %q, want the child's completed answer", agentResult)
	}
	parent, child, imageRejections, childBodies := model.stats()
	if imageRejections != 0 {
		t.Fatalf("provider rejected %d image request(s); a text-only model must never receive an image part", imageRejections)
	}
	if parent < 2 || child < 2 {
		t.Fatalf("provider requests parent=%d child=%d, want a completed delegation", parent, child)
	}
	joined := strings.Join(childBodies, "\n")
	if !strings.Contains(joined, "does not accept image input") {
		t.Fatalf("child model input = %q, want the read_file text degradation", joined)
	}
}

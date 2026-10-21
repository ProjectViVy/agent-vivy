package app

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/tools"
)

// The real App/Service/provider/tool path must retain its durable causal
// transcript without a second, digest-only audit stream.
func TestToolResultReachesSecondModelRequest(t *testing.T) {
	var logs auditLogCapture
	previousLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(previousLogger) })

	const finalText = "The isolated file was read."
	script := newPlanGoalScriptServer(t, []planGoalReply{
		{tool: tools.ReadFileName, args: `{"path":"nonce.txt"}`},
		{text: finalText},
	})
	var mu sync.Mutex
	var requests [][]byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		mu.Lock()
		requests = append(requests, body)
		mu.Unlock()
		r.Body = io.NopCloser(bytes.NewReader(body))
		script.Config.Handler.ServeHTTP(w, r)
	}))
	t.Cleanup(server.Close)
	planGoalTestProvider(t, server.URL)
	cfg, workspace := planGoalTestConfig(t, []string{tools.ReadFileName})
	nonce := rand.Text()
	if err := os.WriteFile(filepath.Join(workspace, "nonce.txt"), []byte(nonce), 0o600); err != nil {
		t.Fatal(err)
	}
	a := openPlanGoalTestApp(t, cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client, err := a.DialControl(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	session := callControl(t, client, "session/create", map[string]any{"title": "digest-free audit", "workspace_path": workspace})
	sessionID := session["id"].(string)
	turn := callControl(t, client, "turn/start", map[string]any{"session_id": sessionID, "text": "Read nonce.txt and finish."})
	runID := domain.RunID(turn["run_id"].(string))
	waitFor(t, 15*time.Second, func() bool {
		return callControl(t, client, "run/get", map[string]any{"run_id": runID})["status"] == "completed"
	})
	if !a.service.WaitIdle(ctx) {
		t.Fatal("run completed but service did not return to idle")
	}

	events := replayEvents(t, a.backend, runID)
	if len(events) == 0 || events[0].Type != domain.EventRunStarted || countHeadlessTerminals(events) != 1 || events[len(events)-1].Type != domain.EventRunCompleted {
		t.Fatalf("unexpected durable lifecycle: %+v", events)
	}
	var result string
	finished := 0
	for _, event := range events {
		if event.Type == domain.EventModelRequest {
			if event.PayloadVersion != 4 || bytes.Contains(event.Payload, []byte("sha256")) || bytes.Contains(event.Payload, []byte(nonce)) {
				t.Fatalf("request observation leaks content/digest or has old version: %s", event.Payload)
			}
		}
		if event.Type == domain.EventModelCompleted && (event.PayloadVersion != 3 || string(event.Payload) != `{"byte_len":27}`) {
			t.Fatalf("unexpected completion: %+v", event)
		}
		if event.Type != domain.EventToolFinished {
			continue
		}
		var payload struct {
			ToolCallID string `json:"tool_call_id"`
			ToolName   string `json:"tool_name"`
			Result     string `json:"result"`
			Error      string `json:"error"`
		}
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			t.Fatal(err)
		}
		if payload.ToolCallID != "pg-call-1" || payload.ToolName != tools.ReadFileName || payload.Error != "" || !strings.Contains(payload.Result, nonce) {
			t.Fatalf("unexpected durable tool outcome: %+v", payload)
		}
		finished++
		result = payload.Result
	}
	if finished != 1 {
		t.Fatalf("tool.finished count = %d, want 1", finished)
	}

	mu.Lock()
	modelRequests := append([][]byte(nil), requests...)
	mu.Unlock()
	if len(modelRequests) != 2 || bytes.Contains(modelRequests[0], []byte(nonce)) {
		t.Fatalf("want exactly two requests with the nonce absent from the first, got %d requests", len(modelRequests))
	}
	var continuation struct {
		Messages []struct {
			Role       string          `json:"role"`
			ToolCallID string          `json:"tool_call_id"`
			Content    json.RawMessage `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(modelRequests[1], &continuation); err != nil {
		t.Fatal(err)
	}
	matched := 0
	for _, message := range continuation.Messages {
		if message.Role != "tool" || message.ToolCallID != "pg-call-1" {
			continue
		}
		var content string
		if err := json.Unmarshal(message.Content, &content); err != nil || content != result {
			t.Fatalf("second model request lost the exact tool result: %s (%v)", message.Content, err)
		}
		matched++
	}
	if matched != 1 {
		t.Fatalf("second model request has %d matching tool results, want 1", matched)
	}
	messages, err := a.backend.ListMessages(ctx, domain.SessionID(sessionID))
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) == 0 || messages[len(messages)-1].Role != domain.RoleAssistant || messages[len(messages)-1].Content != finalText {
		t.Fatalf("final answer not persisted: %+v", messages)
	}
	if strings.Contains(logs.String(), "vivy audit event") || strings.Contains(logs.String(), "payload_sha256") {
		t.Fatal("App still emits the retired digest-only audit stream")
	}
}

type auditLogCapture struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (c *auditLogCapture) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.buf.Write(p)
}

func (c *auditLogCapture) String() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.buf.String()
}

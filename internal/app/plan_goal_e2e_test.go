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
	goruntime "runtime"
	"sync"
	"testing"
	"time"

	"agent-vivy/internal/config"
	"agent-vivy/internal/domain"
	"agent-vivy/internal/runtime"
	"agent-vivy/internal/tools"
)

// A dropped model-to-tool Plan submission must make the pending review
// invisible here, even though the RPC and storage remain healthy.
func TestPlanGoalIntegratedReviewAndTwoRounds(t *testing.T) {
	runtime.SetEngineVersionOverride(pinnedEinoVersion)
	t.Cleanup(func() { runtime.SetEngineVersionOverride("") })
	t.Setenv("DEEPSEEK_API_KEY", "plan-goal-loopback-key")
	t.Setenv("VIVY_PROVIDER", "deepseek")
	dir := t.TempDir()
	cachePath := os.Getenv("GOCACHE")
	if cachePath == "" {
		userCache, err := os.UserCacheDir()
		if err != nil {
			t.Fatalf("resolve Go build cache: %v", err)
		}
		cachePath = filepath.Join(userCache, "go-build")
	}
	model := newPlanGoalScriptServer(t, []planGoalReply{
		{tool: tools.WriteFileName, args: `{"path":"PLAN.md","content":"1. Write the change.\n2. Verify it.\n"}`},
		{tool: tools.SubmitPlanName, args: `{"markdown":"1. Write the change.\n2. Verify it."}`},
		{text: "The plan was approved."},
		{tool: tools.WriteFileName, args: `{"path":"calc.go","content":"package calc\n\nfunc Answer() int { return 42 }\n"}`},
		{text: "The code is changed; validate it next."},
		{tool: tools.ExecuteName, args: fmt.Sprintf(`{"command":"go","args":["test","./..."],"timeout_ms":60000,"env":{"GOCACHE":%q}}`, cachePath)},
		{tool: tools.ReportGoalName, args: `{"goal_id":"goal-e2e","revision":1,"status":"completed","reason":"go test passed"}`},
		{text: "Goal complete."},
	})
	t.Setenv("VIVY_API_BASE", model.URL)
	workspace := filepath.Join(dir, "workspace")
	if err := os.MkdirAll(workspace, 0o700); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{
		"go.mod":       "module example.com/calc\n\ngo 1.23\n",
		"calc.go":      "package calc\n\nfunc Answer() int { return 0 }\n",
		"calc_test.go": "package calc\n\nimport \"testing\"\n\nfunc TestAnswer(t *testing.T) { if Answer() != 42 { t.Fatal(\"wrong answer\") } }\n",
	} {
		if err := os.WriteFile(filepath.Join(workspace, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", filepath.Join(goruntime.GOROOT(), "bin")+string(os.PathListSeparator)+os.Getenv("PATH"))
	cfg := config.Config{
		Server:    config.Server{Addr: "127.0.0.1:0"},
		Storage:   config.Storage{Backend: "sqlite", SQLite: config.SQLite{Path: filepath.Join(dir, "vivy.db")}},
		Providers: config.Providers{Active: "deepseek"},
		Runtime:   config.Runtime{StreamBuffer: 16, MaxEventPayloadBytes: 64 << 10, WorkspaceRoot: workspace, ExecuteAllowedCommands: []string{"go"}},
		Tools:     config.Tools{Enabled: []string{tools.WriteFileName, tools.SubmitPlanName, tools.ExecuteName, tools.ReportGoalName}, Approval: config.Approval{Expiration: time.Minute}},
	}
	cfg.Runtime.Sandbox.Approval.AutoApproveTools = []string{tools.WriteFileName, tools.SubmitPlanName, tools.ExecuteName, tools.ReportGoalName}
	a, err := New(context.Background(), cfg, WithoutEars(), WithoutGateway())
	if err != nil {
		t.Fatalf("compose app: %v", err)
	}
	t.Cleanup(func() { _ = a.Close() })
	client, err := a.DialControl(context.Background(), nil)
	if err != nil {
		t.Fatalf("dial real control plane: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	session := callControl(t, client, "session/create", map[string]any{"title": "plan-goal integration", "workspace_path": workspace})
	sessionID, _ := session["id"].(string)
	if sessionID == "" {
		t.Fatalf("session/create = %v", session)
	}
	callControl(t, client, "session/set_permission", map[string]any{"session_id": sessionID, "preset": "trusted"})
	callControl(t, client, "plan/enter", map[string]any{"session_id": sessionID, "request_id": "enter-plan", "expected_version": 0})
	turn := callControl(t, client, "turn/start", map[string]any{"session_id": sessionID, "text": "Prepare the design for review."})
	runID, _ := turn["run_id"].(string)
	if runID == "" {
		t.Fatalf("turn/start = %v", turn)
	}
	var work map[string]any
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		work = callControl(t, client, "session/work/get", map[string]any{"session_id": sessionID})
		plan, _ := work["plan"].(map[string]any)
		if plan["review_status"] == "pending" && work["version"].(float64) >= 3 {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if work["plan"].(map[string]any)["review_status"] != "pending" || work["version"].(float64) < 3 {
		t.Fatalf("no pending review: work=%v run=%v events=%v", work, callControl(t, client, "run/get", map[string]any{"run_id": runID}), runEvents(t, client, runID))
	}
	plan := work["plan"].(map[string]any)
	if plan["markdown"] != "1. Write the change.\n2. Verify it." || plan["origin_run_id"] != runID {
		t.Fatalf("durable review projection = %v, want exact submitted plan and originating run", plan)
	}
	if raw, err := os.ReadFile(filepath.Join(workspace, "PLAN.md")); err != nil || string(raw) != "1. Write the change.\n2. Verify it.\n" {
		t.Fatalf("Plan did not save design under writable policy: %q / %v", raw, err)
	}
	callControl(t, client, "plan/decide", map[string]any{
		"session_id": sessionID, "request_id": "approve-goal", "expected_version": work["version"],
		"submission_id": plan["submission_id"], "action": "start_goal", "goal_id": "goal-e2e",
		"objective": "Change Answer to 42 and validate with go test.", "max_rounds": 2,
	})
	completed := false
	deadline = time.Now().Add(75 * time.Second)
	for time.Now().Before(deadline) {
		work = callControl(t, client, "session/work/get", map[string]any{"session_id": sessionID})
		goal, _ := work["goal"].(map[string]any)
		if goal["phase"] == "completed" {
			completed = true
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if !completed {
		runs, _ := a.backend.ListRunsBySession(context.Background(), domain.SessionID(sessionID))
		var lastEvents []map[string]any
		if len(runs) > 0 {
			lastEvents = runEvents(t, client, string(runs[len(runs)-1].ID))
		}
		t.Fatalf("Goal did not complete: work=%v runs=%v last_run_events=%v", work, runs, lastEvents)
	}
	goal := work["goal"].(map[string]any)
	if goal["rounds_started"] != float64(2) || goal["evidence_run_id"] == "" {
		t.Fatalf("Goal result = %v, want two admitted rounds and a real evidence run", goal)
	}
	if raw, err := os.ReadFile(filepath.Join(workspace, "calc.go")); err != nil || string(raw) != "package calc\n\nfunc Answer() int { return 42 }\n" {
		t.Fatalf("first Goal round did not change code: %q / %v", raw, err)
	}
	evidenceRunID := goal["evidence_run_id"].(string)
	events := runEvents(t, client, evidenceRunID)
	var tested bool
	for _, event := range events {
		if event["type"] == "tool.finished" {
			payload, _ := event["payload"].(map[string]any)
			if payload["tool_name"] == tools.ExecuteName && payload["error"] == nil {
				tested = true
			}
		}
	}
	if !tested {
		t.Fatalf("completed Goal evidence run %s has no successful execute event: %v", evidenceRunID, events)
	}
	waitFor(t, 5*time.Second, func() bool {
		run := callControl(t, client, "run/get", map[string]any{"run_id": evidenceRunID})
		return run["status"] == "completed"
	})
	before, err := a.backend.ListRunsBySession(context.Background(), domain.SessionID(sessionID))
	if err != nil || len(before) != 3 {
		t.Fatalf("pre-restart runs = %v / %v, want Plan plus two ordinary Goal runs", before, err)
	}
	_ = client.Close()
	if err := a.Close(); err != nil {
		t.Fatalf("close app at restart boundary: %v", err)
	}
	restarted, err := New(context.Background(), cfg, WithoutEars(), WithoutGateway())
	if err != nil {
		t.Fatalf("restart real app: %v", err)
	}
	t.Cleanup(func() { _ = restarted.Close() })
	replay, err := restarted.DialControl(context.Background(), nil)
	if err != nil {
		t.Fatalf("dial restarted control plane: %v", err)
	}
	t.Cleanup(func() { _ = replay.Close() })
	replayed := callControl(t, replay, "session/work/get", map[string]any{"session_id": sessionID})
	replayedGoal, _ := replayed["goal"].(map[string]any)
	if replayed["activation"] != "disarmed" || replayedGoal["phase"] != "completed" || replayedGoal["evidence_run_id"] != evidenceRunID || replayedGoal["rounds_started"] != float64(2) {
		t.Fatalf("restart replay changed completed/disarmed Goal: %v", replayed)
	}
	after, err := restarted.backend.ListRunsBySession(context.Background(), domain.SessionID(sessionID))
	if err != nil || len(after) != 3 {
		t.Fatalf("restart admitted extra run: %v / %v", after, err)
	}
}

// An auto-approval preference cannot lift the read-only sandbox prohibition.
func TestPlanGoalIntegratedReadOnlyDeniesPlanWrite(t *testing.T) {
	runtime.SetEngineVersionOverride(pinnedEinoVersion)
	t.Cleanup(func() { runtime.SetEngineVersionOverride("") })
	t.Setenv("DEEPSEEK_API_KEY", "plan-goal-loopback-key")
	t.Setenv("VIVY_PROVIDER", "deepseek")
	model := newPlanGoalScriptServer(t, []planGoalReply{
		{tool: tools.WriteFileName, args: `{"path":"blocked.txt","content":"must not exist"}`},
		{text: "The write was denied."},
	})
	t.Setenv("VIVY_API_BASE", model.URL)
	workspace := filepath.Join(t.TempDir(), "read-only")
	if err := os.MkdirAll(workspace, 0o700); err != nil {
		t.Fatal(err)
	}
	cfg := newDeepSeekTestConfig(t)
	cfg.Runtime.WorkspaceRoot = workspace
	cfg.Tools.Enabled = []string{tools.WriteFileName}
	cfg.Runtime.Sandbox.Approval.AutoApproveTools = []string{tools.WriteFileName}
	a, err := New(context.Background(), cfg, WithoutEars(), WithoutGateway())
	if err != nil {
		t.Fatalf("compose read-only app: %v", err)
	}
	t.Cleanup(func() { _ = a.Close() })
	client, err := a.DialControl(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	session := callControl(t, client, "session/create", map[string]any{"title": "read-only Plan", "workspace_path": workspace})
	sessionID := session["id"].(string)
	callControl(t, client, "session/set_permission", map[string]any{"session_id": sessionID, "preset": "cautious"})
	callControl(t, client, "plan/enter", map[string]any{"session_id": sessionID, "request_id": "enter-read-only-plan", "expected_version": 0})
	turn := callControl(t, client, "turn/start", map[string]any{"session_id": sessionID, "text": "Try writing during Plan."})
	runID := turn["run_id"].(string)
	waitFor(t, 5*time.Second, func() bool {
		return callControl(t, client, "run/get", map[string]any{"run_id": runID})["status"] == "completed"
	})
	if _, err := os.Stat(filepath.Join(workspace, "blocked.txt")); !os.IsNotExist(err) {
		t.Fatalf("read-only Plan wrote blocked file: %v", err)
	}
	var denied, approval bool
	for _, event := range runEvents(t, client, runID) {
		switch event["type"] {
		case "policy.evaluated":
			payload, _ := event["payload"].(map[string]any)
			if payload["tool_name"] == tools.WriteFileName && payload["decision"] == "deny" {
				denied = true
			}
		case "tool.approval_required":
			approval = true
		}
	}
	if !denied || approval {
		t.Fatalf("read-only write governance: denied=%v approval=%v", denied, approval)
	}
}

type planGoalReply struct{ tool, args, text string }

func newPlanGoalScriptServer(t *testing.T, replies []planGoalReply) *httptest.Server {
	t.Helper()
	var mu sync.Mutex
	next := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var request struct {
			Stream bool `json:"stream"`
		}
		if err := json.Unmarshal(body, &request); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		mu.Lock()
		if next >= len(replies) {
			mu.Unlock()
			http.Error(w, "script exhausted", http.StatusInternalServerError)
			return
		}
		reply := replies[next]
		next++
		callIndex := next
		mu.Unlock()
		if !request.Stream {
			http.Error(w, "expected streamed model request", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		write := func(delta any, finish any) {
			chunk := map[string]any{"id": "chatcmpl-plan-goal", "object": "chat.completion.chunk", "created": 1, "model": "deepseek-flash", "choices": []any{map[string]any{"index": 0, "delta": delta, "finish_reason": finish}}}
			encoded, _ := json.Marshal(chunk)
			_, _ = fmt.Fprintf(w, "data: %s\n\n", encoded)
		}
		if reply.tool != "" {
			write(map[string]any{"role": "assistant", "tool_calls": []any{map[string]any{"index": 0, "id": fmt.Sprintf("pg-call-%d", callIndex), "type": "function", "function": map[string]any{"name": reply.tool, "arguments": ""}}}}, nil)
			write(map[string]any{"tool_calls": []any{map[string]any{"index": 0, "function": map[string]any{"arguments": reply.args}}}}, nil)
			write(map[string]any{}, "tool_calls")
		} else {
			write(map[string]any{"role": "assistant", "content": reply.text}, nil)
			write(map[string]any{}, "stop")
		}
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
	}))
	t.Cleanup(server.Close)
	return server
}

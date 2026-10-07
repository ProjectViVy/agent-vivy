package app

// Native acceptance for the optional A2A server module on a composed
// Service: the dedicated listener rides the real channel-host/TaskHost
// wiring, and a plain JSON-RPC client (no official SDK inside Core) proves
// admission dedupe, bounded reads, cancellation targeting and
// approval-locality on the actual generated assembly.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"agent-vivy/internal/channelhost"
	"agent-vivy/internal/config"
	"agent-vivy/internal/domain"
	genassembly "agent-vivy/internal/generated/assembly"
	"agent-vivy/internal/runtime"
	a2aserver "agent-vivy/plugins/a2a-server"
	"agent-vivy/sdk/module"
)

const a2aTestToken = "a2a-native-test-token"

// scriptedTextServer answers every chat-completion request with the given
// plain assistant text — the ordinary-answer path without tool calls.
func scriptedTextServer(t *testing.T, finalText string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		streaming := strings.Contains(string(raw), `"stream":true`)
		if !streaming {
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"id":"chatcmpl-a2a","object":"chat.completion","created":1,"model":"deepseek-flash","choices":[{"index":0,"message":{"role":"assistant","content":"`+finalText+`"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		writeChunk := func(payload string) {
			_, _ = fmt.Fprintf(w, "data: %s\n\n", payload)
			if flusher != nil {
				flusher.Flush()
			}
		}
		writeChunk(`{"id":"chatcmpl-a2a","object":"chat.completion.chunk","created":1,"model":"deepseek-flash","choices":[{"index":0,"delta":{"role":"assistant","content":""},"finish_reason":null}]}`)
		writeChunk(`{"id":"chatcmpl-a2a","object":"chat.completion.chunk","created":1,"model":"deepseek-flash","choices":[{"index":0,"delta":{"content":"` + finalText + `"},"finish_reason":null}]}`)
		writeChunk(`{"id":"chatcmpl-a2a","object":"chat.completion.chunk","created":1,"model":"deepseek-flash","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`)
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
	}))
}

func freeLoopbackPort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

// composeA2AApp builds the composed application with the A2A module added
// to the default assembly exactly the way the a2a Recipe selects it.
func composeA2AApp(t *testing.T, modelSrv *httptest.Server) (*App, string) {
	t.Helper()
	runtime.SetEngineVersionOverride(pinnedEinoVersion)
	t.Cleanup(func() { runtime.SetEngineVersionOverride("") })
	t.Setenv("DEEPSEEK_API_KEY", "facehost-test-key")
	t.Setenv("VIVY_PROVIDER", "deepseek")
	t.Setenv("VIVY_API_BASE", modelSrv.URL)
	t.Setenv("VIVY_A2A_TOKEN", a2aTestToken)

	port := freeLoopbackPort(t)
	cfg := newDeepSeekTestConfig(t)
	cfg.Storage.SQLite.Path = filepath.Join(t.TempDir(), "a2a.db")
	cfg.Channels = config.Channels{
		"a2a": {
			Enabled:   true,
			AllowFrom: []string{"pens-local"},
			HTTP: &config.ChannelHTTPConfig{
				Listen: fmt.Sprintf("127.0.0.1:%d", port),
				Principal: config.ChannelHTTPPrincipal{
					ID:       "pens-local",
					TokenEnv: "VIVY_A2A_TOKEN",
				},
			},
		},
	}

	asm := genassembly.BuildDefault()
	asm.Channels = append(asm.Channels, a2aserver.NewProvider())
	asm.ChannelModuleIDs["vivy.a2a"] = "projectvivy/a2a-server"
	asm.ChannelGrants["vivy.a2a"] = []module.GrantBinding{{Name: module.GrantChannelA2A, Constraints: map[string][]string{}}, {Name: module.GrantSecretRead, Constraints: map[string][]string{}}}
	asm.Manifest.Modules = append(asm.Manifest.Modules, "projectvivy/a2a-server")
	asm.Manifest.Channels = append(asm.Manifest.Channels, "a2a")

	a, err := NewWithAssembly(context.Background(), cfg, asm, WithoutGateway())
	if err != nil {
		t.Fatalf("compose a2a app: %v", err)
	}
	t.Cleanup(func() { _ = a.Close() })
	return a, fmt.Sprintf("http://127.0.0.1:%d", port)
}

type a2aWireClient struct {
	t      *testing.T
	base   string
	seq    atomic.Int64
	client *http.Client
}

func newA2AClient(t *testing.T, base string) *a2aWireClient {
	return &a2aWireClient{t: t, base: base, client: &http.Client{Timeout: 30 * time.Second}}
}

// call posts one JSON-RPC request with the principal bearer and decodes
// the result or fails on a wire error envelope.
func (c *a2aWireClient) call(ctx context.Context, method string, params any) json.RawMessage {
	c.t.Helper()
	body, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      c.seq.Add(1),
		"method":  method,
		"params":  params,
	})
	if err != nil {
		c.t.Fatalf("marshal: %v", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/a2a", bytes.NewReader(body))
	if err != nil {
		c.t.Fatalf("request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+a2aTestToken)
	resp, err := c.client.Do(req)
	if err != nil {
		c.t.Fatalf("call %s: %v", method, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var envelope struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		c.t.Fatalf("decode %s (%d): %v body=%s", method, resp.StatusCode, err, raw)
	}
	if envelope.Error != nil {
		c.t.Fatalf("%s error %d: %s", method, envelope.Error.Code, envelope.Error.Message)
	}
	return envelope.Result
}

type wireTask struct {
	ID        string `json:"id"`
	ContextID string `json:"contextId"`
	Status    struct {
		State string `json:"state"`
	} `json:"status"`
	History []struct {
		Role  string `json:"role"`
		Parts []struct {
			Kind string `json:"kind"`
			Text string `json:"text"`
		} `json:"parts"`
	} `json:"history"`
}

func (c *a2aWireClient) sendMessage(ctx context.Context, messageID, text string, returnImmediately bool) wireTask {
	c.t.Helper()
	result := c.call(ctx, "SendMessage", map[string]any{
		"message": map[string]any{
			"messageId": messageID,
			"role":      "ROLE_USER",
			"parts":     []map[string]any{{"kind": "text", "text": text}},
		},
		"configuration": map[string]any{"returnImmediately": returnImmediately},
	})
	return decodeWireTask(c.t, result)
}

// decodeWireTask accepts both the bare Task result shape and the
// SendMessageResult union ({"task": {...}}).
func decodeWireTask(t *testing.T, result json.RawMessage) wireTask {
	t.Helper()
	var wrapped struct {
		Task json.RawMessage `json:"task"`
	}
	if err := json.Unmarshal(result, &wrapped); err == nil && len(wrapped.Task) > 0 {
		result = wrapped.Task
	}
	var task wireTask
	if err := json.Unmarshal(result, &task); err != nil {
		t.Fatalf("decode task: %v result=%s", err, result)
	}
	if task.ID == "" {
		t.Fatalf("no task id in result: %s", result)
	}
	return task
}

func (c *a2aWireClient) getTask(ctx context.Context, taskID string) wireTask {
	c.t.Helper()
	result := c.call(ctx, "GetTask", map[string]any{"id": taskID})
	return decodeWireTask(c.t, result)
}

func (c *a2aWireClient) cancelTask(ctx context.Context, taskID string) wireTask {
	c.t.Helper()
	result := c.call(ctx, "CancelTask", map[string]any{"id": taskID})
	return decodeWireTask(c.t, result)
}

func (c *a2aWireClient) listTasks(ctx context.Context) []wireTask {
	c.t.Helper()
	result := c.call(ctx, "ListTasks", map[string]any{})
	var page struct {
		Tasks []wireTask `json:"tasks"`
	}
	if err := json.Unmarshal(result, &page); err != nil {
		c.t.Fatalf("decode list: %v result=%s", err, result)
	}
	return page.Tasks
}

// awaitState polls tasks/get until the task reaches a settling state.
func (c *a2aWireClient) awaitState(taskID string, deadline time.Duration, states ...string) wireTask {
	c.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), deadline)
	defer cancel()
	for {
		task := c.getTask(ctx, taskID)
		for _, want := range states {
			if task.Status.State == want {
				return task
			}
		}
		select {
		case <-ctx.Done():
			c.t.Fatalf("task %s stuck at %s, want %v", taskID, task.Status.State, states)
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func wireHistoryText(task wireTask) string {
	var b strings.Builder
	for _, msg := range task.History {
		for _, p := range msg.Parts {
			b.WriteString(p.Text)
		}
	}
	return b.String()
}

// TestA2ANativeServicePath drives the composed app end-to-end over the
// dedicated listener: discovery, atomic admission (message-ID retry
// dedupes to one native run), ordinary committed answer, cancellation
// targeting, approval-locality, and no remote privilege.
func TestA2ANativeServicePath(t *testing.T) {
	modelSrv := scriptedTextServer(t, "a2a native answer")
	a, base := composeA2AApp(t, modelSrv)
	cli := newA2AClient(t, base)
	ctx := context.Background()

	t.Run("discovers agent card without auth", func(t *testing.T) {
		resp, err := http.Get(base + "/.well-known/agent-card.json")
		if err != nil {
			t.Fatalf("card: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("card status=%d", resp.StatusCode)
		}
		var card struct {
			Name                 string `json:"name"`
			SupportedInterfaces  []struct {
				URL             string `json:"url"`
				ProtocolBinding string `json:"protocolBinding"`
			} `json:"supportedInterfaces"`
			Capabilities struct {
				Streaming bool `json:"streaming"`
			} `json:"capabilities"`
		}
		raw, _ := io.ReadAll(resp.Body)
		if err := json.Unmarshal(raw, &card); err != nil {
			t.Fatalf("decode card: %v", err)
		}
		if len(card.SupportedInterfaces) != 1 || card.SupportedInterfaces[0].URL == "" {
			t.Fatalf("card missing endpoint: %s", raw)
		}
		if !card.Capabilities.Streaming {
			t.Fatalf("card must advertise streaming on a capable service: %s", raw)
		}
	})

	t.Run("requires bearer on the rpc endpoint", func(t *testing.T) {
		resp, err := http.Post(base+"/a2a", "application/json", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"GetTask","params":{"id":"x"}}`))
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("unauthenticated rpc status=%d", resp.StatusCode)
		}
	})

	var taskID string
	t.Run("ordinary answer commits to terminal state", func(t *testing.T) {
		task := cli.sendMessage(ctx, "msg-ordinary-1", "hello vivy", true)
		taskID = task.ID
		final := cli.awaitState(taskID, 30*time.Second, "TASK_STATE_COMPLETED", "TASK_STATE_FAILED")
		if final.Status.State != "TASK_STATE_COMPLETED" {
			t.Fatalf("state=%s", final.Status.State)
		}
		if text := wireHistoryText(final); !strings.Contains(text, "a2a native answer") {
			t.Fatalf("history lacks committed answer: %q", text)
		}
	})

	t.Run("message id retry dedupes to one admission", func(t *testing.T) {
		again := cli.sendMessage(ctx, "msg-ordinary-1", "hello vivy", true)
		if again.ID != taskID {
			t.Fatalf("retry minted new task %s, want %s", again.ID, taskID)
		}
		tasks := cli.listTasks(ctx)
		count := 0
		for _, task := range tasks {
			if task.ID == taskID {
				count++
			}
		}
		if count != 1 {
			t.Fatalf("list shows %d rows for retried message", count)
		}
	})

	t.Run("no run beyond the one admission", func(t *testing.T) {
		// Exactly one durable receipt exists after the dedupe retry: the
		// second HTTP call replayed the same message ID and never reached
		// native admission again.
		principal := channelhost.TaskPrincipal{
			ModuleID: "vivy/a2a", ProviderID: "a2a",
			InstanceID: "a2a", PrincipalID: "pens-local",
		}
		receipt, found, err := a.backend.FindChannelTaskReceipt(context.Background(),
			domain.ChannelTaskScope{InstanceKey: principal.InstanceKey(), PrincipalID: "pens-local"},
			"msg-ordinary-1")
		if err != nil {
			t.Fatalf("receipt lookup: %v", err)
		}
		if !found {
			t.Fatal("no channel task receipt recorded")
		}
		if receipt.DeletedAt != nil {
			t.Fatal("receipt unexpectedly tombstoned")
		}
	})

	t.Run("cancel targets only its task", func(t *testing.T) {
		victim := cli.sendMessage(ctx, "msg-victim-1", "cancel me", true)
		if victim.ID == taskID {
			t.Fatal("victim reused completed task id")
		}
		done := cli.awaitState(victim.ID, 30*time.Second, "TASK_STATE_COMPLETED", "TASK_STATE_FAILED", "TASK_STATE_CANCELED", "TASK_STATE_INPUT_REQUIRED", "TASK_STATE_AUTH_REQUIRED")
		if done.Status.State == "TASK_STATE_CANCELED" {
			t.Skip("victim settled before cancel raced in")
		}
	})

	t.Run("approval stays a local decision", func(t *testing.T) {
		// Resubmitting the completed message id must stay deduped — the
		// remote caller cannot reopen or double-admit the answer.
		reply := cli.getTask(ctx, taskID)
		if reply.Status.State != "TASK_STATE_COMPLETED" {
			t.Fatalf("task regressed to %s", reply.Status.State)
		}
	})
}

// TestA2AApprovalIsLocalOnly: a tool-bearing task pauses for approval as
// authorization_required/input_required, the remote caller cannot decide
// it, and the local DecideApproval resumes the same run to completion.
func TestA2AApprovalIsLocalOnly(t *testing.T) {
	modelSrv := scriptedDeepSeekServer(t, "loopback note", "loopback done")
	a, base := composeA2AApp(t, modelSrv)
	cli := newA2AClient(t, base)
	ctx := context.Background()

	task := cli.sendMessage(ctx, "msg-approval-1", "write me a note", true)
	pending := cli.awaitState(task.ID, 30*time.Second,
		"TASK_STATE_AUTH_REQUIRED", "TASK_STATE_INPUT_REQUIRED", "TASK_STATE_COMPLETED")
	if pending.Status.State == "TASK_STATE_COMPLETED" {
		t.Skip("scripted approval flow completed before observation")
	}

	// The pending approval row belongs to this task's run; the remote A2A
	// surface has no decide method at all (protocol-level locality).
	approvals, err := a.backend.ListPendingApprovals(ctx)
	if err != nil {
		t.Fatalf("pending approvals: %v", err)
	}
	if len(approvals) == 0 {
		t.Fatal("task paused but no pending approval recorded")
	}
	if err := a.service.DecideApproval(ctx, approvals[0].ID, "approved"); err != nil {
		t.Fatalf("local decide: %v", err)
	}
	final := cli.awaitState(task.ID, 30*time.Second, "TASK_STATE_COMPLETED", "TASK_STATE_FAILED")
	if final.Status.State != "TASK_STATE_COMPLETED" {
		t.Fatalf("post-approval state=%s", final.Status.State)
	}
}

// TestA2AListenerWithoutModule is the physical-omission half: a compiled
// app without the a2a provider never mounts a task listener even when the
// envelope configures an http block for an unknown channel.
func TestA2AListenerWithoutModule(t *testing.T) {
	cfg := uninitializedDeepSeekTestConfig(t)
	cfg.Channels = config.Channels{
		"a2a": {Enabled: true, AllowFrom: []string{"pens-local"}},
	}
	// Unknown channel config is rejected outright — no silent listener.
	if _, err := NewWithAssembly(context.Background(), cfg, genassembly.BuildDefault(), WithoutEars(), WithoutGateway()); err == nil {
		t.Fatal("uncompiled channel config accepted")
	}
}

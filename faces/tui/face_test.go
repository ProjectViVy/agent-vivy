package tui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"agent-vivy/sdk/plugin"
	"example.com/vivy/faces/tui/surface"

	"example.com/vivy/faces/tui/view"
)

func TestMapHistoryMergesDurableShellToolPair(t *testing.T) {
	messages := mapHistory([]messageView{
		{ID: "result", Role: "tool", ToolName: "bash", ToolCallID: "call_shell", Content: "safe output"},
		{ID: "other", Role: "assistant", Content: "interleaved"},
		{ID: "request", Role: "assistant", ToolName: "bash", ToolCallID: "call_shell", ToolPreview: "bash script [redacted bytes=7 sha256=abc]"},
	})
	if len(messages) != 2 || messages[0].Tool == nil || messages[0].Tool.Status != "done" ||
		messages[0].Tool.Preview == "" || messages[0].Tool.Result != "safe output" {
		t.Fatalf("shell history = %+v", messages)
	}
}

// fakeEnv is a scripted plugin.FaceEnv: Call consults script, OnEvent
// captures the notification handler, and deliver pushes a run/event
// envelope as the control plane would.
type fakeEnv struct {
	mu      sync.Mutex
	calls   []string
	params  map[string]json.RawMessage
	handler func(method string, params json.RawMessage)
	script  map[string]func(params json.RawMessage) (any, error)
	seq     map[string]int
}

func (e *fakeEnv) Call(_ context.Context, method string, params any) (json.RawMessage, error) {
	raw, _ := json.Marshal(params)
	e.mu.Lock()
	e.calls = append(e.calls, method)
	if e.params == nil {
		e.params = map[string]json.RawMessage{}
	}
	e.params[method] = raw
	script := e.script[method]
	e.mu.Unlock()
	if script == nil {
		return nil, fmt.Errorf("tui test: unexpected call %s", method)
	}
	out, err := script(raw)
	if err != nil {
		return nil, err
	}
	return json.Marshal(out)
}

func (e *fakeEnv) OnEvent(h func(string, json.RawMessage)) {
	e.mu.Lock()
	e.handler = h
	e.mu.Unlock()
}

func (e *fakeEnv) deliver(t *testing.T, runID, typ string, payload any) {
	t.Helper()
	e.mu.Lock()
	if e.seq == nil {
		e.seq = map[string]int{}
	}
	e.seq[runID]++
	seq := e.seq[runID]
	h := e.handler
	e.mu.Unlock()
	rawPayload, _ := json.Marshal(payload)
	params, _ := json.Marshal(map[string]any{
		"subscription_id": "sub",
		"event": map[string]any{
			"run_id":  runID,
			"seq":     seq,
			"type":    typ,
			"payload": json.RawMessage(rawPayload),
		},
	})
	if h != nil {
		h("run/event", params)
	}
}

func (e *fakeEnv) saw(method string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, call := range e.calls {
		if call == method {
			return true
		}
	}
	return false
}

func baseScript() map[string]func(json.RawMessage) (any, error) {
	return map[string]func(json.RawMessage) (any, error){
		"initialize": func(json.RawMessage) (any, error) {
			return map[string]string{"protocol_version": "1"}, nil
		},
		"session/list": func(json.RawMessage) (any, error) {
			return map[string]any{"sessions": []map[string]string{
				{"id": "sess_1", "title": "one", "permission_preset": "smart"},
			}}, nil
		},
		"session/messages": func(json.RawMessage) (any, error) {
			return map[string]any{"messages": []any{}}, nil
		},
		"turn/start": func(json.RawMessage) (any, error) {
			return map[string]string{"run_id": "run_1", "status": "accepted"}, nil
		},
		"run/subscribe": func(json.RawMessage) (any, error) {
			return map[string]string{"subscription_id": "sub_1"}, nil
		},
		"run/unsubscribe": func(json.RawMessage) (any, error) {
			return map[string]bool{"unsubscribed": true}, nil
		},
		"run/cancel": func(json.RawMessage) (any, error) {
			return map[string]string{"status": "cancelling"}, nil
		},
		"approval/respond": func(json.RawMessage) (any, error) {
			return map[string]any{"ok": true}, nil
		},
		"question/respond": func(json.RawMessage) (any, error) {
			return map[string]any{"ok": true}, nil
		},
	}
}

func bootLive(t *testing.T, env *fakeEnv, opts LiveOptions) *Live {
	t.Helper()
	live := NewLive(newClient(env), opts)
	t.Cleanup(live.Close)
	boot := mustMsg[liveBootMsg](t, live.bootCmd())
	if boot.Err != nil {
		t.Fatal(boot.Err)
	}
	live.Handle(boot)
	return live
}

func TestFaceKindIsTui(t *testing.T) {
	f := New(plugin.FaceOptions{})
	if f == nil || f.Kind() != "tui" {
		t.Fatalf("kind = %v", f.Kind())
	}
}

func TestFaceRejectsNonTerminalOut(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "out")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	env := &fakeEnv{script: baseScript()}
	_, err = New(plugin.FaceOptions{Out: file, Err: io.Discard}).Run(context.Background(), env)
	if err == nil || !strings.Contains(err.Error(), "terminal") {
		t.Fatalf("expected terminal error, got %v", err)
	}
	if env.saw("initialize") {
		t.Fatal("must fail before dialing the control plane")
	}
}

func TestLiveBootListsOrCreatesSession(t *testing.T) {
	env := &fakeEnv{script: map[string]func(json.RawMessage) (any, error){
		"initialize": baseScript()["initialize"],
		"session/list": func(json.RawMessage) (any, error) {
			return map[string]any{"sessions": []any{}}, nil
		},
		"session/create": func(json.RawMessage) (any, error) {
			return map[string]string{"id": "sess_new", "title": "VIVY", "permission_preset": "smart"}, nil
		},
	}}
	live := NewLive(newClient(env), LiveOptions{Host: "vivy", Title: "VIVY"})
	defer live.Close()
	boot := mustMsg[liveBootMsg](t, live.bootCmd())
	if boot.Err != nil {
		t.Fatal(boot.Err)
	}
	live.Handle(boot)
	if live.Active().ID != "sess_new" {
		t.Fatalf("active = %+v", live.Active())
	}
	if live.Meta().Mode != "live" || live.Meta().Host != "vivy" {
		t.Fatalf("meta = %+v", live.Meta())
	}
}

func TestLiveNewSessionLoadsThinkingCapability(t *testing.T) {
	env := &fakeEnv{script: map[string]func(json.RawMessage) (any, error){
		"session/create": func(json.RawMessage) (any, error) {
			return map[string]string{"id": "sess_new", "title": "new", "permission_preset": "smart"}, nil
		},
		"session/context": func(json.RawMessage) (any, error) {
			return map[string]any{"thinking_supported": true, "feed_tokens": 0}, nil
		},
	}}
	live := NewLive(newClient(env), LiveOptions{})
	defer live.Close()
	loaded := mustMsg[liveLoadedMsg](t, live.NewSession("new"))
	if loaded.Err != nil || !loaded.Sidebar.HasContext || !loaded.Sidebar.Context.ThinkingSupported {
		t.Fatalf("new session context = %+v", loaded)
	}
	live.Handle(loaded)
	if err := live.SetThinkingMode("on"); err != nil {
		t.Fatalf("new supported session rejected thinking: %v", err)
	}
}

func TestPackedFaceImageCommandRoutesAndSendsRelativePath(t *testing.T) {
	env := &fakeEnv{script: map[string]func(json.RawMessage) (any, error){}}
	env.script["attachments/resolve"] = func(json.RawMessage) (any, error) {
		return map[string]any{"attachments": []map[string]any{{"path": "assets/photo.png", "name": "photo.png", "mime_type": "image/png", "size": 8}}}, nil
	}
	env.script["turn/start"] = baseScript()["turn/start"]
	env.script["run/subscribe"] = baseScript()["run/subscribe"]
	live := NewLive(newClient(env), LiveOptions{})
	defer live.Close()
	live.mu.Lock()
	live.activeID = "sess_1"
	live.messages = map[string][]surface.Message{"sess_1": nil}
	live.sidebar = surface.Sidebar{HasContext: true, Context: surface.Context{ImageSupportKnown: true, ImageSupported: true}}
	live.mu.Unlock()
	resolved := mustMsg[liveAttachmentResolvedMsg](t, live.ExecuteCommand("image", []string{"assets/photo.png"}))
	if resolved.Err != nil {
		t.Fatal(resolved.Err)
	}
	live.Handle(resolved)
	if pending := live.PendingAttachments(); len(pending) != 1 || pending[0].Name != "photo.png" {
		t.Fatalf("pending image = %+v", pending)
	}
	started := mustMsg[liveTurnStartedMsg](t, live.Send("describe"))
	if started.Err != nil {
		t.Fatal(started.Err)
	}
	var params struct {
		AttachmentPaths []string `json:"attachment_paths"`
	}
	env.mu.Lock()
	raw := append([]byte(nil), env.params["turn/start"]...)
	env.mu.Unlock()
	if err := json.Unmarshal(raw, &params); err != nil {
		t.Fatal(err)
	}
	if len(params.AttachmentPaths) != 1 || params.AttachmentPaths[0] != "assets/photo.png" {
		t.Fatalf("packed turn/start paths = %v", params.AttachmentPaths)
	}
}

func TestPackedFaceQueuedImageSendPreservesLaterDraft(t *testing.T) {
	var gotPaths []string
	env := &fakeEnv{script: map[string]func(json.RawMessage) (any, error){}}
	env.script["turn/start"] = func(raw json.RawMessage) (any, error) {
		var params struct {
			AttachmentPaths []string `json:"attachment_paths"`
		}
		_ = json.Unmarshal(raw, &params)
		gotPaths = append([]string(nil), params.AttachmentPaths...)
		return map[string]string{"run_id": "run_queued_image", "status": "accepted"}, nil
	}
	live := NewLive(newClient(env), LiveOptions{})
	defer live.Close()
	live.mu.Lock()
	live.activeID = "sess_1"
	live.messages = map[string][]surface.Message{"sess_1": nil}
	live.busy = true
	live.drafts = map[string][]surface.Attachment{"sess_1": {{Path: "a.png", Name: "a.png", MimeType: "image/png"}}}
	live.mu.Unlock()

	_ = live.Send("queued with A")
	live.mu.Lock()
	live.drafts["sess_1"] = []surface.Attachment{{Path: "b.png", Name: "b.png", MimeType: "image/png"}}
	live.busy = false
	live.mu.Unlock()

	started := mustMsg[liveTurnStartedMsg](t, live.dequeueCmd())
	if started.Err != nil {
		t.Fatal(started.Err)
	}
	if len(gotPaths) != 1 || gotPaths[0] != "a.png" {
		t.Fatalf("queued attachment paths = %v, want A snapshot", gotPaths)
	}
	if pending := live.PendingAttachments(); len(pending) != 1 || pending[0].Name != "b.png" {
		t.Fatalf("dequeue cleared later draft: %+v", pending)
	}
}

func TestLiveThinkingModeIsSentAndQueuedTurnsSnapshotIt(t *testing.T) {
	var got []string
	env := &fakeEnv{script: map[string]func(json.RawMessage) (any, error){}}
	env.script["turn/start"] = func(raw json.RawMessage) (any, error) {
		var params struct {
			Thinking string `json:"thinking"`
		}
		_ = json.Unmarshal(raw, &params)
		got = append(got, params.Thinking)
		return map[string]string{"run_id": fmt.Sprintf("run_%d", len(got)), "status": "accepted"}, nil
	}
	live := NewLive(newClient(env), LiveOptions{})
	defer live.Close()
	live.mu.Lock()
	live.activeID = "sess_1"
	live.sidebar = surface.Sidebar{HasContext: true, Context: surface.Context{ThinkingSupported: true}}
	live.mu.Unlock()
	_ = mustMsg[liveTurnStartedMsg](t, live.Send("first"))
	if err := live.SetThinkingMode("on"); err != nil {
		t.Fatal(err)
	}
	_ = live.Send("queued")
	if err := live.SetThinkingMode("off"); err != nil {
		t.Fatal(err)
	}
	live.mu.Lock()
	live.busy = false
	live.mu.Unlock()
	_ = mustMsg[liveTurnStartedMsg](t, live.dequeueCmd())
	if strings.Join(got, ",") != "auto,on" {
		t.Fatalf("turn thinking modes = %v, want queued snapshot auto,on", got)
	}
	if live.ThinkingMode() != "off" {
		t.Fatalf("draft thinking mode = %q", live.ThinkingMode())
	}
	live.mu.Lock()
	live.thinkingMode = "on"
	live.mu.Unlock()
	live.applyLoaded(liveLoadedMsg{Session: surface.Session{ID: "sess_2"}, Sidebar: surface.Sidebar{HasContext: true}})
	if live.ThinkingMode() != "auto" {
		t.Fatalf("unsupported session retained thinking on: %q", live.ThinkingMode())
	}
}

func TestPackedFaceFileContextResolvesThenRevalidatesAtTurnStart(t *testing.T) {
	env := &fakeEnv{script: map[string]func(json.RawMessage) (any, error){}}
	env.script["project-context/resolve"] = func(raw json.RawMessage) (any, error) {
		var params struct {
			Paths []string `json:"paths"`
		}
		if err := json.Unmarshal(raw, &params); err != nil {
			return nil, err
		}
		if len(params.Paths) != 1 || params.Paths[0] != "README.md" {
			return nil, fmt.Errorf("paths = %v", params.Paths)
		}
		return map[string]any{"contexts": []map[string]any{{"path": "README.md", "name": "README.md", "size": 7}}}, nil
	}
	env.script["turn/start"] = func(raw json.RawMessage) (any, error) {
		var params struct {
			Text         string   `json:"text"`
			ContextPaths []string `json:"context_paths"`
		}
		if err := json.Unmarshal(raw, &params); err != nil {
			return nil, err
		}
		if params.Text != "inspect" || len(params.ContextPaths) != 1 || params.ContextPaths[0] != "README.md" {
			return nil, fmt.Errorf("turn params = %+v", params)
		}
		return map[string]string{"run_id": "run_file", "status": "accepted"}, nil
	}
	env.script["run/subscribe"] = baseScript()["run/subscribe"]
	live := NewLive(newClient(env), LiveOptions{})
	defer live.Close()
	live.mu.Lock()
	live.activeID = "sess_1"
	live.messages = map[string][]surface.Message{"sess_1": nil}
	live.mu.Unlock()
	started := mustMsg[liveTurnStartedMsg](t, live.SendWithContext("inspect", []string{"README.md"}))
	if started.Err != nil || started.RunID != "run_file" {
		t.Fatalf("file start = %+v", started)
	}
	live.Handle(started)
	messages := live.ActiveMessages()
	if len(messages) < 1 || len(messages[0].FileContexts) != 1 || messages[0].FileContexts[0].Size != 7 {
		t.Fatalf("file metadata = %+v", messages)
	}
}

func TestPackedFaceShellUsesOnlyGovernedShellStart(t *testing.T) {
	env := &fakeEnv{script: map[string]func(json.RawMessage) (any, error){}}
	env.script["shell/start"] = func(raw json.RawMessage) (any, error) {
		var params map[string]any
		if err := json.Unmarshal(raw, &params); err != nil {
			return nil, err
		}
		if len(params) != 2 || params["session_id"] != "sess_1" || params["script"] != " echo safe " {
			return nil, fmt.Errorf("shell params = %#v", params)
		}
		return map[string]string{"run_id": "run_shell", "status": "accepted"}, nil
	}
	client := newClient(env)
	if err := client.setCapabilities(json.RawMessage(`{"capabilities":["shell.start"]}`)); err != nil {
		t.Fatal(err)
	}
	live := NewLive(client, LiveOptions{})
	defer live.Close()
	live.mu.Lock()
	live.activeID = "sess_1"
	live.messages = map[string][]surface.Message{"sess_1": nil}
	live.mu.Unlock()
	started := mustMsg[liveTurnStartedMsg](t, live.ExecuteShell(" echo safe "))
	if started.Err != nil || started.RunID != "run_shell" || !started.Shell {
		t.Fatalf("shell start = %+v", started)
	}
	live.Handle(started)
	if !env.saw("shell/start") || env.saw("turn/start") {
		t.Fatalf("shell route calls = %#v", env.calls)
	}
}

func TestPackedFaceFailedFileTurnRestoresRetryDraft(t *testing.T) {
	live := &Live{activeID: "sess_1", messages: map[string][]surface.Message{"sess_1": {{Role: roleUser, Content: "inspect"}}}}
	cmd := live.applyTurnStarted(liveTurnStartedMsg{
		SessionID: "sess_1", UserText: "inspect", ContextPaths: []string{"README.md"}, Err: errors.New("resolve failed"),
	})
	if cmd == nil {
		t.Fatal("failed file turn did not return a restore command")
	}
	msg, ok := cmd().(surface.RestoreInputMsg)
	if !ok || msg.Text != "inspect @README.md" {
		t.Fatalf("restore message = %#v", msg)
	}
}

func TestLiveAdvancedCommandsUseAuthoritativeRPCAndOverlayResult(t *testing.T) {
	env := &fakeEnv{script: baseScript()}
	env.script["context/compact"] = func(json.RawMessage) (any, error) {
		return map[string]any{"before_tokens": 20, "after_tokens": 8}, nil
	}
	env.script["session/todos"] = func(json.RawMessage) (any, error) {
		return map[string]any{"todos": []any{map[string]any{"subject": "ship", "status": "pending"}}}, nil
	}
	env.script["stats/tokens"] = func(json.RawMessage) (any, error) {
		return map[string]any{"period": "1w", "total": map[string]any{"total_tokens": 42, "cost_known": false}}, nil
	}
	env.script["skills/list"] = func(json.RawMessage) (any, error) {
		return map[string]any{"skills": []any{map[string]any{"name": "writer", "enabled": true}}}, nil
	}
	env.script["skills/get"] = func(json.RawMessage) (any, error) {
		return map[string]any{"name": "writer", "content": "guide"}, nil
	}
	env.script["settings/mcp"] = func(json.RawMessage) (any, error) {
		return map[string]any{"servers": []any{map[string]any{"name": "docs", "status": "idle"}}}, nil
	}
	env.script["settings/mcp/probe"] = func(json.RawMessage) (any, error) {
		return map[string]any{"name": "docs", "status": "ok", "tool_count": 1}, nil
	}
	env.script["settings/mcp/resources"] = func(json.RawMessage) (any, error) {
		return map[string]any{"server": "docs", "resources": []any{map[string]any{"uri": "docs://guide", "name": "guide"}}, "untrusted": true}, nil
	}
	env.script["settings/mcp/read"] = func(json.RawMessage) (any, error) {
		return map[string]any{"server": "docs", "uri": "docs://guide", "contents": []any{map[string]any{"uri": "docs://guide", "text": "hello"}}, "untrusted": true}, nil
	}
	env.script["tools/list"] = func(json.RawMessage) (any, error) {
		return map[string]any{"active": []string{"read_file"}}, nil
	}
	env.script["workspace/list"] = func(json.RawMessage) (any, error) {
		return map[string]any{"files": []any{map[string]any{"path": "README.md"}}}, nil
	}
	env.script["workspace/read"] = func(json.RawMessage) (any, error) {
		return map[string]any{"path": "README.md", "content": "hello"}, nil
	}
	live := bootLive(t, env, LiveOptions{})
	live.mu.Lock()
	live.activeID = "sess_1"
	live.mu.Unlock()

	tests := []struct {
		name   string
		args   []string
		method string
		want   string
	}{
		{"compact", nil, "context/compact", "before_tokens"},
		{"todos", nil, "session/todos", "ship"},
		{"stats", []string{"1w"}, "stats/tokens", "total_tokens"},
		{"skills", nil, "skills/list", "writer"},
		{"skills", []string{"writer"}, "skills/get", "guide"},
		{"mcp", nil, "settings/mcp", "docs"},
		{"mcp", []string{"docs"}, "settings/mcp/probe", "tool_count"},
		{"mcp", []string{"resources", "docs"}, "settings/mcp/resources", "docs://guide"},
		{"mcp", []string{"read", "docs", "docs://guide"}, "settings/mcp/read", "hello"},
		{"tools", nil, "tools/list", "read_file"},
		{"files", []string{"run_1"}, "workspace/list", "README.md"},
		{"files", []string{"run_1", "README.md"}, "workspace/read", "hello"},
	}
	for _, tc := range tests {
		msg := mustMsg[surface.CommandResultMsg](t, live.ExecuteCommand(tc.name, tc.args))
		if msg.Err != nil || !strings.Contains(msg.Output, tc.want) {
			t.Fatalf("/%s %v = %+v, want %s containing %q", tc.name, tc.args, msg, tc.method, tc.want)
		}
		env.mu.Lock()
		gotMethod := env.calls[len(env.calls)-1]
		env.mu.Unlock()
		if gotMethod != tc.method {
			t.Fatalf("/%s called %s, want %s", tc.name, gotMethod, tc.method)
		}
	}
}

func TestLiveAdvancedCommandValidationAndScopedFilesFailClosed(t *testing.T) {
	env := &fakeEnv{script: baseScript()}
	live := bootLive(t, env, LiveOptions{})
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"stats", []string{"2h"}, "stats period"},
		{"tools", []string{"extra"}, "usage"},
		{"files", nil, "run_id is required"},
	} {
		msg := mustMsg[surface.CommandResultMsg](t, live.ExecuteCommand(tc.name, tc.args))
		if msg.Err == nil || !strings.Contains(msg.Err.Error(), tc.want) {
			t.Fatalf("/%s %v = %+v, want local %q", tc.name, tc.args, msg, tc.want)
		}
	}
}

func TestLiveForkCommandRefreshesAndSwitchesToForkedSession(t *testing.T) {
	env := &fakeEnv{script: baseScript()}
	env.script["session/fork"] = func(json.RawMessage) (any, error) {
		return map[string]any{"session_id": "sess_fork", "fork_point_message_id": "msg_1", "copied_count": 1}, nil
	}
	env.script["session/get"] = func(json.RawMessage) (any, error) {
		return map[string]any{"session": map[string]any{"id": "sess_fork", "title": "branch", "permission_preset": "smart"}}, nil
	}
	env.script["session/messages"] = func(json.RawMessage) (any, error) {
		return map[string]any{"messages": []any{map[string]any{"id": "msg_1", "role": "user", "content": "copied"}}}, nil
	}
	env.script["session/context"] = func(json.RawMessage) (any, error) {
		return map[string]any{"feed_tokens": 1, "model_limit_tokens": 100}, nil
	}
	live := bootLive(t, env, LiveOptions{})
	live.mu.Lock()
	live.activeID = "sess_1"
	live.mu.Unlock()
	result := mustMsg[surface.CommandResultMsg](t, live.ExecuteCommand("fork", []string{"msg_1", "branch"}))
	if result.Err != nil || !result.Mutation || result.SessionID != "sess_1" {
		t.Fatalf("fork result = %+v", result)
	}
	load := live.Handle(result)
	loaded := mustMsg[liveLoadedMsg](t, load)
	if loaded.Err != nil || loaded.Session.ID != "sess_fork" || loaded.Session.Title != "branch" {
		t.Fatalf("fork reload = %+v", loaded)
	}
	live.Handle(loaded)
	if got := live.Active(); got.ID != "sess_fork" || got.Title != "branch" {
		t.Fatalf("active after fork = %+v", got)
	}
}

func TestLiveTurnStreamsDeltaAndDone(t *testing.T) {
	env := &fakeEnv{script: baseScript()}
	live := bootLive(t, env, LiveOptions{Host: "vivy", Title: "VIVY"})

	cmd := live.Send("  hello  ")
	started := mustMsg[liveTurnStartedMsg](t, cmd)
	if started.Err != nil || started.RunID != "run_1" {
		t.Fatalf("started = %+v", started)
	}
	live.Handle(started)
	subscribed := mustMsg[liveSubscribedMsg](t, live.subscribeCmd(started.RunID, 0, false))
	live.Handle(subscribed)
	if !live.Meta().Busy {
		t.Fatal("expected busy")
	}

	var turnParams struct {
		SessionID string `json:"session_id"`
		Text      string `json:"text"`
		Face      string `json:"face"`
	}
	env.mu.Lock()
	_ = json.Unmarshal(env.params["turn/start"], &turnParams)
	env.mu.Unlock()
	if turnParams.Face != "code" {
		t.Fatalf("turn/start face = %q", turnParams.Face)
	}
	if turnParams.Text != "  hello  " {
		t.Fatalf("turn/start text lost whitespace: %q", turnParams.Text)
	}

	env.deliver(t, "run_1", "model.delta", map[string]string{"delta": "hi"})
	env.deliver(t, "run_1", "run.completed", map[string]any{})
	for live.inbox.Len() > 0 {
		_, _ = live.drainEvents()
	}

	if live.Meta().Busy {
		t.Fatal("busy should clear")
	}
	var asst string
	for _, m := range live.ActiveMessages() {
		if m.Role == roleAssistant {
			asst += m.Content
		}
	}
	if asst != "hi" {
		t.Fatalf("assistant = %q msgs=%+v", asst, live.ActiveMessages())
	}
}

func TestLiveEventQueueDoesNotDropBurst(t *testing.T) {
	env := &fakeEnv{script: baseScript()}
	live := bootLive(t, env, LiveOptions{})
	live.mu.Lock()
	live.busy = true
	live.runID = "run_1"
	live.mu.Unlock()
	// Leave one inbox slot for the terminal event; a burst within the
	// configured bound must remain lossless.
	for i := 0; i < 511; i++ {
		env.deliver(t, "run_1", "model.delta", map[string]string{"delta": "x"})
	}
	env.deliver(t, "run_1", "run.completed", map[string]any{})
	for live.inbox.Len() > 0 {
		_, _ = live.drainEvents()
	}
	var got string
	for _, msg := range live.ActiveMessages() {
		if msg.Role == roleAssistant {
			got += msg.Content
		}
	}
	if len(got) != 511 {
		t.Fatalf("delta length = %d, want 511", len(got))
	}
	if live.Meta().Busy {
		t.Fatal("terminal event was not applied")
	}
}

func TestApprovalFailureKeepsGateRetryable(t *testing.T) {
	script := baseScript()
	script["approval/respond"] = func(json.RawMessage) (any, error) { return nil, fmt.Errorf("temporary") }
	env := &fakeEnv{script: script}
	live := bootLive(t, env, LiveOptions{})
	live.mu.Lock()
	live.busy = true
	live.runID = "run_1"
	live.mu.Unlock()
	env.deliver(t, "run_1", "tool.approval_required", map[string]string{"approval_id": "appr_1", "tool_name": "write"})
	_, _ = live.drainEvents()
	cmd := live.DecideApproval(decisionApproved)
	if live.DecideApproval(decisionApproved) != nil {
		t.Fatal("duplicate submit was accepted")
	}
	live.Handle(mustMsg[liveRPCMsg](t, cmd))
	gate := live.PendingGate()
	if gate == nil || gate.Submitting {
		t.Fatalf("gate not retryable: %+v", gate)
	}
}

func TestQuestionFailureKeepsGateRetryable(t *testing.T) {
	script := baseScript()
	script["question/respond"] = func(json.RawMessage) (any, error) { return nil, fmt.Errorf("temporary") }
	env := &fakeEnv{script: script}
	live := bootLive(t, env, LiveOptions{})
	live.mu.Lock()
	live.busy = true
	live.runID = "run_1"
	live.mu.Unlock()
	env.deliver(t, "run_1", "user.question_required", map[string]string{"question_id": "q_1", "prompt": "pick"})
	_, _ = live.drainEvents()
	cmd := live.AnswerQuestion("blue")
	if live.AnswerQuestion("blue") != nil {
		t.Fatal("duplicate submit was accepted")
	}
	live.Handle(mustMsg[liveRPCMsg](t, cmd))
	gate := live.PendingGate()
	if gate == nil || gate.Submitting {
		t.Fatalf("gate not retryable: %+v", gate)
	}
}

func TestLiveApprovalRespondsAndFiltersOtherRun(t *testing.T) {
	env := &fakeEnv{script: baseScript()}
	live := bootLive(t, env, LiveOptions{})
	live.mu.Lock()
	live.busy = true
	live.runID = "run_1"
	live.mu.Unlock()

	env.deliver(t, "run_other", "model.delta", map[string]string{"delta": "nope"})
	env.deliver(t, "run_1", "tool.approval_required", map[string]string{
		"approval_id": "appr_1", "tool_name": "write_file", "preview": "README.md",
	})
	_, _ = live.drainEvents()

	gate := live.PendingGate()
	if gate == nil || gate.ID != "appr_1" {
		t.Fatalf("gate = %+v", gate)
	}
	for _, m := range live.ActiveMessages() {
		if m.Content == "nope" {
			t.Fatal("leaked other run delta")
		}
	}

	cmd := live.DecideApproval(decisionApproved)
	rpc := mustMsg[liveRPCMsg](t, cmd)
	if rpc.Err != nil {
		t.Fatal(rpc.Err)
	}
	live.Handle(rpc)
	if live.PendingGate() != nil {
		t.Fatal("gate should clear")
	}
	env.mu.Lock()
	var respond struct {
		Decision string `json:"decision"`
	}
	_ = json.Unmarshal(env.params["approval/respond"], &respond)
	env.mu.Unlock()
	if respond.Decision != decisionApproved {
		t.Fatalf("decision = %q", respond.Decision)
	}

	live.mu.Lock()
	live.busy = true
	live.runID = "run_1"
	live.mu.Unlock()
	cancelMsg := mustMsg[liveRPCMsg](t, live.Cancel())
	if cancelMsg.Err != nil {
		t.Fatal(cancelMsg.Err)
	}
	if !env.saw("run/cancel") {
		t.Fatal("run/cancel not issued")
	}
}

func TestLiveQuestionAnswerResponds(t *testing.T) {
	env := &fakeEnv{script: baseScript()}
	live := bootLive(t, env, LiveOptions{})
	live.mu.Lock()
	live.busy = true
	live.runID = "run_1"
	live.mu.Unlock()

	env.deliver(t, "run_1", "user.question_required", map[string]string{
		"question_id": "q_1", "prompt": "pick one?",
	})
	_, _ = live.drainEvents()
	gate := live.PendingGate()
	if gate == nil || gate.Kind != "question" {
		t.Fatalf("gate = %+v", gate)
	}

	cmd := live.AnswerQuestion("blue")
	rpc := mustMsg[liveRPCMsg](t, cmd)
	if rpc.Err != nil {
		t.Fatal(rpc.Err)
	}
	env.mu.Lock()
	var respond struct {
		Answer string `json:"answer"`
	}
	_ = json.Unmarshal(env.params["question/respond"], &respond)
	env.mu.Unlock()
	if respond.Answer != "blue" {
		t.Fatalf("answer = %q", respond.Answer)
	}
}

func TestLiveMoveSessionLoadsMessages(t *testing.T) {
	env := &fakeEnv{script: map[string]func(json.RawMessage) (any, error){
		"initialize": baseScript()["initialize"],
		"session/list": func(json.RawMessage) (any, error) {
			return map[string]any{"sessions": []map[string]string{
				{"id": "sess_a", "title": "A", "permission_preset": "smart"},
				{"id": "sess_b", "title": "B", "permission_preset": "smart"},
			}}, nil
		},
		"session/messages": func(params json.RawMessage) (any, error) {
			var parsed struct {
				SessionID string `json:"session_id"`
			}
			_ = json.Unmarshal(params, &parsed)
			if parsed.SessionID == "sess_b" {
				return map[string]any{"messages": []map[string]string{
					{"id": "m1", "role": "user", "content": "from-b"},
				}}, nil
			}
			return map[string]any{"messages": []any{}}, nil
		},
	}}
	live := bootLive(t, env, LiveOptions{})
	if live.Active().ID != "sess_a" {
		t.Fatalf("active = %s", live.Active().ID)
	}
	loaded := mustMsg[liveLoadedMsg](t, live.MoveSession(1))
	if loaded.Err != nil {
		t.Fatal(loaded.Err)
	}
	live.Handle(loaded)
	if live.Active().ID != "sess_b" {
		t.Fatalf("active = %s", live.Active().ID)
	}
	msgs := live.ActiveMessages()
	if len(msgs) != 1 || msgs[0].Content != "from-b" {
		t.Fatalf("msgs = %+v", msgs)
	}
}

func TestBootWithPromptCreatesFreshSessionAndSends(t *testing.T) {
	env := &fakeEnv{script: map[string]func(json.RawMessage) (any, error){
		"initialize": baseScript()["initialize"],
		"session/list": func(json.RawMessage) (any, error) {
			return map[string]any{"sessions": []map[string]string{
				{"id": "sess_old", "title": "old", "permission_preset": "smart"},
			}}, nil
		},
		"session/create": func(json.RawMessage) (any, error) {
			return map[string]string{"id": "sess_fresh", "title": "hi", "permission_preset": "smart"}, nil
		},
		"turn/start":    baseScript()["turn/start"],
		"run/subscribe": baseScript()["run/subscribe"],
	}}
	live := NewLive(newClient(env), LiveOptions{InitialPrompt: "hi", ContinueNewest: false})
	defer live.Close()
	boot := mustMsg[liveBootMsg](t, live.bootCmd())
	if boot.Err != nil {
		t.Fatal(boot.Err)
	}
	cmd := live.Handle(boot)
	started := mustMsg[liveTurnStartedMsg](t, cmd)
	if started.Err != nil || started.RunID != "run_1" {
		t.Fatalf("started = %+v", started)
	}
	if live.Active().ID != "sess_fresh" {
		t.Fatalf("active = %+v", live.Active())
	}
	if !env.saw("session/create") {
		t.Fatal("expected a fresh session without --continue")
	}
}

func TestBootWithContinueAttachesNewest(t *testing.T) {
	env := &fakeEnv{script: baseScript()}
	live := NewLive(newClient(env), LiveOptions{InitialPrompt: "hi", ContinueNewest: true})
	defer live.Close()
	boot := mustMsg[liveBootMsg](t, live.bootCmd())
	if boot.Err != nil {
		t.Fatal(boot.Err)
	}
	cmd := live.Handle(boot)
	if cmd == nil {
		t.Fatal("expected auto-send cmd")
	}
	started := mustMsg[liveTurnStartedMsg](t, cmd)
	if started.Err != nil {
		t.Fatal(started.Err)
	}
	if live.Active().ID != "sess_1" {
		t.Fatalf("active = %+v", live.Active())
	}
	if env.saw("session/create") {
		t.Fatal("--continue must not create a session")
	}
}

func TestShutdownRunCancelsDanglingRun(t *testing.T) {
	script := baseScript()
	script["run/cancel"] = func(json.RawMessage) (any, error) {
		return map[string]string{"status": "cancelling"}, nil
	}
	script["run/get"] = func(json.RawMessage) (any, error) {
		return map[string]string{"status": "cancelled"}, nil
	}
	env := &fakeEnv{script: script}
	live := bootLive(t, env, LiveOptions{})
	live.mu.Lock()
	live.busy = true
	live.runID = "run_1"
	live.mu.Unlock()

	shutdownRun(context.Background(), newClient(env), live)
	if !env.saw("run/cancel") || !env.saw("run/get") {
		t.Fatalf("calls = %v", env.calls)
	}
}

func TestViewRendersShellWithoutDriver(t *testing.T) {
	wide := view.New(nil)
	updated, _ := wide.Update(tea.WindowSizeMsg{Width: 120, Height: 36})
	wideView := updated.(view.Model).View()
	if !strings.Contains(wideView, "VIVY CODE") || !strings.Contains(wideView, "Sessions") {
		t.Fatalf("missing sidebar skeleton in wide view:\n%s", wideView)
	}
	compact := view.New(nil)
	updated, _ = compact.Update(tea.WindowSizeMsg{Width: 64, Height: 24})
	compactView := updated.(view.Model).View()
	if !strings.Contains(compactView, "VIVY") {
		t.Fatalf("missing wordmark in compact view:\n%s", compactView)
	}
}

func TestLiveSessionMutationFailuresDoNotOptimisticallyChangeState(t *testing.T) {
	script := baseScript()
	script["session/rename"] = func(json.RawMessage) (any, error) { return nil, fmt.Errorf("temporary rename failure") }
	script["session/delete"] = func(json.RawMessage) (any, error) { return nil, fmt.Errorf("temporary delete failure") }
	env := &fakeEnv{script: script}
	live := bootLive(t, env, LiveOptions{})

	rename := mustMsg[surface.SessionsMsg](t, live.RenameSession("sess_1", "renamed"))
	if rename.Err == nil {
		t.Fatal("rename failure was swallowed")
	}
	live.Handle(rename)
	if got := live.Active().Title; got != "one" {
		t.Fatalf("failed rename changed title to %q", got)
	}

	deleted := mustMsg[surface.SessionsMsg](t, live.DeleteSession("sess_1"))
	if deleted.Err == nil {
		t.Fatal("delete failure was swallowed")
	}
	live.Handle(deleted)
	if got := live.Active().ID; got != "sess_1" {
		t.Fatalf("failed delete changed active session to %q", got)
	}
}

func TestLiveIgnoresOutOfOrderSessionLoads(t *testing.T) {
	live := &Live{messages: map[string][]surface.Message{}, loadRequest: 2}
	live.applyLoaded(liveLoadedMsg{Request: 2, Session: surface.Session{ID: "newest", Title: "Newest"}})
	live.applyLoaded(liveLoadedMsg{Request: 1, Session: surface.Session{ID: "stale", Title: "Stale"}})
	if got := live.Active().ID; got != "newest" {
		t.Fatalf("stale load overwrote newest selection: %q", got)
	}
}

func TestLiveBlocksSendWhileSessionLoadIsPending(t *testing.T) {
	live := &Live{messages: map[string][]surface.Message{}, activeID: "old", loadPending: true}
	if cmd := live.Send("must stay a draft"); cmd != nil {
		t.Fatal("send started while a session load was pending")
	}
	if live.busy || len(live.messages["old"]) != 0 || len(live.queue) != 0 {
		t.Fatalf("blocked send mutated live state: busy=%v messages=%d queue=%d", live.busy, len(live.messages["old"]), len(live.queue))
	}
}

func mustMsg[T any](t *testing.T, cmd tea.Cmd) T {
	t.Helper()
	if cmd == nil {
		t.Fatal("nil cmd")
	}
	msg := cmd()
	v, ok := msg.(T)
	if !ok {
		t.Fatalf("got %T %+v", msg, msg)
	}
	return v
}

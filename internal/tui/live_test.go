package tui

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"agent-vivy/internal/domain"
	controlrpc "agent-vivy/internal/rpc"
	"agent-vivy/internal/tui/surface"
)

func TestLiveBootListsOrCreatesSession(t *testing.T) {
	handler := controlrpc.HandlerFunc(func(_ context.Context, _ *controlrpc.Peer, request controlrpc.Request) (any, *controlrpc.Error) {
		switch request.Method {
		case "session/list":
			return map[string]any{"sessions": []map[string]string{}}, nil
		case "session/create":
			return map[string]string{"id": "sess_new", "title": "TUI", "permission_preset": "smart"}, nil
		default:
			return nil, &controlrpc.Error{Code: controlrpc.MethodNotFound, Message: request.Method}
		}
	})
	client, stop := attachTestClient(t, handler)
	defer stop()

	live := NewLive(client, LiveOptions{Host: "127.0.0.1:8787", Title: "TUI"})
	defer live.Close()

	cmd := live.Init()
	if cmd == nil {
		t.Fatal("expected init cmd")
	}
	// Init returns Batch(boot, tick); run until we see liveBootMsg.
	msg := runCmdUntil(t, cmd, 20, func(m tea.Msg) bool {
		_, ok := m.(liveBootMsg)
		return ok
	})
	boot, ok := msg.(liveBootMsg)
	if !ok {
		t.Fatalf("msg = %T %+v", msg, msg)
	}
	if boot.Err != nil {
		t.Fatal(boot.Err)
	}
	live.Handle(boot)
	if live.Active().ID != "sess_new" {
		t.Fatalf("active = %+v", live.Active())
	}
	if live.Meta().Mode != "live" || live.Meta().Host != "127.0.0.1:8787" {
		t.Fatalf("meta = %+v", live.Meta())
	}
}

func TestLiveTurnStreamsDeltaAndDone(t *testing.T) {
	var gotFace string
	handler := controlrpc.HandlerFunc(func(_ context.Context, _ *controlrpc.Peer, request controlrpc.Request) (any, *controlrpc.Error) {
		switch request.Method {
		case "session/list":
			return map[string]any{
				"sessions": []map[string]string{
					{"id": "sess_1", "title": "one", "permission_preset": "smart"},
				},
			}, nil
		case "session/messages":
			return map[string]any{"messages": []any{}}, nil
		case "turn/start":
			var params struct {
				Face string `json:"face"`
			}
			_ = json.Unmarshal(request.Params, &params)
			gotFace = params.Face
			return map[string]string{"run_id": "run_1", "status": "accepted"}, nil
		case "run/subscribe":
			return map[string]string{"subscription_id": "sub_1"}, nil
		default:
			return nil, &controlrpc.Error{Code: controlrpc.MethodNotFound, Message: request.Method}
		}
	})
	client, stop := attachTestClient(t, handler)
	defer stop()

	live := NewLive(client, LiveOptions{Host: "h", Title: "TUI"})
	defer live.Close()

	// Boot
	boot := mustMsg[liveBootMsg](t, live.bootCmd())
	if boot.Err != nil {
		t.Fatal(boot.Err)
	}
	live.Handle(boot)

	cmd := live.Send("hello")
	started := mustMsg[liveTurnStartedMsg](t, cmd)
	if started.Err != nil || started.RunID != "run_1" {
		t.Fatalf("started = %+v", started)
	}
	live.Handle(started)
	if gotFace != "code" {
		t.Fatalf("turn face = %q, want code", gotFace)
	}
	if !live.Meta().Busy {
		t.Fatal("expected busy")
	}
	msgs := live.ActiveMessages()
	if len(msgs) < 1 || msgs[0].Content != "hello" {
		t.Fatalf("messages after send = %+v", msgs)
	}

	// Inject stream events as the peer would.
	pushRunEvent(live, "run_1", "model.delta", map[string]string{"delta": "hi"})
	pushRunEvent(live, "run_1", "run.completed", map[string]any{})
	live.drainEvents()

	if live.Meta().Busy {
		t.Fatal("busy should clear")
	}
	var asst string
	for _, m := range live.ActiveMessages() {
		if m.Role == string(domain.RoleAssistant) {
			asst += m.Content
		}
	}
	if asst != "hi" {
		t.Fatalf("assistant = %q msgs=%+v", asst, live.ActiveMessages())
	}
}

func TestLivePermissionSwitchPersists(t *testing.T) {
	var gotPreset string
	handler := controlrpc.HandlerFunc(func(_ context.Context, _ *controlrpc.Peer, request controlrpc.Request) (any, *controlrpc.Error) {
		switch request.Method {
		case "session/list":
			return map[string]any{"sessions": []map[string]string{{"id": "sess_1", "title": "one", "permission_preset": "smart"}}}, nil
		case "session/messages":
			return map[string]any{"messages": []any{}}, nil
		case "session/set_permission":
			var params struct {
				Preset string `json:"preset"`
			}
			_ = json.Unmarshal(request.Params, &params)
			gotPreset = params.Preset
			return map[string]string{"id": "sess_1", "title": "one", "permission_preset": params.Preset}, nil
		default:
			return nil, &controlrpc.Error{Code: controlrpc.MethodNotFound, Message: request.Method}
		}
	})
	client, stop := attachTestClient(t, handler)
	defer stop()
	live := NewLive(client, LiveOptions{})
	defer live.Close()
	live.Handle(mustMsg[liveBootMsg](t, live.bootCmd()))
	msg := mustMsg[liveRPCMsg](t, live.SetPermission("trusted"))
	live.Handle(msg)
	if gotPreset != "trusted" || live.Active().PermissionPreset != "trusted" {
		t.Fatalf("permission = %q / %+v", gotPreset, live.Active())
	}
}

func TestLiveQueuesWhileBusyAndEscCanClear(t *testing.T) {
	live := &Live{
		messages: map[string][]surface.Message{"sess_1": nil},
		activeID: "sess_1",
		busy:     true,
		ctx:      context.Background(),
	}
	if cmd := live.Send("second task"); cmd == nil {
		t.Fatal("queued send should refresh the view")
	}
	if live.Meta().Queued != 1 {
		t.Fatalf("queued = %d", live.Meta().Queued)
	}
	if !live.ClearQueue() || live.Meta().Queued != 0 {
		t.Fatalf("queue did not clear: %+v", live.Meta())
	}
}

func TestLiveApprovalRespondsAndFiltersOtherRun(t *testing.T) {
	var gotDecision string
	handler := controlrpc.HandlerFunc(func(_ context.Context, _ *controlrpc.Peer, request controlrpc.Request) (any, *controlrpc.Error) {
		switch request.Method {
		case "session/list":
			return map[string]any{
				"sessions": []map[string]string{{"id": "sess_1", "title": "one", "permission_preset": "smart"}},
			}, nil
		case "session/messages":
			return map[string]any{"messages": []any{}}, nil
		case "approval/respond":
			var params struct {
				Decision string `json:"decision"`
			}
			_ = json.Unmarshal(request.Params, &params)
			gotDecision = params.Decision
			return map[string]any{"ok": true}, nil
		case "run/cancel":
			return map[string]any{"status": "cancelling"}, nil
		default:
			return nil, &controlrpc.Error{Code: controlrpc.MethodNotFound, Message: request.Method}
		}
	})
	client, stop := attachTestClient(t, handler)
	defer stop()

	live := NewLive(client, LiveOptions{})
	defer live.Close()
	boot := mustMsg[liveBootMsg](t, live.bootCmd())
	live.Handle(boot)

	live.mu.Lock()
	live.busy = true
	live.runID = "run_1"
	live.mu.Unlock()

	pushRunEvent(live, "run_other", "model.delta", map[string]string{"delta": "nope"})
	pushRunEvent(live, "run_1", "tool.approval_required", map[string]string{
		"approval_id": "appr_1", "tool_name": "write_file", "preview": "README.md",
	})
	live.drainEvents()

	gate := live.PendingGate()
	if gate == nil || gate.ID != "appr_1" {
		t.Fatalf("gate = %+v", gate)
	}
	// Other run delta must not appear.
	for _, m := range live.ActiveMessages() {
		if m.Content == "nope" {
			t.Fatal("leaked other run delta")
		}
	}

	cmd := live.DecideApproval(domain.ApprovalApproved)
	rpc := mustMsg[liveRPCMsg](t, cmd)
	if rpc.Err != nil {
		t.Fatal(rpc.Err)
	}
	live.Handle(rpc)
	if gotDecision != domain.ApprovalApproved {
		t.Fatalf("decision = %q", gotDecision)
	}
	if live.PendingGate() != nil {
		t.Fatal("gate should clear")
	}

	// Cancel path
	live.mu.Lock()
	live.busy = true
	live.runID = "run_1"
	live.mu.Unlock()
	cancelMsg := mustMsg[liveRPCMsg](t, live.Cancel())
	if cancelMsg.Err != nil {
		t.Fatal(cancelMsg.Err)
	}
}

func TestLiveMoveSessionLoadsMessages(t *testing.T) {
	handler := controlrpc.HandlerFunc(func(_ context.Context, _ *controlrpc.Peer, request controlrpc.Request) (any, *controlrpc.Error) {
		switch request.Method {
		case "session/list":
			return map[string]any{
				"sessions": []map[string]string{
					{"id": "sess_a", "title": "A", "permission_preset": "smart"},
					{"id": "sess_b", "title": "B", "permission_preset": "smart"},
				},
			}, nil
		case "session/messages":
			var params struct {
				SessionID string `json:"session_id"`
			}
			_ = json.Unmarshal(request.Params, &params)
			if params.SessionID == "sess_b" {
				return map[string]any{
					"messages": []map[string]string{
						{"id": "m1", "role": "user", "content": "from-b"},
					},
				}, nil
			}
			return map[string]any{"messages": []any{}}, nil
		default:
			return nil, &controlrpc.Error{Code: controlrpc.MethodNotFound, Message: request.Method}
		}
	})
	client, stop := attachTestClient(t, handler)
	defer stop()
	live := NewLive(client, LiveOptions{})
	defer live.Close()
	boot := mustMsg[liveBootMsg](t, live.bootCmd())
	live.Handle(boot)
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

func pushRunEvent(live *Live, runID, typ string, payload any) {
	rawPayload, _ := json.Marshal(payload)
	params, _ := json.Marshal(map[string]any{
		"subscription_id": "sub",
		"event": map[string]any{
			"run_id":  runID,
			"seq":     1,
			"type":    typ,
			"payload": json.RawMessage(rawPayload),
		},
	})
	_, _ = live.client.Handle(context.Background(), nil, controlrpc.Request{
		Method: "run/event",
		Params: params,
	})
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

func runCmdUntil(t *testing.T, cmd tea.Cmd, max int, want func(tea.Msg) bool) tea.Msg {
	t.Helper()
	var queue []tea.Cmd
	if cmd != nil {
		queue = append(queue, cmd)
	}
	for i := 0; i < max && len(queue) > 0; i++ {
		c := queue[0]
		queue = queue[1:]
		if c == nil {
			continue
		}
		msg := c()
		if want(msg) {
			return msg
		}
		// tea.Batch returns a message that is itself executed oddly;
		// for Batch, bubbletea flattens — here we only handle single cmds.
		if b, ok := msg.(tea.BatchMsg); ok {
			for _, sub := range b {
				queue = append(queue, sub)
			}
			continue
		}
		// tick messages: ignore and continue if more cmds
		if _, ok := msg.(liveTickMsg); ok {
			continue
		}
		_ = time.Now()
	}
	t.Fatal("wanted message not found")
	return nil
}

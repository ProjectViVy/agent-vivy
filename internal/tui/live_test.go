package tui

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
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
	pushRunEvent(live, "run_1", 1, "model.delta", map[string]string{"delta": "hi"})
	pushRunEvent(live, "run_1", 2, "run.completed", map[string]any{})
	_, _ = live.drainEvents()

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

func TestLiveKeepsReasoningContinuousAcrossEmptyDelta(t *testing.T) {
	live := &Live{
		messages:  map[string][]surface.Message{"sess_1": nil},
		activeID:  "sess_1",
		ctx:       context.Background(),
		eventWake: make(chan struct{}, 1),
	}
	live.applyNotice(eventNotice{Kind: "reasoning", Delta: "这"})
	live.applyNotice(eventNotice{Kind: "delta", Delta: ""})
	live.applyNotice(eventNotice{Kind: "reasoning", Delta: "是一句"})
	live.applyNotice(eventNotice{Kind: "delta", Delta: ""})
	live.applyNotice(eventNotice{Kind: "reasoning", Delta: "话🙂\n继续"})

	msgs := live.ActiveMessages()
	if len(msgs) != 1 || !msgs[0].Reasoning || msgs[0].Content != "这是一句话🙂\n继续" {
		t.Fatalf("reasoning messages = %+v", msgs)
	}

	live.applyNotice(eventNotice{Kind: "delta", Delta: "答案"})
	msgs = live.ActiveMessages()
	if len(msgs) != 2 || msgs[0].Streaming || !msgs[0].Reasoning || msgs[1].Reasoning || msgs[1].Content != "答案" {
		t.Fatalf("reasoning/answer boundary = %+v", msgs)
	}
}

func TestLiveEventInboxDoesNotDropBurst(t *testing.T) {
	live := &Live{
		messages:  map[string][]surface.Message{"sess_1": nil},
		activeID:  "sess_1",
		ctx:       context.Background(),
		eventWake: make(chan struct{}, 1),
	}
	for i := 0; i < 257; i++ {
		live.enqueueNotice(eventNotice{Kind: "reasoning", Delta: "界"})
	}
	_, _ = live.drainEvents()
	msgs := live.ActiveMessages()
	if len(msgs) != 1 {
		t.Fatalf("burst messages = %d, want 1", len(msgs))
	}
	if msgs[0].Content != strings.Repeat("界", 257) {
		t.Fatalf("burst runes = %d, want 257", len([]rune(msgs[0].Content)))
	}
}

func TestLiveDetectsSequenceGapAndSuppressesReplayDuplicates(t *testing.T) {
	live := &Live{
		messages:  map[string][]surface.Message{"sess_1": nil},
		activeID:  "sess_1",
		runID:     "run_1",
		busy:      true,
		ctx:       context.Background(),
		eventWake: make(chan struct{}, 1),
	}
	live.enqueueNotice(eventNotice{RunID: "run_1", Seq: 1}) // unrendered durable event
	live.enqueueNotice(eventNotice{RunID: "run_1", Seq: 3, Kind: "reasoning", Delta: "丢"})
	finished, gap := live.drainEvents()
	if finished || !gap || live.cursor.LastSeq != 1 || !live.cursor.Recovering {
		t.Fatalf("finished=%v gap=%v seq=%d recovering=%v", finished, gap, live.cursor.LastSeq, live.cursor.Recovering)
	}
	if msgs := live.ActiveMessages(); len(msgs) != 0 {
		t.Fatalf("gap event rendered before replay: %+v", msgs)
	}
	live.recoveryInFlight = true
	live.applySubscribed(liveSubscribedMsg{RunID: "run_1", AfterSeq: 0})
	if !live.recoveryInFlight {
		t.Fatal("delayed initial subscription result cleared recovery state")
	}
	live.applySubscribed(liveSubscribedMsg{RunID: "run_1", AfterSeq: 1, Recovery: true, Err: errors.New("temporary")})
	if !live.Meta().Busy || live.runID != "run_1" {
		t.Fatal("transient replay failure cancelled the active run")
	}
	_, gap = live.drainEvents()
	if !gap || live.inbox.Len() != 1 {
		t.Fatalf("failed replay lost retained gap: gap=%v pending=%d", gap, live.inbox.Len())
	}
	live.enqueueNotice(eventNotice{RunID: "run_1", Seq: 2, Kind: "reasoning", Delta: "补"})
	live.enqueueNotice(eventNotice{RunID: "run_1", Seq: 3, Kind: "reasoning", Delta: "丢"})
	_, gap = live.drainEvents()
	if gap || live.cursor.LastSeq != 3 {
		t.Fatalf("replay gap=%v seq=%d", gap, live.cursor.LastSeq)
	}
	if msgs := live.ActiveMessages(); len(msgs) != 1 || msgs[0].Content != "补丢" {
		t.Fatalf("retained replay messages = %+v", msgs)
	}
	if accept, missing := live.acceptSequence(eventNotice{RunID: "run_1", Seq: 3}); accept || missing {
		t.Fatalf("duplicate seq 3 accept=%v gap=%v", accept, missing)
	}
	live.applySubscribed(liveSubscribedMsg{RunID: "run_1", AfterSeq: 1, Err: errors.New("stale")})
	if !live.Meta().Busy || live.runID != "run_1" {
		t.Fatal("stale subscription failure cancelled an advanced run")
	}
}

func TestLiveRecoveryRPCReplaysRetainedGapOnce(t *testing.T) {
	gotAfter := -1
	handler := controlrpc.HandlerFunc(func(_ context.Context, _ *controlrpc.Peer, request controlrpc.Request) (any, *controlrpc.Error) {
		if request.Method != "run/subscribe" {
			return nil, &controlrpc.Error{Code: controlrpc.MethodNotFound, Message: request.Method}
		}
		var params struct {
			AfterSeq int `json:"after_seq"`
		}
		_ = json.Unmarshal(request.Params, &params)
		gotAfter = params.AfterSeq
		return map[string]string{"subscription_id": "sub_replay"}, nil
	})
	client, stop := attachTestClient(t, handler)
	defer stop()
	live := NewLive(client, LiveOptions{})
	defer live.Close()
	live.mu.Lock()
	live.activeID = "sess_1"
	live.runID = "run_1"
	live.busy = true
	live.mu.Unlock()

	live.enqueueNotice(eventNotice{RunID: "run_1", Seq: 1, Kind: "reasoning", Delta: "甲"})
	live.enqueueNotice(eventNotice{RunID: "run_1", Seq: 3, Kind: "reasoning", Delta: "丙"})
	_, gap := live.drainEvents()
	if !gap {
		t.Fatal("sequence gap was not detected")
	}
	subscribed := mustMsg[liveSubscribedMsg](t, live.recoverSubscriptionCmd())
	if subscribed.Err != nil || gotAfter != 1 || subscribed.AfterSeq != 1 {
		t.Fatalf("recovery subscription = %+v after_seq=%d", subscribed, gotAfter)
	}
	live.Handle(subscribed)
	if !live.replayPending || live.recoverSubscriptionCmd() != nil {
		t.Fatal("successful replay subscription was duplicated before replay arrived")
	}
	live.enqueueNotice(eventNotice{RunID: "run_1", Seq: 2, Kind: "reasoning", Delta: "乙"})
	live.enqueueNotice(eventNotice{RunID: "run_1", Seq: 3, Kind: "reasoning", Delta: "丙"})
	_, gap = live.drainEvents()
	if gap {
		t.Fatal("replayed contiguous events still reported a gap")
	}
	msgs := live.ActiveMessages()
	if len(msgs) != 1 || msgs[0].Content != "甲乙丙" {
		t.Fatalf("replayed reasoning = %+v", msgs)
	}
	if live.replayPending {
		t.Fatal("contiguous replay did not release the replay wait fence")
	}
}

func TestLiveRejectsLateReplayAfterTerminal(t *testing.T) {
	live := &Live{
		messages:  map[string][]surface.Message{"sess_1": nil},
		activeID:  "sess_1",
		runID:     "run_1",
		busy:      true,
		ctx:       context.Background(),
		eventWake: make(chan struct{}, 1),
	}
	live.enqueueNotice(eventNotice{RunID: "run_1", Seq: 1, Kind: "done", Done: true})
	finished, gap := live.drainEvents()
	if !finished || gap || live.Meta().Busy {
		t.Fatalf("terminal finished=%v gap=%v meta=%+v", finished, gap, live.Meta())
	}
	live.enqueueNotice(eventNotice{RunID: "run_1", Seq: 1, Kind: "delta", Delta: "late"})
	_, _ = live.drainEvents()
	if msgs := live.ActiveMessages(); len(msgs) != 0 {
		t.Fatalf("late replay polluted transcript: %+v", msgs)
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

func TestLiveGateFailuresRemainRetryable(t *testing.T) {
	handler := controlrpc.HandlerFunc(func(_ context.Context, _ *controlrpc.Peer, request controlrpc.Request) (any, *controlrpc.Error) {
		if request.Method == "approval/respond" || request.Method == "question/respond" {
			return nil, &controlrpc.Error{Code: controlrpc.InternalError, Message: "temporary"}
		}
		return nil, &controlrpc.Error{Code: controlrpc.MethodNotFound, Message: request.Method}
	})
	client, stop := attachTestClient(t, handler)
	defer stop()
	live := NewLive(client, LiveOptions{})
	defer live.Close()
	live.activeID = "sess_1"
	live.messages["sess_1"] = []surface.Message{{
		ID: "tool_1", Role: surface.RoleTool,
		Tool: &surface.ToolCard{ToolName: "write", Status: "pending", ApprovalID: "approval_1"},
	}}
	live.gate = &surface.Gate{Kind: "approval", ID: "approval_1"}
	cmd := live.DecideApproval(domain.ApprovalApproved)
	if cmd == nil || live.DecideApproval(domain.ApprovalApproved) != nil {
		t.Fatal("approval submit was missing or duplicate submit was accepted")
	}
	live.Handle(mustMsg[liveRPCMsg](t, cmd))
	if gate := live.PendingGate(); gate == nil || gate.Submitting {
		t.Fatalf("approval gate not retryable: %+v", gate)
	}
	if tool := live.ActiveMessages()[0].Tool; tool.Status != "pending" {
		t.Fatalf("failed approval mutated tool: %+v", tool)
	}

	live.gate = &surface.Gate{Kind: "question", ID: "question_1"}
	cmd = live.AnswerQuestion("answer")
	if cmd == nil || live.AnswerQuestion("again") != nil {
		t.Fatal("question submit was missing or duplicate submit was accepted")
	}
	live.Handle(mustMsg[liveRPCMsg](t, cmd))
	if gate := live.PendingGate(); gate == nil || gate.Submitting {
		t.Fatalf("question gate not retryable: %+v", gate)
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

	pushRunEvent(live, "run_other", 1, "model.delta", map[string]string{"delta": "nope"})
	pushRunEvent(live, "run_1", 1, "tool.approval_required", map[string]string{
		"approval_id": "appr_1", "tool_name": "write_file", "preview": "README.md",
	})
	_, _ = live.drainEvents()

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

func pushRunEvent(live *Live, runID string, seq int, typ string, payload any) {
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

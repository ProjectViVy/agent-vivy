package tui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"agent-vivy/internal/domain"
	controlrpc "agent-vivy/internal/rpc"
	"agent-vivy/internal/tui/surface"
	"agent-vivy/sdk/tui/stream"
)

func TestMapHistoryMergesDurableShellToolPair(t *testing.T) {
	messages := mapHistory([]messageView{
		{ID: "result", Role: "tool", ToolName: "bash", ToolCallID: "call_shell", Content: "safe output"},
		{ID: "other", Role: "assistant", Content: "interleaved"},
		{ID: "request", Role: "assistant", ToolName: "bash", ToolCallID: "call_shell", ToolPreview: "bash script [redacted bytes=7 sha256=abc]"},
	})
	if len(messages) != 2 || messages[0].Tool == nil || messages[0].Tool.ToolCallID != "call_shell" || messages[0].Tool.Status != "done" ||
		messages[0].Tool.Preview == "" || messages[0].Tool.Result != "safe output" {
		t.Fatalf("shell history = %+v", messages)
	}
}

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
	var gotFace, gotText string
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
				Text string `json:"text"`
			}
			_ = json.Unmarshal(request.Params, &params)
			gotFace = params.Face
			gotText = params.Text
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

	cmd := live.Send("  hello  ")
	started := mustMsg[liveTurnStartedMsg](t, cmd)
	if started.Err != nil || started.RunID != "run_1" {
		t.Fatalf("started = %+v", started)
	}
	live.Handle(started)
	if gotFace != "code" {
		t.Fatalf("turn face = %q, want code", gotFace)
	}
	if gotText != "  hello  " {
		t.Fatalf("turn text lost whitespace: %q", gotText)
	}
	if !live.Meta().Busy {
		t.Fatal("expected busy")
	}
	msgs := live.ActiveMessages()
	if len(msgs) < 1 || msgs[0].Content != "  hello  " {
		t.Fatalf("messages after send = %+v", msgs)
	}

	// Inject stream events as the peer would.
	pushRunEvent(live, "run_1", 1, "model.delta", map[string]string{"delta": "hi"})
	pushRunEvent(live, "run_1", 2, "run.completed", map[string]any{})
	for live.inbox.Len() > 0 {
		_, _ = live.drainEvents()
	}

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

func TestLiveNewSessionLoadsThinkingCapability(t *testing.T) {
	handler := controlrpc.HandlerFunc(func(_ context.Context, _ *controlrpc.Peer, request controlrpc.Request) (any, *controlrpc.Error) {
		switch request.Method {
		case "session/create":
			return map[string]string{"id": "sess_new", "title": "new", "permission_preset": "smart"}, nil
		case "session/context":
			return map[string]any{"thinking_supported": true, "feed_tokens": 0}, nil
		default:
			return nil, &controlrpc.Error{Code: controlrpc.MethodNotFound, Message: request.Method}
		}
	})
	client, stop := attachTestClient(t, handler)
	defer stop()
	live := NewLive(client, LiveOptions{})
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

func TestLiveWireProjectsAuthoritativeCompletionAndToolCallIdentity(t *testing.T) {
	handler := controlrpc.HandlerFunc(func(context.Context, *controlrpc.Peer, controlrpc.Request) (any, *controlrpc.Error) {
		return nil, &controlrpc.Error{Code: controlrpc.MethodNotFound, Message: "unused"}
	})
	client, stop := attachTestClient(t, handler)
	defer stop()
	live := NewLive(client, LiveOptions{})
	defer live.Close()
	live.mu.Lock()
	live.activeID = "sess_1"
	live.messages = map[string][]surface.Message{"sess_1": nil}
	live.runID = "run_1"
	live.busy = true
	live.mu.Unlock()

	pushRunEvent(live, "run_1", 1, "model.completed", map[string]string{"content": "completed only"})
	pushRunEvent(live, "run_1", 2, "tool.requested", map[string]any{"tool_call_id": "call_1", "tool_name": "read_file", "args": map[string]string{"path": "a"}})
	pushRunEvent(live, "run_1", 3, "tool.requested", map[string]any{"tool_call_id": "call_2", "tool_name": "read_file", "args": map[string]string{"path": "b"}})
	pushRunEvent(live, "run_1", 4, "tool.finished", map[string]string{"tool_call_id": "call_1", "tool_name": "read_file", "result": "a done"})
	for live.inbox.Len() > 0 {
		_, _ = live.drainEvents()
	}
	msgs := live.ActiveMessages()
	if len(msgs) != 3 || msgs[0].Content != "completed only" || msgs[1].Tool == nil || msgs[1].Tool.Status != "done" || msgs[2].Tool == nil || msgs[2].Tool.Status != "pending" {
		t.Fatalf("wire projection = %+v", msgs)
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
	for live.inbox.Len() > 0 {
		_, _ = live.drainEvents()
	}
	msgs := live.ActiveMessages()
	if len(msgs) != 1 {
		t.Fatalf("burst messages = %d, want 1", len(msgs))
	}
	if msgs[0].Content != strings.Repeat("界", 257) {
		t.Fatalf("burst runes = %d, want 257", len([]rune(msgs[0].Content)))
	}
}

func TestLiveInboxOverflowRequestsReplayFromLastAppliedSequence(t *testing.T) {
	live := &Live{
		messages:  map[string][]surface.Message{"sess_1": nil},
		activeID:  "sess_1",
		runID:     "run_1",
		busy:      true,
		inbox:     stream.NewInbox(2),
		ctx:       context.Background(),
		eventWake: make(chan struct{}, 1),
	}
	for seq := 1; seq <= 3; seq++ {
		live.enqueueNotice(eventNotice{RunID: "run_1", Seq: seq, Kind: "reasoning", Delta: fmt.Sprint(seq)})
	}
	finished, replay := live.drainEvents()
	if finished || !replay || live.cursor.LastSeq != 0 || live.inbox.Len() != 0 {
		t.Fatalf("finished=%v replay=%v seq=%d pending=%d", finished, replay, live.cursor.LastSeq, live.inbox.Len())
	}
	if msgs := live.ActiveMessages(); len(msgs) != 0 {
		t.Fatalf("bounded prefix = %+v", msgs)
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
	live.nextRecoveryAt = time.Now().Add(-time.Second)
	if live.recoverSubscriptionCmd() != nil {
		t.Fatal("empty successful replay scheduled a duplicate subscription")
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

func TestLiveNewGapBreaksSuccessfulReplayFence(t *testing.T) {
	live := &Live{
		messages:  map[string][]surface.Message{"sess_1": nil},
		activeID:  "sess_1",
		runID:     "run_1",
		busy:      true,
		ctx:       context.Background(),
		eventWake: make(chan struct{}, 1),
	}
	live.cursor.LastSeq = 1
	live.recoveryInFlight = true
	live.recoveryNeeded = true
	live.applySubscribed(liveSubscribedMsg{RunID: "run_1", AfterSeq: 1, Recovery: true})
	live.enqueueNotice(eventNotice{RunID: "run_1", Seq: 3})
	_, gap := live.drainEvents()
	if !gap || live.replayPending || live.recoverSubscriptionCmd() == nil {
		t.Fatalf("new gap did not reopen recovery: gap=%v replay=%v needed=%v", gap, live.replayPending, live.recoveryNeeded)
	}
}

func TestLiveRecoveryReplacesAndUnsubscribesPreviousStream(t *testing.T) {
	unsubscribed := ""
	handler := controlrpc.HandlerFunc(func(_ context.Context, _ *controlrpc.Peer, request controlrpc.Request) (any, *controlrpc.Error) {
		switch request.Method {
		case "run/subscribe":
			return map[string]string{"subscription_id": "sub_new"}, nil
		case "run/unsubscribe":
			var params struct {
				SubscriptionID string `json:"subscription_id"`
			}
			_ = json.Unmarshal(request.Params, &params)
			unsubscribed = params.SubscriptionID
			return map[string]bool{"unsubscribed": true}, nil
		default:
			return nil, &controlrpc.Error{Code: controlrpc.MethodNotFound, Message: request.Method}
		}
	})
	client, stop := attachTestClient(t, handler)
	defer stop()
	live := NewLive(client, LiveOptions{})
	live.mu.Lock()
	live.runID = "run_1"
	live.busy = true
	live.subscriptionID = "sub_old"
	live.cursor.LastSeq = 4
	live.mu.Unlock()
	msg := mustMsg[liveSubscribedMsg](t, live.subscribeCmd("run_1", 4, true))
	cmd := live.applySubscribed(msg)
	if cmd == nil {
		t.Fatal("replacement did not schedule old subscription cleanup")
	}
	_ = cmd()
	if unsubscribed != "sub_old" || live.subscriptionID != "sub_new" {
		t.Fatalf("unsubscribed=%q current=%q", unsubscribed, live.subscriptionID)
	}
	live.applyNotice(eventNotice{RunID: "run_1", Seq: 5, Kind: "done", Done: true})
	if live.subscriptionID != "" {
		t.Fatalf("terminal retained subscription id %q", live.subscriptionID)
	}
	live.Close()
}

func TestLiveCloseUnsubscribesActiveStream(t *testing.T) {
	unsubscribed := ""
	handler := controlrpc.HandlerFunc(func(_ context.Context, _ *controlrpc.Peer, request controlrpc.Request) (any, *controlrpc.Error) {
		if request.Method != "run/unsubscribe" {
			return nil, &controlrpc.Error{Code: controlrpc.MethodNotFound, Message: request.Method}
		}
		var params struct {
			SubscriptionID string `json:"subscription_id"`
		}
		_ = json.Unmarshal(request.Params, &params)
		unsubscribed = params.SubscriptionID
		return map[string]bool{"unsubscribed": true}, nil
	})
	client, stop := attachTestClient(t, handler)
	defer stop()
	live := NewLive(client, LiveOptions{})
	live.subscriptionID = "sub_close"
	live.Close()
	if unsubscribed != "sub_close" {
		t.Fatalf("close unsubscribed %q", unsubscribed)
	}
}

func TestLiveStreamErrorRecoversAndRejectsRetiredTraffic(t *testing.T) {
	live := &Live{
		messages:       map[string][]surface.Message{"sess_1": nil},
		activeID:       "sess_1",
		runID:          "run_1",
		busy:           true,
		subscriptionID: "sub_old",
		ctx:            context.Background(),
		eventWake:      make(chan struct{}, 1),
	}
	live.recordStreamError(stream.StreamError{SubscriptionID: "sub_old", Message: "replay failed"})
	if !live.recoveryNeeded || live.subscriptionID != "" || !live.Meta().Busy {
		t.Fatalf("stream failure state recovery=%v subscription=%q meta=%+v", live.recoveryNeeded, live.subscriptionID, live.Meta())
	}
	live.enqueueNotice(eventNotice{SubscriptionID: "sub_old", RunID: "run_1", Seq: 1, Kind: "delta", Delta: "stale"})
	live.enqueueNotice(eventNotice{SubscriptionID: "sub_new", RunID: "run_1", Seq: 1, Kind: "delta", Delta: "fresh"})
	_, _ = live.drainEvents()
	if msgs := live.ActiveMessages(); len(msgs) != 1 || msgs[0].Content != "fresh" {
		t.Fatalf("retired stream polluted transcript: %+v", msgs)
	}
	pending := &Live{runID: "run_2", busy: true, recoveryInFlight: true}
	pending.recordStreamError(stream.StreamError{SubscriptionID: "sub_pending", Message: "early failure"})
	pending.applySubscribed(liveSubscribedMsg{RunID: "run_2", SubscriptionID: "sub_pending", Recovery: true})
	if pending.subscriptionID != "" || !pending.recoveryNeeded || pending.replayPending || pending.recoveryInFlight {
		t.Fatalf("early stream error state subscription=%q recovery=%v replay=%v in_flight=%v", pending.subscriptionID, pending.recoveryNeeded, pending.replayPending, pending.recoveryInFlight)
	}
}

func TestLiveLateSubscriptionAfterCloseIsCleanedUp(t *testing.T) {
	unsubscribed := ""
	handler := controlrpc.HandlerFunc(func(_ context.Context, _ *controlrpc.Peer, request controlrpc.Request) (any, *controlrpc.Error) {
		if request.Method != "run/unsubscribe" {
			return nil, &controlrpc.Error{Code: controlrpc.MethodNotFound, Message: request.Method}
		}
		var params struct {
			SubscriptionID string `json:"subscription_id"`
		}
		_ = json.Unmarshal(request.Params, &params)
		unsubscribed = params.SubscriptionID
		return map[string]bool{"unsubscribed": true}, nil
	})
	client, stop := attachTestClient(t, handler)
	defer stop()
	live := NewLive(client, LiveOptions{})
	live.runID = "run_1"
	live.Close()
	cmd := live.applySubscribed(liveSubscribedMsg{RunID: "run_1", SubscriptionID: "sub_late"})
	if cmd == nil {
		t.Fatal("late subscription was not scheduled for cleanup")
	}
	_ = cmd()
	if unsubscribed != "sub_late" {
		t.Fatalf("late subscription cleanup = %q", unsubscribed)
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

func TestLiveThinkingModeIsSentAndQueuedTurnsSnapshotIt(t *testing.T) {
	var got []string
	handler := controlrpc.HandlerFunc(func(_ context.Context, _ *controlrpc.Peer, request controlrpc.Request) (any, *controlrpc.Error) {
		if request.Method != "turn/start" {
			return nil, &controlrpc.Error{Code: controlrpc.MethodNotFound, Message: request.Method}
		}
		var params struct {
			Thinking string `json:"thinking"`
		}
		_ = json.Unmarshal(request.Params, &params)
		got = append(got, params.Thinking)
		return map[string]string{"run_id": fmt.Sprintf("run_%d", len(got)), "status": "accepted"}, nil
	})
	client, stop := attachTestClient(t, handler)
	defer stop()
	live := NewLive(client, LiveOptions{})
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

func TestLiveAdvancedCommandsUseAuthoritativeRPCAndOverlayResult(t *testing.T) {
	var methods []string
	handler := controlrpc.HandlerFunc(func(_ context.Context, _ *controlrpc.Peer, request controlrpc.Request) (any, *controlrpc.Error) {
		methods = append(methods, request.Method)
		switch request.Method {
		case "context/compact":
			return map[string]any{"before_tokens": 20, "after_tokens": 8}, nil
		case "session/todos":
			return map[string]any{"todos": []any{map[string]any{"subject": "ship", "status": "pending"}}}, nil
		case "stats/tokens":
			return map[string]any{"period": "1w", "total": map[string]any{"total_tokens": 42, "cost_known": false}}, nil
		case "skills/list":
			return map[string]any{"skills": []any{map[string]any{"name": "writer", "enabled": true}}}, nil
		case "skills/get":
			return map[string]any{"name": "writer", "content": "guide"}, nil
		case "settings/mcp":
			return map[string]any{"servers": []any{map[string]any{"name": "docs", "status": "idle"}}}, nil
		case "settings/mcp/probe":
			return map[string]any{"name": "docs", "status": "ok", "tool_count": 1}, nil
		case "settings/mcp/resources":
			return map[string]any{"server": "docs", "resources": []any{map[string]any{"uri": "docs://guide", "name": "guide"}}, "untrusted": true}, nil
		case "settings/mcp/read":
			return map[string]any{"server": "docs", "uri": "docs://guide", "contents": []any{map[string]any{"uri": "docs://guide", "text": "hello"}}, "untrusted": true}, nil
		case "tools/list":
			return map[string]any{"active": []string{"read_file"}}, nil
		case "workspace/list":
			return map[string]any{"files": []any{map[string]any{"path": "README.md"}}}, nil
		case "workspace/read":
			return map[string]any{"path": "README.md", "content": "hello"}, nil
		default:
			return nil, &controlrpc.Error{Code: controlrpc.MethodNotFound, Message: request.Method}
		}
	})
	client, stop := attachTestClient(t, handler)
	defer stop()
	live := NewLive(client, LiveOptions{})
	defer live.Close()
	live.mu.Lock()
	live.activeID = "sess_1"
	live.sessions = []surface.Session{{ID: "sess_1", Title: "one"}}
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
		if methods[len(methods)-1] != tc.method {
			t.Fatalf("/%s called %s, want %s", tc.name, methods[len(methods)-1], tc.method)
		}
	}
}

func TestLiveAdvancedCommandValidationAndScopedFilesFailClosed(t *testing.T) {
	client, stop := attachTestClient(t, controlrpc.HandlerFunc(func(_ context.Context, _ *controlrpc.Peer, request controlrpc.Request) (any, *controlrpc.Error) {
		return nil, &controlrpc.Error{Code: controlrpc.MethodNotFound, Message: request.Method}
	}))
	defer stop()
	live := NewLive(client, LiveOptions{})
	defer live.Close()
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
	handler := controlrpc.HandlerFunc(func(_ context.Context, _ *controlrpc.Peer, request controlrpc.Request) (any, *controlrpc.Error) {
		switch request.Method {
		case "session/fork":
			return map[string]any{"session_id": "sess_fork", "fork_point_message_id": "msg_1", "copied_count": 1}, nil
		case "session/get":
			return map[string]any{"session": map[string]any{"id": "sess_fork", "title": "branch", "permission_preset": "smart"}}, nil
		case "session/messages":
			return map[string]any{"messages": []any{map[string]any{"id": "msg_1", "role": "user", "content": "copied"}}}, nil
		case "session/context":
			return map[string]any{"feed_tokens": 1, "model_limit_tokens": 100}, nil
		default:
			return nil, &controlrpc.Error{Code: controlrpc.MethodNotFound, Message: request.Method}
		}
	})
	client, stop := attachTestClient(t, handler)
	defer stop()
	live := NewLive(client, LiveOptions{})
	defer live.Close()
	live.mu.Lock()
	live.activeID = "sess_1"
	live.sessions = []surface.Session{{ID: "sess_1", Title: "one"}}
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

func TestLiveMutationCommandInFlightAndRefreshGuard(t *testing.T) {
	client, stop := attachTestClient(t, controlrpc.HandlerFunc(func(_ context.Context, _ *controlrpc.Peer, request controlrpc.Request) (any, *controlrpc.Error) {
		if request.Method == "context/compact" {
			return map[string]any{"before_tokens": 10, "after_tokens": 4, "folded_messages": 1, "skipped": false}, nil
		}
		return nil, &controlrpc.Error{Code: controlrpc.MethodNotFound, Message: request.Method}
	}))
	defer stop()
	live := NewLive(client, LiveOptions{})
	defer live.Close()
	live.mu.Lock()
	live.activeID = "sess_1"
	live.commandInFlight = true
	live.mu.Unlock()
	blocked := mustMsg[surface.CommandResultMsg](t, live.ExecuteCommand("compact", nil))
	if blocked.Err == nil || !strings.Contains(blocked.Err.Error(), "run or gate") {
		t.Fatalf("duplicate mutation was not rejected: %+v", blocked)
	}
	live.mu.Lock()
	live.commandInFlight = false
	live.loadPending = true
	live.mu.Unlock()
	blocked = mustMsg[surface.CommandResultMsg](t, live.ExecuteCommand("rewind", []string{"msg_1"}))
	if blocked.Err == nil || !strings.Contains(blocked.Err.Error(), "run or gate") {
		t.Fatalf("load-pending mutation was not rejected: %+v", blocked)
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

func TestLiveSessionMutationFailuresDoNotOptimisticallyChangeState(t *testing.T) {
	handler := controlrpc.HandlerFunc(func(_ context.Context, _ *controlrpc.Peer, request controlrpc.Request) (any, *controlrpc.Error) {
		switch request.Method {
		case "session/list":
			return map[string]any{"sessions": []map[string]string{{"id": "sess_1", "title": "original", "permission_preset": "smart"}}}, nil
		case "session/messages":
			return map[string]any{"messages": []any{}}, nil
		case "session/rename", "session/delete":
			return nil, &controlrpc.Error{Code: controlrpc.CodeConflict, Message: "temporary failure"}
		default:
			return nil, &controlrpc.Error{Code: controlrpc.MethodNotFound, Message: request.Method}
		}
	})
	client, stop := attachTestClient(t, handler)
	defer stop()
	live := NewLive(client, LiveOptions{})
	defer live.Close()
	boot := mustMsg[liveBootMsg](t, live.bootCmd())
	if boot.Err != nil {
		t.Fatal(boot.Err)
	}
	live.Handle(boot)

	rename := mustMsg[surface.SessionsMsg](t, live.RenameSession("sess_1", "renamed"))
	if rename.Err == nil {
		t.Fatal("rename failure was swallowed")
	}
	live.Handle(rename)
	if got := live.Active().Title; got != "original" {
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

func TestLiveImageCommandGatesUnknownAndUnsupportedCapabilities(t *testing.T) {
	var resolves int
	handler := controlrpc.HandlerFunc(func(_ context.Context, _ *controlrpc.Peer, request controlrpc.Request) (any, *controlrpc.Error) {
		if request.Method == "attachments/resolve" {
			resolves++
			return map[string]any{"attachments": []map[string]any{{"path": "photo.png", "name": "photo.png", "mime_type": "image/png", "size": 8}}}, nil
		}
		return nil, &controlrpc.Error{Code: controlrpc.MethodNotFound, Message: request.Method}
	})
	client, stop := attachTestClient(t, handler)
	defer stop()
	live := NewLive(client, LiveOptions{})
	defer live.Close()
	live.mu.Lock()
	live.activeID = "sess_1"
	live.messages = map[string][]surface.Message{"sess_1": nil}
	live.sidebar = surface.Sidebar{HasContext: true, Context: surface.Context{ImageSupportKnown: false}}
	live.mu.Unlock()
	unknown := mustMsg[surface.CommandResultMsg](t, live.ExecuteCommand("image", []string{"photo.png"}))
	if unknown.Err == nil || !strings.Contains(unknown.Err.Error(), "known") {
		t.Fatalf("unknown image capability result = %+v", unknown)
	}
	live.mu.Lock()
	live.sidebar.Context.ImageSupportKnown = true
	live.sidebar.Context.ImageSupported = false
	live.mu.Unlock()
	unsupported := mustMsg[surface.CommandResultMsg](t, live.ExecuteCommand("image", []string{"photo.png"}))
	if unsupported.Err == nil || !strings.Contains(unsupported.Err.Error(), "does not support") {
		t.Fatalf("unsupported image capability result = %+v", unsupported)
	}
	if resolves != 0 {
		t.Fatalf("resolver called despite capability gate: %d", resolves)
	}
}

func TestLiveImageDraftResolvesRendersAndSendsPathOnly(t *testing.T) {
	var gotPaths []string
	handler := controlrpc.HandlerFunc(func(_ context.Context, _ *controlrpc.Peer, request controlrpc.Request) (any, *controlrpc.Error) {
		switch request.Method {
		case "attachments/resolve":
			return map[string]any{"attachments": []map[string]any{{"path": "assets/photo.png", "name": "photo.png", "mime_type": "image/png", "size": 8}}}, nil
		case "turn/start":
			var params struct {
				AttachmentPaths []string `json:"attachment_paths"`
			}
			_ = json.Unmarshal(request.Params, &params)
			gotPaths = append([]string(nil), params.AttachmentPaths...)
			return map[string]string{"run_id": "run_image", "status": "accepted"}, nil
		case "run/subscribe":
			return map[string]string{"subscription_id": "sub_image"}, nil
		default:
			return nil, &controlrpc.Error{Code: controlrpc.MethodNotFound, Message: request.Method}
		}
	})
	client, stop := attachTestClient(t, handler)
	defer stop()
	live := NewLive(client, LiveOptions{})
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
		t.Fatalf("pending attachment = %+v", pending)
	}
	cmd := live.Send("describe")
	started := mustMsg[liveTurnStartedMsg](t, cmd)
	if started.Err != nil || started.RunID != "run_image" {
		t.Fatalf("turn started = %+v", started)
	}
	if len(live.PendingAttachments()) != 0 {
		t.Fatal("successful send retained pending image draft")
	}
	live.Handle(started)
	if len(gotPaths) != 1 || gotPaths[0] != "assets/photo.png" {
		t.Fatalf("turn/start paths = %v", gotPaths)
	}
	messages := live.ActiveMessages()
	if len(messages) == 0 || len(messages[0].Attachments) != 1 || messages[0].Attachments[0].Name != "photo.png" {
		t.Fatalf("optimistic history attachment = %+v", messages)
	}
}

func TestLiveImageSendFailureRetainsDraftAndQueueIsSessionBound(t *testing.T) {
	var turnStarts int
	handler := controlrpc.HandlerFunc(func(_ context.Context, _ *controlrpc.Peer, request controlrpc.Request) (any, *controlrpc.Error) {
		if request.Method == "turn/start" {
			turnStarts++
			return nil, &controlrpc.Error{Code: controlrpc.InvalidParams, Message: "rejected"}
		}
		return nil, &controlrpc.Error{Code: controlrpc.MethodNotFound, Message: request.Method}
	})
	client, stop := attachTestClient(t, handler)
	defer stop()
	live := NewLive(client, LiveOptions{})
	defer live.Close()
	live.mu.Lock()
	live.activeID = "sess_1"
	live.messages = map[string][]surface.Message{"sess_1": nil}
	live.drafts = map[string][]surface.Attachment{"sess_1": {{Path: "photo.png", Name: "photo.png", MimeType: "image/png", Size: 8}}}
	live.mu.Unlock()
	started := mustMsg[liveTurnStartedMsg](t, live.Send("try"))
	if started.Err == nil {
		t.Fatal("rejected turn unexpectedly succeeded")
	}
	live.Handle(started)
	if pending := live.PendingAttachments(); len(pending) != 1 || pending[0].Name != "photo.png" {
		t.Fatalf("failed send did not retain draft: %+v", pending)
	}

	live.mu.Lock()
	live.busy = true
	live.drafts["sess_1"] = []surface.Attachment{{Path: "queued.png", Name: "queued.png", MimeType: "image/png"}}
	live.mu.Unlock()
	_ = live.Send("queued")
	live.mu.Lock()
	if len(live.queue) != 1 || live.queue[0].SessionID != "sess_1" || len(live.queue[0].Attachments) != 1 {
		t.Fatalf("queued image snapshot = %+v", live.queue)
	}
	live.activeID = "sess_2"
	live.messages["sess_2"] = nil
	live.busy = false
	live.mu.Unlock()
	_ = live.dequeueCmd()
	if turnStarts != 1 {
		t.Fatalf("session-switched queue was sent: turn starts=%d", turnStarts)
	}
	live.mu.Lock()
	deferred := append([]surface.Attachment(nil), live.drafts["sess_1"]...)
	queued := append([]queuedTurn(nil), live.queue...)
	live.mu.Unlock()
	if len(deferred) != 0 || len(queued) != 1 || queued[0].Text != "queued" || len(queued[0].Attachments) != 1 || queued[0].Attachments[0].Name != "queued.png" {
		t.Fatalf("session-bound queue snapshot lost: drafts=%+v queue=%+v", deferred, queued)
	}
}

func TestLiveQueuedImageSendPreservesLaterDraft(t *testing.T) {
	var gotPaths []string
	handler := controlrpc.HandlerFunc(func(_ context.Context, _ *controlrpc.Peer, request controlrpc.Request) (any, *controlrpc.Error) {
		if request.Method != "turn/start" {
			return nil, &controlrpc.Error{Code: controlrpc.MethodNotFound, Message: request.Method}
		}
		var params struct {
			AttachmentPaths []string `json:"attachment_paths"`
		}
		_ = json.Unmarshal(request.Params, &params)
		gotPaths = append([]string(nil), params.AttachmentPaths...)
		return map[string]string{"run_id": "run_queued_image", "status": "accepted"}, nil
	})
	client, stop := attachTestClient(t, handler)
	defer stop()
	live := NewLive(client, LiveOptions{})
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

func TestLiveFileContextResolvesBeforeTurnAndStoresMetadataOnly(t *testing.T) {
	var methods []string
	var gotPaths []string
	var gotText string
	handler := controlrpc.HandlerFunc(func(_ context.Context, _ *controlrpc.Peer, request controlrpc.Request) (any, *controlrpc.Error) {
		methods = append(methods, request.Method)
		switch request.Method {
		case "project-context/resolve":
			var params struct {
				Paths []string `json:"paths"`
			}
			_ = json.Unmarshal(request.Params, &params)
			gotPaths = append([]string(nil), params.Paths...)
			return map[string]any{"contexts": []map[string]any{{"path": "README.md", "name": "README.md", "size": 42}}}, nil
		case "turn/start":
			var params struct {
				Text         string   `json:"text"`
				ContextPaths []string `json:"context_paths"`
			}
			_ = json.Unmarshal(request.Params, &params)
			gotText = params.Text
			gotPaths = append(gotPaths, params.ContextPaths...)
			return map[string]string{"run_id": "run_file", "status": "accepted"}, nil
		case "run/subscribe":
			return map[string]string{"subscription_id": "sub_file"}, nil
		default:
			return nil, &controlrpc.Error{Code: controlrpc.MethodNotFound, Message: request.Method}
		}
	})
	client, stop := attachTestClient(t, handler)
	defer stop()
	live := NewLive(client, LiveOptions{})
	defer live.Close()
	live.mu.Lock()
	live.activeID = "sess_1"
	live.messages = map[string][]surface.Message{"sess_1": nil}
	live.mu.Unlock()
	started := mustMsg[liveTurnStartedMsg](t, live.SendWithContext("inspect files", []string{"README.md"}))
	if started.Err != nil || started.RunID != "run_file" {
		t.Fatalf("file turn = %+v", started)
	}
	if len(methods) != 2 || methods[0] != "project-context/resolve" || methods[1] != "turn/start" {
		t.Fatalf("file call order = %v", methods)
	}
	if gotText != "inspect files" || strings.Join(gotPaths, "|") != "README.md|README.md" {
		t.Fatalf("file request text=%q paths=%v", gotText, gotPaths)
	}
	live.Handle(started)
	messages := live.ActiveMessages()
	if len(messages) < 1 || len(messages[0].FileContexts) != 1 || messages[0].FileContexts[0].Size != 42 {
		t.Fatalf("file metadata not retained: %+v", messages)
	}
	if messages[0].FileContexts[0].Name != "README.md" || messages[0].Content != "inspect files" {
		t.Fatalf("file metadata/content = %+v", messages[0])
	}
}

func TestLiveShellUsesOnlyShellStartAndPreservesScript(t *testing.T) {
	var methods []string
	var got map[string]any
	handler := controlrpc.HandlerFunc(func(_ context.Context, _ *controlrpc.Peer, request controlrpc.Request) (any, *controlrpc.Error) {
		methods = append(methods, request.Method)
		switch request.Method {
		case "shell/start":
			if err := json.Unmarshal(request.Params, &got); err != nil {
				return nil, &controlrpc.Error{Code: controlrpc.InvalidParams, Message: err.Error()}
			}
			return map[string]string{"run_id": "run_shell", "status": "accepted"}, nil
		case "run/subscribe":
			return map[string]string{"subscription_id": "sub_shell"}, nil
		default:
			return nil, &controlrpc.Error{Code: controlrpc.MethodNotFound, Message: request.Method}
		}
	})
	client, stop := attachTestClient(t, handler)
	defer stop()
	if err := client.setCapabilities(json.RawMessage(`{"capabilities":["shell.start"]}`)); err != nil {
		t.Fatal(err)
	}
	live := NewLive(client, LiveOptions{})
	defer live.Close()
	live.mu.Lock()
	live.activeID = "sess_1"
	live.messages = map[string][]surface.Message{"sess_1": nil}
	live.mu.Unlock()
	script := "  printf 'safe' && printf done  "
	started := mustMsg[liveTurnStartedMsg](t, live.ExecuteShell(script))
	if started.Err != nil || started.RunID != "run_shell" || !started.Shell {
		t.Fatalf("shell turn = %+v", started)
	}
	if len(methods) != 1 || methods[0] != "shell/start" {
		t.Fatalf("shell bypassed route: %v", methods)
	}
	if len(got) != 2 || got["session_id"] != "sess_1" || got["script"] != script {
		t.Fatalf("shell params = %#v", got)
	}
	live.Handle(started)
	if messages := live.ActiveMessages(); len(messages) < 1 || messages[0].Content != "!"+script {
		t.Fatalf("shell optimistic message = %+v", messages)
	}
}

func TestLiveFailedFileTurnRestoresRetryDraft(t *testing.T) {
	live := &Live{activeID: "sess_1", messages: map[string][]surface.Message{"sess_1": {{Role: string(domain.RoleUser), Content: "inspect"}}}}
	cmd := live.applyTurnStarted(liveTurnStartedMsg{
		SessionID: "sess_1", UserText: "inspect", ContextPaths: []string{"README.md", "internal/app.go"}, Err: errors.New("resolve failed"),
	})
	if cmd == nil {
		t.Fatal("failed file turn did not return a restore command")
	}
	msg, ok := cmd().(surface.RestoreInputMsg)
	if !ok || msg.Text != "inspect @README.md @internal/app.go" {
		t.Fatalf("restore message = %#v", msg)
	}
}

package tui

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"

	controlrpc "agent-vivy/internal/rpc"
)

func TestREPLHelpAndQuit(t *testing.T) {
	handler := controlrpc.HandlerFunc(func(_ context.Context, _ *controlrpc.Peer, request controlrpc.Request) (any, *controlrpc.Error) {
		switch request.Method {
		case "session/create":
			return map[string]string{"id": "sess_1", "title": "TUI", "permission_preset": "smart"}, nil
		default:
			return nil, &controlrpc.Error{Code: controlrpc.MethodNotFound, Message: request.Method}
		}
	})
	client, stop := attachTestClient(t, handler)
	defer stop()

	var out bytes.Buffer
	err := RunREPL(context.Background(), client, Options{
		Input:  strings.NewReader("/help\n/quit\n"),
		Output: &out,
		Title:  "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "session sess_1") {
		t.Fatalf("missing session banner:\n%s", got)
	}
	if !strings.Contains(got, "/cancel") {
		t.Fatalf("missing help:\n%s", got)
	}
}

func TestREPLCommandsUseSharedParserAndNeverForwardUnknownSlash(t *testing.T) {
	var createdTitle string
	var renamedTitle string
	var deletedID string
	var turnCalls int
	handler := controlrpc.HandlerFunc(func(_ context.Context, _ *controlrpc.Peer, request controlrpc.Request) (any, *controlrpc.Error) {
		switch request.Method {
		case "session/create":
			var params struct {
				Title string `json:"title"`
			}
			_ = json.Unmarshal(request.Params, &params)
			createdTitle = params.Title
			return map[string]string{"id": "sess_new", "title": params.Title, "permission_preset": "smart"}, nil
		case "session/rename":
			var params struct {
				Title string `json:"title"`
			}
			_ = json.Unmarshal(request.Params, &params)
			renamedTitle = params.Title
			return map[string]string{"id": "sess_1", "title": params.Title, "permission_preset": "smart"}, nil
		case "session/list":
			return map[string]any{"sessions": []map[string]string{
				{"id": "sess_1", "title": "one", "permission_preset": "smart"},
				{"id": "sess_2", "title": "two", "permission_preset": "smart"},
			}}, nil
		case "session/delete":
			var params struct {
				SessionID string `json:"session_id"`
			}
			_ = json.Unmarshal(request.Params, &params)
			deletedID = params.SessionID
			return map[string]bool{"deleted": true}, nil
		case "session/set_permission":
			var params struct {
				Preset string `json:"preset"`
			}
			_ = json.Unmarshal(request.Params, &params)
			return map[string]string{"id": "sess_1", "title": "one", "permission_preset": params.Preset}, nil
		case "turn/start":
			turnCalls++
			return map[string]string{"run_id": "run_1", "status": "accepted"}, nil
		default:
			return nil, &controlrpc.Error{Code: controlrpc.MethodNotFound, Message: request.Method}
		}
	})
	client, stop := attachTestClient(t, handler)
	defer stop()

	var out bytes.Buffer
	r := &repl{
		client:  client,
		out:     &out,
		session: sessionView{ID: "sess_1", Title: "one", PermissionPreset: "smart"},
		events:  make(chan eventNotice, 1),
	}
	if err := r.handleLine(context.Background(), `/does-not-exist "🙂"`); err != nil {
		t.Fatal(err)
	}
	if turnCalls != 0 || !strings.Contains(out.String(), "unknown command /does-not-exist") {
		t.Fatalf("unknown command was forwarded or hidden: turns=%d output=%q", turnCalls, out.String())
	}
	if err := r.handleLine(context.Background(), `/new "你好 世界"`); err != nil {
		t.Fatal(err)
	}
	if createdTitle != "你好 世界" {
		t.Fatalf("quoted title = %q", createdTitle)
	}
	if err := r.handleLine(context.Background(), `/rename '改名 🙂'`); err != nil {
		t.Fatal(err)
	}
	if renamedTitle != "改名 🙂" {
		t.Fatalf("quoted rename = %q", renamedTitle)
	}
	if err := r.handleLine(context.Background(), `/permission trusted`); err != nil {
		t.Fatal(err)
	}
	if err := r.handleLine(context.Background(), `/queue clear`); err != nil {
		t.Fatal(err)
	}
	if err := r.handleLine(context.Background(), `/delete sess_2`); err != nil {
		t.Fatal(err)
	}
	if deletedID != "" {
		t.Fatal("delete mutated before confirmation")
	}
	if err := r.handleLine(context.Background(), "n"); err != nil {
		t.Fatal(err)
	}
	if deletedID != "" {
		t.Fatal("delete denial still mutated session")
	}
	if err := r.handleLine(context.Background(), `/delete sess_2`); err != nil {
		t.Fatal(err)
	}
	if err := r.handleLine(context.Background(), "yes"); err != nil {
		t.Fatal(err)
	}
	if deletedID != "sess_2" {
		t.Fatalf("confirmed delete id=%q", deletedID)
	}
}

func TestREPLAdvancedCommandsUseRPCAndPrefixesStayLocal(t *testing.T) {
	var methods []string
	handler := controlrpc.HandlerFunc(func(_ context.Context, _ *controlrpc.Peer, request controlrpc.Request) (any, *controlrpc.Error) {
		methods = append(methods, request.Method)
		switch request.Method {
		case "context/compact":
			return map[string]any{"before_tokens": 20, "after_tokens": 8}, nil
		case "session/context":
			return map[string]any{"feed_tokens": 8, "total_messages": 2}, nil
		case "session/fork":
			return map[string]any{"session_id": "sess_fork", "fork_point_message_id": "msg_1", "copied_count": 1}, nil
		case "session/get":
			return map[string]any{"session": map[string]any{"id": "sess_fork", "title": "branch", "permission_preset": "smart"}}, nil
		case "session/rewind":
			return map[string]any{"cutoff_message_id": "msg_1", "remaining_count": 1}, nil
		case "session/messages":
			return map[string]any{"messages": []any{map[string]any{"id": "msg_1", "role": "user", "content": "kept"}}}, nil
		case "session/todos":
			return map[string]any{"todos": []any{map[string]any{"subject": "ship"}}}, nil
		case "stats/tokens":
			return map[string]any{"period": "1w", "total": map[string]any{"total_tokens": 42, "cost_known": false}}, nil
		case "skills/list":
			return map[string]any{"skills": []any{map[string]any{"name": "writer"}}}, nil
		case "settings/mcp":
			return map[string]any{"servers": []any{map[string]any{"name": "docs"}}}, nil
		case "settings/mcp/resources":
			return map[string]any{"server": "docs", "resources": []any{map[string]any{"uri": "docs://guide", "name": "guide"}}, "untrusted": true}, nil
		case "settings/mcp/read":
			return map[string]any{"server": "docs", "uri": "docs://guide", "contents": []any{map[string]any{"uri": "docs://guide", "text": "hello"}}, "untrusted": true}, nil
		case "tools/list":
			return map[string]any{"active": []string{"read_file"}}, nil
		case "workspace/list":
			return map[string]any{"files": []any{map[string]any{"path": "README.md"}}}, nil
		default:
			return nil, &controlrpc.Error{Code: controlrpc.MethodNotFound, Message: request.Method}
		}
	})
	client, stop := attachTestClient(t, handler)
	defer stop()
	var out bytes.Buffer
	r := &repl{
		client:  client,
		out:     &out,
		session: sessionView{ID: "sess_1", Title: "one"},
		runID:   "run_1",
		events:  make(chan eventNotice, 1),
	}
	for _, line := range []string{"!echo", "@README.md"} {
		if err := r.handleLine(context.Background(), line); err != nil {
			t.Fatal(err)
		}
	}
	if len(methods) != 0 {
		t.Fatalf("unavailable prefixes unexpectedly called RPCs: %v", methods)
	}
	for _, tc := range []struct {
		line   string
		method string
		want   string
	}{
		{"/todos", "session/todos", "ship"},
		{"/stats 1w", "stats/tokens", "total_tokens"},
		{"/skills", "skills/list", "writer"},
		{"/mcp", "settings/mcp", "docs"},
		{"/mcp resources docs", "settings/mcp/resources", "docs://guide"},
		{"/mcp read docs docs://guide", "settings/mcp/read", "hello"},
		{"/tools", "tools/list", "read_file"},
		{"/files run_1", "workspace/list", "README.md"},
	} {
		if err := r.handleLine(context.Background(), tc.line); err != nil {
			t.Fatal(err)
		}
		if len(methods) == 0 || methods[len(methods)-1] != tc.method {
			t.Fatalf("%s called %v, want %s", tc.line, methods, tc.method)
		}
		if !strings.Contains(out.String(), tc.want) {
			t.Fatalf("%s output missing %q:\n%s", tc.line, tc.want, out.String())
		}
	}
	if err := r.handleLine(context.Background(), "/compact"); err != nil {
		t.Fatal(err)
	}
	if len(methods) == 0 || methods[len(methods)-1] != "workspace/list" {
		t.Fatalf("compact asked for confirmation but called RPCs: %v", methods)
	}
	if err := r.handleLine(context.Background(), "y"); err != nil {
		t.Fatal(err)
	}
	if len(methods) == 0 || methods[len(methods)-1] != "session/context" || !strings.Contains(out.String(), "before_tokens") || !strings.Contains(out.String(), "feed_tokens") {
		t.Fatalf("confirmed compact did not call/render result: methods=%v output=%s", methods, out.String())
	}
	if err := r.handleLine(context.Background(), "/fork msg_1 branch"); err != nil {
		t.Fatal(err)
	}
	if err := r.handleLine(context.Background(), "yes"); err != nil {
		t.Fatal(err)
	}
	if r.session.ID != "sess_fork" || r.session.Title != "branch" || methods[len(methods)-1] != "session/messages" {
		t.Fatalf("confirmed fork did not converge to forked session: session=%+v methods=%v", r.session, methods)
	}
	if err := r.handleLine(context.Background(), "/rewind msg_1"); err != nil {
		t.Fatal(err)
	}
	if err := r.handleLine(context.Background(), "y"); err != nil {
		t.Fatal(err)
	}
	if methods[len(methods)-1] != "session/messages" || !strings.Contains(out.String(), "you: kept") {
		t.Fatalf("confirmed rewind did not refresh history: methods=%v output=%s", methods, out.String())
	}
	if !strings.Contains(out.String(), "shell commands are unavailable") || !strings.Contains(out.String(), "workspace references are unavailable") {
		t.Fatalf("local unavailable diagnostics missing:\n%s", out.String())
	}
}

func TestREPLEscapedLocalPrefixesBecomeModelText(t *testing.T) {
	var turns []string
	var thinking []string
	handler := controlrpc.HandlerFunc(func(_ context.Context, _ *controlrpc.Peer, request controlrpc.Request) (any, *controlrpc.Error) {
		switch request.Method {
		case "session/context":
			return map[string]any{"thinking_supported": true}, nil
		case "turn/start":
			var params struct {
				Text     string `json:"text"`
				Thinking string `json:"thinking"`
			}
			_ = json.Unmarshal(request.Params, &params)
			turns = append(turns, params.Text)
			thinking = append(thinking, params.Thinking)
			return map[string]any{"run_id": "run_1", "status": "accepted"}, nil
		case "run/subscribe":
			return map[string]any{"status": "subscribed"}, nil
		default:
			return nil, &controlrpc.Error{Code: controlrpc.MethodNotFound, Message: request.Method}
		}
	})
	client, stop := attachTestClient(t, handler)
	defer stop()
	r := &repl{
		client:  client,
		out:     &bytes.Buffer{},
		session: sessionView{ID: "sess_1"},
		events:  make(chan eventNotice, 1),
	}
	if err := r.handleLine(context.Background(), "/thinking on"); err != nil {
		t.Fatal(err)
	}
	for _, input := range []string{"!!echo safe", "@@README.md"} {
		r.events <- eventNotice{Done: true}
		if err := r.handleLine(context.Background(), input); err != nil {
			t.Fatal(err)
		}
	}
	if len(turns) != 2 || turns[0] != "!echo safe" || turns[1] != "@README.md" {
		t.Fatalf("escaped prefix turns = %#v", turns)
	}
	if strings.Join(thinking, ",") != "on,on" {
		t.Fatalf("REPL turn thinking modes = %v", thinking)
	}
}

func TestREPLGateTakesPriorityOverSlashParser(t *testing.T) {
	var turnCalls int
	handler := controlrpc.HandlerFunc(func(_ context.Context, _ *controlrpc.Peer, request controlrpc.Request) (any, *controlrpc.Error) {
		if request.Method == "turn/start" {
			turnCalls++
		}
		return map[string]any{}, nil
	})
	client, stop := attachTestClient(t, handler)
	defer stop()
	var out bytes.Buffer
	r := &repl{
		client:  client,
		out:     &out,
		session: sessionView{ID: "sess_1"},
		pending: &gatePrompt{Kind: "approval", ID: "approval_1"},
		events:  make(chan eventNotice, 1),
	}
	if err := r.handleLine(context.Background(), "/help"); err != nil {
		t.Fatal(err)
	}
	if turnCalls != 0 || !strings.Contains(out.String(), "type y or n") {
		t.Fatalf("gate did not consume slash line: turns=%d output=%q", turnCalls, out.String())
	}
}

func TestREPLCancelDrainsTerminalEventAndClearsBusy(t *testing.T) {
	var cancelled string
	handler := controlrpc.HandlerFunc(func(_ context.Context, _ *controlrpc.Peer, request controlrpc.Request) (any, *controlrpc.Error) {
		if request.Method != "run/cancel" {
			return nil, &controlrpc.Error{Code: controlrpc.MethodNotFound, Message: request.Method}
		}
		var params struct {
			RunID string `json:"run_id"`
		}
		_ = json.Unmarshal(request.Params, &params)
		cancelled = params.RunID
		return map[string]string{"status": "cancelling"}, nil
	})
	client, stop := attachTestClient(t, handler)
	defer stop()
	r := &repl{
		client: client,
		out:    &bytes.Buffer{},
		busy:   true,
		runID:  "run_1",
		events: make(chan eventNotice, 1),
	}
	r.events <- eventNotice{Done: true}
	if err := r.handleLine(context.Background(), "/cancel"); err != nil {
		t.Fatal(err)
	}
	if cancelled != "run_1" || r.busy || r.runID != "" {
		t.Fatalf("cancel did not close run: cancelled=%q busy=%v run=%q", cancelled, r.busy, r.runID)
	}
}

func TestClientNotifyDecodesStreamDelta(t *testing.T) {
	// Live streaming is exercised through OnNotify (WebSocket run/event).
	// Keep this path deterministic without a JSONL peer race on AfterResponse.
	client := &Client{}
	var deltas strings.Builder
	var done bool
	client.OnNotify(func(method string, params json.RawMessage) {
		if method != "run/event" {
			return
		}
		event, ok := decodeStreamEvent(params)
		if !ok {
			return
		}
		notice := interpret(event)
		deltas.WriteString(notice.Delta)
		if notice.Done {
			done = true
		}
	})
	paramsDelta, _ := json.Marshal(map[string]any{
		"subscription_id": "sub_1",
		"event": map[string]any{
			"run_id": "run_1", "seq": 1, "type": "model.delta",
			"payload": map[string]string{"delta": "hi"},
		},
	})
	paramsDone, _ := json.Marshal(map[string]any{
		"subscription_id": "sub_1",
		"event": map[string]any{
			"run_id": "run_1", "seq": 2, "type": "run.completed",
			"payload": map[string]any{},
		},
	})
	if _, err := client.Handle(context.Background(), nil, controlrpc.Request{Method: "run/event", Params: paramsDelta}); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Handle(context.Background(), nil, controlrpc.Request{Method: "run/event", Params: paramsDone}); err != nil {
		t.Fatal(err)
	}
	if deltas.String() != "hi" || !done {
		t.Fatalf("delta=%q done=%v", deltas.String(), done)
	}
}

func TestREPLTurnStartAndSubscribeRPC(t *testing.T) {
	handler := controlrpc.HandlerFunc(func(_ context.Context, _ *controlrpc.Peer, request controlrpc.Request) (any, *controlrpc.Error) {
		switch request.Method {
		case "session/create":
			return map[string]string{"id": "sess_1", "title": "TUI", "permission_preset": "smart"}, nil
		case "turn/start":
			return map[string]string{"run_id": "run_1", "status": "accepted"}, nil
		case "run/subscribe":
			return map[string]string{"subscription_id": "sub_1"}, nil
		case "run/cancel":
			return map[string]string{"status": "cancelling"}, nil
		default:
			return nil, &controlrpc.Error{Code: controlrpc.MethodNotFound, Message: request.Method}
		}
	})
	client, stop := attachTestClient(t, handler)
	defer stop()

	// Avoid blocking drainRun on a missing stream: cancel immediately via /cancel
	// after starting is awkward in one input line; instead only assert session + help
	// path and that turn RPCs succeed through the client helpers.
	ctx := context.Background()
	session, err := client.createSession(ctx, "rpc")
	if err != nil {
		t.Fatal(err)
	}
	accepted, err := client.startTurn(ctx, session.ID, "hello", "code", "auto")
	if err != nil {
		t.Fatal(err)
	}
	if accepted.RunID != "run_1" {
		t.Fatalf("run = %+v", accepted)
	}
	if err := client.subscribe(ctx, accepted.RunID, 0); err != nil {
		t.Fatal(err)
	}
}

func attachTestClient(t *testing.T, serverHandler controlrpc.Handler) (*Client, func()) {
	t.Helper()
	left, right := io.Pipe()
	up, down := io.Pipe()
	client := &Client{}
	serverPeer := controlrpc.NewPeer(controlrpc.NewJSONLTransport(left, down, left.Close), serverHandler, controlrpc.Options{OutgoingBuffer: 16})
	clientPeer := controlrpc.NewPeer(controlrpc.NewJSONLTransport(up, right, up.Close), client, controlrpc.Options{OutgoingBuffer: 16})
	client.peer = clientPeer
	ctx, cancel := context.WithCancel(context.Background())
	go func() { _ = serverPeer.Serve(ctx) }()
	go func() { _ = clientPeer.Serve(ctx) }()
	return client, func() {
		cancel()
		_ = serverPeer.Close()
		_ = clientPeer.Close()
	}
}

func TestSplitAddr(t *testing.T) {
	httpBase, wsBase, err := splitAddr("127.0.0.1:8787")
	if err != nil {
		t.Fatal(err)
	}
	if httpBase != "http://127.0.0.1:8787" || wsBase != "ws://127.0.0.1:8787" {
		t.Fatalf("%s %s", httpBase, wsBase)
	}
}

func TestCallJSONRoundTrip(t *testing.T) {
	handler := controlrpc.HandlerFunc(func(_ context.Context, _ *controlrpc.Peer, request controlrpc.Request) (any, *controlrpc.Error) {
		if request.Method == "session/create" {
			return map[string]string{"id": "sess_x", "title": "n"}, nil
		}
		return nil, &controlrpc.Error{Code: controlrpc.MethodNotFound, Message: request.Method}
	})
	client, stop := attachTestClient(t, handler)
	defer stop()
	session, err := client.createSession(context.Background(), "n")
	if err != nil {
		t.Fatal(err)
	}
	if session.ID != "sess_x" {
		t.Fatalf("session = %+v", session)
	}
}

// rpc.go implements `--mode rpc`: a stdin/stdout JSONL command loop on top
// of the face Host — pi's headless embedding protocol. One JSON object per
// line in and out; responses carry the command's `id` for correlation;
// journal events for the active run stream as un-correlated records using
// the same record names as --mode json (sdk/facerun JSONLSink).
//
// Commands whose backend does not exist yet (thinking levels, retry knobs,
// bash) answer success:false with "not implemented" and land as F1 and the
// deferred set arrive — see the spec §5.3 map.
package face

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"agent-vivy/sdk/facerun"
	faceport "agent-vivy/sdk/port/face"
)

// rpcCommand is one incoming line: {"id": "...", "type": "<cmd>", ...rest}.
// Rest fields are captured raw and re-decoded per command.
type rpcCommand struct {
	ID   string          `json:"id,omitempty"`
	Type string          `json:"type"`
	Raw  json.RawMessage `json:"-"`
}

type rpcMode struct {
	env     faceport.Host
	in      io.Reader
	out     io.Writer
	errW    io.Writer
	writeMu sync.Mutex

	mu        sync.Mutex
	sessionID string
	activeRun string

	proj facerun.JSONLSink // event projection (shares --mode json record names)
}

func (f *terminalFace) runRPCMode(ctx context.Context, env faceport.Host) (faceport.Result, error) {
	if f.opts.In == nil {
		return faceport.Result{Status: "failed"}, errors.New("rpc: stdin is required")
	}
	m := &rpcMode{
		env:  env,
		in:   f.opts.In,
		out:  f.opts.Out,
		errW: f.opts.Err,
		proj: facerun.JSONLSink{Out: nil}, // rebound below once m exists
	}
	m.proj = facerun.JSONLSink{Out: m}
	if _, err := env.Call(ctx, "initialize", nil); err != nil {
		return faceport.Result{}, fmt.Errorf("rpc: initialize: %w", err)
	}
	if err := m.resolveSession(ctx, f.opts); err != nil {
		return faceport.Result{}, err
	}
	env.OnEvent(m.onNotify)

	scanner := bufio.NewScanner(m.in)
	scanner.Buffer(make([]byte, 0, 64<<10), 4<<20)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var cmd rpcCommand
		var raw json.RawMessage
		err := json.Unmarshal([]byte(line), &raw)
		if err == nil {
			cmd.Raw = raw
			err = json.Unmarshal(raw, &cmd)
		}
		if err != nil || cmd.Type == "" {
			m.respond("", "", false, nil, "invalid command line: not a JSON object with a type")
			continue
		}
		m.dispatch(ctx, cmd)
	}
	if err := scanner.Err(); err != nil {
		return faceport.Result{Status: "failed"}, fmt.Errorf("rpc: stdin: %w", err)
	}
	return faceport.Result{Status: "completed"}, nil // stdin EOF = graceful shutdown
}

// Write lets JSONLSink emit event records through the protocol writer.
func (m *rpcMode) Write(p []byte) (int, error) {
	m.writeMu.Lock()
	defer m.writeMu.Unlock()
	return m.out.Write(p)
}

func (m *rpcMode) respond(id, command string, success bool, data any, errText string) {
	rec := map[string]any{"type": "response", "command": command, "success": success}
	if id != "" {
		rec["id"] = id
	}
	if success {
		rec["data"] = data
	} else {
		rec["error"] = errText
	}
	line, _ := json.Marshal(rec)
	m.writeMu.Lock()
	defer m.writeMu.Unlock()
	_, _ = m.out.Write(append(line, '\n'))
}

func (m *rpcMode) resolveSession(ctx context.Context, opts faceport.Options) error {
	if id := strings.TrimSpace(opts.SessionID); id != "" {
		m.sessionID = id
		return nil
	}
	if opts.ContinueNewest {
		raw, err := m.env.Call(ctx, "session/list", nil)
		if err != nil {
			return fmt.Errorf("rpc: session/list: %w", err)
		}
		var list struct {
			Sessions []struct {
				ID string `json:"id"`
			} `json:"sessions"`
		}
		if err := json.Unmarshal(raw, &list); err != nil {
			return fmt.Errorf("rpc: decode session/list: %w", err)
		}
		if len(list.Sessions) == 0 {
			return errors.New("rpc: no sessions to continue; run without --continue to start one")
		}
		m.sessionID = list.Sessions[0].ID
		return nil
	}
	raw, err := m.env.Call(ctx, "session/create", map[string]any{"title": "vivy-code rpc"})
	if err != nil {
		return fmt.Errorf("rpc: session/create: %w", err)
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &created); err != nil || created.ID == "" {
		return fmt.Errorf("rpc: session/create returned %s", raw)
	}
	m.sessionID = created.ID
	return nil
}

// sessionState snapshots the rpc session state (pi RpcSessionState shape).
func (m *rpcMode) snapshot() (sessionID, activeRun string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.sessionID, m.activeRun
}

func (m *rpcMode) dispatch(ctx context.Context, cmd rpcCommand) {
	respond := func(success bool, data any, errText string) {
		m.respond(cmd.ID, cmd.Type, success, data, errText)
	}
	call := func(method string, params map[string]any) (json.RawMessage, bool) {
		raw, err := m.env.Call(ctx, method, params)
		if err != nil {
			respond(false, nil, err.Error())
			return nil, false
		}
		return raw, true
	}
	notImplemented := func(feature string) {
		respond(false, nil, fmt.Sprintf("%s not implemented yet (lands with %s)", cmd.Type, feature))
	}
	decode := func(v any) bool {
		if err := json.Unmarshal(cmd.Raw, v); err != nil {
			respond(false, nil, fmt.Sprintf("invalid %s params: %v", cmd.Type, err))
			return false
		}
		return true
	}

	sessionID, activeRun := m.snapshot()
	// steerCommand issues one queued turn through the kernel-owned
	// dual-track queue (VCP-B1): "steer" injects at the next turn boundary,
	// "follow_up" waits for terminal settle. Both answer pi-style
	// disposition; an idle session degrades to a fresh run via the RPC
	// fallback (run_id in the payload).
	steerCommand := func(track, message string) {
		raw, ok := call("turn/"+track, map[string]any{
			"session_id": sessionID, "text": message,
		})
		if !ok {
			return
		}
		var res struct {
			Queued bool   `json:"queued"`
			RunID  string `json:"run_id"`
		}
		_ = json.Unmarshal(raw, &res)
		if res.Queued {
			respond(true, map[string]any{"disposition": "queued"}, "")
			return
		}
		respond(true, map[string]any{"disposition": "started", "run_id": res.RunID}, "")
	}
	switch cmd.Type {
	case "prompt", "follow_up":
		var p struct {
			Message           string `json:"message"`
			StreamingBehavior string `json:"streamingBehavior"`
		}
		if !decode(&p) {
			return
		}
		if strings.TrimSpace(p.Message) == "" {
			respond(false, nil, "message is required")
			return
		}
		if p.StreamingBehavior == "steer" {
			steerCommand("steer", p.Message)
			return
		}
		if cmd.Type == "follow_up" || activeRun != "" {
			steerCommand("follow_up", p.Message)
			return
		}
		runID, err := m.startTurn(ctx, p.Message)
		if err != nil {
			respond(false, nil, err.Error())
			return
		}
		respond(true, map[string]any{"disposition": "started", "run_id": runID}, "")
	case "steer":
		var p struct {
			Message string `json:"message"`
		}
		if !decode(&p) {
			return
		}
		if strings.TrimSpace(p.Message) == "" {
			respond(false, nil, "message is required")
			return
		}
		steerCommand("steer", p.Message)
	case "abort":
		if activeRun == "" {
			respond(true, nil, "")
			return
		}
		if _, ok := call("run/cancel", map[string]any{"run_id": activeRun}); ok {
			respond(true, nil, "")
		}
	case "interrupt":
		if activeRun == "" {
			respond(true, nil, "")
			return
		}
		if _, ok := call("turn/interrupt", map[string]any{"run_id": activeRun}); ok {
			respond(true, nil, "")
		}
	case "clear_queue":
		raw, ok := call("queue/clear", map[string]any{"session_id": sessionID})
		if !ok {
			return
		}
		var res struct {
			Texts []string `json:"texts"`
		}
		_ = json.Unmarshal(raw, &res)
		respond(true, map[string]any{"steering": []string{}, "followUp": res.Texts}, "")
	case "new_session":
		var p struct {
			ParentSession string `json:"parentSession"`
			Title         string `json:"title"`
		}
		_ = decode(&p)
		raw, ok := call("session/create", map[string]any{"title": p.Title})
		if !ok {
			return
		}
		var created struct {
			ID string `json:"id"`
		}
		_ = json.Unmarshal(raw, &created)
		m.mu.Lock()
		m.sessionID = created.ID
		m.mu.Unlock()
		respond(true, map[string]any{"cancelled": false, "session_id": created.ID}, "")
	case "switch_session":
		var p struct {
			SessionPath string `json:"sessionPath"`
			SessionID   string `json:"session_id"`
		}
		if !decode(&p) {
			return
		}
		target := strings.TrimSpace(p.SessionID)
		if target == "" {
			target = strings.TrimSpace(p.SessionPath) // pi field name; VIVY ids double as paths
		}
		if target == "" {
			respond(false, nil, "session_id (or sessionPath) is required")
			return
		}
		if _, ok := call("session/get", map[string]any{"session_id": target}); !ok {
			return
		}
		m.mu.Lock()
		m.sessionID = target
		m.mu.Unlock()
		respond(true, map[string]any{"cancelled": false, "session_id": target}, "")
	case "set_session_name":
		var p struct {
			Name string `json:"name"`
		}
		if !decode(&p) {
			return
		}
		if _, ok := call("session/rename", map[string]any{"session_id": sessionID, "title": p.Name}); ok {
			respond(true, nil, "")
		}
	case "get_state":
		pending := 0
		steerMode, followUpMode := "all", "all"
		if raw, ok := call("queue/state", map[string]any{"session_id": sessionID}); ok {
			var res struct {
				Pending      int    `json:"pending"`
				SteerMode    string `json:"steer_mode"`
				FollowUpMode string `json:"follow_up_mode"`
			}
			if json.Unmarshal(raw, &res) == nil {
				pending, steerMode, followUpMode = res.Pending, res.SteerMode, res.FollowUpMode
			}
		}
		respond(true, map[string]any{
			"session_id":            sessionID,
			"isStreaming":           activeRun != "",
			"isCompacting":          false,
			"pendingMessageCount":   pending,
			"steeringMode":          steerMode,
			"followUpMode":          followUpMode,
			"autoCompactionEnabled": true,
		}, "")
	case "get_messages":
		if raw, ok := call("session/messages", map[string]any{"session_id": sessionID}); ok {
			respond(true, rawToMap(raw), "")
		}
	case "get_last_assistant_text":
		raw, ok := call("session/messages", map[string]any{"session_id": sessionID})
		if !ok {
			return
		}
		respond(true, map[string]any{"text": lastAssistantText(raw)}, "")
	case "get_session_stats":
		if raw, ok := call("stats/tokens", map[string]any{"session_id": sessionID}); ok {
			respond(true, rawToMap(raw), "")
		}
	case "get_available_models":
		if raw, ok := call("settings/providers", nil); ok {
			respond(true, rawToMap(raw), "")
		}
	case "set_model":
		var p struct {
			Provider string `json:"provider"`
			ModelID  string `json:"modelId"`
			Model    string `json:"model"`
		}
		if !decode(&p) {
			return
		}
		model := p.ModelID
		if model == "" {
			model = p.Model
		}
		if p.Provider == "" || model == "" {
			respond(false, nil, "provider and modelId are required")
			return
		}
		if raw, ok := call("settings/model/select", map[string]any{"provider": p.Provider, "model": model}); ok {
			respond(true, rawToMap(raw), "")
		}
	case "compact":
		var p struct {
			CustomInstructions string `json:"customInstructions"`
		}
		_ = decode(&p)
		params := map[string]any{"session_id": sessionID}
		if p.CustomInstructions != "" {
			params["instructions"] = p.CustomInstructions // D1 lands the kernel side; passed through already
		}
		if raw, ok := call("context/compact", params); ok {
			respond(true, rawToMap(raw), "")
		}
	case "fork":
		var p struct {
			EntryID   string `json:"entryId"`
			MessageID string `json:"message_id"`
		}
		if !decode(&p) {
			return
		}
		cut := p.EntryID
		if cut == "" {
			cut = p.MessageID
		}
		if cut == "" {
			respond(false, nil, "entryId (message id) is required")
			return
		}
		raw, ok := call("session/fork", map[string]any{"session_id": sessionID, "message_id": cut})
		if !ok {
			return
		}
		m.adoptForked(raw)
		respond(true, map[string]any{"cancelled": false, "session": rawToMap(raw)}, "")
	case "clone":
		// Kernel C1 clone: a fork pinned at the effective tail with the
		// session.cloned_from provenance event (also covers empty sessions).
		cloned, ok := call("session/clone", map[string]any{"session_id": sessionID})
		if !ok {
			return
		}
		m.adoptForked(cloned)
		respond(true, map[string]any{"cancelled": false, "session": rawToMap(cloned)}, "")
	case "get_fork_messages":
		raw, ok := call("session/messages", map[string]any{"session_id": sessionID})
		if !ok {
			return
		}
		respond(true, map[string]any{"messages": forkableMessages(raw)}, "")
	case "get_commands":
		respond(true, map[string]any{"commands": rpcCommands()}, "")
	case "set_thinking_level", "cycle_thinking_level", "get_available_thinking_levels":
		notImplemented("F1 thinking levels")
	case "set_steering_mode", "set_follow_up_mode":
		var p struct {
			Mode string `json:"mode"`
		}
		if !decode(&p) {
			return
		}
		track := "follow_up"
		if cmd.Type == "set_steering_mode" {
			track = "steer"
		}
		if _, ok := call("queue/mode", map[string]any{
			"session_id": sessionID, "track": track, "mode": p.Mode,
		}); ok {
			respond(true, nil, "")
		}
	case "set_auto_compaction", "set_auto_retry", "abort_retry":
		notImplemented("settings/retry surfaces")
	case "cycle_model":
		notImplemented("scoped model cycling (F3)")
	case "bash", "abort_bash":
		respond(false, nil, "bash is refused: commands must route through the governed ToolHost, not a raw exec")
	case "export_html":
		// The kernel renders a standalone HTML transcript into the instance
		// exports dir and returns its path; the pi outputPath parameter is
		// accepted but the kernel chooses the location.
		raw, ok := call("session/export", map[string]any{"session_id": sessionID, "format": "html"})
		if !ok {
			return
		}
		respond(true, rawToMap(raw), "")
	case "get_tree":
		raw, ok := call("session/tree", map[string]any{})
		if !ok {
			return
		}
		respond(true, rawToMap(raw), "")
	case "get_entries":
		raw, ok := call("session/messages", map[string]any{"session_id": sessionID})
		if !ok {
			return
		}
		respond(true, map[string]any{"entries": forkableMessages(raw)}, "")
	case "extension_ui_response":
		// no extension UI surface — acknowledged and ignored
	default:
		respond(false, nil, fmt.Sprintf("unknown command %q", cmd.Type))
	}
}

// startTurn starts a run for the current session and subscribes for events.
// Returns the run_id; the terminal tail (turn_end/agent_end/agent_settled)
// is emitted by the event pump when the run settles.
func (m *rpcMode) startTurn(ctx context.Context, text string) (string, error) {
	m.mu.Lock()
	sessionID := m.sessionID
	m.mu.Unlock()
	raw, err := m.env.Call(ctx, "turn/start", map[string]any{
		"session_id": sessionID,
		"text":       text,
		"face":       "code",
	})
	if err != nil {
		return "", fmt.Errorf("turn/start: %w", err)
	}
	var start struct {
		RunID string `json:"run_id"`
	}
	if err := json.Unmarshal(raw, &start); err != nil || start.RunID == "" {
		return "", fmt.Errorf("turn/start returned %s", raw)
	}
	m.mu.Lock()
	m.activeRun = start.RunID
	m.mu.Unlock()
	m.proj.Emit("session", map[string]any{"session_id": sessionID})
	m.proj.Emit("turn_start", map[string]any{"session_id": sessionID, "run_id": start.RunID})
	if _, err := m.env.Call(ctx, "run/subscribe", map[string]any{"run_id": start.RunID}); err != nil {
		return "", fmt.Errorf("run/subscribe: %w", err)
	}
	return start.RunID, nil
}

func (m *rpcMode) adoptForked(raw json.RawMessage) {
	var f struct {
		SessionID string `json:"session_id"`
		ID        string `json:"id"`
	}
	if json.Unmarshal(raw, &f) != nil {
		return
	}
	id := f.SessionID
	if id == "" {
		id = f.ID
	}
	if id != "" {
		m.mu.Lock()
		m.sessionID = id
		m.mu.Unlock()
	}
}

// admitSettledFollowUp resolves the follow-up run the kernel auto-started
// for a settle, then subscribes it onto the stream. The admission lands in
// the session queue moments after the terminal event, so poll briefly.
func (m *rpcMode) admitSettledFollowUp(settledRunID string) {
	for i := 0; i < 40; i++ {
		raw, err := m.env.Call(context.Background(), "queue/state", map[string]any{
			"session_id":   m.sessionID,
			"after_run_id": settledRunID,
		})
		if err == nil {
			var qs struct {
				AdmittedRunID string `json:"admitted_run_id"`
			}
			if json.Unmarshal(raw, &qs) == nil && qs.AdmittedRunID != "" {
				m.mu.Lock()
				m.activeRun = qs.AdmittedRunID
				m.mu.Unlock()
				m.proj.Emit("turn_start", map[string]any{"session_id": m.sessionID, "run_id": qs.AdmittedRunID, "admitted": true})
				if _, err := m.env.Call(context.Background(), "run/subscribe", map[string]any{"run_id": qs.AdmittedRunID}); err != nil {
					m.proj.Emit("error", map[string]any{"message": fmt.Sprintf("follow-up run/subscribe failed: %v", err)})
				}
				return
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// onNotify projects journal events of the active run onto stdout and drives
// the settle tail + follow-up drain.
func (m *rpcMode) onNotify(method string, params json.RawMessage) {
	if method != "run/event" {
		return
	}
	var wire struct {
		Event struct {
			RunID   string          `json:"run_id"`
			Seq     int64           `json:"seq"`
			Type    string          `json:"type"`
			Payload json.RawMessage `json:"payload"`
		} `json:"event"`
	}
	if err := json.Unmarshal(params, &wire); err != nil {
		return
	}
	typ := wire.Event.Type

	m.mu.Lock()
	if wire.Event.RunID == "" || wire.Event.RunID != m.activeRun {
		m.mu.Unlock()
		return
	}
	m.mu.Unlock()

	// Terminal statuses settle through the lifecycle tail; everything else
	// projects through the shared mapping table.
	switch typ {
	case "run.completed", "run.failed", "run.cancelled":
		status := map[string]string{
			"run.completed": "completed",
			"run.failed":    "failed",
			"run.cancelled": "cancelled",
		}[typ]
		if typ == "run.failed" {
			var p struct {
				Message string `json:"message"`
			}
			_ = json.Unmarshal(wire.Event.Payload, &p)
			m.proj.Emit("error", map[string]any{"vivy": typ, "message": p.Message})
		}
		m.proj.RunSettled(status)
		m.mu.Lock()
		m.activeRun = ""
		m.mu.Unlock()
		// Kernel-admitted follow-up: the queue drain may auto-start a run
		// when this run settles. Admission races the terminal publish, so
		// poll queue/state{after_run_id} briefly for the admission record,
		// then subscribe so the stream projects like a prompt's.
		go m.admitSettledFollowUp(wire.Event.RunID)
		return
	}
	// model.delta is sink-rendered in facerun; rpc mode projects it itself.
	if typ == "model.delta" {
		var d struct {
			Delta string `json:"delta"`
		}
		if err := json.Unmarshal(wire.Event.Payload, &d); err == nil && d.Delta != "" {
			m.proj.Emit("message_update", map[string]any{"vivy": typ, "delta": d.Delta})
		}
		return
	}
	m.proj.JournalEvent(typ, wire.Event.Seq, wire.Event.Payload)
}

// lastAssistantText finds the last assistant message's text in a
// session/messages result ({"messages":[{role,content|text,...}]}).
func lastAssistantText(raw json.RawMessage) string {
	var list struct {
		Messages []map[string]any `json:"messages"`
	}
	if json.Unmarshal(raw, &list) != nil {
		return ""
	}
	for i := len(list.Messages) - 1; i >= 0; i-- {
		msg := list.Messages[i]
		if msg["role"] != "assistant" {
			continue
		}
		for _, key := range []string{"text", "content"} {
			if s, ok := msg[key].(string); ok && s != "" {
				return s
			}
		}
	}
	return ""
}

func forkableMessages(raw json.RawMessage) []map[string]any {
	var list struct {
		Messages []map[string]any `json:"messages"`
	}
	if json.Unmarshal(raw, &list) != nil {
		return []map[string]any{}
	}
	out := make([]map[string]any, 0, len(list.Messages))
	for _, msg := range list.Messages {
		id, _ := msg["id"].(string)
		if id == "" {
			id, _ = msg["message_id"].(string)
		}
		text, _ := msg["text"].(string)
		if text == "" {
			text, _ = msg["content"].(string)
		}
		if id != "" {
			out = append(out, map[string]any{"entryId": id, "text": text})
		}
	}
	return out
}

func rawToMap(raw json.RawMessage) map[string]any {
	var m map[string]any
	if json.Unmarshal(raw, &m) != nil {
		return map[string]any{"raw": json.RawMessage(raw)}
	}
	return m
}

// rpcCommands is the get_commands answer: the pi command set with an
// "available" flag so embedders see which commands this build honors.
func rpcCommands() []map[string]any {
	type c struct {
		name  string
		avail bool
	}
	cmds := []c{
		{"prompt", true}, {"follow_up", true}, {"steer", true},
		{"abort", true}, {"interrupt", true}, {"clear_queue", true},
		{"new_session", true}, {"switch_session", true}, {"set_session_name", true},
		{"get_state", true}, {"get_messages", true}, {"get_last_assistant_text", true},
		{"get_session_stats", true}, {"get_fork_messages", true}, {"fork", true}, {"clone", true},
		{"set_model", true}, {"get_available_models", true}, {"cycle_model", false},
		{"compact", true}, {"set_auto_compaction", false},
		{"set_thinking_level", false}, {"cycle_thinking_level", false}, {"get_available_thinking_levels", false},
		{"set_steering_mode", true}, {"set_follow_up_mode", true},
		{"set_auto_retry", false}, {"abort_retry", false},
		{"bash", false}, {"abort_bash", false},
		{"export_html", true}, {"get_tree", true}, {"get_entries", true},
		{"get_commands", true},
	}
	out := make([]map[string]any, 0, len(cmds))
	for _, c := range cmds {
		out = append(out, map[string]any{"name": c.name, "available": c.avail, "source": "core"})
	}
	return out
}

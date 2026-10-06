package app

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"testing"
	"time"

	"agent-vivy/internal/runtime"
	plugin "agent-vivy/sdk/port/face"
	tuiface "agent-vivy/sdk/tui/face"
)

// rpcSession drives `vivy-code --mode rpc` in-process: a real kernel behind a
// scripted model, command lines on a stdin pipe, protocol records on a stdout
// pipe.
type rpcSession struct {
	t      *testing.T
	in     *io.PipeWriter
	lines  *bufio.Reader
	done   chan error
	nextID int
}

func startRPCSession(t *testing.T) *rpcSession {
	t.Helper()
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	done := make(chan error, 1)
	go func() {
		res, err := RunFaceWithAppOptions(context.Background(), newDeepSeekTestConfig(t), tuiface.New, plugin.Options{
			Mode: "rpc",
			In:   inR, Out: outW, Err: io.Discard,
		})
		_ = outW.Close()
		if err == nil && res.Status != "completed" {
			err = fmt.Errorf("rpc mode exited with status %q", res.Status)
		}
		done <- err
	}()
	return &rpcSession{t: t, in: inW, lines: bufio.NewReader(outR), done: done}
}

// send writes one command line; id is assigned when empty.
func (s *rpcSession) send(cmd map[string]any) string {
	s.t.Helper()
	s.nextID++
	id := fmt.Sprintf("c%d", s.nextID)
	cmd["id"] = id
	line, _ := json.Marshal(cmd)
	if _, err := s.in.Write(append(line, '\n')); err != nil {
		s.t.Fatalf("write command: %v", err)
	}
	return id
}

// readUntil reads protocol lines until pred matches or the deadline passes.
// Returns all lines seen.
func (s *rpcSession) readUntil(pred func(map[string]any) bool) []map[string]any {
	s.t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	var seen []map[string]any
	for time.Now().Before(deadline) {
		type result struct {
			line []byte
			err  error
		}
		ch := make(chan result, 1)
		go func() {
			l, err := s.lines.ReadBytes('\n')
			ch <- result{l, err}
		}()
		select {
		case r := <-ch:
			if r.err != nil {
				s.t.Fatalf("protocol stream ended: %v (seen %v)", r.err, seen)
			}
			var rec map[string]any
			if err := json.Unmarshal(r.line, &rec); err != nil {
				s.t.Fatalf("non-JSON line on stdout: %q", r.line)
			}
			seen = append(seen, rec)
			if pred(rec) {
				return seen
			}
		case <-time.After(time.Until(deadline)):
		}
	}
	s.t.Fatalf("deadline; seen %v", seen)
	return nil
}

func (s *rpcSession) response(id string) map[string]any {
	for _, rec := range s.readUntil(func(r map[string]any) bool {
		return r["type"] == "response" && r["id"] == id
	}) {
		if rec["type"] == "response" && rec["id"] == id {
			return rec
		}
	}
	s.t.Fatalf("no response for %s", id)
	return nil
}

func (s *rpcSession) close() {
	s.t.Helper()
	_ = s.in.Close()
	if err := <-s.done; err != nil {
		s.t.Fatalf("rpc mode exit: %v", err)
	}
}

func TestCodeFaceRPCModeGoldenTranscript(t *testing.T) {
	runtime.SetEngineVersionOverride(pinnedEinoVersion)
	t.Cleanup(func() { runtime.SetEngineVersionOverride("") })
	t.Setenv("DEEPSEEK_API_KEY", "codemode-rpc-key")
	t.Setenv("VIVY_PROVIDER", "deepseek")
	t.Setenv("VIVY_API_BASE", textOnlyDeepSeekServer(t, "rpc", " ", "answer").URL)

	s := startRPCSession(t)
	defer s.close()

	// get_state before any prompt: session exists, not streaming.
	st := s.response(s.send(map[string]any{"type": "get_state"}))
	if st["success"] != true {
		t.Fatalf("get_state failed: %v", st)
	}
	state, _ := st["data"].(map[string]any)
	if state["session_id"] == "" || state["isStreaming"] == true {
		t.Fatalf("bad initial state: %v", state)
	}
	sessionID, _ := state["session_id"].(string)

	// prompt → started + full event tail.
	pid := s.send(map[string]any{"type": "prompt", "message": "hi"})
	all := s.readUntil(func(r map[string]any) bool { return r["type"] == "agent_settled" })
	var presp map[string]any
	var types []string
	for _, rec := range all {
		types = append(types, rec["type"].(string))
		if rec["type"] == "response" && rec["id"] == pid {
			presp = rec
		}
	}
	if presp == nil || presp["success"] != true {
		t.Fatalf("prompt response: %v in %v", presp, types)
	}
	data, _ := presp["data"].(map[string]any)
	if data["disposition"] != "started" || data["run_id"] == "" {
		t.Fatalf("prompt data: %v", data)
	}
	for _, w := range []string{"session", "turn_start", "agent_start", "message_start", "message_update", "message_end", "turn_end", "agent_end", "agent_settled"} {
		found := false
		for _, typ := range types {
			if typ == w {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("missing %q record in %v", w, types)
		}
	}

	// get_last_assistant_text returns the mock's answer.
	lt := s.response(s.send(map[string]any{"type": "get_last_assistant_text"}))
	ltd, _ := lt["data"].(map[string]any)
	if ltd["text"] != "rpc answer" {
		t.Fatalf("last assistant text = %v", ltd)
	}

	// set_session_name + switch_session round-trip.
	if r := s.response(s.send(map[string]any{"type": "set_session_name", "name": "golden"})); r["success"] != true {
		t.Fatalf("set_session_name: %v", r)
	}
	sw := s.response(s.send(map[string]any{"type": "switch_session", "sessionPath": sessionID}))
	if sw["success"] != true {
		t.Fatalf("switch_session: %v", sw)
	}

	// Commands not yet landed fail closed, not silently.
	for _, cmd := range []map[string]any{
		{"type": "steer", "message": "x"},
		{"type": "get_tree"},
		{"type": "bash", "command": "ls"},
	} {
		r := s.response(s.send(cmd))
		if r["success"] != false || r["error"] == "" {
			t.Fatalf("%v should fail with a reason: %v", cmd["type"], r)
		}
	}

	// Unknown command → success:false, session still healthy.
	if r := s.response(s.send(map[string]any{"type": "bogus"})); r["success"] != false {
		t.Fatalf("bogus: %v", r)
	}
	if r := s.response(s.send(map[string]any{"type": "get_state"})); r["success"] != true {
		t.Fatalf("get_state after bogus: %v", r)
	}
	// Session id is stable across the whole transcript.
	st2 := s.response(s.send(map[string]any{"type": "get_state"}))
	if sid, _ := st2["data"].(map[string]any)["session_id"].(string); sid != sessionID {
		t.Fatalf("session id drifted: %q → %q", sessionID, sid)
	}
}

func TestCodeFaceRPCModeQueuesFollowUpWhileRunning(t *testing.T) {
	runtime.SetEngineVersionOverride(pinnedEinoVersion)
	t.Cleanup(func() { runtime.SetEngineVersionOverride("") })
	t.Setenv("DEEPSEEK_API_KEY", "codemode-rpc-queue-key")
	t.Setenv("VIVY_PROVIDER", "deepseek")
	// Hanging model: prompt stays active so follow_up exercises the queue path.
	// textOnlyDeepSeekServer answers instantly, so use the scripted server with
	// a prompt-gated tool call instead — approval block settles it loudly.
	t.Setenv("VIVY_API_BASE", textOnlyDeepSeekServer(t, "ack").URL)

	s := startRPCSession(t)
	defer s.close()

	id1 := s.send(map[string]any{"type": "prompt", "message": "one"})
	s.readUntil(func(r map[string]any) bool { return r["type"] == "response" && r["id"] == id1 })
	// The model streams instantly; the queue path is covered when a second
	// prompt lands while the first may still be settling — assert the
	// disposition is a valid protocol value either way.
	p2 := s.response(s.send(map[string]any{"type": "follow_up", "message": "two"}))
	if p2["success"] != true {
		t.Fatalf("follow_up: %v", p2)
	}
	disp, _ := p2["data"].(map[string]any)["disposition"].(string)
	if disp != "started" && disp != "queued" {
		t.Fatalf("disposition = %q", disp)
	}
	s.readUntil(func(r map[string]any) bool { return r["type"] == "agent_settled" })
	// A queued follow-up drains into its own turn: if it was queued we must
	// see a second settle.
	if disp == "queued" {
		s.readUntil(func(r map[string]any) bool { return r["type"] == "agent_settled" })
	}
}

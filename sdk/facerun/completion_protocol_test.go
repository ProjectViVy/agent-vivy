package facerun

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestCompletionV3ReplayDoesNotDuplicate(t *testing.T) {
	var out, diagnostics bytes.Buffer
	e := &engine{runID: "r", sink: &TextSink{Out: &out, Err: &diagnostics}}
	terminal := make(chan string, 1)
	send := func(seq int, kind string, payload any, version int) {
		raw, _ := json.Marshal(map[string]any{"event": map[string]any{"run_id": "r", "seq": seq, "type": kind, "payload": payload, "payload_version": version}})
		e.onEvent(raw, terminal)
	}
	send(1, "model.request", map[string]any{}, 4)
	send(2, "model.delta", map[string]any{"delta": "界"}, 1)
	send(2, "model.delta", map[string]any{"delta": "界"}, 1)
	send(3, "model.completed", map[string]any{"byte_len": 3}, 3)
	send(3, "model.completed", map[string]any{"byte_len": 3}, 3)
	if out.String() != "界\n" || diagnostics.Len() != 0 {
		t.Fatalf("replayed output: %q, %s", out.String(), diagnostics.String())
	}
	select {
	case <-terminal:
		t.Fatal("model completion terminated run")
	default:
	}
	send(4, "run.completed", map[string]any{}, 1)
	select {
	case got := <-terminal:
		if got != "completed" {
			t.Fatal(got)
		}
	default:
		t.Fatal("missing run terminal")
	}
}

func TestCompletionV3Framing(t *testing.T) {
	e := &engine{completionLen: 10}
	if err := e.completeModelLocked(3, json.RawMessage(`{"byte_len":10}`)); err != nil {
		t.Fatal(err)
	}
	if e.completionLen != 0 {
		t.Fatal("completion did not reset")
	}
	for _, raw := range []string{`{}`, `{"byte_len":null}`, `{"byte_len":-1}`, `{"byte_len":1.5}`, `{"byte_len":1}`, `{"byte_len":0,"content_sha256":"old"}`} {
		if err := e.completeModelLocked(3, json.RawMessage(raw)); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
	if err := e.completeModelLocked(2, json.RawMessage(`{"byte_len":0}`)); err == nil {
		t.Fatal("accepted live v2")
	}
}

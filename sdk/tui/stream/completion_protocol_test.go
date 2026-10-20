package stream

import (
	"encoding/json"
	"testing"
)

func TestCompletionV3Framing(t *testing.T) {
	if _, err := ParseModelDelta(json.RawMessage(`{"delta":null}`)); err == nil {
		t.Fatal("accepted null delta")
	}
	var tracker ModelCompletionTracker
	if err := tracker.AddDelta("你好🌱"); err != nil {
		t.Fatal(err)
	}
	if err := tracker.Complete(3, json.RawMessage(`{"byte_len":10}`)); err != nil {
		t.Fatal(err)
	}
	if err := tracker.Complete(3, json.RawMessage(`{"byte_len":0}`)); err != nil {
		t.Fatal(err)
	}
	notice := Interpret(Event{Type: "model.completed", PayloadVersion: 3, Payload: json.RawMessage(`{"byte_len":0}`)})
	if notice.Done || notice.ProtocolError != "" || !notice.HasCompleted {
		t.Fatalf("completion terminated run: %+v", notice)
	}
	for _, raw := range []string{`{}`, `{"byte_len":null}`, `{"byte_len":-1}`, `{"byte_len":1.5}`, `{"byte_len":0,"content_sha256":"old"}`} {
		if err := tracker.Complete(3, json.RawMessage(raw)); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
	if err := tracker.Complete(2, json.RawMessage(`{"byte_len":0}`)); err == nil {
		t.Fatal("accepted live v2")
	}
	if err := tracker.AddDelta("x"); err != nil {
		t.Fatal(err)
	}
	tracker.BeginRound()
	if err := tracker.Complete(3, json.RawMessage(`{"byte_len":0}`)); err != nil {
		t.Fatal(err)
	}
}

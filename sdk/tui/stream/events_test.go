package stream

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDecodeRetainsRunAndSequence(t *testing.T) {
	event, ok := Decode(json.RawMessage(`{"subscription_id":"sub","event":{"run_id":"run_1","seq":7,"type":"model.delta","payload":{"delta":"x"}}}`))
	if !ok || event.SubscriptionID != "sub" || event.RunID != "run_1" || event.Seq != 7 || event.Type != "model.delta" {
		t.Fatalf("event = %+v ok=%v", event, ok)
	}
	if got := PayloadString(event.Payload, "delta"); got != "x" {
		t.Fatalf("delta = %q", got)
	}
}

func TestDecodeStreamErrorRequiresSubscription(t *testing.T) {
	failure, ok := DecodeStreamError(json.RawMessage(`{"subscription_id":"sub_1","message":"replay failed"}`))
	if !ok || failure.SubscriptionID != "sub_1" || failure.Message != "replay failed" {
		t.Fatalf("failure=%+v ok=%v", failure, ok)
	}
	if _, ok := DecodeStreamError(json.RawMessage(`{"message":"missing id"}`)); ok {
		t.Fatal("stream error without subscription id was accepted")
	}
}

func TestDecodeRejectsUnsequencedWireEvent(t *testing.T) {
	if event, ok := Decode(json.RawMessage(`{"subscription_id":"sub","event":{"run_id":"run_1","seq":0,"type":"model.delta","payload":{"delta":"lost"}}}`)); ok {
		t.Fatalf("unsequenced event accepted: %+v", event)
	}
}

func TestInterpretKeepsUnknownSequence(t *testing.T) {
	notice := Interpret(Event{RunID: "run_1", Seq: 3, Type: "context.compacted"})
	if notice.RunID != "run_1" || notice.Seq != 3 || notice.Kind != "" {
		t.Fatalf("unknown notice = %+v", notice)
	}
	if got := Interpret(Event{Type: "model.reasoning_delta", Payload: json.RawMessage(`{"delta":"思考"}`)}); got.Kind != "reasoning" || got.Delta != "思考" {
		t.Fatalf("reasoning = %+v", got)
	}
}

func TestInterpretCompletedAndToolIdentity(t *testing.T) {
	completed := Interpret(Event{Type: "model.completed", Payload: json.RawMessage(`{"content":"final"}`)})
	if completed.Kind != "model_completed" || !completed.HasCompleted || completed.Completed != "final" {
		t.Fatalf("completed = %+v", completed)
	}
	request := Interpret(Event{Type: "model.request"})
	if request.Kind != "model_request" {
		t.Fatalf("request = %+v", request)
	}
	requested := Interpret(Event{Type: "tool.requested", Payload: json.RawMessage(`{"tool_call_id":"call_1","tool_name":"read_file","args":{"path":"a"}}`)})
	finished := Interpret(Event{Type: "tool.finished", Payload: json.RawMessage(`{"tool_call_id":"call_1","tool_name":"read_file","result":"ok"}`)})
	if requested.ToolCallID != "call_1" || finished.ToolCallID != "call_1" {
		t.Fatalf("tool identity requested=%+v finished=%+v", requested, finished)
	}
}

func TestInterpretToolResultShowsDiffDiagnostics(t *testing.T) {
	notice := Interpret(Event{
		Type:    "tool.finished",
		Payload: json.RawMessage(`{"tool_name":"patch","result":"{\"path\":\"a.go\",\"diff\":\"@@ -1 +1 @@\\n-old\\n+new\",\"diagnostics\":\"ok\"}"}`),
	})
	if notice.Kind != "tool_finished" || !strings.Contains(notice.Line, "a.go") || !strings.Contains(notice.Line, "+new") || !strings.Contains(notice.Line, "Diagnostics") {
		t.Fatalf("notice = %+v", notice)
	}
}

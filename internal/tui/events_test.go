package tui

import (
	"encoding/json"
	"strings"
	"testing"

	"agent-vivy/internal/domain"
)

func TestInterpretModelDeltaAndTerminal(t *testing.T) {
	delta := interpret(streamEvent{
		Type:    domain.EventModelDelta,
		Payload: json.RawMessage(`{"delta":"hello"}`),
	})
	if delta.Delta != "hello" {
		t.Fatalf("delta = %q", delta.Delta)
	}
	done := interpret(streamEvent{Type: domain.EventRunCompleted})
	if !done.Done || done.Failed {
		t.Fatalf("completed = %+v", done)
	}
	failed := interpret(streamEvent{
		Type:    domain.EventRunFailed,
		Payload: json.RawMessage(`{"message":"boom"}`),
	})
	if !failed.Done || !failed.Failed || failed.Message != "boom" {
		t.Fatalf("failed = %+v", failed)
	}
}

func TestInterpretApprovalGate(t *testing.T) {
	notice := interpret(streamEvent{
		Type:    domain.EventToolApprovalRequired,
		Payload: json.RawMessage(`{"approval_id":"appr_1","tool_name":"write_file","preview":"README.md"}`),
	})
	if notice.Gate == nil || notice.Gate.Kind != "approval" || notice.Gate.ID != "appr_1" {
		t.Fatalf("gate = %+v", notice.Gate)
	}
	if got, ok := parseApproval("y"); !ok || got != domain.ApprovalApproved {
		t.Fatalf("y => %q %v", got, ok)
	}
	if got, ok := parseApproval("denied"); !ok || got != domain.ApprovalDenied {
		t.Fatalf("denied => %q %v", got, ok)
	}
	if _, ok := parseApproval("maybe"); ok {
		t.Fatal("maybe must not parse")
	}
}

func TestDecodeStreamEventEnvelope(t *testing.T) {
	event, ok := decodeStreamEvent(json.RawMessage(`{"subscription_id":"sub","event":{"run_id":"run_1","seq":7,"type":"model.delta","payload":{"delta":"x"}}}`))
	if !ok || event.Type != domain.EventModelDelta || event.RunID != "run_1" || event.Seq != 7 {
		t.Fatalf("event = %+v ok=%v", event, ok)
	}
	if payloadString(event.Payload, "delta") != "x" {
		t.Fatalf("payload = %s", event.Payload)
	}
}

func TestToolResultProjectsDiffAndDiagnostics(t *testing.T) {
	notice := interpret(streamEvent{Type: domain.EventToolFinished, Payload: json.RawMessage(`{"tool_name":"patch","result":"{\"path\":\"a.go\",\"diff\":\"@@ -1 +1 @@\\n-old\\n+new\",\"diagnostics\":\"ok\"}"}`)})
	if notice.Kind != "tool_finished" || !strings.Contains(notice.Line, "a.go") || !strings.Contains(notice.Line, "+new") || !strings.Contains(notice.Line, "Diagnostics") {
		t.Fatalf("notice = %+v", notice)
	}
}

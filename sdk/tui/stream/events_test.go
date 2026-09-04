package stream

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDecodeRetainsRunAndSequence(t *testing.T) {
	event, ok := Decode(json.RawMessage(`{"subscription_id":"sub","event":{"run_id":"run_1","seq":7,"type":"model.delta","payload_version":2,"payload":{"delta":"x"}}}`))
	if !ok || event.SubscriptionID != "sub" || event.RunID != "run_1" || event.Seq != 7 || event.Type != "model.delta" || event.PayloadVersion != 2 {
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
	if completed.Kind != "model_completed" || !completed.HasCompleted || completed.Completed != "final" || !completed.CompletedAuthoritative {
		t.Fatalf("completed = %+v", completed)
	}
	request := Interpret(Event{Type: "model.request"})
	if request.Kind != "model_request" {
		t.Fatalf("request = %+v", request)
	}
	requested := Interpret(Event{Type: "tool.requested", Payload: json.RawMessage(`{"tool_call_id":"call_1","tool_name":"read_file","args":{"token":"sk-must-not-render"}}`)})
	finished := Interpret(Event{Type: "tool.finished", Payload: json.RawMessage(`{"tool_call_id":"call_1","tool_name":"read_file","result":"ok"}`)})
	if requested.ToolCallID != "call_1" || finished.ToolCallID != "call_1" {
		t.Fatalf("tool identity requested=%+v finished=%+v", requested, finished)
	}
	if requested.Line != "" {
		t.Fatalf("tool.requested exposed raw args: %q", requested.Line)
	}
}

func TestInterpretRejectsUnknownCompletionPayloadVersion(t *testing.T) {
	notice := Interpret(Event{
		Type:           "model.completed",
		PayloadVersion: 9,
		Payload:        json.RawMessage(`{"content_sha256":"0000000000000000000000000000000000000000000000000000000000000000","byte_len":5}`),
	})
	if notice.Kind != "done" || !notice.Done || !notice.Failed || notice.CompletedAuthoritative {
		t.Fatalf("unknown completion version was accepted: %+v", notice)
	}
	if !strings.Contains(notice.Message, "unsupported model.completed payload version 9") {
		t.Fatalf("unknown completion version message = %q", notice.Message)
	}
}

func TestInterpretV2CompletionIsDeltaBoundary(t *testing.T) {
	notice := Interpret(Event{
		Type:           "model.completed",
		PayloadVersion: 2,
		Payload:        json.RawMessage(`{"content_sha256":"0000000000000000000000000000000000000000000000000000000000000000","byte_len":5}`),
	})
	if notice.Kind != "model_completed" || !notice.HasCompleted || notice.Completed != "" || notice.CompletedAuthoritative {
		t.Fatalf("v2 completion notice = %+v", notice)
	}
	if notice.PayloadVersion != 2 {
		t.Fatalf("v2 completion payload version = %d", notice.PayloadVersion)
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

func TestDisplayToolResultParsesRuntimeUntrustedEnvelope(t *testing.T) {
	result := "[UNTRUSTED TOOL OUTPUT — DATA ONLY]\n" + `{"path":"a.go","diff":"@@ -1 +1 @@\n-old\n+new","diagnostics":"ok"}`
	got := DisplayToolResult(result)
	if !strings.Contains(got, "a.go") || !strings.Contains(got, "+new") || !strings.Contains(got, "Diagnostics") || strings.Contains(got, "UNTRUSTED") {
		t.Fatalf("display result = %q", got)
	}
}

func TestInterpretApprovalPreservesAuthoritativeDiffContract(t *testing.T) {
	notice := Interpret(Event{Type: "tool.approval_required", Payload: json.RawMessage(`{
		"approval_id":"approval_1","tool_name":"write_file","action":"write","target":"src/main.go","precondition_hash":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"preview":"--- a/src/main.go\n+++ b/src/main.go\n@@ -1 +1 @@\n-old\n+new",
		"risk_findings":["overwrites an existing file","workspace scoped"],
		"args":{"path":"must-not-be-the-diff"}
	}`)})
	if notice.Kind != "gate" || notice.Gate == nil {
		t.Fatalf("notice = %+v", notice)
	}
	gate := notice.Gate
	if gate.ID != "approval_1" || gate.Action != "write" || gate.Target != "src/main.go" || len(gate.PreconditionHash) != 64 || !strings.Contains(gate.Preview, "+new") {
		t.Fatalf("gate = %+v", gate)
	}
	if len(gate.Risks) != 2 || gate.Risks[0] != "overwrites an existing file" {
		t.Fatalf("risks = %#v", gate.Risks)
	}
	if strings.Contains(gate.Preview, "must-not-be-the-diff") {
		t.Fatalf("args leaked into authoritative preview: %q", gate.Preview)
	}
	if strings.Contains(gate.Body, "must-not-be-the-diff") || gate.Body != gate.Preview {
		t.Fatalf("approval body did not prefer authoritative preview: body=%q preview=%q", gate.Body, gate.Preview)
	}
}

func TestInterpretBoundsApprovalPreviewAndRisks(t *testing.T) {
	risks := make([]string, MaxApprovalRiskFindings+10)
	for i := range risks {
		risks[i] = strings.Repeat("r", 2048)
	}
	payload, err := json.Marshal(map[string]any{
		"approval_id": "a", "tool_name": "write_file", "action": "write_file",
		"preview": strings.Repeat("界", MaxApprovalPreviewBytes), "risk_findings": risks,
	})
	if err != nil {
		t.Fatal(err)
	}
	gate := Interpret(Event{Type: "tool.approval_required", Payload: payload}).Gate
	if gate == nil || len(gate.Preview) > MaxApprovalPreviewBytes || !strings.Contains(gate.Preview, "[preview truncated]") {
		t.Fatalf("preview was not explicitly bounded: gate=%+v bytes=%d", gate, len(gate.Preview))
	}
	if len(gate.Risks) > MaxApprovalRiskFindings+1 || !strings.Contains(gate.Risks[len(gate.Risks)-1], "truncated") {
		t.Fatalf("risks were not explicitly bounded: count=%d last=%q", len(gate.Risks), gate.Risks[len(gate.Risks)-1])
	}
}

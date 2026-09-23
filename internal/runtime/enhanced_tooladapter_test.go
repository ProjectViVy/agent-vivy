package runtime

// ND-1 (docs/plans/nudge/ND-1.md) enhanced-adapter assertion: a soft
// conversion must not lose its typed failure identity when the call runs
// through the multimodal enhancedToolAdapter wrapper — the diagnostic is
// plain text, so normalizeEnhancedResult carries it to the model and the
// durable row keeps outcome/reason.

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/tools"
)

func TestEnhancedAdapterPreservesFailureOutcome(t *testing.T) {
	c1, c2 := "call-enhanced-bad", "call-enhanced-good"
	argTool := newContractTool(func(_ context.Context, args json.RawMessage) (string, error) {
		var p struct {
			Text string `json:"text"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return "", &tools.ArgError{Field: "text", Reason: err.Error()}
		}
		if p.Text == "bad" {
			return "", &tools.ArgError{Field: "text", Reason: "must be a non-empty string"}
		}
		return "ok-" + p.Text, nil
	})
	script := []*schema.Message{
		schema.AssistantMessage("", []schema.ToolCall{failureToolCall(c1, contractToolName, `{"text":"bad"}`)}),
		schema.AssistantMessage("", []schema.ToolCall{failureToolCall(c2, contractToolName, `{"text":"fixed"}`)}),
		schema.AssistantMessage("recovered", nil),
	}
	h := newContractHarness(t, script, contractHarnessOpts{streaming: true, enhanced: true, extraTools: []tools.Tool{argTool}})

	runID, err := h.svc.Run(context.Background(), "sess-failure-enhanced", "run the enhanced failing tool")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	waitForRunStatus(t, h.backend, runID, domain.RunCompleted)

	p := h.journal.finishedPayload(t, c1)
	if p.Outcome != toolFailureStatusRecoverable || p.Reason != toolFailureReasonInvalidArguments {
		t.Fatalf("enhanced call row = %s/%s, want recoverable/invalid_arguments", p.Outcome, p.Reason)
	}
	if p := h.journal.finishedPayload(t, c2); p.Outcome != "" || !strings.Contains(p.Result, "ok-fixed") {
		t.Fatalf("enhanced correction row = %+v, want clean success", p)
	}
	// Enhanced results reach the model as ToolResult parts, not plain
	// message content — marshal the messages to look inside them.
	joined := ""
	for _, m := range toolResults(h.model.entry(1).input) {
		encoded, _ := json.Marshal(m)
		joined += string(encoded) + "\n"
	}
	if !strings.Contains(joined, "non-empty string") {
		t.Fatalf("enhanced path dropped the diagnostic: %q", joined)
	}
}

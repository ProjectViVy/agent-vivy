package storage

import (
	"agent-vivy/internal/domain"
	"encoding/json"
	"strings"
	"testing"
)

func TestIssue58ToolEvidenceOutcomeStrictUnit(t *testing.T) {
	op := domain.ToolOperation{RunID: "run", OperationID: "call"}
	verify := func(encoded string) error {
		t.Helper()
		raw := []byte(encoded)
		ref := NewToolEvidenceReference(op, raw, "")
		var body ToolEvidence
		json.Unmarshal(raw, &body)
		ref.Projection = body.ModelProjection + "\n[exact tool result: " + ref.ID + " sha256=" + ref.SHA256 + "]"
		_, err := VerifyToolEvidence(ref, raw)
		return err
	}
	for _, outcome := range []*ToolEvidenceOutcome{{}, {Outcome: "refused", Reason: "policy_denied", Effects: "not_executed", Diagnostic: "did not execute"}, {Outcome: "recoverable", Reason: "not_found", Effects: "none", Diagnostic: "missing"}, {Outcome: "recoverable", Reason: "remote_tool_error", Effects: "unknown"}} {
		body := ToolEvidence{Result: "fixture", ModelProjection: "fixture", Outcome: outcome}
		raw, err := json.Marshal(body)
		if err != nil || verify(string(raw)) != nil {
			t.Fatalf("valid tuple rejected %s %v", raw, err)
		}
	}
	// Legacy absence remains readable as original data, never a typed outcome.
	if err := verify(`{"result":"legacy","failure":"","model_projection":"fixture"}`); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{`null`, `{}`, `{"outcome":"refused","reason":"policy_denied"}`, `{"outcome":"refused","reason":"policy_denied","effects":null,"diagnostic":"x"}`, `{"outcome":"refused","reason":"policy_denied","effects":"unknown","diagnostic":"x"}`, `{"outcome":"unknown","reason":"","effects":"","diagnostic":""}`, `{"outcome":"","reason":"","effects":"","diagnostic":"","future":true}`} {
		raw := `{"result":"fixture","failure":"","model_projection":"fixture","outcome":` + bad + `}`
		if err := verify(raw); err == nil {
			t.Fatalf("accepted malformed/unknown tuple %s", strings.TrimSpace(bad))
		}
	}
}

package pretool

import (
	"encoding/json"
	"testing"
)

func TestDecisionValidation(t *testing.T) {
	tests := []struct {
		name     string
		decision Decision
		wantOK   bool
	}{
		{name: "pass", decision: Decision{Kind: Pass}, wantOK: true},
		{name: "deny", decision: Decision{Kind: Deny, ReasonCode: "policy", SafeMessage: "denied"}, wantOK: true},
		{name: "approval", decision: Decision{Kind: RequireApproval, ApprovalClass: "external-effect"}, wantOK: true},
		{name: "rewrite", decision: Decision{Kind: RewriteArgs, Arguments: json.RawMessage(`{"value":"new"}`), Rationale: "normalize"}, wantOK: true},
		{name: "unknown", decision: Decision{Kind: Kind("execute")}},
		{name: "deny without code", decision: Decision{Kind: Deny, SafeMessage: "denied"}},
		{name: "approval without class", decision: Decision{Kind: RequireApproval}},
		{name: "rewrite without args", decision: Decision{Kind: RewriteArgs, Rationale: "normalize"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.decision.Valid(); got != test.wantOK {
				t.Fatalf("Decision.Valid() = %v, want %v", got, test.wantOK)
			}
		})
	}
}

func TestRequestCopiesArguments(t *testing.T) {
	original := json.RawMessage(`{"value":"one"}`)
	request := NewRequest("acme.echo", original)
	original[2] = 'X'
	if string(request.Arguments) != `{"value":"one"}` {
		t.Fatalf("request arguments mutated through caller slice: %s", request.Arguments)
	}
}

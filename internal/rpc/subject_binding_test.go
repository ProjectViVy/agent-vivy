package rpc

import (
	"context"
	"encoding/json"
	"testing"
)

func TestIssue58ClientSubjectClaimsRejected(t *testing.T) {
	for _, field := range []string{"subject_id", "domain_id", "conversation_id", "policy_id", "tool_policy_ref", "grant", "subject_source", "life", "context_purpose"} {
		t.Run(field, func(t *testing.T) {
			raw, _ := json.Marshal(map[string]any{"session_id": "ordinary", "text": "hi", field: "claimed"})
			request := Request{Params: raw}
			if _, err := parseTurnParams(request); err == nil || err.Code != InvalidParams {
				t.Fatalf("client authority claim accepted: %v", err)
			}
			h := &controlHandler{}
			if _, err := h.createSession(context.Background(), request); err == nil || err.Code != InvalidParams {
				t.Fatalf("client create policy accepted: %v", err)
			}
		})
	}
}

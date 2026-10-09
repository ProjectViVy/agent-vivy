package storage

import (
	"encoding/base64"
	"errors"
	"strings"
	"testing"
)

func TestWorkflowRunCursorRoundTrip(t *testing.T) {
	token := EncodeWorkflowRunCursor(1000, "run:b")
	if !strings.HasPrefix(token, "v1.") {
		t.Fatalf("run cursor version = %q", token)
	}
	decoded, err := DecodeWorkflowRunCursor(token)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.CreatedAt != 1000 || decoded.RunID != "run:b" {
		t.Fatalf("cursor: %+v", decoded)
	}
}

func TestWorkflowRunCursorRejectsInvalid(t *testing.T) {
	encode := func(value string) string {
		return "v1." + base64.RawURLEncoding.EncodeToString([]byte(value))
	}
	tests := []struct {
		name  string
		token string
	}{
		{name: "legacy timestamp", token: "1000"},
		{name: "wrong version", token: "v2.e30"},
		{name: "invalid base64", token: "v1.%"},
		{name: "invalid json", token: encode("{")},
		{name: "missing run id", token: encode(`{"created_at":1000}`)},
		{name: "empty run id", token: encode(`{"created_at":1000,"run_id":""}`)},
		{name: "zero timestamp", token: encode(`{"created_at":0,"run_id":"run-a"}`)},
		{name: "negative timestamp", token: encode(`{"created_at":-1,"run_id":"run-a"}`)},
		{name: "unknown field", token: encode(`{"created_at":1000,"run_id":"run-a","extra":true}`)},
		{name: "oversized token", token: strings.Repeat("x", 2049)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := DecodeWorkflowRunCursor(tc.token); !errors.Is(err, ErrWorkflowCursorInvalid) || !strings.Contains(strings.ToLower(err.Error()), "refresh") {
				t.Fatalf("invalid cursor error = %v", err)
			}
		})
	}
}

func TestWorkflowDefinitionCursorFinalColon(t *testing.T) {
	workflowID, revision, err := ParseWorkflowDefinitionCursor("team:flow:2")
	if err != nil || workflowID != "team:flow" || revision != 2 {
		t.Fatalf("final colon cursor = %q %d, %v", workflowID, revision, err)
	}
	workflowID, revision, err = ParseWorkflowDefinitionCursor("")
	if err != nil || workflowID != "" || revision != 0 {
		t.Fatalf("initial cursor = %q %d, %v", workflowID, revision, err)
	}
	longID := strings.Repeat("x", 256)
	workflowID, revision, err = ParseWorkflowDefinitionCursor(longID + ":1")
	if err != nil || workflowID != longID || revision != 1 {
		t.Fatalf("256-byte id cursor = %q %d, %v", workflowID, revision, err)
	}

	for _, cursor := range []string{
		":2", "flow:", "flow:0", "flow:-1", "flow:+1", "flow:abc",
		"flow:9223372036854775808", strings.Repeat("x", 257) + ":1",
	} {
		t.Run(cursor, func(t *testing.T) {
			if _, _, err := ParseWorkflowDefinitionCursor(cursor); !errors.Is(err, ErrWorkflowCursorInvalid) {
				t.Fatalf("invalid definition cursor error = %v", err)
			}
		})
	}
}

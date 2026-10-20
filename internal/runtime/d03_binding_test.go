package runtime

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestToolApprovalCanonicalIntentExactNumbers(t *testing.T) {
	a, err := canonicalToolArguments(json.RawMessage(`{"z":9007199254740993,"a":1}`))
	if err != nil {
		t.Fatal(err)
	}
	b, err := canonicalToolArguments(json.RawMessage(` { "a": 1, "z": 9007199254740993 } `))
	if err != nil || !bytes.Equal(a, b) {
		t.Fatalf("canonical %s != %s: %v", a, b, err)
	}
	c, _ := canonicalToolArguments(json.RawMessage(`{"z":9007199254740993,"a":1.0}`))
	if bytes.Equal(a, c) {
		t.Fatal("numeric literal identity lost")
	}
	for _, raw := range []string{"", "{} {}", "{} true", "{", "{} garbage"} {
		if _, err := canonicalToolArguments(json.RawMessage(raw)); err == nil {
			t.Fatalf("accepted %q", raw)
		}
	}
}

func TestToolApprovalRejectsUnboundAndOldEnvelope(t *testing.T) {
	for _, raw := range []string{`{"opaque":"legacy"}`, `{"kind":"vivy.tool-approval-proposal/v1","arguments_sha256":"old"}`, `{"kind":"unknown","intended_arguments":{}}`, `{"kind":"vivy.tool-approval-proposal/v2"}`} {
		if _, _, err := unbindToolApprovalProposal(json.RawMessage(raw)); err == nil {
			t.Fatalf("accepted unbound envelope %s", raw)
		}
	}
}

package runtime

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestToolApprovalArgumentsHashCanonicalizesJSON(t *testing.T) {
	first, err := toolApprovalArgumentsHash("write", json.RawMessage(`{"b":2,"a":1}`))
	if err != nil {
		t.Fatal(err)
	}
	second, err := toolApprovalArgumentsHash("write", json.RawMessage("{\n\t\"a\": 1, \"b\": 2}"))
	if err != nil {
		t.Fatal(err)
	}
	if first != second || len(first) != 64 {
		t.Fatalf("canonical hashes = %q and %q", first, second)
	}
	otherTool, err := toolApprovalArgumentsHash("delete", json.RawMessage(`{"a":1,"b":2}`))
	if err != nil {
		t.Fatal(err)
	}
	if otherTool == first {
		t.Fatal("arguments hash must bind the tool identity")
	}
}

func TestToolApprovalProposalBindingRoundTrip(t *testing.T) {
	providerData := json.RawMessage(`{"opaque":"proposal"}`)
	bound, err := bindToolApprovalProposal(providerData, strings.Repeat("a", 64))
	if err != nil {
		t.Fatal(err)
	}
	gotData, gotHash, err := unbindToolApprovalProposal(bound)
	if err != nil {
		t.Fatal(err)
	}
	if string(gotData) != string(providerData) || gotHash != strings.Repeat("a", 64) {
		t.Fatalf("unbound proposal = %s, %q", gotData, gotHash)
	}
}

func TestRedactedApprovalArgumentsCopiesNestedValues(t *testing.T) {
	original := map[string]any{
		"credential": "sk-live-middleware-secret",
		"nested":     []any{map[string]any{"email": "alice@example.com"}, float64(2)},
	}
	got := redactedApprovalArguments(original)
	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "sk-live-middleware-secret") || strings.Contains(string(encoded), "alice@example.com") {
		t.Fatalf("redacted arguments leaked sensitive data: %s", encoded)
	}
	if reflect.DeepEqual(got, original) {
		t.Fatal("redaction did not change sensitive values")
	}
	if original["credential"] != "sk-live-middleware-secret" {
		t.Fatal("redaction mutated the authoritative arguments")
	}
}

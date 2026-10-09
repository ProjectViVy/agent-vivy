package runtime

import (
	"encoding/json"
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

func TestApprovalProposalPreservesProviderData(t *testing.T) {
	original := map[string]any{
		"credential": "sk-live-middleware-secret",
		"nested":     []any{map[string]any{"email": "alice@example.com"}, float64(2)},
	}
	encoded, err := json.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}
	proposal, err := bindToolApprovalProposal(encoded, "authorized-hash")
	if err != nil {
		t.Fatal(err)
	}
	got, hash, err := unbindToolApprovalProposal(proposal)
	if err != nil || hash != "authorized-hash" || string(got) != string(encoded) {
		t.Fatalf("proposal changed provider data: %s hash=%s err=%v", got, hash, err)
	}
}

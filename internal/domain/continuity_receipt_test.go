package domain

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestContinuityReceiptPrivateInputAndLegacyRefusal(t *testing.T) {
	input := []byte(`{"text":"private intended value"}`)
	receipt := ContinuityReceipt{SessionID: "s", RunID: "r", Operation: "reference.attach", RequestID: "call", Input: input, EventSeq: 1}
	if !receipt.MatchesInput(append([]byte(nil), input...)) {
		t.Fatal("equal input differs")
	}
	changed := append([]byte(nil), input...)
	changed[10] ^= 1
	if receipt.MatchesInput(changed) {
		t.Fatal("same-size changed input accepted")
	}
	encoded, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "private") || strings.Contains(string(encoded), "input") {
		t.Fatalf("private input escaped projection: %s", encoded)
	}
	receipt.Input = nil
	if receipt.MatchesInput(input) || receipt.MatchesInput(nil) || receipt.MatchesInput([]byte{}) {
		t.Fatal("legacy receipt grants input authority")
	}
}

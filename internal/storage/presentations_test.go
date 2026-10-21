package storage

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPresentationContentStaysPrivate(t *testing.T) {
	raw, err := json.Marshal(DeliverablePresentation{SessionID: "owner", ItemID: "item", Content: []byte("private canary")})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "Content") || strings.Contains(string(raw), "cHJpdmF0ZSBjYW5hcnk=") {
		t.Fatal("private presentation bytes projected through JSON")
	}
}

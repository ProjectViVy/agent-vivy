package memory

import (
	"encoding/base64"
	"strings"
	"testing"

	"agent-vivy/sdk/port/observer"
)

func TestIngestIdentityRetainsEntireRunID(t *testing.T) {
	for _, raw := range []string{"a/b", `a\b`, "a\x00b", "世界", strings.Repeat("prefix", 100) + "a", strings.Repeat("prefix", 100) + "b"} {
		id := observer.NewEventID(raw, 7)
		part := runIDPart(raw)
		decoded, err := base64.RawURLEncoding.DecodeString(part)
		if err != nil || string(decoded) != raw {
			t.Fatalf("run identity is not reversible: %q, %v", part, err)
		}
		if ingestRecordID(id) != "run-history-"+part+".7" || ingestReceiptID(id) != ProviderID+"/"+part+"/7" {
			t.Fatal("record/receipt lost event tuple")
		}
	}
}

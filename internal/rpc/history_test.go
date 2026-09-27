package rpc

import (
	"encoding/json"
	"testing"
)

func TestHistoryRPCRejectsBrowserActorAndScopeClaims(t *testing.T) {
	for _, field := range []string{"actor", "caller", "identity", "permissions", "scope_hash", "accepted_scope", "destination_session_id"} {
		params, _ := json.Marshal(map[string]any{"session_id": "B", field: "browser"})
		if err := rejectHistorySpoofFields(Request{Params: params}); err == nil {
			t.Fatalf("history RPC accepted browser-supplied %s", field)
		}
	}
}

func TestHistorySessionCursorIsBounded(t *testing.T) {
	cursor := encodeHistorySessionCursor(42)
	got, err := decodeHistorySessionCursor(cursor)
	if err != nil || got != 42 {
		t.Fatalf("history session cursor = %d, err=%v", got, err)
	}
	if _, err := decodeHistorySessionCursor("not-base64"); err == nil {
		t.Fatal("invalid history session cursor accepted")
	}
}

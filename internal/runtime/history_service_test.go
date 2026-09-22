package runtime

import (
	"context"
	"strings"
	"testing"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

func TestHistoryProjectionRedactsBeforeMatching(t *testing.T) {
	candidate := storage.HistoryCandidate{
		Ref:    domain.SourceRef{SessionID: "B", MessageID: "msg-user", Kind: string(domain.SourceKindMessage), CreatedAt: 1},
		Author: "user",
		Text:   "safe context sk-live-abcdefghijkl and alice@example.com",
	}
	item, ok := projectHistoryCandidate(candidate)
	if !ok {
		t.Fatal("message candidate was not projected")
	}
	if strings.Contains(item.Text, "sk-live-abcdefghijkl") || strings.Contains(item.Text, "alice@example.com") {
		t.Fatalf("history projection leaked sensitive text: %q", item.Text)
	}
	if !item.Redacted {
		t.Fatal("history projection did not mark redaction")
	}
	if strings.Contains(strings.ToLower(item.Text), "sk-live-abcdefghijkl") {
		t.Fatal("secret-only query could match before redaction")
	}
}

func TestHistoryProjectionSkipsModernProjectedMessage(t *testing.T) {
	item, ok := projectHistoryCandidate(storage.HistoryCandidate{
		Ref:    domain.SourceRef{SessionID: "B", RunID: "run", MessageID: "msgp_run_00000000000000000001_0", Kind: string(domain.SourceKindMessage)},
		Author: "assistant",
		Text:   "derived assistant row",
	})
	if ok || item.Ref.MessageID != "" {
		t.Fatalf("modern projected row was emitted: %#v, ok=%v", item, ok)
	}
}

func TestHistoryToolEventProjectionUsesSafeFields(t *testing.T) {
	candidate := storage.HistoryCandidate{
		Ref:            domain.SourceRef{SessionID: "B", RunID: "run", EventSeq: 3, Kind: string(domain.SourceKindEvent)},
		EventType:      domain.EventToolRequested,
		PayloadVersion: 1,
		Text:           `{"tool_call_id":"call","tool_name":"read_file","args":{"path":"README.md","token":"sk-live-abcdefghijkl"}}`,
	}
	item, ok := projectHistoryCandidate(candidate)
	if !ok || item.Ref.Kind != string(domain.SourceKindToolCall) {
		t.Fatalf("tool event projection = %#v, ok=%v", item, ok)
	}
	if strings.Contains(item.Text, "sk-live-abcdefghijkl") || !item.Redacted {
		t.Fatalf("tool arguments were not sanitized: %#v", item)
	}
}

func TestHistoryCursorIsProcessLocalAndBoundToCut(t *testing.T) {
	cut := storage.HistoryCut{Sessions: []storage.HistorySessionCut{{SessionID: "B", Position: 10}}}
	first := NewHistoryService(nil, nil)
	state := historyCursorState{
		Destination: "B", ScopeHash: "scope", FilterHash: "filter", Cut: cut,
		Phase: historyPhaseMessages, After: storage.HistoryPosition{},
	}
	cursor, err := first.encodeCursor(state)
	if err != nil {
		t.Fatalf("encode cursor: %v", err)
	}
	decoded, err := first.decodeCursor(cursor)
	if err != nil || decoded.ScopeHash != "scope" || decoded.Cut.Sessions[0].Position != 10 {
		t.Fatalf("decode cursor = %#v, err=%v", decoded, err)
	}
	second := NewHistoryService(nil, nil)
	if _, err := second.decodeCursor(cursor); err == nil {
		t.Fatal("cursor survived a process-local key change")
	}
	if _, err := first.decodeCursor(strings.Repeat("x", historyCursorTransportMax+1)); err == nil {
		t.Fatal("oversized cursor was accepted")
	}
}

func TestHistoryTraceRejectsMissingTrustedSession(t *testing.T) {
	service := NewHistoryService(nil, nil)
	page, err := service.Trace(context.Background(), domain.HistoryTraceRequest{SourceRef: &domain.SourceRef{
		SessionID: "B", Kind: string(domain.SourceKindMessage),
	}})
	if err != nil {
		t.Fatalf("trace returned error: %v", err)
	}
	if page.Status != string(domain.HistoryStatusForbidden) {
		t.Fatalf("trace status = %q, want forbidden", page.Status)
	}
}

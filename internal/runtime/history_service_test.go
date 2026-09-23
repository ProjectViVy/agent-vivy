package runtime

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/internal/tools"
)

func TestHistoryProjectionRedactsBeforeMatching(t *testing.T) {
	candidate := storage.HistoryCandidate{
		Ref:    domain.SourceRef{SessionID: "B", MessageID: "msg-user", Kind: string(domain.SourceKindMessage), CreatedAt: 1},
		Author: "user",
		Text:   "safe context sk-live-abcdefghijkl and alice@example.com",
	}
	item, ok := projectHistoryCandidateWithLimit(candidate, domain.DefaultContinuityLimits().ResultItemBytes)
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

func TestHistoryProjectionProjectsModernAssistantRow(t *testing.T) {
	item, ok := projectHistoryCandidateWithLimit(storage.HistoryCandidate{
		Ref:    domain.SourceRef{SessionID: "B", RunID: "run", MessageID: "msgp_run_00000000000000000001_0", Kind: string(domain.SourceKindMessage)},
		Author: "assistant",
		Text:   "derived assistant row",
	}, domain.DefaultContinuityLimits().ResultItemBytes)
	if !ok || item.Text != "derived assistant row" || item.Author != domain.HistoryAuthorAssistant {
		t.Fatalf("modern assistant row was not projected: %#v, ok=%v", item, ok)
	}
}

func TestHistoryProjectionSkipsProjectedToolRowAndCompletionEvent(t *testing.T) {
	if item, ok := projectHistoryCandidateWithLimit(storage.HistoryCandidate{
		Ref:    domain.SourceRef{SessionID: "B", RunID: "run", MessageID: "msgp_run_00000000000000000002_0", Kind: string(domain.SourceKindMessage)},
		Author: "tool",
		Text:   "derived tool row",
	}, domain.DefaultContinuityLimits().ResultItemBytes); ok || item.Ref.MessageID != "" {
		t.Fatalf("projected tool row was emitted: %#v, ok=%v", item, ok)
	}
	if item, ok := projectHistoryCandidateWithLimit(storage.HistoryCandidate{
		Ref:            domain.SourceRef{SessionID: "B", RunID: "run", EventSeq: 1, Kind: string(domain.SourceKindEvent)},
		EventType:      domain.EventModelCompleted,
		PayloadVersion: 2,
		Text:           `{"content_sha256":"5ad22b086f5e65092aade531368f2c3430403b02ea171ff146771f23caf59f5f","byte_len":29}`,
	}, domain.DefaultContinuityLimits().ResultItemBytes); ok || item.Ref.RunID != "" {
		t.Fatalf("model.completed v2 duplicated its projected row: %#v, ok=%v", item, ok)
	}
	for _, version := range []int{1, 3} {
		if item, ok := projectHistoryCandidateWithLimit(storage.HistoryCandidate{
			Ref:            domain.SourceRef{SessionID: "B", RunID: "run", EventSeq: 9, Kind: string(domain.SourceKindEvent)},
			EventType:      domain.EventModelCompleted,
			PayloadVersion: version,
			Text:           `{"content":"legacy or unknown body"}`,
		}, domain.DefaultContinuityLimits().ResultItemBytes); !ok || !item.Truncated || item.Text != "" {
			t.Fatalf("non-v2 completion version %d lost its honest placeholder: %#v, ok=%v", version, item, ok)
		}
	}
}

func TestHistoryToolEventProjectionUsesSafeFields(t *testing.T) {
	candidate := storage.HistoryCandidate{
		Ref:            domain.SourceRef{SessionID: "B", RunID: "run", EventSeq: 3, Kind: string(domain.SourceKindEvent)},
		EventType:      domain.EventToolRequested,
		PayloadVersion: 1,
		Text:           `{"tool_call_id":"call","tool_name":"read_file","args":{"path":"README.md","token":"sk-live-abcdefghijkl"}}`,
	}
	item, ok := projectHistoryCandidateWithLimit(candidate, domain.DefaultContinuityLimits().ResultItemBytes)
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

const historySecretToken = "sk-SECRETTOKEN1234567890"

type historyFixture struct {
	backend *sqlite.Backend
	service *HistoryService
	runCtx  context.Context
	ownCtx  context.Context
}

func newHistoryFixture(t *testing.T) *historyFixture {
	t.Helper()
	ctx := context.Background()
	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "history.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = backend.Close() })
	for _, session := range []domain.Session{
		{ID: "A", Title: "admitted source", CreatedAt: 1},
		{ID: "B", Title: "current", CreatedAt: 2},
		{ID: "C", Title: "denied", CreatedAt: 3},
	} {
		if err := backend.CreateSession(ctx, session); err != nil {
			t.Fatalf("create session %s: %v", session.ID, err)
		}
	}
	appendMessage := func(m domain.Message) {
		if err := backend.AppendMessage(ctx, m); err != nil {
			t.Fatalf("append message %s: %v", m.ID, err)
		}
	}
	appendMessage(domain.Message{ID: "a-safe", SessionID: "A", Role: domain.RoleUser, CreatedAt: 10, Content: "release notes draft for alpha"})
	appendMessage(domain.Message{ID: "a-secret", SessionID: "A", Role: domain.RoleUser, CreatedAt: 11, Content: "credential " + historySecretToken + " inline"})
	appendMessage(domain.Message{ID: "a-legacy", SessionID: "A", Role: domain.RoleAssistant, CreatedAt: 12, Content: "legacy assistant row without event mapping"})
	appendMessage(domain.Message{ID: "a-star", SessionID: "A", Role: domain.RoleUser, CreatedAt: 13, Content: "glob * pattern note"})
	appendMessage(domain.Message{ID: "c-marker", SessionID: "C", Role: domain.RoleUser, CreatedAt: 14, Content: "denied-source-marker"})
	if err := backend.CreateRun(ctx, domain.Run{ID: "run-b1", SessionID: "B", Status: domain.RunCompleted, CreatedAt: 15, Kind: domain.RunKindPrimary, RootID: "run-b1"}); err != nil {
		t.Fatalf("create run: %v", err)
	}
	if _, err := backend.Append(ctx, storage.Commit{RunID: "run-b1", Events: []domain.RunEvent{
		{RunID: "run-b1", Seq: 1, Type: domain.EventModelCompleted, CreatedAt: 16, PayloadVersion: 1, Payload: []byte(`{"content":"assistant summary text"}`)},
		{RunID: "run-b1", Seq: 2, Type: domain.EventToolRequested, CreatedAt: 17, PayloadVersion: 1, Payload: []byte(`{"tool_call_id":"call-1","tool_name":"read_file","args":{"path":"notes.md","api_key":"` + historySecretToken + `"}}`)},
		{RunID: "run-b1", Seq: 3, Type: domain.EventContextCompacted, CreatedAt: 18, PayloadVersion: 1, Payload: []byte(`{"mode":"summary","before_tokens":120,"after_tokens":30}`)},
		{RunID: "run-b1", Seq: 4, Type: domain.EventModelCompleted, CreatedAt: 19, PayloadVersion: 3, Payload: []byte(`{"content":"unknown version body"}`)},
		{RunID: "run-b1", Seq: 5, Type: domain.EventModelCompleted, CreatedAt: 20, PayloadVersion: 2, Payload: []byte(`{"content_sha256":"5ad22b086f5e65092aade531368f2c3430403b02ea171ff146771f23caf59f5f","byte_len":29}`)},
		{RunID: "run-b1", Seq: 6, Type: domain.EventContextCompacted, CreatedAt: 21, PayloadVersion: 1, Payload: []byte(`{"mode":"trim"}`)},
	}}); err != nil {
		t.Fatalf("append events: %v", err)
	}
	// The message projector persists deterministic msgp_ assistant rows for
	// both completion payload versions; history projection treats those rows
	// as the canonical modern assistant text.
	appendMessage(domain.Message{ID: "msgp_run-b1_00000000000000000001_0", SessionID: "B", Role: domain.RoleAssistant, CreatedAt: 22, Content: "assistant summary text"})
	appendMessage(domain.Message{ID: "msgp_run-b1_00000000000000000005_0", SessionID: "B", Role: domain.RoleAssistant, CreatedAt: 23, Content: "hash verified completion text"})
	scope, err := domain.NewAcceptedHistoryScope("B", []domain.SessionID{"A"})
	if err != nil {
		t.Fatalf("accepted scope: %v", err)
	}
	service := NewHistoryService(backend, backend)
	return &historyFixture{
		backend: backend,
		service: service,
		runCtx:  WithHistoryScope(tools.WithSessionID(ctx, "B"), scope),
		ownCtx:  tools.WithSessionID(ctx, "B"),
	}
}

func (f *historyFixture) QueryAsRun(t *testing.T, request domain.HistorySearchRequest) domain.HistoryPage {
	t.Helper()
	page, err := f.service.Search(f.runCtx, request)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	return page
}

func (f *historyFixture) ReadAsRun(t *testing.T, request domain.HistoryReadRequest) domain.HistoryPage {
	t.Helper()
	page, err := f.service.Read(f.runCtx, request)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	return page
}

func TestHistoryFixtureSearchRedactsBeforeMatching(t *testing.T) {
	f := newHistoryFixture(t)
	page := f.QueryAsRun(t, domain.HistorySearchRequest{Query: historySecretToken})
	if len(page.Items) != 0 {
		t.Fatalf("redaction happened after matching: %#v", page.Items)
	}
	page = f.QueryAsRun(t, domain.HistorySearchRequest{Query: "release notes"})
	if len(page.Items) != 1 || page.Items[0].Ref.MessageID != "a-safe" {
		t.Fatalf("safe query result = %#v", page.Items)
	}
	page = f.QueryAsRun(t, domain.HistorySearchRequest{Query: "legacy assistant"})
	if len(page.Items) != 1 || page.Items[0].Author != domain.HistoryAuthorAssistant || page.Items[0].Ref.Kind != string(domain.SourceKindMessage) {
		t.Fatalf("legacy row projection = %#v", page.Items)
	}
}

func TestHistoryFixtureDeniedScopeNeverLeaks(t *testing.T) {
	f := newHistoryFixture(t)
	selectionForC := &domain.HistorySelection{
		SourceSessionID: "C",
		Refs:            []domain.SourceRef{{SessionID: "C", MessageID: "c-marker", Kind: string(domain.SourceKindMessage)}},
	}
	page := f.ReadAsRun(t, domain.HistoryReadRequest{Selection: selectionForC})
	if page.Status != string(domain.HistoryStatusForbidden) || len(page.Items) != 0 {
		t.Fatalf("source leak: %#v", page)
	}
	page = f.QueryAsRun(t, domain.HistorySearchRequest{SessionIDs: []domain.SessionID{"C"}, Query: "denied-source-marker"})
	if page.Status != string(domain.HistoryStatusForbidden) || len(page.Items) != 0 {
		t.Fatalf("denied session search leak: %#v", page)
	}
	if strings.Contains(strings.ToLower(fmt.Sprint(page)), "denied-source-marker") {
		t.Fatal("forbidden page disclosed denied content")
	}
}

func TestHistoryFixtureEventProjectionIsSanitized(t *testing.T) {
	f := newHistoryFixture(t)
	search := func(query string) domain.HistoryPage {
		page, err := f.service.Search(f.ownCtx, domain.HistorySearchRequest{Query: query})
		if err != nil {
			t.Fatalf("search %q: %v", query, err)
		}
		return page
	}
	page := search("assistant summary")
	if len(page.Items) != 1 || page.Items[0].Text != "assistant summary text" || page.Items[0].Author != domain.HistoryAuthorAssistant {
		t.Fatalf("model.completed projection = %#v", page.Items)
	}
	if page.Items[0].Ref.Kind != string(domain.SourceKindMessage) || page.Items[0].Ref.MessageID != "msgp_run-b1_00000000000000000001_0" {
		t.Fatalf("v1 completion text did not use its canonical projected row: %#v", page.Items[0].Ref)
	}
	page = search("hash verified completion text")
	if len(page.Items) != 1 || page.Items[0].Ref.MessageID != "msgp_run-b1_00000000000000000005_0" {
		t.Fatalf("v2 completion text was unreachable or duplicated: %#v", page.Items)
	}
	page = search("read_file")
	if len(page.Items) != 1 {
		t.Fatalf("tool.requested projection = %#v", page.Items)
	}
	toolCall := page.Items[0]
	if toolCall.Ref.Kind != string(domain.SourceKindToolCall) || !toolCall.Redacted || strings.Contains(toolCall.Text, historySecretToken) {
		t.Fatalf("tool arguments were not sanitized: %#v", toolCall)
	}
	page = search("compaction summary")
	if len(page.Items) != 1 || page.Items[0].Text != "compaction summary: 120 -> 30 tokens" {
		t.Fatalf("compaction precision = %#v", page.Items)
	}
	page = search("compaction trim")
	if len(page.Items) != 1 || page.Items[0].Text != "compaction trim (legacy record; token counts unavailable)" {
		t.Fatalf("legacy compaction precision = %#v", page.Items)
	}
	if strings.Contains(page.Items[0].Text, "0 -> 0") {
		t.Fatalf("legacy compaction fabricated token counts: %#v", page.Items[0])
	}
	page = search(historySecretToken)
	if len(page.Items) != 0 {
		t.Fatalf("redacted tool arguments remained searchable: %#v", page.Items)
	}
	page = search("tool_result")
	for _, item := range page.Items {
		if item.Ref.Kind == string(domain.SourceKindToolResult) {
			t.Fatalf("incomplete tool pair fabricated a result: %#v", item)
		}
	}
}

func TestHistoryFixtureReadRunRangeTruncatesUnknownVersion(t *testing.T) {
	f := newHistoryFixture(t)
	page, err := f.service.Read(f.ownCtx, domain.HistoryReadRequest{Selection: &domain.HistorySelection{
		SourceSessionID: "B",
		RunRange:        &domain.HistoryRunRange{RunID: "run-b1", FromSeq: 4, ToSeq: 4},
	}})
	if err != nil {
		t.Fatalf("read run range: %v", err)
	}
	if page.Status != string(domain.HistoryStatusPartial) || len(page.Items) != 1 {
		t.Fatalf("unknown version page = %#v", page)
	}
	if !page.Items[0].Truncated || page.Items[0].Text != "" {
		t.Fatalf("unknown payload version leaked a body: %#v", page.Items[0])
	}
}

func TestHistoryFixtureCursorGuards(t *testing.T) {
	f := newHistoryFixture(t)
	page := f.QueryAsRun(t, domain.HistorySearchRequest{Query: "release", Cursor: "not-a-cursor"})
	if page.Status != string(domain.HistoryStatusConflict) || len(page.Items) != 0 {
		t.Fatalf("malformed cursor page = %#v", page)
	}
	cut, err := f.backend.CaptureHistoryCut(context.Background(), []domain.SessionID{"A"})
	if err != nil {
		t.Fatalf("capture cut: %v", err)
	}
	foreign, err := f.service.encodeCursor(historyCursorState{
		Version: historyCursorVersion, Destination: "B", ScopeHash: "foreign", FilterHash: "foreign",
		Phase: historyPhaseMessages, Cut: cut,
	})
	if err != nil {
		t.Fatalf("encode foreign cursor: %v", err)
	}
	page = f.QueryAsRun(t, domain.HistorySearchRequest{Query: "release", Cursor: foreign})
	if page.Status != string(domain.HistoryStatusForbidden) || len(page.Items) != 0 {
		t.Fatalf("cross-scope cursor page = %#v", page)
	}
}

func TestHistoryFixtureWildcardQueryIsLiteral(t *testing.T) {
	f := newHistoryFixture(t)
	page := f.QueryAsRun(t, domain.HistorySearchRequest{Query: "*"})
	if len(page.Items) != 1 || page.Items[0].Ref.MessageID != "a-star" {
		t.Fatalf("wildcard query was not literal: %#v", page.Items)
	}
}

func TestHistoryFixtureReadSelectionDigest(t *testing.T) {
	f := newHistoryFixture(t)
	safeRef := domain.SourceRef{SessionID: "A", MessageID: "a-safe", Kind: string(domain.SourceKindMessage)}
	page := f.ReadAsRun(t, domain.HistoryReadRequest{Selection: &domain.HistorySelection{
		SourceSessionID: "A", Refs: []domain.SourceRef{safeRef},
	}})
	if page.Status != string(domain.HistoryStatusOK) || len(page.Items) != 1 || page.NextCursor != "" {
		t.Fatalf("complete read page = %#v", page)
	}
	if page.SelectionDigest == "" || page.SelectionDigest != CanonicalHistorySelectionDigest(page.Items) {
		t.Fatalf("selection digest = %q", page.SelectionDigest)
	}
	legacyRef := domain.SourceRef{SessionID: "A", MessageID: "a-legacy", Kind: string(domain.SourceKindMessage)}
	page = f.ReadAsRun(t, domain.HistoryReadRequest{
		Selection: &domain.HistorySelection{SourceSessionID: "A", Refs: []domain.SourceRef{safeRef, legacyRef}},
		Limit:     1,
	})
	if page.SelectionDigest != "" {
		t.Fatal("partial page exposed a selection digest")
	}
	if page.NextCursor == "" {
		t.Fatal("partial page omitted the continuation cursor")
	}
}

func TestHistoryFixtureTraceImmediateProvenance(t *testing.T) {
	f := newHistoryFixture(t)
	page, err := f.service.Trace(f.runCtx, domain.HistoryTraceRequest{SourceRef: &domain.SourceRef{
		SessionID: "A", MessageID: "a-safe", Kind: string(domain.SourceKindMessage),
	}})
	if err != nil {
		t.Fatalf("trace: %v", err)
	}
	if len(page.Items) != 1 || page.Items[0].Ref.MessageID != "a-safe" {
		t.Fatalf("trace page = %#v", page)
	}
	complete := page.Status == string(domain.HistoryStatusOK) && page.NextCursor == ""
	partial := page.Status == string(domain.HistoryStatusPartial) && page.NextCursor != ""
	if !complete && !partial {
		t.Fatalf("trace status = %q with continuation cursor present: %v", page.Status, page.NextCursor != "")
	}
	if !slices.Contains(page.Warnings, "immediate_provenance_only") {
		t.Fatalf("trace warnings = %v", page.Warnings)
	}
	page, err = f.service.Trace(f.runCtx, domain.HistoryTraceRequest{SourceRef: &domain.SourceRef{
		SessionID: "C", MessageID: "c-marker", Kind: string(domain.SourceKindMessage),
	}})
	if err != nil {
		t.Fatalf("trace denied: %v", err)
	}
	if page.Status != string(domain.HistoryStatusForbidden) || len(page.Items) != 0 {
		t.Fatalf("denied trace leak: %#v", page)
	}
}

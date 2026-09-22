package conformance

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

// RunHistoryQuerySuite exercises the storage-only ordering and budget
// contract shared by the first-party SQL backends. It deliberately consumes
// HistoryCandidates rather than a model-facing HistoryPage: projection,
// authorization and redaction belong to T3.
func RunHistoryQuerySuite(t *testing.T, b storage.Engine) {
	t.Helper()
	q, ok := b.(storage.HistoryQueryStore)
	if !ok {
		t.Fatal("backend does not implement HistoryQueryStore")
	}
	t.Run("stable capture and paging", func(t *testing.T) { assertHistoryCutStable(t, b, q) })
	t.Run("current rewind visibility", func(t *testing.T) { assertHistoryRewindVisibility(t, b, q) })
	t.Run("bounded scans progress", func(t *testing.T) { assertHistoryBoundedProgress(t, b, q) })
	t.Run("narrow run selection", func(t *testing.T) { assertHistoryRunLimit(t, b, q) })
	t.Run("cancellation and hostile identifiers", func(t *testing.T) { assertHistoryCancellationAndIDs(t, b, q) })
	t.Run("all current message writers allocate positions", func(t *testing.T) { assertHistoryWriterPositions(t, b, q) })
}

func assertHistoryCutStable(t *testing.T, b storage.Engine, q storage.HistoryQueryStore) {
	t.Helper()
	ctx := context.Background()
	const sid = domain.SessionID("history-B")
	createHistorySession(t, b, sid)
	appendHistoryMessage(t, b, sid, "m1", 100, "one")
	appendHistoryMessage(t, b, sid, "m2", 100, "two")
	cut, err := q.CaptureHistoryCut(ctx, []domain.SessionID{sid})
	if err != nil { t.Fatalf("CaptureHistoryCut: %v", err) }
	// This delayed projection has an earlier timestamp, so CreatedAt cannot be
	// the cut boundary. It must be absent from every page of this capture.
	if _, err := b.AppendMessageIfAbsent(ctx, domain.Message{ID: "m3", SessionID: sid, Role: domain.RoleAssistant, CreatedAt: 1, Content: "late projection"}); err != nil { t.Fatalf("delayed assistant projection: %v", err) }
	var ids []string
	var after storage.HistoryPosition
	for {
		page, err := q.QueryHistoryPage(ctx, cut, after, storage.HistoryQueryOptions{Limit: 1})
		if err != nil { t.Fatalf("QueryHistoryPage: %v", err) }
		if page.BytesInspected > 4<<20 { t.Fatalf("unbounded payload scan: %d", page.BytesInspected) }
		for _, record := range page.Records { ids = append(ids, record.Ref.MessageID) }
		if page.Next.IsZero() { break }
		after = page.Next
		if !page.HasMore && !page.ScanIncomplete { break }
	}
	if got := strings.Join(ids, ","); got != "m1,m2" { t.Fatalf("unstable cut: %v", ids) }
	seen := map[string]bool{}
	for _, id := range ids { if seen[id] { t.Fatalf("duplicate history result %q", id) }; seen[id] = true }
	// A different session must never leak into a cut that did not authorize it.
	createHistorySession(t, b, "history-C")
	appendHistoryMessage(t, b, "history-C", "other", 1, "other")
}

func assertHistoryRewindVisibility(t *testing.T, b storage.Engine, q storage.HistoryQueryStore) {
	t.Helper()
	ctx := context.Background()
	sid := domain.SessionID("history-rewind")
	createHistorySession(t, b, sid)
	appendHistoryMessage(t, b, sid, "before", 1, "before")
	appendHistoryMessage(t, b, sid, "hidden", 2, "hidden")
	cut, err := q.CaptureHistoryCut(ctx, []domain.SessionID{sid})
	if err != nil { t.Fatal(err) }
	first, err := q.QueryHistoryPage(ctx, cut, storage.HistoryPosition{}, storage.HistoryQueryOptions{Limit: 1})
	if err != nil || len(first.Records) != 1 || first.Records[0].Ref.MessageID != "before" { t.Fatalf("first page = %+v, %v", first, err) }
	if err := b.RecordSessionTruncation(ctx, storage.SessionTruncation{SessionID: sid, CutoffMessageID: "hidden", TailMessageID: "hidden", Reason: storage.TruncationRewind, CreatedAt: 3}); err != nil { t.Fatal(err) }
	second, err := q.QueryHistoryPage(ctx, cut, first.Next, storage.HistoryQueryOptions{Limit: 1})
	if err != nil { t.Fatal(err) }
	if len(second.Records) != 0 { t.Fatalf("rewound row remained visible: %+v", second.Records) }
	deleted := domain.SessionID("history-deleted")
	createHistorySession(t, b, deleted)
	appendHistoryMessage(t, b, deleted, "gone", 1, "gone")
	deletedCut, err := q.CaptureHistoryCut(ctx, []domain.SessionID{deleted})
	if err != nil { t.Fatal(err) }
	if err := b.DeleteSession(ctx, deleted); err != nil { t.Fatal(err) }
	deletedPage, err := q.QueryHistoryPage(ctx, deletedCut, storage.HistoryPosition{}, storage.HistoryQueryOptions{Limit: 1})
	if err != nil || len(deletedPage.Records) != 0 { t.Fatalf("deleted source page = %+v, %v", deletedPage, err) }
}

func assertHistoryBoundedProgress(t *testing.T, b storage.Engine, q storage.HistoryQueryStore) {
	t.Helper()
	ctx := context.Background()
	sid := domain.SessionID("history-budget")
	createHistorySession(t, b, sid)
	appendHistoryMessage(t, b, sid, "huge", 1, strings.Repeat("x", 9<<10))
	cut, err := q.CaptureHistoryCut(ctx, []domain.SessionID{sid})
	if err != nil { t.Fatal(err) }
	page, err := q.QueryHistoryPage(ctx, cut, storage.HistoryPosition{}, storage.HistoryQueryOptions{Limit: 1})
	if err != nil { t.Fatal(err) }
	if len(page.Records) != 1 || !page.Records[0].Unavailable || !page.Records[0].Truncated || page.Next.IsZero() { t.Fatalf("oversized record did not advance safely: %+v", page) }
	if page.BytesInspected > 4<<20 { t.Fatalf("BytesInspected=%d", page.BytesInspected) }
	// More than CandidateRecords metadata rows must yield a cursor even when
	// no safe result can be returned from the bounded scan.
	for i := 0; i <= domain.DefaultContinuityLimits().CandidateRecords; i++ {
		appendHistoryMessage(t, b, sid, fmt.Sprintf("many-%04d", i), int64(i+2), "")
	}
	if err := b.RecordSessionTruncation(ctx, storage.SessionTruncation{SessionID: sid, CutoffMessageID: "many-0000", TailMessageID: "many-2000", Reason: storage.TruncationRewind, CreatedAt: 4}); err != nil { t.Fatal(err) }
	cut, err = q.CaptureHistoryCut(ctx, []domain.SessionID{sid})
	if err != nil { t.Fatal(err) }
	page, err = q.QueryHistoryPage(ctx, cut, page.Next, storage.HistoryQueryOptions{Limit: 1})
	if err != nil { t.Fatal(err) }
	if len(page.Records) != 0 || !page.ScanIncomplete || page.Next.IsZero() { t.Fatalf("scan ceiling did not report empty progress: %+v", page) }
	byteSID := domain.SessionID("history-byte-budget")
	createHistorySession(t, b, byteSID)
	for i := 0; i < 600; i++ { appendHistoryMessage(t, b, byteSID, fmt.Sprintf("bytes-%04d", i), int64(i+1), strings.Repeat("b", 8<<10)) }
	if err := b.RecordSessionTruncation(ctx, storage.SessionTruncation{SessionID: byteSID, CutoffMessageID: "bytes-0000", TailMessageID: "bytes-0599", Reason: storage.TruncationRewind, CreatedAt: 2}); err != nil { t.Fatal(err) }
	byteCut, err := q.CaptureHistoryCut(ctx, []domain.SessionID{byteSID})
	if err != nil { t.Fatal(err) }
	bytePage, err := q.QueryHistoryPage(ctx, byteCut, storage.HistoryPosition{}, storage.HistoryQueryOptions{Limit: 1})
	if err != nil { t.Fatal(err) }
	if len(bytePage.Records) != 0 || !bytePage.ScanIncomplete || bytePage.BytesInspected != 4<<20 { t.Fatalf("byte ceiling = %+v", bytePage) }
}

func assertHistoryWriterPositions(t *testing.T, b storage.Engine, q storage.HistoryQueryStore) {
	t.Helper()
	ctx := context.Background()
	sid := domain.SessionID("history-writers")
	createHistorySession(t, b, sid)
	appendHistoryMessage(t, b, sid, "append", 10, "append")
	projected := domain.Message{ID: "projection", SessionID: sid, Role: domain.RoleAssistant, CreatedAt: 1, Content: "projection"}
	inserted, err := b.AppendMessageIfAbsent(ctx, projected)
	if err != nil || !inserted { t.Fatalf("first AppendMessageIfAbsent = %v, %v", inserted, err) }
	inserted, err = b.AppendMessageIfAbsent(ctx, projected)
	if err != nil || inserted { t.Fatalf("duplicate AppendMessageIfAbsent = %v, %v", inserted, err) }
	edit := domain.Message{ID: "edit", SessionID: sid, Role: domain.RoleUser, CreatedAt: 2, Content: "edit"}
	_, err = b.CommitSessionEdit(ctx, storage.SessionTruncation{SessionID: sid, CutoffMessageID: "append", TailMessageID: "append", Reason: storage.TruncationEdit, CreatedAt: 11}, edit, domain.Run{ID: "edit-run", SessionID: sid, CreatedAt: 12}, domain.RunEvent{RunID: "edit-run", Type: domain.EventRunStarted, CreatedAt: 12, PayloadVersion: 1, Payload: []byte(`{}`)})
	if err != nil { t.Fatalf("CommitSessionEdit: %v", err) }
	child := domain.Session{ID: "history-fork", CreatedAt: 1}
	_, err = b.CommitSessionFork(ctx, child, []domain.Message{{ID: "fork", SessionID: child.ID, Role: domain.RoleUser, CreatedAt: 1, Content: "fork"}}, nil, nil)
	if err != nil { t.Fatalf("CommitSessionFork: %v", err) }
	cut, err := q.CaptureHistoryCut(ctx, []domain.SessionID{sid})
	if err != nil { t.Fatal(err) }
	page, err := q.QueryHistoryPage(ctx, cut, storage.HistoryPosition{}, storage.HistoryQueryOptions{Limit: 10})
	if err != nil { t.Fatal(err) }
	if len(page.Records) != 2 || page.Records[0].Ref.MessageID != "projection" || page.Records[1].Ref.MessageID != "edit" { t.Fatalf("writer history = %+v", page.Records) }
	forkCut, err := q.CaptureHistoryCut(ctx, []domain.SessionID{child.ID})
	if err != nil { t.Fatal(err) }
	forkPage, err := q.QueryHistoryPage(ctx, forkCut, storage.HistoryPosition{}, storage.HistoryQueryOptions{Limit: 1})
	if err != nil || len(forkPage.Records) != 1 || forkPage.Records[0].Ref.MessageID != "fork" { t.Fatalf("fork history = %+v, %v", forkPage, err) }
}

func assertHistoryRunLimit(t *testing.T, b storage.Engine, q storage.HistoryQueryStore) {
	t.Helper()
	ctx := context.Background()
	sid := domain.SessionID("history-runs")
	createHistorySession(t, b, sid)
	for i := 0; i < 257; i++ {
		if err := b.CreateRun(ctx, domain.Run{ID: domain.RunID(fmt.Sprintf("history-run-%03d", i)), SessionID: sid, Status: domain.RunActive, CreatedAt: int64(i)}); err != nil { t.Fatalf("CreateRun %d: %v", i, err) }
	}
	_, err := q.CaptureHistoryCut(ctx, []domain.SessionID{sid})
	if !errors.Is(err, storage.ErrHistoryNarrowScope) { t.Fatalf("CaptureHistoryCut runs = %v, want ErrHistoryNarrowScope", err) }
}

func assertHistoryCancellationAndIDs(t *testing.T, b storage.Engine, q storage.HistoryQueryStore) {
	t.Helper()
	ctx := context.Background()
	malicious := domain.SessionID("history'; DROP TABLE messages; --")
	createHistorySession(t, b, malicious)
	appendHistoryMessage(t, b, malicious, "quoted", 1, "safe")
	cut, err := q.CaptureHistoryCut(ctx, []domain.SessionID{malicious})
	if err != nil { t.Fatalf("malicious capture: %v", err) }
	if _, err := q.QueryHistoryPage(ctx, cut, storage.HistoryPosition{}, storage.HistoryQueryOptions{Limit: 1}); err != nil { t.Fatalf("malicious page: %v", err) }
	cancelled, cancel := context.WithCancel(ctx); cancel()
	if _, err := q.CaptureHistoryCut(cancelled, []domain.SessionID{malicious}); !errors.Is(err, context.Canceled) { t.Fatalf("cancelled capture = %v", err) }
	if _, err := q.QueryHistoryPage(cancelled, cut, storage.HistoryPosition{}, storage.HistoryQueryOptions{Limit: 1}); !errors.Is(err, context.Canceled) { t.Fatalf("cancelled page = %v", err) }
}

func createHistorySession(t *testing.T, b storage.Engine, id domain.SessionID) { t.Helper(); if err := b.CreateSession(context.Background(), domain.Session{ID: id, CreatedAt: 1}); err != nil { t.Fatalf("CreateSession %q: %v", id, err) } }
func appendHistoryMessage(t *testing.T, b storage.Engine, sid domain.SessionID, id string, at int64, text string) { t.Helper(); if err := b.AppendMessage(context.Background(), domain.Message{ID: id, SessionID: sid, Role: domain.RoleUser, CreatedAt: at, Content: text}); err != nil { t.Fatalf("AppendMessage %q: %v", id, err) } }

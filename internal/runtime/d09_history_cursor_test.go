package runtime

import (
	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestD09HistoryCursorRestartRetainsHistoricalCut(t *testing.T) {
	f := newHistoryFixture(t)
	request := domain.HistorySearchRequest{Limit: 1}
	first := f.QueryAsRun(t, request)
	if first.NextCursor == "" {
		t.Fatal("missing continuation")
	}
	if err := f.backend.AppendMessage(context.Background(), domain.Message{ID: "a-later", SessionID: "A", Role: domain.RoleUser, CreatedAt: 100, Content: "later message outside cut"}); err != nil {
		t.Fatal(err)
	}
	f.service = NewHistoryService(f.backend, f.backend)
	request.Cursor = first.NextCursor
	request.Limit = 50
	page := f.QueryAsRun(t, request)
	if page.Status != string(domain.HistoryStatusOK) && page.Status != string(domain.HistoryStatusPartial) {
		t.Fatalf("restart lost continuation: %s", page.Status)
	}
	for _, item := range page.Items {
		if item.Ref.MessageID == "a-later" {
			t.Fatal("historical cut silently advanced")
		}
	}
}
func TestD09HistoryCursorCannotExpandCutOrSelectedScope(t *testing.T) {
	for _, change := range []string{"future-session", "foreign-selected-session", "future-run"} {
		t.Run(change, func(t *testing.T) {
			f := newHistoryFixture(t)
			scope, err := domain.NewAcceptedHistoryScope("B", []domain.SessionID{"A", "B"})
			if err != nil {
				t.Fatal(err)
			}
			ctx := WithHistoryScope(f.ownCtx, scope)
			request := domain.HistorySearchRequest{SessionIDs: []domain.SessionID{"A"}, Limit: 1}
			if change == "future-run" {
				request.SessionIDs = []domain.SessionID{"B"}
			}
			first, err := f.service.Search(ctx, request)
			if err != nil || first.NextCursor == "" {
				t.Fatalf("first page: %+v %v", first, err)
			}
			state, err := f.service.decodeCursor(first.NextCursor)
			if err != nil {
				t.Fatal(err)
			}
			switch change {
			case "future-session":
				state.Cut.Sessions[0].Position += 999
			case "foreign-selected-session":
				state.Cut, err = f.backend.CaptureHistoryCut(ctx, []domain.SessionID{"A", "B"})
				if err != nil {
					t.Fatal(err)
				}
			case "future-run":
				state.Cut.Runs[0].Seq += 999
			}
			request.Cursor, err = f.service.encodeCursor(state)
			if err != nil {
				t.Fatal(err)
			}
			request.Limit = 50
			page, err := f.service.Search(ctx, request)
			if err != nil {
				t.Fatal(err)
			}
			if page.Status == string(domain.HistoryStatusOK) || page.Status == string(domain.HistoryStatusPartial) || len(page.Items) > 0 {
				t.Fatalf("edited cut accepted: %s / %d items", page.Status, len(page.Items))
			}
		})
	}
}
func TestD09HistoryCursorReauthorizesScopeOnEveryPage(t *testing.T) {
	f := newHistoryFixture(t)
	request := domain.HistorySearchRequest{Limit: 1}
	first := f.QueryAsRun(t, request)
	if first.NextCursor == "" {
		t.Fatal("missing continuation")
	}
	request.Cursor = first.NextCursor
	page, err := f.service.Search(f.ownCtx, request)
	if err != nil {
		t.Fatal(err)
	}
	if page.Status != string(domain.HistoryStatusForbidden) || len(page.Items) > 0 {
		t.Fatalf("revoked scope accepted: %+v", page)
	}
}

func TestD09HistoryCursorRejectsUntrustedMalformedPositions(t *testing.T) {
	for _, change := range []string{"version1", "bad-phase", "phase-after-mismatch", "hidden-zero-position", "future-after", "foreign-run", "oversized", "trailing-json", "unknown-field"} {
		t.Run(change, func(t *testing.T) {
			f := newHistoryFixture(t)
			request := domain.HistorySearchRequest{Limit: 1}
			first := f.QueryAsRun(t, request)
			if first.NextCursor == "" {
				t.Fatal("missing continuation")
			}
			state, err := f.service.decodeCursor(first.NextCursor)
			if err != nil {
				t.Fatal(err)
			}
			switch change {
			case "version1":
				state.Version = 1
			case "bad-phase":
				state.Phase = "invented"
			case "phase-after-mismatch":
				state.Phase = historyPhaseEvents
				state.After = storage.HistoryPosition{Stream: storage.HistoryStreamMessage, SessionID: "A", Position: 1}
			case "hidden-zero-position":
				state.After = storage.HistoryPosition{SessionID: "C", Position: 1}
			case "future-after":
				state.After = storage.HistoryPosition{Stream: storage.HistoryStreamMessage, SessionID: "A", Position: 99999}
			case "foreign-run":
				state.Cut.Runs = []storage.HistoryRunCut{{RunID: "foreign-run", SessionID: "A", Seq: 1}}
			}
			raw, err := json.Marshal(state)
			if err != nil {
				t.Fatal(err)
			}
			switch change {
			case "trailing-json":
				raw = append(raw, []byte(` {}`)...)
			case "unknown-field":
				raw = append(raw[:len(raw)-1], []byte(`,"authority":"operator"}`)...)
			}
			request.Cursor = base64.RawURLEncoding.EncodeToString(raw)
			if change == "oversized" {
				request.Cursor = strings.Repeat("x", historyCursorTransportMax+1)
			}
			page := f.QueryAsRun(t, request)
			if page.Status == string(domain.HistoryStatusOK) || page.Status == string(domain.HistoryStatusPartial) || len(page.Items) != 0 {
				t.Fatalf("malformed token gained data: %+v", page)
			}
		})
	}
}
func TestD09HistoryCursorComparesNormalizedFilterValues(t *testing.T) {
	f := newHistoryFixture(t)
	request := domain.HistorySearchRequest{Query: "RELEASE", Kinds: []string{"message", "event"}, Limit: 1}
	first := f.QueryAsRun(t, request)
	if first.NextCursor == "" {
		t.Fatal("missing continuation")
	}
	request.Cursor = first.NextCursor
	request.Query = "release"
	request.Kinds = []string{"event", "message", "event"}
	page := f.QueryAsRun(t, request)
	if page.Status != string(domain.HistoryStatusOK) && page.Status != string(domain.HistoryStatusPartial) {
		t.Fatalf("equivalent normalized filter refused: %+v", page)
	}
	request.Query = "different"
	page = f.QueryAsRun(t, request)
	if page.Status != string(domain.HistoryStatusForbidden) || len(page.Items) != 0 {
		t.Fatalf("changed filter accepted: %+v", page)
	}
}

func TestD09HistoryCursorReappliesCurrentTruncationAndDeletion(t *testing.T) {
	for _, reason := range []string{storage.TruncationRewind, storage.TruncationEdit, storage.TruncationFork} {
		t.Run(reason, func(t *testing.T) {
			f := newHistoryFixture(t)
			request := domain.HistorySearchRequest{Limit: 1}
			first := f.QueryAsRun(t, request)
			if first.NextCursor == "" {
				t.Fatal("missing continuation")
			}
			if err := f.backend.RecordSessionTruncation(context.Background(), storage.SessionTruncation{SessionID: "A", CutoffMessageID: "a-star", TailMessageID: "a-star", Reason: reason, CreatedAt: 30}); err != nil {
				t.Fatal(err)
			}
			request.Cursor = first.NextCursor
			request.Limit = 50
			page := f.QueryAsRun(t, request)
			found := false
			for _, item := range page.Items {
				if item.Ref.MessageID == "a-star" {
					found = true
				}
			}
			if found != (reason == storage.TruncationFork) {
				t.Fatalf("wrong current visibility for %s: found=%v", reason, found)
			}
		})
	}
	f := newHistoryFixture(t)
	request := domain.HistorySearchRequest{Limit: 1}
	first := f.QueryAsRun(t, request)
	if err := f.backend.DeleteSession(context.Background(), "A"); err != nil {
		t.Fatal(err)
	}
	request.Cursor = first.NextCursor
	page := f.QueryAsRun(t, request)
	if page.Status != string(domain.HistoryStatusNotFound) || len(page.Items) > 0 {
		t.Fatalf("deleted source remains readable: %s", page.Status)
	}
}
func TestD09HistoryLongReadSelectionNarrowsWithoutSkipping(t *testing.T) {
	f := newHistoryFixture(t)
	refs := make([]domain.SourceRef, 100)
	for i := range refs {
		id := fmt.Sprintf("long-%03d-%s", i, strings.Repeat("x", 200))
		if err := f.backend.AppendMessage(context.Background(), domain.Message{ID: id, SessionID: "A", Role: domain.RoleUser, CreatedAt: int64(100 + i), Content: fmt.Sprintf("selected %03d", i)}); err != nil {
			t.Fatal(err)
		}
		refs[i] = domain.SourceRef{SessionID: "A", MessageID: id, Kind: "message"}
	}
	request := domain.HistoryReadRequest{Selection: &domain.HistorySelection{SourceSessionID: "A", Refs: refs}, Limit: 1}
	page := f.ReadAsRun(t, request)
	if err := page.Validate(domain.DefaultContinuityLimits()); err != nil {
		t.Fatalf("oversized cursor escaped page bound: %v", err)
	}
	if page.Status != string(domain.HistoryStatusInvalidArgument) || page.Reason == nil || *page.Reason != "narrow_scope" || len(page.Items) != 0 || page.NextCursor != "" {
		t.Fatalf("large selection must refuse without skipping: status=%s items=%d cursor=%d", page.Status, len(page.Items), len(page.NextCursor))
	}
	seen := map[string]bool{}
	for offset := 0; offset < len(refs); offset += 10 {
		request.Selection = &domain.HistorySelection{SourceSessionID: "A", Refs: refs[offset : offset+10]}
		request.Cursor = ""
		for pageNo := 0; pageNo < 200; pageNo++ {
			page = f.ReadAsRun(t, request)
			if err := page.Validate(domain.DefaultContinuityLimits()); err != nil {
				t.Fatal(err)
			}
			if page.Status != string(domain.HistoryStatusOK) && page.Status != string(domain.HistoryStatusPartial) && page.Status != string(domain.HistoryStatusNotFound) {
				t.Fatalf("narrow selection failed: %s", page.Status)
			}
			for _, item := range page.Items {
				if seen[item.Ref.MessageID] {
					t.Fatal("duplicate selected result")
				}
				seen[item.Ref.MessageID] = true
			}
			if page.NextCursor == "" {
				break
			}
			request.Cursor = page.NextCursor
			if pageNo == 199 {
				t.Fatal("pagination failed to finish")
			}
		}
	}
	if len(seen) != len(refs) {
		t.Fatalf("selected records skipped: got %d want %d", len(seen), len(refs))
	}
}

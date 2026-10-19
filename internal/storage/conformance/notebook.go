package conformance

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	nb "agent-vivy/internal/notebookcontract"
)

// NotebookStore returns the narrow notebook surface or fails the test.
func NotebookStore(t *testing.T, slot Slot) nb.Store {
	t.Helper()
	return slot.Engine.Notebook()
}

// NotebookMC builds a trusted mutation context with a computed canonical
// digest, matching how a trusted host stamps requests.
func NotebookMC(t *testing.T, scope nb.ScopeID, key string) nb.MutationContext {
	t.Helper()
	return nb.MutationContext{
		ScopeID:      scope,
		Actor:        nb.Actor{Kind: nb.ActorHuman, Ref: "local:operator"},
		OperationKey: key,
	}
}

// AssertNotebookContract runs the N1 scoped-content contract against one
// backend slot: retry idempotence, CAS conflicts, scope isolation, tombstones,
// comments and bounds — plus reopen persistence.
func AssertNotebookContract(t *testing.T, slot Slot) {
	t.Helper()
	ctx := context.Background()
	const scope = nb.HomeScopeID
	s := NotebookStore(t, slot)
	human := func(key string) nb.MutationContext { return NotebookMC(t, scope, key) }

	t.Run("seeds system sections on first access", func(t *testing.T) {
		page, err := s.ListSections(ctx, scope, nb.ListSectionsRequest{})
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		if len(page.Sections) != 4 {
			t.Fatalf("sections = %+v, want 4 role sections", page.Sections)
		}
		seen := map[nb.SystemRole]bool{}
		for _, sec := range page.Sections {
			seen[sec.SystemRole] = true
			if sec.ID != nb.RoleSectionID(sec.SystemRole) || sec.Version != 1 {
				t.Fatalf("role section identity = %+v", sec)
			}
		}
		for _, role := range nb.SystemRoles {
			if !seen[role] {
				t.Fatalf("missing role section %q", role)
			}
		}
		// Idempotent: second list stays four rows.
		page2, err := s.ListSections(ctx, scope, nb.ListSectionsRequest{})
		if err != nil || len(page2.Sections) != 4 {
			t.Fatalf("reseed duplicated sections: %+v err=%v", page2, err)
		}
	})

	entry := func(section, title, body, key string) nb.MutationReceipt {
		t.Helper()
		rc, err := s.CreateEntry(ctx, human(key), nb.CreateEntryRequest{SectionID: section, Title: title, Markdown: body})
		if err != nil {
			t.Fatalf("create entry %q: %v", title, err)
		}
		return rc
	}
	_ = entry

	t.Run("create save and read back", func(t *testing.T) {
		rc, err := s.CreateEntry(ctx, human("op-create-1"), nb.CreateEntryRequest{
			SectionID: "section-notes", Title: "first", Markdown: "hello **world**",
		})
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		if rc.ResourceID == "" || rc.RevisionID == "" || rc.Version != 1 || rc.Replayed {
			t.Fatalf("receipt = %+v", rc)
		}
		view, err := s.GetEntry(ctx, scope, nb.GetEntryRequest{EntryID: rc.ResourceID})
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		if view.Revision.Markdown != "hello **world**" || view.Revision.Origin != nb.OriginHuman ||
			view.Revision.ID != rc.RevisionID || view.Entry.HeadRevisionID != rc.RevisionID || view.Entry.Title != "first" {
			t.Fatalf("view = %+v", view)
		}
		save, err := s.SaveEntry(ctx, human("op-save-1"), nb.SaveEntryRequest{
			EntryID: rc.ResourceID, ExpectedVersion: 1, BaseRevisionID: rc.RevisionID,
			Title: "first v2", Markdown: "edited body",
		})
		if err != nil {
			t.Fatalf("save: %v", err)
		}
		if save.Version != 2 || save.RevisionID == rc.RevisionID {
			t.Fatalf("save receipt = %+v", save)
		}
		view, err = s.GetEntry(ctx, scope, nb.GetEntryRequest{EntryID: rc.ResourceID})
		if err != nil || view.Revision.Markdown != "edited body" || view.Revision.ParentRevisionID != rc.RevisionID || view.Entry.Version != 2 {
			t.Fatalf("post-save view = %+v err=%v", view, err)
		}
		revs, err := s.ListRevisions(ctx, scope, nb.ListRevisionsRequest{EntryID: rc.ResourceID})
		if err != nil || len(revs.Revisions) != 2 || revs.Revisions[0].Sequence != 2 || revs.Revisions[0].Markdown != "" {
			t.Fatalf("revisions = %+v err=%v (metadata must not embed bodies)", revs, err)
		}
	})

	t.Run("retry after commit returns identical receipt", func(t *testing.T) {
		mc := human("op-retry-1")
		req := nb.SaveEntryRequest{EntryID: "", ExpectedVersion: 1, Title: "x"}
		rc, err := s.CreateEntry(ctx, human("op-retry-create"), nb.CreateEntryRequest{SectionID: "section-notes", Title: "r", Markdown: "r"})
		if err != nil {
			t.Fatal(err)
		}
		req.EntryID = rc.ResourceID
		req.BaseRevisionID = rc.RevisionID
		req.Markdown = "retry body"
		r1, err := s.SaveEntry(ctx, mc, req)
		if err != nil {
			t.Fatalf("first save: %v", err)
		}
		// Discard acknowledgement, reopen, retry identical key+request.
		reopened, err := slot.Reopen()
		if err != nil {
			t.Fatalf("reopen: %v", err)
		}
		s = reopened.Notebook()
		r2, err := s.SaveEntry(ctx, mc, req)
		if err != nil {
			t.Fatalf("retry save: %v", err)
		}
		if !r2.Replayed || r2.ResourceID != r1.ResourceID || r2.Version != r1.Version || r2.RevisionID != r1.RevisionID {
			t.Fatalf("retry receipt = %+v, want identical replay of %+v", r2, r1)
		}
		revs, err := s.ListRevisions(ctx, scope, nb.ListRevisionsRequest{EntryID: rc.ResourceID})
		if err != nil || len(revs.Revisions) != 2 {
			t.Fatalf("retry minted extra history: %+v err=%v", revs, err)
		}
		// Same key, different body -> idempotency_conflict.
		bad := req
		bad.Markdown = "different"
		if _, err := s.SaveEntry(ctx, mc, bad); !errors.Is(err, nb.ErrIdempotencyConflict) {
			t.Fatalf("reused key with different body = %v, want idempotency_conflict", err)
		}
	})

	t.Run("cas conflicts carry current metadata", func(t *testing.T) {
		rc, err := s.CreateEntry(ctx, human("op-cas-create"), nb.CreateEntryRequest{SectionID: "section-notes", Title: "cas", Markdown: "a"})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.SaveEntry(ctx, human("op-cas-ver"), nb.SaveEntryRequest{
			EntryID: rc.ResourceID, ExpectedVersion: 5, BaseRevisionID: rc.RevisionID, Title: "t", Markdown: "b",
		}); !errors.Is(err, nb.ErrRevisionConflict) {
			t.Fatalf("stale version = %v, want revision_conflict", err)
		} else if v, r := nb.CurrentOf(err); v != 1 || r != rc.RevisionID {
			t.Fatalf("conflict metadata = %d,%q", v, r)
		}
		if _, err := s.SaveEntry(ctx, human("op-cas-base"), nb.SaveEntryRequest{
			EntryID: rc.ResourceID, ExpectedVersion: 1, BaseRevisionID: "rev-wrong", Title: "t", Markdown: "b",
		}); !errors.Is(err, nb.ErrRevisionConflict) {
			t.Fatalf("stale base = %v, want revision_conflict", err)
		}
		// Two writers race one base: exactly one wins.
		var wg sync.WaitGroup
		results := make(chan error, 2)
		for i, key := range []string{"op-race-a", "op-race-b"} {
			wg.Add(1)
			go func(key string, i int) {
				defer wg.Done()
				_, err := s.SaveEntry(ctx, human(key), nb.SaveEntryRequest{
					EntryID: rc.ResourceID, ExpectedVersion: 1, BaseRevisionID: rc.RevisionID,
					Title: "racer", Markdown: strings.Repeat("x", i+1),
				})
				results <- err
			}(key, i)
		}
		wg.Wait()
		close(results)
		ok, conflicts := 0, 0
		for err := range results {
			if err == nil {
				ok++
			} else if errors.Is(err, nb.ErrRevisionConflict) || errors.Is(err, nb.ErrIdempotencyConflict) {
				conflicts++
			} else {
				t.Fatalf("unexpected race error: %v", err)
			}
		}
		if ok != 1 || conflicts != 1 {
			t.Fatalf("race outcome ok=%d conflicts=%d, want exactly one winner", ok, conflicts)
		}
		// Concurrent retry of the SAME key+body yields one receipt.
		mc := human("op-race-same")
		req := nb.SaveEntryRequest{EntryID: rc.ResourceID, ExpectedVersion: 2, BaseRevisionID: "", Title: "t", Markdown: "same"}
		var wg2 sync.WaitGroup
		receipts := make(chan nb.MutationReceipt, 4)
		errs := make(chan error, 4)
		for i := 0; i < 4; i++ {
			wg2.Add(1)
			go func() {
				defer wg2.Done()
				// discover current head first for a well-formed request
				v, err := s.GetEntry(ctx, scope, nb.GetEntryRequest{EntryID: req.EntryID})
				if err != nil {
					errs <- err
					return
				}
				req.BaseRevisionID = v.Entry.HeadRevisionID
				req.ExpectedVersion = v.Entry.Version
				r, err := s.SaveEntry(ctx, mc, req)
				if err != nil {
					errs <- err
					return
				}
				receipts <- r
			}()
		}
		wg2.Wait()
		close(receipts)
		close(errs)
		var first nb.MutationReceipt
		n := 0
		for r := range receipts {
			n++
			if n == 1 {
				first = r
			} else if r.ResourceID != first.ResourceID || r.RevisionID != first.RevisionID || r.Version != first.Version {
				t.Fatalf("concurrent identical retry diverged: %+v vs %+v", first, r)
			}
		}
		for err := range errs {
			if !errors.Is(err, nb.ErrRevisionConflict) && !errors.Is(err, nb.ErrIdempotencyConflict) {
				t.Fatalf("unexpected retry error: %v", err)
			}
		}
		if n == 0 {
			t.Fatal("no writer succeeded the identical-key race")
		}
	})

	t.Run("scope isolation", func(t *testing.T) {
		rc, err := s.CreateEntry(ctx, human("op-scope-a"), nb.CreateEntryRequest{SectionID: "section-notes", Title: "scoped", Markdown: "x"})
		if err != nil {
			t.Fatal(err)
		}
		other := nb.ScopeID("ws.v1:other")
		if _, err := s.GetEntry(ctx, other, nb.GetEntryRequest{EntryID: rc.ResourceID}); !errors.Is(err, nb.ErrNotFound) {
			t.Fatalf("foreign get = %v, want not_found", err)
		}
		if foreign, err := s.ListEntries(ctx, other, nb.ListEntriesRequest{SectionID: "section-notes"}); err != nil {
			t.Fatalf("foreign list err = %v", err)
		} else if len(foreign.Entries) != 0 {
			t.Fatalf("foreign scope listed %d entries", len(foreign.Entries))
		}
		page, err := s.ListEntries(ctx, scope, nb.ListEntriesRequest{})
		if err != nil || page.NextCursor == "" {
			page2, err2 := s.ListEntries(ctx, scope, nb.ListEntriesRequest{SectionID: "section-notes"})
			if err2 != nil {
				t.Fatalf("list: %v", err2)
			}
			page = page2
		}
		if page.NextCursor != "" {
			if _, err := s.ListEntries(ctx, other, nb.ListEntriesRequest{Cursor: page.NextCursor}); !errors.Is(err, nb.ErrInvalidRequest) {
				t.Fatalf("foreign cursor = %v, want invalid_request", err)
			}
		}
		if _, err := s.GetEntry(ctx, scope, nb.GetEntryRequest{EntryID: rc.ResourceID, RevisionID: "rev-foreign"}); !errors.Is(err, nb.ErrNotFound) {
			t.Fatalf("foreign revision = %v, want not_found", err)
		}
	})

	t.Run("sections lifecycle", func(t *testing.T) {
		rc, err := s.CreateSection(ctx, human("op-sec-create"), nb.CreateSectionRequest{Title: "Scratch"})
		if err != nil {
			t.Fatalf("create section: %v", err)
		}
		up, err := s.UpdateSection(ctx, human("op-sec-up"), nb.UpdateSectionRequest{ID: rc.ResourceID, Title: "Scratch 2", ExpectedVersion: 1})
		if err != nil || up.Version != 2 {
			t.Fatalf("update section = %+v err=%v", up, err)
		}
		if _, err := s.UpdateSection(ctx, human("op-sec-up-stale"), nb.UpdateSectionRequest{ID: rc.ResourceID, Title: "x", ExpectedVersion: 1}); !errors.Is(err, nb.ErrRevisionConflict) {
			t.Fatalf("stale section update = %v", err)
		}
		// Populated section cannot be deleted.
		e1, err := s.CreateEntry(ctx, human("op-sec-e"), nb.CreateEntryRequest{SectionID: rc.ResourceID, Title: "inside", Markdown: "x"})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.DeleteSection(ctx, human("op-sec-del"), nb.DeleteSectionRequest{ID: rc.ResourceID, ExpectedVersion: 2}); !errors.Is(err, nb.ErrSectionNotEmpty) {
			t.Fatalf("delete populated = %v, want section_not_empty", err)
		}
		if _, err := s.DeleteSection(ctx, human("op-sec-role"), nb.DeleteSectionRequest{ID: "section-notes", ExpectedVersion: 1}); !errors.Is(err, nb.ErrSectionInUse) {
			t.Fatalf("delete role section = %v, want section_in_use", err)
		}
		// Move the entry out, then delete works.
		if _, err := s.MoveEntry(ctx, human("op-sec-move"), nb.MoveEntryRequest{EntryID: e1.ResourceID, SectionID: "section-notes", ExpectedVersion: 1}); err != nil {
			t.Fatalf("move: %v", err)
		}
		del, err := s.DeleteSection(ctx, human("op-sec-del2"), nb.DeleteSectionRequest{ID: rc.ResourceID, ExpectedVersion: 2})
		if err != nil || del.ResourceID != rc.ResourceID {
			t.Fatalf("delete section = %+v err=%v", del, err)
		}
		// Moving into the deleted section is refused.
		if _, err := s.MoveEntry(ctx, human("op-sec-move2"), nb.MoveEntryRequest{EntryID: e1.ResourceID, SectionID: rc.ResourceID, ExpectedVersion: 2}); !errors.Is(err, nb.ErrNotFound) {
			t.Fatalf("move into deleted section = %v, want not_found", err)
		}
		if _, err := s.RestoreSection(ctx, human("op-sec-res"), nb.RestoreSectionRequest{ID: rc.ResourceID, ExpectedVersion: 3}); err != nil {
			t.Fatalf("restore section: %v", err)
		}
	})

	t.Run("entry tombstone retains history", func(t *testing.T) {
		rc, err := s.CreateEntry(ctx, human("op-tomb-create"), nb.CreateEntryRequest{SectionID: "section-notes", Title: "gone", Markdown: "x"})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.DeleteEntry(ctx, human("op-tomb-del"), nb.DeleteEntryRequest{EntryID: rc.ResourceID, ExpectedVersion: 1}); err != nil {
			t.Fatalf("delete: %v", err)
		}
		if _, err := s.GetEntry(ctx, scope, nb.GetEntryRequest{EntryID: rc.ResourceID}); !errors.Is(err, nb.ErrNotFound) {
			t.Fatalf("get deleted = %v, want not_found", err)
		}
		page, err := s.ListEntries(ctx, scope, nb.ListEntriesRequest{SectionID: "section-notes", Limit: nb.MaxPageRows})
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range page.Entries {
			if e.ID == rc.ResourceID {
				t.Fatalf("deleted entry %q leaked into default list", e.ID)
			}
		}
		withDeleted, err := s.ListEntries(ctx, scope, nb.ListEntriesRequest{SectionID: "section-notes", IncludeDeleted: true})
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, e := range withDeleted.Entries {
			if e.ID == rc.ResourceID && e.DeletedAt != 0 {
				found = true
			}
		}
		if !found {
			t.Fatal("tombstone missing from include_deleted list")
		}
		revs, err := s.ListRevisions(ctx, scope, nb.ListRevisionsRequest{EntryID: rc.ResourceID})
		if err != nil || len(revs.Revisions) != 1 {
			t.Fatalf("tombstone lost revisions: %+v err=%v", revs, err)
		}
		if _, err := s.RestoreEntry(ctx, human("op-tomb-res"), nb.RestoreEntryRequest{EntryID: rc.ResourceID, ExpectedVersion: 2}); err != nil {
			t.Fatalf("restore: %v", err)
		}
		if _, err := s.GetEntry(ctx, scope, nb.GetEntryRequest{EntryID: rc.ResourceID}); err != nil {
			t.Fatalf("get restored: %v", err)
		}
	})

	t.Run("comments lifecycle and anchors", func(t *testing.T) {
		rc, err := s.CreateEntry(ctx, human("op-cm-create"), nb.CreateEntryRequest{SectionID: "section-notes", Title: "c", Markdown: "x"})
		if err != nil {
			t.Fatal(err)
		}
		cc, err := s.CreateComment(ctx, human("op-cm-add"), nb.CreateCommentRequest{EntryID: rc.ResourceID, AnchorRevisionID: rc.RevisionID, Body: "looks good"})
		if err != nil {
			t.Fatalf("comment: %v", err)
		}
		if _, err := s.CreateComment(ctx, human("op-cm-bad"), nb.CreateCommentRequest{EntryID: rc.ResourceID, AnchorRevisionID: "rev-x", Body: "y"}); !errors.Is(err, nb.ErrNotFound) {
			t.Fatalf("bad anchor = %v, want not_found", err)
		}
		newBody := "edited"
		up, err := s.UpdateComment(ctx, human("op-cm-edit"), nb.UpdateCommentRequest{CommentID: cc.ResourceID, ExpectedVersion: 1, Body: &newBody})
		if err != nil || up.Version != 2 {
			t.Fatalf("edit comment = %+v err=%v", up, err)
		}
		if _, err := s.UpdateComment(ctx, human("op-cm-resolve"), nb.UpdateCommentRequest{CommentID: cc.ResourceID, ExpectedVersion: 2, Status: nb.CommentResolved}); err != nil {
			t.Fatalf("resolve: %v", err)
		}
		list, err := s.ListComments(ctx, scope, nb.ListCommentsRequest{EntryID: rc.ResourceID, Status: nb.CommentResolved})
		if err != nil || len(list.Comments) != 1 || list.Comments[0].Body != "edited" || list.Comments[0].AnchorRevisionID != rc.RevisionID {
			t.Fatalf("comments = %+v err=%v", list, err)
		}
		if _, err := s.UpdateComment(ctx, human("op-cm-del"), nb.UpdateCommentRequest{CommentID: cc.ResourceID, ExpectedVersion: 3, Status: nb.CommentDeleted}); err != nil {
			t.Fatalf("delete comment: %v", err)
		}
		if _, err := s.UpdateComment(ctx, human("op-cm-restore"), nb.UpdateCommentRequest{CommentID: cc.ResourceID, ExpectedVersion: 4, Status: nb.CommentActive}); err != nil {
			t.Fatalf("restore comment: %v", err)
		}
	})

	t.Run("adopt restores old revision as new history", func(t *testing.T) {
		rc, err := s.CreateEntry(ctx, human("op-ad-create"), nb.CreateEntryRequest{SectionID: "section-notes", Title: "v1", Markdown: "one"})
		if err != nil {
			t.Fatal(err)
		}
		sv, err := s.SaveEntry(ctx, human("op-ad-save"), nb.SaveEntryRequest{EntryID: rc.ResourceID, ExpectedVersion: 1, BaseRevisionID: rc.RevisionID, Title: "v2", Markdown: "two"})
		if err != nil {
			t.Fatal(err)
		}
		_, err = s.AdoptRevision(ctx, human("op-ad-adopt"), nb.AdoptRevisionRequest{EntryID: rc.ResourceID, RevisionID: rc.RevisionID, ExpectedVersion: 2})
		if err != nil {
			t.Fatalf("adopt: %v", err)
		}
		view, err := s.GetEntry(ctx, scope, nb.GetEntryRequest{EntryID: rc.ResourceID})
		if err != nil {
			t.Fatal(err)
		}
		if view.Revision.Markdown != "one" || view.Revision.BaseRevisionID != rc.RevisionID || view.Revision.ID == sv.RevisionID || view.Revision.Sequence != 3 {
			t.Fatalf("adopted view = %+v", view)
		}
	})

	t.Run("bounds enforced before writes", func(t *testing.T) {
		big := strings.Repeat("a", nb.MaxBodyBytes+1)
		if _, err := s.CreateEntry(ctx, human("op-bound-big"), nb.CreateEntryRequest{SectionID: "section-notes", Title: "t", Markdown: big}); !errors.Is(err, nb.ErrLimitExceeded) {
			t.Fatalf("oversize body = %v, want limit_exceeded", err)
		}
		if _, err := s.CreateEntry(ctx, human("op-bound-title"), nb.CreateEntryRequest{SectionID: "section-notes", Title: " ", Markdown: "x"}); !errors.Is(err, nb.ErrInvalidRequest) {
			t.Fatalf("empty title = %v, want invalid_request", err)
		}
		if _, err := s.CreateEntry(ctx, human("op-bound-utf8"), nb.CreateEntryRequest{SectionID: "section-notes", Title: "t", Markdown: string([]byte{0xff, 0xfe})}); !errors.Is(err, nb.ErrInvalidRequest) {
			t.Fatalf("invalid utf8 = %v, want invalid_request", err)
		}
		if _, err := s.CreateEntry(ctx, human("op-bound-sec"), nb.CreateEntryRequest{SectionID: "sec-nope", Title: "t", Markdown: "x"}); !errors.Is(err, nb.ErrNotFound) {
			t.Fatalf("missing section = %v, want not_found", err)
		}
		rc, err := s.CreateEntry(ctx, human("op-bound-c"), nb.CreateEntryRequest{SectionID: "section-notes", Title: "t", Markdown: "x"})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.CreateComment(ctx, human("op-bound-cm"), nb.CreateCommentRequest{EntryID: rc.ResourceID, Body: strings.Repeat("b", nb.MaxCommentBytes+1)}); !errors.Is(err, nb.ErrLimitExceeded) {
			t.Fatalf("oversize comment = %v, want limit_exceeded", err)
		}
		if _, err := s.SaveEntry(ctx, nb.MutationContext{ScopeID: scope, Actor: nb.Actor{Kind: nb.ActorHuman, Ref: "r"}}, nb.SaveEntryRequest{EntryID: rc.ResourceID}); !errors.Is(err, nb.ErrInvalidRequest) {
			t.Fatalf("missing op key = %v, want invalid_request", err)
		}
		if _, err := s.SaveEntry(ctx, nb.MutationContext{ScopeID: scope, Actor: nb.Actor{Kind: nb.ActorWorkflow, Ref: "r"}, OperationKey: "k"}, nb.SaveEntryRequest{EntryID: rc.ResourceID}); !errors.Is(err, nb.ErrInvalidRequest) {
			t.Fatalf("workflow actor on ordinary path = %v, want invalid_request", err)
		}
	})

	t.Run("stable metadata pagination", func(t *testing.T) {
		sec, err := s.CreateSection(ctx, human("op-page-sec"), nb.CreateSectionRequest{Title: "Paged"})
		if err != nil {
			t.Fatal(err)
		}
		var ids []string
		for i := 0; i < 5; i++ {
			rc, err := s.CreateEntry(ctx, human("op-page-e"+string(rune('a'+i))), nb.CreateEntryRequest{SectionID: sec.ResourceID, Title: "p", Markdown: "x"})
			if err != nil {
				t.Fatal(err)
			}
			ids = append(ids, rc.ResourceID)
		}
		page1, err := s.ListEntries(ctx, scope, nb.ListEntriesRequest{SectionID: sec.ResourceID, Limit: 3})
		if err != nil || len(page1.Entries) != 3 || page1.NextCursor == "" {
			t.Fatalf("page1 = %+v err=%v", page1, err)
		}
		page2, err := s.ListEntries(ctx, scope, nb.ListEntriesRequest{SectionID: sec.ResourceID, Limit: 3, Cursor: page1.NextCursor})
		if err != nil || len(page2.Entries) != 2 || page2.NextCursor != "" {
			t.Fatalf("page2 = %+v err=%v", page2, err)
		}
		seen := map[string]bool{}
		for _, e := range append(page1.Entries, page2.Entries...) {
			seen[e.ID] = true
		}
		if len(seen) != 5 {
			t.Fatalf("pages duplicated or dropped entries: %v", seen)
		}
		// Cursor minted under a different filter is rejected.
		if _, err := s.ListEntries(ctx, scope, nb.ListEntriesRequest{SectionID: "section-notes", Cursor: page1.NextCursor}); !errors.Is(err, nb.ErrInvalidRequest) {
			t.Fatalf("cross-filter cursor = %v, want invalid_request", err)
		}
	})
}

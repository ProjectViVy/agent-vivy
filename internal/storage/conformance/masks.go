package conformance

import (
	"context"
	"errors"
	"sync"
	"testing"

	"agent-vivy/internal/domain"
	mask "agent-vivy/internal/maskcontract"
	"agent-vivy/internal/storage"
)

// RunMasks exercises the focused MaskStore extension against every Core
// Storage backend. It is separate from the historical CN suite so the
// existing Engine contract remains stable while the optional mask capability
// is proved by both SQLite and PostgreSQL.
func RunMasks(t *testing.T, h Harness) {
	t.Helper()
	cases := []struct {
		name string
		run  func(*testing.T, Harness)
	}{
		{"crud-cas-and-idempotent-create", maskCRUD},
		{"selection-capture-and-delete-race", maskSelectionRace},
		{"fork-copies-selection-and-serializes-delete", maskForkAndDelete},
		{"session-delete-reopen-removes-selection-preserves-catalog", maskReopen},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) { test.run(t, h) })
	}
}

func maskCRUD(t *testing.T, h Harness) {
	b := fresh(t, h)
	ctx := context.Background()
	store := requireMaskStore(t, b)
	if err := b.CreateSession(ctx, domain.Session{ID: "mask-crud", Title: "mask", CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	request := conformanceMaskRequest("000000000001", "CRUD")
	created, err := store.CreateCustomMask(ctx, request)
	if err != nil {
		t.Fatalf("CreateCustomMask: %v", err)
	}
	if created.BuiltIn || created.Revision != 1 || created.ID == "" || created.Digest == "" {
		t.Fatalf("created = %+v, want custom revision 1", created)
	}
	retry, err := store.CreateCustomMask(ctx, request)
	if err != nil || retry.ID != created.ID {
		t.Fatalf("same operation retry = %+v/%v, want original id", retry, err)
	}
	page, err := store.ListCustomMasks(ctx, mask.ListRequest{Limit: 1})
	if err != nil || len(page.Items) != 1 || page.Items[0].ID != created.ID {
		t.Fatalf("list = %+v/%v", page, err)
	}
	updated, err := store.UpdateCustomMask(ctx, mask.UpdateRequest{
		ID: created.ID, ExpectedRevision: 1, Name: "CRUD updated", Description: "updated", Body: "updated body",
	})
	if err != nil || updated.Revision != 2 || updated.Digest == created.Digest {
		t.Fatalf("update = %+v/%v", updated, err)
	}
	retry, err = store.CreateCustomMask(ctx, request)
	if err != nil || retry.ID != created.ID || retry.Revision != 2 || retry.Body != updated.Body {
		t.Fatalf("same operation retry after edit = %+v/%v, want current original-id definition", retry, err)
	}
	divergent := request
	divergent.Body = "different original body"
	if _, err := store.CreateCustomMask(ctx, divergent); !maskHasCode(err, mask.CodeRevisionConflict) {
		t.Fatalf("divergent operation retry after edit = %v, want revision_conflict", err)
	}
	if _, err := store.UpdateCustomMask(ctx, mask.UpdateRequest{
		ID: created.ID, ExpectedRevision: 1, Name: "stale", Body: "body",
	}); !maskHasCode(err, mask.CodeRevisionConflict) {
		t.Fatalf("stale positive update = %v, want revision_conflict", err)
	}
	if _, err := store.DeleteCustomMask(ctx, mask.DeleteRequest{ID: created.ID, ExpectedRevision: 1}); !maskHasCode(err, mask.CodeRevisionConflict) {
		t.Fatalf("stale delete = %v, want revision_conflict", err)
	}
	if _, err := store.DeleteCustomMask(ctx, mask.DeleteRequest{ID: created.ID, ExpectedRevision: 2}); err != nil {
		t.Fatalf("delete = %v", err)
	}
	if _, err := store.GetCustomMask(ctx, created.ID); !maskHasCode(err, mask.CodeNotFound) {
		t.Fatalf("get deleted = %v, want not_found", err)
	}
}

func maskSelectionRace(t *testing.T, h Harness) {
	b := fresh(t, h)
	ctx := context.Background()
	store := requireMaskStore(t, b)
	for _, id := range []domain.SessionID{"mask-select-a", "mask-select-b"} {
		if err := b.CreateSession(ctx, domain.Session{ID: id, Title: string(id), CreatedAt: 1}); err != nil {
			t.Fatal(err)
		}
	}
	created, err := store.CreateCustomMask(ctx, conformanceMaskRequest("000000000002", "Race"))
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := store.SetMaskSelection(ctx, mask.SetSelectionRequest{
				SessionID: "mask-select-a", MaskID: created.ID, ExpectedRevision: 0,
			})
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	wins, conflicts := 0, 0
	for err := range results {
		if err == nil {
			wins++
		} else if maskHasCode(err, mask.CodeRevisionConflict) {
			conflicts++
		} else {
			t.Fatalf("selection race error = %v", err)
		}
	}
	if wins != 1 || conflicts != 1 {
		t.Fatalf("selection race wins/conflicts = %d/%d, want 1/1", wins, conflicts)
	}
	capture, err := store.ReadMaskCapture(ctx, "mask-select-a")
	if err != nil || capture.Mask == nil || capture.Selection.Revision != 1 {
		t.Fatalf("selected capture = %+v/%v", capture, err)
	}
	if _, err := store.SetMaskSelection(ctx, mask.SetSelectionRequest{SessionID: "mask-select-a", MaskID: "", ExpectedRevision: 1}); err != nil {
		t.Fatal(err)
	}
	deleteDone := make(chan error, 1)
	selectDone := make(chan error, 1)
	go func() {
		_, err := store.DeleteCustomMask(ctx, mask.DeleteRequest{ID: created.ID, ExpectedRevision: 1})
		deleteDone <- err
	}()
	go func() {
		_, err := store.SetMaskSelection(ctx, mask.SetSelectionRequest{
			SessionID: "mask-select-b", MaskID: created.ID, ExpectedRevision: 0,
		})
		selectDone <- err
	}()
	deleteErr, selectErr := <-deleteDone, <-selectDone
	if deleteErr == nil {
		if !maskHasCode(selectErr, mask.CodeNotFound) {
			t.Fatalf("delete won, selection error = %v, want not_found", selectErr)
		}
		capture, err = store.ReadMaskCapture(ctx, "mask-select-b")
		if err != nil || capture.Selection.MaskID != "" {
			t.Fatalf("deleted race left selection = %+v/%v", capture.Selection, err)
		}
	} else {
		if !maskHasCode(deleteErr, mask.CodeMaskInUse) || selectErr != nil {
			t.Fatalf("selection won delete/selection errors = %v/%v", deleteErr, selectErr)
		}
		capture, err = store.ReadMaskCapture(ctx, "mask-select-b")
		if err != nil || capture.Mask == nil || capture.Mask.ID != created.ID {
			t.Fatalf("selected race capture = %+v/%v", capture, err)
		}
	}
}

func maskForkAndDelete(t *testing.T, h Harness) {
	b := fresh(t, h)
	ctx := context.Background()
	store := requireMaskStore(t, b)
	if err := b.CreateSession(ctx, domain.Session{ID: "mask-fork-source", Title: "source", CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetMaskSelection(ctx, mask.SetSelectionRequest{
		SessionID: "mask-fork-source", MaskID: mask.BuiltinResearcherID, ExpectedRevision: 0,
	}); err != nil {
		t.Fatal(err)
	}
	markers := []storage.SessionTruncation{{
		SessionID: "mask-fork-source", Reason: storage.TruncationFork, ForkSessionID: "mask-fork-child",
	}}
	if _, err := b.CommitSessionFork(ctx, domain.Session{ID: "mask-fork-child", Title: "child", CreatedAt: 2}, nil, markers, nil); err != nil {
		t.Fatalf("CommitSessionFork: %v", err)
	}
	child, err := store.ReadMaskCapture(ctx, "mask-fork-child")
	if err != nil || child.Selection.MaskID != mask.BuiltinResearcherID || child.Selection.Revision != 1 {
		t.Fatalf("child selection = %+v/%v", child.Selection, err)
	}
	if err := b.DeleteSession(ctx, "mask-fork-source"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ReadMaskCapture(ctx, "mask-fork-source"); !maskHasCode(err, mask.CodeNotFound) {
		t.Fatalf("deleted source capture = %v, want not_found", err)
	}
	if _, err := store.ReadMaskCapture(ctx, "mask-fork-child"); err != nil {
		t.Fatalf("child after source deletion = %v", err)
	}

	if err := b.CreateSession(ctx, domain.Session{ID: "mask-custom-fork-source", Title: "custom source", CreatedAt: 3}); err != nil {
		t.Fatal(err)
	}
	custom, err := store.CreateCustomMask(ctx, conformanceMaskRequest("000000000004", "Fork race"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetMaskSelection(ctx, mask.SetSelectionRequest{
		SessionID: "mask-custom-fork-source", MaskID: custom.ID, ExpectedRevision: 0,
	}); err != nil {
		t.Fatal(err)
	}
	customMarkers := []storage.SessionTruncation{{
		SessionID: "mask-custom-fork-source", Reason: storage.TruncationFork, ForkSessionID: "mask-custom-fork-child",
	}}
	forkDone := make(chan error, 1)
	deleteDone := make(chan error, 1)
	go func() {
		_, err := b.CommitSessionFork(ctx, domain.Session{ID: "mask-custom-fork-child", Title: "custom child", CreatedAt: 4}, nil, customMarkers, nil)
		forkDone <- err
	}()
	go func() {
		_, err := store.DeleteCustomMask(ctx, mask.DeleteRequest{ID: custom.ID, ExpectedRevision: 1})
		deleteDone <- err
	}()
	if forkErr, deleteErr := <-forkDone, <-deleteDone; forkErr != nil || !maskHasCode(deleteErr, mask.CodeMaskInUse) {
		t.Fatalf("custom fork/delete race = fork %v, delete %v; want fork success and mask_in_use", forkErr, deleteErr)
	}
	customChild, err := store.ReadMaskCapture(ctx, "mask-custom-fork-child")
	if err != nil || customChild.Mask == nil || customChild.Mask.ID != custom.ID || customChild.Selection.Revision != 1 {
		t.Fatalf("custom fork child capture = %+v/%v", customChild, err)
	}
}

func maskReopen(t *testing.T, h Harness) {
	slot := h.Setup(t)
	ctx := context.Background()
	b := slot.Engine
	store := requireMaskStore(t, b)
	if err := b.CreateSession(ctx, domain.Session{ID: "mask-reopen", Title: "mask", CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	created, err := store.CreateCustomMask(ctx, conformanceMaskRequest("000000000003", "Reopen"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetMaskSelection(ctx, mask.SetSelectionRequest{SessionID: "mask-reopen", MaskID: created.ID, ExpectedRevision: 0}); err != nil {
		t.Fatal(err)
	}
	if err := b.DeleteSession(ctx, "mask-reopen"); err != nil {
		t.Fatalf("delete selected session: %v", err)
	}
	if err := b.Close(); err != nil {
		t.Fatal(err)
	}
	b, err = slot.Reopen()
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	t.Cleanup(func() { _ = b.Close() })
	store = requireMaskStore(t, b)
	loaded, err := store.GetCustomMask(ctx, created.ID)
	if err != nil || loaded.ID != created.ID {
		t.Fatalf("reopened custom mask = %+v/%v", loaded, err)
	}
	if _, err := store.ReadMaskCapture(ctx, "mask-reopen"); !maskHasCode(err, mask.CodeNotFound) {
		t.Fatalf("deleted session capture after reopen = %v, want not_found", err)
	}
	if err := b.CreateSession(ctx, domain.Session{ID: "mask-reopen", Title: "recreated", CreatedAt: 2}); err != nil {
		t.Fatalf("recreate deleted session: %v", err)
	}
	capture, err := store.ReadMaskCapture(ctx, "mask-reopen")
	if err != nil || capture.Selection.MaskID != "" || capture.Selection.Revision != 0 || capture.Mask != nil {
		t.Fatalf("recreated session inherited deleted selection = %+v/%v", capture, err)
	}
}

func requireMaskStore(t *testing.T, b storage.Engine) storage.MaskStore {
	t.Helper()
	store, ok := b.(storage.MaskStore)
	if !ok {
		t.Fatal("backend does not implement MaskStore")
	}
	return store
}

func conformanceMaskRequest(last, name string) mask.CreateRequest {
	return mask.CreateRequest{
		OperationID: "00000000-0000-4000-8000-" + last,
		Name:        name, Description: "conformance mask", Body: "Use a concise testing voice.",
	}
}

func maskHasCode(err error, code string) bool {
	var typed *mask.Error
	return errors.As(err, &typed) && typed.Code == code
}

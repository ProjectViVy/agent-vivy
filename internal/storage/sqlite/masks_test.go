package sqlite

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"agent-vivy/internal/domain"
	mask "agent-vivy/internal/maskcontract"
	"agent-vivy/internal/storage"
)

func maskRequest(n string) mask.CreateRequest {
	return mask.CreateRequest{
		OperationID: fmt.Sprintf("00000000-0000-4000-8000-%012d", len(n)+1),
		Name:        n,
		Description: "custom test mask",
		Body:        "Use a concise testing voice.",
	}
}

func TestMaskStoreLifecycle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "masks.db")
	ctx := context.Background()
	b, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	store := maskStore(t, b)
	if err := b.CreateSession(ctx, domain.Session{ID: "mask-session", Title: "mask", CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}

	request := maskRequest("Test Mask")
	created, err := store.CreateCustomMask(ctx, request)
	if err != nil {
		t.Fatalf("CreateCustomMask: %v", err)
	}
	if created.BuiltIn || created.Revision != 1 || created.ID == "" || created.Digest == "" {
		t.Fatalf("created definition = %+v, want custom revision 1", created)
	}
	retry, err := store.CreateCustomMask(ctx, request)
	if err != nil || retry.ID != created.ID {
		t.Fatalf("idempotent create = %+v/%v, want original definition", retry, err)
	}
	request.Body = "different body"
	if _, err := store.CreateCustomMask(ctx, request); !hasMaskCode(err, mask.CodeRevisionConflict) {
		t.Fatalf("divergent create retry = %v, want revision conflict", err)
	}

	page, err := store.ListCustomMasks(ctx, mask.ListRequest{Limit: 1})
	if err != nil || len(page.Items) != 1 || page.Items[0].ID != created.ID || page.Items[0].Digest != created.Digest {
		t.Fatalf("list custom masks = %+v/%v", page, err)
	}
	loaded, err := store.GetCustomMask(ctx, created.ID)
	if err != nil || loaded.Body != created.Body {
		t.Fatalf("get custom mask = %+v/%v", loaded, err)
	}

	selection, err := store.SetMaskSelection(ctx, mask.SetSelectionRequest{SessionID: "mask-session", MaskID: created.ID, ExpectedRevision: 0})
	if err != nil || selection.Revision != 1 {
		t.Fatalf("set selection rev0 = %+v/%v", selection, err)
	}
	capture, err := store.ReadMaskCapture(ctx, "mask-session")
	if err != nil || capture.Mask == nil || capture.Mask.ID != created.ID || capture.Mask.Body != created.Body || capture.Selection.Revision != 1 {
		t.Fatalf("capture = %+v/%v", capture, err)
	}
	if _, err := store.DeleteCustomMask(ctx, mask.DeleteRequest{ID: created.ID, ExpectedRevision: 1}); !hasMaskCode(err, mask.CodeMaskInUse) {
		t.Fatalf("delete in use = %v, want mask_in_use", err)
	}
	selection, err = store.SetMaskSelection(ctx, mask.SetSelectionRequest{SessionID: "mask-session", MaskID: "", ExpectedRevision: 1})
	if err != nil || selection.Revision != 2 {
		t.Fatalf("unmask = %+v/%v, want selection revision 2", selection, err)
	}
	if _, err := store.DeleteCustomMask(ctx, mask.DeleteRequest{ID: created.ID, ExpectedRevision: 1}); err != nil {
		t.Fatalf("delete after unmask: %v", err)
	}
	if _, err := store.GetCustomMask(ctx, created.ID); !hasMaskCode(err, mask.CodeNotFound) {
		t.Fatalf("get deleted = %v, want not_found", err)
	}
	if err := b.Close(); err != nil {
		t.Fatal(err)
	}
	b, err = Open(ctx, path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer func() { _ = b.Close() }()
	store = maskStore(t, b)
	capture, err = store.ReadMaskCapture(ctx, "mask-session")
	if err != nil || capture.Selection.MaskID != "" || capture.Selection.Revision != 2 {
		t.Fatalf("reopened selection = %+v/%v", capture.Selection, err)
	}
}

func TestMaskStoreBuiltInAndForkCopy(t *testing.T) {
	b := openBackend(t)
	ctx := context.Background()
	store := maskStore(t, b)
	if err := b.CreateSession(ctx, domain.Session{ID: "mask-source", Title: "mask-source", CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	selection, err := store.SetMaskSelection(ctx, mask.SetSelectionRequest{SessionID: "mask-source", MaskID: mask.BuiltinWriterID, ExpectedRevision: 0})
	if err != nil || selection.MaskID != mask.BuiltinWriterID {
		t.Fatalf("built-in selection = %+v/%v", selection, err)
	}
	capture, err := store.ReadMaskCapture(ctx, "mask-source")
	if err != nil || capture.Mask != nil || capture.Selection.MaskID != mask.BuiltinWriterID {
		t.Fatalf("built-in capture = %+v/%v", capture, err)
	}
	markers := []storage.SessionTruncation{{
		SessionID: "mask-source", Reason: storage.TruncationFork, ForkSessionID: "mask-child",
	}}
	if _, err := b.CommitSessionFork(ctx, domain.Session{ID: "mask-child", Title: "child", CreatedAt: 2}, nil, markers, nil); err != nil {
		t.Fatalf("CommitSessionFork: %v", err)
	}
	child, err := store.ReadMaskCapture(ctx, "mask-child")
	if err != nil || child.Selection.MaskID != mask.BuiltinWriterID || child.Selection.Revision != 1 {
		t.Fatalf("fork selection = %+v/%v", child.Selection, err)
	}
	if err := b.DeleteSession(ctx, "mask-source"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ReadMaskCapture(ctx, "mask-source"); !hasMaskCode(err, mask.CodeNotFound) {
		t.Fatalf("deleted source capture = %v, want not_found", err)
	}
	if _, err := store.ReadMaskCapture(ctx, "mask-child"); err != nil {
		t.Fatalf("child selection after source deletion: %v", err)
	}
}

func maskStore(t *testing.T, b *Backend) storage.MaskStore {
	t.Helper()
	store, ok := any(b).(storage.MaskStore)
	if !ok {
		t.Fatal("backend does not implement MaskStore")
	}
	return store
}

func hasMaskCode(err error, code string) bool {
	var typed *mask.Error
	return errors.As(err, &typed) && typed.Code == code
}

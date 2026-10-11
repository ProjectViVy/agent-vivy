package conformance

import (
	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

type BundleSlot struct {
	Store storage.BundleStore
	Close func() error
}
type BundleHarness struct{ Setup func(*testing.T) BundleSlot }

// RunBundles exercises the backend-independent receipt, cancellation, and
// retained evidence contract against each real Core Storage dialect.
func RunBundles(t *testing.T, h BundleHarness) {
	t.Helper()
	slot := h.Setup(t)
	defer slot.Close()
	ctx := context.Background()
	state, err := slot.Store.ReadBundleState(ctx)
	if err != nil || state.Catalog.Epoch != 0 {
		t.Fatalf("initial toolkit state=%+v, %v", state, err)
	}
	id := "10000000-0000-4000-8000-000000000001"
	digest := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	op, err := slot.Store.BeginBundleOperation(ctx, storage.BeginOperation{ID: id, InputDigest: digest, Kind: "preview", ExpectedEpoch: 0})
	if err != nil || op.Revision != 1 {
		t.Fatalf("begin operation=%+v, %v", op, err)
	}
	retry, err := slot.Store.BeginBundleOperation(ctx, storage.BeginOperation{ID: id, InputDigest: digest, Kind: "preview", ExpectedEpoch: 0})
	if err != nil || retry.ID != op.ID {
		t.Fatalf("replay=%+v, %v", retry, err)
	}
	if _, err := slot.Store.BeginBundleOperation(ctx, storage.BeginOperation{ID: id, InputDigest: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", Kind: "preview", ExpectedEpoch: 0}); err == nil {
		t.Fatal("changed operation payload replayed")
	}
	cancelled, err := slot.Store.CancelBundleOperation(ctx, storage.CancelOperation{ID: id, ExpectedRevision: 1})
	if err != nil || cancelled.State != "cancelled" {
		t.Fatalf("cancel=%+v, %v", cancelled, err)
	}
	payload := []byte("backend conformance snapshot")
	sum := sha256.Sum256(payload)
	snapshot := storage.RunBundleSnapshot{RunID: domain.RunID("bundle-conformance-run"), SchemaVersion: 1, Payload: payload, PayloadSHA256: hex.EncodeToString(sum[:])}
	if err := slot.Store.SaveRunBundleSnapshot(ctx, snapshot); err != nil {
		t.Fatal(err)
	}
	loaded, err := slot.Store.LoadRunBundleSnapshot(ctx, snapshot.RunID)
	if err != nil || string(loaded.Payload) != string(payload) {
		t.Fatalf("snapshot=%+v, %v", loaded, err)
	}
}

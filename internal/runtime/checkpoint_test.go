package runtime

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"testing"

	"agent-vivy/internal/storage/sqlite"
)

func newCheckpointFixture(t *testing.T, engineVersion string) (*VersionedCheckpointStore, *sqlite.Blobs) {
	t.Helper()
	ctx := context.Background()
	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "checkpoints.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = backend.Close() })
	store, err := NewVersionedCheckpointStore(backend.Blobs(), engineVersion)
	if err != nil {
		t.Fatalf("new versioned checkpoint store: %v", err)
	}
	return store, backend.Blobs()
}

func TestVersionedCheckpointStoreRoundTrip(t *testing.T) {
	store, _ := newCheckpointFixture(t, "v0.9.13")
	ctx := context.Background()
	payload := []byte("\x00\x01gob-ish eino checkpoint bytes\xff")

	if err := store.Set(ctx, "ckpt-run_1", payload); err != nil {
		t.Fatalf("set: %v", err)
	}
	got, ok, err := store.Get(ctx, "ckpt-run_1")
	if err != nil || !ok {
		t.Fatalf("get: ok=%v err=%v", ok, err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("payload mismatch: got %x want %x", got, payload)
	}

	_, ok, err = store.Get(ctx, "ckpt-missing")
	if ok || err != nil {
		t.Fatalf("missing id: ok=%v err=%v", ok, err)
	}
}

func TestVersionedCheckpointStoreGenerationFlip(t *testing.T) {
	store, _ := newCheckpointFixture(t, "v0.9.13")
	ctx := context.Background()

	if err := store.Set(ctx, "ckpt-run_2", []byte("generation one")); err != nil {
		t.Fatalf("set first: %v", err)
	}
	if err := store.Set(ctx, "ckpt-run_2", []byte("generation two")); err != nil {
		t.Fatalf("set second: %v", err)
	}
	got, ok, err := store.Get(ctx, "ckpt-run_2")
	if err != nil || !ok {
		t.Fatalf("get: ok=%v err=%v", ok, err)
	}
	if string(got) != "generation two" {
		t.Fatalf("read stale generation: %q", got)
	}
}

func TestVersionedCheckpointStoreChecksumFailClosed(t *testing.T) {
	store, blobs := newCheckpointFixture(t, "v0.9.13")
	ctx := context.Background()

	if err := store.Set(ctx, "ckpt-run_3", []byte("intact payload")); err != nil {
		t.Fatalf("set: %v", err)
	}
	raw, ok, err := blobs.Get(ctx, "ckpt-run_3")
	if err != nil || !ok {
		t.Fatalf("raw get: ok=%v err=%v", ok, err)
	}
	raw[len(raw)-1] ^= 0xff // corrupt one payload byte behind the envelope
	if err := blobs.Put(ctx, "ckpt-run_3", raw); err != nil {
		t.Fatalf("raw put: %v", err)
	}

	_, _, err = store.Get(ctx, "ckpt-run_3")
	if !errors.Is(err, ErrCheckpointCorrupted) {
		t.Fatalf("want corrupted error, got %v", err)
	}
}

func TestVersionedCheckpointStoreEngineVersionFailClosed(t *testing.T) {
	store, blobs := newCheckpointFixture(t, "v0.9.13")
	ctx := context.Background()
	if err := store.Set(ctx, "ckpt-run_4", []byte("payload")); err != nil {
		t.Fatalf("set: %v", err)
	}

	upgraded, err := NewVersionedCheckpointStore(blobs, "v0.10.0")
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	_, _, err = upgraded.Get(ctx, "ckpt-run_4")
	if !errors.Is(err, ErrCheckpointEngineVersionMismatch) {
		t.Fatalf("want version mismatch error, got %v", err)
	}
}

func TestVersionedCheckpointStoreDelete(t *testing.T) {
	store, _ := newCheckpointFixture(t, "v0.9.13")
	ctx := context.Background()
	if err := store.Set(ctx, "ckpt-run_5", []byte("payload")); err != nil {
		t.Fatalf("set: %v", err)
	}
	if err := store.Delete(ctx, "ckpt-run_5"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	_, ok, err := store.Get(ctx, "ckpt-run_5")
	if ok || err != nil {
		t.Fatalf("get after delete: ok=%v err=%v", ok, err)
	}
}

func TestNewVersionedCheckpointStoreRejectsBadDeps(t *testing.T) {
	if _, err := NewVersionedCheckpointStore(nil, "v0.9.13"); err == nil {
		t.Fatal("nil blob store accepted")
	}
	store, _ := newCheckpointFixture(t, "v0.9.13")
	if _, err := NewVersionedCheckpointStore(store.blobs, ""); err == nil {
		t.Fatal("empty engine version accepted")
	}
}

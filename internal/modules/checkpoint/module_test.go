package checkpoint

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"testing"

	"agent-vivy/internal/runtime"
	"agent-vivy/internal/storage/sqlite"
)

func TestCheckpointModulePreservesVersionedEnvelopeAndGenerationFlip(t *testing.T) {
	backend, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "checkpoint.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = backend.Close() })
	provider, err := Compose(backend.Blobs(), "v0.9.13")
	if err != nil {
		t.Fatal(err)
	}
	store := provider.Store()
	if err := store.Set(context.Background(), "run", []byte("first")); err != nil {
		t.Fatal(err)
	}
	if err := store.Set(context.Background(), "run", []byte("second")); err != nil {
		t.Fatal(err)
	}
	got, ok, err := store.Get(context.Background(), "run")
	if err != nil || !ok || !bytes.Equal(got, []byte("second")) {
		t.Fatalf("Get() = %q, %v, %v", got, ok, err)
	}

	other, err := Compose(backend.Blobs(), "v0.10.0")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := other.Store().Get(context.Background(), "run"); !errors.Is(err, runtime.ErrCheckpointEngineVersionMismatch) {
		t.Fatalf("version mismatch error = %v", err)
	}
}

func TestCheckpointModuleOwnsCanonicalCorePort(t *testing.T) {
	descriptor := NewModule().Descriptor()
	if descriptor.Module.ID != ID || len(descriptor.Provides) != 1 || descriptor.Provides[0].Port != Port {
		t.Fatalf("descriptor = %#v", descriptor)
	}
}

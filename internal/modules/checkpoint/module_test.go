package checkpoint

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"agent-vivy/internal/runtime"
	"agent-vivy/internal/storage/sqlite"
)

func TestCheckpointModuleRequiresAdmittedRun(t *testing.T) {
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
	// Runtime owns admitted context construction and the roundtrip/engine/
	// generation matrix. Module composition must preserve that boundary.
	if err := store.Set(context.Background(), "ckpt-run", []byte("unbound")); !errors.Is(err, runtime.ErrCheckpointPromptMismatch) {
		t.Fatalf("unbound module write = %v", err)
	}
	if _, found, err := backend.Blobs().Get(context.Background(), "ckpt-run"); err != nil || found {
		t.Fatalf("module bypassed admission: found=%v err=%v", found, err)
	}
}

func TestCheckpointModuleOwnsCanonicalCorePort(t *testing.T) {
	descriptor := NewModule().Descriptor()
	if descriptor.Module.ID != ID || len(descriptor.Provides) != 1 || descriptor.Provides[0].Port != Port {
		t.Fatalf("descriptor = %#v", descriptor)
	}
}

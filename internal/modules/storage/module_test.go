package storage

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"agent-vivy/internal/config"
	"agent-vivy/internal/domain"
	storagecontract "agent-vivy/internal/storage"
)

func TestStorageModuleOpensSQLiteWithJournalAuthorityIntact(t *testing.T) {
	cfg := config.Config{}
	cfg.Storage.Backend = "sqlite"
	cfg.Storage.SQLite.Path = filepath.Join(t.TempDir(), "vivy.db")
	engine, err := Open(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = engine.Close() })

	runID := domain.RunID("run-storage-module")
	_, err = engine.Append(context.Background(), storagecontract.Commit{RunID: runID, Events: []domain.RunEvent{
		{Type: domain.EventRunCompleted},
		{Type: domain.EventRunFailed},
	}})
	if !errors.Is(err, storagecontract.ErrCommitInvalid) {
		t.Fatalf("two-terminal commit error = %v, want ErrCommitInvalid", err)
	}
}

func TestStorageModuleOwnsCanonicalCorePort(t *testing.T) {
	descriptor := NewModule().Descriptor()
	if descriptor.Module.ID != ID || len(descriptor.Provides) != 1 || descriptor.Provides[0].Port != Port {
		t.Fatalf("descriptor = %#v", descriptor)
	}
}

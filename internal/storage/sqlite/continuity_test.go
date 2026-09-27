package sqlite

import (
	"context"
	"path/filepath"
	"testing"

	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/conformance"
)

func continuitySlot(t *testing.T) conformance.Slot {
	path := filepath.Join(t.TempDir(), "continuity.db")
	open := func() (storage.Engine, error) {
		return Open(context.Background(), path)
	}
	eng, err := open()
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = eng.Close() })
	return conformance.Slot{Engine: eng, Reopen: open, OpenSecond: open}
}

func TestContinuityAtomic(t *testing.T) {
	conformance.AssertContinuityAtomic(t, continuitySlot(t))
}

func TestContinuityRetry(t *testing.T) {
	conformance.AssertContinuityRetry(t, continuitySlot(t))
}

func TestContinuityExpectations(t *testing.T) {
	conformance.AssertContinuityExpectations(t, continuitySlot(t))
}

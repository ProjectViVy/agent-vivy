package sqlite

import (
	"context"
	"path/filepath"
	"testing"

	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/conformance"
)

func TestBackendConformance(t *testing.T) {
	conformance.Run(t, conformance.Harness{
		DualOpen: conformance.DualOpenShared,
		Setup: func(t *testing.T) conformance.Slot {
			path := filepath.Join(t.TempDir(), "conformance.db")
			open := func() (storage.Engine, error) {
				return Open(context.Background(), path)
			}
			eng, err := open()
			if err != nil {
				t.Fatalf("Open: %v", err)
			}
			return conformance.Slot{
				Engine:     eng,
				Reopen:     open,
				OpenSecond: open,
			}
		},
	})
}

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
	conformance.RunMasks(t, conformance.Harness{
		DualOpen: conformance.DualOpenShared,
		Setup: func(t *testing.T) conformance.Slot {
			path := filepath.Join(t.TempDir(), "mask-conformance.db")
			open := func() (storage.Engine, error) {
				return Open(context.Background(), path)
			}
			eng, err := open()
			if err != nil {
				t.Fatalf("Open: %v", err)
			}
			return conformance.Slot{Engine: eng, Reopen: open, OpenSecond: open}
		},
	})
}

func TestBundleStoreConformance(t *testing.T) {
	conformance.RunBundles(t, conformance.BundleHarness{Setup: func(t *testing.T) conformance.BundleSlot {
		b, err := Open(context.Background(), filepath.Join(t.TempDir(), "bundles.db"))
		if err != nil {
			t.Fatal(err)
		}
		return conformance.BundleSlot{Store: b.BundleStore(), Close: b.Close}
	}})
}

func TestHistoryConformance(t *testing.T) {
	b := openBackend(t)
	conformance.RunHistoryQuerySuite(t, b)
}

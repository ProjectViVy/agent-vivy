package postgres

import (
	"context"
	"fmt"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/conformance"
)

var schemaSeq atomic.Uint64

func TestBackendConformance(t *testing.T) {
	dsn := os.Getenv("VIVY_POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("VIVY_POSTGRES_TEST_DSN not set")
	}
	conformance.Run(t, conformance.Harness{
		DualOpen: conformance.DualOpenExclusive,
		Setup: func(t *testing.T) conformance.Slot {
			schema := fmt.Sprintf("cn_%d_%d", time.Now().UnixNano(), schemaSeq.Add(1))
			open := func() (storage.Engine, error) {
				return OpenSchema(context.Background(), dsn, schema)
			}
			eng, err := open()
			if err != nil {
				t.Fatalf("OpenSchema: %v", err)
			}
			return conformance.Slot{
				Engine:     eng,
				Reopen:     open,
				OpenSecond: open,
			}
		},
	})
}

package postgres

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/conformance"
)

func continuitySlot(t *testing.T) conformance.Slot {
	dsn := os.Getenv("VIVY_POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("VIVY_POSTGRES_TEST_DSN not set")
	}
	schema := fmt.Sprintf("continuity_%d_%d", time.Now().UnixNano(), schemaSeq.Add(1))
	open := func() (storage.Engine, error) {
		return OpenSchema(context.Background(), dsn, schema)
	}
	eng, err := open()
	if err != nil {
		t.Fatalf("OpenSchema: %v", err)
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

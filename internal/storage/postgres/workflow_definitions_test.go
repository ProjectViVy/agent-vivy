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

var workflowDefinitionSchemaSeq atomic.Uint64

func workflowDefinitionSlot(t *testing.T) conformance.Slot {
	t.Helper()
	dsn := os.Getenv("VIVY_POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("VIVY_POSTGRES_TEST_DSN not set")
	}
	schema := fmt.Sprintf("wdef_%d_%d", time.Now().UnixNano(), workflowDefinitionSchemaSeq.Add(1))
	open := func() (storage.Engine, error) { return OpenSchema(context.Background(), dsn, schema) }
	b, err := open()
	if err != nil {
		t.Fatalf("OpenSchema: %v", err)
	}
	t.Cleanup(func() { _ = b.Close() })
	return conformance.Slot{
		Engine: b,
		Reopen: func() (storage.Engine, error) {
			_ = b.Close()
			reopened, err := open()
			if err != nil {
				return nil, err
			}
			t.Cleanup(func() { _ = reopened.Close() })
			return reopened, nil
		},
	}
}

func TestWorkflowDefinitionContractPostgres(t *testing.T) {
	conformance.AssertWorkflowDefinitionContract(t, workflowDefinitionSlot(t))
}

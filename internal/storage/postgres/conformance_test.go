package postgres

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"agent-vivy/internal/domain"
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

func TestHistoryConformance(t *testing.T) {
	dsn := os.Getenv("VIVY_POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("VIVY_POSTGRES_TEST_DSN not set")
	}
	schema := fmt.Sprintf("history_%d_%d", time.Now().UnixNano(), schemaSeq.Add(1))
	b, err := OpenSchema(context.Background(), dsn, schema)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = b.Close() })
	conformance.RunHistoryQuerySuite(t, b)
}

func TestWorkspaceUpdateSerializesWithFirstRunAcrossTransactions(t *testing.T) {
	dsn := os.Getenv("VIVY_POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("VIVY_POSTGRES_TEST_DSN not set")
	}
	ctx := context.Background()
	schema := fmt.Sprintf("workspace_race_%d_%d", time.Now().UnixNano(), schemaSeq.Add(1))
	backend, err := OpenSchema(ctx, dsn, schema)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = backend.Close() })
	const sessionID = domain.SessionID("session-workspace-race")
	if err := backend.CreateSession(ctx, domain.Session{ID: sessionID, CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}

	blocker, err := backend.db.SQL.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = blocker.Rollback() }()
	var locked string
	if err := blocker.QueryRowContext(ctx, `SELECT id FROM sessions WHERE id = $1 FOR UPDATE`, sessionID).Scan(&locked); err != nil {
		t.Fatal(err)
	}
	createDone := make(chan error, 1)
	go func() {
		createDone <- backend.CreateRun(ctx, domain.Run{ID: "run-workspace-race", SessionID: sessionID, Status: domain.RunActive, CreatedAt: 2})
	}()
	waitForPostgresLockWait(t, backend, `SELECT id FROM sessions WHERE id = $1 FOR UPDATE`, 1)
	updateDone := make(chan error, 1)
	go func() { updateDone <- backend.UpdateSessionWorkspace(ctx, sessionID, "/late") }()
	waitForPostgresLockWait(t, backend, `SELECT id FROM sessions WHERE id = $1 FOR UPDATE`, 2)
	if err := blocker.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := <-createDone; err != nil {
		t.Fatalf("create run: %v", err)
	}
	if err := <-updateDone; !errors.Is(err, storage.ErrConflict) {
		t.Fatalf("workspace update = %v, want conflict after first run", err)
	}
}

func waitForPostgresLockWait(t *testing.T, backend *Backend, query string, minimum int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var waiting int
		err := backend.db.SQL.QueryRow(`SELECT COUNT(*) FROM pg_stat_activity
			WHERE wait_event_type = 'Lock' AND query = $1`, query).Scan(&waiting)
		if err == nil && waiting >= minimum {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for PostgreSQL row-lock waiter: %s", query)
}

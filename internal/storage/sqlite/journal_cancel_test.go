package sqlite

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	moderncsqlite "modernc.org/sqlite"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

// Inject at the real driver's commit boundary: all Append statements execute
// against SQLite, then the transaction rolls back and returns the chosen error.
// This deterministically exercises the error shape database/sql can produce
// when its context watcher rolls back before Commit wins the transaction.
type journalCommitFaultConnector struct {
	path   string
	fault  error
	cancel context.CancelFunc
}

func (c *journalCommitFaultConnector) Driver() driver.Driver { return &moderncsqlite.Driver{} }

func (c *journalCommitFaultConnector) Connect(context.Context) (driver.Conn, error) {
	conn, err := c.Driver().Open("file:" + c.path)
	if err != nil {
		return nil, err
	}
	return &journalCommitFaultConn{Conn: conn, fault: c}, nil
}

type journalCommitFaultConn struct {
	driver.Conn
	fault *journalCommitFaultConnector
}

func (c *journalCommitFaultConn) Begin() (driver.Tx, error) {
	tx, err := c.Conn.Begin()
	if err != nil {
		return nil, err
	}
	return &journalCommitFaultTx{Tx: tx, fault: c.fault}, nil
}

type journalCommitFaultTx struct {
	driver.Tx
	fault *journalCommitFaultConnector
}

func (tx *journalCommitFaultTx) Commit() error {
	err := tx.fault.fault
	if err == nil {
		return tx.Tx.Commit()
	}
	tx.fault.fault = nil
	if tx.fault.cancel != nil {
		tx.fault.cancel()
	}
	if rollbackErr := tx.Tx.Rollback(); rollbackErr != nil {
		return fmt.Errorf("fault fixture rollback: %w", rollbackErr)
	}
	return err
}

func TestJournalAppendPreservesCancelledTransactionCause(t *testing.T) {
	storageFailure := errors.New("commit storage failure")
	for _, tc := range []struct {
		name      string
		fault     error
		cancel    bool
		wantCause bool
	}{
		{name: "cancelled transaction", fault: sql.ErrTxDone, cancel: true, wantCause: true},
		{name: "active transaction", fault: sql.ErrTxDone},
		{name: "genuine failure during cancellation", fault: storageFailure, cancel: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "journal.db")
			initial, err := Open(context.Background(), path)
			if err != nil {
				t.Fatal(err)
			}
			if err := initial.Close(); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			connector := &journalCommitFaultConnector{path: path, fault: tc.fault}
			if tc.cancel {
				connector.cancel = cancel
			}
			db := sql.OpenDB(connector)
			db.SetMaxOpenConns(1)
			backend := &Backend{db: db}
			t.Cleanup(func() { _ = backend.Close() })
			commit := storage.Commit{RunID: "cancelled-append", Events: []domain.RunEvent{ev(domain.EventRunStarted)}}
			if _, err := backend.Append(ctx, commit); !errors.Is(err, tc.fault) || errors.Is(err, context.Canceled) != tc.wantCause {
				t.Fatalf("Append error = %v; want original cause and cancelled=%t", err, tc.wantCause)
			}
			var count int
			if err := db.QueryRow("SELECT COUNT(*) FROM run_events").Scan(&count); err != nil || count != 0 {
				t.Fatalf("rolled-back events = %d, %v; want 0", count, err)
			}
			if seq, err := backend.Append(context.Background(), commit); err != nil || seq != 1 {
				t.Fatalf("subsequent Append = %d, %v; want 1", seq, err)
			}
			if err := backend.Close(); err != nil {
				t.Fatal(err)
			}
			reopened, err := Open(context.Background(), path)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = reopened.Close() })
			commit.Events = []domain.RunEvent{ev(domain.EventModelDelta)}
			if seq, err := reopened.Append(context.Background(), commit); err != nil || seq != 2 {
				t.Fatalf("reopened Append = %d, %v; want 2", seq, err)
			}
		})
	}
}

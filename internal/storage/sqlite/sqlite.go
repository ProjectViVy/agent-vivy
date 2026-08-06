// Package sqlite is the V0 reference backend for the four Vivy storage
// contracts (D-026, D-031), built on modernc.org/sqlite (pure Go, no
// CGO). SQLite specifics never leak above the contracts (D-027).
package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"

	"agent-vivy/internal/storage"
)

// migrations apply in order; each runs inside its own transaction and
// records itself in schema_migrations. Failure aborts startup (FR-8).
var migrations = []struct {
	version int64
	sql     string
}{
	{1, migration001},
}

// Open opens (or creates) the database at path and applies all pending
// migrations. The returned Backend implements the four storage contracts.
func Open(ctx context.Context, path string) (*Backend, error) {
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		return nil, fmt.Errorf("storage: open sqlite %s: %w", path, err)
	}
	// Single writer keeps SQLite contention trivial without leaking WAL or
	// locking knobs through the contracts (D-027).
	db.SetMaxOpenConns(1)

	b := &Backend{db: db}
	if err := b.migrate(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return b, nil
}

// Backend bundles the four contracts over one database handle. Snapshot
// and blob accessors are separate handles because their Get/Put signatures
// differ; all of them share the same underlying database.
type Backend struct {
	db *sql.DB
}

// Compile-time proof that every contract is satisfied.
var (
	_ storage.Journal       = (*Backend)(nil)
	_ storage.LeaseStore    = (*Backend)(nil)
	_ storage.SnapshotStore = (*Snapshot)(nil)
	_ storage.BlobStore     = (*Blobs)(nil)
)

// Snapshot returns the snapshot handle over this database.
func (b *Backend) Snapshot() *Snapshot { return &Snapshot{db: b.db} }

// Blobs returns the blob (checkpoint) handle over this database.
func (b *Backend) Blobs() *Blobs { return &Blobs{db: b.db} }

// Close releases the database handle.
func (b *Backend) Close() error { return b.db.Close() }

func (b *Backend) migrate(ctx context.Context) error {
	if _, err := b.db.ExecContext(ctx,
		`CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at INTEGER NOT NULL
		)`); err != nil {
		return fmt.Errorf("storage: init schema_migrations: %w", err)
	}

	for _, m := range migrations {
		var n int
		if err := b.db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM schema_migrations WHERE version = ?`, m.version).Scan(&n); err != nil {
			return fmt.Errorf("storage: check migration %d: %w", m.version, err)
		}
		if n > 0 {
			continue
		}

		tx, err := b.db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("storage: begin migration %d: %w", m.version, err)
		}
		if _, err := tx.ExecContext(ctx, m.sql); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("storage: apply migration %d: %w", m.version, err)
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)`,
			m.version, time.Now().UnixMilli()); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("storage: record migration %d: %w", m.version, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("storage: commit migration %d: %w", m.version, err)
		}
	}
	return nil
}

// migration001 creates the full V0 schema (§5.2). snapshots extends the
// sketch: SnapshotStore is one of the four contracts (D-026).
const migration001 = `
CREATE TABLE sessions (
	id TEXT PRIMARY KEY,
	title TEXT NOT NULL,
	created_at INTEGER NOT NULL
);

CREATE TABLE messages (
	id TEXT PRIMARY KEY,
	session_id TEXT NOT NULL,
	run_id TEXT NOT NULL DEFAULT '',
	role TEXT NOT NULL,
	created_at INTEGER NOT NULL,
	content BLOB NOT NULL,
	FOREIGN KEY(session_id) REFERENCES sessions(id)
);

CREATE TABLE runs (
	id TEXT PRIMARY KEY,
	session_id TEXT NOT NULL,
	status TEXT NOT NULL,
	created_at INTEGER NOT NULL,
	FOREIGN KEY(session_id) REFERENCES sessions(id)
);

CREATE TABLE run_events (
	run_id TEXT NOT NULL,
	seq INTEGER NOT NULL,
	type TEXT NOT NULL,
	created_at INTEGER NOT NULL,
	payload_version INTEGER NOT NULL,
	payload BLOB NOT NULL,
	PRIMARY KEY(run_id, seq)
);

CREATE TABLE approvals (
	id TEXT PRIMARY KEY,
	run_id TEXT NOT NULL,
	tool_call_id TEXT NOT NULL,
	decision TEXT NOT NULL,
	expires_at INTEGER NOT NULL
);

CREATE TABLE snapshots (
	key TEXT PRIMARY KEY,
	value BLOB NOT NULL,
	version INTEGER NOT NULL
);

CREATE TABLE checkpoints (
	id TEXT PRIMARY KEY,
	generation INTEGER NOT NULL,
	checksum TEXT NOT NULL,
	created_at INTEGER NOT NULL
);

CREATE TABLE checkpoint_generations (
	id TEXT NOT NULL,
	generation INTEGER NOT NULL,
	blob BLOB NOT NULL,
	PRIMARY KEY(id, generation)
);

CREATE TABLE leases (
	key TEXT PRIMARY KEY,
	owner TEXT NOT NULL,
	expires_at INTEGER NOT NULL
);
`

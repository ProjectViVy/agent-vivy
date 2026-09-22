// Package sqlite is the V0 reference backend for the Vivy storage
// contracts (D-026, D-031), built on modernc.org/sqlite (pure Go, no
// CGO). SQLite specifics never leak above the contracts (D-027).
package sqlite

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"sync"
	"time"

	_ "modernc.org/sqlite"

	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/migrations"
)

const (
	organismLeaseKey = "vivy/organism"
	leaseTTL         = 30 * time.Second
	leaseHeartbeat   = 10 * time.Second
)

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
	if err := migrations.Apply(ctx, db, migrations.SQLite); err != nil {
		_ = db.Close()
		return nil, err
	}
	return b, nil
}

// Backend bundles the application contracts over one database handle. Snapshot
// and blob accessors are separate handles because their Get/Put signatures
// differ; all of them share the same underlying database.
type Backend struct {
	db         *sql.DB
	leaseOwner string
	stopLease  context.CancelFunc
	leaseDone  chan struct{}
	closeOnce  sync.Once
}

// Compile-time proof that every contract is satisfied.
var (
	_ storage.Journal            = (*Backend)(nil)
	_ storage.LeaseStore         = (*Backend)(nil)
	_ storage.ApprovalStore      = (*Backend)(nil)
	_ storage.QuestionStore      = (*Backend)(nil)
	_ storage.SkillRevisionStore = (*Backend)(nil)
	_ storage.TodoStore          = (*Backend)(nil)
	_ storage.ReviewStore        = (*Backend)(nil)
	_ storage.NoteStore          = (*Backend)(nil)
	_ storage.StudioStore        = (*Backend)(nil)
	_ storage.SnapshotStore      = (*Snapshot)(nil)
	_ storage.BlobStore          = (*Blobs)(nil)
	_ storage.Engine             = (*Backend)(nil)
	_ storage.WorkStore          = (*Backend)(nil)
	_ storage.CheckpointOrphaner = (*Backend)(nil)
)

// Snapshot returns the snapshot handle over this database.
func (b *Backend) Snapshot() storage.SnapshotStore { return &Snapshot{db: b.db} }

// Blobs returns the blob (checkpoint) handle over this database.
func (b *Backend) Blobs() storage.BlobStore { return &Blobs{db: b.db} }

// TakeOrganismLease claims the shared-workspace exclusive lease. A second
// process on the same Journal returns storage.ErrLeaseHeld. Tests that
// call Open without this remain concurrent-safe on distinct files.
func (b *Backend) TakeOrganismLease(ctx context.Context) error {
	owner, err := instanceOwner()
	if err != nil {
		return err
	}
	ok, err := b.Acquire(ctx, organismLeaseKey, owner, leaseTTL)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("%w: the shared Vivy workspace is already in use by another process", storage.ErrLeaseHeld)
	}
	b.leaseOwner = owner
	leaseCtx, cancel := context.WithCancel(context.Background())
	b.stopLease = cancel
	b.leaseDone = make(chan struct{})
	go b.heartbeat(leaseCtx)
	return nil
}

func instanceOwner() (string, error) {
	host, _ := os.Hostname()
	var nonce [8]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return "", fmt.Errorf("storage: lease nonce: %w", err)
	}
	return fmt.Sprintf("%s:%d:%s", host, os.Getpid(), hex.EncodeToString(nonce[:])), nil
}

func (b *Backend) heartbeat(ctx context.Context) {
	defer close(b.leaseDone)
	ticker := time.NewTicker(leaseHeartbeat)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ok, err := b.Acquire(ctx, organismLeaseKey, b.leaseOwner, leaseTTL)
			if err != nil || !ok {
				return
			}
		}
	}
}

// Close releases the organism lease (when taken) and the database handle.
func (b *Backend) Close() error {
	var err error
	b.closeOnce.Do(func() {
		if b.stopLease != nil {
			b.stopLease()
		}
		if b.leaseDone != nil {
			select {
			case <-b.leaseDone:
			case <-time.After(leaseHeartbeat + time.Second):
			}
		}
		if b.leaseOwner != "" {
			_ = b.Release(context.Background(), organismLeaseKey, b.leaseOwner)
		}
		err = b.db.Close()
	})
	return err
}

// DropCheckpointPointer is the CN-11 crash-shape hook.
func (b *Backend) DropCheckpointPointer(ctx context.Context, id string) error {
	_, err := b.db.ExecContext(ctx, `DELETE FROM checkpoints WHERE id = ?`, id)
	return err
}

// CountCheckpointGenerations is the CN-11 crash-shape hook.
func (b *Backend) CountCheckpointGenerations(ctx context.Context, id string) (int, error) {
	var n int
	err := b.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM checkpoint_generations WHERE id = ?`, id).Scan(&n)
	return n, err
}

// DeleteCheckpointGenerations is the CN-11 crash-shape hook.
func (b *Backend) DeleteCheckpointGenerations(ctx context.Context, id string) error {
	_, err := b.db.ExecContext(ctx, `DELETE FROM checkpoint_generations WHERE id = ?`, id)
	return err
}

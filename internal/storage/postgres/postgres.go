package postgres

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"regexp"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"

	"agent-vivy/internal/storage"
)

const (
	schemaVersion    = 16
	organismLeaseKey = "vivy/organism"
	leaseTTL         = 30 * time.Second
	leaseHeartbeat   = 10 * time.Second
	defaultMaxConns  = 8
)

var schemaNamePattern = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// Backend is the optional Postgres Journal. One process holds the organism
// lease; a second Open of the same database returns storage.ErrLeaseHeld.
type Backend struct {
	db         *DB
	lockConn   *sql.Conn
	leaseOwner string
	stopLease  context.CancelFunc
	leaseDone  chan struct{}
	closeOnce  sync.Once
}

var (
	_ storage.Engine             = (*Backend)(nil)
	_ storage.CheckpointOrphaner = (*Backend)(nil)
)

// Open connects to DSN, migrates the public schema, and takes the instance lease.
func Open(ctx context.Context, dsn string) (*Backend, error) {
	return open(ctx, dsn, "")
}

// OpenSchema is for tests: each case gets an isolated schema on one DSN.
func OpenSchema(ctx context.Context, dsn, schema string) (*Backend, error) {
	if !schemaNamePattern.MatchString(schema) {
		return nil, fmt.Errorf("storage: invalid postgres schema %q", schema)
	}
	return open(ctx, dsn, schema)
}

func open(ctx context.Context, dsn, schema string) (*Backend, error) {
	if dsn == "" {
		return nil, fmt.Errorf("storage: empty postgres DSN")
	}
	if schema != "" {
		admin, err := openPool(dsn, "")
		if err != nil {
			return nil, err
		}
		_, err = admin.ExecContext(ctx, `CREATE SCHEMA IF NOT EXISTS `+schema)
		_ = admin.Close()
		if err != nil {
			return nil, fmt.Errorf("storage: create schema %s: %w", schema, err)
		}
	}
	raw, err := openPool(dsn, schema)
	if err != nil {
		return nil, err
	}
	raw.SetMaxOpenConns(defaultMaxConns)
	b := &Backend{db: &DB{SQL: raw}}
	if err := b.migrate(ctx); err != nil {
		_ = raw.Close()
		return nil, err
	}
	if err := b.takeSessionLock(ctx); err != nil {
		_ = raw.Close()
		return nil, err
	}
	owner, err := instanceOwner()
	if err != nil {
		_ = b.lockConn.Close()
		_ = raw.Close()
		return nil, err
	}
	ok, err := b.Acquire(ctx, organismLeaseKey, owner, leaseTTL)
	if err != nil {
		_ = b.lockConn.Close()
		_ = raw.Close()
		return nil, err
	}
	if !ok {
		_ = b.lockConn.Close()
		_ = raw.Close()
		return nil, storage.ErrLeaseHeld
	}
	b.leaseOwner = owner
	leaseCtx, cancel := context.WithCancel(context.Background())
	b.stopLease = cancel
	b.leaseDone = make(chan struct{})
	go b.heartbeat(leaseCtx)
	return b, nil
}

func openPool(dsn, schema string) (*sql.DB, error) {
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("storage: parse postgres DSN: %w", err)
	}
	if schema != "" {
		cfg.RuntimeParams["search_path"] = schema
	}
	return stdlib.OpenDB(*cfg), nil
}

func instanceOwner() (string, error) {
	host, _ := os.Hostname()
	var nonce [8]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return "", fmt.Errorf("storage: lease nonce: %w", err)
	}
	return fmt.Sprintf("%s:%d:%s", host, os.Getpid(), hex.EncodeToString(nonce[:])), nil
}

func (b *Backend) migrate(ctx context.Context) error {
	if _, err := b.db.SQL.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version BIGINT PRIMARY KEY,
			applied_at BIGINT NOT NULL
		)`); err != nil {
		return fmt.Errorf("storage: init postgres schema_migrations: %w", err)
	}
	var n int
	if err := b.db.SQL.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM schema_migrations WHERE version = $1`, schemaVersion).Scan(&n); err != nil {
		return fmt.Errorf("storage: check postgres schema version: %w", err)
	}
	if n > 0 {
		return nil
	}
	tx, err := b.db.SQL.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("storage: begin postgres schema: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Version 15 was also used by the pre-reconciliation channel branch. A
	// version-15 database may therefore have the message provenance columns
	// without cron_jobs, so version 16 is a repair migration rather than a
	// second full bootstrap.
	var prior int
	if err := tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM schema_migrations WHERE version = $1`, 15).Scan(&prior); err != nil {
		return fmt.Errorf("storage: check postgres schema version 15: %w", err)
	}
	if prior == 0 {
		var priorV14 int
		if err := tx.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM schema_migrations WHERE version = $1`, 14).Scan(&priorV14); err != nil {
			return fmt.Errorf("storage: check postgres schema version 14: %w", err)
		}
		ddl := schemaV15
		if priorV14 > 0 {
			ddl = schemaV15Upgrade
		}
		if _, err := tx.ExecContext(ctx, ddl); err != nil {
			return fmt.Errorf("storage: apply postgres schema 15: %w", err)
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO schema_migrations (version, applied_at) VALUES ($1, $2)`,
			15, time.Now().UnixMilli()); err != nil {
			return fmt.Errorf("storage: record postgres schema version 15: %w", err)
		}
	}
	if _, err := tx.ExecContext(ctx, schemaV16Upgrade); err != nil {
		return fmt.Errorf("storage: apply postgres schema 16: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO schema_migrations (version, applied_at) VALUES ($1, $2)`,
		16, time.Now().UnixMilli()); err != nil {
		return fmt.Errorf("storage: record postgres schema version 16: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("storage: commit postgres schema: %w", err)
	}
	return nil
}

const sessionLockSQL = `SELECT pg_try_advisory_lock(hashtextextended(current_schema() || '/vivy/organism', 0))`

func (b *Backend) takeSessionLock(ctx context.Context) error {
	conn, err := b.db.SQL.Conn(ctx)
	if err != nil {
		return fmt.Errorf("storage: reserve postgres lock connection: %w", err)
	}
	var locked bool
	if err := conn.QueryRowContext(ctx, sessionLockSQL).Scan(&locked); err != nil {
		_ = conn.Close()
		return fmt.Errorf("storage: take postgres session lock: %w", err)
	}
	if !locked {
		_ = conn.Close()
		return storage.ErrLeaseHeld
	}
	b.lockConn = conn
	return nil
}

func (b *Backend) fence() {
	b.db.fenced.Store(true)
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
			if b.lockConn != nil {
				pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
				err := b.lockConn.PingContext(pingCtx)
				cancel()
				if err != nil {
					b.fence()
					return
				}
			}
			ok, err := b.Acquire(ctx, organismLeaseKey, b.leaseOwner, leaseTTL)
			if err != nil {
				continue
			}
			if !ok {
				b.fence()
				return
			}
		}
	}
}

// Snapshot returns the snapshot handle over this database.
func (b *Backend) Snapshot() storage.SnapshotStore { return &Snapshot{db: b.db} }

// Blobs returns the blob handle over this database.
func (b *Backend) Blobs() storage.BlobStore { return &Blobs{db: b.db} }

// Close releases the organism lease and the pool.
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
		if b.lockConn != nil {
			_ = b.lockConn.Close()
			b.lockConn = nil
		}
		if b.leaseOwner != "" {
			_, _ = b.db.SQL.ExecContext(context.Background(),
				`DELETE FROM leases WHERE key = $1 AND owner = $2`, organismLeaseKey, b.leaseOwner)
		}
		err = b.db.Close()
	})
	return err
}

func (b *Backend) DropCheckpointPointer(ctx context.Context, id string) error {
	_, err := b.db.ExecContext(ctx, `DELETE FROM checkpoints WHERE id = ?`, id)
	return err
}

func (b *Backend) CountCheckpointGenerations(ctx context.Context, id string) (int, error) {
	var n int
	err := b.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM checkpoint_generations WHERE id = ?`, id).Scan(&n)
	return n, err
}

func (b *Backend) DeleteCheckpointGenerations(ctx context.Context, id string) error {
	_, err := b.db.ExecContext(ctx, `DELETE FROM checkpoint_generations WHERE id = ?`, id)
	return err
}

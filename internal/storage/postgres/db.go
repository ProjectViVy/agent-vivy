package postgres

import (
	"context"
	"database/sql"
	"strconv"
	"strings"
	"sync/atomic"

	"agent-vivy/internal/storage"
)

// DB wraps *sql.DB and rebinds `?` placeholders to Postgres `$n`.
type DB struct {
	SQL    *sql.DB
	fenced atomic.Bool
}

func (d *DB) guard() error {
	if d.fenced.Load() {
		return storage.ErrLeaseLost
	}
	return nil
}

func (d *DB) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	if err := d.guard(); err != nil {
		return nil, err
	}
	return d.SQL.ExecContext(ctx, rebind(query), args...)
}

func (d *DB) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	if err := d.guard(); err != nil {
		return nil, err
	}
	return d.SQL.QueryContext(ctx, rebind(query), args...)
}

func (d *DB) QueryRowContext(ctx context.Context, query string, args ...any) *Row {
	if err := d.guard(); err != nil {
		return &Row{err: err}
	}
	return &Row{row: d.SQL.QueryRowContext(ctx, rebind(query), args...)}
}

func (d *DB) BeginTx(ctx context.Context, opts *sql.TxOptions) (*Tx, error) {
	if err := d.guard(); err != nil {
		return nil, err
	}
	tx, err := d.SQL.BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	return &Tx{SQL: tx}, nil
}

func (d *DB) Close() error { return d.SQL.Close() }

// Tx wraps *sql.Tx with the same placeholder rewrite.
type Tx struct {
	SQL *sql.Tx
}

func (t *Tx) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return t.SQL.ExecContext(ctx, rebind(query), args...)
}

func (t *Tx) QueryRowContext(ctx context.Context, query string, args ...any) *Row {
	return &Row{row: t.SQL.QueryRowContext(ctx, rebind(query), args...)}
}

// Row is *sql.Row plus a fence error that Scan surfaces.
type Row struct {
	err error
	row *sql.Row
}

func (r *Row) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	return r.row.Scan(dest...)
}

func (t *Tx) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	return t.SQL.PrepareContext(ctx, rebind(query))
}

func (t *Tx) Commit() error   { return t.SQL.Commit() }
func (t *Tx) Rollback() error { return t.SQL.Rollback() }

func rebind(query string) string {
	n := 0
	var b strings.Builder
	b.Grow(len(query) + 8)
	for i := 0; i < len(query); i++ {
		if query[i] == '?' {
			n++
			b.WriteByte('$')
			b.WriteString(strconv.Itoa(n))
			continue
		}
		b.WriteByte(query[i])
	}
	return b.String()
}

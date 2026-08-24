package postgres

import (
	"context"
	"errors"
	"testing"

	"agent-vivy/internal/storage"
)

func TestFencedWritesReturnLeaseLost(t *testing.T) {
	d := &DB{}
	d.fenced.Store(true)
	if _, err := d.ExecContext(context.Background(), `SELECT 1`); !errors.Is(err, storage.ErrLeaseLost) {
		t.Fatalf("ExecContext = %v, want ErrLeaseLost", err)
	}
	if _, err := d.QueryContext(context.Background(), `SELECT 1`); !errors.Is(err, storage.ErrLeaseLost) {
		t.Fatalf("QueryContext = %v, want ErrLeaseLost", err)
	}
	if _, err := d.BeginTx(context.Background(), nil); !errors.Is(err, storage.ErrLeaseLost) {
		t.Fatalf("BeginTx = %v, want ErrLeaseLost", err)
	}
	err := d.QueryRowContext(context.Background(), `INSERT INTO studio_events VALUES (?,?,?,?)`).Scan()
	if !errors.Is(err, storage.ErrLeaseLost) {
		t.Fatalf("QueryRowContext.Scan = %v, want ErrLeaseLost", err)
	}
}

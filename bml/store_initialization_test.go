package bml

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"modernc.org/sqlite"
)

var cancelledInitializationSequence atomic.Uint64

type cancelOnOpenDriver struct {
	driver.Driver
	cancel context.CancelFunc
}

func (d cancelOnOpenDriver) Open(name string) (driver.Conn, error) {
	conn, err := d.Driver.Open(name)
	if err == nil {
		d.cancel()
	}
	return conn, err
}

func TestInitializationCancellationLeavesReopenableStore(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	dir := t.TempDir()
	path := filepath.Join(dir, storeFileName)
	name := fmt.Sprintf("bml-cancel-initialize-%d", cancelledInitializationSequence.Add(1))
	sql.Register(name, cancelOnOpenDriver{Driver: &sqlite.Driver{}, cancel: cancel})
	db, err := sql.Open(name, fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=busy_timeout(%d)", path, busyTimeoutMS))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := &Store{db: db, path: path, workspaceID: "workspace"}
	if err := store.initialize(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("initialize error = %v, want cancellation", err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenExisting(context.Background(), dir, "workspace")
	if err != nil {
		t.Fatalf("reopen after cancelled initialization: %v", err)
	}
	defer reopened.Close()
	if tables := openStoreTables(t, path); len(tables) != 5 {
		t.Fatalf("incomplete schema after cancellation: %v", tables)
	}
}

func TestPreCancelledInitializationDoesNotCreateStore(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	dir := t.TempDir()
	if _, err := Open(ctx, dir, "workspace"); !errors.Is(err, context.Canceled) {
		t.Fatalf("open error = %v, want cancellation", err)
	}
	if _, err := os.Stat(filepath.Join(dir, storeFileName)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("cancelled open created a database: %v", err)
	}
}

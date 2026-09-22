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
	mask "agent-vivy/internal/maskcontract"
	"agent-vivy/internal/storage"
)

var maskSchemaSeq atomic.Uint64

func TestMaskStoreCRUDAndSelection(t *testing.T) {
	dsn := os.Getenv("VIVY_POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("VIVY_POSTGRES_TEST_DSN not set")
	}
	schema := fmt.Sprintf("mask_%d_%d", time.Now().UnixNano(), maskSchemaSeq.Add(1))
	b, err := OpenSchema(context.Background(), dsn, schema)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = b.Close() })
	ctx := context.Background()
	if err := b.CreateSession(ctx, domain.Session{ID: "mask-session", Title: "mask", CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	store, ok := any(b).(storage.MaskStore)
	if !ok {
		t.Fatal("backend does not implement MaskStore")
	}
	request := mask.CreateRequest{
		OperationID: "00000000-0000-4000-8000-000000000001",
		Name: "Postgres Mask", Description: "test", Body: "Use a concise testing voice.",
	}
	created, err := store.CreateCustomMask(ctx, request)
	if err != nil || created.ID == "" || created.Revision != 1 || created.BuiltIn {
		t.Fatalf("create = %+v/%v", created, err)
	}
	selection, err := store.SetMaskSelection(ctx, mask.SetSelectionRequest{SessionID: "mask-session", MaskID: created.ID, ExpectedRevision: 0})
	if err != nil || selection.Revision != 1 {
		t.Fatalf("set selection = %+v/%v", selection, err)
	}
	capture, err := store.ReadMaskCapture(ctx, "mask-session")
	if err != nil || capture.Mask == nil || capture.Mask.ID != created.ID {
		t.Fatalf("capture = %+v/%v", capture, err)
	}
	if _, err := store.DeleteCustomMask(ctx, mask.DeleteRequest{ID: created.ID, ExpectedRevision: 1}); !hasMaskCode(err, mask.CodeMaskInUse) {
		t.Fatalf("delete in use = %v, want mask_in_use", err)
	}
}

func TestMaskSelectionRevisionZeroSerializesOnDedicatedConnections(t *testing.T) {
	b, ctx := openMaskTestBackend(t, "mask_rev0")
	if err := b.CreateSession(ctx, domain.Session{ID: "mask-rev0", Title: "mask", CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	firstConn, err := b.db.SQL.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = firstConn.Close() }()
	secondConn, err := b.db.SQL.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = secondConn.Close() }()
	var secondPID int
	if err := secondConn.QueryRowContext(ctx, `SELECT pg_backend_pid()`).Scan(&secondPID); err != nil {
		t.Fatal(err)
	}
	firstSQLTx, err := firstConn.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	firstTx := &Tx{SQL: firstSQLTx}
	in := mask.SetSelectionRequest{SessionID: "mask-rev0", MaskID: mask.BuiltinWriterID, ExpectedRevision: 0}
	first, err := b.setMaskSelectionTx(ctx, firstTx, in)
	if err != nil {
		_ = firstTx.Rollback()
		t.Fatal(err)
	}
	if first.Revision != 1 {
		_ = firstTx.Rollback()
		t.Fatalf("first selection = %+v, want revision 1", first)
	}
	secondSQLTx, err := secondConn.BeginTx(ctx, nil)
	if err != nil {
		_ = firstTx.Rollback()
		t.Fatal(err)
	}
	secondTx := &Tx{SQL: secondSQLTx}
	secondDone := make(chan error, 1)
	go func() {
		_, err := b.setMaskSelectionTx(ctx, secondTx, in)
		if err != nil {
			_ = secondTx.Rollback()
		} else {
			err = secondTx.Commit()
		}
		secondDone <- err
	}()
	waitForPostgresBackendLockWait(t, b, secondPID)
	if err := firstTx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := <-secondDone; !hasMaskCode(err, mask.CodeRevisionConflict) {
		t.Fatalf("second revision-0 writer = %v, want revision_conflict", err)
	}
	capture, err := b.ReadMaskCapture(ctx, "mask-rev0")
	if err != nil || capture.Selection.Revision != 1 || capture.Selection.MaskID != mask.BuiltinWriterID {
		t.Fatalf("stored selection = %+v/%v", capture.Selection, err)
	}
}

func TestMaskSelectionSerializesAfterDedicatedDelete(t *testing.T) {
	b, ctx := openMaskTestBackend(t, "mask_delete_order")
	if err := b.CreateSession(ctx, domain.Session{ID: "mask-delete-order", Title: "mask", CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	created, err := b.CreateCustomMask(ctx, mask.CreateRequest{
		OperationID: "00000000-0000-4000-8000-000000000021",
		Name: "delete ordering", Body: "definition selected concurrently",
	})
	if err != nil {
		t.Fatal(err)
	}
	deleteConn, err := b.db.SQL.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = deleteConn.Close() }()
	selectConn, err := b.db.SQL.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = selectConn.Close() }()
	var selectPID int
	if err := selectConn.QueryRowContext(ctx, `SELECT pg_backend_pid()`).Scan(&selectPID); err != nil {
		t.Fatal(err)
	}
	deleteSQLTx, err := deleteConn.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	deleteTx := &Tx{SQL: deleteSQLTx}
	if _, err := b.deleteCustomMaskTx(ctx, deleteTx, mask.DeleteRequest{ID: created.ID, ExpectedRevision: 1}); err != nil {
		_ = deleteTx.Rollback()
		t.Fatal(err)
	}
	selectSQLTx, err := selectConn.BeginTx(ctx, nil)
	if err != nil {
		_ = deleteTx.Rollback()
		t.Fatal(err)
	}
	selectTx := &Tx{SQL: selectSQLTx}
	selectDone := make(chan error, 1)
	go func() {
		_, err := b.setMaskSelectionTx(ctx, selectTx, mask.SetSelectionRequest{
			SessionID: "mask-delete-order", MaskID: created.ID, ExpectedRevision: 0,
		})
		if err != nil {
			_ = selectTx.Rollback()
		} else {
			err = selectTx.Commit()
		}
		selectDone <- err
	}()
	waitForPostgresBackendLockWait(t, b, selectPID)
	if err := deleteTx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := <-selectDone; !hasMaskCode(err, mask.CodeNotFound) {
		t.Fatalf("selection ordered after delete = %v, want not_found", err)
	}
	capture, err := b.ReadMaskCapture(ctx, "mask-delete-order")
	if err != nil || capture.Selection.Revision != 0 || capture.Selection.MaskID != "" {
		t.Fatalf("delete ordering left selection = %+v/%v", capture.Selection, err)
	}
}

func TestReadMaskCaptureSeesOneCommittedDefinitionVersion(t *testing.T) {
	b, ctx := openMaskTestBackend(t, "mask_capture")
	if err := b.CreateSession(ctx, domain.Session{ID: "mask-capture", Title: "mask", CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	created, err := b.CreateCustomMask(ctx, mask.CreateRequest{
		OperationID: "00000000-0000-4000-8000-000000000022",
		Name: "before", Body: "before body",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.SetMaskSelection(ctx, mask.SetSelectionRequest{SessionID: "mask-capture", MaskID: created.ID, ExpectedRevision: 0}); err != nil {
		t.Fatal(err)
	}
	conn, err := b.db.SQL.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	newDigest := mask.DefinitionDigest(created.ID, "after", "after body")
	if _, err := tx.ExecContext(ctx, `UPDATE mask_definitions SET name = $1, body = $2, revision = 2, digest = $3 WHERE id = $4`, "after", []byte("after body"), newDigest, created.ID); err != nil {
		_ = tx.Rollback()
		t.Fatal(err)
	}
	before, err := b.ReadMaskCapture(ctx, "mask-capture")
	if err != nil || before.Mask == nil || before.Mask.Name != "before" || before.Mask.Body != "before body" || before.Mask.DefinitionRevision != 1 || before.Mask.Digest != created.Digest {
		_ = tx.Rollback()
		t.Fatalf("capture during uncommitted update = %+v/%v", before, err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	after, err := b.ReadMaskCapture(ctx, "mask-capture")
	if err != nil || after.Mask == nil || after.Mask.Name != "after" || after.Mask.Body != "after body" || after.Mask.DefinitionRevision != 2 || after.Mask.Digest != newDigest {
		t.Fatalf("capture after committed update = %+v/%v", after, err)
	}
}

func TestCommitSessionForkRollsBackCopiedMaskSelectionAfterLaterFailure(t *testing.T) {
	b, ctx := openMaskTestBackend(t, "mask_fork_rollback")
	for _, session := range []domain.Session{
		{ID: "mask-fork-source", Title: "source", CreatedAt: 1},
		{ID: "mask-collision-owner", Title: "owner", CreatedAt: 1},
	} {
		if err := b.CreateSession(ctx, session); err != nil {
			t.Fatal(err)
		}
	}
	created, err := b.CreateCustomMask(ctx, mask.CreateRequest{
		OperationID: "00000000-0000-4000-8000-000000000023",
		Name: "fork rollback", Body: "selection must roll back",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.SetMaskSelection(ctx, mask.SetSelectionRequest{SessionID: "mask-fork-source", MaskID: created.ID, ExpectedRevision: 0}); err != nil {
		t.Fatal(err)
	}
	if err := b.AppendMessage(ctx, domain.Message{ID: "mask-duplicate-message", SessionID: "mask-collision-owner", Role: domain.RoleUser, Content: "existing", CreatedAt: 2}); err != nil {
		t.Fatal(err)
	}
	markers := []storage.SessionTruncation{{SessionID: "mask-fork-source", Reason: storage.TruncationFork, ForkSessionID: "mask-fork-child"}}
	messages := []domain.Message{{ID: "mask-duplicate-message", SessionID: "mask-fork-child", Role: domain.RoleUser, Content: "copy", CreatedAt: 2}}
	if _, err := b.CommitSessionFork(ctx, domain.Session{ID: "mask-fork-child", Title: "child", CreatedAt: 2}, messages, markers, nil); err == nil {
		t.Fatal("expected duplicate message failure after selection copy")
	}
	if _, err := b.GetSession(ctx, "mask-fork-child"); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("child survived failed fork: %v", err)
	}
	var copied int
	if err := b.db.SQL.QueryRowContext(ctx, `SELECT COUNT(*) FROM session_mask_selections WHERE session_id = $1`, "mask-fork-child").Scan(&copied); err != nil {
		t.Fatal(err)
	}
	if copied != 0 {
		t.Fatalf("copied selection survived failed fork: %d", copied)
	}
}

func openMaskTestBackend(t *testing.T, prefix string) (*Backend, context.Context) {
	t.Helper()
	dsn := os.Getenv("VIVY_POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("VIVY_POSTGRES_TEST_DSN not set; PostgreSQL mask concurrency test requires a real server")
	}
	ctx := context.Background()
	schema := fmt.Sprintf("%s_%d_%d", prefix, time.Now().UnixNano(), maskSchemaSeq.Add(1))
	b, err := OpenSchema(ctx, dsn, schema)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = b.Close() })
	return b, ctx
}

func waitForPostgresBackendLockWait(t *testing.T, b *Backend, pid int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var waiting bool
		err := b.db.SQL.QueryRowContext(context.Background(), `
			SELECT wait_event_type = 'Lock' FROM pg_stat_activity WHERE pid = $1`, pid).Scan(&waiting)
		if err == nil && waiting {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for PostgreSQL backend %d to block on a row lock", pid)
}

func hasMaskCode(err error, code string) bool {
	var typed *mask.Error
	return errors.As(err, &typed) && typed.Code == code
}

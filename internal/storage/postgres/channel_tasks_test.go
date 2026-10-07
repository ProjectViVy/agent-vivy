package postgres

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

var channelTaskSchemaSeq atomic.Int64

func openChannelTaskBackend(t *testing.T) *Backend {
	t.Helper()
	dsn := os.Getenv("VIVY_POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("VIVY_POSTGRES_TEST_DSN not set")
	}
	schema := fmt.Sprintf("channel_tasks_%d_%d", time.Now().UnixNano(), channelTaskSchemaSeq.Add(1))
	backend, err := OpenSchema(context.Background(), dsn, schema)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = backend.Close() })
	return backend
}

func channelTaskFixture(sessionID domain.SessionID, runID domain.RunID, messageID, remote string) storage.ChannelTaskCommit {
	return storage.ChannelTaskCommit{
		PrimaryRunCommit: storage.PrimaryRunCommit{
			Message: domain.Message{ID: messageID, SessionID: sessionID, RunID: runID, Role: domain.RoleUser, CreatedAt: 2, Content: "hello a2a"},
			Run:     domain.Run{ID: runID, SessionID: sessionID, Kind: domain.RunKindPrimary, Status: domain.RunActive, CreatedAt: 2},
			Started: domain.RunEvent{RunID: runID, Type: domain.EventRunStarted, CreatedAt: 2, PayloadVersion: 1, Payload: []byte(`{}`)},
		},
		Scope:      domain.ChannelTaskScope{InstanceKey: "projectvivy/a2a-server:a2a:a2a", PrincipalID: "principal-1"},
		MessageID:  remote,
		InputHash:  sha256.Sum256([]byte("hello a2a")),
		NewSession: &domain.Session{ID: sessionID, CreatedAt: 2},
		Admitted: domain.RunEvent{
			RunID: runID, Type: domain.EventChannelTaskAdmitted, CreatedAt: 2, PayloadVersion: 1,
			Payload: []byte(`{"session_id":"` + string(sessionID) + `","run_id":"` + string(runID) + `","message_id":"` + remote + `"}`),
		},
	}
}

// TestChannelTaskAdmissionFaultMatrix is the PostgreSQL twin of the SQLite
// matrix: identical semantics under a second dialect and real row locks.
func TestChannelTaskAdmissionFaultMatrix(t *testing.T) {
	b := openChannelTaskBackend(t)
	ctx := context.Background()

	res, err := b.CommitChannelTask(ctx, channelTaskFixture("sess_ct_a", "run_ct_a", "msg_ct_a", "remote-1"))
	if err != nil {
		t.Fatalf("CommitChannelTask: %v", err)
	}
	if !res.NewlyCommitted || len(res.Events) != 2 || res.Events[1].Type != domain.EventChannelTaskAdmitted {
		t.Fatalf("first commit = %+v", res)
	}
	if res.Receipt.SessionID != "sess_ct_a" || res.Receipt.RunID != "run_ct_a" || res.Receipt.Operation != "submit" {
		t.Fatalf("receipt = %+v", res.Receipt)
	}

	retry := channelTaskFixture("sess_ct_a2", "run_ct_a2", "msg_ct_a2", "remote-1")
	res2, err := b.CommitChannelTask(ctx, retry)
	if err != nil {
		t.Fatalf("retry: %v", err)
	}
	if res2.NewlyCommitted || res2.Receipt.RunID != "run_ct_a" {
		t.Fatalf("retry = %+v, want original receipt", res2)
	}

	changed := channelTaskFixture("sess_ct_a", "run_ct_a", "msg_ct_a", "remote-1")
	changed.InputHash = sha256.Sum256([]byte("different"))
	if _, err := b.CommitChannelTask(ctx, changed); !errors.Is(err, storage.ErrConflict) {
		t.Fatalf("changed hash = %v, want ErrConflict", err)
	}

	foreign := domain.ChannelTaskScope{InstanceKey: "projectvivy/a2a-server:a2a:a2a", PrincipalID: "principal-2"}
	if _, found, err := b.FindChannelTaskReceipt(ctx, foreign, "remote-1"); err != nil || found {
		t.Fatalf("foreign receipt = found=%v err=%v", found, err)
	}
	if _, err := b.GetChannelTaskOwner(ctx, foreign, "run_ct_a"); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("foreign owner = %v, want ErrNotFound", err)
	}
	adopt := channelTaskFixture("sess_ct_a", "run_ct_x", "msg_ct_x", "remote-x")
	adopt.Scope = foreign
	adopt.NewSession = nil
	if _, err := b.CommitChannelTask(ctx, adopt); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("foreign adoption = %v, want ErrNotFound", err)
	}

	if err := b.CreateSession(ctx, domain.Session{ID: "sess_native", CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	unowned := channelTaskFixture("sess_native", "run_ct_d", "msg_ct_d", "remote-4")
	unowned.NewSession = nil
	if _, err := b.CommitChannelTask(ctx, unowned); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("unowned context = %v, want ErrNotFound", err)
	}
	taken := channelTaskFixture("sess_native", "run_ct_d2", "msg_ct_d2", "remote-5")
	if _, err := b.CommitChannelTask(ctx, taken); !errors.Is(err, storage.ErrConflict) {
		t.Fatalf("taken candidate session = %v, want ErrConflict", err)
	}

	if _, err := b.CommitChannelTask(ctx, channelTaskFixture("sess_ct_e", "run_ct_e", "msg_ct_e", "remote-6")); err != nil {
		t.Fatal(err)
	}
	busy := channelTaskFixture("sess_ct_e", "run_ct_e2", "msg_ct_e2", "remote-7")
	busy.NewSession = nil
	if _, err := b.CommitChannelTask(ctx, busy); !errors.Is(err, storage.ErrWorkRunConflict) {
		t.Fatalf("busy context second submit = %v, want ErrWorkRunConflict", err)
	}

	bad := channelTaskFixture("sess_ct_f", "run_ct_f", "msg_ct_f", "remote-8")
	bad.Admitted.RunID = "run_other"
	bad.Admitted.Payload = []byte(`{}`)
	if _, err := b.CommitChannelTask(ctx, bad); err == nil {
		t.Fatal("inconsistent admission must fail")
	}
	var n int
	if err := b.db.SQL.QueryRowContext(ctx, "SELECT COUNT(*) FROM sessions WHERE id = $1", "sess_ct_f").Scan(&n); err != nil || n != 0 {
		t.Fatalf("orphan session rows = %d %v", n, err)
	}
	if err := b.db.SQL.QueryRowContext(ctx, "SELECT COUNT(*) FROM channel_task_receipts WHERE message_id = $1", "remote-8").Scan(&n); err != nil || n != 0 {
		t.Fatalf("orphan receipts = %d %v", n, err)
	}
	if err := b.db.SQL.QueryRowContext(ctx, "SELECT COUNT(*) FROM channel_task_contexts WHERE session_id = $1", "sess_ct_f").Scan(&n); err != nil || n != 0 {
		t.Fatalf("orphan contexts = %d %v", n, err)
	}
}

// TestChannelTaskOwnershipAndTombstone — PostgreSQL twin of the SQLite
// ownership/tombstone matrix.
func TestChannelTaskOwnershipAndTombstone(t *testing.T) {
	b := openChannelTaskBackend(t)
	ctx := context.Background()
	p1 := domain.ChannelTaskScope{InstanceKey: "projectvivy/a2a-server:a2a:a2a", PrincipalID: "principal-1"}
	p2 := domain.ChannelTaskScope{InstanceKey: "projectvivy/a2a-server:a2a:a2a", PrincipalID: "principal-2"}

	if _, err := b.CommitChannelTask(ctx, channelTaskFixture("sess_own_a", "run_own_a", "msg_own_a", "shared-msg")); err != nil {
		t.Fatal(err)
	}
	other := channelTaskFixture("sess_own_b", "run_own_b", "msg_own_b", "shared-msg")
	other.Scope = p2
	if _, err := b.CommitChannelTask(ctx, other); err != nil {
		t.Fatal(err)
	}
	if _, err := b.GetChannelTaskOwner(ctx, p2, "run_own_a"); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("guessed run owner = %v, want ErrNotFound", err)
	}
	page, err := b.ListChannelTaskRuns(ctx, storage.ChannelTaskRunQuery{Scope: p1, Limit: 10})
	if err != nil || len(page.Runs) != 1 || page.Runs[0].ID != "run_own_a" {
		t.Fatalf("list = %+v %v", page, err)
	}
	if err := b.DeleteSession(ctx, "sess_own_a"); err != nil {
		t.Fatal(err)
	}
	if _, found, err := b.FindChannelTaskReceipt(ctx, p1, "shared-msg"); err != nil || found {
		t.Fatalf("tombstoned receipt = found=%v err=%v", found, err)
	}
	resub := channelTaskFixture("sess_own_a2", "run_own_a2", "msg_own_a2", "shared-msg")
	res, err := b.CommitChannelTask(ctx, resub)
	if err != nil || !res.NewlyCommitted || res.Receipt.SessionID == "sess_own_a" {
		t.Fatalf("resubmit = %+v %v, want fresh session", res, err)
	}
	if _, err := b.CommitChannelTask(ctx, channelTaskFixture("sess_own_c", "run_own_c", "msg_own_c", "remote-c")); err != nil {
		t.Fatal(err)
	}
	if _, err := b.db.ExecContext(ctx, `DELETE FROM runs WHERE id = 'run_own_c'`); err != nil {
		t.Fatal(err)
	}
	if _, err := b.ListChannelTaskRuns(ctx, storage.ChannelTaskRunQuery{Scope: p1, SessionID: "sess_own_c"}); err == nil {
		t.Fatal("missing run under live receipt must report corruption")
	}
}

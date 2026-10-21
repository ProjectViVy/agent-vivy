package postgres

import (
	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/migrations"
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"
	"time"
)

type d09ChildInserter interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func insertD09DeleteChild(t *testing.T, ctx context.Context, db d09ChildInserter, id string) {
	t.Helper()
	if _, err := db.ExecContext(ctx, `INSERT INTO sessions(id,title,created_at) VALUES($1,'child',1)`, id); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO child_sessions(child_session_id,origin_parent_session_id,origin_parent_run_id,authorizer_run_id,initial_activation_run_id,activation_run_id,operation_key,request_digest,authority_ceiling_digest,authority_ceiling_json,activation_operation_key,activation_request_digest,activation_tools_json,state,created_at,updated_at)
 VALUES($1,'z-parent','parent-run','parent-run','child-run','child-run',$1,'fixture','fixture','{}',$1,'fixture','[]','open',1,1)`, id); err != nil {
		t.Fatal(err)
	}
}
func waitD09BlockedBy(t *testing.T, ctx context.Context, db *sql.DB, pid, blocker int) {
	t.Helper()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		var blocked bool
		if err := db.QueryRowContext(ctx, `SELECT $2 = ANY(pg_blocking_pids($1))`, pid, blocker).Scan(&blocked); err != nil {
			t.Fatal(err)
		}
		if blocked {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatal("expected lock interleave not reached")
		case <-ticker.C:
		}
	}
}
func TestD09ReferenceSessionLocksDoNotCycleWithSubtreeDeletePostgres(t *testing.T) {
	base, db, manifest := generationFixturePool(t)
	buildGenerationPrefix(t, base, db, manifest, len(manifest.Migrations(migrations.Postgres)))
	ctx, cancel := context.WithTimeout(base, 10*time.Second)
	defer cancel()
	if _, err := db.ExecContext(ctx, `INSERT INTO sessions(id,title,created_at) VALUES('z-parent','parent',1)`); err != nil {
		t.Fatal(err)
	}
	insertD09DeleteChild(t, ctx, db, "a-child")
	var schema string
	if err := db.QueryRowContext(ctx, `SELECT current_schema()`).Scan(&schema); err != nil {
		t.Fatal(err)
	}
	deleter, err := openPool(os.Getenv("VIVY_POSTGRES_TEST_DSN"), schema)
	if err != nil {
		t.Fatal(err)
	}
	defer deleter.Close()
	deleter.SetMaxOpenConns(1)
	var deletePID int
	if err := deleter.QueryRowContext(ctx, `SELECT pg_backend_pid()`).Scan(&deletePID); err != nil {
		t.Fatal(err)
	}
	actor, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer actor.Rollback()
	var actorPID int
	if err := actor.QueryRowContext(ctx, `SELECT pg_backend_pid()`).Scan(&actorPID); err != nil {
		t.Fatal(err)
	}
	var locked string
	if err := actor.QueryRowContext(ctx, `SELECT id FROM sessions WHERE id='a-child' FOR UPDATE`).Scan(&locked); err != nil {
		t.Fatal(err)
	}
	result := make(chan error, 1)
	go func() { result <- (&Backend{db: &DB{SQL: deleter}}).DeleteSession(ctx, "z-parent") }()
	waitD09BlockedBy(t, ctx, db, deletePID, actorPID)
	// Continue the actual shared source-lock path after its first canonical lock.
	lockErr := postgresLockContinuitySessions(ctx, actor, storage.ContinuityAdmission{SessionID: "z-parent", Expectations: []storage.SourceExpectation{{SourceSessionID: "a-child", TruncationMarkers: 0}}})
	_ = actor.Rollback()
	deleteErr := <-result
	if lockErr != nil || deleteErr != nil {
		t.Fatalf("reference/delete lock cycle: lock=%v delete=%v", lockErr, deleteErr)
	}
}
func TestD09SubtreeDeleteRechecksMembershipAfterCanonicalLocksPostgres(t *testing.T) {
	base, db, manifest := generationFixturePool(t)
	buildGenerationPrefix(t, base, db, manifest, len(manifest.Migrations(migrations.Postgres)))
	ctx, cancel := context.WithTimeout(base, 10*time.Second)
	defer cancel()
	if _, err := db.ExecContext(ctx, `INSERT INTO sessions(id,title,created_at) VALUES('z-parent','parent',1)`); err != nil {
		t.Fatal(err)
	}
	insertD09DeleteChild(t, ctx, db, "a-child")
	var schema string
	if err := db.QueryRowContext(ctx, `SELECT current_schema()`).Scan(&schema); err != nil {
		t.Fatal(err)
	}
	deleter, err := openPool(os.Getenv("VIVY_POSTGRES_TEST_DSN"), schema)
	if err != nil {
		t.Fatal(err)
	}
	defer deleter.Close()
	deleter.SetMaxOpenConns(1)
	var deletePID int
	if err := deleter.QueryRowContext(ctx, `SELECT pg_backend_pid()`).Scan(&deletePID); err != nil {
		t.Fatal(err)
	}
	creator, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer creator.Rollback()
	var creatorPID int
	if err := creator.QueryRowContext(ctx, `SELECT pg_backend_pid()`).Scan(&creatorPID); err != nil {
		t.Fatal(err)
	}
	var locked string
	if err := creator.QueryRowContext(ctx, `SELECT id FROM sessions WHERE id='z-parent' FOR UPDATE`).Scan(&locked); err != nil {
		t.Fatal(err)
	}
	result := make(chan error, 1)
	go func() { result <- (&Backend{db: &DB{SQL: deleter}}).DeleteSession(ctx, "z-parent") }()
	waitD09BlockedBy(t, ctx, db, deletePID, creatorPID)
	insertD09DeleteChild(t, ctx, creator, "b-new-child")
	if err := creator.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := <-result; !errors.Is(err, storage.ErrConflict) {
		t.Fatalf("changed subtree must refuse before writes: %v", err)
	}
	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sessions WHERE id IN ('z-parent','a-child','b-new-child')`).Scan(&count); err != nil || count != 3 {
		t.Fatalf("membership refusal partially deleted rows: %d %v", count, err)
	}
}

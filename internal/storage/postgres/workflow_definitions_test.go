package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/conformance"
)

var workflowDefinitionSchemaSeq atomic.Uint64

func workflowDefinitionSlot(t *testing.T) conformance.Slot {
	t.Helper()
	dsn := os.Getenv("VIVY_POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("VIVY_POSTGRES_TEST_DSN not set")
	}
	schema := fmt.Sprintf("wdef_%d_%d", time.Now().UnixNano(), workflowDefinitionSchemaSeq.Add(1))
	open := func() (storage.Engine, error) { return OpenSchema(context.Background(), dsn, schema) }
	b, err := open()
	if err != nil {
		t.Fatalf("OpenSchema: %v", err)
	}
	t.Cleanup(func() { _ = b.Close() })
	return conformance.Slot{
		Engine: b,
		Reopen: func() (storage.Engine, error) {
			_ = b.Close()
			reopened, err := open()
			if err != nil {
				return nil, err
			}
			t.Cleanup(func() { _ = reopened.Close() })
			return reopened, nil
		},
	}
}

func TestWorkflowDefinitionContractPostgres(t *testing.T) {
	conformance.AssertWorkflowDefinitionContract(t, workflowDefinitionSlot(t))
}

func TestWorkflowDraftLegacyETagRotates(t *testing.T) {
	ctx := context.Background()
	slot := workflowDefinitionSlot(t)
	b := slot.Engine.(*Backend)
	const sessionID = "sess-legacy-etag"
	const workflowID = "wf-legacy"
	const legacyETag = "wf-legacy:0123456789abcdef:1001"
	if err := b.CreateSession(ctx, domain.Session{ID: domain.SessionID(sessionID), CreatedAt: 1}); err != nil {
		t.Fatalf("create author session: %v", err)
	}
	original := []byte(`{"legacy":"before"}`)
	legacyDigest := postgresWorkflowTestDigest(original)
	if _, err := b.db.ExecContext(ctx, `INSERT INTO workflow_definition_drafts
		(workflow_id,etag,artifact,definition_digest,artifact_digest,archived,author_session_id,created_at,updated_at)
		VALUES (?,?,?,?,?,0,?,?,?)`, workflowID, legacyETag, original, legacyDigest, legacyDigest, sessionID, 1000, 1000); err != nil {
		t.Fatalf("seed legacy draft: %v", err)
	}
	updatedArtifact := []byte(`{"legacy":"after"}`)
	updatedDigest := postgresWorkflowTestDigest(updatedArtifact)
	updated, err := b.UpdateWorkflowDraftCAS(ctx, sessionID, storage.WorkflowDraftUpdate{
		WorkflowID: workflowID, ExpectedETag: legacyETag, ArtifactJSON: updatedArtifact,
		DefinitionDigest: updatedDigest, ArtifactDigest: updatedDigest, Now: 1000,
	})
	if err != nil {
		t.Fatalf("edit legacy draft: %v", err)
	}
	if updated.ETag == legacyETag {
		t.Fatalf("legacy token remained valid after edit: %q", updated.ETag)
	}
	reopenedEngine, err := slot.Reopen()
	if err != nil {
		t.Fatalf("reopen backend: %v", err)
	}
	reopened := reopenedEngine.(*Backend)
	d, err := reopened.GetWorkflowDraft(ctx, sessionID, workflowID)
	if err != nil {
		t.Fatalf("read reopened draft: %v", err)
	}
	if d.ETag != updated.ETag || string(d.ArtifactJSON) != string(updatedArtifact) || d.CreatedAt != 1000 || d.UpdatedAt != 1000 {
		t.Fatalf("reopened draft changed: %+v", d)
	}
	if _, err := reopened.UpdateWorkflowDraftCAS(ctx, sessionID, storage.WorkflowDraftUpdate{
		WorkflowID: workflowID, ExpectedETag: legacyETag, ArtifactJSON: original,
		DefinitionDigest: legacyDigest, ArtifactDigest: legacyDigest, Now: 1001,
	}); !errors.Is(err, storage.ErrWorkflowDefinitionConflict) {
		t.Fatalf("legacy token should conflict after reopen, got %v", err)
	}
	final, err := reopened.GetWorkflowDraft(ctx, sessionID, workflowID)
	if err != nil || string(final.ArtifactJSON) != string(updatedArtifact) {
		t.Fatalf("legacy token changed reopened artifact: draft=%+v err=%v", final, err)
	}
}

func TestWorkflowDraftConcurrentAbsentInsertConflict(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	slot := workflowDefinitionSlot(t)
	b := slot.Engine.(*Backend)
	const sessionID = "sess-absent-race"
	const workflowID = "wf-absent-race"
	if err := b.CreateSession(ctx, domain.Session{ID: domain.SessionID(sessionID), CreatedAt: 1}); err != nil {
		t.Fatalf("create author session: %v", err)
	}
	blocker, err := b.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin table-lock transaction: %v", err)
	}
	blockerOpen := true
	defer func() {
		if blockerOpen {
			_ = blocker.Rollback()
		}
	}()
	if _, err := blocker.ExecContext(ctx, `LOCK TABLE workflow_definition_drafts IN SHARE MODE`); err != nil {
		t.Fatalf("hold table lock: %v", err)
	}
	results := make(chan error, 2)
	for _, tag := range []string{"race-a", "race-b"} {
		go func(tag string) {
			artifact := []byte(fmt.Sprintf(`{"creator":%q}`, tag))
			digest := postgresWorkflowTestDigest(artifact)
			_, err := b.UpdateWorkflowDraftCAS(ctx, sessionID, storage.WorkflowDraftUpdate{
				WorkflowID: workflowID, ExpectedETag: storage.WorkflowDefinitionETagAbsent,
				ArtifactJSON: artifact, DefinitionDigest: digest, ArtifactDigest: digest, Now: 1000,
			})
			results <- err
		}(tag)
	}
	deadline := time.NewTimer(10 * time.Second)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer deadline.Stop()
	defer ticker.Stop()
	for {
		var waiting int
		if err := b.db.QueryRowContext(ctx, `SELECT count(*) FROM pg_locks l
			JOIN pg_class c ON c.oid=l.relation
			JOIN pg_namespace n ON n.oid=c.relnamespace
			WHERE l.locktype='relation' AND n.nspname=current_schema()
			AND c.relname='workflow_definition_drafts' AND l.mode='RowExclusiveLock' AND NOT l.granted`).Scan(&waiting); err != nil {
			t.Fatalf("inspect pending insert locks: %v", err)
		}
		if waiting >= 2 {
			break
		}
		select {
		case <-deadline.C:
			t.Fatalf("timed out waiting for both absent-token inserts to block; saw %d", waiting)
		case <-ctx.Done():
			t.Fatalf("test context ended while waiting for inserts: %v", ctx.Err())
		case <-ticker.C:
		}
	}
	if err := blocker.Commit(); err != nil {
		t.Fatalf("release table lock: %v", err)
	}
	blockerOpen = false
	successes, conflicts := 0, 0
	for range 2 {
		select {
		case err := <-results:
			switch {
			case err == nil:
				successes++
			case errors.Is(err, storage.ErrWorkflowDefinitionConflict):
				var pgErr *pgconn.PgError
				if !errors.As(err, &pgErr) || pgErr.Code != "23505" || pgErr.ConstraintName != "workflow_definition_drafts_pkey" {
					t.Fatalf("conflict lost its PostgreSQL cause: %v", err)
				}
				conflicts++
			default:
				t.Fatalf("absent-token insert returned unexpected error: %v", err)
			}
		case <-ctx.Done():
			t.Fatalf("timed out receiving insert result: %v", ctx.Err())
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("concurrent creates yielded %d successes and %d conflicts; want one each", successes, conflicts)
	}
	winner, err := b.GetWorkflowDraft(ctx, sessionID, workflowID)
	if err != nil {
		t.Fatalf("read winning draft: %v", err)
	}
	if string(winner.ArtifactJSON) != `{"creator":"race-a"}` && string(winner.ArtifactJSON) != `{"creator":"race-b"}` {
		t.Fatalf("unexpected winning artifact: %s", winner.ArtifactJSON)
	}
}

func postgresWorkflowTestDigest(value []byte) string {
	sum := sha256.Sum256(value)
	return "inofy-normal-v1:sha256:" + hex.EncodeToString(sum[:])
}

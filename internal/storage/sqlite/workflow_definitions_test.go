package sqlite

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/conformance"
	"agent-vivy/internal/storage/migrations"
)

func workflowDefinitionSlot(t *testing.T) conformance.Slot {
	t.Helper()
	path := filepath.Join(t.TempDir(), "vivy-definitions.db")
	open := func() (storage.Engine, error) { return Open(context.Background(), path) }
	b, err := open()
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = b.Close() })
	return conformance.Slot{
		Engine: b,
		Reopen: func() (storage.Engine, error) {
			reopened, err := open()
			if err != nil {
				return nil, err
			}
			t.Cleanup(func() { _ = reopened.Close() })
			return reopened, nil
		},
	}
}

func TestWorkflowDefinitionContract(t *testing.T) {
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
	legacyDigest := workflowTestDigest(original)
	if _, err := b.db.ExecContext(ctx, `INSERT INTO workflow_definition_drafts
		(workflow_id,etag,artifact,definition_digest,artifact_digest,archived,author_session_id,created_at,updated_at)
		VALUES (?,?,?,?,?,0,?,?,?)`, workflowID, legacyETag, original, legacyDigest, legacyDigest, sessionID, 1000, 1000); err != nil {
		t.Fatalf("seed legacy draft: %v", err)
	}
	updatedArtifact := []byte(`{"legacy":"after"}`)
	updatedDigest := workflowTestDigest(updatedArtifact)
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

func workflowTestDigest(value []byte) string {
	sum := sha256.Sum256(value)
	return "inofy-normal-v1:sha256:" + hex.EncodeToString(sum[:])
}

// TestWorkflowDefinitionUpgradeFrom034 seeds a database at migration head
// 034 then opens it through the production runner: the definitions
// migration must apply in place.
func TestWorkflowDefinitionUpgradeFrom034(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "vivy-upgrade.db")
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open raw sqlite: %v", err)
	}
	manifest, err := migrations.Embedded()
	if err != nil {
		t.Fatalf("manifest: %v", err)
	}
	if _, err := raw.ExecContext(ctx, `CREATE TABLE schema_migrations (
		version BIGINT PRIMARY KEY, name TEXT NOT NULL, checksum TEXT NOT NULL, applied_at BIGINT NOT NULL)`); err != nil {
		t.Fatalf("create schema_migrations: %v", err)
	}
	var applied int64
	for _, m := range manifest.Migrations(migrations.SQLite) {
		if m.Version > 34 {
			break
		}
		if _, err := raw.ExecContext(ctx, m.SQL); err != nil {
			t.Fatalf("apply %d %s: %v", m.Version, m.Name, err)
		}
		if _, err := raw.ExecContext(ctx,
			`INSERT INTO schema_migrations (version, name, checksum, applied_at) VALUES (?,?,?,?)`,
			m.Version, m.Name, m.Checksum, time.Now().UnixMilli()); err != nil {
			t.Fatalf("record %d: %v", m.Version, err)
		}
		applied = m.Version
	}
	if applied != 34 {
		t.Fatalf("fixture stopped at migration %d, want 34", applied)
	}
	if err := raw.Close(); err != nil {
		t.Fatalf("close fixture db: %v", err)
	}

	b, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("Open after upgrade: %v", err)
	}
	defer func() { _ = b.Close() }()
	var count int
	if err := b.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM workflow_definition_drafts`).Scan(&count); err != nil {
		t.Fatalf("upgraded draft table: %v", err)
	}
	if err := b.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM workflow_definition_revisions`).Scan(&count); err != nil {
		t.Fatalf("upgraded revision table: %v", err)
	}
	var ncol int
	if err := b.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM pragma_table_info('workflow_revisions') WHERE name = 'definition_id'`).Scan(&ncol); err != nil || ncol != 1 {
		t.Fatalf("definition_id column missing after upgrade: %d err=%v", ncol, err)
	}
}

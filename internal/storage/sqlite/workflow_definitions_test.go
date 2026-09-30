package sqlite

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

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

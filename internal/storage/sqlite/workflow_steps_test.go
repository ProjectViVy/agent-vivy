package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/conformance"
	"agent-vivy/internal/storage/migrations"
)

func workflowStepSlot(t *testing.T) conformance.Slot {
	t.Helper()
	path := filepath.Join(t.TempDir(), "vivy-steps.db")
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

func TestWorkflowStepContract(t *testing.T) {
	conformance.AssertWorkflowStepContract(t, workflowStepSlot(t))
}

// TestWorkflowStepUpgradeFrom033 seeds a database at migration head 033 then
// opens it through the production runner: the workflow-step migration must
// apply in place and the committed state must stay readable.
func TestWorkflowStepUpgradeFrom033(t *testing.T) {
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
		if m.Version > 33 {
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
	if applied != 33 {
		t.Fatalf("fixture stopped at migration %d, want 33", applied)
	}
	if err := raw.Close(); err != nil {
		t.Fatalf("close fixture db: %v", err)
	}

	b, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("Open after upgrade: %v", err)
	}
	defer func() { _ = b.Close() }()
	slot := conformance.Slot{Engine: b}
	// A committed step over the upgraded schema must work end to end.
	wf, s := conformance.WorkflowStepFixture(t, slot, "upg")
	c := conformance.NewStepCommit(wf, "u1", 1, "", storage.WorkflowStepAdmitted, "upg")
	c.Events = []storage.WorkflowStepEvent{conformance.StepAdmitEvent("upg")}
	if _, err := s.CommitWorkflowStep(ctx, c); err != nil {
		t.Fatalf("commit over upgraded schema: %v", err)
	}
	st, err := s.LoadWorkflowStep(ctx, wf)
	if err != nil || st.Projection == nil || st.Projection.Status != storage.WorkflowStepAdmitted {
		t.Fatalf("load after upgrade = %+v err=%v", st, err)
	}
	var count int
	if err := b.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM workflow_revisions WHERE workflow_run_id = ? AND program_digest IS NOT NULL`, wf).Scan(&count); err != nil || count != 1 {
		t.Fatalf("upgraded revision identity columns: count=%d err=%v", count, err)
	}
	fmt.Println("upgraded schema at head", applied, "-> current")
}

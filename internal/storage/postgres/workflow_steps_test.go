package postgres

import (
	"context"
	"fmt"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/conformance"
	"agent-vivy/internal/storage/migrations"
)

var workflowStepSchemaSeq atomic.Uint64

func workflowStepSlot(t *testing.T) (conformance.Slot, string) {
	t.Helper()
	dsn := os.Getenv("VIVY_POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("VIVY_POSTGRES_TEST_DSN not set")
	}
	schema := fmt.Sprintf("wstep_%d_%d", time.Now().UnixNano(), workflowStepSchemaSeq.Add(1))
	open := func() (storage.Engine, error) { return OpenSchema(context.Background(), dsn, schema) }
	b, err := open()
	if err != nil {
		t.Fatalf("OpenSchema: %v", err)
	}
	t.Cleanup(func() { _ = b.Close() })
	return conformance.Slot{
		Engine: b,
		Reopen: func() (storage.Engine, error) {
			_ = b.Close() // release the organism lease before the reopen
			reopened, err := open()
			if err != nil {
				return nil, err
			}
			t.Cleanup(func() { _ = reopened.Close() })
			return reopened, nil
		},
	}, dsn
}

func TestWorkflowStepContractPostgres(t *testing.T) {
	slot, _ := workflowStepSlot(t)
	conformance.AssertWorkflowStepContract(t, slot)
}

// TestWorkflowStepUpgradeFrom033Postgres seeds a schema at migration head 033
// (skipping the metadata/lookup maintenance rows the runner records on a real
// open) and verifies the new migration applies in place.
func TestWorkflowStepUpgradeFrom033Postgres(t *testing.T) {
	slot, dsn := workflowStepSlot(t)
	_ = slot
	ctx := context.Background()
	schema := fmt.Sprintf("wupg_%d_%d", time.Now().UnixNano(), workflowStepSchemaSeq.Add(1))

	admin, err := openPool(dsn, "")
	if err != nil {
		t.Fatalf("admin pool: %v", err)
	}
	t.Cleanup(func() {
		_, _ = admin.ExecContext(context.Background(), `DROP SCHEMA IF EXISTS `+schema+` CASCADE`)
		_ = admin.Close()
	})
	if _, err := admin.ExecContext(ctx, `CREATE SCHEMA `+schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	setup, err := openPool(dsn, schema)
	if err != nil {
		t.Fatalf("setup pool: %v", err)
	}
	manifest, err := migrations.Embedded()
	if err != nil {
		t.Fatalf("manifest: %v", err)
	}
	if _, err := setup.ExecContext(ctx, `CREATE TABLE schema_migrations (
		version BIGINT PRIMARY KEY, name TEXT NOT NULL, checksum TEXT NOT NULL, applied_at BIGINT NOT NULL)`); err != nil {
		t.Fatalf("create schema_migrations: %v", err)
	}
	applied := int64(0)
	for _, m := range manifest.Migrations(migrations.Postgres) {
		if m.Version > 33 {
			break
		}
		if _, err := setup.ExecContext(ctx, m.SQL); err != nil {
			t.Fatalf("apply %d %s: %v", m.Version, m.Name, err)
		}
		if _, err := setup.ExecContext(ctx,
			`INSERT INTO schema_migrations (version, name, checksum, applied_at) VALUES ($1,$2,$3,$4)`,
			m.Version, m.Name, m.Checksum, time.Now().UnixMilli()); err != nil {
			t.Fatalf("record %d: %v", m.Version, err)
		}
		applied = m.Version
	}
	_ = setup.Close()
	if applied != 33 {
		t.Fatalf("fixture stopped at migration %d, want 33", applied)
	}

	b, err := OpenSchema(ctx, dsn, schema)
	if err != nil {
		t.Fatalf("OpenSchema after upgrade: %v", err)
	}
	defer func() { _ = b.Close() }()
	slot2 := conformance.Slot{Engine: b}
	wf, s := conformance.WorkflowStepFixture(t, slot2, "pgupg")
	c := conformance.NewStepCommit(wf, "u1", 1, "", storage.WorkflowStepAdmitted, "pgupg")
	c.Events = []storage.WorkflowStepEvent{conformance.StepAdmitEvent("pgupg")}
	if _, err := s.CommitWorkflowStep(ctx, c); err != nil {
		t.Fatalf("commit over upgraded schema: %v", err)
	}
	st, err := s.LoadWorkflowStep(ctx, wf)
	if err != nil || st.Projection == nil || st.Projection.Status != storage.WorkflowStepAdmitted {
		t.Fatalf("load after upgrade = %+v err=%v", st, err)
	}
}

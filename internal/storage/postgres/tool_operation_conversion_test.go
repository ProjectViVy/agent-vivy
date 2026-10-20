package postgres

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"agent-vivy/internal/storage/migrations"
)

func TestToolOperationCopiedBaselineConversionPostgres(t *testing.T) {
	dsn := os.Getenv("VIVY_POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("VIVY_POSTGRES_TEST_DSN not set")
	}
	ctx := context.Background()
	sourceSchema := fmt.Sprintf("op_source_%d", time.Now().UnixNano())
	targetSchema := sourceSchema + "_copy"
	admin, err := openPool(dsn, "")
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	defer admin.ExecContext(ctx, `DROP SCHEMA IF EXISTS `+sourceSchema+` CASCADE`)
	defer admin.ExecContext(ctx, `DROP SCHEMA IF EXISTS `+targetSchema+` CASCADE`)
	if _, err := admin.ExecContext(ctx, `CREATE SCHEMA `+sourceSchema+`; CREATE SCHEMA `+targetSchema); err != nil {
		t.Fatal(err)
	}
	source, err := openPool(dsn, sourceSchema)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	manifest, err := migrations.Embedded()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := source.ExecContext(ctx, `CREATE TABLE schema_migrations(version BIGINT PRIMARY KEY,name TEXT NOT NULL,checksum TEXT NOT NULL,applied_at BIGINT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	for _, migration := range manifest.Migrations(migrations.Postgres)[:35] {
		if _, err := source.ExecContext(ctx, migration.SQL); err != nil {
			t.Fatal(err)
		}
		if _, err := source.ExecContext(ctx, `INSERT INTO schema_migrations VALUES($1,$2,$3,1)`, migration.Version, migration.Name, migration.Checksum); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := source.ExecContext(ctx, `
		INSERT INTO sessions(id,title,created_at) VALUES('s','source',1);
		INSERT INTO runs(id,session_id,status,created_at) VALUES('active','s','active',2),('terminal','s','completed',2);
		INSERT INTO approvals(id,run_id,tool_call_id,decision,expires_at) VALUES
		('pending','active','call','pending',50),('approved','active','call','approved',50),('history','terminal','call','approved',50);
		INSERT INTO tool_operations(run_id,operation_id,tool_name,request_digest,middleware_input_arguments,arguments_digest,effective_arguments,state,claim_owner,result,failure,created_at,updated_at)
		SELECT 'active',s,'tool','old-request',convert_to('{}','UTF8'),'old-effective',convert_to('{}','UTF8'),s,'old-owner','old-result','old-failure',11,12
		FROM unnest(ARRAY['admitted','claimed','completed']) AS s;
	`); err != nil {
		t.Fatal(err)
	}
	historicalPayload := []byte(`{"operation_id":"completed","tool_name":"tool","state":"completed","request_digest":"historical-request","arguments_digest":"historical-effective"}`)
	if _, err := source.ExecContext(ctx, `INSERT INTO run_events(run_id,seq,type,created_at,payload_version,payload) VALUES('active',1,'tool.operation',12,1,$1)`, historicalPayload); err != nil {
		t.Fatal(err)
	}
	rows, err := source.QueryContext(ctx, `SELECT table_name FROM information_schema.tables WHERE table_schema=current_schema() AND table_type='BASE TABLE'`)
	if err != nil {
		t.Fatal(err)
	}
	var tables []string
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			t.Fatal(err)
		}
		tables = append(tables, table)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	_ = rows.Close()
	// Copy the quiescent disposable schema. LIKE copies column/PK/index
	// shape; this fixture needs no new sequence or foreign-key writes.
	for _, table := range tables {
		if !schemaNamePattern.MatchString(table) {
			t.Fatalf("unsafe fixture table %q", table)
		}
		if _, err := admin.ExecContext(ctx, `CREATE TABLE `+targetSchema+`.`+table+` (LIKE `+sourceSchema+`.`+table+` INCLUDING ALL); INSERT INTO `+targetSchema+`.`+table+` SELECT * FROM `+sourceSchema+`.`+table); err != nil {
			t.Fatal(err)
		}
	}
	target, err := openPool(dsn, targetSchema)
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close()
	if err := migrations.ApplyManifest(ctx, target, migrations.Postgres, manifest); err != nil {
		t.Fatal(err)
	}
	fragment, err := os.ReadFile("../migrations/transition/postgres/expire_baseline_approvals.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, commit := range []bool{false, true} {
		tx, err := target.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := tx.ExecContext(ctx, string(fragment), 100); err != nil {
			t.Fatal(err)
		}
		if _, err := tx.ExecContext(ctx, `UPDATE runs SET status='failed' WHERE id='active'; INSERT INTO run_events(run_id,seq,type,created_at,payload_version,payload) VALUES('active',2,'run.failed',100,1,convert_to('{}','UTF8'))`); err != nil {
			t.Fatal(err)
		}
		if commit {
			if err := tx.Commit(); err != nil {
				t.Fatal(err)
			}
		} else {
			_ = tx.Rollback()
			var decision string
			if err := target.QueryRowContext(ctx, `SELECT decision FROM approvals WHERE id='pending'`).Scan(&decision); err != nil || decision != "pending" {
				t.Fatalf("fragment escaped rollback: %s %v", decision, err)
			}
		}
	}
	var preservedPayload []byte
	if err := target.QueryRowContext(ctx, `SELECT payload FROM run_events WHERE run_id='active' AND seq=1`).Scan(&preservedPayload); err != nil || !bytes.Equal(preservedPayload, historicalPayload) {
		t.Fatalf("historical lifecycle payload changed: %s %v", preservedPayload, err)
	}
	var terminalCount int
	if err := target.QueryRowContext(ctx, `SELECT COUNT(*) FROM run_events WHERE run_id='active' AND type IN ('run.completed','run.failed','run.cancelled')`).Scan(&terminalCount); err != nil || terminalCount != 1 {
		t.Fatalf("converted terminal count=%d %v", terminalCount, err)
	}
	for id, want := range map[string]string{"pending": "expired", "approved": "expired", "history": "approved"} {
		var decision string
		if err := target.QueryRowContext(ctx, `SELECT decision FROM approvals WHERE id=$1`, id).Scan(&decision); err != nil || decision != want {
			t.Fatalf("converted approval %s: %s %v", id, decision, err)
		}
	}
	for _, state := range []string{"admitted", "claimed", "completed"} {
		var request []byte
		var status, reason, effects sql.NullString
		var actual, owner, result, failure string
		var created, updated int64
		err := target.QueryRowContext(ctx, `SELECT request_arguments,outcome_status,outcome_reason,outcome_effects,state,claim_owner,result,failure,created_at,updated_at FROM tool_operations WHERE operation_id=$1`, state).
			Scan(&request, &status, &reason, &effects, &actual, &owner, &result, &failure, &created, &updated)
		if err != nil || request != nil || status.Valid || reason.Valid || effects.Valid || actual != state || owner != "old-owner" || result != "old-result" || failure != "old-failure" || created != 11 || updated != 12 {
			t.Fatalf("converted historical operation changed: %s %v", state, err)
		}
	}
	if err := migrations.Apply(ctx, target, migrations.Postgres); err != nil {
		t.Fatalf("target reopen: %v", err)
	}
	var count int
	if err := source.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations`).Scan(&count); err != nil || count != 35 {
		t.Fatalf("source metadata changed: %d %v", count, err)
	}
	if err := source.QueryRowContext(ctx, `SELECT COUNT(*) FROM tool_operations WHERE request_digest='old-request' AND arguments_digest='old-effective'`).Scan(&count); err != nil || count != 3 {
		t.Fatalf("source operations changed: %d %v", count, err)
	}
	var decision string
	if err := source.QueryRowContext(ctx, `SELECT decision FROM approvals WHERE id='approved'`).Scan(&decision); err != nil || decision != "approved" {
		t.Fatalf("source authority changed: %s %v", decision, err)
	}
}

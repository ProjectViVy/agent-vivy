package postgres

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
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
	if _, err := source.ExecContext(ctx, `CREATE TABLE schema_migrations(version BIGINT PRIMARY KEY,name TEXT NOT NULL,applied_at BIGINT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	for _, migration := range manifest.Migrations(migrations.Postgres)[:35] {
		if _, err := source.ExecContext(ctx, migration.SQL); err != nil {
			t.Fatal(err)
		}
		if _, err := source.ExecContext(ctx, `INSERT INTO schema_migrations VALUES($1,$2,1)`, migration.Version, migration.Name); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := source.ExecContext(ctx, `
		INSERT INTO sessions(id,title,created_at) VALUES('s','source',1);
		INSERT INTO runs(id,session_id,status,created_at) VALUES('active','s','active',2),('terminal','s','completed',2);
 INSERT INTO run_events(run_id,seq,type,created_at,payload_version,payload) VALUES('terminal',1,'run.completed',3,1,convert_to('{}','UTF8'));
		INSERT INTO approvals(id,run_id,tool_call_id,decision,expires_at) VALUES
		('pending','active','call','pending',50),('approved','active','call','approved',50),('history','terminal','call','approved',50);
		INSERT INTO tool_operations(run_id,operation_id,tool_name,request_digest,middleware_input_arguments,arguments_digest,effective_arguments,state,claim_owner,result,failure,created_at,updated_at)
		SELECT 'active',s,'tool','old-request',convert_to('{}','UTF8'),'old-effective',convert_to('{}','UTF8'),s,'old-owner','old-result','old-failure',11,12
		FROM unnest(ARRAY['admitted','claimed','completed']) AS s;
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := source.ExecContext(ctx, `
 INSERT INTO runs(id,session_id,status,created_at) VALUES('accepted','s','accepted',2),('queued','s','queued',2);
 INSERT INTO questions(id,run_id,tool_call_id,prompt,status,expires_at) VALUES
 ('question','active','call',convert_to('preserved private bytes','UTF8'),'pending',50),('answered','active','call',convert_to('preserved private bytes','UTF8'),'answered',50),('question-history','terminal','call',convert_to('preserved private bytes','UTF8'),'pending',50);
 INSERT INTO checkpoint_generations(id,generation,blob) VALUES ('ckpt-active',1,convert_to('preserved private bytes','UTF8')),('private-owner',1,convert_to('preserved private bytes','UTF8'));
 INSERT INTO checkpoints(id,generation,checksum,created_at) VALUES ('ckpt-active',1,'old-checksum',1),('private-owner',1,'old-checksum',1);
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
	restoreCopiedWorkflowIndexName(t, ctx, target)
	finishMigrationFixture(t, ctx, target, manifest, 35)
	fragment, err := os.ReadFile("../migrations/transition/postgres/expire_baseline_approvals.sql")
	if err != nil {
		t.Fatal(err)
	}
	settlement, err := os.ReadFile("../migrations/transition/postgres/settle_baseline_runs.sql")
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
		if _, err := tx.ExecContext(ctx, strings.ReplaceAll(string(settlement), "$1", "100")); err != nil {
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
			var pointers int
			if err := target.QueryRow(`SELECT COUNT(*) FROM checkpoints WHERE id='ckpt-active'`).Scan(&pointers); err != nil || pointers != 1 {
				t.Fatalf("rollback checkpoint=%d %v", pointers, err)
			}
			var questionStatus string
			if err := target.QueryRow(`SELECT status FROM questions WHERE id='question'`).Scan(&questionStatus); err != nil || questionStatus != "pending" {
				t.Fatalf("rollback question=%s %v", questionStatus, err)
			}

		}
	}
	for id, want := range map[string]string{"active": "failed", "accepted": "cancelled", "queued": "cancelled", "terminal": "completed"} {
		var actual string
		query := `SELECT status FROM runs WHERE id = $1`
		if err := target.QueryRowContext(ctx, query, id).Scan(&actual); err != nil || actual != want {
			t.Fatalf("converted Run %s=%s, want %s: %v", id, actual, want, err)
		}
		var count int
		if err := target.QueryRowContext(ctx, `SELECT COUNT(*) FROM run_events WHERE run_id=$1 AND type IN ('run.completed','run.failed','run.cancelled')`, id).Scan(&count); err != nil || count != 1 {
			t.Fatalf("terminal %s=%d %v", id, count, err)
		}
	}
	for id, want := range map[string]string{"question": "expired", "answered": "answered", "question-history": "pending"} {
		var actual string
		if err := target.QueryRowContext(ctx, `SELECT status FROM questions WHERE id=$1`, id).Scan(&actual); err != nil || actual != want {
			t.Fatalf("converted question %s=%s %v", id, actual, err)
		}
	}
	var pointers, generations int
	if err := target.QueryRow(`SELECT COUNT(*) FROM checkpoints`).Scan(&pointers); err != nil || pointers != 1 {
		t.Fatalf("private blob pointer changed: %d %v", pointers, err)
	}
	if err := target.QueryRow(`SELECT COUNT(*) FROM checkpoint_generations`).Scan(&generations); err != nil || generations != 2 {
		t.Fatalf("historical bytes lost: %d %v", generations, err)
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

package migrations

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCopiedBaselineConversionPreservesUnboundHistory(t *testing.T) {
	ctx := context.Background()
	manifest, err := Embedded()
	if err != nil {
		t.Fatal(err)
	}
	baseline := Manifest{byDialect: map[Dialect][]Migration{
		SQLite: manifest.Migrations(SQLite)[:35], Postgres: manifest.Migrations(Postgres)[:35],
	}}
	sourcePath := filepath.Join(t.TempDir(), "source.db")
	source, err := sql.Open("sqlite", sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := applyManifest(ctx, source, SQLite, baseline); err != nil {
		t.Fatal(err)
	}
	if _, err := source.Exec(`
		INSERT INTO sessions(id,title,created_at) VALUES ('s','source',1);
		INSERT INTO runs(id,session_id,status,created_at) VALUES ('active','s','active',2), ('terminal','s','completed',2);
		INSERT INTO run_events(run_id,seq,type,created_at,payload_version,payload) VALUES ('terminal',1,'run.completed',3,1,'{}');
		INSERT INTO approvals(id,run_id,tool_call_id,decision,expires_at) VALUES
		('pending','active','call','pending',50), ('approved','active','call','approved',50),
		('denied','active','call','denied',50), ('terminal-history','terminal','call','approved',50);
	`); err != nil {
		t.Fatal(err)
	}
	for _, state := range []string{"admitted", "claimed", "completed"} {
		if _, err := source.Exec(`INSERT INTO tool_operations
			(run_id,operation_id,tool_name,request_digest,middleware_input_arguments,arguments_digest,effective_arguments,state,claim_owner,result,failure,created_at,updated_at)
			VALUES ('active',?,'write_note','old-request',x'7b7d','old-effective',x'7b7d',?,'old-owner','old-result','old-failure',11,12)`, state, state); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := source.ExecContext(ctx, `
 INSERT INTO runs(id,session_id,status,created_at) VALUES('accepted','s','accepted',2),('queued','s','queued',2);
 INSERT INTO questions(id,run_id,tool_call_id,prompt,status,expires_at) VALUES
 ('question','active','call',CAST('preserved private bytes' AS BLOB),'pending',50),('answered','active','call',CAST('preserved private bytes' AS BLOB),'answered',50),('question-history','terminal','call',CAST('preserved private bytes' AS BLOB),'pending',50);
 INSERT INTO checkpoint_generations(id,generation,blob) VALUES ('ckpt-active',1,CAST('preserved private bytes' AS BLOB)),('private-owner',1,CAST('preserved private bytes' AS BLOB));
 INSERT INTO checkpoints(id,generation,checksum,created_at) VALUES ('ckpt-active',1,'old-checksum',1),('private-owner',1,'old-checksum',1);
 `); err != nil {
		t.Fatal(err)
	}

	historicalPayload := []byte(`{"operation_id":"completed","tool_name":"write_note","state":"completed","request_digest":"historical-request","arguments_digest":"historical-effective"}`)
	if _, err := source.Exec(`INSERT INTO run_events(run_id,seq,type,created_at,payload_version,payload) VALUES('active',1,'tool.operation',12,1,?)`, historicalPayload); err != nil {
		t.Fatal(err)
	}
	if err := source.Close(); err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	targetPath := filepath.Join(t.TempDir(), "target.db")
	if err := os.WriteFile(targetPath, original, 0600); err != nil {
		t.Fatal(err)
	}
	target, err := sql.Open("sqlite", targetPath)
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close()
	if err := applyManifest(ctx, target, SQLite, manifest); err != nil {
		t.Fatal(err)
	}
	fragment, err := os.ReadFile("transition/sqlite/expire_baseline_approvals.sql")
	if err != nil {
		t.Fatal(err)
	}
	settlement, err := os.ReadFile("transition/sqlite/settle_baseline_runs.sql")
	if err != nil {
		t.Fatal(err)
	}

	// D08/D14 supplies the complete settlement in one target transaction.
	// Deliberate rollback proves approval revocation cannot commit alone.
	settle := func(commit bool) {
		t.Helper()
		tx, err := target.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback()
		if _, err := tx.ExecContext(ctx, string(fragment), 100); err != nil {
			t.Fatal(err)
		}
		if _, err := tx.ExecContext(ctx, strings.ReplaceAll(string(settlement), "?1", "100")); err != nil {
			t.Fatal(err)
		}
		if commit {
			if err := tx.Commit(); err != nil {
				t.Fatal(err)
			}
		}
	}
	settle(false)
	var decision, status string
	if err := target.QueryRow(`SELECT decision FROM approvals WHERE id='pending'`).Scan(&decision); err != nil || decision != "pending" {
		t.Fatalf("rollback approval=%s %v", decision, err)
	}
	if err := target.QueryRow(`SELECT status FROM runs WHERE id='active'`).Scan(&status); err != nil || status != "active" {
		t.Fatalf("rollback Run=%s %v", status, err)
	}
	var pointerCount int
	if err := target.QueryRow(`SELECT COUNT(*) FROM checkpoints WHERE id='ckpt-active'`).Scan(&pointerCount); err != nil || pointerCount != 1 {
		t.Fatalf("rollback checkpoint=%d %v", pointerCount, err)
	}
	var questionStatus string
	if err := target.QueryRow(`SELECT status FROM questions WHERE id='question'`).Scan(&questionStatus); err != nil || questionStatus != "pending" {
		t.Fatalf("rollback question=%s %v", questionStatus, err)
	}

	settle(true)
	for id, want := range map[string]string{"active": "failed", "accepted": "cancelled", "queued": "cancelled", "terminal": "completed"} {
		var actual string
		query := `SELECT status FROM runs WHERE id = ?`
		if err := target.QueryRowContext(ctx, query, id).Scan(&actual); err != nil || actual != want {
			t.Fatalf("converted Run %s=%s, want %s: %v", id, actual, want, err)
		}
		var count int
		if err := target.QueryRowContext(ctx, `SELECT COUNT(*) FROM run_events WHERE run_id=? AND type IN ('run.completed','run.failed','run.cancelled')`, id).Scan(&count); err != nil || count != 1 {
			t.Fatalf("terminal %s=%d %v", id, count, err)
		}
	}
	for id, want := range map[string]string{"question": "expired", "answered": "answered", "question-history": "pending"} {
		var actual string
		if err := target.QueryRowContext(ctx, `SELECT status FROM questions WHERE id=?`, id).Scan(&actual); err != nil || actual != want {
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
	if err := target.QueryRow(`SELECT payload FROM run_events WHERE run_id='active' AND seq=1`).Scan(&preservedPayload); err != nil || !bytes.Equal(preservedPayload, historicalPayload) {
		t.Fatalf("historical lifecycle payload changed: %s %v", preservedPayload, err)
	}
	var terminalCount int
	if err := target.QueryRow(`SELECT COUNT(*) FROM run_events WHERE run_id='active' AND type IN ('run.completed','run.failed','run.cancelled')`).Scan(&terminalCount); err != nil || terminalCount != 1 {
		t.Fatalf("converted terminal count=%d %v", terminalCount, err)
	}
	for id, want := range map[string]string{"pending": "expired", "approved": "expired", "denied": "denied", "terminal-history": "approved"} {
		if err := target.QueryRow(`SELECT decision FROM approvals WHERE id=?`, id).Scan(&decision); err != nil || decision != want {
			t.Fatalf("converted approval %s=%s %v", id, decision, err)
		}
	}
	var reason []byte
	if err := target.QueryRow(`SELECT decision_reason FROM approvals WHERE id='approved'`).Scan(&reason); err != nil || string(reason) != "upgrade_recovery_required" {
		t.Fatalf("converted reason=%s %v", reason, err)
	}
	for _, state := range []string{"admitted", "claimed", "completed"} {
		var request []byte
		var actualState, owner, result, failure string
		var status, reason, effects sql.NullString
		var created, updated int64
		err := target.QueryRow(`SELECT request_arguments,state,claim_owner,result,failure,outcome_status,outcome_reason,outcome_effects,created_at,updated_at FROM tool_operations WHERE operation_id=?`, state).
			Scan(&request, &actualState, &owner, &result, &failure, &status, &reason, &effects, &created, &updated)
		if err != nil || request != nil || status.Valid || reason.Valid || effects.Valid || actualState != state || owner != "old-owner" || result != "old-result" || failure != "old-failure" || created != 11 || updated != 12 {
			t.Fatal(fmt.Sprintf("conversion changed historical operation %s: %s/%s/%s/%s/%d/%d, null=%v/%v/%v/%v, %v", state, actualState, owner, result, failure, created, updated, request == nil, status.Valid, reason.Valid, effects.Valid, err))
		}
	}
	if err := Apply(ctx, target, SQLite); err != nil {
		t.Fatalf("converted target reopen: %v", err)
	}
	unchanged, err := os.ReadFile(sourcePath)
	if err != nil || !bytes.Equal(original, unchanged) {
		t.Fatalf("source changed during copy conversion: %v", err)
	}
}

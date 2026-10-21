package postgres

import (
	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/migrations"
	"errors"
	"testing"
)

func TestD09ReceiptLegacyOutcomeAndRollbackPostgres(t *testing.T) {
	ctx, db, manifest := generationFixturePool(t)
	buildGenerationPrefix(t, ctx, db, manifest, 38)
	if _, err := db.ExecContext(ctx, `INSERT INTO sessions(id,title,created_at) VALUES('s','retained',1);
 INSERT INTO runs(id,session_id,status,created_at) VALUES('r','s','completed',2);
 INSERT INTO run_events(run_id,seq,type,created_at,payload_version,payload) VALUES('r',1,'context.reference_attached',3,1,convert_to('{"retained":"outcome"}','UTF8'));
 INSERT INTO continuity_receipts(session_id,operation,request_id,input_hash,run_id,event_seq) VALUES('s','reference.attach','call','unrecoverable','r',1);
 CREATE VIEW continuity_drop_guard AS SELECT input_hash FROM continuity_receipts;`); err != nil {
		t.Fatal(err)
	}
	if err := applyGenerationCopyFixture(ctx, db, manifest); err == nil {
		t.Fatal("dependent-view failure succeeded")
	}
	var retired string
	if err := db.QueryRowContext(ctx, `SELECT input_hash FROM continuity_receipts WHERE request_id='call'`).Scan(&retired); err != nil || retired != "unrecoverable" {
		t.Fatalf("rollback lost old input: %s %v", retired, err)
	}
	var head int
	if err := db.QueryRowContext(ctx, `SELECT MAX(version) FROM schema_migrations`).Scan(&head); err != nil || head != 38 {
		t.Fatalf("rollback metadata=%d: %v", head, err)
	}
	if _, err := db.ExecContext(ctx, `DROP VIEW continuity_drop_guard`); err != nil {
		t.Fatal(err)
	}
	if err := migrations.Apply(ctx, db, migrations.Postgres); err != nil {
		t.Fatal(err)
	}
	backend := &Backend{db: &DB{SQL: db}}
	receipt, found, err := backend.FindContinuityReceipt(ctx, "s", "reference.attach", "call")
	if err != nil || !found || receipt.Input != nil || receipt.RunID != "r" || receipt.EventSeq != 1 {
		t.Fatalf("legacy receipt reconstructed/lost: found=%v err=%v", found, err)
	}
	mutation := storage.ContinuityMutation{SessionID: "s", RunID: "r", Event: domain.RunEvent{RunID: "r", Type: domain.EventContextReferenceAttached, PayloadVersion: 1, Payload: []byte(`{}`)}, Receipt: domain.ContinuityReceipt{SessionID: "s", Operation: "reference.attach", RequestID: "call", Input: []byte(`{"selection":"unrecoverable"}`)}}
	if _, err := backend.CommitContinuityOperation(ctx, mutation); !errors.Is(err, storage.ErrConflict) {
		t.Fatalf("legacy receipt replayed: %v", err)
	}
	var payload []byte
	if err := db.QueryRowContext(ctx, `SELECT payload FROM run_events WHERE run_id='r' AND seq=1`).Scan(&payload); err != nil || string(payload) != `{"retained":"outcome"}` {
		t.Fatalf("outcome changed: %q %v", payload, err)
	}
	if err := migrations.Apply(ctx, db, migrations.Postgres); err != nil {
		t.Fatal(err)
	}
}

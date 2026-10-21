package sqlite

import (
	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"context"
	"errors"
	"testing"
)

func TestD09ReceiptUnknownInputFailsClosed(t *testing.T) {
	slot := continuitySlot(t)
	backend := slot.Engine.(*Backend)
	ctx := context.Background()
	if _, err := backend.db.ExecContext(ctx, `INSERT INTO sessions(id,title,created_at) VALUES('s','retained',1);
 INSERT INTO runs(id,session_id,status,created_at) VALUES('r','s','completed',2);
 INSERT INTO run_events(run_id,seq,type,created_at,payload_version,payload) VALUES('r',1,'context.reference_attached',3,1,CAST('{"retained":"outcome"}' AS BLOB));
 INSERT INTO continuity_receipts(session_id,operation,request_id,intended_input,run_id,event_seq) VALUES('s','reference.attach','call',NULL,'r',1);`); err != nil {
		t.Fatal(err)
	}
	receipt, found, err := backend.FindContinuityReceipt(ctx, "s", "reference.attach", "call")
	if err != nil || !found || receipt.Input != nil {
		t.Fatalf("unknown input reconstructed: found=%v err=%v", found, err)
	}
	mutation := storage.ContinuityMutation{SessionID: "s", RunID: "r", Event: domain.RunEvent{RunID: "r", Type: domain.EventContextReferenceAttached, PayloadVersion: 1, Payload: []byte(`{}`)}, Receipt: domain.ContinuityReceipt{SessionID: "s", Operation: "reference.attach", RequestID: "call", Input: []byte(`{"selection":"unrecoverable"}`)}}
	if _, err := backend.CommitContinuityOperation(ctx, mutation); !errors.Is(err, storage.ErrConflict) {
		t.Fatalf("unknown input replayed: %v", err)
	}
}

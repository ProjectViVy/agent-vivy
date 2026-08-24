package runtime

import (
	"context"
	"strings"
	"testing"

	"agent-vivy/internal/domain"
)

type auditRecorder struct{ records []AuditRecord }

func (r *auditRecorder) Record(_ context.Context, record AuditRecord) {
	r.records = append(r.records, record)
}

func TestAuditHookStoresOnlyPayloadDigest(t *testing.T) {
	recorder := &auditRecorder{}
	hook := AuditHook{Sink: recorder}
	secret := []byte(`{"token":"sk-live-abcdefghijkl"}`)
	hook.OnRunEvent(context.Background(), domain.RunEvent{
		RunID: "run-audit", Seq: 4, Type: domain.EventModelUsage,
		CreatedAt: 123, Payload: secret,
	})
	if len(recorder.records) != 1 {
		t.Fatalf("records = %+v", recorder.records)
	}
	record := recorder.records[0]
	if record.PayloadBytes != len(secret) || record.PayloadSHA256 == "" || strings.Contains(record.PayloadSHA256, "sk-live") {
		t.Fatalf("audit record = %+v", record)
	}
}

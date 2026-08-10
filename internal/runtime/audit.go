package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"

	"agent-vivy/internal/domain"
)

// AuditRecord is metadata about a durable event. The payload is represented
// by size and a digest so audit logs can correlate events without becoming a
// second transcript or leaking tool arguments/secrets.
type AuditRecord struct {
	RunID         domain.RunID
	Seq           domain.EventSeq
	Type          domain.EventType
	CreatedAt     int64
	PayloadBytes  int
	PayloadSHA256 string
}

// AuditSink receives bounded event metadata after the event is durable and
// visible to the normal live sink. It is advisory and cannot change state.
type AuditSink interface {
	Record(context.Context, AuditRecord)
}

// AuditHook adapts an AuditSink to the lifecycle hook seam.
type AuditHook struct {
	Sink AuditSink
}

func (h AuditHook) OnRunEvent(ctx context.Context, ev domain.RunEvent) {
	if h.Sink == nil {
		return
	}
	sum := sha256.Sum256(ev.Payload)
	h.Sink.Record(ctx, AuditRecord{
		RunID: ev.RunID, Seq: ev.Seq, Type: ev.Type, CreatedAt: ev.CreatedAt,
		PayloadBytes: len(ev.Payload), PayloadSHA256: hex.EncodeToString(sum[:]),
	})
}

// SlogAuditSink emits only stable metadata. The payload itself never crosses
// this boundary.
type SlogAuditSink struct {
	Logger *slog.Logger
}

func (s SlogAuditSink) Record(_ context.Context, record AuditRecord) {
	logger := s.Logger
	if logger == nil {
		logger = slog.Default()
	}
	logger.Debug("vivy audit event",
		"run", string(record.RunID), "seq", record.Seq, "type", record.Type,
		"payload_bytes", record.PayloadBytes, "payload_sha256", record.PayloadSHA256,
	)
}

package sqlite

import (
	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/conformance"
	"context"
	"errors"
	"fmt"
	"testing"
)

func TestIssue58SealedSourceTamperRejectedOnRetry(t *testing.T) {
	b := openBackend(t)
	ctx := context.Background()
	conformance.Issue58Identity(t, b)
	s := conformance.Issue58Source("tamper-source", false)
	if err := b.CreateSession(ctx, domain.Session{ID: s.SessionID}); err != nil {
		t.Fatal(err)
	}
	in := conformance.Issue58Completed(t, b, s, "tamper-run")
	sealed, err := b.SealSubjectInteraction(ctx, s.SubjectHistoryBinding, in)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.db.ExecContext(ctx, `UPDATE messages SET content='corrupted' WHERE id='user_tamper-run'`); err != nil {
		t.Fatal(err)
	}
	if _, err := b.SealSubjectInteraction(ctx, s.SubjectHistoryBinding, sealed); !errors.Is(err, storage.ErrSubjectEvidenceIncomplete) {
		t.Fatalf("idempotent retry accepted changed original: %v", err)
	}
	if rows, err := b.ReadSubjectInteraction(ctx, s.SubjectHistoryBinding, sealed.ID); !errors.Is(err, storage.ErrSubjectEvidenceIncomplete) || len(rows) != 0 {
		t.Fatalf("tampered original read: %v %v", rows, err)
	}
}
func TestIssue58ExactEvidenceRequiredBeforeSeal(t *testing.T) {
	for _, test := range []string{"missing-projection", "truncated-tool", "unfinished-deltas", "excluded", "attachment-change"} {
		t.Run(test, func(t *testing.T) {
			b := openBackend(t)
			ctx := context.Background()
			conformance.Issue58Identity(t, b)
			s := conformance.Issue58Source("evidence-source", false)
			if err := b.CreateSession(ctx, domain.Session{ID: s.SessionID}); err != nil {
				t.Fatal(err)
			}
			a := conformance.Issue58Admission(s, "evidence-run")
			if test == "attachment-change" {
				a.Message.Attachments = []domain.Attachment{{Name: "image", MimeType: "image/png", Data: []byte("exact durable bytes")}}
			}
			if _, err := b.CommitRunAdmission(ctx, a); err != nil {
				t.Fatal(err)
			}
			ev := domain.RunEvent{RunID: a.Run.ID, PayloadVersion: 1, Type: domain.EventModelDelta, Payload: []byte(`{"delta":"partial"}`)}
			switch test {
			case "missing-projection":
				complete := domain.RunEvent{RunID: a.Run.ID, PayloadVersion: 2, Type: domain.EventModelCompleted, Payload: []byte(`{"byte_len":7,"content_sha256":"9834a14ab9bcaa0f6a8da71073617eac8f004e596a3fa11d807b84631b825d9d"}`)}
				if _, err := b.Append(ctx, storage.Commit{RunID: a.Run.ID, Events: []domain.RunEvent{ev, complete}}); err != nil {
					t.Fatal(err)
				}
			case "truncated-tool":
				ev.Type = domain.EventToolFinished
				ev.Payload = []byte(`{"result":"shortened"}`)
				fallthrough
			case "unfinished-deltas":
				if _, err := b.Append(ctx, storage.Commit{RunID: a.Run.ID, Events: []domain.RunEvent{ev}}); err != nil {
					t.Fatal(err)
				}
			case "excluded":
				if _, err := b.db.ExecContext(ctx, `UPDATE messages SET exclude_automatic_ingest=1 WHERE id=?`, a.Message.ID); err != nil {
					t.Fatal(err)
				}
			}
			terminal := domain.RunEvent{RunID: a.Run.ID, Type: domain.EventRunFailed, PayloadVersion: 1, Payload: []byte(`{"outcome":"failed"}`)}
			seq, err := b.Append(ctx, storage.Commit{RunID: a.Run.ID, Events: []domain.RunEvent{terminal}})
			if err != nil {
				t.Fatal(err)
			}
			if err := b.SetRunStatus(ctx, a.Run.ID, domain.RunFailed); err != nil {
				t.Fatal(err)
			}
			digest, err := storage.SubjectEvidenceDigest([]domain.Message{a.Message})
			if err != nil {
				t.Fatal(err)
			}
			if test == "attachment-change" {
				if _, err := b.db.ExecContext(ctx, `UPDATE message_attachments SET data=? WHERE message_id=?`, []byte("changed"), a.Message.ID); err != nil {
					t.Fatal(err)
				}
			}
			in := storage.SubjectInteraction{ID: fmt.Sprintf("%s:%d", a.Run.ID, seq), SessionID: s.SessionID, RunID: a.Run.ID, TerminalSeq: seq, SourceDigest: digest}
			if _, err := b.SealSubjectInteraction(ctx, s.SubjectHistoryBinding, in); !errors.Is(err, storage.ErrSubjectEvidenceIncomplete) {
				t.Fatalf("incomplete evidence sealed: %v", err)
			}
			if high, err := b.CaptureSubjectHistory(ctx, s.SubjectHistoryBinding); err != nil || high != 0 {
				t.Fatalf("failed seal consumed sequence: %d %v", high, err)
			}
		})
	}
}

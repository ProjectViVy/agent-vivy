// Package subjectsql owns metadata-only subject transactions shared by the two
// core SQL backends. Dialect-specific schema remains in storage/migrations.
package subjectsql

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"math"
	"strconv"
	"strings"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

type Store struct {
	DB       *sql.DB
	Postgres bool
}

func (s Store) query(q string) string {
	if !s.Postgres {
		return q
	}
	var out strings.Builder
	n := 0
	for _, r := range q {
		if r == '?' {
			n++
			out.WriteString("$" + strconv.Itoa(n))
		} else {
			out.WriteRune(r)
		}
	}
	return out.String()
}
func (s Store) one(ctx context.Context, tx *sql.Tx, q string, args []any, out ...any) error {
	return tx.QueryRowContext(ctx, s.query(q), args...).Scan(out...)
}
func (s Store) exec(ctx context.Context, tx *sql.Tx, q string, args ...any) error {
	_, err := tx.ExecContext(ctx, s.query(q), args...)
	return err
}
func (s Store) begin(ctx context.Context) (*sql.Tx, error) { return s.DB.BeginTx(ctx, nil) }

func (s Store) identity(ctx context.Context, tx *sql.Tx, b storage.SubjectHistoryBinding) error {
	if b.SubjectID == "" || b.DomainID == "" {
		return storage.ErrSubjectBinding
	}
	var raw []byte
	if err := s.one(ctx, tx, `SELECT value FROM snapshots WHERE key=?`, []any{domain.SubjectIdentityKey}, &raw); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return storage.ErrSubjectBinding
		}
		return err
	}
	var id domain.SubjectIdentity
	if json.Unmarshal(raw, &id) != nil || !id.Valid() || id.SubjectID != b.SubjectID || id.DomainID != b.DomainID {
		return storage.ErrSubjectBinding
	}
	return nil
}

// RegisterAdmissionTx runs under the existing session lock, in the existing
// admission transaction. No standalone registration can outlive a failed run.
// PinRunTx is called after the run row is inserted in the same transaction.
func (s Store) RegisterAdmissionTx(ctx context.Context, tx *sql.Tx, run domain.Run, source *storage.SubjectSourceBinding) error {
	existing, err := s.source(ctx, tx, run.SessionID)
	if err != nil && !errors.Is(err, storage.ErrNotFound) {
		return err
	}
	if source == nil {
		if err == nil {
			return storage.ErrSubjectBinding
		}
		return nil
	}
	if source.Validate() != nil || source.SessionID != run.SessionID || (run.Kind != "" && run.Kind != domain.RunKindPrimary) || run.ParentID != "" || run.Depth != 0 || run.Purpose != "" {
		return storage.ErrSubjectBinding
	}
	if err := s.identity(ctx, tx, source.SubjectHistoryBinding); err != nil {
		return err
	}
	var purpose string
	var children int
	if err := s.one(ctx, tx, `SELECT purpose,(SELECT COUNT(*) FROM child_sessions WHERE child_session_id=?) FROM sessions WHERE id=?`, []any{run.SessionID, run.SessionID}, &purpose, &children); err != nil {
		return err
	}
	if purpose != "" || children != 0 {
		return storage.ErrSubjectBinding
	}
	if err == nil {
		if !storage.SameSubjectSourceBinding(existing, *source) {
			return storage.ErrSubjectBinding
		}
		return nil
	}
	raw, err := json.Marshal(source)
	if err != nil {
		return err
	}
	if source.ConversationID != "" {
		// A trusted main binding cannot convert an existing ordinary conversation.
		var count int
		if err := s.one(ctx, tx, `SELECT COUNT(*) FROM runs WHERE session_id=?`, []any{run.SessionID}, &count); err != nil {
			return err
		}
		if count != 0 {
			return storage.ErrSubjectBinding
		}
		policy := *source
		policy.SessionID = ""
		policyRaw, err := json.Marshal(policy)
		if err != nil {
			return err
		}
		if err := s.exec(ctx, tx, `INSERT INTO subject_main_conversations(subject_id,domain_id,conversation_id,policy_binding_json) VALUES(?,?,?,?) ON CONFLICT(subject_id) DO NOTHING`, source.SubjectID, source.DomainID, source.ConversationID, policyRaw); err != nil {
			return err
		}
		var stored []byte
		if err := s.one(ctx, tx, `SELECT policy_binding_json FROM subject_main_conversations WHERE subject_id=?`, []any{source.SubjectID}, &stored); err != nil {
			return err
		}
		if string(stored) != string(policyRaw) {
			return storage.ErrSubjectBinding
		}
	}
	return s.exec(ctx, tx, `INSERT INTO subject_sources(session_id,subject_id,domain_id,conversation_id,binding_json) VALUES(?,?,?,?,?)`, run.SessionID, source.SubjectID, source.DomainID, source.ConversationID, raw)
}

func (s Store) PinRunTx(ctx context.Context, tx *sql.Tx, run domain.Run, source *storage.SubjectSourceBinding, messageID string) error {
	if source == nil {
		return nil
	}
	raw, err := json.Marshal(source)
	if err != nil {
		return err
	}
	if err := s.exec(ctx, tx, `INSERT INTO run_subject_sources(run_id,session_id,binding_json,admitted_message_id) VALUES(?,?,?,?) ON CONFLICT(run_id) DO NOTHING`, run.ID, run.SessionID, raw, messageID); err != nil {
		return err
	}
	var stored []byte
	if err := s.one(ctx, tx, `SELECT binding_json FROM run_subject_sources WHERE run_id=?`, []any{run.ID}, &stored); err != nil {
		return err
	}
	if string(stored) != string(raw) {
		return storage.ErrSubjectBinding
	}
	return nil
}
func (s Store) source(ctx context.Context, tx *sql.Tx, sid domain.SessionID) (storage.SubjectSourceBinding, error) {
	var raw []byte
	err := s.one(ctx, tx, `SELECT binding_json FROM subject_sources WHERE session_id=?`, []any{sid}, &raw)
	if errors.Is(err, sql.ErrNoRows) {
		return storage.SubjectSourceBinding{}, storage.ErrNotFound
	}
	if err != nil {
		return storage.SubjectSourceBinding{}, err
	}
	var source storage.SubjectSourceBinding
	if json.Unmarshal(raw, &source) != nil || source.Validate() != nil || source.SessionID != sid {
		return storage.SubjectSourceBinding{}, storage.ErrSubjectBinding
	}
	return source, nil
}
func (s Store) GetSubjectSource(ctx context.Context, sid domain.SessionID) (storage.SubjectSourceBinding, error) {
	tx, err := s.begin(ctx)
	if err != nil {
		return storage.SubjectSourceBinding{}, err
	}
	defer tx.Rollback()
	return s.source(ctx, tx, sid)
}
func (s Store) GetRunSubjectSource(ctx context.Context, rid domain.RunID) (storage.SubjectSourceBinding, error) {
	tx, err := s.begin(ctx)
	if err != nil {
		return storage.SubjectSourceBinding{}, err
	}
	defer tx.Rollback()
	return s.runSource(ctx, tx, rid)
}
func (s Store) runSource(ctx context.Context, tx *sql.Tx, rid domain.RunID) (storage.SubjectSourceBinding, error) {
	var raw []byte
	err := s.one(ctx, tx, `SELECT binding_json FROM run_subject_sources WHERE run_id=?`, []any{rid}, &raw)
	if errors.Is(err, sql.ErrNoRows) {
		return storage.SubjectSourceBinding{}, storage.ErrNotFound
	}
	if err != nil {
		return storage.SubjectSourceBinding{}, err
	}
	var source storage.SubjectSourceBinding
	if json.Unmarshal(raw, &source) != nil || source.Validate() != nil {
		return storage.SubjectSourceBinding{}, storage.ErrSubjectBinding
	}
	current, err := s.source(ctx, tx, source.SessionID)
	if err != nil {
		return storage.SubjectSourceBinding{}, err
	}
	if !storage.SameSubjectSourceBinding(source, current) {
		return storage.SubjectSourceBinding{}, storage.ErrSubjectBinding
	}
	return source, nil
}
func (s Store) lockHead(ctx context.Context, tx *sql.Tx, b storage.SubjectHistoryBinding) error {
	if err := s.identity(ctx, tx, b); err != nil {
		return err
	}
	if err := s.exec(ctx, tx, `INSERT INTO subject_history_heads(subject_id,domain_id,next_sequence) VALUES(?,?,1) ON CONFLICT(subject_id,domain_id) DO NOTHING`, b.SubjectID, b.DomainID); err != nil {
		return err
	}
	return s.exec(ctx, tx, `UPDATE subject_history_heads SET next_sequence=next_sequence WHERE subject_id=? AND domain_id=?`, b.SubjectID, b.DomainID)
}
func (s Store) SealSubjectInteraction(ctx context.Context, b storage.SubjectHistoryBinding, in storage.SubjectInteraction) (storage.SubjectInteraction, error) {
	tx, err := s.begin(ctx)
	if err != nil {
		return storage.SubjectInteraction{}, err
	}
	defer tx.Rollback()
	if err := s.lockHead(ctx, tx, b); err != nil {
		return storage.SubjectInteraction{}, err
	}
	var existing storage.SubjectInteraction
	err = s.one(ctx, tx, `SELECT sequence,interaction_id,session_id,run_id,terminal_seq,source_digest FROM subject_interactions WHERE subject_id=? AND domain_id=? AND interaction_id=?`, []any{b.SubjectID, b.DomainID, in.ID}, &existing.Sequence, &existing.ID, &existing.SessionID, &existing.RunID, &existing.TerminalSeq, &existing.SourceDigest)
	if err == nil {
		want := in
		want.Sequence = existing.Sequence
		if want != existing {
			return storage.SubjectInteraction{}, storage.ErrSubjectInteractionConflict
		}
		messages, _, err := s.evidence(ctx, tx, b, existing)
		if err != nil {
			return storage.SubjectInteraction{}, err
		}
		digest, err := storage.SubjectEvidenceDigest(messages)
		if err != nil {
			return storage.SubjectInteraction{}, err
		}
		if digest != existing.SourceDigest {
			return storage.SubjectInteraction{}, storage.ErrSubjectEvidenceIncomplete
		}
		return existing, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return storage.SubjectInteraction{}, err
	}
	messages, outcome, err := s.evidence(ctx, tx, b, in)
	if err != nil {
		return storage.SubjectInteraction{}, err
	}
	digest, err := storage.SubjectEvidenceDigest(messages)
	if err != nil {
		return storage.SubjectInteraction{}, err
	}
	if digest != in.SourceDigest {
		return storage.SubjectInteraction{}, storage.ErrSubjectEvidenceIncomplete
	}
	var next int64
	if err := s.one(ctx, tx, `SELECT next_sequence FROM subject_history_heads WHERE subject_id=? AND domain_id=?`, []any{b.SubjectID, b.DomainID}, &next); err != nil {
		return storage.SubjectInteraction{}, err
	}
	if next <= 0 || next == math.MaxInt64 || (in.Sequence != 0 && in.Sequence != uint64(next)) {
		return storage.SubjectInteraction{}, storage.ErrSubjectInteractionConflict
	}
	in.Sequence = uint64(next)
	if err := s.exec(ctx, tx, `INSERT INTO subject_interactions(subject_id,domain_id,sequence,interaction_id,session_id,run_id,terminal_seq,source_digest,terminal_type) VALUES(?,?,?,?,?,?,?,?,?)`, b.SubjectID, b.DomainID, next, in.ID, in.SessionID, in.RunID, in.TerminalSeq, in.SourceDigest, outcome); err != nil {
		return storage.SubjectInteraction{}, err
	}
	if err := s.exec(ctx, tx, `UPDATE subject_history_heads SET next_sequence=? WHERE subject_id=? AND domain_id=?`, next+1, b.SubjectID, b.DomainID); err != nil {
		return storage.SubjectInteraction{}, err
	}
	if err := tx.Commit(); err != nil {
		return storage.SubjectInteraction{}, err
	}
	return in, nil
}
func (s Store) CaptureSubjectHistory(ctx context.Context, b storage.SubjectHistoryBinding) (uint64, error) {
	tx, err := s.begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	if err := s.identity(ctx, tx, b); err != nil {
		return 0, err
	}
	var next uint64
	err = s.one(ctx, tx, `SELECT next_sequence FROM subject_history_heads WHERE subject_id=? AND domain_id=?`, []any{b.SubjectID, b.DomainID}, &next)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return next - 1, nil
}
func (s Store) ListSubjectInteractions(ctx context.Context, b storage.SubjectHistoryBinding, after, through uint64, limit int) ([]storage.SubjectInteraction, error) {
	if limit < 1 || limit > 1000 || after > math.MaxInt64 || through > math.MaxInt64 {
		return nil, storage.ErrSubjectBinding
	}
	tx, err := s.begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err := s.identity(ctx, tx, b); err != nil {
		return nil, err
	}
	// Zero is an empty captured prefix. CaptureSubjectHistory is the explicit
	// first-page cut, so empty captures never acquire later interactions.
	if after > through {
		return nil, storage.ErrSubjectBinding
	}
	rows, err := tx.QueryContext(ctx, s.query(`SELECT sequence,interaction_id,session_id,run_id,terminal_seq,source_digest FROM subject_interactions WHERE subject_id=? AND domain_id=? AND sequence>? AND sequence<=? ORDER BY sequence LIMIT ?`), b.SubjectID, b.DomainID, int64(after), int64(through), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []storage.SubjectInteraction{}
	for rows.Next() {
		var row storage.SubjectInteraction
		if err := rows.Scan(&row.Sequence, &row.ID, &row.SessionID, &row.RunID, &row.TerminalSeq, &row.SourceDigest); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}
func (s Store) ReadSubjectInteraction(ctx context.Context, b storage.SubjectHistoryBinding, id string) ([]domain.Message, error) {
	tx, err := s.begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err := s.identity(ctx, tx, b); err != nil {
		return nil, err
	}
	var in storage.SubjectInteraction
	err = s.one(ctx, tx, `SELECT sequence,interaction_id,session_id,run_id,terminal_seq,source_digest FROM subject_interactions WHERE subject_id=? AND domain_id=? AND interaction_id=?`, []any{b.SubjectID, b.DomainID, id}, &in.Sequence, &in.ID, &in.SessionID, &in.RunID, &in.TerminalSeq, &in.SourceDigest)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, storage.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	messages, _, err := s.evidence(ctx, tx, b, in)
	if err != nil {
		return nil, err
	}
	digest, err := storage.SubjectEvidenceDigest(messages)
	if err != nil {
		return nil, err
	}
	if digest != in.SourceDigest {
		return nil, storage.ErrSubjectEvidenceIncomplete
	}
	return messages, nil
}
func (s Store) ArchiveRetainedSession(ctx context.Context, sid domain.SessionID) error {
	tx, err := s.begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	source, err := s.source(ctx, tx, sid)
	if err != nil {
		return err
	}
	if err := s.identity(ctx, tx, source.SubjectHistoryBinding); err != nil {
		return err
	}
	if err := s.exec(ctx, tx, `UPDATE subject_sources SET archived=1 WHERE session_id=?`, sid); err != nil {
		return err
	}
	return tx.Commit()
}

// ProtectDeleteTx is checked for every member of a session subtree before any
// hard-delete statement. Protection starts at admission, before terminal seal.
func (s Store) ProtectDeleteTx(ctx context.Context, tx *sql.Tx, sid domain.SessionID) error {
	var n int
	if err := s.one(ctx, tx, `SELECT COUNT(*) FROM subject_sources WHERE session_id=?`, []any{sid}, &n); err != nil {
		return err
	}
	if n > 0 {
		return storage.ErrRetainedSession
	}
	return nil
}

var _ storage.SubjectSourceStore = Store{}

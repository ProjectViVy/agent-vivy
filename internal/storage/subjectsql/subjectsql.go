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
			allowed, err := s.onlyExcludedShellRuns(ctx, tx, run.SessionID)
			if err != nil {
				return err
			}
			if !allowed {
				return storage.ErrSubjectBinding
			}
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
	if err := s.exec(ctx, tx, `INSERT INTO run_subject_sources(run_id,session_id,binding_json,admitted_message_id) VALUES(?,?,?,?) ON CONFLICT(run_id) DO NOTHING`, run.ID, run.SessionID, raw, nullableMessageID(messageID)); err != nil {
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
	err = s.one(ctx, tx, `SELECT sequence,interaction_id,session_id,run_id,terminal_seq,source_digest,disposition FROM subject_interactions WHERE subject_id=? AND domain_id=? AND interaction_id=?`, []any{b.SubjectID, b.DomainID, in.ID}, &existing.Sequence, &existing.ID, &existing.SessionID, &existing.RunID, &existing.TerminalSeq, &existing.SourceDigest, &existing.Disposition)
	if err == nil {
		existing.Disposition = storedDisposition(existing.Disposition)
		want := in
		want.Sequence = existing.Sequence
		if want != existing {
			return storage.SubjectInteraction{}, storage.ErrSubjectInteractionConflict
		}
		if existing.Disposition == storage.SubjectInteractionExcluded {
			return existing, nil
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
	if err := s.completeTerminalEvidence(ctx, tx, b, in.RunID); err != nil {
		return storage.SubjectInteraction{}, err
	}
	return s.insertInteraction(ctx, tx, b, in, outcome)
}

// insertInteraction allocates the next sequence under the head lock, stores the
// interaction and commits. in.Disposition selects included or excluded.
func (s Store) insertInteraction(ctx context.Context, tx *sql.Tx, b storage.SubjectHistoryBinding, in storage.SubjectInteraction, outcome string) (storage.SubjectInteraction, error) {
	var next int64
	if err := s.one(ctx, tx, `SELECT next_sequence FROM subject_history_heads WHERE subject_id=? AND domain_id=?`, []any{b.SubjectID, b.DomainID}, &next); err != nil {
		return storage.SubjectInteraction{}, err
	}
	if next <= 0 || next == math.MaxInt64 || (in.Sequence != 0 && in.Sequence != uint64(next)) {
		return storage.SubjectInteraction{}, storage.ErrSubjectInteractionConflict
	}
	in.Sequence = uint64(next)
	if err := s.exec(ctx, tx, `INSERT INTO subject_interactions(subject_id,domain_id,sequence,interaction_id,session_id,run_id,terminal_seq,source_digest,terminal_type,disposition) VALUES(?,?,?,?,?,?,?,?,?,?)`, b.SubjectID, b.DomainID, next, in.ID, in.SessionID, in.RunID, in.TerminalSeq, in.SourceDigest, outcome, dispositionOrIncluded(in.Disposition)); err != nil {
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

// storedDisposition maps the column back to the in-memory form where an
// included interaction has an empty Disposition.
func storedDisposition(d string) string {
	if d == "included" {
		return ""
	}
	return d
}

func dispositionOrIncluded(d string) string {
	if d == "" {
		return "included"
	}
	return d
}

// SealSubjectExclusion seals a terminal run whose evidence can never resolve.
// Storage re-derives the condition inside the head transaction, so a healthy or
// merely not-yet-projected run can never be excluded by a caller's say-so.
func (s Store) SealSubjectExclusion(ctx context.Context, b storage.SubjectHistoryBinding, in storage.SubjectInteraction) (storage.SubjectInteraction, error) {
	tx, err := s.begin(ctx)
	if err != nil {
		return storage.SubjectInteraction{}, err
	}
	defer tx.Rollback()
	if err := s.lockHead(ctx, tx, b); err != nil {
		return storage.SubjectInteraction{}, err
	}
	digest, err := storage.SubjectEvidenceDigest([]domain.Message{})
	if err != nil {
		return storage.SubjectInteraction{}, err
	}
	in.SourceDigest, in.Disposition = digest, storage.SubjectInteractionExcluded
	var existing storage.SubjectInteraction
	err = s.one(ctx, tx, `SELECT sequence,interaction_id,session_id,run_id,terminal_seq,source_digest,disposition FROM subject_interactions WHERE subject_id=? AND domain_id=? AND interaction_id=?`, []any{b.SubjectID, b.DomainID, in.ID}, &existing.Sequence, &existing.ID, &existing.SessionID, &existing.RunID, &existing.TerminalSeq, &existing.SourceDigest, &existing.Disposition)
	if err == nil {
		existing.Disposition = storedDisposition(existing.Disposition)
		want := in
		want.Sequence = existing.Sequence
		if want != existing {
			return storage.SubjectInteraction{}, storage.ErrSubjectInteractionConflict
		}
		return existing, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return storage.SubjectInteraction{}, err
	}
	if _, _, err := s.evidence(ctx, tx, b, in); !errors.Is(err, storage.ErrSubjectEvidenceUnresolvable) {
		if err == nil {
			err = storage.ErrSubjectEvidenceIncomplete
		}
		return storage.SubjectInteraction{}, err
	}
	var outcome string
	if err := s.one(ctx, tx, `SELECT type FROM run_events WHERE run_id=? AND seq=?`, []any{in.RunID, in.TerminalSeq}, &outcome); err != nil {
		return storage.SubjectInteraction{}, err
	}
	return s.insertInteraction(ctx, tx, b, in, outcome)
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
	rows, err := tx.QueryContext(ctx, s.query(`SELECT sequence,interaction_id,session_id,run_id,terminal_seq,source_digest,disposition FROM subject_interactions WHERE subject_id=? AND domain_id=? AND sequence>? AND sequence<=? ORDER BY sequence LIMIT ?`), b.SubjectID, b.DomainID, int64(after), int64(through), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []storage.SubjectInteraction{}
	for rows.Next() {
		var row storage.SubjectInteraction
		if err := rows.Scan(&row.Sequence, &row.ID, &row.SessionID, &row.RunID, &row.TerminalSeq, &row.SourceDigest, &row.Disposition); err != nil {
			return nil, err
		}
		row.Disposition = storedDisposition(row.Disposition)
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
	err = s.one(ctx, tx, `SELECT sequence,interaction_id,session_id,run_id,terminal_seq,source_digest,disposition FROM subject_interactions WHERE subject_id=? AND domain_id=? AND interaction_id=?`, []any{b.SubjectID, b.DomainID, id}, &in.Sequence, &in.ID, &in.SessionID, &in.RunID, &in.TerminalSeq, &in.SourceDigest, &in.Disposition)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, storage.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	in.Disposition = storedDisposition(in.Disposition)
	if in.Disposition == storage.SubjectInteractionExcluded {
		// No original ever became canonical evidence for this run.
		return []domain.Message{}, nil
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

func nullableMessageID(id string) any {
	if id == "" {
		return nil
	}
	return id
}

func (s Store) ReadSubjectRunEvidence(ctx context.Context, b storage.SubjectHistoryBinding, rid domain.RunID) ([]domain.Message, storage.SubjectInteraction, error) {
	tx, err := s.begin(ctx)
	if err != nil {
		return nil, storage.SubjectInteraction{}, err
	}
	defer tx.Rollback()
	if err := s.identity(ctx, tx, b); err != nil {
		return nil, storage.SubjectInteraction{}, err
	}
	in := storage.SubjectInteraction{RunID: rid}
	if err := s.one(ctx, tx, `SELECT session_id FROM runs WHERE id=?`, []any{rid}, &in.SessionID); err != nil {
		return nil, in, err
	}
	if err := s.one(ctx, tx, `SELECT seq FROM run_events WHERE run_id=? AND type IN ('run.completed','run.failed','run.cancelled') ORDER BY seq DESC LIMIT 1`, []any{rid}, &in.TerminalSeq); err != nil {
		if err == sql.ErrNoRows {
			err = storage.ErrSubjectEvidenceIncomplete
		}
		return nil, in, err
	}
	in.ID = storage.SubjectInteractionID(rid, in.TerminalSeq)
	messages, _, err := s.evidence(ctx, tx, b, in)
	if err != nil {
		return nil, in, err
	}
	in.SourceDigest, err = storage.SubjectEvidenceDigest(messages)
	return messages, in, err
}

// onlyExcludedShellRuns never converts an existing conversation. An already
// trusted singleton may first be used through !! before its first shared turn.
func (s Store) onlyExcludedShellRuns(ctx context.Context, tx *sql.Tx, sid domain.SessionID) (bool, error) {
	rows, err := tx.QueryContext(ctx, s.query(`SELECT r.id,r.kind,r.status,r.purpose,r.depth,r.parent_run_id,e.type,e.payload,
 (SELECT COUNT(*) FROM messages m WHERE m.session_id=r.session_id),
 (SELECT COUNT(*) FROM run_events x WHERE x.run_id=r.id AND x.type IN ('model.request','model.delta','model.completed'))
 FROM runs r LEFT JOIN run_events e ON e.run_id=r.id AND e.seq=1 WHERE r.session_id=?`), sid)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	found := false
	for rows.Next() {
		var rid, kind, status, purpose, parent string
		var depth, messages, models int
		var typ sql.NullString
		var raw []byte
		if err := rows.Scan(&rid, &kind, &status, &purpose, &depth, &parent, &typ, &raw, &messages, &models); err != nil {
			return false, err
		}
		var p struct {
			NoContext bool `json:"no_context"`
		}
		if (kind != "" && kind != string(domain.RunKindPrimary)) || !domain.RunStatus(status).Terminal() || purpose != "" || depth != 0 || parent != "" || typ.String != string(domain.EventRunStarted) || messages != 0 || models != 0 || json.Unmarshal(raw, &p) != nil || !p.NoContext || !storage.IsGovernedShellStart(raw) {
			return false, nil
		}
		found = true
	}
	return found, rows.Err()
}

// completeTerminalEvidence uses the same head transaction as allocation. A
// known terminal gap cannot be skipped; active and unpinned runs do not enter
// this cut. Complete sources keep durable seal order, independent of clocks.
func (s Store) completeTerminalEvidence(ctx context.Context, tx *sql.Tx, b storage.SubjectHistoryBinding, current domain.RunID) error {
	rows, err := tx.QueryContext(ctx, s.query(`SELECT p.run_id,p.session_id,e.seq
 FROM run_subject_sources p JOIN subject_sources h ON h.session_id=p.session_id
 JOIN run_events e ON e.run_id=p.run_id AND e.type IN ('run.completed','run.failed','run.cancelled')
 LEFT JOIN subject_interactions i ON i.run_id=p.run_id
 WHERE h.subject_id=? AND h.domain_id=? AND i.run_id IS NULL AND p.run_id<>? ORDER BY p.run_id`), b.SubjectID, b.DomainID, current)
	if err != nil {
		return err
	}
	var pending []storage.SubjectInteraction
	for rows.Next() {
		var in storage.SubjectInteraction
		if err := rows.Scan(&in.RunID, &in.SessionID, &in.TerminalSeq); err != nil {
			rows.Close()
			return err
		}
		in.ID = storage.SubjectInteractionID(in.RunID, in.TerminalSeq)
		pending = append(pending, in)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, in := range pending {
		if _, _, err := s.evidence(ctx, tx, b, in); err != nil && !errors.Is(err, storage.ErrSubjectEvidenceUnresolvable) {
			return err
		}
	}
	return nil
}

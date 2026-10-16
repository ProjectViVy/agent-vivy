package subjectsql

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

// SQLite pins the database snapshot at the first identity SELECT and retains
// it for this read transaction (including WAL mode). PostgreSQL needs explicit
// REPEATABLE READ: independent READ COMMITTED statements could mix two cuts.
func (s Store) contextTx(ctx context.Context) (*sql.Tx, error) {
	opts := &sql.TxOptions{ReadOnly: true}
	if s.Postgres {
		opts.Isolation = sql.LevelRepeatableRead
	}
	return s.DB.BeginTx(ctx, opts)
}
func (s Store) CaptureSubjectContext(ctx context.Context, b storage.SubjectHistoryBinding) (storage.SubjectContextSnapshot, error) {
	out, err := s.captureSubjectContext(ctx, b)
	if err != nil {
		return storage.SubjectContextSnapshot{}, err
	}
	return out, nil
}
func (s Store) captureSubjectContext(ctx context.Context, b storage.SubjectHistoryBinding) (storage.SubjectContextSnapshot, error) {
	var out storage.SubjectContextSnapshot
	tx, err := s.contextTx(ctx)
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	if err = s.identity(ctx, tx, b); err != nil {
		return out, err
	}

	// Repair and capture are separate phases. A terminal which commits after
	// the runtime's pending cut must not disappear behind an older ceiling.
	pending, err := s.pendingRunsTx(ctx, tx, b)
	if err != nil {
		return out, err
	}
	if len(pending) > 0 {
		return out, storage.ErrSubjectEvidenceIncomplete
	}
	var next uint64
	err = s.one(ctx, tx, `SELECT next_sequence FROM subject_history_heads WHERE subject_id=? AND domain_id=?`, []any{b.SubjectID, b.DomainID}, &next)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return out, err
	}
	if err == nil {
		if next == 0 {
			return out, storage.ErrSubjectEvidenceIncomplete
		}
		out.Through = next - 1
	}

	out.Sources = []storage.SubjectSourceBinding{}
	members, err := tx.QueryContext(ctx, s.query(`SELECT session_id,binding_json FROM subject_sources WHERE subject_id=? AND domain_id=? ORDER BY session_id`), b.SubjectID, b.DomainID)
	if err != nil {
		return out, err
	}
	for members.Next() {
		var sid domain.SessionID
		var raw []byte
		if err = members.Scan(&sid, &raw); err != nil {
			members.Close()
			return out, err
		}
		var source storage.SubjectSourceBinding
		if json.Unmarshal(raw, &source) != nil || source.Validate() != nil || source.SessionID != sid || source.SubjectHistoryBinding != b {
			members.Close()
			return out, storage.ErrSubjectBinding
		}
		out.Sources = append(out.Sources, source)
	}
	err = members.Err()
	members.Close()
	if err != nil {
		return out, err
	}
	out.Dispositions = []storage.SubjectDisposition{}
	rows, err := tx.QueryContext(ctx, s.query(`SELECT t.id,t.session_id,t.run_id,t.cutoff_message_id,t.tail_message_id,t.work_seq,t.reason,t.fork_session_id,t.created_at FROM session_truncations t JOIN subject_sources h ON h.session_id=t.session_id WHERE h.subject_id=? AND h.domain_id=? AND t.reason IN ('rewind','edit') ORDER BY t.session_id,t.id`), b.SubjectID, b.DomainID)
	if err != nil {
		return out, err
	}
	var markers []storage.SubjectDisposition
	for rows.Next() {
		var d storage.SubjectDisposition
		err = rows.Scan(&d.MarkerID, &d.SessionID, &d.Marker.RunID, &d.Marker.CutoffMessageID, &d.Marker.TailMessageID, &d.Marker.WorkSeq, &d.Marker.Reason, &d.Marker.ForkSessionID, &d.Marker.CreatedAt)
		if err != nil {
			rows.Close()
			return out, err
		}
		d.Marker.SessionID = d.SessionID
		markers = append(markers, d)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	for _, d := range markers {
		pin, err := s.source(ctx, tx, d.SessionID)
		if err != nil {
			return out, err
		}
		if pin.SubjectHistoryBinding != b {
			return out, storage.ErrSubjectBinding
		}
		d.Targets, err = s.dispositionTargets(ctx, tx, b, out.Through, d.Marker)
		if err != nil {
			return out, err
		}
		if len(d.Targets) > 0 {
			out.Dispositions = append(out.Dispositions, d)
		}
	}
	// The moving ceiling is deliberately absent: unrelated new interactions do
	// not alter any marker's exact affected identities.
	raw, err := json.Marshal(struct {
		Version      int
		Binding      storage.SubjectHistoryBinding
		Dispositions []storage.SubjectDisposition
	}{1, b, out.Dispositions})
	if err != nil {
		return out, err
	}
	digest := sha256.Sum256(raw)
	out.DispositionDigest = hex.EncodeToString(digest[:])
	if err = tx.Commit(); err != nil {
		return out, err
	}
	return out, nil
}

type dispositionMessage struct {
	id  string
	sid domain.SessionID
	rid domain.RunID
}

func (s Store) dispositionTargets(ctx context.Context, tx *sql.Tx, b storage.SubjectHistoryBinding, through uint64, m storage.SessionTruncation) ([]storage.SubjectDispositionTarget, error) {
	var first, last int64
	if err := s.one(ctx, tx, `SELECT position FROM messages WHERE session_id=? AND id=?`, []any{m.SessionID, m.CutoffMessageID}, &first); err != nil {
		return nil, missingDispositionEvidence(err)
	}
	if err := s.one(ctx, tx, `SELECT position FROM messages WHERE session_id=? AND id=?`, []any{m.SessionID, m.TailMessageID}, &last); err != nil {
		return nil, missingDispositionEvidence(err)
	}
	if first <= 0 || last < first {
		return nil, storage.ErrSubjectEvidenceIncomplete
	}
	rows, err := tx.QueryContext(ctx, s.query(`SELECT id,run_id FROM messages WHERE session_id=? AND position>=? AND position<=? ORDER BY position`), m.SessionID, first, last)
	if err != nil {
		return nil, err
	}
	var views []dispositionMessage
	for rows.Next() {
		v := dispositionMessage{sid: m.SessionID}
		if err = rows.Scan(&v.id, &v.rid); err != nil {
			rows.Close()
			return nil, err
		}
		views = append(views, v)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	targets := map[string]*storage.SubjectDispositionTarget{}
	for _, v := range views {
		var target storage.SubjectDispositionTarget
		err = s.one(ctx, tx, `SELECT interaction_id,sequence,session_id,run_id,source_digest FROM subject_interactions WHERE subject_id=? AND domain_id=? AND run_id=? AND sequence<=?`, []any{b.SubjectID, b.DomainID, v.rid, through}, &target.InteractionID, &target.Sequence, &target.SourceSessionID, &target.RunID, &target.SourceDigest)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return nil, err
		}
		canonical, chain, err := s.canonicalDispositionMessage(ctx, tx, v)
		if err != nil {
			return nil, err
		}
		if target.SourceSessionID != canonical.sid {
			return nil, storage.ErrSubjectEvidenceIncomplete
		}
		pin, err := s.runSource(ctx, tx, target.RunID)
		if err != nil {
			return nil, err
		}
		if pin.SubjectHistoryBinding != b || pin.SessionID != canonical.sid {
			return nil, storage.ErrSubjectBinding
		}
		for _, sid := range chain {
			source, err := s.source(ctx, tx, sid)
			if err != nil {
				return nil, err
			}
			if source.SubjectHistoryBinding != b {
				return nil, storage.ErrSubjectBinding
			}
		}
		current := targets[target.InteractionID]
		if current == nil {
			current = &target
			targets[target.InteractionID] = current
		}
		current.MessageIDs = append(current.MessageIDs, canonical.id)
		current.ViewMessageIDs = append(current.ViewMessageIDs, v.id)
	}
	out := make([]storage.SubjectDispositionTarget, 0, len(targets))
	for _, target := range targets {
		out = append(out, *target)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].InteractionID < out[j].InteractionID })
	return out, nil
}
func missingDispositionEvidence(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return storage.ErrSubjectEvidenceIncomplete
	}
	return err
}

// Resolve only exact persisted links. An old copy without provenance cannot be
// guessed from text, timestamps, or the inherited run ID.
func (s Store) canonicalDispositionMessage(ctx context.Context, tx *sql.Tx, v dispositionMessage) (dispositionMessage, []domain.SessionID, error) {
	seen := map[string]bool{}
	chain := []domain.SessionID{}
	for depth := 0; depth < 128; depth++ {
		if seen[v.id] {
			return v, nil, storage.ErrSubjectEvidenceIncomplete
		}
		seen[v.id] = true
		chain = append(chain, v.sid)
		var sid domain.SessionID
		var id string
		err := s.one(ctx, tx, `SELECT parent_session_id,parent_message_id FROM message_copy_sources WHERE message_id=?`, []any{v.id}, &sid, &id)
		if errors.Is(err, sql.ErrNoRows) {
			var owner domain.SessionID
			if err = s.one(ctx, tx, `SELECT session_id FROM runs WHERE id=?`, []any{v.rid}, &owner); err != nil {
				return v, nil, missingDispositionEvidence(err)
			}
			if owner != v.sid {
				return v, nil, storage.ErrSubjectEvidenceIncomplete
			}
			return v, chain, nil
		}
		if err != nil {
			return v, nil, err
		}
		var rid domain.RunID
		if err = s.one(ctx, tx, `SELECT run_id FROM messages WHERE session_id=? AND id=?`, []any{sid, id}, &rid); err != nil {
			return v, nil, missingDispositionEvidence(err)
		}
		if sid == "" || id == "" || rid != v.rid {
			return v, nil, storage.ErrSubjectEvidenceIncomplete
		}
		v = dispositionMessage{id: id, sid: sid, rid: rid}
	}
	return v, nil, storage.ErrSubjectEvidenceIncomplete
}

// This one metadata query captures a finite pending set. It never scans source
// bodies, treats active/unpinned runs as pending, or invents chronological order.
func (s Store) ListSubjectPendingRuns(ctx context.Context, b storage.SubjectHistoryBinding) ([]storage.SubjectPendingRun, error) {
	tx, err := s.contextTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err = s.identity(ctx, tx, b); err != nil {
		return nil, err
	}

	return s.pendingRunsTx(ctx, tx, b)
}
func (s Store) pendingRunsTx(ctx context.Context, tx *sql.Tx, b storage.SubjectHistoryBinding) ([]storage.SubjectPendingRun, error) {
	rows, err := tx.QueryContext(ctx, s.query(`SELECT p.session_id,p.run_id,e.seq FROM run_subject_sources p JOIN subject_sources h ON h.session_id=p.session_id JOIN runs r ON r.id=p.run_id LEFT JOIN run_events e ON e.run_id=p.run_id AND e.type IN ('run.completed','run.failed','run.cancelled') LEFT JOIN subject_interactions i ON i.run_id=p.run_id WHERE h.subject_id=? AND h.domain_id=? AND i.run_id IS NULL AND (r.status IN ('completed','failed','cancelled') OR e.seq IS NOT NULL) ORDER BY p.run_id,e.seq`), b.SubjectID, b.DomainID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []storage.SubjectPendingRun{}
	for rows.Next() {
		var p storage.SubjectPendingRun
		var terminal sql.NullInt64
		if err = rows.Scan(&p.SessionID, &p.RunID, &terminal); err != nil {
			return nil, err
		}
		if !terminal.Valid || terminal.Int64 <= 0 {
			return nil, storage.ErrSubjectEvidenceIncomplete
		}
		p.TerminalSeq = domain.EventSeq(terminal.Int64)
		if len(out) > 0 && out[len(out)-1].RunID == p.RunID {
			return nil, storage.ErrSubjectEvidenceIncomplete
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

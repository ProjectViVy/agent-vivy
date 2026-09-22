package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

var _ storage.HistoryQueryStore = (*Backend)(nil)

// CaptureHistoryCut reads message and run ceilings in one read transaction.
// Positions, not timestamps, are the snapshot boundary.
func (b *Backend) CaptureHistoryCut(ctx context.Context, ids []domain.SessionID) (storage.HistoryCut, error) {
	ids, err := storage.CanonicalHistorySessions(ids)
	if err != nil { return storage.HistoryCut{}, err }
	tx, err := b.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil { return storage.HistoryCut{}, fmt.Errorf("storage: begin history capture: %w", err) }
	defer func() { _ = tx.Rollback() }()
	cut := storage.HistoryCut{Sessions: make([]storage.HistorySessionCut, 0, len(ids))}
	for _, id := range ids {
		var next int64
		if err := tx.QueryRowContext(ctx, `SELECT next_message_position FROM sessions WHERE id = ?`, id).Scan(&next); err != nil {
			if errors.Is(err, sql.ErrNoRows) { return storage.HistoryCut{}, storage.ErrNotFound }
			return storage.HistoryCut{}, fmt.Errorf("storage: capture session history ceiling: %w", err)
		}
		cut.Sessions = append(cut.Sessions, storage.HistorySessionCut{SessionID: id, Position: next - 1})
	}
	args := sessionArgs(ids)
	rows, err := tx.QueryContext(ctx, `SELECT r.id, COALESCE(MAX(e.seq), 0)
		FROM runs r LEFT JOIN run_events e ON e.run_id = r.id
		WHERE r.session_id IN (`+strings.TrimRight(strings.Repeat("?,", len(args)), ",")+		`) GROUP BY r.id ORDER BY r.id LIMIT 257`, args...)
	if err != nil { return storage.HistoryCut{}, fmt.Errorf("storage: capture history run ceilings: %w", err) }
	defer rows.Close()
	for rows.Next() {
		var id string; var seq int64
		if err := rows.Scan(&id, &seq); err != nil { return storage.HistoryCut{}, fmt.Errorf("storage: scan history run ceiling: %w", err) }
		cut.Runs = append(cut.Runs, storage.HistoryRunCut{RunID: domain.RunID(id), Seq: domain.EventSeq(seq)})
	}
	if err := rows.Err(); err != nil { return storage.HistoryCut{}, err }
	if err := rows.Close(); err != nil { return storage.HistoryCut{}, err }
	if len(cut.Runs) > storage.HistoryCutRunMax { return storage.HistoryCut{}, storage.ErrHistoryNarrowScope }
	if err := tx.Commit(); err != nil { return storage.HistoryCut{}, fmt.Errorf("storage: commit history capture: %w", err) }
	return cut, nil
}

func (b *Backend) QueryHistoryPage(ctx context.Context, cut storage.HistoryCut, after storage.HistoryPosition, options storage.HistoryQueryOptions) (storage.HistoryCandidates, error) {
	if err := cut.Validate(); err != nil { return storage.HistoryCandidates{}, err }
	if err := after.Validate(); err != nil { return storage.HistoryCandidates{}, err }
	if !after.IsZero() && after.Stream != storage.HistoryStreamMessage { return storage.HistoryCandidates{}, fmt.Errorf("storage: message query does not accept %s cursor", after.Stream) }
	if !cut.ContainsMessagePosition(after) { return storage.HistoryCandidates{}, fmt.Errorf("storage: history cursor is outside capture cut") }
	limit, limits, err := options.Effective()
	if err != nil { return storage.HistoryCandidates{}, err }
	tx, err := b.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil { return storage.HistoryCandidates{}, fmt.Errorf("storage: begin history page: %w", err) }
	defer func() { _ = tx.Rollback() }()
	meta, err := sqliteHistoryMetadata(ctx, tx, cut, after, limits.CandidateRecords+1)
	if err != nil { return storage.HistoryCandidates{}, err }
	hidden, err := sqliteHiddenHistoryPositions(ctx, tx, cut)
	if err != nil { return storage.HistoryCandidates{}, err }
	out := storage.HistoryCandidates{}
	for i, item := range meta {
		if err := ctx.Err(); err != nil { return storage.HistoryCandidates{}, err }
		if i >= limits.CandidateRecords { out.ScanIncomplete = true; break }
		if len(out.Records) >= limit { out.HasMore = true; break }
		out.Next = storage.HistoryPosition{Stream: storage.HistoryStreamMessage, SessionID: item.sessionID, Position: item.position}
		remaining := limits.CandidateBytes - out.BytesInspected
		if item.bytes > remaining {
			if remaining > 0 { out.BytesInspected += remaining }
			if !hidden[item.sessionID][item.position] { out.Records = append(out.Records, unavailableHistoryCandidate(item)) }
			out.ScanIncomplete = i+1 < len(meta)
			break
		}
		out.BytesInspected += item.bytes
		if hidden[item.sessionID][item.position] { continue }
		if item.bytes > limits.ResultItemBytes {
			out.Records = append(out.Records, unavailableHistoryCandidate(item))
			continue
		}
		var body []byte
		if err := tx.QueryRowContext(ctx, `SELECT content FROM messages WHERE id = ? AND session_id = ? AND position = ?`, item.id, item.sessionID, item.position).Scan(&body); err != nil {
			if errors.Is(err, sql.ErrNoRows) { continue } // deleted after metadata in a non-serializable backend.
			return storage.HistoryCandidates{}, fmt.Errorf("storage: load bounded history payload: %w", err)
		}
		candidate := unavailableHistoryCandidate(item)
		candidate.Unavailable, candidate.Truncated, candidate.Text = false, false, string(body)
		out.Records = append(out.Records, candidate)
	}
	if len(meta) > limits.CandidateRecords { out.ScanIncomplete = true }
	if err := tx.Commit(); err != nil { return storage.HistoryCandidates{}, fmt.Errorf("storage: commit history page: %w", err) }
	return out, nil
}

type historyMetadata struct { id string; sessionID domain.SessionID; runID domain.RunID; role string; createdAt, position int64; bytes int }

func sqliteHistoryMetadata(ctx context.Context, tx *sql.Tx, cut storage.HistoryCut, after storage.HistoryPosition, max int) ([]historyMetadata, error) {
	clauses := make([]string, 0, len(cut.Sessions)); args := make([]any, 0, len(cut.Sessions)*2+3)
	for _, c := range cut.Sessions { clauses = append(clauses, "(session_id = ? AND position <= ?)"); args = append(args, c.SessionID, c.Position) }
	if !after.IsZero() { args = append(args, after.SessionID, after.SessionID, after.Position) }
	args = append(args, max)
	query := `SELECT id, session_id, run_id, role, created_at, position, length(content) FROM messages WHERE (`+strings.Join(clauses, " OR ")+		`)`
	if !after.IsZero() { query += ` AND (session_id > ? OR (session_id = ? AND position > ?))` }
	query += ` ORDER BY session_id, position LIMIT ?`
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil { return nil, fmt.Errorf("storage: enumerate history metadata: %w", err) }
	defer rows.Close()
	var out []historyMetadata
	for rows.Next() { var item historyMetadata; var sid, rid string; if err := rows.Scan(&item.id, &sid, &rid, &item.role, &item.createdAt, &item.position, &item.bytes); err != nil { return nil, fmt.Errorf("storage: scan history metadata: %w", err) }; item.sessionID, item.runID = domain.SessionID(sid), domain.RunID(rid); out = append(out, item) }
	return out, rows.Err()
}

func sqliteHiddenHistoryPositions(ctx context.Context, tx *sql.Tx, cut storage.HistoryCut) (map[domain.SessionID]map[int64]bool, error) {
	hidden := make(map[domain.SessionID]map[int64]bool, len(cut.Sessions))
	for _, c := range cut.Sessions {
		hidden[c.SessionID] = map[int64]bool{}
		rows, err := tx.QueryContext(ctx, `SELECT cutoff_message_id, tail_message_id FROM session_truncations WHERE session_id = ? AND (reason = ? OR reason = ?) ORDER BY id`, c.SessionID, storage.TruncationRewind, storage.TruncationEdit)
		if err != nil { return nil, fmt.Errorf("storage: list history truncations: %w", err) }
		var markers [][2]string
		for rows.Next() { var from, to string; if err := rows.Scan(&from, &to); err != nil { rows.Close(); return nil, err }; markers = append(markers, [2]string{from, to}) }
		if err := rows.Err(); err != nil { rows.Close(); return nil, err }; rows.Close()
		for _, marker := range markers { var first, last int64; a := tx.QueryRowContext(ctx, `SELECT position FROM messages WHERE id = ? AND session_id = ?`, marker[0], c.SessionID).Scan(&first); z := tx.QueryRowContext(ctx, `SELECT position FROM messages WHERE id = ? AND session_id = ?`, marker[1], c.SessionID).Scan(&last); if a == nil && z == nil && first <= last { for p := first; p <= last; p++ { hidden[c.SessionID][p] = true } } }
	}
	return hidden, nil
}

func unavailableHistoryCandidate(item historyMetadata) storage.HistoryCandidate { return storage.HistoryCandidate{Ref: domain.SourceRef{SessionID: item.sessionID, RunID: item.runID, MessageID: item.id, Kind: string(domain.SourceKindMessage), CreatedAt: item.createdAt}, Author: item.role, Unavailable: true, Truncated: true} }
func sessionArgs(ids []domain.SessionID) []any { out := make([]any, len(ids)); for i := range ids { out[i] = ids[i] }; return out }

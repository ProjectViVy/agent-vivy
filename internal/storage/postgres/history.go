package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

var _ storage.HistoryQueryStore = (*Backend)(nil)

func (b *Backend) CaptureHistoryCut(ctx context.Context, ids []domain.SessionID) (storage.HistoryCut, error) {
	ids, err := storage.CanonicalHistorySessions(ids)
	if err != nil {
		return storage.HistoryCut{}, err
	}
	tx, err := b.db.SQL.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return storage.HistoryCut{}, fmt.Errorf("storage: begin history capture: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	cut := storage.HistoryCut{Sessions: make([]storage.HistorySessionCut, 0, len(ids))}
	for _, id := range ids {
		if err := ctx.Err(); err != nil {
			return storage.HistoryCut{}, err
		}
		var next int64
		if err := tx.QueryRowContext(ctx, `SELECT next_message_position FROM sessions WHERE id = $1`, id).Scan(&next); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return storage.HistoryCut{}, storage.ErrNotFound
			}
			return storage.HistoryCut{}, fmt.Errorf("storage: capture session history ceiling: %w", err)
		}
		cut.Sessions = append(cut.Sessions, storage.HistorySessionCut{SessionID: id, Position: next - 1})
	}

	args := postgresSessionArgs(ids)
	rows, err := tx.QueryContext(ctx, `SELECT r.id, r.session_id, COALESCE(MAX(e.seq), 0)
		FROM runs r LEFT JOIN run_events e ON e.run_id = r.id
		WHERE r.session_id IN (`+postgresPlaceholders(len(args))+`)
		GROUP BY r.id, r.session_id ORDER BY r.id LIMIT 257`, args...)
	if err != nil {
		return storage.HistoryCut{}, fmt.Errorf("storage: capture history run ceilings: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		if err := ctx.Err(); err != nil {
			return storage.HistoryCut{}, err
		}
		var runID, sessionID string
		var seq int64
		if err := rows.Scan(&runID, &sessionID, &seq); err != nil {
			return storage.HistoryCut{}, fmt.Errorf("storage: scan history run ceiling: %w", err)
		}
		cut.Runs = append(cut.Runs, storage.HistoryRunCut{
			RunID:     domain.RunID(runID),
			SessionID: domain.SessionID(sessionID),
			Seq:       domain.EventSeq(seq),
		})
	}
	if err := rows.Err(); err != nil {
		return storage.HistoryCut{}, err
	}
	if err := rows.Close(); err != nil {
		return storage.HistoryCut{}, err
	}
	if len(cut.Runs) > storage.HistoryCutRunMax {
		return storage.HistoryCut{}, storage.ErrHistoryNarrowScope
	}
	if err := tx.Commit(); err != nil {
		return storage.HistoryCut{}, fmt.Errorf("storage: commit history capture: %w", err)
	}
	return cut, nil
}

func (b *Backend) QueryHistoryPage(ctx context.Context, cut storage.HistoryCut, after storage.HistoryPosition, options storage.HistoryQueryOptions) (storage.HistoryCandidates, error) {
	if err := ctx.Err(); err != nil {
		return storage.HistoryCandidates{}, err
	}
	if err := cut.Validate(); err != nil {
		return storage.HistoryCandidates{}, err
	}
	if err := after.Validate(); err != nil {
		return storage.HistoryCandidates{}, err
	}
	limit, limits, err := options.Effective()
	if err != nil {
		return storage.HistoryCandidates{}, err
	}
	stream, err := options.ResolveStream(after)
	if err != nil {
		return storage.HistoryCandidates{}, err
	}
	switch stream {
	case storage.HistoryStreamMessage:
		if !cut.ContainsMessagePosition(after) {
			return storage.HistoryCandidates{}, fmt.Errorf("storage: history cursor is outside message capture cut")
		}
		return b.queryMessageHistoryPage(ctx, cut, after, limit, limits)
	case storage.HistoryStreamRunEvent:
		if !cut.ContainsRunEventPosition(after) {
			return storage.HistoryCandidates{}, fmt.Errorf("storage: history cursor is outside run-event capture cut")
		}
		return b.queryRunEventHistoryPage(ctx, cut, after, limit, limits)
	default:
		return storage.HistoryCandidates{}, fmt.Errorf("storage: invalid history stream %q", stream)
	}
}

func (b *Backend) queryMessageHistoryPage(ctx context.Context, cut storage.HistoryCut, after storage.HistoryPosition, limit int, limits domain.ContinuityLimits) (storage.HistoryCandidates, error) {
	tx, err := b.db.SQL.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return storage.HistoryCandidates{}, fmt.Errorf("storage: begin history page: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	meta, err := postgresHistoryMetadata(ctx, tx, cut, after, limits.CandidateRecords+1)
	if err != nil {
		return storage.HistoryCandidates{}, err
	}
	out := storage.HistoryCandidates{}
	for i, item := range meta {
		if err := ctx.Err(); err != nil {
			return storage.HistoryCandidates{}, err
		}
		if i >= limits.CandidateRecords {
			out.ScanIncomplete = true
			break
		}
		if len(out.Records) >= limit {
			out.HasMore = true
			break
		}
		out.Next = storage.HistoryPosition{Stream: storage.HistoryStreamMessage, SessionID: item.sessionID, Position: item.position}
		remaining := limits.CandidateBytes - out.BytesInspected
		if item.bytes > remaining {
			if remaining > 0 {
				out.BytesInspected += remaining
			}
			if !item.hidden {
				out.Records = append(out.Records, postgresUnavailableHistoryCandidate(item))
			}
			more := i+1 < len(meta)
			out.HasMore = more
			out.ScanIncomplete = more
			break
		}
		out.BytesInspected += item.bytes
		if item.hidden {
			continue
		}
		if item.bytes > limits.ResultItemBytes {
			out.Records = append(out.Records, postgresUnavailableHistoryCandidate(item))
			continue
		}
		var body []byte
		if err := tx.QueryRowContext(ctx, `SELECT content FROM messages WHERE id = $1 AND session_id = $2 AND position = $3`, item.id, item.sessionID, item.position).Scan(&body); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
			return storage.HistoryCandidates{}, fmt.Errorf("storage: load bounded history payload: %w", err)
		}
		candidate := postgresUnavailableHistoryCandidate(item)
		if utf8.Valid(body) {
			candidate.Unavailable = false
			candidate.Truncated = false
			candidate.Text = string(body)
		}
		out.Records = append(out.Records, candidate)
	}
	if err := tx.Commit(); err != nil {
		return storage.HistoryCandidates{}, fmt.Errorf("storage: commit history page: %w", err)
	}
	return out, nil
}

func (b *Backend) queryRunEventHistoryPage(ctx context.Context, cut storage.HistoryCut, after storage.HistoryPosition, limit int, limits domain.ContinuityLimits) (storage.HistoryCandidates, error) {
	if len(cut.Runs) == 0 {
		return storage.HistoryCandidates{}, nil
	}
	tx, err := b.db.SQL.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return storage.HistoryCandidates{}, fmt.Errorf("storage: begin run-event history page: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	meta, err := postgresRunEventMetadata(ctx, tx, cut, after, limits.CandidateRecords+1)
	if err != nil {
		return storage.HistoryCandidates{}, err
	}
	out := storage.HistoryCandidates{}
	for i, item := range meta {
		if err := ctx.Err(); err != nil {
			return storage.HistoryCandidates{}, err
		}
		if i >= limits.CandidateRecords {
			out.ScanIncomplete = true
			break
		}
		if len(out.Records) >= limit {
			out.HasMore = true
			break
		}
		out.Next = storage.HistoryPosition{Stream: storage.HistoryStreamRunEvent, RunID: item.runID, Seq: item.seq}
		remaining := limits.CandidateBytes - out.BytesInspected
		if item.bytes > remaining {
			if remaining > 0 {
				out.BytesInspected += remaining
			}
			if !item.hidden {
				out.Records = append(out.Records, postgresUnavailableRunEventCandidate(item))
			}
			more := i+1 < len(meta)
			out.HasMore = more
			out.ScanIncomplete = more
			break
		}
		out.BytesInspected += item.bytes
		if item.hidden {
			continue
		}
		if item.bytes > limits.ResultItemBytes {
			out.Records = append(out.Records, postgresUnavailableRunEventCandidate(item))
			continue
		}
		var body []byte
		if err := tx.QueryRowContext(ctx, `SELECT payload FROM run_events WHERE run_id = $1 AND seq = $2`, item.runID, item.seq).Scan(&body); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
			return storage.HistoryCandidates{}, fmt.Errorf("storage: load bounded run-event payload: %w", err)
		}
		candidate := postgresUnavailableRunEventCandidate(item)
		if json.Valid(body) {
			candidate.Unavailable = false
			candidate.Truncated = false
			candidate.Text = string(body)
		}
		out.Records = append(out.Records, candidate)
	}
	if err := tx.Commit(); err != nil {
		return storage.HistoryCandidates{}, fmt.Errorf("storage: commit run-event history page: %w", err)
	}
	return out, nil
}

type postgresHistoryMetadataRow struct {
	id        string
	sessionID domain.SessionID
	runID     domain.RunID
	role      string
	createdAt int64
	position  int64
	bytes     int
	hidden    bool
}

func postgresHistoryMetadata(ctx context.Context, tx *sql.Tx, cut storage.HistoryCut, after storage.HistoryPosition, max int) ([]postgresHistoryMetadataRow, error) {
	clauses := make([]string, 0, len(cut.Sessions))
	args := make([]any, 0, len(cut.Sessions)*2+5)
	args = append(args, storage.TruncationRewind, storage.TruncationEdit)
	number := 3
	for _, c := range cut.Sessions {
		clauses = append(clauses, fmt.Sprintf("(m.session_id = $%d AND m.position <= $%d)", number, number+1))
		args = append(args, c.SessionID, c.Position)
		number += 2
	}
	query := `SELECT m.id, m.session_id, m.run_id, m.role, m.created_at, m.position,
		octet_length(m.content),
		EXISTS (
			SELECT 1 FROM session_truncations st
			JOIN messages cutoff ON cutoff.id = st.cutoff_message_id AND cutoff.session_id = st.session_id
			JOIN messages tail ON tail.id = st.tail_message_id AND tail.session_id = st.session_id
			WHERE st.session_id = m.session_id AND (st.reason = $1 OR st.reason = $2)
				AND cutoff.position <= tail.position
				AND m.position BETWEEN cutoff.position AND tail.position
		)
		FROM messages m WHERE (` + strings.Join(clauses, " OR ") + `)`
	if !after.IsZero() {
		query += fmt.Sprintf(` AND (m.session_id > $%d OR (m.session_id = $%d AND m.position > $%d))`, number, number+1, number+2)
		args = append(args, after.SessionID, after.SessionID, after.Position)
		number += 3
	}
	query += fmt.Sprintf(` ORDER BY m.session_id, m.position LIMIT $%d`, number)
	args = append(args, max)

	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("storage: enumerate history metadata: %w", err)
	}
	defer rows.Close()
	out := make([]postgresHistoryMetadataRow, 0, max)
	for rows.Next() {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		var item postgresHistoryMetadataRow
		var sessionID, runID string
		if err := rows.Scan(&item.id, &sessionID, &runID, &item.role, &item.createdAt, &item.position, &item.bytes, &item.hidden); err != nil {
			return nil, fmt.Errorf("storage: scan history metadata: %w", err)
		}
		item.sessionID = domain.SessionID(sessionID)
		item.runID = domain.RunID(runID)
		out = append(out, item)
	}
	return out, rows.Err()
}

type postgresRunEventMetadataRow struct {
	runID          domain.RunID
	sessionID      domain.SessionID
	seq            domain.EventSeq
	eventType      domain.EventType
	createdAt      int64
	payloadVersion int
	bytes          int
	hidden         bool
}

func postgresRunEventMetadata(ctx context.Context, tx *sql.Tx, cut storage.HistoryCut, after storage.HistoryPosition, max int) ([]postgresRunEventMetadataRow, error) {
	clauses := make([]string, 0, len(cut.Runs))
	args := make([]any, 0, len(cut.Runs)*3+6)
	args = append(args, storage.TruncationRewind, storage.TruncationEdit)
	number := 3
	for _, c := range cut.Runs {
		clauses = append(clauses, fmt.Sprintf("(e.run_id = $%d AND r.session_id = $%d AND e.seq <= $%d)", number, number+1, number+2))
		args = append(args, c.RunID, c.SessionID, c.Seq)
		number += 3
	}
	query := `SELECT e.run_id, r.session_id, e.seq, e.type, e.created_at, e.payload_version,
		octet_length(e.payload),
		EXISTS (
			SELECT 1 FROM messages projected
			JOIN session_truncations st ON st.session_id = projected.session_id
			JOIN messages cutoff ON cutoff.id = st.cutoff_message_id AND cutoff.session_id = st.session_id
			JOIN messages tail ON tail.id = st.tail_message_id AND tail.session_id = st.session_id
			WHERE projected.session_id = r.session_id AND projected.run_id = e.run_id
				AND (st.reason = $1 OR st.reason = $2)
				AND cutoff.position <= tail.position
				AND projected.position BETWEEN cutoff.position AND tail.position
		)
		FROM run_events e JOIN runs r ON r.id = e.run_id
		WHERE (` + strings.Join(clauses, " OR ") + `)`
	if !after.IsZero() {
		query += fmt.Sprintf(` AND (e.run_id > $%d OR (e.run_id = $%d AND e.seq > $%d))`, number, number+1, number+2)
		args = append(args, after.RunID, after.RunID, after.Seq)
		number += 3
	}
	query += fmt.Sprintf(` ORDER BY e.run_id, e.seq LIMIT $%d`, number)
	args = append(args, max)

	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("storage: enumerate run-event history metadata: %w", err)
	}
	defer rows.Close()
	out := make([]postgresRunEventMetadataRow, 0, max)
	for rows.Next() {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		var item postgresRunEventMetadataRow
		var runID, sessionID, eventType string
		var seq int64
		if err := rows.Scan(&runID, &sessionID, &seq, &eventType, &item.createdAt, &item.payloadVersion, &item.bytes, &item.hidden); err != nil {
			return nil, fmt.Errorf("storage: scan run-event history metadata: %w", err)
		}
		item.runID = domain.RunID(runID)
		item.sessionID = domain.SessionID(sessionID)
		item.seq = domain.EventSeq(seq)
		item.eventType = domain.EventType(eventType)
		out = append(out, item)
	}
	return out, rows.Err()
}

func postgresUnavailableHistoryCandidate(item postgresHistoryMetadataRow) storage.HistoryCandidate {
	return storage.HistoryCandidate{
		Ref: domain.SourceRef{
			SessionID: item.sessionID,
			RunID:     item.runID,
			MessageID: item.id,
			Kind:      string(domain.SourceKindMessage),
			CreatedAt: item.createdAt,
		},
		Author:      item.role,
		Unavailable: true,
		Truncated:   true,
	}
}

func postgresUnavailableRunEventCandidate(item postgresRunEventMetadataRow) storage.HistoryCandidate {
	return storage.HistoryCandidate{
		Ref: domain.SourceRef{
			SessionID: item.sessionID,
			RunID:     item.runID,
			EventSeq:  item.seq,
			Kind:      string(domain.SourceKindEvent),
			CreatedAt: item.createdAt,
		},
		EventType:      item.eventType,
		PayloadVersion: item.payloadVersion,
		Unavailable:    true,
		Truncated:      true,
	}
}

func postgresSessionArgs(ids []domain.SessionID) []any {
	out := make([]any, len(ids))
	for i := range ids {
		out[i] = ids[i]
	}
	return out
}

func postgresPlaceholders(n int) string {
	parts := make([]string, n)
	for i := range parts {
		parts[i] = fmt.Sprintf("$%d", i+1)
	}
	return strings.Join(parts, ",")
}

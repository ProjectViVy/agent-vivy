package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

const terminalTypes = `('run.completed','run.failed','run.cancelled','child.completed','child.failed','child.cancelled')`

// Append stores the commit atomically with a freshly assigned contiguous
// seq range. The exactly-one-terminal invariant (D-008) is enforced here:
// a run that already holds a terminal event rejects further appends, and
// a commit may contain at most one terminal event.
func (b *Backend) Append(ctx context.Context, commit storage.Commit) (domain.EventSeq, error) {
	if commit.RunID == "" || len(commit.Events) == 0 {
		return 0, storage.ErrCommitInvalid
	}
	terminals := 0
	for _, e := range commit.Events {
		if e.Type.Terminal() {
			terminals++
		}
	}
	if terminals > 1 {
		return 0, storage.ErrCommitInvalid
	}

	tx, err := b.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("storage: begin append: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var closed int
	if err := tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM run_events WHERE run_id = ? AND type IN `+terminalTypes,
		commit.RunID).Scan(&closed); err != nil {
		return 0, fmt.Errorf("storage: check terminal: %w", err)
	}
	if closed > 0 {
		return 0, storage.ErrRunClosed
	}

	var maxSeq sql.NullInt64
	if err := tx.QueryRowContext(ctx,
		`SELECT MAX(seq) FROM run_events WHERE run_id = ?`, commit.RunID).Scan(&maxSeq); err != nil {
		return 0, fmt.Errorf("storage: read max seq: %w", err)
	}

	seq := maxSeq.Int64
	stmt, err := tx.PrepareContext(ctx,
		`INSERT INTO run_events (run_id, seq, type, created_at, payload_version, payload)
		 VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, fmt.Errorf("storage: prepare insert: %w", err)
	}
	defer func() { _ = stmt.Close() }()

	for _, e := range commit.Events {
		seq++
		if _, err := stmt.ExecContext(ctx,
			commit.RunID, seq, string(e.Type), e.CreatedAt, e.PayloadVersion, e.Payload); err != nil {
			return 0, fmt.Errorf("storage: insert event seq %d: %w", seq, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("storage: commit append: %w", err)
	}
	return domain.EventSeq(seq), nil
}

// Replay streams the run's events with seq > after in seq order.
func (b *Backend) Replay(ctx context.Context, runID domain.RunID, after domain.EventSeq) (storage.Iterator[storage.Entry], error) {
	rows, err := b.db.QueryContext(ctx,
		`SELECT run_id, seq, type, created_at, payload_version, payload
		 FROM run_events WHERE run_id = ? AND seq > ? ORDER BY seq`, runID, int64(after))
	if err != nil {
		return nil, fmt.Errorf("storage: replay %s: %w", runID, err)
	}
	return &replayIter{rows: rows}, nil
}

type replayIter struct {
	rows *sql.Rows
	cur  storage.Entry
	err  error
}

func (it *replayIter) Next() bool {
	if !it.rows.Next() {
		it.err = it.rows.Err()
		return false
	}
	var runID, typ string
	var e storage.Entry
	if err := it.rows.Scan(&runID,
		(*int64)(&e.Event.Seq), &typ, &e.Event.CreatedAt,
		&e.Event.PayloadVersion, &e.Event.Payload); err != nil {
		it.err = fmt.Errorf("storage: scan replay row: %w", err)
		return false
	}
	e.Event.RunID = domain.RunID(runID)
	e.Event.Type = domain.EventType(typ)
	it.cur = e
	return true
}

func (it *replayIter) Value() storage.Entry { return it.cur }
func (it *replayIter) Err() error           { return it.err }
func (it *replayIter) Close() error         { return it.rows.Close() }

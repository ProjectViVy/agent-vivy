package postgres

import (
	"context"
	"fmt"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

var (
	_ storage.ChannelDeliveryStore    = (*Backend)(nil)
	_ storage.ChannelMaintenanceStore = (*Backend)(nil)
)

// UpsertChannelDelivery inserts the intent row or replaces every field of
// the row keyed by RunID. The caller owns the state machine; the store only
// persists it.
func (b *Backend) UpsertChannelDelivery(ctx context.Context, d storage.ChannelDelivery) error {
	if d.RunID == "" {
		return fmt.Errorf("storage: upsert channel delivery: empty run id")
	}
	_, err := b.db.ExecContext(ctx, `
		INSERT INTO channel_deliveries
			(run_id, session_id, channel, chat_id, topic_id, state, attempts, created_at_ms, updated_at_ms)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(run_id) DO UPDATE SET
			session_id = excluded.session_id,
			channel = excluded.channel,
			chat_id = excluded.chat_id,
			topic_id = excluded.topic_id,
			state = excluded.state,
			attempts = excluded.attempts,
			updated_at_ms = excluded.updated_at_ms`,
		string(d.RunID), string(d.SessionID), d.Channel, d.ChatID, d.TopicID,
		d.State, d.Attempts, d.CreatedAtMs, d.UpdatedAtMs)
	if err != nil {
		return fmt.Errorf("storage: upsert channel delivery %s: %w", d.RunID, err)
	}
	return nil
}

// ListOpenChannelDeliveries returns armed and pending rows, oldest first,
// so a restart reconcile drains delivery intents in arrival order. failed
// rows are terminal and never returned.
func (b *Backend) ListOpenChannelDeliveries(ctx context.Context) ([]storage.ChannelDelivery, error) {
	rows, err := b.db.QueryContext(ctx, `
		SELECT run_id, session_id, channel, chat_id, topic_id, state, attempts, created_at_ms, updated_at_ms
		FROM channel_deliveries
		WHERE state IN (?, ?)
		ORDER BY created_at_ms, run_id`, storage.ChannelDeliveryArmed, storage.ChannelDeliveryPending)
	if err != nil {
		return nil, fmt.Errorf("storage: list open channel deliveries: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []storage.ChannelDelivery
	for rows.Next() {
		var d storage.ChannelDelivery
		var runID, sessionID string
		if err := rows.Scan(&runID, &sessionID, &d.Channel, &d.ChatID, &d.TopicID,
			&d.State, &d.Attempts, &d.CreatedAtMs, &d.UpdatedAtMs); err != nil {
			return nil, fmt.Errorf("storage: scan channel delivery: %w", err)
		}
		d.RunID = domain.RunID(runID)
		d.SessionID = domain.SessionID(sessionID)
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("storage: iterate channel deliveries: %w", err)
	}
	return out, nil
}

// DeleteChannelDelivery removes the intent row. Deleting an unknown row is
// not an error: the delivery goroutine and the restart reconcile may race
// a successful send.
func (b *Backend) DeleteChannelDelivery(ctx context.Context, runID domain.RunID) error {
	if _, err := b.db.ExecContext(ctx,
		`DELETE FROM channel_deliveries WHERE run_id = ?`, string(runID)); err != nil {
		return fmt.Errorf("storage: delete channel delivery %s: %w", runID, err)
	}
	return nil
}

// PruneChannelInboundEvents deletes chanin_* run_events created before the
// cutoff. The escaped LIKE keeps the underscore a literal, and only the
// provenance pseudo-runs match — real run events are never touched.
func (b *Backend) PruneChannelInboundEvents(ctx context.Context, olderThan time.Time) (int, error) {
	res, err := b.db.ExecContext(ctx,
		`DELETE FROM run_events WHERE run_id LIKE 'chanin\_%' ESCAPE '\' AND created_at < ?`,
		olderThan.UnixMilli())
	if err != nil {
		return 0, fmt.Errorf("storage: prune channel inbound events: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("storage: prune channel inbound events: rows affected: %w", err)
	}
	return int(n), nil
}

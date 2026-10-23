package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"agent-vivy/internal/domain"
)

// Like tool exposure and compaction markers, idle queue removals use a
// synthetic Journal stream. It contains tombstones only, never another queue.
// IDs are never reused, so a later Run snapshot cannot resurrect a removal.
func queueControlRunID(sessionID domain.SessionID) domain.RunID {
	return domain.QueueControlJournalID(sessionID)
}
func (s *Service) applyQueueControls(ctx context.Context, sessionID domain.SessionID, pending []domain.QueuedTurn) ([]domain.QueuedTurn, error) {
	if len(pending) == 0 {
		return pending, nil
	}
	it, err := s.deps.Journal.Replay(ctx, queueControlRunID(sessionID), 0)
	if err != nil {
		return nil, fmt.Errorf("runtime: replay queue controls: %w", err)
	}
	defer it.Close()
	for it.Next() {
		ev := it.Value().Event
		var p payloadTurnDequeued
		if ev.Type != domain.EventTurnDequeued {
			return nil, errors.New("runtime: invalid queue control event")
		}
		if err := json.Unmarshal(ev.Payload, &p); err != nil {
			return nil, err
		}
		if p.QueueID == "" {
			return nil, errors.New("runtime: empty queue control id")
		}
		pending = removeQueued(pending, p.QueueID)
	}
	if err := it.Err(); err != nil {
		return nil, err
	}
	if err := it.Close(); err != nil {
		return nil, err
	}
	return pending, nil
}

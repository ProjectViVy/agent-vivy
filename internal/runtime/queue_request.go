package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"agent-vivy/internal/domain"
)

type queueSnapshot struct {
	Pending      []domain.QueuedTurn `json:"pending"`
	SteerMode    string              `json:"steer_mode"`
	FollowUpMode string              `json:"follow_up_mode"`
}

func queueRequest(ctx context.Context, o RunOptions) (*domain.QueuedTurnOptions, error) {
	if o.BeforeStart != nil || o.GoalRound != nil {
		return nil, errors.New("runtime: host admission callbacks cannot be queued")
	}
	if _, _, _, _, err := normalizeRunOptions(o.Mode, o.Profile, o.CollaborationMode, o.CollaborationVersion); err != nil {
		return nil, err
	}
	if _, err := normalizeThinkingMode(o.Thinking); err != nil {
		return nil, err
	}
	if _, err := normalizeFace(o.Face); err != nil {
		return nil, err
	}
	if _, err := normalizeFileContextsContext(ctx, o.FileContexts); err != nil {
		return nil, err
	}
	r := domain.QueuedTurnOptions{Mode: o.Mode, Profile: o.Profile, Face: o.Face, Thinking: o.Thinking, CollaborationMode: o.CollaborationMode, CollaborationVersion: o.CollaborationVersion, Provenance: o.Provenance, Attachments: o.Attachments, FileContexts: o.FileContexts, Continuity: o.Continuity}
	// The JSON round trip detaches every caller-owned byte slice and pointer.
	data, err := json.Marshal(r)
	if err != nil {
		return nil, err
	}
	if string(data) == "{}" {
		return nil, nil
	}
	var cloned domain.QueuedTurnOptions
	if err := json.Unmarshal(data, &cloned); err != nil {
		return nil, err
	}
	return &cloned, nil
}

func queuedRunOptions(item domain.QueuedTurn) RunOptions {
	if item.Request == nil {
		return RunOptions{}
	}
	r := item.Request
	return RunOptions{Mode: r.Mode, Profile: r.Profile, Face: r.Face, Thinking: r.Thinking, CollaborationMode: r.CollaborationMode, CollaborationVersion: r.CollaborationVersion, Provenance: r.Provenance, Attachments: r.Attachments, FileContexts: r.FileContexts, Continuity: r.Continuity}
}

func (s *Service) loadQueuedRequest(ctx context.Context, item *domain.QueuedTurn) error {
	if item.RequestRef == "" {
		return nil
	} // legacy text-only marker
	if s.deps.QueueRequests == nil {
		return errors.New("runtime: queued request store not wired")
	}
	data, ok, err := s.deps.QueueRequests.Get(ctx, item.RequestRef)
	if err != nil {
		return fmt.Errorf("runtime: load queued request: %w", err)
	}
	if !ok {
		return errors.New("runtime: queued request snapshot missing")
	}
	var request domain.QueuedTurnOptions
	if err := json.Unmarshal(data, &request); err != nil {
		return fmt.Errorf("runtime: decode queued request: %w", err)
	}
	item.Request = &request
	return nil
}

// Caller holds projectionMu: enqueue, removal, settlement and admission share
// that fence, so the snapshot and the new run's input cannot disagree.
func (s *Service) queueAdmissionSnapshot(ctx context.Context, sessionID domain.SessionID, consumed string) *queueSnapshot {
	q := s.queueFor(ctx, sessionID)
	s.mu.Lock()
	defer s.mu.Unlock()
	snapshot := &queueSnapshot{Pending: []domain.QueuedTurn{}, SteerMode: q.steerMode, FollowUpMode: q.followUpMode}
	for _, item := range append(append([]domain.QueuedTurn(nil), q.steer...), q.followUp...) {
		if item.ID == consumed {
			continue
		}
		item.Track = domain.QueueTrackFollowUp
		snapshot.Pending = append(snapshot.Pending, item)
	}
	return snapshot
}

func (s *Service) commitQueueAdmission(sessionID domain.SessionID, runID domain.RunID, consumed string) {
	s.mu.Lock()
	q := s.queues[sessionID]
	if q == nil {
		s.mu.Unlock()
		return
	}
	var removed []domain.QueuedTurn
	pending := append(append([]domain.QueuedTurn(nil), q.steer...), q.followUp...)
	q.steer, q.followUp = nil, nil
	for _, item := range pending {
		if item.ID == consumed {
			removed = append(removed, item)
			continue
		}
		item.EnqueuedOn = runID
		item.Track = domain.QueueTrackFollowUp
		q.followUp = append(q.followUp, item)
	}
	s.mu.Unlock()
	s.deleteQueuedRequests(context.Background(), removed)
}

func (s *Service) deleteQueuedRequests(ctx context.Context, items []domain.QueuedTurn) {
	if s.deps.QueueRequests == nil {
		return
	}
	for _, item := range items {
		if item.RequestRef == "" {
			continue
		}
		if err := s.deps.QueueRequests.Delete(context.WithoutCancel(ctx), item.RequestRef); err != nil {
			slog.Warn("queued request cleanup failed", "queue_id", item.ID, "err", err)
		}
	}
}

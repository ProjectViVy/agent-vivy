package runtime

import (
	"context"
	"errors"
	"strings"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

var (
	ErrWorkUnavailable     = errors.New("runtime: work control is unavailable")
	ErrWorkSessionRequired = errors.New("runtime: work session is required")
	ErrWorkRunUnavailable  = errors.New("runtime: work run is unavailable")
	ErrWorkRunTerminal     = errors.New("runtime: work run has reached a terminal work state")
)

func (s *Service) ReadWork(ctx context.Context, sessionID domain.SessionID) (domain.WorkState, error) {
	if s == nil || s.deps.Work == nil {
		return domain.WorkState{}, ErrWorkUnavailable
	}
	if strings.TrimSpace(string(sessionID)) == "" {
		return domain.WorkState{}, ErrWorkSessionRequired
	}
	return s.deps.Work.ReadWork(ctx, sessionID)
}

func (s *Service) CommitWork(ctx context.Context, mutation domain.WorkMutation) (storage.WorkCommitResult, error) {
	if s == nil || s.deps.Work == nil {
		return storage.WorkCommitResult{}, ErrWorkUnavailable
	}
	if err := storage.ValidateWorkMutation(mutation); err != nil {
		return storage.WorkCommitResult{}, err
	}
	result, err := s.deps.Work.CommitWork(ctx, mutation)
	if err == nil && !result.Replayed && s.deps.WorkSink != nil {
		s.deps.WorkSink.Publish(result.Event)
	}
	return result, err
}

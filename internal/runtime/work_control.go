package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

var (
	ErrWorkUnavailable     = errors.New("runtime: work control is unavailable")
	ErrWorkSessionRequired = errors.New("runtime: work session is required")
	ErrWorkRunUnavailable  = errors.New("runtime: work run is unavailable")
	ErrWorkRunTerminal     = errors.New("runtime: work run has reached a terminal work state")
)

// EnterPlan coordinates the human Goal-to-Plan transition. The Goal pause is
// durable before cancellation; neither admission nor projection locks are
// held while its owned run drains.
func (s *Service) EnterPlan(ctx context.Context, mutation domain.WorkMutation) (storage.WorkCommitResult, error) {
	if s == nil || s.deps.Work == nil || mutation.Kind != domain.WorkEventPlanEntered {
		return storage.WorkCommitResult{}, storage.ErrWorkInvalidMutation
	}
	if err := storage.ValidateWorkMutation(mutation); err != nil {
		return storage.WorkCommitResult{}, err
	}
	gate := s.sessionAdmission(mutation.SessionID)
	gate.Lock()
	s.projectionMu.Lock()
	if s.sessionDeleted(mutation.SessionID) {
		s.projectionMu.Unlock()
		gate.Unlock()
		return storage.WorkCommitResult{}, storage.ErrNotFound
	}
	state, err := s.ReadWork(ctx, mutation.SessionID)
	if err != nil {
		s.projectionMu.Unlock()
		gate.Unlock()
		return storage.WorkCommitResult{}, err
	}
	s.mu.Lock()
	runID := s.goalRuns[mutation.SessionID]
	s.mu.Unlock()
	needsPause := state.Goal != nil && state.Goal.Phase == domain.WorkPhaseActive
	needsDrain := state.Goal != nil && state.Goal.Phase == domain.WorkPhasePaused && runID != ""
	if !needsPause && !needsDrain {
		result, commitErr := s.CommitWork(ctx, mutation)
		s.projectionMu.Unlock()
		gate.Unlock()
		return result, commitErr
	}
	if state.Version != mutation.ExpectedVersion {
		s.projectionMu.Unlock()
		gate.Unlock()
		return storage.WorkCommitResult{}, storage.ErrWorkVersionConflict
	}
	s.mu.Lock()
	s.planTransitions[mutation.SessionID]++
	token := s.planTransitions[mutation.SessionID]
	s.mu.Unlock()
	versionAfterPause := state.Version
	if needsPause {
		pauseIdentity := sha256.Sum256([]byte("plan-pause\x00" + mutation.RequestID + "\x00" + mutation.RequestHash))
		pause := domain.WorkMutation{
			SessionID: mutation.SessionID, ExpectedVersion: state.Version,
			RequestID:   "plan-pause-" + hex.EncodeToString(pauseIdentity[:12]),
			RequestHash: hex.EncodeToString(pauseIdentity[:]),
			Kind:        domain.WorkEventGoalPaused, Goal: state.Goal.Ref,
			Reason: "paused for Plan", EvidenceRunID: runID,
		}
		paused, pauseErr := s.CommitWork(ctx, pause)
		if pauseErr != nil {
			s.projectionMu.Unlock()
			gate.Unlock()
			return storage.WorkCommitResult{}, pauseErr
		}
		versionAfterPause = paused.State.Version
		s.mu.Lock()
		s.goalDisarmed[mutation.SessionID] = struct{}{}
		s.mu.Unlock()
	}
	s.projectionMu.Unlock()
	gate.Unlock()
	s.CancelGoal(mutation.SessionID)
	if err := s.waitGoalRunCleanup(ctx, mutation.SessionID, runID); err != nil {
		return storage.WorkCommitResult{}, err
	}

	gate.Lock()
	defer gate.Unlock()
	s.projectionMu.Lock()
	defer s.projectionMu.Unlock()
	s.mu.Lock()
	stale := s.planTransitions[mutation.SessionID] != token
	_, deleted := s.deletedSessions[mutation.SessionID]
	s.mu.Unlock()
	if stale || deleted {
		return storage.WorkCommitResult{}, storage.ErrWorkVersionConflict
	}
	latest, err := s.ReadWork(ctx, mutation.SessionID)
	if err != nil {
		return storage.WorkCommitResult{}, err
	}
	if latest.Version != versionAfterPause || latest.Goal == nil || latest.Goal.Ref != state.Goal.Ref || latest.Goal.Phase != domain.WorkPhasePaused {
		return storage.WorkCommitResult{}, storage.ErrWorkVersionConflict
	}
	mutation.ExpectedVersion = latest.Version
	return s.CommitWork(ctx, mutation)
}

func (s *Service) waitGoalRunCleanup(ctx context.Context, sessionID domain.SessionID, runID domain.RunID) error {
	if runID == "" {
		return nil
	}
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		s.mu.Lock()
		current := s.goalRuns[sessionID] == runID
		_, active := s.active[runID]
		s.mu.Unlock()
		if !current && !active {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

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
	if err != nil && mutation.Kind == domain.WorkEventGoalPaused {
		// The durable phase is unchanged, but this process must not start
		// another Goal round after a failed human stop request.
		s.mu.Lock()
		s.goalDisarmed[mutation.SessionID] = struct{}{}
		s.mu.Unlock()
	}
	if err == nil && !result.Replayed && s.deps.WorkSink != nil {
		s.deps.WorkSink.Publish(result.Event)
	}
	return result, err
}

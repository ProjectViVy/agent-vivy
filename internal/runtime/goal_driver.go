package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

var (
	ErrGoalDriverUnavailable = errors.New("runtime: goal admission driver is unavailable")
	ErrGoalSessionRequired   = errors.New("runtime: goal session is required")
)

// WakeGoal schedules one event-driven Goal admission attempt. It deliberately
// does not poll: a Goal is woken only by an explicit human mutation or by a
// successful terminal event from its admitted round.
func (s *Service) WakeGoal(sessionID domain.SessionID) {
	s.wakeGoal(sessionID, true)
}

func (s *Service) wakeGoal(sessionID domain.SessionID, explicit bool) {
	if s == nil || strings.TrimSpace(string(sessionID)) == "" {
		return
	}
	s.mu.Lock()
	if s.stopping {
		s.mu.Unlock()
		return
	}
	if explicit {
		delete(s.goalDisarmed, sessionID)
	} else if _, disarmed := s.goalDisarmed[sessionID]; disarmed {
		s.mu.Unlock()
		return
	}
	if s.goalWakeRunning == nil {
		s.goalWakeRunning = make(map[domain.SessionID]struct{})
	}
	if s.goalWakePending == nil {
		s.goalWakePending = make(map[domain.SessionID]struct{})
	}
	if _, running := s.goalWakeRunning[sessionID]; running {
		s.goalWakePending[sessionID] = struct{}{}
		s.mu.Unlock()
		return
	}
	s.goalWakeRunning[sessionID] = struct{}{}
	s.goalWG.Add(1)
	s.mu.Unlock()
	go func() {
		defer s.goalWG.Done()
		for {
			s.mu.Lock()
			delete(s.goalWakePending, sessionID)
			stopping := s.stopping
			s.mu.Unlock()
			if stopping {
				s.finishGoalWake(sessionID)
				return
			}
			if err := s.admitGoalRound(context.Background(), sessionID); err != nil && !errors.Is(err, storage.ErrWorkRunConflict) {
				// A failed admission leaves the Goal durable and disarmed. The
				// next explicit human action can retry it; there is no outer retry.
			}
			s.mu.Lock()
			_, pending := s.goalWakePending[sessionID]
			stopping = s.stopping
			if !pending || stopping {
				delete(s.goalWakeRunning, sessionID)
				delete(s.goalWakePending, sessionID)
				s.mu.Unlock()
				return
			}
			s.mu.Unlock()
		}
	}()
}

func (s *Service) finishGoalWake(sessionID domain.SessionID) {
	s.mu.Lock()
	delete(s.goalWakeRunning, sessionID)
	delete(s.goalWakePending, sessionID)
	s.mu.Unlock()
}

// CancelGoal cancels the process-local run currently owned by a Goal. Durable
// pause/clear mutations remain authoritative even if the run is already
// settling; terminal cleanup will observe the new phase and will not wake it.
// StopAutomaticWork closes the process-local admission gate. Existing runs
// are left to CancelAll/WaitIdle; no new automatic round can be admitted
// after this returns.
func (s *Service) StopAutomaticWork() {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.stopping = true
	s.mu.Unlock()
	s.goalWG.Wait()
}

func (s *Service) CancelGoal(sessionID domain.SessionID) {
	if s == nil {
		return
	}
	s.mu.Lock()
	runID := s.goalRuns[sessionID]
	s.mu.Unlock()
	if runID != "" {
		s.Cancel(runID)
	}
}

// CancelRun is the public run/cancel path. A run still owned by the current
// active Goal is durably blocked before its cancellation is signalled. Other
// runs retain the ordinary Cancel behavior.
func (s *Service) CancelRun(ctx context.Context, runID domain.RunID) (bool, error) {
	if s == nil {
		return false, nil
	}
	s.mu.Lock()
	sessionID := s.goalRunSessions[runID]
	goalRef := s.goalRunRefs[runID]
	owned := sessionID != "" && s.goalRuns[sessionID] == runID
	s.mu.Unlock()
	if owned {
		state, err := s.ReadWork(ctx, sessionID)
		if err != nil {
			return false, err
		}
		if state.Goal != nil && state.Goal.Phase == domain.WorkPhaseActive &&
			state.Goal.Ref == goalRef && state.Goal.EvidenceRunID == runID {
			// Terminal publication holds projectionMu through ownership cleanup.
			// Recheck under that same boundary: WorkVersion CAS alone cannot
			// tell whether this process still owns the run being cancelled.
			s.projectionMu.Lock()
			s.mu.Lock()
			cancel := s.active[runID]
			current := s.goalRuns[sessionID] == runID && s.goalRunRefs[runID] == goalRef && cancel != nil
			s.mu.Unlock()
			if !current {
				s.projectionMu.Unlock()
				return false, nil
			}
			reason := "goal round cancelled"
			requestID := fmt.Sprintf("goal-cancel-%s-%d", runID, state.Version)
			hash := sha256.Sum256([]byte(fmt.Sprintf("%s\x00%s\x00%d\x00%s", requestID, runID, state.Version, reason)))
			if _, err := s.CommitWork(ctx, domain.WorkMutation{
				SessionID: sessionID, ExpectedVersion: state.Version,
				RequestID: requestID, RequestHash: hex.EncodeToString(hash[:]),
				Kind: domain.WorkEventGoalBlocked, Goal: goalRef,
				Reason: reason, EvidenceRunID: runID,
			}); err != nil {
				s.projectionMu.Unlock()
				return false, err
			}
			cancel()
			s.projectionMu.Unlock()
			// Pending approval/question cancellation can emitTerminal itself,
			// so finish it outside projectionMu. The durable disarm and signal
			// already won; cleanup racing this call cannot turn it into NotFound.
			s.Cancel(runID)
			return true, nil
		}
	}
	return s.Cancel(runID), nil
}

// GoalActivation reports process-local authority. It is intentionally never
// persisted: after a restart a durable active Goal is disarmed until a host
// explicitly wakes it.
func (s *Service) GoalActivation(sessionID domain.SessionID) (activation string, currentRunID domain.RunID) {
	if s == nil {
		return "disarmed", ""
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, disarmed := s.goalDisarmed[sessionID]; disarmed {
		return "disarmed", s.goalRuns[sessionID]
	}
	if runID := s.goalRuns[sessionID]; runID != "" {
		return "armed", runID
	}
	if _, starting := s.goalStarting[sessionID]; starting {
		return "armed", ""
	}
	return "disarmed", ""
}

func (s *Service) admitGoalRound(ctx context.Context, sessionID domain.SessionID) error {
	if s == nil || s.deps.Work == nil || s.deps.GoalRuns == nil {
		return ErrGoalDriverUnavailable
	}
	if strings.TrimSpace(string(sessionID)) == "" {
		return ErrGoalSessionRequired
	}
	sessionAdmission := s.sessionAdmission(sessionID)
	sessionAdmission.Lock()
	defer sessionAdmission.Unlock()
	s.mu.Lock()
	stopping := s.stopping
	humanPending := s.humanPending[sessionID] > 0
	_, disarmed := s.goalDisarmed[sessionID]
	s.mu.Unlock()
	if stopping || humanPending || disarmed {
		return nil
	}
	state, err := s.deps.Work.ReadWork(ctx, sessionID)
	if err != nil {
		return err
	}
	if state.Goal == nil || state.Goal.Phase != domain.WorkPhaseActive || state.Plan.Active {
		return nil
	}
	if state.Goal.RoundsStarted >= state.Goal.MaxRounds {
		return nil
	}
	s.mu.Lock()
	if s.goalRuns[sessionID] != "" {
		s.mu.Unlock()
		return nil
	}
	if _, starting := s.goalStarting[sessionID]; starting {
		s.mu.Unlock()
		return nil
	}
	s.goalStarting[sessionID] = struct{}{}
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.goalStarting, sessionID)
		s.mu.Unlock()
	}()

	s.mu.Lock()
	stopping = s.stopping
	humanPending = s.humanPending[sessionID] > 0
	_, disarmed = s.goalDisarmed[sessionID]
	s.mu.Unlock()
	if stopping || humanPending || disarmed {
		return nil
	}

	round := state.Goal.RoundsStarted + 1
	requestID := fmt.Sprintf("goal-round-%s-%d-%d", state.Goal.Ref.ID, state.Goal.Ref.Revision, round)
	hashInput := fmt.Sprintf("%s\x00%d\x00%d\x00%s", state.Goal.Ref.ID, state.Goal.Ref.Revision, round, state.Goal.Objective)
	hash := sha256.Sum256([]byte(hashInput))
	_, err = s.runWithAdmissionGate(ctx, sessionID, state.Goal.Objective, RunOptions{
		GoalRound: &GoalRoundAdmission{
			ExpectedVersion: state.Version,
			RequestID:       requestID,
			RequestHash:     hex.EncodeToString(hash[:]),
			Goal:            state.Goal.Ref,
			Round:           round,
		},
	}, nil, true)
	return err
}

const goalRecoveryPageSize = 1000

// recoveredGoalRef finds the admitted Goal reference for a run after a
// process restart. Goal admission is durable in the session work stream; the
// process-local maps are intentionally rebuilt only for settlement/resume.
func (s *Service) recoveredGoalRef(ctx context.Context, run domain.Run) (domain.GoalRef, bool) {
	if s == nil || s.deps.Work == nil || run.ID == "" || run.SessionID == "" {
		return domain.GoalRef{}, false
	}
	cursor := domain.WorkState{SessionID: run.SessionID}
	var found domain.GoalRef
	for {
		events, next, err := s.deps.Work.ReplayWork(ctx, run.SessionID, cursor, goalRecoveryPageSize)
		if err != nil {
			slog.Warn("restart recovery: goal admission replay failed", "run", string(run.ID), "after", cursor.Version, "err", err)
			return domain.GoalRef{}, false
		}
		if len(events) == 0 {
			return found, found.ID != ""
		}
		cursor = next
		for _, event := range events {
			if event.Kind != domain.WorkEventGoalRoundAdmitted {
				continue
			}
			admission := event.Admission
			if admission.RunID == "" {
				admission = event.Mutation.Admission
			}
			if found.ID == "" && admission.RunID == run.ID && admission.Goal.ID != "" && admission.Goal.Revision > 0 {
				found = admission.Goal
			}
		}
		if len(events) < goalRecoveryPageSize {
			return found, found.ID != ""
		}
	}
}

func (s *Service) goalCreatedByRun(ctx context.Context, sessionID domain.SessionID, runID domain.RunID) bool {
	if s == nil || s.deps.Work == nil || sessionID == "" || runID == "" {
		return false
	}
	state, err := s.deps.Work.ReadWork(ctx, sessionID)
	if err != nil || state.Goal == nil {
		return false
	}
	return state.Goal.Phase == domain.WorkPhaseActive &&
		state.Goal.RoundsStarted == 0 &&
		state.Goal.EvidenceRunID == runID
}

func (s *Service) rememberRecoveredGoalRun(ctx context.Context, run domain.Run) {
	goalRef, ok := s.recoveredGoalRef(ctx, run)
	if !ok {
		return
	}
	s.mu.Lock()
	s.goalRunSessions[run.ID] = run.SessionID
	s.goalRunRefs[run.ID] = goalRef
	s.mu.Unlock()
}

func (s *Service) settleGoalRound(ctx context.Context, sessionID domain.SessionID, runID domain.RunID, status domain.RunStatus, goalRef domain.GoalRef) {
	if s == nil || s.deps.Work == nil || sessionID == "" || runID == "" || goalRef.ID == "" || goalRef.Revision <= 0 {
		return
	}
	state, err := s.deps.Work.ReadWork(ctx, sessionID)
	if err != nil || state.Goal == nil || state.Goal.Phase != domain.WorkPhaseActive {
		return
	}
	if state.Goal.Ref != goalRef {
		// A human edit invalidated this run's report, but its wake was
		// deferred while the old run owned the session. Recheck the latest
		// durable revision only after the old run has been cleaned up.
		s.wakeGoal(sessionID, false)
		return
	}
	if status == domain.RunCompleted && state.Goal.RoundsStarted < state.Goal.MaxRounds {
		s.wakeGoal(sessionID, false)
		return
	}

	reason := "goal round failed"
	if status == domain.RunCancelled {
		reason = "goal round cancelled"
	} else if status == domain.RunCompleted {
		reason = "goal round limit reached"
	}
	requestID := fmt.Sprintf("goal-settle-%s-%d", runID, state.Version)
	hashInput := fmt.Sprintf("%s\x00%s\x00%d\x00%s", requestID, runID, state.Version, reason)
	hash := sha256.Sum256([]byte(hashInput))
	if _, err := s.CommitWork(ctx, domain.WorkMutation{
		SessionID:       sessionID,
		ExpectedVersion: state.Version,
		RequestID:       requestID,
		RequestHash:     hex.EncodeToString(hash[:]),
		Kind:            domain.WorkEventGoalBlocked,
		Goal:            state.Goal.Ref,
		Reason:          reason,
		EvidenceRunID:   runID,
	}); err != nil {
		slog.Error("goal round settlement failed", "session", string(sessionID), "run", string(runID), "goal", string(state.Goal.Ref.ID), "err", err)
	}
}

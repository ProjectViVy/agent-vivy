package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/tools"
)

const maxModelGoalRounds = 1000
const maxModelPlanBytes = 256 << 10

var _ tools.WorkControlOperations = (*Service)(nil)

func (s *Service) modelWorkContext(ctx context.Context) (domain.SessionID, domain.RunID, error) {
	if s == nil || s.deps.Work == nil || s.deps.Runs == nil {
		return "", "", ErrWorkUnavailable
	}
	sessionID := tools.SessionIDFromContext(ctx)
	runID := tools.RunIDFromContext(ctx)
	if strings.TrimSpace(string(sessionID)) == "" || strings.TrimSpace(string(runID)) == "" {
		return "", "", ErrWorkSessionRequired
	}
	run, err := s.deps.Runs.GetRun(ctx, runID)
	if err != nil {
		return "", "", err
	}
	if run.SessionID != sessionID || (run.Kind != "" && run.Kind != domain.RunKindPrimary) {
		return "", "", fmt.Errorf("runtime: work tool run is not the session primary run")
	}
	if run.Status != domain.RunActive {
		return "", "", ErrWorkRunUnavailable
	}
	s.mu.Lock()
	owner, live := s.runSessions[runID]
	s.mu.Unlock()
	if !live || owner != sessionID {
		return "", "", ErrWorkRunUnavailable
	}
	return sessionID, runID, nil
}

// WorkRunFenced reports whether a model Goal run has already committed a
// terminal work report. The tool adapter uses this process-local fence to
// reject every later tool call in the same model batch.
func (s *Service) WorkRunFenced(ctx context.Context) bool {
	if s == nil {
		return false
	}
	runID := tools.RunIDFromContext(ctx)
	if runID == "" {
		return false
	}
	s.mu.Lock()
	_, fenced := s.workFenced[runID]
	s.mu.Unlock()
	return fenced
}

func (s *Service) fenceWorkRun(runID domain.RunID) {
	if s == nil || runID == "" {
		return
	}
	s.mu.Lock()
	s.workFenced[runID] = struct{}{}
	s.mu.Unlock()
}

func modelWorkIdentity(runID domain.RunID, operation string, input any) (string, string, error) {
	raw, err := json.Marshal(input)
	if err != nil {
		return "", "", err
	}
	digest := sha256.Sum256(append([]byte("vivy:model-work:v1\x00"+string(runID)+"\x00"+operation+"\x00"), raw...))
	hash := hex.EncodeToString(digest[:])
	return "model-work-" + operation + "-" + hash[:24], hash, nil
}

func (s *Service) commitModelWork(ctx context.Context, sessionID domain.SessionID, runID domain.RunID, operation string, input any, build func(string, string, domain.WorkState) domain.WorkMutation) (domain.WorkState, error) {
	requestID, requestHash, err := modelWorkIdentity(domain.RunID(runID), operation, input)
	if err != nil {
		return domain.WorkState{}, err
	}
	state, err := s.ReadWork(ctx, sessionID)
	if err != nil {
		return domain.WorkState{}, err
	}
	mutation := build(requestID, requestHash, state)
	result, err := s.CommitWork(ctx, mutation)
	if err != nil {
		return domain.WorkState{}, err
	}
	return result.State, nil
}

func (s *Service) EnterPlanMode(ctx context.Context) (domain.WorkState, error) {
	sessionID, runID, err := s.modelWorkContext(ctx)
	if err != nil {
		return domain.WorkState{}, err
	}
	return s.commitModelWork(ctx, sessionID, runID, "enter-plan", nil, func(requestID, requestHash string, state domain.WorkState) domain.WorkMutation {
		return domain.WorkMutation{
			SessionID: sessionID, ExpectedVersion: state.Version,
			RequestID: requestID, RequestHash: requestHash, Kind: domain.WorkEventPlanEntered,
		}
	})
}

func (s *Service) SubmitPlan(ctx context.Context, markdown string) (domain.WorkState, error) {
	sessionID, runID, err := s.modelWorkContext(ctx)
	if err != nil {
		return domain.WorkState{}, err
	}
	markdown = strings.TrimSpace(markdown)
	if markdown == "" || len([]byte(markdown)) > maxModelPlanBytes {
		return domain.WorkState{}, fmt.Errorf("runtime: plan markdown is empty or too large")
	}
	return s.commitModelWork(ctx, sessionID, runID, "submit-plan", map[string]string{"markdown": markdown}, func(requestID, requestHash string, state domain.WorkState) domain.WorkMutation {
		return domain.WorkMutation{
			SessionID: sessionID, ExpectedVersion: state.Version,
			RequestID: requestID, RequestHash: requestHash, Kind: domain.WorkEventPlanSubmitted,
			PlanSubmissionID: "submission-" + requestID[len("model-work-submit-plan-"):],
			PlanMarkdown:     markdown, PlanOriginRunID: runID,
		}
	})
}

func (s *Service) GetGoal(ctx context.Context) (*domain.GoalState, error) {
	sessionID, _, err := s.modelWorkContext(ctx)
	if err != nil {
		return nil, err
	}
	state, err := s.ReadWork(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if state.Goal == nil {
		return nil, nil
	}
	goal := *state.Goal
	return &goal, nil
}

func (s *Service) CreateGoal(ctx context.Context, objective string, maxRounds int) (domain.WorkState, error) {
	sessionID, runID, err := s.modelWorkContext(ctx)
	if err != nil {
		return domain.WorkState{}, err
	}
	objective = strings.TrimSpace(objective)
	if objective == "" || len([]byte(objective)) > 8<<10 || maxRounds <= 0 || maxRounds > maxModelGoalRounds {
		return domain.WorkState{}, fmt.Errorf("runtime: invalid Goal objective or round limit")
	}
	state, err := s.commitModelWork(ctx, sessionID, runID, "create-goal", map[string]any{"objective": objective, "max_rounds": maxRounds}, func(requestID, requestHash string, state domain.WorkState) domain.WorkMutation {
		return domain.WorkMutation{
			SessionID: sessionID, ExpectedVersion: state.Version,
			RequestID: requestID, RequestHash: requestHash, Kind: domain.WorkEventGoalCreated,
			Goal:      domain.GoalRef{ID: "goal-" + requestID[len("model-work-create-goal-"):], Revision: 1},
			Objective: objective, MaxRounds: maxRounds,
		}
	})
	if err == nil {
		s.WakeGoal(sessionID)
	}
	return state, err
}

func (s *Service) ReportGoal(ctx context.Context, goalID string, revision int64, status, reason string) (domain.WorkState, error) {
	sessionID, runID, err := s.modelWorkContext(ctx)
	if err != nil {
		return domain.WorkState{}, err
	}
	s.mu.Lock()
	owned := s.goalRuns[sessionID] == runID
	s.mu.Unlock()
	if !owned {
		return domain.WorkState{}, errors.New("runtime: only the admitted Goal run may report Goal state")
	}
	goalID, reason = strings.TrimSpace(goalID), strings.TrimSpace(reason)
	if goalID == "" || revision <= 0 || (status != "completed" && status != "blocked") {
		return domain.WorkState{}, errors.New("runtime: invalid Goal report")
	}
	kind := domain.WorkEventGoalCompleted
	if status == "blocked" {
		kind = domain.WorkEventGoalBlocked
	}
	if reason == "" {
		reason = "model reported " + status
	}
	state, err := s.commitModelWork(ctx, sessionID, runID, "report-goal", map[string]any{
		"goal_id": goalID, "revision": revision, "status": status, "reason": reason,
	}, func(requestID, requestHash string, state domain.WorkState) domain.WorkMutation {
		return domain.WorkMutation{
			SessionID: sessionID, ExpectedVersion: state.Version,
			RequestID: requestID, RequestHash: requestHash, Kind: kind,
			Goal:   domain.GoalRef{ID: goalID, Revision: revision},
			Reason: reason, EvidenceRunID: runID,
		}
	})
	if err == nil {
		s.fenceWorkRun(runID)
	}
	return state, err
}

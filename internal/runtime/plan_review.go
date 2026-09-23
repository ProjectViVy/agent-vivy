package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/cloudwego/eino/components/tool"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/tools"
)

const planReviewInterruptPrefix = "vivy:plan-review:v1:"

var ErrPlanReviewUnavailable = errors.New("runtime: exact Plan review resume is unavailable")

type planReviewResume struct {
	SubmissionID string                    `json:"submission_id"`
	Action       domain.PlanDecisionAction `json:"action"`
	Feedback     string                    `json:"feedback,omitempty"`
}

func encodePlanReviewInterrupt(submissionID string) string {
	return planReviewInterruptPrefix + strings.TrimSpace(submissionID)
}

func decodePlanReviewInterrupt(info any) (string, bool) {
	encoded, ok := info.(string)
	if !ok || !strings.HasPrefix(encoded, planReviewInterruptPrefix) {
		return "", false
	}
	submissionID := strings.TrimSpace(strings.TrimPrefix(encoded, planReviewInterruptPrefix))
	return submissionID, submissionID != ""
}

func interruptPlanSubmission(ctx context.Context, toolName, result string) error {
	if toolName != tools.SubmitPlanName {
		return nil
	}
	var output struct {
		Plan struct {
			SubmissionID string `json:"submission_id"`
			ReviewStatus string `json:"review_status"`
		} `json:"plan"`
	}
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		return fmt.Errorf("runtime: decode submitted Plan result: %w", err)
	}
	if output.Plan.ReviewStatus != string(domain.PlanReviewPending) || output.Plan.SubmissionID == "" {
		return errors.New("runtime: submitted Plan did not enter review")
	}
	return tool.StatefulInterrupt(ctx, encodePlanReviewInterrupt(output.Plan.SubmissionID), output.Plan.SubmissionID)
}

// resumePlanReview handles only the exact submit_plan call that created the
// checkpoint. Other calls from the same model batch retain their interrupt
// and cannot consume the human decision.
func resumePlanReview(ctx context.Context, toolName string, maxResultBytes int) (string, bool, error) {
	if toolName != tools.SubmitPlanName {
		return "", false, nil
	}
	wasInterrupted, hasState, submissionID := tool.GetInterruptState[string](ctx)
	if !wasInterrupted {
		return "", false, nil
	}
	if !hasState || submissionID == "" {
		return "", true, errors.New("runtime: Plan review checkpoint is missing its submission ID")
	}
	isTarget, hasData, encodedDecision := tool.GetResumeContext[string](ctx)
	if !isTarget {
		return "", true, tool.StatefulInterrupt(ctx, encodePlanReviewInterrupt(submissionID), submissionID)
	}
	if !hasData {
		return "", true, errors.New("runtime: Plan review resume is missing its decision")
	}
	var decision planReviewResume
	if err := json.Unmarshal([]byte(encodedDecision), &decision); err != nil || decision.SubmissionID != submissionID {
		return "", true, errors.New("runtime: Plan review resume does not match its submission")
	}
	var message string
	switch decision.Action {
	case domain.PlanDecisionRevise:
		message = "The user returned this Plan for revision. Incorporate the feedback before submitting another Plan."
	case domain.PlanDecisionExecuteOnce:
		message = "The user approved this Plan for one execution. Continue within the existing tool permissions."
	case domain.PlanDecisionStartGoal:
		message = "The user accepted this Plan and requested a Goal. Continue only with work authorized by the active Goal."
	default:
		return "", true, fmt.Errorf("runtime: unsupported Plan review action %q", decision.Action)
	}
	if decision.Feedback != "" {
		message += "\nReviewer feedback: " + decision.Feedback
	}
	return compactToolResult(untrustedToolResultHeader+message, maxResultBytes), true, nil
}

func (s *Service) handlePlanReviewInterrupt(ctx context.Context, m *eventMapper, sessionID domain.SessionID, selectedTools []string, mode domain.RunMode) {
	details := m.interrupt
	fail := func(err error) {
		slog.Warn("Plan review handling failed", "run", string(m.runID), "err", err)
		s.emitTerminal(ctx, m, m.build(domain.EventRunFailed, payloadRunFailed{
			CauseCategory: causeInternalError,
			Message:       "The Plan could not be paused for review. No Plan execution was authorized.",
		}))
	}
	if details == nil || details.PlanSubmissionID == "" || details.ResumeTarget == "" ||
		details.ToolName != tools.SubmitPlanName || details.ToolCallID == "" {
		fail(errors.New("Plan review interrupt lacks an exact submission or tool-call target"))
		return
	}
	if s.engine.cfg.Checkpoints == nil {
		fail(errors.New("checkpoint bridge not wired"))
		return
	}
	if s.deps.Work == nil {
		fail(ErrWorkUnavailable)
		return
	}
	persistCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), terminalPersistTimeout)
	defer cancel()
	if _, ok, err := s.engine.cfg.Checkpoints.Get(persistCtx, checkpointIDFor(m.runID)); err != nil || !ok {
		fail(fmt.Errorf("checkpoint not readable: ok=%v: %w", ok, err))
		return
	}
	state, err := s.deps.Work.ReadWork(persistCtx, sessionID)
	if err != nil {
		fail(fmt.Errorf("read Plan review state: %w", err))
		return
	}
	plan := state.Plan
	if !plan.Active || plan.ReviewStatus != domain.PlanReviewPending ||
		plan.SubmissionID != details.PlanSubmissionID || plan.OriginRunID != m.runID ||
		plan.OriginToolCallID != details.ToolCallID {
		fail(errors.New("Plan submission does not match the interrupted run and tool call"))
		return
	}
	requestID, requestHash := planReviewSuspendIdentity(m.runID, details.PlanSubmissionID, details.ToolCallID, details.ResumeTarget)
	mutation := domain.WorkMutation{
		SessionID: sessionID, ExpectedVersion: state.Version,
		RequestID: requestID, RequestHash: requestHash, Kind: domain.WorkEventPlanReviewSuspended,
		PlanSubmissionID: details.PlanSubmissionID, PlanOriginRunID: m.runID,
		PlanOriginToolCallID: details.ToolCallID, PlanResumeTarget: details.ResumeTarget,
		PlanBlockedToolCalls: siblingToolCallIDs(m.toolBatch, details.ToolCallID),
	}
	if err := storage.ValidateWorkMutation(mutation); err != nil {
		fail(err)
		return
	}
	committed, err := s.deps.Work.CommitWork(persistCtx, mutation)
	if err != nil {
		fail(fmt.Errorf("persist Plan review target: %w", err))
		return
	}
	if committed.State.Plan.ResumeTarget != details.ResumeTarget || committed.State.Plan.SubmissionID != details.PlanSubmissionID {
		fail(errors.New("persisted Plan review target changed"))
		return
	}
	blocked := make(map[string]struct{}, len(committed.State.Plan.BlockedToolCalls))
	for _, callID := range committed.State.Plan.BlockedToolCalls {
		blocked[callID] = struct{}{}
	}
	ledger := s.ledgerForRun(m.runID)
	s.mu.Lock()
	if ctx.Err() != nil {
		s.mu.Unlock()
		s.emitTerminal(ctx, m, m.build(domain.EventRunCancelled, payloadRunCancelled{Reason: reasonUserRequested}))
		return
	}
	delete(s.workFenced, m.runID)
	s.workBlockedCalls[m.runID] = blocked
	s.pending[m.runID] = pendingRun{
		sessionID: sessionID, workspaceID: contextWorkspaceID(ctx), mapper: m,
		selectedTools: append([]string(nil), selectedTools...),
		mounted:       tools.MountedToolsFromContext(ctx), mode: mode,
		profile: policyProfile(ctx), snapshot: policySnapshot(ctx),
		sandboxMode: sandboxMode(ctx), approvalPolicy: approvalPolicy(ctx),
		face: runFace(ctx), ledger: ledger,
		planSubmissionID: details.PlanSubmissionID, planToolCallID: details.ToolCallID,
		planResumeTarget: details.ResumeTarget, planBlockedCalls: append([]string(nil), committed.State.Plan.BlockedToolCalls...),
	}
	s.mu.Unlock()
	if !committed.Replayed && s.deps.WorkSink != nil {
		s.deps.WorkSink.Publish(committed.Event)
	}
}

func siblingToolCallIDs(openCalls []openToolCall, submittedCallID string) []string {
	blocked := make([]string, 0, len(openCalls))
	for _, call := range openCalls {
		if call.id != "" && call.id != submittedCallID {
			blocked = append(blocked, call.id)
		}
	}
	return blocked
}

func planReviewSuspendIdentity(runID domain.RunID, submissionID, toolCallID, resumeTarget string) (string, string) {
	raw, _ := json.Marshal([]string{string(runID), submissionID, toolCallID, resumeTarget})
	digest := sha256.Sum256(append([]byte("vivy:plan-review-suspend:v1\x00"), raw...))
	requestHash := hex.EncodeToString(digest[:])
	return "plan-review-suspend-" + requestHash[:24], requestHash
}

// DecidePlan commits the human decision and resumes the originating model
// call once. The work event's request ID is the idempotency boundary: a replay
// returns its stored result without starting another engine resume.
func (s *Service) DecidePlan(ctx context.Context, mutation domain.WorkMutation) (storage.WorkCommitResult, error) {
	if mutation.Kind != domain.WorkEventPlanDecided {
		return storage.WorkCommitResult{}, storage.ErrWorkInvalidMutation
	}
	state, err := s.ReadWork(ctx, mutation.SessionID)
	if err != nil {
		return storage.WorkCommitResult{}, err
	}
	if state.Plan.ReviewStatus != domain.PlanReviewPending {
		// Let the store return the original committed event to an exact retry;
		// a distinct stale decision will be rejected by the reducer.
		return s.CommitWork(ctx, mutation)
	}
	if state.Plan.OriginRunID != "" {
		s.mu.Lock()
		pending, ok := s.pending[state.Plan.OriginRunID]
		s.mu.Unlock()
		if !ok || pending.planSubmissionID != mutation.PlanSubmissionID ||
			pending.planToolCallID != state.Plan.OriginToolCallID ||
			pending.planResumeTarget != state.Plan.ResumeTarget || state.Plan.ResumeTarget == "" {
			return storage.WorkCommitResult{}, ErrPlanReviewUnavailable
		}
	}
	result, err := s.CommitWork(ctx, mutation)
	if err != nil || result.Replayed || state.Plan.OriginRunID == "" {
		return result, err
	}

	s.mu.Lock()
	current, ok := s.pending[state.Plan.OriginRunID]
	if !ok || current.planSubmissionID != mutation.PlanSubmissionID || current.planResumeTarget != state.Plan.ResumeTarget {
		s.mu.Unlock()
		return result, nil
	}
	delete(s.pending, state.Plan.OriginRunID)
	s.mu.Unlock()

	resumeValue, err := json.Marshal(planReviewResume{
		SubmissionID: mutation.PlanSubmissionID, Action: mutation.PlanAction, Feedback: mutation.PlanFeedback,
	})
	if err != nil {
		return result, err
	}
	toolName := pendingToolName(current, current.planToolCallID)
	if toolName != tools.SubmitPlanName {
		return result, ErrPlanReviewUnavailable
	}
	resumeBatchIDs := make([]string, 0, 1+len(current.planBlockedCalls))
	resumeBatchIDs = append(resumeBatchIDs, current.planToolCallID)
	resumeBatchIDs = append(resumeBatchIDs, current.planBlockedCalls...)
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.resumeRun(current.sessionID, current.workspaceID, toolName, current.selectedTools, current.mounted,
			current.mode, current.profile, current.snapshot, current.sandboxMode, current.approvalPolicy,
			current.face, current.ledger, state.Plan.OriginRunID, current.planToolCallID,
			current.planResumeTarget, string(resumeValue), nil, "", "", resumeBatchIDs)
	}()
	return result, nil
}

func (s *Service) rebuildPendingPlanReview(ctx context.Context, run domain.Run, plan domain.PlanState, workspaceID string) {
	ledger := s.recoverBudgetLedger(ctx, run.ID)
	if ledger == nil {
		return
	}
	toolName, selectedTools, mode, face, profile, snapshot, sandboxMode, approvalPolicy := s.approvalDetails(ctx, run.ID)
	if toolName == "" {
		toolName = tools.SubmitPlanName
	}
	if len(selectedTools) == 0 {
		selectedTools = []string{toolName}
	}
	m := newEventMapper(run.ID, s.engine.cfg.MaxEventPayloadBytes)
	m.setRunScope(s.deps.TenantID, workspaceID, string(run.SessionID))
	m.setContextViewID(s.contextViewForRun(ctx, run.ID))
	providerName, modelID := s.usageRoutesForRun(ctx, run.ID)
	m.setUsageRoutes(providerName, modelID, s.engine.cfg.SummaryModelID)
	m.registerOpenCall(openToolCall{id: plan.OriginToolCallID, name: tools.SubmitPlanName})
	blocked := make(map[string]struct{}, len(plan.BlockedToolCalls))
	for _, callID := range plan.BlockedToolCalls {
		blocked[callID] = struct{}{}
	}
	goalRef, isGoalRun := s.recoveredGoalRef(ctx, run)
	s.mu.Lock()
	if isGoalRun {
		s.goalRuns[run.SessionID] = run.ID
		s.goalRunSessions[run.ID] = run.SessionID
		s.goalRunRefs[run.ID] = goalRef
	}
	s.workGates[run.ID] = &sync.Mutex{}
	s.workBlockedCalls[run.ID] = blocked
	s.pending[run.ID] = pendingRun{
		sessionID: run.SessionID, workspaceID: workspaceID, mapper: m,
		selectedTools: selectedTools, mounted: s.recoveredMounts(ctx, run.ID),
		mode: mode, profile: profile, snapshot: snapshot, sandboxMode: sandboxMode,
		approvalPolicy: approvalPolicy, face: face, ledger: ledger,
		planSubmissionID: plan.SubmissionID, planToolCallID: plan.OriginToolCallID,
		planResumeTarget: plan.ResumeTarget, planBlockedCalls: append([]string(nil), plan.BlockedToolCalls...),
	}
	s.runSessions[run.ID] = run.SessionID
	s.ledgers[run.ID] = ledger
	s.snapshots[run.ID] = snapshot
	s.mu.Unlock()
}

// CancelPlanReview closes a pending model run when a human leaves Plan mode.
// The durable PlanLeft mutation remains the source of truth; the run is
// cancelled only after that state change succeeds.
func (s *Service) CancelPlanReview(sessionID domain.SessionID) {
	if s == nil || sessionID == "" {
		return
	}
	s.mu.Lock()
	var runIDs []domain.RunID
	for runID, pending := range s.pending {
		if pending.sessionID == sessionID && pending.planSubmissionID != "" {
			runIDs = append(runIDs, runID)
		}
	}
	s.mu.Unlock()
	for _, runID := range runIDs {
		s.Cancel(runID)
	}
}

func (s *Service) cancelPlanReviewForRun(ctx context.Context, sessionID domain.SessionID, runID domain.RunID) {
	if s == nil || s.deps.Work == nil || sessionID == "" || runID == "" {
		return
	}
	state, err := s.deps.Work.ReadWork(ctx, sessionID)
	if err != nil || state.Plan.ReviewStatus != domain.PlanReviewPending || state.Plan.OriginRunID != runID {
		return
	}
	requestID, requestHash, err := modelWorkIdentity(runID, "cancel-plan-review", state.Plan.OriginToolCallID, map[string]string{
		"submission_id": state.Plan.SubmissionID,
	})
	if err != nil {
		return
	}
	if _, err := s.CommitWork(ctx, domain.WorkMutation{
		SessionID: sessionID, ExpectedVersion: state.Version,
		RequestID: requestID, RequestHash: requestHash, Kind: domain.WorkEventPlanReviewCancelled,
		PlanSubmissionID: state.Plan.SubmissionID, PlanOriginRunID: runID,
		Reason: "originating run ended before human review",
	}); err != nil && !errors.Is(err, storage.ErrWorkVersionConflict) {
		slog.Warn("cancel Plan review after run close failed", "run", string(runID), "err", err)
	}
}

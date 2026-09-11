package runtime

import (
	"context"
	"encoding/json"
	"sort"
	"strings"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/tools"
)

// InvokeActionTool is the runtime-owned ToolHost seam for a Control Action.
// It is deliberately narrower than ExecuteBrokerTool: an action may only
// invoke a tool from an already-active parent run, with that run's pinned
// policy snapshot, selected-tool allow-list, sandbox, budget, hooks, and
// journal. It cannot create a child worker or supply a replacement policy.
//
// The action host performs the action-level Grant and bridge authorization
// checks before calling this method. This method repeats the session/run and
// ToolHost checks immediately before execution to close the bridge TOCTOU.
func (s *Service) InvokeActionTool(ctx context.Context, sessionID domain.SessionID, runID domain.RunID, name string, args json.RawMessage) (string, error) {
	if s == nil || s.engine == nil || s.deps.Journal == nil || s.deps.Runs == nil || s.deps.Sink == nil || s.deps.Sessions == nil {
		return "", ErrPolicyDenied
	}
	if ctx == nil {
		ctx = context.Background()
	}
	sessionID = domain.SessionID(strings.TrimSpace(string(sessionID)))
	runID = domain.RunID(strings.TrimSpace(string(runID)))
	name = strings.TrimSpace(name)
	if sessionID == "" || runID == "" || name == "" || len(name) > 256 || strings.ContainsAny(name, "\r\n\x00") {
		return "", ErrPolicyDenied
	}
	if len(args) == 0 {
		args = json.RawMessage(`{}`)
	}
	if len(args) > actionToolMaxInputBytes || !json.Valid(args) {
		return "", ErrPolicyDenied
	}

	// Snapshot every process-local authority under one lock. The active run
	// check is intentionally required; an action cannot invoke a ToolHost in
	// an unrelated session or manufacture a run identity from its arguments.
	s.mu.Lock()
	boundSession, active := s.runSessions[runID]
	_, pending := s.pending[runID]
	allowed, selected := s.runTools[runID]
	snapshot := s.snapshots[runID]
	ledger := s.ledgers[runID]
	eng := s.engine
	s.mu.Unlock()
	if !active || pending || boundSession != sessionID || !selected || ledger == nil || snapshot.Profile == "" || snapshot.Hash == "" {
		return "", ErrPolicyDenied
	}
	run, err := s.deps.Runs.GetRun(ctx, runID)
	if err != nil || run.SessionID != sessionID || run.Status != domain.RunActive {
		return "", ErrPolicyDenied
	}
	if _, ok := allowed[name]; !ok {
		return "", ErrPolicyDenied
	}
	tool, ok := eng.toolByName[name]
	if !ok || tool == nil {
		return "", ErrPolicyDenied
	}
	if eng.cfg.Policy == nil {
		return "", ErrPolicyDenied
	}
	evaluation, err := eng.cfg.Policy.Evaluate(snapshot.Profile, tool.Spec(), args)
	if err != nil || evaluation.Snapshot.Hash != snapshot.Hash || evaluation.Decision != domain.PolicyAllow {
		return "", ErrPolicyDenied
	}

	// Re-resolve the session's sandbox/approval state immediately before the
	// ToolHost call. A pending run or a changed session policy must not be
	// treated as the action's authority.
	sandboxMode, approvalPolicy := s.sessionSandbox(ctx, sessionID)
	if !sandboxMode.Valid() || !approvalPolicy.Valid() {
		return "", ErrPolicyDenied
	}
	if err := ledger.ReserveToolCall(); err != nil {
		return "", ErrPolicyDenied
	}
	// Reserve both durable lifecycle events before the side effect. If the
	// second reservation cannot fit the run budget, no Tool call is started.
	if err := ledger.ReserveEvent(); err != nil {
		return "", ErrPolicyDenied
	}
	if err := ledger.ReserveEvent(); err != nil {
		return "", ErrPolicyDenied
	}

	// Re-check process/run authority after every potentially blocking decision
	// and before journaling the pre-effect record. appendRunEvent then performs
	// the storage/session deletion fence as the final admission check.
	s.mu.Lock()
	stillActive := s.runSessions[runID] == sessionID && s.active[runID] != nil
	_, stillPending := s.pending[runID]
	s.mu.Unlock()
	if !stillActive || stillPending {
		return "", ErrPolicyDenied
	}
	mapper := newEventMapper(runID, eng.cfg.MaxEventPayloadBytes)
	callID := newPrefixedID("action_tool_")
	_, err = s.appendRunEvent(ctx, mapper.build(domain.EventToolStarted, payloadToolStarted{ToolCallID: callID, ToolName: name}), false)
	if err != nil {
		return "", ErrPolicyDenied
	}

	toolCtx := withSelectedTools(ctx, mapKeys(allowed))
	toolCtx = withPolicyProfile(toolCtx, snapshot.Profile)
	toolCtx = withPolicySnapshot(toolCtx, snapshot)
	toolCtx = withRunID(toolCtx, runID)
	toolCtx = withSessionID(toolCtx, sessionID)
	toolCtx = withSessionSandbox(toolCtx, sandboxMode, approvalPolicy)
	toolCtx = withGovernanceEventSink(toolCtx, s.governanceSink(mapper, sessionID, ledger))
	adapter := newToolAdapter(tool, eng.cfg.MaxToolResultBytes, eng.cfg.Policy, eng.cfg.ToolHooks, eng.cfg.AutoApproveTools)
	result, invokeErr := adapter.InvokableRun(toolCtx, string(args))
	if invokeErr != nil {
		_, _ = s.appendRunEvent(context.WithoutCancel(ctx), mapper.build(domain.EventToolFinished, payloadToolFinished{ToolCallID: callID, ToolName: name, Error: "tool execution failed; the action did not complete"}), false)
		return "", ErrPolicyDenied
	}
	result = tools.RedactSensitive(result)
	if len(result) > actionToolMaxResultBytes || strings.Contains(result, "[REDACTED") {
		_, _ = s.appendRunEvent(context.WithoutCancel(ctx), mapper.build(domain.EventToolFinished, payloadToolFinished{ToolCallID: callID, ToolName: name, Error: "tool result rejected by the action boundary"}), false)
		return "", ErrPolicyDenied
	}
	if _, err := s.appendRunEvent(ctx, mapper.build(domain.EventToolFinished, payloadToolFinished{ToolCallID: callID, ToolName: name, Result: result}), false); err != nil {
		return "", ErrPolicyDenied
	}
	return result, nil
}

const (
	actionToolMaxInputBytes  = 1 << 20
	actionToolMaxResultBytes = 1 << 20
)

func mapKeys(values map[string]struct{}) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

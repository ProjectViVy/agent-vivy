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
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/tools"
)

const (
	// maxShellScriptBytes is the input bound for one !shell invocation. The
	// exact script is retained only in the protected state seam; public
	// journal/proposal projections carry a redacted label and a short hash.
	maxShellScriptBytes = 64 << 10
	// maxShellResultBytes is an outer bound on the serialized direct-shell
	// result. The command backend independently caps each output stream; this
	// envelope cap keeps a large stdout+stderr pair out of the event stream.
	maxShellResultBytes = 64 << 10
	// JSON event metadata (tool ids, result key, and the untrusted marker) is
	// outside the compacted result string. Leave a fixed envelope allowance so
	// the serialized tool.finished payload itself remains <= 64 KiB.
	shellResultEventOverhead = 2 << 10
	// shellResumeTargetPrefix is an opaque lifecycle marker used by restart
	// recovery. It contains no script or user content.
	shellResumeTargetPrefix = "shell:"
)

var (
	errShellApprovalRequired = errors.New("runtime: shell approval required")
	errShellRunClosed        = errors.New("runtime: shell run already closed")
)

// shellState is private execution state. It is never put in Journal or an
// Approval row. When ServiceDeps.ShellState is wired, it is serialized into
// the opaque blob store under a random reference so a pending approval can
// resume after restart without putting the original script in review data.
type shellState struct {
	Args      json.RawMessage  `json:"args"`
	SessionID domain.SessionID `json:"session_id"`
}

// shellPendingRun is deliberately independent from pendingRun. The latter
// is coupled to an Eino checkpoint and Resume(); direct shell runs have no
// model turn and therefore must never enter that path.
type shellPendingRun struct {
	sessionID      domain.SessionID
	mapper         *eventMapper
	runCtx         context.Context
	args           json.RawMessage
	toolCallID     string
	stateRef       string
	mode           domain.RunMode
	profile        domain.PolicyProfile
	snapshot       domain.PolicySnapshot
	sandboxMode    domain.SandboxMode
	approvalPolicy domain.ApprovalPolicy
	face           domain.Face
	ledger         *BudgetLedger
	requested      bool
	approvalReady  chan struct{}
	approvedHash   string
}

// shellTool resolves only the active bash surface. A hidden bash tool must
// not become an alternate RPC execution capability.
func (s *Service) shellTool() (tools.Tool, bool) {
	if s == nil || s.engine == nil {
		return nil, false
	}
	for _, candidate := range s.engine.activeTools {
		if candidate != nil && candidate.Spec().Name == tools.BashName {
			if probe, ok := candidate.(interface{ Available() bool }); ok && !probe.Available() {
				return nil, false
			}
			return candidate, true
		}
	}
	return nil, false
}

// ShellAvailable reports whether shell/start can be served by the active
// governed tool surface. It is a capability probe only; every invocation
// still repeats validation and policy checks inside RunShell.
func (s *Service) ShellAvailable() bool {
	if s == nil || s.deps.Journal == nil || s.deps.Runs == nil || s.deps.Messages == nil ||
		s.deps.Approvals == nil || s.deps.Sessions == nil || s.deps.Workspaces == nil ||
		s.deps.ShellState == nil || s.deps.Sink == nil {
		return false
	}
	_, ok := s.shellTool()
	return ok
}

// RunShell starts one direct, runtime-owned shell run. It persists only the
// run.started boundary synchronously; the tool lifecycle is driven in a
// detached goroutine and is visible through the ordinary run/subscribe
// stream. No user message and no model.request are created.
func (s *Service) RunShell(ctx context.Context, sessionID domain.SessionID, script string) (domain.RunID, error) {
	if s == nil || s.engine == nil {
		return "", errors.New("runtime: service not wired")
	}
	if err := s.applyPendingEngineReload(ctx, nil); err != nil {
		return "", err
	}
	if !s.ShellAvailable() {
		return "", ErrShellUnavailable
	}
	if sessionID == "" {
		return "", errors.New("runtime: shell session id is required")
	}
	if s.deps.Sessions == nil {
		return "", errors.New("runtime: session store not wired")
	}
	if _, err := s.deps.Sessions.GetSession(ctx, sessionID); err != nil {
		return "", fmt.Errorf("runtime: shell session: %w", err)
	}
	tool, ok := s.shellTool()
	if !ok || tool == nil {
		return "", ErrShellUnavailable
	}
	if len(script) == 0 || strings.TrimSpace(script) == "" {
		return "", errors.New("runtime: shell script must not be empty")
	}
	if len(script) > maxShellScriptBytes {
		return "", fmt.Errorf("runtime: shell script exceeds %d bytes", maxShellScriptBytes)
	}
	if strings.IndexByte(script, 0) >= 0 {
		return "", errors.New("runtime: shell script contains a NUL byte")
	}
	if s.deps.Workspaces == nil {
		// The backend's workspace manager is the host-isolation boundary. A
		// direct RPC must not degrade to a process with the current cwd.
		return "", ErrShellUnavailable
	}
	spec := tool.Spec()
	args, err := json.Marshal(map[string]string{"command": script})
	if err != nil {
		return "", fmt.Errorf("runtime: encode shell arguments: %w", err)
	}
	if err := tools.ValidateArgs(spec, args); err != nil {
		return "", err
	}
	if err := tools.ValidateArgsSafety(spec, args); err != nil {
		return "", err
	}
	if classifier, ok := tool.(tools.InvocationClassifier); ok {
		class, findings, err := classifier.ClassifyInvocation(args)
		if err != nil {
			return "", err
		}
		if class == tools.InvocationDenied {
			reason := strings.Join(findings, "; ")
			if reason == "" {
				reason = "deny-table match"
			}
			return "", fmt.Errorf("%w: %s (%s)", ErrPolicyDenied, tools.BashName, reason)
		}
	} else {
		// A shell-capable tool without the classifier is not a governed
		// shell. Failing closed here prevents future registry changes from
		// accidentally making an unclassified process callable.
		return "", ErrShellUnavailable
	}

	profile := s.defaultProfile
	if !profile.Valid() {
		profile = domain.PolicyProfileDefault
	}
	policy := s.engine.cfg.Policy
	if policy == nil {
		policy, err = NewPolicyEngine(nil)
		if err != nil {
			return "", err
		}
	}
	snapshot, err := policy.Snapshot(profile)
	if err != nil {
		return "", err
	}
	sandboxMode, approvalPolicy := s.sessionSandbox(ctx, sessionID)
	if sandboxMode == domain.SandboxModeReadOnly {
		return "", fmt.Errorf("%w: shell execution is disabled in read-only mode", ErrSandboxDenied)
	}
	ledger, err := NewBudgetLedger(s.deps.Budget)
	if err != nil {
		return "", err
	}
	if err := ledger.ReserveEvent(); err != nil {
		return "", err
	}
	runID := newRunID()
	if _, err := s.deps.Workspaces.Ensure(ctx, runID); err != nil {
		return "", fmt.Errorf("runtime: allocate shell workspace: %w", err)
	}
	now := time.Now().UnixMilli()
	run := domain.Run{ID: runID, SessionID: sessionID, Status: domain.RunAccepted, CreatedAt: now, Kind: domain.RunKindPrimary}
	m := newEventMapper(runID, s.engine.cfg.MaxEventPayloadBytes)
	// Provider/model are intentionally empty: a direct shell run is not a
	// model generation and must never pretend otherwise in audit data.
	started := m.build(domain.EventRunStarted, payloadRunStarted{
		Mode: string(domain.RunModeNormal), Face: string(domain.FaceTui),
		PolicyProfile: string(profile), PolicyHash: snapshot.Hash,
		SandboxMode: string(sandboxMode), ApprovalPolicy: string(approvalPolicy),
	})
	if err := s.deps.Runs.CreateRun(ctx, run); err != nil {
		return "", fmt.Errorf("runtime: create shell run: %w", err)
	}
	seq, err := s.deps.Journal.Append(ctx, storage.Commit{RunID: runID, Events: []domain.RunEvent{started}})
	if err != nil {
		return "", fmt.Errorf("runtime: persist shell run.started: %w", err)
	}
	started.Seq = seq
	if err := s.deps.Runs.SetRunStatus(ctx, runID, domain.RunActive); err != nil {
		return "", fmt.Errorf("runtime: activate shell run: %w", err)
	}
	s.publish(ctx, started)

	runCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	pending := shellPendingRun{
		sessionID: sessionID, mapper: m, args: append(json.RawMessage(nil), args...),
		toolCallID: newPrefixedID("call_shell_"), mode: domain.RunModeNormal,
		profile: profile, snapshot: snapshot, sandboxMode: sandboxMode,
		approvalPolicy: approvalPolicy, face: domain.FaceTui, ledger: ledger,
	}
	pending.runCtx = runCtx
	s.mu.Lock()
	s.active[runID] = cancel
	s.ledgers[runID] = ledger
	s.snapshots[runID] = snapshot
	s.mu.Unlock()
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.runShell(runCtx, pending, false)
	}()
	return runID, nil
}

// shellContext creates the same policy/session/governance context used by
// model tools, plus the direct marker that keeps command execution strictly
// foreground. Public message projections are already safe because direct
// shell events contain only redacted arguments and sanitized results.
func (s *Service) shellContext(ctx context.Context, p shellPendingRun) context.Context {
	ctx = withDirectShell(ctx)
	ctx = withSessionID(withRunID(withPolicySnapshot(withPolicyProfile(withRunMode(withFace(ctx, p.face), p.mode), p.profile), p.snapshot), p.mapper.runID), p.sessionID)
	ctx = withSessionSandbox(ctx, p.sandboxMode, p.approvalPolicy)
	ctx = tools.WithRunID(ctx, p.mapper.runID)
	ctx = tools.WithSessionID(ctx, p.sessionID)
	ctx = tools.WithMountedTools(ctx, tools.NewMountedTools())
	return withGovernanceEventSink(ctx, s.governanceSink(p.mapper, p.sessionID, p.ledger))
}

// runShell drives one direct lifecycle. approved is true only after the
// durable approval decision event has committed; no call reaches invoke
// before that point.
func (s *Service) runShell(ctx context.Context, p shellPendingRun, approved bool) {
	ctx = s.shellContext(ctx, p)
	if !p.requested {
		if !s.persistShellEvent(ctx, p, p.mapper.build(domain.EventToolRequested, payloadToolRequested{
			ToolCallID: p.toolCallID, ToolName: tools.BashName, Args: shellAuditArgs(p.args),
		}), true) {
			return
		}
		p.requested = true
	}
	tool, ok := s.shellTool()
	if !ok || tool == nil {
		s.failShell(ctx, p, errors.New("runtime: governed shell is unavailable"))
		return
	}
	adapter := newToolAdapter(tool, s.shellResultBudget(), s.engine.cfg.Policy, s.engine.cfg.ToolHooks, s.engine.cfg.AutoApproveTools)
	var finalArgs json.RawMessage
	var err error
	if approved {
		finalArgs, err = s.authorizeApprovedShell(ctx, adapter, p.args, p.approvedHash, p.snapshot.Hash)
	} else {
		finalArgs, err = s.authorizeShell(ctx, adapter, p.args, false)
	}
	if errors.Is(err, errShellApprovalRequired) {
		if err := s.openShellApproval(ctx, p, finalArgs); err != nil {
			if errors.Is(err, errShellRunClosed) {
				return
			}
			s.failShell(ctx, p, err)
		}
		return
	}
	if err != nil {
		s.failShell(ctx, p, err)
		return
	}
	if ctx.Err() != nil {
		s.emitTerminal(ctx, p.mapper, p.mapper.build(domain.EventRunCancelled, payloadRunCancelled{Reason: reasonUserRequested}))
		return
	}
	if p.stateRef != "" {
		defer s.deleteShellState(p.stateRef)
	}
	if !s.persistShellEvent(ctx, p, p.mapper.build(domain.EventToolStarted, payloadToolStarted{
		ToolCallID: p.toolCallID, ToolName: tools.BashName,
	}), false) {
		return
	}
	rawResult, execErr := adapter.invoke(ctx, string(finalArgs))
	result := ""
	if rawResult != "" {
		result = s.sanitizeShellResult(rawResult)
	}
	finished := payloadToolFinished{ToolCallID: p.toolCallID, ToolName: tools.BashName, Result: result}
	if execErr != nil {
		finished.Error = shellPublicError(execErr)
	}
	finishCtx := ctx
	if ctx.Err() != nil {
		finishCtx = context.WithoutCancel(ctx)
	}
	if !s.persistShellEvent(finishCtx, p, p.mapper.build(domain.EventToolFinished, finished), false) {
		return
	}
	if execErr != nil {
		if ctx.Err() != nil || errors.Is(execErr, context.Canceled) {
			s.emitTerminal(ctx, p.mapper, p.mapper.build(domain.EventRunCancelled, payloadRunCancelled{Reason: reasonUserRequested}))
			return
		}
		message := "The shell invocation could not be completed."
		if errors.Is(execErr, context.DeadlineExceeded) {
			message = "The shell invocation timed out."
		}
		s.emitTerminal(ctx, p.mapper, p.mapper.build(domain.EventRunFailed, payloadRunFailed{
			CauseCategory: causeToolError, Message: message,
		}))
		return
	}
	s.emitTerminal(ctx, p.mapper, p.mapper.build(domain.EventRunCompleted, payloadRunCompleted{}))
}

// authorizeShell mirrors toolAdapter's validation order while keeping the
// approval transition in Service, where the direct durable state lives.
func (s *Service) authorizeShell(ctx context.Context, adapter *toolAdapter, input json.RawMessage, approved bool) (json.RawMessage, error) {
	spec := adapter.t.Spec()
	if err := tools.ValidateArgs(spec, input); err != nil {
		return nil, err
	}
	if err := tools.ValidateArgsSafety(spec, input); err != nil {
		return nil, err
	}
	profile := policyProfile(ctx)
	evaluation, err := adapter.policy.Evaluate(profile, spec, input)
	if err != nil {
		return nil, err
	}
	emitGovernanceEvent(ctx, GovernanceEvent{Type: domain.EventPolicyEvaluated, ToolName: spec.Name, Decision: string(evaluation.Decision), Profile: profile, PolicyHash: evaluation.Snapshot.Hash, Reason: evaluation.Reason})
	if evaluation.Decision == domain.PolicyDeny {
		if runMode(ctx) == domain.RunModePlan && !spec.Readonly {
			return nil, fmt.Errorf("%w: %s", ErrPlanModeToolDenied, spec.Name)
		}
		return nil, fmt.Errorf("%w: %s (%s)", ErrPolicyDenied, spec.Name, evaluation.Reason)
	}
	args := append(json.RawMessage(nil), input...)
	if adapter.hooks != nil {
		args, err = adapter.hooks.PreToolUse(ctx, ToolHookCall{RunID: contextRunID(ctx), ToolName: spec.Name, Arguments: args, Profile: profile})
		if err != nil {
			return nil, err
		}
		if err := tools.ValidateArgs(spec, args); err != nil {
			return nil, err
		}
		if err := tools.ValidateArgsSafety(spec, args); err != nil {
			return nil, err
		}
		if err := validateDirectShellArgs(args); err != nil {
			return nil, err
		}
		if string(args) != string(input) {
			evaluation, err = adapter.policy.Evaluate(profile, spec, args)
			if err != nil {
				return nil, err
			}
			emitGovernanceEvent(ctx, GovernanceEvent{Type: domain.EventPolicyEvaluated, ToolName: spec.Name, Decision: string(evaluation.Decision), Profile: profile, PolicyHash: evaluation.Snapshot.Hash, Reason: "post-hook argument rewrite: " + evaluation.Reason})
			if evaluation.Decision != domain.PolicyAllow {
				if evaluation.Decision == domain.PolicyDeny {
					return nil, fmt.Errorf("%w: rewritten arguments for %s", ErrPolicyDenied, spec.Name)
				}
				return nil, fmt.Errorf("%w: rewritten arguments for %s require a fresh approval", ErrPolicyDenied, spec.Name)
			}
		}
	}
	if classifier, ok := adapter.t.(tools.InvocationClassifier); ok {
		class, findings, err := classifier.ClassifyInvocation(args)
		if err != nil {
			return nil, err
		}
		if class == tools.InvocationDenied {
			reason := strings.Join(findings, "; ")
			if reason == "" {
				reason = "deny-table match"
			}
			return nil, fmt.Errorf("%w: %s (%s)", ErrPolicyDenied, spec.Name, reason)
		}
		if class == tools.InvocationSafe && evaluation.Decision == domain.PolicyPrompt && approvalPolicy(ctx) == domain.ApprovalPolicyAuto && !approved {
			emitGovernanceEvent(ctx, GovernanceEvent{Type: domain.EventPolicyEvaluated, ToolName: spec.Name, Decision: string(domain.PolicyAllow), Profile: profile, PolicyHash: evaluation.Snapshot.Hash, Reason: "safe read-only invocation auto-approved"})
			return args, nil
		}
	}
	if evaluation.Decision == domain.PolicyPrompt {
		if approved {
			if approvalPolicy(ctx) == domain.ApprovalPolicyNever {
				return nil, fmt.Errorf("%w: %s (approval policy is 'never')", ErrPolicyDenied, spec.Name)
			}
			return args, nil
		}
		approvalEval := adapter.policy.EvaluateApprovalPolicy(approvalPolicy(ctx), spec, adapter.autoApprove)
		if approvalEval.AutoApprove {
			return args, nil
		}
		if !approvalEval.ShouldAsk {
			return nil, fmt.Errorf("%w: %s (%s)", ErrPolicyDenied, spec.Name, approvalEval.Reason)
		}
		return args, errShellApprovalRequired
	}
	return args, nil
}

// authorizeApprovedShell revalidates the exact reviewed arguments without
// re-running pre-tool hooks. A hook rewrite is part of the approved payload;
// running the hook again could turn approved script A into unreviewed B.
func (s *Service) authorizeApprovedShell(ctx context.Context, adapter *toolAdapter, args json.RawMessage, approvedHash, policyHash string) (json.RawMessage, error) {
	if approvedHash == "" || shellApprovalHash(args, policyHash, adapter.hooks) != approvedHash {
		return nil, fmt.Errorf("%w: approved shell arguments changed", ErrPolicyDenied)
	}
	if err := validateDirectShellArgs(args); err != nil {
		return nil, err
	}
	spec := adapter.t.Spec()
	if err := tools.ValidateArgs(spec, args); err != nil {
		return nil, err
	}
	if err := tools.ValidateArgsSafety(spec, args); err != nil {
		return nil, err
	}
	evaluation, err := adapter.policy.Evaluate(policyProfile(ctx), spec, args)
	if err != nil {
		return nil, err
	}
	if evaluation.Snapshot.Hash != policyHash {
		return nil, fmt.Errorf("%w: shell policy generation changed after approval", ErrPolicyDenied)
	}
	if evaluation.Decision == domain.PolicyDeny || approvalPolicy(ctx) == domain.ApprovalPolicyNever {
		return nil, fmt.Errorf("%w: approved shell no longer permitted", ErrPolicyDenied)
	}
	classifier, ok := adapter.t.(tools.InvocationClassifier)
	if !ok {
		return nil, ErrShellUnavailable
	}
	class, findings, err := classifier.ClassifyInvocation(args)
	if err != nil {
		return nil, err
	}
	if class == tools.InvocationDenied {
		return nil, fmt.Errorf("%w: %s", ErrPolicyDenied, strings.Join(findings, "; "))
	}
	return append(json.RawMessage(nil), args...), nil
}

func validateDirectShellArgs(args json.RawMessage) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(args, &fields); err != nil || fields == nil {
		return errors.New("runtime: direct shell arguments must be an object")
	}
	if len(fields) != 1 {
		return errors.New("runtime: direct shell accepts only command")
	}
	if _, ok := fields["command"]; !ok {
		return errors.New("runtime: direct shell command is required")
	}
	var command string
	if err := json.Unmarshal(fields["command"], &command); err != nil || strings.TrimSpace(command) == "" {
		return errors.New("runtime: direct shell command must be a non-empty string")
	}
	if len(command) > maxShellScriptBytes {
		return fmt.Errorf("runtime: direct shell command exceeds %d bytes", maxShellScriptBytes)
	}
	return nil
}

func (s *Service) openShellApproval(ctx context.Context, p shellPendingRun, args json.RawMessage) error {
	if s.deps.Approvals == nil {
		return errors.New("runtime: approval store not wired")
	}
	if _, ok := s.shellTool(); !ok {
		return ErrShellUnavailable
	}
	proposal, err := s.engine.PrepareProposal(ctx, tools.BashName, args)
	if err != nil {
		return err
	}
	stateRef := shellStateRefForRun(p.mapper.runID)
	state := shellState{Args: append(json.RawMessage(nil), args...), SessionID: p.sessionID}
	if err := s.storeShellState(ctx, stateRef, state); err != nil {
		return err
	}
	// Review metadata is public/auditable. It deliberately contains no raw
	// command, while the exact args remain behind stateRef.
	audit := shellAuditLabel(args)
	proposal.Action = tools.BashName
	proposal.Target = audit
	proposal.Preview = audit
	proposal.Data = shellStateRefData(stateRef)
	proposal.RiskFindings = boundShellFindings(proposal.RiskFindings)
	expiresAt := time.Now().Add(s.shellApprovalExpiration()).UnixMilli()
	approvedHash := shellApprovalHash(args, p.snapshot.Hash, s.engine.cfg.ToolHooks)
	approval := domain.Approval{
		ID: newPrefixedID("apr_"), RunID: p.mapper.runID, ToolCallID: p.toolCallID,
		Decision: domain.ApprovalPending, ExpiresAt: expiresAt, TimeoutAt: expiresAt,
		CreatedAt: time.Now().UnixMilli(), ResumeTarget: shellResumeTarget(p.mapper.runID),
		Kind: domain.ApprovalKindRun, ToolName: tools.BashName, Action: proposal.Action,
		Target: proposal.Target, Preview: proposal.Preview,
		RiskFindings: append([]string(nil), proposal.RiskFindings...), ProposalData: append([]byte(nil), proposal.Data...),
		SandboxMode: string(p.sandboxMode), ApprovalPolicy: string(p.approvalPolicy),
		PreconditionHash: approvedHash,
	}
	p.args = append(json.RawMessage(nil), args...)
	p.stateRef = stateRef
	p.requested = true
	p.approvedHash = approvedHash
	p.approvalReady = make(chan struct{})
	// Register before the approval row becomes externally visible. Review
	// operations wait for approvalReady, which closes only after the
	// approval-required event is durable.
	s.mu.Lock()
	s.shellPending[p.mapper.runID] = p
	s.mu.Unlock()
	if err := s.deps.Approvals.CreateApproval(ctx, approval); err != nil {
		s.removeShellPending(p.mapper.runID, p.mapper)
		close(p.approvalReady)
		s.deleteShellState(stateRef)
		return err
	}
	ev := p.mapper.build(domain.EventToolApprovalRequired, payloadToolApprovalRequired{
		ApprovalID: approval.ID, ToolCallID: p.toolCallID, ToolName: tools.BashName,
		Args: shellAuditArgs(args), ExpiresAt: expiresAt, SelectedTools: []string{tools.BashName},
		Mode: string(p.mode), Face: string(p.face), PolicyProfile: string(p.profile), PolicyHash: p.snapshot.Hash,
		SandboxMode: string(p.sandboxMode), ApprovalPolicy: string(p.approvalPolicy),
		PreconditionHash: p.approvedHash,
		Action:           approval.Action, Target: approval.Target, Preview: approval.Preview,
		RiskFindings: append([]string(nil), approval.RiskFindings...),
	})
	if !s.persistShellEvent(ctx, p, ev, false) {
		s.removeShellPending(p.mapper.runID, p.mapper)
		close(p.approvalReady)
		_ = s.cancelApproval(context.Background(), approval, "shell approval event could not be persisted")
		s.deleteShellState(stateRef)
		return errShellRunClosed
	}
	close(p.approvalReady)
	if ctx.Err() != nil {
		if _, removed := s.removeShellPending(p.mapper.runID, p.mapper); removed {
			_ = s.cancelApproval(context.Background(), approval, "shell run cancelled while suspending")
			s.deleteShellState(stateRef)
			s.emitTerminal(context.Background(), p.mapper, p.mapper.build(domain.EventRunCancelled, payloadRunCancelled{Reason: reasonUserRequested}))
		}
	}
	return nil
}

func (s *Service) resumeShell(p shellPendingRun, approval domain.Approval, decision string) {
	ctx := p.runCtx
	if ctx == nil {
		ctx = context.WithoutCancel(context.Background())
	}
	p.args = append(json.RawMessage(nil), p.args...)
	p.stateRef = shellStateRef(approval.ProposalData)
	p.approvedHash = approval.PreconditionHash
	p.requested = true
	if decision == domain.ApprovalDenied {
		ctx = s.shellContext(ctx, p)
		defer s.deleteShellState(p.stateRef)
		finished := payloadToolFinished{ToolCallID: p.toolCallID, ToolName: tools.BashName, Error: "bash was denied by the user and did not run; continue without it."}
		if s.persistShellEvent(ctx, p, p.mapper.build(domain.EventToolFinished, finished), false) {
			s.emitTerminal(ctx, p.mapper, p.mapper.build(domain.EventRunCompleted, payloadRunCompleted{}))
		}
		return
	}
	s.runShell(ctx, p, true)
}

func (s *Service) persistShellEvent(ctx context.Context, p shellPendingRun, re domain.RunEvent, toolCall bool) bool {
	if p.ledger != nil {
		if toolCall {
			if err := p.ledger.ReserveToolCall(); err != nil {
				s.emitTerminal(ctx, p.mapper, p.mapper.build(domain.EventRunFailed, payloadRunFailed{CauseCategory: causeInternalError, Message: "The shell run reached a safety budget."}))
				return false
			}
		}
		if err := p.ledger.ReserveEvent(); err != nil {
			s.emitTerminal(ctx, p.mapper, p.mapper.build(domain.EventRunFailed, payloadRunFailed{CauseCategory: causeInternalError, Message: "The shell run reached a safety budget."}))
			return false
		}
	}
	return s.persistAndPublish(withDirectShell(ctx), p.sessionID, re)
}

func (s *Service) failShell(ctx context.Context, p shellPendingRun, err error) {
	if p.stateRef != "" {
		defer s.deleteShellState(p.stateRef)
	}
	if s.persistShellEvent(ctx, p, p.mapper.build(domain.EventToolFinished, payloadToolFinished{
		ToolCallID: p.toolCallID, ToolName: tools.BashName, Error: shellPublicError(err),
	}), false) {
		s.emitTerminal(ctx, p.mapper, p.mapper.build(domain.EventRunFailed, payloadRunFailed{CauseCategory: causeInternalError, Message: "The shell invocation was blocked or failed."}))
	}
}

func (s *Service) shellResultBudget() int {
	budget := s.engine.cfg.MaxToolResultBytes
	maxResult := maxShellResultBytes - shellResultEventOverhead
	if eventBudget := s.engine.cfg.MaxEventPayloadBytes - shellResultEventOverhead; eventBudget > 0 && eventBudget < maxResult {
		maxResult = eventBudget
	}
	if budget <= 0 || budget > maxResult {
		return maxResult
	}
	return budget
}

func (s *Service) shellApprovalExpiration() time.Duration {
	if s.deps.ApprovalExpiration > 0 {
		return s.deps.ApprovalExpiration
	}
	return 5 * time.Minute
}

func shellResumeTarget(runID domain.RunID) string { return shellResumeTargetPrefix + string(runID) }

func shellStateRefForRun(runID domain.RunID) string { return "shell_state_" + string(runID) }

func (s *Service) cleanupOrphanedShellState(ctx context.Context) {
	lister, ok := s.deps.ShellState.(storage.BlobPrefixLister)
	if !ok || s.deps.Runs == nil {
		return
	}
	refs, err := lister.ListPrefix(ctx, "shell_state_")
	if err != nil {
		slog.Warn("protected shell state gc list failed", "err", err)
		return
	}
	for _, ref := range refs {
		if !strings.HasPrefix(ref, "shell_state_") {
			continue
		}
		runID := domain.RunID(strings.TrimPrefix(ref, "shell_state_"))
		run, err := s.deps.Runs.GetRun(ctx, runID)
		if err == nil && !run.Status.Terminal() {
			continue
		}
		if err != nil && !errors.Is(err, storage.ErrNotFound) {
			slog.Warn("protected shell state gc run lookup failed", "state_ref", ref, "err", err)
			continue
		}
		s.deleteShellState(ref)
	}
}

func shellStateRefData(ref string) json.RawMessage {
	data, _ := json.Marshal(struct {
		Ref string `json:"state_ref"`
	}{Ref: ref})
	return data
}

func shellStateRef(data []byte) string {
	var value struct {
		Ref string `json:"state_ref"`
	}
	if json.Unmarshal(data, &value) != nil {
		return ""
	}
	return strings.TrimSpace(value.Ref)
}

func (s *Service) storeShellState(ctx context.Context, ref string, state shellState) error {
	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("runtime: encode shell state: %w", err)
	}
	s.mu.Lock()
	s.shellStates[ref] = shellState{Args: append(json.RawMessage(nil), state.Args...), SessionID: state.SessionID}
	s.mu.Unlock()
	if s.deps.ShellState == nil {
		return nil
	}
	if err := s.deps.ShellState.Put(ctx, ref, data); err != nil {
		s.mu.Lock()
		delete(s.shellStates, ref)
		s.mu.Unlock()
		return fmt.Errorf("runtime: persist protected shell state: %w", err)
	}
	return nil
}

func (s *Service) loadShellState(ctx context.Context, ref string) (shellState, bool, error) {
	if ref == "" {
		return shellState{}, false, errors.New("runtime: shell state reference is empty")
	}
	s.mu.Lock()
	state, ok := s.shellStates[ref]
	s.mu.Unlock()
	if ok {
		return state, true, nil
	}
	if s.deps.ShellState == nil {
		return shellState{}, false, nil
	}
	data, found, err := s.deps.ShellState.Get(ctx, ref)
	if err != nil || !found {
		return shellState{}, found, err
	}
	if len(data) > maxShellScriptBytes+4096 {
		return shellState{}, false, errors.New("runtime: protected shell state exceeds bound")
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return shellState{}, false, fmt.Errorf("runtime: decode protected shell state: %w", err)
	}
	if len(state.Args) == 0 || len(state.Args) > maxShellScriptBytes+4096 {
		return shellState{}, false, errors.New("runtime: protected shell state arguments are invalid")
	}
	s.mu.Lock()
	s.shellStates[ref] = shellState{Args: append(json.RawMessage(nil), state.Args...), SessionID: state.SessionID}
	s.mu.Unlock()
	return state, true, nil
}

func (s *Service) deleteShellState(ref string) {
	if ref == "" {
		return
	}
	s.mu.Lock()
	delete(s.shellStates, ref)
	s.mu.Unlock()
	if s.deps.ShellState != nil {
		var err error
		for attempt := 1; attempt <= 3; attempt++ {
			deleteCtx, cancel := context.WithTimeout(context.Background(), terminalPersistTimeout)
			err = s.deps.ShellState.Delete(deleteCtx, ref)
			cancel()
			if err == nil {
				return
			}
		}
		if err != nil {
			// The opaque key is safe to log; never include the stored script.
			slog.Warn("protected shell state cleanup failed after retries", "state_ref", ref, "err", err)
		}
	}
}

func (s *Service) removeShellPending(runID domain.RunID, mapper *eventMapper) (shellPendingRun, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.shellPending[runID]
	if ok && (mapper == nil || p.mapper == mapper) {
		delete(s.shellPending, runID)
		return p, true
	}
	return shellPendingRun{}, false
}

func (s *Service) waitShellApprovalReady(ctx context.Context, runID domain.RunID) error {
	s.mu.Lock()
	p, ok := s.shellPending[runID]
	s.mu.Unlock()
	if !ok || p.approvalReady == nil {
		return errors.New("runtime: shell approval is not resumable")
	}
	select {
	case <-p.approvalReady:
		s.mu.Lock()
		_, stillPending := s.shellPending[runID]
		s.mu.Unlock()
		if !stillPending {
			return errors.New("runtime: shell approval closed before it became durable")
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func isShellApproval(approval domain.Approval) bool {
	return approval.ToolName == tools.BashName && strings.HasPrefix(approval.ResumeTarget, shellResumeTargetPrefix)
}

// rebuildShellPending restores only a waiting direct shell approval. It never
// restarts a command that was already started before the process disappeared;
// runs without a pending approval continue through the ordinary fail-closed
// recovery path in Service.recover.
func (s *Service) rebuildShellPending(ctx context.Context, run domain.Run, approval domain.Approval) error {
	if !isShellApproval(approval) {
		return ErrShellUnavailable
	}
	durable, err := s.shellApprovalRequiredDurable(ctx, run.ID, approval.ID)
	if err != nil {
		return err
	}
	if !durable {
		return errors.New("runtime: shell approval-required event is missing")
	}
	ref := shellStateRef(approval.ProposalData)
	if ref != shellStateRefForRun(run.ID) {
		return errors.New("runtime: protected shell state reference does not match run")
	}
	state, ok, err := s.loadShellState(ctx, ref)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("runtime: protected shell state is unavailable after restart")
	}
	if state.SessionID != "" && state.SessionID != run.SessionID {
		return errors.New("runtime: protected shell state session mismatch")
	}
	tool, ok := s.shellTool()
	if !ok || tool == nil {
		return ErrShellUnavailable
	}
	if err := tools.ValidateArgs(tool.Spec(), state.Args); err != nil {
		return err
	}
	if err := tools.ValidateArgsSafety(tool.Spec(), state.Args); err != nil {
		return err
	}
	classifier, ok := tool.(tools.InvocationClassifier)
	if !ok {
		return ErrShellUnavailable
	}
	class, findings, err := classifier.ClassifyInvocation(state.Args)
	if err != nil {
		return err
	}
	if class == tools.InvocationDenied {
		reason := strings.Join(findings, "; ")
		if reason == "" {
			reason = "deny-table match"
		}
		return fmt.Errorf("%w: %s (%s)", ErrPolicyDenied, tools.BashName, reason)
	}
	profile, snapshot, mode, face, sandboxMode, approvalPolicy := s.shellRecoveryMetadata(ctx, run.ID, approval)
	if approval.PreconditionHash == "" || shellApprovalHash(state.Args, snapshot.Hash, s.engine.cfg.ToolHooks) != approval.PreconditionHash {
		return errors.New("runtime: protected shell state or governance generation does not match approval")
	}
	ledger, err := s.recoverShellBudgetLedger(ctx, run.ID)
	if err != nil {
		return err
	}
	runCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	p := shellPendingRun{
		sessionID: run.SessionID, mapper: newEventMapper(run.ID, s.engine.cfg.MaxEventPayloadBytes),
		runCtx: runCtx, args: append(json.RawMessage(nil), state.Args...), toolCallID: approval.ToolCallID,
		stateRef: ref, mode: mode, profile: profile, snapshot: snapshot,
		sandboxMode: sandboxMode, approvalPolicy: approvalPolicy, face: face, ledger: ledger, requested: true,
		approvalReady: make(chan struct{}),
		approvedHash:  approval.PreconditionHash,
	}
	close(p.approvalReady)
	s.mu.Lock()
	s.active[run.ID] = cancel
	s.shellPending[run.ID] = p
	s.ledgers[run.ID] = ledger
	s.snapshots[run.ID] = snapshot
	s.mu.Unlock()
	return nil
}

func (s *Service) shellApprovalRequiredDurable(ctx context.Context, runID domain.RunID, approvalID string) (bool, error) {
	it, err := s.deps.Journal.Replay(ctx, runID, 0)
	if err != nil {
		return false, err
	}
	defer func() { _ = it.Close() }()
	for it.Next() {
		event := it.Value().Event
		if event.Type != domain.EventToolApprovalRequired {
			continue
		}
		var payload payloadToolApprovalRequired
		if json.Unmarshal(event.Payload, &payload) == nil && payload.ApprovalID == approvalID {
			return true, nil
		}
	}
	if err := it.Err(); err != nil {
		return false, err
	}
	return false, nil
}

// ApprovalRequiredDurable is the visibility barrier for approval RPCs.
// A pending database row is not human-visible until its matching journal
// event exists, which keeps review and restart recovery on the same truth.
func (s *Service) ApprovalRequiredDurable(ctx context.Context, runID domain.RunID, approvalID string) bool {
	durable, err := s.shellApprovalRequiredDurable(ctx, runID, approvalID)
	return err == nil && durable
}

func (s *Service) shellRecoveryMetadata(ctx context.Context, runID domain.RunID, approval domain.Approval) (domain.PolicyProfile, domain.PolicySnapshot, domain.RunMode, domain.Face, domain.SandboxMode, domain.ApprovalPolicy) {
	profile := s.defaultProfile
	if !profile.Valid() {
		profile = domain.PolicyProfileDefault
	}
	snapshot := domain.PolicySnapshot{Profile: profile}
	mode := domain.RunModeNormal
	face := domain.FaceTui
	sandboxMode := domain.SandboxMode(approval.SandboxMode)
	if !sandboxMode.Valid() {
		sandboxMode = domain.SandboxModeWorkspaceWrite
	}
	approvalPolicy := domain.ApprovalPolicy(approval.ApprovalPolicy)
	if !approvalPolicy.Valid() {
		approvalPolicy = domain.ApprovalPolicyAsk
	}
	it, err := s.deps.Journal.Replay(ctx, runID, 0)
	if err != nil {
		return profile, snapshot, mode, face, sandboxMode, approvalPolicy
	}
	defer func() { _ = it.Close() }()
	for it.Next() {
		ev := it.Value().Event
		switch ev.Type {
		case domain.EventRunStarted:
			var started payloadRunStarted
			if json.Unmarshal(ev.Payload, &started) == nil {
				if domain.PolicyProfile(started.PolicyProfile).Valid() {
					profile = domain.PolicyProfile(started.PolicyProfile)
				}
				if started.PolicyHash != "" {
					snapshot = domain.PolicySnapshot{Profile: profile, Hash: started.PolicyHash}
				}
				if domain.RunMode(started.Mode).Valid() {
					mode = domain.RunMode(started.Mode)
				}
				if domain.Face(started.Face).Valid() {
					face = domain.Face(started.Face)
				}
				if domain.SandboxMode(started.SandboxMode).Valid() {
					sandboxMode = domain.SandboxMode(started.SandboxMode)
				}
				if domain.ApprovalPolicy(started.ApprovalPolicy).Valid() {
					approvalPolicy = domain.ApprovalPolicy(started.ApprovalPolicy)
				}
			}
		case domain.EventToolApprovalRequired:
			var required payloadToolApprovalRequired
			if json.Unmarshal(ev.Payload, &required) == nil {
				if domain.PolicyProfile(required.PolicyProfile).Valid() {
					profile = domain.PolicyProfile(required.PolicyProfile)
				}
				if required.PolicyHash != "" {
					snapshot = domain.PolicySnapshot{Profile: profile, Hash: required.PolicyHash}
				}
				if domain.RunMode(required.Mode).Valid() {
					mode = domain.RunMode(required.Mode)
				}
				if domain.Face(required.Face).Valid() {
					face = domain.Face(required.Face)
				}
				if domain.SandboxMode(required.SandboxMode).Valid() {
					sandboxMode = domain.SandboxMode(required.SandboxMode)
				}
				if domain.ApprovalPolicy(required.ApprovalPolicy).Valid() {
					approvalPolicy = domain.ApprovalPolicy(required.ApprovalPolicy)
				}
			}
		}
	}
	return profile, snapshot, mode, face, sandboxMode, approvalPolicy
}

// recoverShellBudgetLedger mirrors BudgetLedger.ReplayEvent but does not
// charge a model generation for a direct shell tool.requested event.
func (s *Service) recoverShellBudgetLedger(ctx context.Context, runID domain.RunID) (*BudgetLedger, error) {
	ledger, err := NewBudgetLedger(s.deps.Budget)
	if err != nil {
		return nil, err
	}
	it, err := s.deps.Journal.Replay(ctx, runID, 0)
	if err != nil {
		return nil, fmt.Errorf("runtime: replay shell budget: %w", err)
	}
	defer func() { _ = it.Close() }()
	for it.Next() {
		ev := it.Value().Event
		if ev.Type.Terminal() {
			continue
		}
		if err := ledger.ReserveEvent(); err != nil {
			return nil, err
		}
		switch ev.Type {
		case domain.EventToolRequested:
			if err := ledger.ReserveToolCall(); err != nil {
				return nil, err
			}
		case domain.EventProviderRetry:
			if err := ledger.ReserveRetry(); err != nil {
				return nil, err
			}
		}
	}
	if err := it.Err(); err != nil {
		return nil, fmt.Errorf("runtime: replay shell budget: %w", err)
	}
	return ledger, nil
}

func shellScript(args json.RawMessage) string {
	var value struct {
		Command string `json:"command"`
	}
	if json.Unmarshal(args, &value) != nil {
		return ""
	}
	return value.Command
}

func shellAuditLabel(args json.RawMessage) string {
	script := shellScript(args)
	sum := sha256.Sum256([]byte(script))
	hash := hex.EncodeToString(sum[:])
	if len(hash) > 16 {
		hash = hash[:16]
	}
	return fmt.Sprintf("bash script [redacted bytes=%d sha256=%s]", len(script), hash)
}

func shellApprovalHash(args json.RawMessage, policyHash string, hooks *ToolHookChain) string {
	hash := sha256.New()
	_, _ = hash.Write(args)
	_, _ = hash.Write([]byte{0})
	_, _ = hash.Write([]byte(policyHash))
	if hooks != nil {
		_, _ = hash.Write([]byte(fmt.Sprintf("\x00%d", hooks.timeout)))
		for _, hook := range hooks.hooks {
			if hook != nil {
				identity := hook.Name()
				if versioned, ok := hook.(ToolHookIdentity); ok {
					identity = versioned.GovernanceIdentity()
				}
				_, _ = hash.Write([]byte("\x00" + identity))
			}
		}
	}
	sum := hash.Sum(nil)
	return hex.EncodeToString(sum)
}

func shellAuditArgs(args json.RawMessage) map[string]any {
	return map[string]any{"command": shellAuditLabel(args)}
}

func boundShellFindings(findings []string) []string {
	const maxFindings = 16
	if len(findings) > maxFindings {
		findings = findings[:maxFindings]
	}
	out := make([]string, 0, len(findings))
	for _, finding := range findings {
		finding = tools.RedactSensitive(finding)
		if len(finding) > 256 {
			finding = finding[:256] + "..."
		}
		out = append(out, finding)
	}
	return out
}

func (s *Service) sanitizeShellResult(raw string) string {
	budget := s.shellResultBudget()
	var result tools.CommandResult
	if err := json.Unmarshal([]byte(raw), &result); err == nil {
		result.Command = "bash (script redacted)"
		result.Cwd = "."
		result.Stdout = tools.RedactSensitive(result.Stdout)
		result.Stderr = tools.RedactSensitive(result.Stderr)
		encoded, marshalErr := json.Marshal(result)
		if marshalErr == nil {
			return compactToolResult(untrustedToolResultHeader+string(encoded), budget)
		}
	}
	return compactToolResult(untrustedToolResultHeader+tools.RedactSensitive(raw), budget)
}

func shellPublicError(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, context.Canceled) {
		return "shell invocation cancelled"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "shell invocation timed out"
	}
	if errors.Is(err, ErrPolicyDenied) || errors.Is(err, ErrPlanModeToolDenied) {
		return "shell invocation denied by policy"
	}
	if errors.Is(err, ErrHookBlocked) {
		return "shell invocation blocked by hook"
	}
	if errors.Is(err, ErrSandboxDenied) {
		return "shell invocation blocked by sandbox"
	}
	if errors.Is(err, ErrShellUnavailable) {
		return "governed shell is unavailable"
	}
	return "shell invocation failed validation or execution"
}

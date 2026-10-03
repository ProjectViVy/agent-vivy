package runtime

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	inofy "github.com/ProjectViVy/inofy"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/orchestration"
	"agent-vivy/internal/storage"
)

var (
	ErrWorkflowRecoveryRequired   = errors.New("runtime: active workflow requires recovery; refusing to replay it")
	ErrWorkflowNodeUnknownOutcome = errors.New("runtime: workflow node outcome is unknown; refusing to replay it")
	// ErrWorkflowLegacyFormat marks discriminator rows written by the removed
	// first-party descriptor engine. They stay committed as history but are
	// never re-executed or inspected.
	ErrWorkflowLegacyFormat = errors.New("runtime: workflow revision uses the retired descriptor format")
)

type WorkflowStartResult struct {
	Run      domain.Run
	Revision domain.WorkflowRevision
	Created  bool
}

type WorkflowNodeProjection struct {
	Key           string `json:"key"`
	Status        string `json:"status"`
	ChildRunID    string `json:"child_run_id,omitempty"`
	ResultDigest  string `json:"result_digest,omitempty"`
	ErrorCategory string `json:"error_category,omitempty"`
	Message       string `json:"message,omitempty"`
}

type WorkflowDetails struct {
	Run            domain.Run
	RevisionDigest string
	// Definition is the committed canonical INOFY definition.
	Definition   json.RawMessage
	EngineStatus string
	Nodes        []WorkflowNodeProjection
	Outputs      map[string]string
}

// inofyWorkflowLimits is the fixed host ceiling for admitted definitions: it
// is applied at compile time and passed as the run request limits, so the
// stored EffectiveLimits are deterministic.
func inofyWorkflowLimits() inofy.Limits {
	return inofy.Limits{
		MaxNodes: orchestration.MaxNodes, MaxEdges: orchestration.MaxEdges,
		Parallelism: orchestration.MaxWidth, MaxRepeatNesting: 1, MaxIterations: 8,
		MaxActivations: 12, MaxAttemptsPerCall: 1,
		NodeTimeoutMS: 60_000, RunTimeoutMS: 600_000,
		MaxDefinitionBytes:  64 << 10,
		MaxNodeInputBytes:   orchestration.MaxOutputBytes,
		MaxNodeOutputBytes:  orchestration.MaxOutputBytes,
		MaxOutputBytesTotal: orchestration.MaxOutputBytes,
		MaxCheckpointBytes:  16 << 20,
		MaxPredicateDepth:   8,
		MaxConcurrentRuns:   4, MaxPendingAdmissions: 32,
	}
}

// inofyHostBinding binds the compiled program identity to the authority record
// admitted under it; the executor refuses an envelope with any other binding.
func inofyHostBinding(authorityDigest, programDigest string) string {
	sum := sha256.Sum256([]byte(authorityDigest + "\x00" + programDigest))
	return hex.EncodeToString(sum[:])
}

// inofyEngine resolves the durable step/commit surface the INOFY RunStore
// adapter drives.
func (s *Service) inofyEngine() (storage.Engine, error) {
	engine, ok := s.deps.Sessions.(storage.Engine)
	if !ok || engine == nil {
		return nil, ErrINOFYStorageUnavailable
	}
	return engine, nil
}

func (s *Service) validateWorkflowDepth(parent domain.Run) error {
	// A workflow occupies one child level and each graph node occupies one
	// additional child level. Keep both inside the existing bounded tree.
	if parent.Depth+2 > maxChildSessionDepth {
		return fmt.Errorf("runtime: workflow and node children exceed maximum child depth %d", maxChildSessionDepth)
	}
	if parent.Kind == domain.RunKindWorkflow {
		return errors.New("runtime: nested workflows are not supported")
	}
	return nil
}

// WorkflowDefinitionSource binds an admitted Run to its reusable published
// definition (or to a draft snapshot when Revision is 0).
type WorkflowDefinitionSource struct {
	DefinitionID       string
	DefinitionRevision uint64
}

// inofyMaxRunInputBytes bounds the admitted run input payload.
const inofyMaxRunInputBytes = 64 << 10

// normalizeINOFYInput canonicalizes the run input: empty becomes {}, the value
// must be a JSON object, and the digest covers its canonical (key-sorted) form.
func normalizeINOFYInput(raw json.RawMessage) (json.RawMessage, string, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		raw = json.RawMessage(`{}`)
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, "", fmt.Errorf("runtime: workflow input must be valid JSON: %w", err)
	}
	if _, ok := v.(map[string]any); !ok {
		return nil, "", errors.New("runtime: workflow input must be a JSON object")
	}
	canonical, err := json.Marshal(v)
	if err != nil {
		return nil, "", err
	}
	if len(canonical) > inofyMaxRunInputBytes {
		return nil, "", errors.New("runtime: workflow input exceeds the admitted byte bound")
	}
	sum := sha256.Sum256(canonical)
	return canonical, "inofy-normal-v1:sha256:" + hex.EncodeToString(sum[:]), nil
}

// StartINOFYWorkflow atomically admits an immutable schema-2 revision and its
// workflow Run, then executes the compiled program through the embedded INOFY
// engine with the S11-D governed child executor. A repeated operation key
// returns the same Run; an active Run from another process is fenced for
// explicit recovery instead of replay.
func (s *Service) StartINOFYWorkflow(ctx context.Context, parentRunID domain.RunID, operationKey string, raw json.RawMessage) (WorkflowStartResult, error) {
	return s.startINOFYWorkflow(ctx, parentRunID, operationKey, raw, nil, nil, nil)
}

func (s *Service) startINOFYWorkflow(ctx context.Context, parentRunID domain.RunID, operationKey string, raw, input json.RawMessage, source *WorkflowDefinitionSource, trusted *trustedSpec) (WorkflowStartResult, error) {
	if s == nil || s.engine == nil || s.deps.Journal == nil || s.deps.Runs == nil ||
		s.deps.Sessions == nil || s.deps.Sink == nil || s.deps.Workspaces == nil ||
		s.deps.WorkflowRevisions == nil {
		return WorkflowStartResult{}, errors.New("runtime: workflow execution is not wired")
	}
	if _, err := s.inofyEngine(); err != nil {
		return WorkflowStartResult{}, err
	}
	if parentRunID == "" || strings.TrimSpace(operationKey) == "" || len(operationKey) > 128 {
		return WorkflowStartResult{}, errors.New("runtime: workflow parent Run and operation key are required")
	}
	operationKey = strings.TrimSpace(operationKey)

	// Serialize same-process retries so an accepted Run cannot be activated
	// twice while its process-local cancellation and authority state is built.
	s.workflowStartMu.Lock()
	defer s.workflowStartMu.Unlock()

	s.projectionMu.Lock()
	parent, snapshot, parentLedger, parentTools, err := s.currentChildAuthorizer(ctx, parentRunID)
	if err != nil {
		s.projectionMu.Unlock()
		return WorkflowStartResult{}, err
	}
	if err := s.validateWorkflowDepth(parent); err != nil {
		s.projectionMu.Unlock()
		return WorkflowStartResult{}, err
	}
	if s.sessionDeleted(parent.SessionID) {
		s.projectionMu.Unlock()
		return WorkflowStartResult{}, storage.ErrNotFound
	}
	allowedTools := s.workflowChildTools(parentTools)
	var admitted inofyAdmission
	if trusted != nil {
		// Trusted strategy admission is code-owned: it skips the authored
		// definition decoder and carries no tool ceiling.
		admitted = trusted.admitted
		allowedTools = nil
	} else {
		var admitErr error
		admitted, admitErr = validateINOFYDefinition(ctx, raw, allowedTools)
		if admitErr != nil {
			s.projectionMu.Unlock()
			return WorkflowStartResult{}, fmt.Errorf("%w: %w", ErrINOFYInvalidDefinition, admitErr)
		}
	}
	canonicalInput, inputDigest, err := normalizeINOFYInput(input)
	if err != nil {
		s.projectionMu.Unlock()
		return WorkflowStartResult{}, err
	}
	var definitionID string
	var definitionRevision uint64
	if source != nil {
		definitionID = source.DefinitionID
		definitionRevision = source.DefinitionRevision
	}
	session, err := s.deps.Sessions.GetSession(ctx, parent.SessionID)
	if err != nil {
		s.projectionMu.Unlock()
		return WorkflowStartResult{}, err
	}
	sandboxMode, approvalPolicy := session.EffectiveSandbox()
	var authorityJSON []byte
	var authorityDigest string
	if trusted != nil {
		authorityJSON, authorityDigest, err = trustedWorkflowAuthorityRecord(snapshot, sandboxMode, approvalPolicy, trusted.strategyID)
	} else {
		authorityJSON, authorityDigest, err = workflowAuthorityRecord(snapshot, sandboxMode, approvalPolicy, allowedTools)
	}
	if err != nil {
		s.projectionMu.Unlock()
		return WorkflowStartResult{}, err
	}
	limitsJSON, err := json.Marshal(inofyWorkflowLimits())
	if err != nil {
		s.projectionMu.Unlock()
		return WorkflowStartResult{}, err
	}
	descriptorDigest := sha256Hex(admitted.CanonicalJSON)
	hostBinding := inofyHostBinding(authorityDigest, admitted.Meta.ProgramDigest)

	if existing, lookupErr := s.deps.WorkflowRevisions.GetWorkflowRevisionByOperation(ctx, parent.ID, operationKey); lookupErr == nil {
		if existing.SchemaVersion != 2 || existing.DescriptorDigest != descriptorDigest ||
			existing.AuthorityDigest != authorityDigest || existing.ParentSessionID != parent.SessionID ||
			existing.ProgramDigest != admitted.Meta.ProgramDigest || existing.HostBindingID != hostBinding ||
			existing.InputDigest != inputDigest || existing.DefinitionID != definitionID ||
			existing.DefinitionRevision != definitionRevision ||
			!bytes.Equal(existing.DescriptorJSON, admitted.CanonicalJSON) {
			s.projectionMu.Unlock()
			return WorkflowStartResult{}, storage.ErrWorkflowRevisionConflict
		}
		run, getErr := s.deps.Runs.GetRun(ctx, existing.RunID)
		if getErr != nil {
			s.projectionMu.Unlock()
			return WorkflowStartResult{}, getErr
		}
		if run.Status.Terminal() {
			s.projectionMu.Unlock()
			return WorkflowStartResult{Run: run, Revision: existing, Created: false}, nil
		}
		if run.Status == domain.RunActive {
			s.mu.Lock()
			_, local := s.active[run.ID]
			s.mu.Unlock()
			s.projectionMu.Unlock()
			if local {
				return WorkflowStartResult{Run: run, Revision: existing, Created: false}, nil
			}
			return WorkflowStartResult{Run: run, Revision: existing, Created: false}, ErrWorkflowRecoveryRequired
		}
		// Run accepted but never executed (post-admission crash). Rebind the
		// committed projection and re-run the compiled program at the next
		// writer epoch.
		epoch, fenceErr := s.inofyResumeEpoch(ctx, existing.RunID)
		if fenceErr != nil {
			s.projectionMu.Unlock()
			return WorkflowStartResult{}, fenceErr
		}
		activationCtx, activationCancel := context.WithTimeout(context.WithoutCancel(ctx), terminalPersistTimeout)
		result, startErr := s.activateWorkflowLocked(activationCtx, parent, snapshot, parentLedger, allowedTools, existing, run, false)
		s.projectionMu.Unlock()
		if startErr != nil {
			activationCancel()
			return WorkflowStartResult{}, startErr
		}
		nodes, nodesErr := s.workflowNodes(activationCtx, existing.RunID, trustedStrategyOf(trusted), canonicalInput)
		if nodesErr != nil {
			activationCancel()
			return WorkflowStartResult{}, nodesErr
		}
		launchErr := s.launchINOFYWorkflow(activationCtx, result, admitted.Program, canonicalInput, inofy.ExecutionRef{
			RunID: string(existing.RunID), Epoch: epoch,
			ProgramDigest: existing.ProgramDigest, HostBindingID: existing.HostBindingID,
		}, nodes)
		activationCancel()
		return result, launchErr
	} else if !errors.Is(lookupErr, storage.ErrNotFound) {
		s.projectionMu.Unlock()
		return WorkflowStartResult{}, lookupErr
	}

	if err := parentLedger.ReserveEvent(); err != nil {
		s.projectionMu.Unlock()
		return WorkflowStartResult{}, err
	}
	now := time.Now().UnixMilli()
	workflowRunID := domain.RunID(newPrefixedID("workflow_"))
	rootID := parent.RootID
	if rootID == "" {
		rootID = parent.ID
	}
	run := domain.Run{
		ID: workflowRunID, SessionID: parent.SessionID, Status: domain.RunAccepted, CreatedAt: now,
		Kind: domain.RunKindWorkflow, ParentID: parent.ID, RootID: rootID, Depth: parent.Depth + 1,
	}
	workspace, err := s.deps.Workspaces.Ensure(withSessionID(ctx, parent.SessionID), run.ID)
	if err != nil {
		s.projectionMu.Unlock()
		return WorkflowStartResult{}, fmt.Errorf("runtime: allocate workflow workspace: %w", err)
	}
	mapper := newEventMapper(run.ID, s.engine.cfg.MaxEventPayloadBytes)
	mapper.setRunScope(s.deps.TenantID, workspace.ID, string(parent.SessionID))
	providerName, modelID := s.CurrentModel()
	mapper.setUsageRoutes(providerName, modelID, s.engine.cfg.SummaryModelID)
	started := mapper.build(domain.EventRunStarted, payloadRunStarted{
		Provider: providerName, Model: modelID, Mode: string(domain.RunModeNormal), Face: string(domain.FaceWeb),
		PolicyProfile: string(snapshot.Profile), PolicyHash: snapshot.Hash,
		SandboxMode: string(sandboxMode), ApprovalPolicy: string(approvalPolicy),
	})
	started.CreatedAt = now
	revision := domain.WorkflowRevision{
		RunID: run.ID, ParentRunID: parent.ID, ParentSessionID: parent.SessionID, RootRunID: rootID,
		OperationKey: operationKey, DescriptorDigest: descriptorDigest, AuthorityDigest: authorityDigest,
		DescriptorJSON: append([]byte(nil), admitted.CanonicalJSON...), AuthorityJSON: authorityJSON,
		SchemaVersion: 2, CreatedAt: now,
		ProgramDigest: admitted.Meta.ProgramDigest, CatalogDigest: admitted.Meta.CatalogDigest,
		CompilerVersion: admitted.Meta.CompilerVersion, EinoBuild: admitted.Meta.EinoBuild,
		InputDigest: inputDigest, InputJSON: append([]byte(nil), canonicalInput...),
		EffectiveLimits: limitsJSON, HostBindingID: hostBinding,
		DefinitionID: definitionID, DefinitionRevision: definitionRevision,
	}
	committed, err := s.deps.WorkflowRevisions.CommitWorkflowAdmission(ctx, storage.WorkflowAdmission{
		Revision: revision, Run: run, Started: started,
	})
	if err != nil {
		if releaser, ok := s.deps.Workspaces.(WorkspaceReleaser); ok {
			_ = releaser.Release(context.WithoutCancel(ctx), workspace)
		}
		s.projectionMu.Unlock()
		return WorkflowStartResult{}, err
	}
	if committed.Revision.DescriptorDigest != descriptorDigest || committed.Revision.AuthorityDigest != authorityDigest ||
		committed.Revision.SchemaVersion != 2 || committed.Revision.ProgramDigest != admitted.Meta.ProgramDigest ||
		committed.Revision.InputDigest != inputDigest || committed.Revision.DefinitionID != definitionID ||
		committed.Revision.DefinitionRevision != definitionRevision {
		s.projectionMu.Unlock()
		return WorkflowStartResult{}, storage.ErrWorkflowRevisionConflict
	}
	if committed.Run.Status.Terminal() {
		s.projectionMu.Unlock()
		return WorkflowStartResult{Run: committed.Run, Revision: committed.Revision, Created: false}, nil
	}
	if committed.Run.Status == domain.RunActive && !committed.Created {
		s.mu.Lock()
		_, local := s.active[committed.Run.ID]
		s.mu.Unlock()
		s.projectionMu.Unlock()
		if local {
			return WorkflowStartResult{Run: committed.Run, Revision: committed.Revision, Created: false}, nil
		}
		return WorkflowStartResult{Run: committed.Run, Revision: committed.Revision, Created: false}, ErrWorkflowRecoveryRequired
	}
	activationCtx, activationCancel := context.WithTimeout(context.WithoutCancel(ctx), terminalPersistTimeout)
	result, err := s.activateWorkflowLocked(activationCtx, parent, snapshot, parentLedger, allowedTools, committed.Revision, committed.Run, committed.Created)
	s.projectionMu.Unlock()
	if err != nil {
		activationCancel()
		return WorkflowStartResult{}, err
	}
	if committed.Created {
		s.publish(activationCtx, committed.Started)
	}
	nodes, nodesErr := s.workflowNodes(activationCtx, committed.Run.ID, trustedStrategyOf(trusted), canonicalInput)
	if nodesErr != nil {
		activationCancel()
		return WorkflowStartResult{}, nodesErr
	}
	launchErr := s.launchINOFYWorkflow(activationCtx, result, admitted.Program, canonicalInput, inofy.ExecutionRef{
		RunID: string(committed.Run.ID), Epoch: 1,
		ProgramDigest: committed.Revision.ProgramDigest, HostBindingID: committed.Revision.HostBindingID,
	}, nodes)
	activationCancel()
	return result, launchErr
}

// inofyResumeEpoch returns the writer epoch a re-run must carry: the next
// epoch after the stored projection, or 1 when nothing was committed.
func (s *Service) inofyResumeEpoch(ctx context.Context, runID domain.RunID) (uint64, error) {
	engine, err := s.inofyEngine()
	if err != nil {
		return 0, err
	}
	state, err := engine.LoadWorkflowStep(ctx, runID)
	if err != nil {
		return 0, fmt.Errorf("runtime: load workflow engine state: %w", err)
	}
	if state.Projection == nil {
		return 1, nil
	}
	if state.Projection.Status == storage.WorkflowStepRunning ||
		state.Projection.Status == storage.WorkflowStepWaiting ||
		state.Projection.Status == storage.WorkflowStepRecoveryRequired ||
		state.Projection.Status.Terminal() {
		// Anything past admitted is engine-owned: a running run is
		// recovery-classified, a waiting run needs the resume surface, and a
		// terminal or recovery_required run is never re-executed.
		return 0, ErrWorkflowRecoveryRequired
	}
	return state.Projection.Epoch + 1, nil
}

// activateWorkflowLocked installs process-local lineage, budget, workspace,
// and cancellation state while the session deletion fence is held.
func (s *Service) activateWorkflowLocked(ctx context.Context, parent domain.Run, snapshot domain.PolicySnapshot, parentLedger *BudgetLedger,
	allowedTools []string, revision domain.WorkflowRevision, run domain.Run, created bool) (WorkflowStartResult, error) {
	if s.sessionDeleted(parent.SessionID) {
		return WorkflowStartResult{}, storage.ErrNotFound
	}
	workspace, err := s.deps.Workspaces.Ensure(withSessionID(ctx, parent.SessionID), run.ID)
	if err != nil {
		return WorkflowStartResult{}, err
	}
	ledger, err := parentLedger.Child(s.deps.Budget)
	if err != nil {
		return WorkflowStartResult{}, err
	}
	if run.Status == domain.RunAccepted {
		if err := s.deps.Runs.SetRunStatus(ctx, run.ID, domain.RunActive); err != nil {
			return WorkflowStartResult{}, err
		}
		run.Status = domain.RunActive
	}
	s.mu.Lock()
	s.runSessions[run.ID] = parent.SessionID
	s.ledgers[run.ID] = ledger
	s.snapshots[run.ID] = snapshot
	s.runTools[run.ID] = childToolSet(allowedTools)
	s.mu.Unlock()
	_ = workspace // Ensure is also the host's workflow workspace authority.
	return WorkflowStartResult{Run: run, Revision: revision, Created: created}, nil
}

// launchINOFYWorkflow executes the admitted program in the background. The
// INOFY terminal commit is the only graph-native terminal owner: a Go-level
// error leaves the run non-terminal so restart recovery can classify it.
func (s *Service) launchINOFYWorkflow(ctx context.Context, result WorkflowStartResult, program *inofy.Program, input json.RawMessage, ref inofy.ExecutionRef, nodes inofy.NodeExecutor) error {
	if result.Run.ID == "" || program == nil {
		return errors.New("runtime: admitted workflow Run is empty")
	}
	engine, err := s.inofyEngine()
	if err != nil {
		return err
	}
	runCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	s.mu.Lock()
	if _, active := s.active[result.Run.ID]; active {
		s.mu.Unlock()
		cancel()
		return nil
	}
	s.active[result.Run.ID] = cancel
	s.runSessions[result.Run.ID] = result.Run.SessionID
	s.mu.Unlock()
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		// Terminal ownership stays with INOFY: the engine writes its own
		// classification (succeeded / failed / cancelled / recovery_required)
		// even when the run context is cancelled mid-flight. The host never
		// fabricates a native terminal here; an engine that exits without a
		// durable decision is re-classified by restart recovery.
		runs := newINOFYRunStore(engine)
		runs.publish = s.publish
		_, _ = program.Run(runCtx, inofy.RunRequest{
			Ref: ref, Input: input, Limits: inofyWorkflowLimits(),
		}, inofy.Bindings{Nodes: nodes, Runs: runs})
		s.mu.Lock()
		delete(s.active, result.Run.ID)
		s.mu.Unlock()
	}()
	return nil
}

type persistedWorkflowAuthority struct {
	PolicyProfile  domain.PolicyProfile  `json:"policy_profile"`
	PolicyHash     string                `json:"policy_hash"`
	SandboxMode    domain.SandboxMode    `json:"sandbox_mode"`
	ApprovalPolicy domain.ApprovalPolicy `json:"approval_policy"`
	ToolNames      []string              `json:"tool_names"`
	// TrustedStrategy classifies a host-bound strategy run. Empty keeps the
	// authored read-only ceiling; a non-empty value is only ever written by
	// the code-owned trusted admission path, never from caller input.
	TrustedStrategy string `json:"trusted_strategy,omitempty"`
}

func (s *Service) recoverWorkflowRun(ctx context.Context, run domain.Run, _ string) error {
	if s == nil || run.Kind != domain.RunKindWorkflow || run.Status.Terminal() || s.deps.WorkflowRevisions == nil ||
		s.engine == nil || s.deps.Sessions == nil {
		return errors.New("runtime: workflow recovery dependencies are incomplete")
	}
	engine, err := s.inofyEngine()
	if err != nil {
		return err
	}
	s.mu.Lock()
	_, active := s.active[run.ID]
	_, pending := s.pending[run.ID]
	s.mu.Unlock()
	if active || pending {
		return nil
	}
	revision, err := s.deps.WorkflowRevisions.GetWorkflowRevision(ctx, run.ID)
	if err != nil {
		return fmt.Errorf("runtime: load workflow revision for recovery: %w", err)
	}
	if revision.SchemaVersion != 2 {
		// Discriminator-1 rows stay historical and excluded from auto-recovery.
		return nil
	}
	if revision.RunID != run.ID || revision.ParentRunID != run.ParentID || revision.ParentSessionID != run.SessionID ||
		revision.RootRunID != run.RootID || revision.CreatedAt != run.CreatedAt {
		return storage.ErrWorkflowRevisionConflict
	}
	parent, err := s.deps.Runs.GetRun(ctx, revision.ParentRunID)
	if err != nil {
		return fmt.Errorf("runtime: load workflow authorizer lineage: %w", err)
	}
	rootID := parent.RootID
	if rootID == "" {
		rootID = parent.ID
	}
	if parent.SessionID != run.SessionID || rootID != run.RootID || parent.Depth+1 != run.Depth {
		return storage.ErrWorkflowRevisionConflict
	}
	authority, err := decodeWorkflowAuthority(revision)
	if err != nil {
		return err
	}
	canonicalTools := authority.ToolNames
	snapshot, err := s.engine.cfg.Policy.Snapshot(authority.PolicyProfile)
	if err != nil || snapshot.Hash != authority.PolicyHash {
		return errors.New("runtime: workflow policy snapshot is no longer compatible")
	}
	session, err := s.deps.Sessions.GetSession(ctx, run.SessionID)
	if err != nil {
		return fmt.Errorf("runtime: load workflow Session during recovery: %w", err)
	}
	sandboxMode, approvalPolicy := session.EffectiveSandbox()
	if sandboxMode != authority.SandboxMode || approvalPolicy != authority.ApprovalPolicy {
		return errors.New("runtime: workflow Session authority changed before recovery")
	}
	var allowedTools []string
	var admitted inofyAdmission
	if authority.TrustedStrategy != "" {
		// Trusted revisions rebind to the same code-owned strategy catalog
		// and require the bound Domain to be wired; a saved descriptor or
		// catalog drift fails the digest compare below.
		if s.deps.Cognitive == nil || s.deps.Cognitive.Domain == nil {
			return ErrCognitiveUnavailable
		}
		admitted, err = trustedStrategyAdmission(ctx, authority.TrustedStrategy)
	} else {
		allowedTools = s.workflowChildTools(canonicalTools)
		if !sameStrings(allowedTools, canonicalTools) {
			return errors.New("runtime: workflow tools no longer match the read-only authority ceiling")
		}
		admitted, err = validateINOFYDefinition(ctx, revision.DescriptorJSON, allowedTools)
	}
	if err != nil || admitted.Meta.ProgramDigest != revision.ProgramDigest ||
		admitted.Meta.CatalogDigest != revision.CatalogDigest ||
		!bytes.Equal(admitted.CanonicalJSON, revision.DescriptorJSON) ||
		inofyHostBinding(revision.AuthorityDigest, revision.ProgramDigest) != revision.HostBindingID {
		return storage.ErrWorkflowRevisionConflict
	}
	state, err := engine.LoadWorkflowStep(ctx, run.ID)
	if err != nil {
		return fmt.Errorf("runtime: load workflow engine state for recovery: %w", err)
	}
	var epoch uint64 = 1
	if state.Projection != nil {
		switch state.Projection.Status {
		case storage.WorkflowStepAdmitted, storage.WorkflowStepRunning:
			// Admitted re-executes from the committed log; running is
			// classified recovery_required by the engine on Load, never replayed.
			epoch = state.Projection.Epoch + 1
		default:
			// Waiting, terminal and recovery_required projections are settled.
			return nil
		}
	}
	ledgers, err := s.recoverBudgetLedgers(ctx, run.ID)
	if err != nil {
		return fmt.Errorf("runtime: restore workflow run-tree budget: %w", err)
	}
	current, err := s.deps.Runs.GetRun(ctx, run.ID)
	if err != nil {
		return err
	}
	if current.Status.Terminal() {
		return nil
	}
	if current.Status == domain.RunAccepted {
		if err := s.deps.Runs.SetRunStatus(ctx, run.ID, domain.RunActive); err != nil {
			return err
		}
		current.Status = domain.RunActive
	}
	s.mu.Lock()
	for id, ledger := range ledgers {
		s.ledgers[id] = ledger
		if p, ok := s.pending[id]; ok {
			p.ledger = ledger
			s.pending[id] = p
		}
	}
	s.snapshots[run.ID] = snapshot
	s.runTools[run.ID] = childToolSet(allowedTools)
	s.runSessions[run.ID] = run.SessionID
	s.mu.Unlock()
	nodes, nodesErr := s.workflowNodes(ctx, run.ID, authority.TrustedStrategy, revision.InputJSON)
	if nodesErr != nil {
		return nodesErr
	}
	return s.launchINOFYWorkflow(ctx, WorkflowStartResult{Run: current, Revision: revision}, admitted.Program, revision.InputJSON, inofy.ExecutionRef{
		RunID: string(run.ID), Epoch: epoch,
		ProgramDigest: revision.ProgramDigest, HostBindingID: revision.HostBindingID,
	}, nodes)
}

func workflowAuthorityRecord(snapshot domain.PolicySnapshot, sandbox domain.SandboxMode, approval domain.ApprovalPolicy, tools []string) ([]byte, string, error) {
	canonical, err := domain.CanonicalToolNames(tools)
	if err != nil {
		return nil, "", err
	}
	encoded, err := json.Marshal(persistedWorkflowAuthority{
		PolicyProfile: snapshot.Profile, PolicyHash: snapshot.Hash, SandboxMode: sandbox,
		ApprovalPolicy: approval, ToolNames: canonical,
	})
	if err != nil {
		return nil, "", err
	}
	sum := sha256.Sum256(encoded)
	return encoded, hex.EncodeToString(sum[:]), nil
}

func decodeWorkflowAuthority(revision domain.WorkflowRevision) (persistedWorkflowAuthority, error) {
	var authority persistedWorkflowAuthority
	decoder := json.NewDecoder(bytes.NewReader(revision.AuthorityJSON))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&authority); err != nil {
		return authority, errors.New("runtime: stored workflow authority is invalid")
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		return authority, errors.New("runtime: stored workflow authority has trailing data")
	}
	canonicalTools, err := domain.CanonicalToolNames(authority.ToolNames)
	if err != nil || authority.PolicyHash == "" || !authority.PolicyProfile.Valid() || !sameStrings(canonicalTools, authority.ToolNames) {
		return authority, errors.New("runtime: stored workflow authority is incomplete")
	}
	var canonicalAuthority []byte
	var authorityDigest string
	if authority.TrustedStrategy != "" {
		canonicalAuthority, authorityDigest, err = trustedWorkflowAuthorityRecord(
			domain.PolicySnapshot{Profile: authority.PolicyProfile, Hash: authority.PolicyHash},
			authority.SandboxMode, authority.ApprovalPolicy, authority.TrustedStrategy,
		)
	} else {
		canonicalAuthority, authorityDigest, err = workflowAuthorityRecord(
			domain.PolicySnapshot{Profile: authority.PolicyProfile, Hash: authority.PolicyHash},
			authority.SandboxMode, authority.ApprovalPolicy, canonicalTools,
		)
	}
	if err != nil || authorityDigest != revision.AuthorityDigest || !bytes.Equal(canonicalAuthority, revision.AuthorityJSON) {
		return authority, errors.New("runtime: stored workflow authority digest does not match")
	}
	return authority, nil
}

// inofyNodeEvent mirrors the journal payloads the RunStore adapter commits for
// graph node lifecycle events.
type inofyNodeEvent struct {
	started       bool
	completed     bool
	failed        bool
	waiting       bool
	resultDigest  string
	errorCategory string
	message       string
}

// inofyNodeStates replays the workflow journal into one state per graph
// node, keyed by the bare node id while matching the committed logical path.
func inofyNodeStates(events []domain.RunEvent, runID domain.RunID, nodePaths map[string]string) map[string]inofyNodeEvent {
	states := make(map[string]inofyNodeEvent, len(nodePaths))
	pathToID := make(map[string]string, len(nodePaths))
	for id, path := range nodePaths {
		states[id] = inofyNodeEvent{}
		pathToID[path] = id
	}
	for _, event := range events {
		var payload struct {
			NodeKey       string `json:"node_key"`
			ResultDigest  string `json:"result_digest"`
			CauseCategory string `json:"cause_category"`
			ErrorCategory string `json:"error_category"`
			Message       string `json:"message"`
		}
		if len(event.Payload) > 0 {
			_ = json.Unmarshal(event.Payload, &payload)
		}
		id, known := pathToID[payload.NodeKey]
		if !known {
			continue
		}
		state := states[id]
		switch event.Type {
		case domain.EventWorkflowNodeStarted, domain.EventWorkflowNodeAttempt:
			state.started = true
		case domain.EventWorkflowNodeCompleted:
			state.started, state.completed = true, true
			state.resultDigest = payload.ResultDigest
		case domain.EventWorkflowNodeDegraded:
			state.started, state.completed = true, true
			state.resultDigest = payload.ResultDigest
		case domain.EventWorkflowNodeFailed:
			state.started, state.failed = true, true
			if payload.ErrorCategory != "" {
				state.errorCategory = payload.ErrorCategory
			} else {
				state.errorCategory = payload.CauseCategory
			}
			state.message = payload.Message
		case domain.EventWorkflowNodeWaiting:
			state.started, state.waiting = true, true
		}
		states[id] = state
	}
	return states
}

// inofyPointerValue resolves a stored node result blob plus a JSON pointer
// (e.g. "/result") into the bound output value.
func inofyPointerValue(blob json.RawMessage, pointer string) (string, error) {
	var value any
	if err := json.Unmarshal(blob, &value); err != nil {
		return "", err
	}
	if pointer != "" {
		if !strings.HasPrefix(pointer, "/") {
			return "", errors.New("runtime: workflow output binding pointer is invalid")
		}
		for _, segment := range strings.Split(strings.TrimPrefix(pointer, "/"), "/") {
			object, ok := value.(map[string]any)
			if !ok {
				return "", errors.New("runtime: workflow output binding pointer misses")
			}
			value, ok = object[segment]
			if !ok {
				return "", errors.New("runtime: workflow output binding pointer misses")
			}
		}
	}
	switch v := value.(type) {
	case string:
		return v, nil
	case nil:
		return "", errors.New("runtime: workflow output binding resolves to null")
	default:
		raw, err := json.Marshal(v)
		if err != nil {
			return "", err
		}
		return string(raw), nil
	}
}

// decodeINOFYRevision decodes the committed canonical definition payload.
func decodeINOFYRevision(revision domain.WorkflowRevision) (inofy.Definition, error) {
	var definition inofy.Definition
	decoder := json.NewDecoder(bytes.NewReader(revision.DescriptorJSON))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&definition); err != nil {
		return inofy.Definition{}, fmt.Errorf("runtime: decode stored workflow revision: %w", err)
	}
	return definition, nil
}

// GetWorkflow returns the durable committed identity and node projection for
// one workflow Run, backed by the INOFY step projection plus journal events.
func (s *Service) GetWorkflow(ctx context.Context, runID domain.RunID) (WorkflowDetails, error) {
	if s == nil || s.deps.WorkflowRevisions == nil || s.deps.Runs == nil || s.deps.Journal == nil {
		return WorkflowDetails{}, errors.New("runtime: workflow inspection is not wired")
	}
	revision, err := s.deps.WorkflowRevisions.GetWorkflowRevision(ctx, runID)
	if err != nil {
		return WorkflowDetails{}, err
	}
	if revision.SchemaVersion != 2 {
		return WorkflowDetails{}, ErrWorkflowLegacyFormat
	}
	run, err := s.deps.Runs.GetRun(ctx, runID)
	if err != nil {
		return WorkflowDetails{}, err
	}
	definition, err := decodeINOFYRevision(revision)
	if err != nil {
		return WorkflowDetails{}, err
	}
	engine, err := s.inofyEngine()
	if err != nil {
		return WorkflowDetails{}, err
	}
	state, err := engine.LoadWorkflowStep(ctx, runID)
	if err != nil {
		return WorkflowDetails{}, fmt.Errorf("runtime: load workflow projection: %w", err)
	}
	details := WorkflowDetails{
		Run: run, RevisionDigest: revision.DescriptorDigest,
		Definition: append(json.RawMessage(nil), revision.DescriptorJSON...),
	}
	if state.Projection != nil {
		details.EngineStatus = string(state.Projection.Status)
	} else if run.Status.Terminal() {
		details.EngineStatus = string(run.Status)
	} else {
		details.EngineStatus = string(storage.WorkflowStepAdmitted)
	}
	iterator, err := s.deps.Journal.Replay(ctx, runID, 0)
	if err != nil {
		return WorkflowDetails{}, err
	}
	var events []domain.RunEvent
	for iterator.Next() {
		events = append(events, iterator.Value().Event)
	}
	if closeErr := iterator.Close(); closeErr != nil {
		return WorkflowDetails{}, closeErr
	}
	if err := iterator.Err(); err != nil {
		return WorkflowDetails{}, err
	}
	// Committed node identities are INOFY logical paths (/graph/nodes/<id>);
	// event payloads, unresolved ops, result rows and the derived child run
	// id all address that path, never the bare graph id.
	nodeKeys := make([]string, 0, len(definition.Graph.Nodes))
	nodePaths := make(map[string]string, len(definition.Graph.Nodes))
	for _, node := range definition.Graph.Nodes {
		nodeKeys = append(nodeKeys, node.ID)
		nodePaths[node.ID] = "/graph/nodes/" + node.ID
	}
	states := inofyNodeStates(events, runID, nodePaths)
	unresolved := map[string]bool{}
	if state.Projection != nil && len(state.Projection.UnresolvedJSON) > 0 {
		var ops []struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal(state.Projection.UnresolvedJSON, &ops); err == nil {
			for _, op := range ops {
				unresolved[op.Path] = true
			}
		}
	}
	for _, key := range nodeKeys {
		path := nodePaths[key]
		nodeState := states[key]
		status := "waiting"
		if nodeState.started {
			status = "running"
		}
		if nodeState.waiting && !nodeState.completed && !nodeState.failed {
			status = "waiting"
		}
		if nodeState.completed {
			status = "completed"
		}
		if nodeState.failed {
			status = "failed"
			if nodeState.errorCategory == causeCancelled {
				status = "cancelled"
			}
		}
		if unresolved[path] && !nodeState.completed && !nodeState.failed {
			status = "blocked"
		}
		if !nodeState.started && run.Status == domain.RunCancelled {
			status = "cancelled"
		}
		if !nodeState.started && run.Status == domain.RunFailed {
			status = "blocked"
		}
		childRunID := ""
		if nodeState.started {
			childRunID = string(inofyChildRunID(string(runID) + "/" + path))
		}
		details.Nodes = append(details.Nodes, WorkflowNodeProjection{
			Key: key, Status: status, ChildRunID: childRunID, ResultDigest: nodeState.resultDigest,
			ErrorCategory: nodeState.errorCategory, Message: nodeState.message,
		})
	}
	if run.Status != domain.RunCompleted {
		return details, nil
	}
	details.Outputs = make(map[string]string, len(definition.Graph.Outputs))
	latestResult := make(map[string]storage.WorkflowStepResult, len(state.Results))
	for _, result := range state.Results {
		current, ok := latestResult[result.Path]
		if !ok || result.Attempt > current.Attempt {
			latestResult[result.Path] = result
		}
	}
	for key, binding := range definition.Graph.Outputs {
		if len(binding.Literal) > 0 {
			var literal any
			if err := json.Unmarshal(binding.Literal, &literal); err != nil {
				return WorkflowDetails{}, fmt.Errorf("runtime: decode workflow literal output %q: %w", key, err)
			}
			if text, ok := literal.(string); ok {
				details.Outputs[key] = text
			} else {
				details.Outputs[key] = string(binding.Literal)
			}
			continue
		}
		result, ok := latestResult["/graph/nodes/"+binding.Source]
		if !ok || result.BlobID == "" {
			return WorkflowDetails{}, ErrWorkflowNodeUnknownOutcome
		}
		blob, found, err := engine.Blobs().Get(ctx, result.BlobID)
		if err != nil || !found {
			return WorkflowDetails{}, fmt.Errorf("runtime: load workflow result blob: %w", err)
		}
		sum := sha256.Sum256(blob)
		if hex.EncodeToString(sum[:]) != result.Digest {
			return WorkflowDetails{}, storage.ErrWorkflowRevisionConflict
		}
		value, err := inofyPointerValue(json.RawMessage(blob), binding.Pointer)
		if err != nil {
			return WorkflowDetails{}, err
		}
		details.Outputs[key] = value
	}
	encoded, err := json.Marshal(details.Outputs)
	if err != nil {
		return WorkflowDetails{}, err
	}
	if len(encoded) > orchestration.MaxOutputBytes {
		return WorkflowDetails{}, fmt.Errorf("runtime: workflow outputs exceed %d bytes", orchestration.MaxOutputBytes)
	}
	return details, nil
}

func (s *Service) ListWorkflows(ctx context.Context, parentRunID domain.RunID) ([]WorkflowDetails, error) {
	if s == nil || s.deps.WorkflowRevisions == nil {
		return nil, errors.New("runtime: workflow listing is not wired")
	}
	revisions, err := s.deps.WorkflowRevisions.ListWorkflowRevisions(ctx, parentRunID)
	if err != nil {
		return nil, err
	}
	if len(revisions) > 100 {
		revisions = revisions[len(revisions)-100:]
	}
	result := make([]WorkflowDetails, 0, len(revisions))
	for _, revision := range revisions {
		if revision.SchemaVersion != 2 {
			continue
		}
		details, err := s.GetWorkflow(ctx, revision.RunID)
		if err != nil {
			return nil, err
		}
		result = append(result, details)
	}
	return result, nil
}

func (s *Service) CancelWorkflow(ctx context.Context, runID domain.RunID) (domain.Run, error) {
	if s == nil || s.deps.WorkflowRevisions == nil || s.deps.Runs == nil {
		return domain.Run{}, errors.New("runtime: workflow cancellation is not wired")
	}
	if _, err := s.deps.WorkflowRevisions.GetWorkflowRevision(ctx, runID); err != nil {
		return domain.Run{}, err
	}
	if !s.Cancel(runID) {
		run, err := s.deps.Runs.GetRun(ctx, runID)
		if err != nil {
			return domain.Run{}, err
		}
		if !run.Status.Terminal() {
			return domain.Run{}, ErrWorkflowRecoveryRequired
		}
		return run, nil
	}
	return s.deps.Runs.GetRun(ctx, runID)
}

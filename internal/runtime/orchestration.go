package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/tools"
)

var ErrNativeOrchestrationApprovalPending = errors.New("runtime: native orchestration approval is pending")

const nativeOrchestrationResumeTargetPrefix = "native-orchestration:"

type nativeOrchestrationRequest struct {
	RunID domain.RunID
	// Plain bytes round-trip through Eino native slice serialization.
	DescriptorJSON   []byte
	NodeKey          string
	Task             string
	RequiresApproval bool
}

type nativeOrchestrationBinding struct {
	WorkflowRunID    string          `json:"workflow_run_id"`
	Descriptor       json.RawMessage `json:"descriptor"`
	NodeKey          string          `json:"node_key"`
	Task             string          `json:"task"`
	RequiresApproval bool            `json:"requires_approval,omitempty"`
}

func nativeOrchestrationArguments(request nativeOrchestrationRequest) ([]byte, error) {
	return json.Marshal(nativeOrchestrationBinding{string(request.RunID), request.DescriptorJSON, request.NodeKey, request.Task, request.RequiresApproval})
}

type nativeOrchestrationResult struct {
	Output string
}

type orchestrationProofInput struct {
	A nativeOrchestrationRequest
	B nativeOrchestrationRequest
}

type orchestrationProofJoinInput struct {
	A nativeOrchestrationResult
	B nativeOrchestrationResult
}

type orchestrationProofOutput struct {
	Result string
	Audit  string
}

type orchestrationNodeDescriptor struct {
	Key              string `json:"key"`
	Task             string `json:"task,omitempty"`
	RequiresApproval bool   `json:"requires_approval,omitempty"`
}

type orchestrationEdgeDescriptor struct {
	From string `json:"from"`
	To   string `json:"to"`
	Kind string `json:"kind"`
}

type orchestrationDescriptor struct {
	SchemaVersion int                           `json:"schema_version"`
	Nodes         []orchestrationNodeDescriptor `json:"nodes"`
	Edges         []orchestrationEdgeDescriptor `json:"edges"`
	Outputs       []string                      `json:"outputs"`
}

func init() {
	schema.RegisterName[nativeOrchestrationRequest]("vivy_native_orchestration_request_v1")
	schema.RegisterName[nativeOrchestrationResult]("vivy_native_orchestration_result_v1")
	schema.RegisterName[orchestrationProofInput]("vivy_orchestration_proof_input_v1")
	schema.RegisterName[orchestrationProofJoinInput]("vivy_orchestration_proof_join_input_v1")
	schema.RegisterName[orchestrationProofOutput]("vivy_orchestration_proof_output_v1")
}

func nativeOrchestrationDescriptor(taskA, taskB string) ([]byte, error) {
	return nativeOrchestrationDescriptorWithApproval(taskA, taskB, false)
}

func nativeOrchestrationDescriptorWithApproval(taskA, taskB string, requiresApproval bool) ([]byte, error) {
	if strings.TrimSpace(taskA) == "" || strings.TrimSpace(taskB) == "" {
		return nil, errors.New("runtime: native orchestration tasks must not be empty")
	}
	descriptor := orchestrationDescriptor{
		SchemaVersion: 2,
		Nodes: []orchestrationNodeDescriptor{
			{Key: "a", Task: taskA, RequiresApproval: requiresApproval}, {Key: "b", Task: taskB}, {Key: "join"}, {Key: "audit"},
		},
		Edges: []orchestrationEdgeDescriptor{
			{From: "start", To: "a", Kind: "input"},
			{From: "start", To: "b", Kind: "input"},
			{From: "a", To: "join", Kind: "input"},
			{From: "b", To: "join", Kind: "input"},
			{From: "start", To: "audit", Kind: "input"},
			{From: "join", To: "audit", Kind: "dependency"},
		},
		Outputs: []string{"join", "audit"},
	}
	encoded, err := json.Marshal(descriptor)
	if err != nil {
		return nil, fmt.Errorf("runtime: marshal native orchestration descriptor: %w", err)
	}
	return canonicalToolArguments(encoded)
}

type nativeOrchestrationNodeRunner func(context.Context, nativeOrchestrationRequest) (nativeOrchestrationResult, error)

// executeNativeOrchestrationWorkflow is the Service-owned Eino Workflow
// boundary for the bounded A,B -> join proof. Eino owns scheduling; each node
// enters the same durable operation boundary as ordinary brokered calls.
func (s *Service) executeNativeOrchestrationWorkflow(ctx context.Context, input orchestrationProofInput) (orchestrationProofOutput, error) {
	return s.executeNativeOrchestrationWorkflowWith(ctx, input, nil)
}

// executeNativeOrchestrationWorkflowWith keeps compile hooks private to the
// runtime package so conformance tests can place real Eino interrupts at
// stable node boundaries without introducing a public workflow option.
func (s *Service) executeNativeOrchestrationWorkflowWith(ctx context.Context, input orchestrationProofInput, runner nativeOrchestrationNodeRunner, options ...compose.GraphCompileOption) (orchestrationProofOutput, error) {
	coordinator, ok := toolOperationCoordinatorFromContext(ctx).(serviceToolOperationCoordinator)
	if !ok || coordinator.service != s {
		return orchestrationProofOutput{}, ErrToolOperationUnavailable
	}
	prompt, hasPrompt := runPrompt(ctx)
	if !hasPrompt || prompt.RunID != coordinator.runID {
		return orchestrationProofOutput{}, ErrCheckpointPromptMismatch
	}
	if len(input.A.DescriptorJSON) == 0 || len(input.B.DescriptorJSON) == 0 {
		return orchestrationProofOutput{}, fmt.Errorf("%w: native workflow input has no original descriptor", ErrWorkflowRecoveryRequired)
	}
	if input.A.RunID != coordinator.runID || input.B.RunID != coordinator.runID ||
		!bytes.Equal(input.A.DescriptorJSON, input.B.DescriptorJSON) ||
		input.A.NodeKey != "a" || input.B.NodeKey != "b" ||
		strings.TrimSpace(input.A.Task) == "" || strings.TrimSpace(input.B.Task) == "" {
		return orchestrationProofOutput{}, errors.New("runtime: invalid native orchestration workflow descriptor")
	}
	if input.B.RequiresApproval {
		return orchestrationProofOutput{}, errors.New("runtime: approval is only supported for workflow node a in the bounded proof")
	}
	admitted, err := decodeNativeOrchestrationInput(coordinator.runID, input.A.DescriptorJSON)
	if err != nil {
		return orchestrationProofOutput{}, err
	}
	if input.A.Task != admitted.A.Task || input.B.Task != admitted.B.Task || input.A.RequiresApproval != admitted.A.RequiresApproval {
		return orchestrationProofOutput{}, storage.ErrToolOperationConflict
	}
	descriptor := input.A.DescriptorJSON
	if err := coordinator.ensureRunCanExecute(ctx); err != nil {
		return orchestrationProofOutput{}, err
	}
	if err := s.validateNativeOrchestrationRun(ctx, coordinator, descriptor); err != nil {
		return orchestrationProofOutput{}, err
	}
	if s.engine == nil || s.engine.cfg.Checkpoints == nil {
		return orchestrationProofOutput{}, errors.New("runtime: native orchestration requires durable checkpoints")
	}
	if runner == nil {
		runner = s.runNativeOrchestration
	}
	workflow := compose.NewWorkflow[orchestrationProofInput, orchestrationProofOutput]()
	workflow.AddLambdaNode("a", compose.InvokableLambda(func(ctx context.Context, request nativeOrchestrationRequest) (nativeOrchestrationResult, error) {
		return runner(ctx, request)
	})).AddInput(compose.START, compose.FromField("A"))
	workflow.AddLambdaNode("b", compose.InvokableLambda(func(ctx context.Context, request nativeOrchestrationRequest) (nativeOrchestrationResult, error) {
		return runner(ctx, request)
	})).AddInput(compose.START, compose.FromField("B"))
	workflow.AddLambdaNode("join", compose.InvokableLambda(func(_ context.Context, input orchestrationProofJoinInput) (string, error) {
		return input.A.Output + "+" + input.B.Output, nil
	})).
		AddInput("a", compose.ToField("A")).
		AddInput("b", compose.ToField("B"))
	workflow.AddLambdaNode("audit", compose.InvokableLambda(func(_ context.Context, request nativeOrchestrationRequest) (string, error) {
		return request.NodeKey + ":after-join", nil
	})).AddInput(compose.START, compose.FromField("A")).AddDependency("join")
	workflow.End().
		AddInput("join", compose.ToField("Result")).
		AddInput("audit", compose.ToField("Audit"))

	compileOptions := []compose.GraphCompileOption{
		compose.WithGraphName("vivy-issue39-native-proof"),
		compose.WithCheckPointStore(NewEinoCheckpointAdapter(s.engine.cfg.Checkpoints)),
	}
	compileOptions = append(compileOptions, options...)
	runnable, err := workflow.Compile(ctx, compileOptions...)
	if err != nil {
		return orchestrationProofOutput{}, fmt.Errorf("runtime: compile native orchestration workflow: %w", err)
	}
	output, err := runnable.Invoke(ctx, input, compose.WithCheckPointID(checkpointIDFor(coordinator.runID)))
	if err != nil {
		if info, interrupted := compose.ExtractInterruptInfo(err); interrupted {
			handled, approvalErr := s.persistNativeOrchestrationApproval(ctx, coordinator, input, info)
			if approvalErr != nil {
				return orchestrationProofOutput{}, approvalErr
			}
			if handled {
				return orchestrationProofOutput{}, ErrNativeOrchestrationApprovalPending
			}
		}
		return orchestrationProofOutput{}, err
	}
	return output, nil
}

func (s *Service) validateNativeOrchestrationRun(ctx context.Context, coordinator serviceToolOperationCoordinator, descriptor []byte) error {
	if s.deps.Messages == nil {
		return errors.New("runtime: workflow message store is unavailable")
	}
	messages, err := s.deps.Messages.ListMessages(ctx, coordinator.sessionID)
	if err != nil {
		return fmt.Errorf("runtime: load workflow descriptor: %w", err)
	}
	for _, message := range messages {
		if message.RunID != coordinator.runID || message.Source != "workflow" || message.Role != domain.RoleUser {
			continue
		}
		if _, err := decodeNativeOrchestrationInput(coordinator.runID, []byte(message.Content)); err != nil {
			return err
		}
		if string(descriptor) != message.Content {
			return storage.ErrToolOperationConflict
		}
		return nil
	}
	return errors.New("runtime: workflow descriptor is not bound to its Run")
}

func (s *Service) loadNativeOrchestrationInput(ctx context.Context, coordinator serviceToolOperationCoordinator) (orchestrationProofInput, []byte, error) {
	if s.deps.Messages == nil {
		return orchestrationProofInput{}, nil, errors.New("runtime: workflow message store is unavailable")
	}
	messages, err := s.deps.Messages.ListMessages(ctx, coordinator.sessionID)
	if err != nil {
		return orchestrationProofInput{}, nil, fmt.Errorf("runtime: load workflow descriptor: %w", err)
	}
	for _, message := range messages {
		if message.RunID != coordinator.runID || message.Source != "workflow" || message.Role != domain.RoleUser {
			continue
		}
		raw := []byte(message.Content)
		input, err := decodeNativeOrchestrationInput(coordinator.runID, raw)
		if err != nil {
			return orchestrationProofInput{}, nil, err
		}
		return input, raw, nil
	}
	return orchestrationProofInput{}, nil, errors.New("runtime: workflow descriptor is not bound to its Run")
}

func (s *Service) persistNativeOrchestrationApproval(ctx context.Context, coordinator serviceToolOperationCoordinator, input orchestrationProofInput, info *compose.InterruptInfo) (bool, error) {
	if info == nil {
		return false, errors.New("runtime: Eino returned an empty orchestration interrupt")
	}
	for _, interrupt := range info.InterruptContexts {
		if interrupt == nil || !interrupt.IsRootCause {
			continue
		}
		toolName, args, intendedArguments, message, ok := decodeToolApprovalInterrupt(interrupt.Info)
		if !ok || toolName != "orchestration.node" {
			continue
		}
		var binding nativeOrchestrationBinding
		encoded, err := json.Marshal(args)
		if err != nil || json.Unmarshal(encoded, &binding) != nil {
			return false, errors.New("runtime: orchestration approval payload is invalid")
		}
		if binding.WorkflowRunID != string(coordinator.runID) || !bytes.Equal(binding.Descriptor, input.A.DescriptorJSON) ||
			binding.NodeKey != "a" || binding.Task != input.A.Task || !binding.RequiresApproval || !input.A.RequiresApproval ||
			interrupt.ID == "" {
			return false, errors.New("runtime: orchestration approval does not match the active workflow")
		}
		mapper := s.newResumeEventMapper(ctx, coordinator.runID)
		mapper.interrupt = &interruptDetails{
			ResumeTarget:      nativeOrchestrationResumeTarget(interrupt.ID),
			ToolCallID:        workflowNodeOperationID("a"),
			ToolName:          toolName,
			Args:              args,
			IntendedArguments: intendedArguments,
			Message:           message,
		}
		if !s.handleInterrupt(ctx, mapper, coordinator.sessionID, nil, runMode(ctx)) {
			return false, errors.New("runtime: Service could not persist the workflow approval suspension")
		}
		return true, nil
	}
	return false, nil
}

func (s *Service) runNativeOrchestration(ctx context.Context, request nativeOrchestrationRequest) (nativeOrchestrationResult, error) {
	if wasInterrupted, hasState, encodedState := einotool.GetInterruptState[string](ctx); wasInterrupted {
		if !hasState {
			return nativeOrchestrationResult{}, errors.New("runtime: workflow approval checkpoint state is unavailable")
		}
		restored, err := nativeOrchestrationRequestFromApprovalState(encodedState)
		if err != nil {
			return nativeOrchestrationResult{}, err
		}
		request = restored
	}
	if len(request.DescriptorJSON) == 0 {
		return nativeOrchestrationResult{}, fmt.Errorf("%w: native workflow checkpoint has no original descriptor", ErrWorkflowRecoveryRequired)
	}
	if request.RunID == "" || !validWorkflowNodeKey(request.NodeKey) || strings.TrimSpace(request.Task) == "" {
		return nativeOrchestrationResult{}, fmt.Errorf("runtime: invalid orchestration node identity or task (run_id=%t descriptor_present=%t node=%q task_empty=%t)",
			request.RunID != "", len(request.DescriptorJSON) > 0, request.NodeKey, strings.TrimSpace(request.Task) == "")
	}
	coordinator := toolOperationCoordinatorFromContext(ctx)
	serviceCoordinator, ok := coordinator.(serviceToolOperationCoordinator)
	if !ok || serviceCoordinator.service != s {
		return nativeOrchestrationResult{}, ErrToolOperationUnavailable
	}
	if serviceCoordinator.runID != request.RunID {
		return nativeOrchestrationResult{}, errors.New("runtime: orchestration node does not belong to the active workflow Run")
	}
	if s.deps.ToolOperations == nil {
		return nativeOrchestrationResult{}, ErrToolOperationUnavailable
	}
	admitted, err := decodeNativeOrchestrationInput(request.RunID, request.DescriptorJSON)
	if err != nil {
		return nativeOrchestrationResult{}, err
	}
	expected := admitted.A
	if request.NodeKey == "b" {
		expected = admitted.B
	} else if request.NodeKey != "a" {
		return nativeOrchestrationResult{}, storage.ErrToolOperationConflict
	}
	if request.Task != expected.Task || request.RequiresApproval != expected.RequiresApproval {
		return nativeOrchestrationResult{}, storage.ErrToolOperationConflict
	}
	if err := s.validateNativeOrchestrationRun(ctx, serviceCoordinator, request.DescriptorJSON); err != nil {
		return nativeOrchestrationResult{}, err
	}
	operationID := workflowNodeOperationID(request.NodeKey)
	requestBytes, err := nativeOrchestrationArguments(request)
	if err != nil {
		return nativeOrchestrationResult{}, fmt.Errorf("runtime: marshal orchestration node binding: %w", err)
	}
	operation, found, err := coordinator.Lookup(ctx, operationID, "orchestration.node", requestBytes)
	if err != nil {
		return nativeOrchestrationResult{}, err
	}
	if !found {
		operation, err = coordinator.Admit(ctx, operationID, "orchestration.node", requestBytes, requestBytes, requestBytes)
		if err != nil {
			return nativeOrchestrationResult{}, err
		}
	}
	if request.RequiresApproval {
		wasInterrupted, hasState, encodedState := einotool.GetInterruptState[string](ctx)
		if !wasInterrupted {
			return nativeOrchestrationResult{}, interruptForToolApproval(ctx, "orchestration.node", requestBytes, "This workflow node requires approval before it can run.")
		}
		if !hasState {
			return nativeOrchestrationResult{}, errors.New("runtime: workflow approval checkpoint state is unavailable")
		}
		stateToolName, _, stateArguments, _, valid := decodeToolApprovalInterrupt(encodedState)
		currentArguments, canonicalErr := canonicalToolArguments(requestBytes)
		if !valid || stateToolName != "orchestration.node" || canonicalErr != nil || !bytes.Equal(stateArguments, currentArguments) {
			return nativeOrchestrationResult{}, errors.New("runtime: workflow approval checkpoint does not match the node")
		}
		isTarget, hasData, decision := einotool.GetResumeContext[string](ctx)
		if !isTarget || !hasData {
			if len(approvedToolArguments(ctx)) > 0 && approvedToolCall(ctx) == operationID {
				return nativeOrchestrationResult{}, errors.New("runtime: workflow approval resume target does not match its node")
			}
			return nativeOrchestrationResult{}, einotool.StatefulInterrupt(ctx, encodedState, encodedState)
		}
		if approvedToolCall(ctx) != operationID {
			return nativeOrchestrationResult{}, errors.New("runtime: workflow approval belongs to another tool call")
		}
		switch decision {
		case domain.ApprovalApproved:
			if err := validateResumedToolApproval(ctx, "orchestration.node", requestBytes); err != nil {
				return nativeOrchestrationResult{}, err
			}
		case domain.ApprovalDenied:
			if err := validateResumedToolApproval(ctx, "orchestration.node", requestBytes); err != nil {
				return nativeOrchestrationResult{}, err
			}
			output, err := coordinator.Execute(ctx, operation, func(context.Context) (domain.ToolOperationOutcome, error) {
				return outcomeWithFailure("denied by the reviewer; node did not run", refusalFailure(toolFailureReasonUserDenied, "denied by the reviewer")), nil
			})
			if err != nil {
				return nativeOrchestrationResult{}, err
			}
			return nativeOrchestrationResult{Output: output}, nil
		default:
			return nativeOrchestrationResult{}, fmt.Errorf("runtime: invalid workflow approval decision %q", decision)
		}
	}
	output, err := coordinator.Execute(ctx, operation, func(context.Context) (domain.ToolOperationOutcome, error) {
		return successToolOutcome(request.Task), nil
	})
	if err != nil {
		return nativeOrchestrationResult{}, err
	}
	return nativeOrchestrationResult{Output: output}, nil
}

func nativeOrchestrationRequestFromApprovalState(encodedState string) (nativeOrchestrationRequest, error) {
	toolName, args, intendedArguments, _, ok := decodeToolApprovalInterrupt(encodedState)
	if !ok || toolName != "orchestration.node" || len(intendedArguments) == 0 {
		return nativeOrchestrationRequest{}, errors.New("runtime: workflow approval checkpoint state is invalid")
	}
	requestBytes, err := json.Marshal(args)
	if err != nil {
		return nativeOrchestrationRequest{}, fmt.Errorf("runtime: encode workflow approval state: %w", err)
	}
	verifiedArguments, err := canonicalToolArguments(requestBytes)
	if err != nil || !bytes.Equal(verifiedArguments, intendedArguments) {
		return nativeOrchestrationRequest{}, errors.New("runtime: workflow approval checkpoint state failed its integrity check")
	}
	var binding nativeOrchestrationBinding
	if err := json.Unmarshal(requestBytes, &binding); err != nil {
		return nativeOrchestrationRequest{}, fmt.Errorf("runtime: decode workflow approval state: %w", err)
	}
	request := nativeOrchestrationRequest{
		RunID: domain.RunID(binding.WorkflowRunID), DescriptorJSON: binding.Descriptor,
		NodeKey: binding.NodeKey, Task: binding.Task, RequiresApproval: binding.RequiresApproval,
	}
	if len(request.DescriptorJSON) == 0 {
		return nativeOrchestrationRequest{}, fmt.Errorf("%w: native workflow approval has no original descriptor", ErrWorkflowRecoveryRequired)
	}
	if request.RunID == "" || request.NodeKey != "a" ||
		strings.TrimSpace(request.Task) == "" || !request.RequiresApproval {
		return nativeOrchestrationRequest{}, errors.New("runtime: workflow approval checkpoint does not contain an approvable node")
	}
	admitted, err := decodeNativeOrchestrationInput(request.RunID, request.DescriptorJSON)
	if err != nil {
		return nativeOrchestrationRequest{}, err
	}
	if request.Task != admitted.A.Task || request.RequiresApproval != admitted.A.RequiresApproval {
		return nativeOrchestrationRequest{}, storage.ErrToolOperationConflict
	}
	return request, nil
}

func validateNativeOrchestrationApprovalBinding(approval domain.Approval, input orchestrationProofInput) error {
	if approval.RunID != input.A.RunID || approval.ToolName != "orchestration.node" ||
		approval.ToolCallID != workflowNodeOperationID("a") ||
		!isNativeOrchestrationResumeTarget(approval.ResumeTarget) ||
		strings.TrimPrefix(approval.ResumeTarget, nativeOrchestrationResumeTargetPrefix) == "" {
		return errors.New("runtime: workflow approval identity does not match its node")
	}
	_, approvedArguments, err := unbindToolApprovalProposal(approval.ProposalData)
	if err != nil {
		return fmt.Errorf("runtime: workflow approval proposal is invalid: %w", err)
	}
	arguments, err := nativeOrchestrationArguments(input.A)
	if err != nil {
		return fmt.Errorf("runtime: encode workflow approval binding: %w", err)
	}
	expectedArguments, err := canonicalToolArguments(arguments)
	if err != nil {
		return fmt.Errorf("runtime: canonicalize workflow approval binding: %w", err)
	}
	if len(approvedArguments) == 0 || !bytes.Equal(approvedArguments, expectedArguments) {
		return errors.New("runtime: workflow approval arguments do not match the admitted node")
	}
	return nil
}

func nativeOrchestrationResumeTarget(interruptID string) string {
	return nativeOrchestrationResumeTargetPrefix + interruptID
}

func isNativeOrchestrationResumeTarget(target string) bool {
	return strings.HasPrefix(target, nativeOrchestrationResumeTargetPrefix)
}

func (s *Service) resumeNativeOrchestrationApproval(p pendingRun, approval domain.Approval, decision string) {
	if approval.ResumeTarget == nativeOrchestrationResumeTargetPrefix {
		m := s.newResumeEventMapper(context.Background(), approval.RunID)
		s.emitTerminal(context.Background(), m, m.build(domain.EventRunFailed, payloadRunFailed{
			CauseCategory: causeInternalError,
			Message:       "The workflow approval could not be resumed; its checkpoint did not identify a node.",
		}))
		return
	}
	s.mu.Lock()
	if s.runTools[approval.RunID] == nil {
		selected := make(map[string]struct{}, len(p.selectedTools))
		for _, name := range p.selectedTools {
			selected[name] = struct{}{}
		}
		s.runTools[approval.RunID] = selected
	}
	s.mu.Unlock()
	if err := s.applyPendingEngineReload(context.Background(), nil); err != nil {
		slog.Warn("pending engine reload failed before workflow resume", "run", string(approval.RunID), "err", err)
	}
	baseCtx := context.Background()
	m := s.newResumeEventMapper(baseCtx, approval.RunID)
	ctx := withWorkspaceID(withSessionID(withRunID(withPolicySnapshot(withPolicyProfile(withRunMode(withFace(withSelectedTools(baseCtx, p.selectedTools), p.face), p.mode), p.profile), p.snapshot), approval.RunID), p.sessionID), p.workspaceID)
	ctx = withToolOperationCoordinator(ctx, s.newToolOperationCoordinator(approval.RunID, p.sessionID))
	ctx = withSessionSandbox(ctx, p.sandboxMode, p.approvalPolicy)
	ctx = tools.WithSessionID(ctx, p.sessionID)
	providerLabel, modelLabel := s.CurrentModel()
	ctx = domain.WithRunLabels(ctx, domain.RunLabels{Provider: providerLabel, Model: modelLabel})
	mounted := p.mounted
	if mounted == nil {
		mounted = tools.NewMountedTools()
	}
	ctx = tools.WithMountedTools(ctx, mounted)
	ctx = tools.WithToolActivation(ctx, s.sessionToolActivation(ctx, p.sessionID))
	promptCtx, hasPrompt, promptErr := s.promptSnapshotContext(ctx, approval.RunID)
	if promptErr != nil || !hasPrompt {
		if promptErr == nil {
			promptErr = ErrCheckpointPromptMismatch
		}
		s.emitTerminal(ctx, m, s.terminalEvent(ctx, m, promptErr))
		return
	}
	ctx = promptCtx
	coordinator := serviceToolOperationCoordinator{service: s, runID: approval.RunID, sessionID: p.sessionID}
	input, descriptor, err := s.loadNativeOrchestrationInput(ctx, coordinator)
	if err == nil {
		err = s.validateNativeOrchestrationRun(ctx, coordinator, descriptor)
	}
	if err != nil {
		s.emitTerminal(ctx, m, s.terminalEvent(ctx, m, err))
		return
	}
	if err := validateNativeOrchestrationApprovalBinding(approval, input); err != nil {
		s.emitTerminal(ctx, m, s.terminalEvent(ctx, m, err))
		return
	}
	_, intended, bindingErr := unbindToolApprovalProposal(approval.ProposalData)
	if bindingErr != nil {
		s.emitTerminal(ctx, m, s.terminalEvent(ctx, m, bindingErr))
		return
	}
	ctx = withApprovedToolArguments(ctx, intended)
	ctx = withApprovedToolCall(ctx, approval.ToolCallID)
	interruptID := strings.TrimPrefix(approval.ResumeTarget, nativeOrchestrationResumeTargetPrefix)
	ctx = compose.ResumeWithData(ctx, interruptID, decision)
	output, err := s.executeNativeOrchestrationWorkflow(ctx, input)
	if errors.Is(err, ErrNativeOrchestrationApprovalPending) {
		return
	}
	if err != nil {
		s.emitTerminal(ctx, m, s.terminalEvent(ctx, m, err))
		return
	}
	content, err := json.Marshal(output)
	if err == nil && s.deps.Messages == nil {
		err = errors.New("runtime: workflow message store is unavailable")
	}
	if err == nil {
		err = s.deps.Messages.AppendMessage(ctx, domain.Message{
			ID: newMessageID(), SessionID: p.sessionID, RunID: approval.RunID,
			Role: domain.RoleAssistant, CreatedAt: time.Now().UnixMilli(),
			Content: string(content), Source: "workflow",
		})
	}
	if err != nil {
		s.emitTerminal(ctx, m, s.terminalEvent(ctx, m, err))
		return
	}
	s.emitTerminal(ctx, m, m.build(domain.EventRunCompleted, payloadRunCompleted{}))
}

// SQL tool-operation ownership already scopes this node key to one Run. Exact
// original arguments, including the descriptor, decide replay versus conflict.
func workflowNodeOperationID(nodeKey string) string { return "workflow-node-" + nodeKey }

func validWorkflowNodeKey(key string) bool {
	if key == "" || len(key) > 64 {
		return false
	}
	for _, char := range key {
		if !(char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || char == '-' || char == '_') {
			return false
		}
	}
	return true
}

// decodeNativeOrchestrationInput validates the complete canonical proof graph.
// Its authority remains the original workflow message committed with the Run.
func decodeNativeOrchestrationInput(runID domain.RunID, raw []byte) (orchestrationProofInput, error) {
	if len(raw) == 0 {
		return orchestrationProofInput{}, fmt.Errorf("%w: native workflow checkpoint has no original descriptor", ErrWorkflowRecoveryRequired)
	}
	if len(raw) > 64<<10 {
		return orchestrationProofInput{}, errors.New("runtime: native orchestration descriptor exceeds its byte bound")
	}
	var descriptor orchestrationDescriptor
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&descriptor); err != nil {
		return orchestrationProofInput{}, fmt.Errorf("runtime: decode workflow descriptor: %w", err)
	}
	if descriptor.SchemaVersion != 2 {
		return orchestrationProofInput{}, fmt.Errorf("%w: native workflow descriptor requires the exact-input format", ErrWorkflowRecoveryRequired)
	}
	if len(descriptor.Nodes) != 4 {
		return orchestrationProofInput{}, errors.New("runtime: unsupported native orchestration descriptor")
	}
	var taskA, taskB string
	var requiresApproval bool
	for _, node := range descriptor.Nodes {
		switch node.Key {
		case "a":
			taskA, requiresApproval = node.Task, node.RequiresApproval
		case "b":
			taskB = node.Task
		case "join", "audit":
			if node.Task != "" || node.RequiresApproval {
				return orchestrationProofInput{}, storage.ErrToolOperationConflict
			}
		default:
			return orchestrationProofInput{}, storage.ErrToolOperationConflict
		}
	}
	canonical, err := nativeOrchestrationDescriptorWithApproval(taskA, taskB, requiresApproval)
	if err != nil {
		return orchestrationProofInput{}, err
	}
	if !bytes.Equal(canonical, raw) {
		return orchestrationProofInput{}, storage.ErrToolOperationConflict
	}
	return orchestrationProofInput{
		A: nativeOrchestrationRequest{RunID: runID, DescriptorJSON: append(json.RawMessage(nil), canonical...), NodeKey: "a", Task: taskA, RequiresApproval: requiresApproval},
		B: nativeOrchestrationRequest{RunID: runID, DescriptorJSON: append(json.RawMessage(nil), canonical...), NodeKey: "b", Task: taskB},
	}, nil
}

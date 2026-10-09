package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

type nativeOrchestrationRequest struct {
	RunID          domain.RunID
	RevisionDigest string
	NodeKey        string
	Task           string
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
	Key  string `json:"key"`
	Task string `json:"task,omitempty"`
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

func nativeOrchestrationDescriptor(taskA, taskB string) ([]byte, string, error) {
	if strings.TrimSpace(taskA) == "" || strings.TrimSpace(taskB) == "" {
		return nil, "", errors.New("runtime: native orchestration tasks must not be empty")
	}
	descriptor := orchestrationDescriptor{
		SchemaVersion: 1,
		Nodes: []orchestrationNodeDescriptor{
			{Key: "a", Task: taskA}, {Key: "b", Task: taskB}, {Key: "join"}, {Key: "audit"},
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
		return nil, "", fmt.Errorf("runtime: marshal native orchestration descriptor: %w", err)
	}
	return encoded, operationDigest(encoded), nil
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
	if input.A.RunID != coordinator.runID || input.B.RunID != coordinator.runID ||
		!validWorkflowRevisionDigest(input.A.RevisionDigest) || input.A.RevisionDigest != input.B.RevisionDigest ||
		input.A.NodeKey != "a" || input.B.NodeKey != "b" ||
		strings.TrimSpace(input.A.Task) == "" || strings.TrimSpace(input.B.Task) == "" {
		return orchestrationProofOutput{}, errors.New("runtime: invalid native orchestration workflow descriptor")
	}
	descriptor, revision, err := nativeOrchestrationDescriptor(input.A.Task, input.B.Task)
	if err != nil {
		return orchestrationProofOutput{}, err
	}
	if revision != input.A.RevisionDigest {
		return orchestrationProofOutput{}, storage.ErrToolOperationConflict
	}
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
		if string(descriptor) != message.Content {
			return storage.ErrToolOperationConflict
		}
		return nil
	}
	return errors.New("runtime: workflow descriptor is not bound to its Run")
}

func (s *Service) runNativeOrchestration(ctx context.Context, request nativeOrchestrationRequest) (nativeOrchestrationResult, error) {
	if request.RunID == "" || !validWorkflowRevisionDigest(request.RevisionDigest) || !validWorkflowNodeKey(request.NodeKey) || strings.TrimSpace(request.Task) == "" {
		return nativeOrchestrationResult{}, fmt.Errorf("runtime: invalid orchestration node identity or task (run_id=%t revision=%t node=%q task_empty=%t)",
			request.RunID != "", validWorkflowRevisionDigest(request.RevisionDigest), request.NodeKey, strings.TrimSpace(request.Task) == "")
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
	operationID := workflowNodeOperationID(request.RevisionDigest, request.NodeKey)
	requestPayload := struct {
		WorkflowRunID string `json:"workflow_run_id"`
		Revision      string `json:"revision_digest"`
		NodeKey       string `json:"node_key"`
		Task          string `json:"task"`
	}{
		WorkflowRunID: string(request.RunID), Revision: request.RevisionDigest,
		NodeKey: request.NodeKey, Task: request.Task,
	}
	requestBytes, err := json.Marshal(requestPayload)
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

	output, err := coordinator.Execute(ctx, operation, func(context.Context) (string, error) {
		return request.Task, nil
	})
	if err != nil {
		return nativeOrchestrationResult{}, err
	}
	return nativeOrchestrationResult{Output: output}, nil
}

func workflowNodeOperationID(revisionDigest, nodeKey string) string {
	return "workflow-node-" + operationDigest([]byte(revisionDigest+"\x00"+nodeKey))
}

func validWorkflowRevisionDigest(digest string) bool {
	if len(digest) != 64 {
		return false
	}
	for _, char := range digest {
		if !(char >= '0' && char <= '9' || char >= 'a' && char <= 'f') {
			return false
		}
	}
	return true
}

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

package tools

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/orchestration"
)

const (
	WorkflowName              = "workflow"
	maxWorkflowDescriptorSize = 64 << 10
	maxWorkflowToolResultSize = orchestration.MaxOutputBytes + 1024
)

// WorkflowTaskResult is the bounded result returned to the authoring agent.
type WorkflowTaskResult struct {
	WorkflowRunID string            `json:"workflow_run_id"`
	Outputs       map[string]string `json:"outputs"`
}

// WorkflowOperations connects the model-facing tool to the governed runtime.
// The operation key must be stable for retries of the same model tool call.
type WorkflowOperations interface {
	RunWorkflow(context.Context, domain.RunID, string, json.RawMessage) (WorkflowTaskResult, error)
	DefinitionSchema() json.RawMessage
}

type workflowTool struct{ ops WorkflowOperations }

func NewWorkflow(ops WorkflowOperations) Tool { return &workflowTool{ops: ops} }

func (t *workflowTool) Spec() domain.ToolSpec {
	return domain.ToolSpec{
		Name: WorkflowName,
		Description: "Create and run a bounded INOFY Definition of read-only child tasks. " +
			"Each node receives only its task and explicitly connected predecessor outputs in a fresh child context. " +
			"The host validates the graph and limits its size and tools. Use this for independent research or analysis steps, " +
			"declare dependencies and final outputs explicitly, keep tasks self-contained, and never include secrets. " +
			"Workflow nodes cannot create nested workflows or use write-capable tools.",
		Readonly: true,
		Keywords: []string{"workflow", "parallel", "orchestration", "tasks"},
		Schema:   t.ops.DefinitionSchema(),
	}
}

func (t *workflowTool) InvokableRun(ctx context.Context, args json.RawMessage) (string, error) {
	if t == nil || t.ops == nil {
		return "", errors.New("workflow runtime is not wired")
	}
	parentRunID := RunIDFromContext(ctx)
	callID := ToolCallIDFromContext(ctx)
	if parentRunID == "" || callID == "" {
		return "", errors.New("workflow tool requires a run-scoped model call")
	}
	if len(bytes.TrimSpace(args)) > maxWorkflowDescriptorSize {
		return "", fmt.Errorf("workflow definition exceeds %d bytes", maxWorkflowDescriptorSize)
	}
	if err := ValidateArgs(t.Spec(), args); err != nil {
		return "", err
	}
	callDigest := sha256.Sum256([]byte(callID))
	operationKey := "workflow-tool-" + hex.EncodeToString(callDigest[:])
	result, err := t.ops.RunWorkflow(ctx, parentRunID, operationKey, args)
	if err != nil {
		return "", err
	}
	if result.WorkflowRunID == "" {
		return "", errors.New("workflow runtime returned no workflow Run identity")
	}
	encodedOutputs, err := json.Marshal(result.Outputs)
	if err != nil {
		return "", fmt.Errorf("encode workflow outputs: %w", err)
	}
	if len(encodedOutputs) > orchestration.MaxOutputBytes {
		return "", fmt.Errorf("workflow outputs exceed %d bytes", orchestration.MaxOutputBytes)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("encode workflow result: %w", err)
	}
	if len(encoded) > maxWorkflowToolResultSize {
		return "", fmt.Errorf("workflow result exceeds %d bytes", maxWorkflowToolResultSize)
	}
	return string(encoded), nil
}

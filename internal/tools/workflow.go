package tools

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"

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
	RunWorkflow(context.Context, domain.RunID, string, orchestration.Descriptor) (WorkflowTaskResult, error)
}

type workflowTool struct{ ops WorkflowOperations }

func NewWorkflow(ops WorkflowOperations) Tool { return &workflowTool{ops: ops} }

func (t *workflowTool) Spec() domain.ToolSpec {
	return domain.ToolSpec{
		Name: WorkflowName,
		Description: "Create and run a bounded DAG of independent read-only tasks. " +
			"Each node receives only its task and explicitly connected predecessor outputs in a fresh child context. " +
			"The host validates the graph and limits its size and tools. Use this for independent research or analysis steps, " +
			"declare dependencies and final outputs explicitly, keep tasks self-contained, and never include secrets. " +
			"Workflow nodes cannot create nested workflows or use write-capable tools.",
		Readonly: true,
		Keywords: []string{"workflow", "dag", "parallel", "orchestration", "tasks"},
		Schema:   json.RawMessage(workflowDescriptorSchema),
	}
}

const workflowDescriptorSchema = `{
  "$schema":"https://json-schema.org/draft/2020-12/schema",
  "type":"object",
  "additionalProperties":false,
  "required":["schema_version","start_nodes","nodes","edges","outputs"],
  "properties":{
    "schema_version":{"type":"integer","const":1},
    "start_nodes":{"type":"array","minItems":1,"maxItems":12,"items":{"type":"string","minLength":1,"maxLength":48}},
    "nodes":{"type":"array","minItems":1,"maxItems":12,"items":{"type":"object","additionalProperties":false,"required":["key","task"],"properties":{"key":{"type":"string","minLength":1,"maxLength":48},"task":{"type":"string","minLength":1,"maxLength":4096},"tool_names":{"type":"array","maxItems":32,"items":{"type":"string","minLength":1,"maxLength":128}}}}},
    "edges":{"type":"array","maxItems":24,"items":{"type":"object","additionalProperties":false,"required":["from","to"],"properties":{"from":{"type":"string","minLength":1,"maxLength":48},"to":{"type":"string","minLength":1,"maxLength":48},"input_key":{"type":"string","maxLength":48}}}},
    "outputs":{"type":"array","minItems":1,"maxItems":4,"items":{"type":"string","minLength":1,"maxLength":48}}
  }
}`

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
		return "", fmt.Errorf("workflow descriptor exceeds %d bytes", maxWorkflowDescriptorSize)
	}
	if err := ValidateArgs(t.Spec(), args); err != nil {
		return "", err
	}
	decoder := json.NewDecoder(bytes.NewReader(args))
	decoder.DisallowUnknownFields()
	var descriptor orchestration.Descriptor
	if err := decoder.Decode(&descriptor); err != nil {
		return "", fmt.Errorf("decode workflow descriptor: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			err = errors.New("multiple JSON values")
		}
		return "", fmt.Errorf("decode workflow descriptor: %w", err)
	}
	callDigest := sha256.Sum256([]byte(callID))
	operationKey := "workflow-tool-" + hex.EncodeToString(callDigest[:])
	result, err := t.ops.RunWorkflow(ctx, parentRunID, operationKey, descriptor)
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

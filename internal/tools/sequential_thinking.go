package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"agent-vivy/internal/domain"
)

const SequentialThinkingName = "sequential_thinking"

type SequentialThoughtRequest struct {
	Thought           string
	ThoughtNumber     int
	TotalThoughts     int
	NextThoughtNeeded bool
	IsRevision        bool
	RevisesThought    int
	BranchFromThought int
	BranchID          string
}

type SequentialThoughtResponse struct {
	Thought           string `json:"thought"`
	ThoughtNumber     int    `json:"thought_number"`
	TotalThoughts     int    `json:"total_thoughts"`
	NextThoughtNeeded bool   `json:"next_thought_needed"`
	IsRevision        bool   `json:"is_revision,omitempty"`
	BranchID          string `json:"branch_id,omitempty"`
	Accepted          bool   `json:"accepted"`
}

type SequentialThinkingOperations interface {
	Think(context.Context, domain.RunID, SequentialThoughtRequest) (SequentialThoughtResponse, error)
}

type sequentialThinkingTool struct{ ops SequentialThinkingOperations }

func NewSequentialThinking(ops SequentialThinkingOperations) Tool {
	return &sequentialThinkingTool{ops: ops}
}
func (t *sequentialThinkingTool) Spec() domain.ToolSpec {
	return domain.ToolSpec{Name: SequentialThinkingName, Description: "Records a bounded reasoning step, revision, or branch for the current run; it performs no external side effect.", Readonly: true, Keywords: []string{"think", "reason", "plan", "step"}, Params: map[string]domain.ToolParam{
		"thought":             {Desc: "The current reasoning step.", Required: true},
		"thought_number":      {Desc: "One-based step number.", Type: "integer", Required: true},
		"total_thoughts":      {Desc: "Expected total steps, bounded by the runtime.", Type: "integer", Required: true},
		"next_thought_needed": {Desc: "Whether another thought is needed.", Type: "boolean", Required: true},
		"is_revision":         {Desc: "Whether this revises an earlier step.", Type: "boolean"},
		"revises_thought":     {Desc: "Earlier step number when revising.", Type: "integer"},
		"branch_from_thought": {Desc: "Earlier step number when creating a branch.", Type: "integer"},
		"branch_id":           {Desc: "Optional stable branch identifier."},
	}}
}
func (t *sequentialThinkingTool) InvokableRun(ctx context.Context, args json.RawMessage) (string, error) {
	var input struct {
		Thought           string `json:"thought"`
		ThoughtNumber     int    `json:"thought_number"`
		TotalThoughts     int    `json:"total_thoughts"`
		NextThoughtNeeded bool   `json:"next_thought_needed"`
		IsRevision        bool   `json:"is_revision"`
		RevisesThought    int    `json:"revises_thought"`
		BranchFromThought int    `json:"branch_from_thought"`
		BranchID          string `json:"branch_id"`
	}
	if err := json.Unmarshal(args, &input); err != nil {
		return "", fmt.Errorf("sequential_thinking: invalid arguments: %w", err)
	}
	if strings.TrimSpace(input.Thought) == "" {
		return "", &ArgError{Field: "thought", Reason: "is required"}
	}
	if t.ops == nil {
		return "", fmt.Errorf("tools: sequential thinking backend not wired")
	}
	result, err := t.ops.Think(ctx, RunIDFromContext(ctx), SequentialThoughtRequest{Thought: input.Thought, ThoughtNumber: input.ThoughtNumber, TotalThoughts: input.TotalThoughts, NextThoughtNeeded: input.NextThoughtNeeded, IsRevision: input.IsRevision, RevisesThought: input.RevisesThought, BranchFromThought: input.BranchFromThought, BranchID: input.BranchID})
	if err != nil {
		return "", err
	}
	return marshalToolResult(result)
}

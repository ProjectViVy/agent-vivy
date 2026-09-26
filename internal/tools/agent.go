package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"agent-vivy/internal/domain"
)

const AgentName = "agent"

// maxAgentTaskBytes bounds the delegation brief so a runaway model cannot
// push unbounded text into the child harness.
const maxAgentTaskBytes = 64 << 10

// AgentOperations is the sub-agent seam the agent tool is built over; the
// app layer implements it over the durable child-run machinery (parent-owned
// budget, parent-session approvals, read-only tool surface).
type AgentOperations interface {
	// StartAgentTask runs one bounded sub-agent to completion and returns its
	// final answer. The parent run id comes from the run-scoped context. The
	// child receives no parent persona or transcript.
	StartAgentTask(ctx context.Context, task string) (string, error)
}

type agentTool struct {
	ops AgentOperations
}

func NewAgent(ops AgentOperations) Tool { return &agentTool{ops: ops} }

func (t *agentTool) Spec() domain.ToolSpec {
	return domain.ToolSpec{
		Name: AgentName,
		Description: "Delegate a focused sub-task to a fresh-context sub-agent and return its final answer. " +
			"The sub-agent receives only the task and read-only tools (no parent history, persona, file writes, shell, or MCP), " +
			"sees nothing from this conversation, and cannot spawn further sub-agents. " +
			"Give it a complete, self-contained brief. Use it for research or exploration whose intermediate " +
			"steps would otherwise flood this conversation.",
		Readonly: true,
		Keywords: []string{"agent", "subagent", "delegate", "task"},
		Params: map[string]domain.ToolParam{
			"task": {Desc: "Complete, self-contained brief for the sub-agent; it sees nothing from this conversation.", Required: true},
		},
	}
}

func (t *agentTool) InvokableRun(ctx context.Context, args json.RawMessage) (string, error) {
	if t.ops == nil {
		return "", fmt.Errorf("sub-agent machinery is not wired")
	}
	var params struct {
		Task string `json:"task"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("agent arguments must be a JSON object: %w", err)
	}
	task := strings.TrimSpace(params.Task)
	if task == "" {
		return "", fmt.Errorf("tool argument %q: must not be empty", "task")
	}
	if len(task) > maxAgentTaskBytes {
		return "", fmt.Errorf("tool argument %q: exceeds the %d KiB limit", "task", maxAgentTaskBytes>>10)
	}
	return t.ops.StartAgentTask(ctx, task)
}

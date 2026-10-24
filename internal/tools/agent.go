package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"agent-vivy/internal/domain"
)

const AgentName = "agent"

// maxAgentTaskBytes bounds the delegation brief so a runaway model cannot
// push unbounded text into the child harness.
const maxAgentTaskBytes = 64 << 10

// maxChildTaskDetailBytes bounds the child diagnostic carried by
// ChildTaskError so a runaway child failure cannot flood the parent context.
const maxChildTaskDetailBytes = 2 << 10

// ChildTaskError reports a delegated child that did not complete. It
// describes the child's task, not the parent run: the runtime converts it to
// an ordinary tool result so the parent model can continue without it.
type ChildTaskError struct {
	Detail string
}

func (e *ChildTaskError) Error() string {
	if detail := strings.TrimSpace(e.Detail); detail != "" {
		return "sub-agent failed: " + detail
	}
	return "sub-agent failed"
}

// NewChildTaskError bounds and redacts a raw child failure message before it
// enters the parent's tool result.
func NewChildTaskError(detail string) *ChildTaskError {
	return &ChildTaskError{Detail: boundChildDetail(detail)}
}

func boundChildDetail(text string) string {
	text = strings.TrimSpace(RedactSensitive(text))
	if len(text) <= maxChildTaskDetailBytes {
		return text
	}
	cut := maxChildTaskDetailBytes
	for cut > 0 && !utf8.RuneStart(text[cut]) {
		cut--
	}
	return text[:cut]
}

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
		return "", &ArgError{Field: "args", Reason: fmt.Sprintf("must be a JSON object: %v", err)}
	}
	task := strings.TrimSpace(params.Task)
	if task == "" {
		return "", &ArgError{Field: "task", Reason: "must not be empty"}
	}
	if len(task) > maxAgentTaskBytes {
		return "", &ArgError{Field: "task", Reason: fmt.Sprintf("exceeds the %d KiB limit", maxAgentTaskBytes>>10)}
	}
	return t.ops.StartAgentTask(ctx, task)
}

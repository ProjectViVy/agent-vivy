package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	controlrpc "agent-vivy/internal/rpc"
	"agent-vivy/internal/tools"
)

// agentToolRef defers the worker-manager binding: the builtin registry is
// built before the manager (which needs the registered tool set), so the
// agent tool holds a stable pointer that is armed once the manager exists.
type agentToolRef struct {
	mu      sync.Mutex
	manager *workerManager
}

func (r *agentToolRef) arm(manager *workerManager) {
	r.mu.Lock()
	r.manager = manager
	r.mu.Unlock()
}

// StartAgentTask runs one synchronous sub-agent over the durable child-run
// machinery: clean context, read-only tool surface, parent-owned budget,
// approvals surfacing in the parent session. The parent run id comes from
// the run-scoped tool context.
func (r *agentToolRef) StartAgentTask(ctx context.Context, task, mask string) (string, error) {
	r.mu.Lock()
	m := r.manager
	r.mu.Unlock()
	if m == nil {
		return "", errors.New("sub-agent machinery is not wired")
	}
	parentRunID := tools.RunIDFromContext(ctx)
	if parentRunID == "" {
		return "", errors.New("agent tool requires a run-scoped parent")
	}
	started, err := m.StartChild(ctx, controlrpc.ChildRequest{
		ParentRunID: string(parentRunID),
		Text:        task,
		System:      agentSystemPrompt(mask),
		ToolNames:   m.readOnlyToolNames(),
	})
	if err != nil {
		return "", err
	}
	final, err := m.WaitChild(ctx, started.ID)
	if err != nil {
		// The parent context is gone (cancel or shutdown); do not leave the
		// child running detached behind a failed tool call.
		_, _ = m.CancelChild(context.WithoutCancel(ctx), started.ID)
		return "", err
	}
	switch final.Status {
	case "completed":
		return final.Result, nil
	case "cancelled":
		return "", errors.New("sub-agent was cancelled")
	default:
		if final.Error != "" {
			return "", fmt.Errorf("sub-agent failed: %s", final.Error)
		}
		return "", errors.New("sub-agent failed")
	}
}

// agentSystemPrompt builds the child's system message. The mask is a
// persona hint, not a named agent: the sub-agent stays a disposable,
// single-task vivy with no kernel and no durable identity.
func agentSystemPrompt(mask string) string {
	var b strings.Builder
	b.WriteString("You are a Vivy sub-agent executing one focused task delegated by the main agent. ")
	b.WriteString("You have a clean context and read-only tools; finish the task efficiently and put the complete, self-contained answer in your final message — it is returned verbatim to the delegating agent.")
	if mask != "" {
		b.WriteString("\nPersona hint (mask): " + mask)
	}
	return b.String()
}

// readOnlyToolNames selects the read-only subset of registered tools for the
// sub-agent surface: spec-marked readonly tools minus MCP surfaces and the
// agent tool itself (sub-agents do not spawn sub-agents).
func (m *workerManager) readOnlyToolNames() []string {
	names := make([]string, 0, len(m.tools))
	for _, name := range m.toolOrder {
		if name == tools.AgentName || strings.HasPrefix(name, "mcp_") {
			continue
		}
		if tool, ok := m.tools[name]; ok && tool.Spec().Readonly {
			names = append(names, name)
		}
	}
	return names
}

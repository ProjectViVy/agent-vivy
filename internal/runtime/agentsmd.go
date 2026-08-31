package runtime

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/middlewares/agentsmd"
)

// D6 ratifies the workspace AGENTS.md as the only injected context file:
// no CLAUDE.md/VIVY.md priority chain. The eino agentsmd middleware loads
// it per run through a Backend and injects it transiently before the first
// user message on every model call — the message never enters the persisted
// transcript, so compaction never sees it and needs no carve-out.
const (
	AgentsMDFileName = "AGENTS.md"
	// agentsMDMaxBytes caps the cumulative injected content including
	// @imports; a single oversized root file is further bounded by the
	// filesystem backend's own read cap.
	agentsMDMaxBytes = 64 << 10
)

// AgentsMDBackend is the engine's AGENTS.md loading seam. The alias keeps
// eino's middleware package out of the app wiring (D-007); the
// EinoFilesystemBackend implements it by resolving paths inside the run
// workspace of the request context.
type AgentsMDBackend = agentsmd.Backend

var _ AgentsMDBackend = (*EinoFilesystemBackend)(nil)

// newAgentsMDHandler builds the agentsmd middleware for one engine.
func newAgentsMDHandler(ctx context.Context, backend AgentsMDBackend) (adk.ChatModelAgentMiddleware, error) {
	return agentsmd.New(ctx, &agentsmd.Config{
		Backend: backend,
		// D6: exactly one file, resolved inside the current run workspace.
		// A missing file is a non-fatal warning, so runs without one are
		// unaffected; other read errors abort the load and fail the call.
		AgentsMDFiles:       []string{AgentsMDFileName},
		AllAgentsMDMaxBytes: agentsMDMaxBytes,
		OnLoadWarning: func(filePath string, err error) {
			slog.Debug("agentsmd: skipping context file", "file", filePath, "reason", err.Error())
		},
	})
}

// buildAgentsMDHandler is the NewEngine seam: nil backend disables injection.
func buildAgentsMDHandler(ctx context.Context, backend AgentsMDBackend) (adk.ChatModelAgentMiddleware, error) {
	if backend == nil {
		return nil, nil
	}
	handler, err := newAgentsMDHandler(ctx, backend)
	if err != nil {
		return nil, fmt.Errorf("runtime: agentsmd middleware: %w", err)
	}
	return handler, nil
}

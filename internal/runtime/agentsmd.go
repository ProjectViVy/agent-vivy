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
// eino's middleware package out of the app wiring (D-007). Without an
// instruction root the EinoFilesystemBackend resolves paths inside the
// run workspace; WithInstructionRoot switches to ProjectAgentsMDBackend.
type AgentsMDBackend = agentsmd.Backend

var _ AgentsMDBackend = (*EinoFilesystemBackend)(nil)

// newAgentsMDHandler builds the agentsmd middleware for one engine.
func newAgentsMDHandler(ctx context.Context, backend AgentsMDBackend, files []string) (adk.ChatModelAgentMiddleware, error) {
	if len(files) == 0 {
		files = []string{AgentsMDFileName}
	}
	return agentsmd.New(ctx, &agentsmd.Config{
		Backend: backend,
		// D6: only AGENTS.md (discovered list when an instruction root is
		// wired). A missing file is a non-fatal warning.
		AgentsMDFiles:       append([]string(nil), files...),
		AllAgentsMDMaxBytes: agentsMDMaxBytes,
		OnLoadWarning: func(filePath string, err error) {
			slog.Debug("agentsmd: skipping context file", "file", filePath, "reason", err.Error())
		},
	})
}

// buildAgentsMDHandler is the NewEngine seam: nil backend disables injection.
func buildAgentsMDHandler(ctx context.Context, backend AgentsMDBackend, files []string) (adk.ChatModelAgentMiddleware, error) {
	if backend == nil {
		return nil, nil
	}
	handler, err := newAgentsMDHandler(ctx, backend, files)
	if err != nil {
		return nil, fmt.Errorf("runtime: agentsmd middleware: %w", err)
	}
	return handler, nil
}

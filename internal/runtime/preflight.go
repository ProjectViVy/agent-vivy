package runtime

import (
	"context"
	"errors"
	"strings"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/tools"
)

// PreflightStatus is the side-effect-free readiness result for one proposed
// run.
type PreflightStatus string

const (
	PreflightReady   PreflightStatus = "ready"
	PreflightWarning PreflightStatus = "warning"
	PreflightBlocked PreflightStatus = "blocked"
)

// PreflightResult is safe to show before a run is accepted. It contains no
// model output and does not imply that a run or tool call was persisted.
type PreflightResult struct {
	Status        PreflightStatus
	Mode          domain.RunMode
	SelectedTools []string
	ContextBytes  int
	Warnings      []string
	Blockers      []string
}

// Preflight validates context and policy without creating durable state or
// invoking the provider/tool graph.
func (s *Service) Preflight(ctx context.Context, sessionID domain.SessionID, userText string, options RunOptions) (PreflightResult, error) {
	if s.engine == nil || s.deps.Messages == nil {
		return PreflightResult{}, errors.New("runtime: service not wired")
	}
	result := PreflightResult{Status: PreflightReady}
	mode, err := normalizeRunMode(options.Mode)
	if err != nil {
		return PreflightResult{}, err
	}
	result.Mode = mode
	if strings.TrimSpace(userText) == "" {
		result.Status = PreflightBlocked
		result.Blockers = []string{"text must not be empty"}
		return result, nil
	}
	_, selection, stats, err := s.runMessages(ctx, sessionID, userText)
	result.ContextBytes = stats.Bytes
	result.SelectedTools = selection.Names()
	if err != nil {
		if errors.Is(err, ErrContextBudgetExceeded) {
			result.Status = PreflightBlocked
			result.Blockers = []string{"the transient run context exceeds its configured budget"}
			return result, nil
		}
		return PreflightResult{}, err
	}
	for _, spec := range selection.Specs {
		switch {
		case mode == domain.RunModePlan && !spec.Readonly:
			result.Blockers = append(result.Blockers, spec.Name+" is unavailable in plan mode")
		case spec.Interaction == domain.ToolInteractionQuestion:
			result.Warnings = append(result.Warnings, spec.Name+" will pause for a user answer")
		case !spec.Readonly:
			result.Warnings = append(result.Warnings, spec.Name+" will require approval before execution")
		}
	}
	for _, finding := range tools.ScanPrompt(userText) {
		result.Warnings = append(result.Warnings, finding.Code+": "+finding.Message)
	}
	if len(result.Blockers) > 0 {
		result.Status = PreflightBlocked
	} else if len(result.Warnings) > 0 {
		result.Status = PreflightWarning
	}
	return result, nil
}

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
	Face          domain.Face
	PolicyProfile domain.PolicyProfile
	PolicyHash    string
	SelectedTools []string
	ToolDecisions []PolicyPreview
	ContextBytes  int
	HookReady     bool
	Warnings      []string
	Blockers      []string
	NextActions   []string
}

type PolicyPreview struct {
	ToolName string
	Decision domain.PolicyDecision
	Reason   string
}

// Preflight validates context and policy without creating durable state or
// invoking the provider/tool graph.
func (s *Service) Preflight(ctx context.Context, sessionID domain.SessionID, userText string, options RunOptions) (PreflightResult, error) {
	if s.engine == nil || s.deps.Messages == nil {
		return PreflightResult{}, errors.New("runtime: service not wired")
	}
	result := PreflightResult{Status: PreflightReady}
	if options.Profile == "" {
		options.Profile = s.defaultProfile
	}
	mode, profile, err := normalizeRunPolicy(options.Mode, options.Profile)
	if err != nil {
		return PreflightResult{}, err
	}
	face, err := normalizeFace(options.Face)
	if err != nil {
		return PreflightResult{}, err
	}
	result.Mode = mode
	result.Face = face
	result.PolicyProfile = profile
	if s.engine.cfg.Policy != nil {
		snapshot, snapshotErr := s.engine.cfg.Policy.Snapshot(profile)
		if snapshotErr != nil {
			return PreflightResult{}, snapshotErr
		}
		result.PolicyHash = snapshot.Hash
	}
	result.HookReady = s.engine.cfg.ToolHooks != nil
	if strings.TrimSpace(userText) == "" {
		result.Status = PreflightBlocked
		result.Blockers = []string{"text must not be empty"}
		return result, nil
	}
	_, selection, stats, err := s.runMessages(ctx, sessionID, userText, s.engine, face)
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
		evaluation, evalErr := s.engine.cfg.Policy.Evaluate(profile, spec, nil)
		if evalErr != nil {
			return PreflightResult{}, evalErr
		}
		result.ToolDecisions = append(result.ToolDecisions, PolicyPreview{
			ToolName: spec.Name, Decision: evaluation.Decision, Reason: evaluation.Reason,
		})
		switch evaluation.Decision {
		case domain.PolicyDeny:
			if mode == domain.RunModePlan && !spec.Readonly {
				result.Blockers = append(result.Blockers, spec.Name+" is unavailable in plan mode")
			} else {
				result.Blockers = append(result.Blockers, spec.Name+" is denied by policy profile "+string(profile))
			}
		case domain.PolicyPrompt:
			result.Warnings = append(result.Warnings, spec.Name+" requires approval before execution")
		case domain.PolicyAllow:
			if spec.Interaction == domain.ToolInteractionQuestion {
				result.Warnings = append(result.Warnings, spec.Name+" will pause for a user answer")
			}
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
	if len(result.Warnings) > 0 {
		result.NextActions = append(result.NextActions, "review warnings before starting the run")
	}
	if len(result.Blockers) > 0 {
		result.NextActions = append(result.NextActions, "change the policy profile or request a safer tool set")
	}
	return result, nil
}

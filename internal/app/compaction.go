package app

import (
	"fmt"
	"log/slog"
	"strings"

	"agent-vivy/internal/app/settings"
	"agent-vivy/internal/config"
	"agent-vivy/internal/runtime"
)

// compactionPolicyFor merges the settings overlay over the config defaults
// and resolves max_tokens=0 against the provider model's context window.
// The result is the engine-visible compaction snapshot.
func compactionPolicyFor(cfg config.Config, overlay *settings.CompactionSettings, modelWindow int) runtime.CompactionPolicy {
	c := cfg.Runtime.Compaction
	enabled := c.Enabled
	maxTokens := c.MaxTokens
	pct := c.TriggerPercent
	keep := c.KeepRecent
	if overlay != nil {
		if overlay.Enabled != nil {
			enabled = *overlay.Enabled
		}
		if overlay.MaxTokens != 0 {
			maxTokens = overlay.MaxTokens
		}
		if overlay.TriggerPercent != 0 {
			pct = overlay.TriggerPercent
		}
		if overlay.KeepRecent != 0 {
			keep = overlay.KeepRecent
		}
	}
	if maxTokens <= 0 {
		maxTokens = modelWindow
	}
	return runtime.CompactionPolicy{Enabled: enabled, MaxTokens: maxTokens, TriggerPercent: pct, KeepRecent: keep}
}

// mergedCompactionConfig folds the settings overlay into the config struct
// so the startup engine already uses the user's compaction values (the same
// overlay is re-applied on every settings save via ScheduleEngineReload).
func mergedCompactionConfig(base config.CompactionConfig, overlay *settings.CompactionSettings) config.CompactionConfig {
	if overlay == nil {
		return base
	}
	if overlay.Enabled != nil {
		base.Enabled = *overlay.Enabled
	}
	if overlay.MaxTokens != 0 {
		base.MaxTokens = overlay.MaxTokens
	}
	if overlay.TriggerPercent != 0 {
		base.TriggerPercent = overlay.TriggerPercent
	}
	if overlay.KeepRecent != 0 {
		base.KeepRecent = overlay.KeepRecent
	}
	return base
}

// buildEngineConfig assembles the full EngineConfig from the validated
// config plus the resolved compaction policy. It is shared by the startup
// engine and the settings-save reload path so both see identical wiring.
// agentsMDBackend and offloadBackend are typically the same
// *runtime.EinoFilesystemBackend serving the run workspace.
func buildEngineConfig(cfg config.Config, skillBackend *runtime.EinoSkillBackend, agentsMDBackend runtime.AgentsMDBackend, checkpoints *runtime.VersionedCheckpointStore, policy *runtime.PolicyEngine, hooks *runtime.ToolHookChain, cmp *runtime.CompactionPolicy, summaryModel runtime.SummaryModel, offloadBackend *runtime.EinoFilesystemBackend) runtime.EngineConfig {
	engineCfg := runtime.EngineConfig{
		StreamBuffer:         cfg.Runtime.StreamBuffer,
		MaxEventPayloadBytes: cfg.Runtime.MaxEventPayloadBytes,
		MaxToolTurns:         cfg.Runtime.MaxToolTurns,
		MaxContextBytes:      cfg.Runtime.MaxContextBytes,
		MaxHistoryMessages:   cfg.Runtime.MaxHistoryMessages,
		MaxToolResultBytes:   cfg.Runtime.MaxToolResultBytes,
		Checkpoints:          checkpoints,
		Policy:               policy,
		ToolHooks:            hooks,
		AutoApproveTools:     cfg.Runtime.Sandbox.Approval.AutoApproveTools,
		Compaction:           cmp,
		SummaryModelID:       cfg.Runtime.Compaction.SummaryModel,
	}
	if summaryModel != nil {
		engineCfg.SummaryModel = summaryModel
	}
	if skillBackend != nil {
		engineCfg.SkillBackend = skillBackend
	}
	if agentsMDBackend != nil {
		engineCfg.AgentsMDBackend = agentsMDBackend
	}
	if offloadBackend != nil {
		engineCfg.OffloadBackend = offloadBackend
	}
	return engineCfg
}

// sameCompactionPolicy reports whether two policies are equal; a nil and an
// empty-disabled policy count as equal (both mean "no compression").
func projectInstructionBackends(logger *slog.Logger, instructionRoot string, skillBackend *runtime.EinoSkillBackend, fileBackend *runtime.EinoFilesystemBackend) (runtime.AgentsMDBackend, []string, error) {
	var workspace runtime.AgentsMDBackend
	if fileBackend != nil {
		workspace = fileBackend
	}
	if strings.TrimSpace(instructionRoot) == "" {
		return workspace, nil, nil
	}
	discovered, err := runtime.DiscoverProjectInstructions(instructionRoot)
	if err != nil {
		return nil, nil, fmt.Errorf("app: discover project instructions: %w", err)
	}
	backend, err := runtime.NewProjectAgentsMDBackend(discovered.Root)
	if err != nil {
		return nil, nil, fmt.Errorf("app: project agentsmd backend: %w", err)
	}
	if skillBackend != nil {
		if err := skillBackend.SetProjectSkillRoots(discovered.SkillRoots); err != nil {
			return nil, nil, fmt.Errorf("app: project skill roots: %w", err)
		}
	}
	if logger != nil {
		logger.Info("project instructions discovered",
			"root", discovered.Root,
			"agents_md", discovered.AgentsMDFiles,
			"skill_roots", len(discovered.SkillRoots))
	}
	return backend, discovered.AgentsMDFiles, nil
}

func sameCompactionPolicy(a, b *runtime.CompactionPolicy) bool {
	if a == nil || b == nil {
		return (a == nil) == (b == nil)
	}
	return a.Enabled == b.Enabled && a.MaxTokens == b.MaxTokens &&
		a.TriggerPercent == b.TriggerPercent && a.KeepRecent == b.KeepRecent
}

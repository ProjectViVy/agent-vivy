package app

import (
	"context"
	"encoding/json"
	"fmt"

	"agent-vivy/internal/config"
	"agent-vivy/internal/domain"
	"agent-vivy/internal/modelhost"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/tools"
	workflow "agent-vivy/internal/workflow"
	"agent-vivy/internal/workflowhost"
)

// workflowCapabilitySource supplies the live collaborators the WorkflowHost
// capability snapshot reads at each validation boundary. All fields are
// deferred lookups so the snapshot never freezes startup-time state.
type workflowCapabilitySource struct {
	GenerationID  string
	CurrentModel  func() ResolvedModel
	ModelStatuses func(activeID string, activeReady bool) []modelhost.ProfileStatus
	ToolSpecs     func() []domain.ToolSpec
}

// newWorkflowOperations assembles the WorkflowHost behind the sealed
// vivy/workflow module. It persists nothing itself: definitions and run
// projections stay in the single storage backend, and execution stays with
// the injected domain.PlanExecutor (WF-1: unavailable).
func newWorkflowOperations(cfg config.Config, backend storage.Engine, source workflowCapabilitySource) (tools.WorkflowOperations, error) {
	limits := cfg.EffectiveWorkflowConfig()
	host, err := workflowhost.New(workflowhost.Config{
		Definitions: backend, WorkflowRuns: backend, Sessions: backend, Runs: backend, Journal: backend,
		Executor: workflowhost.UnavailableExecutor{},
		Limits: workflow.Limits{
			MaxNodes:           limits.MaxNodes,
			MaxParallelism:     limits.MaxParallelism,
			MaxNodeOutputBytes: limits.MaxNodeOutputBytes,
			DefinitionMaxBytes: limits.DefinitionMaxBytes,
		},
		Capabilities: workflowhost.CapabilitySourceFunc(func(context.Context) (workflowhost.CapabilitySnapshot, error) {
			current := source.CurrentModel()
			profiles := make(map[string]workflow.ModelCapability)
			for _, status := range source.ModelStatuses(current.Provider, current.Ready) {
				ready := status.State == modelhost.ProfileReady
				modelID := ""
				if ready {
					modelID = current.Model
				} else if len(status.ModelIDs) > 0 {
					modelID = status.ModelIDs[0]
				}
				profiles[status.ID] = workflow.ModelCapability{
					Provider: status.ID, Model: modelID, Configured: ready, Active: ready,
				}
			}
			catalog := make(map[string]workflow.ToolCapability)
			available := make(map[string]bool)
			for _, spec := range source.ToolSpecs() {
				catalog[spec.Name] = workflow.ToolCapability{Readonly: spec.Readonly, Available: true}
				available[spec.Name] = true
			}
			identity, err := json.Marshal(map[string]any{
				"generation_id":  source.GenerationID,
				"model_profiles": profiles,
				"tools":          catalog,
			})
			if err != nil {
				return workflowhost.CapabilitySnapshot{}, err
			}
			return workflowhost.CapabilitySnapshot{
				Validation: workflow.ValidationContext{ModelProfiles: profiles, Tools: catalog, AvailableTools: available},
				Identity:   identity,
			}, nil
		}),
	})
	if err != nil {
		return nil, fmt.Errorf("app: construct WorkflowHost: %w", err)
	}
	return host, nil
}

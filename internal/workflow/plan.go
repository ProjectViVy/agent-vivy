package workflow

import (
	"encoding/json"
	"fmt"

	"agent-vivy/internal/domain"
)

// Compile turns a validated, immutable definition into the domain-only seam
// consumed by WF-2. The returned plan owns all byte slices and carries
// concrete provider identities for model nodes, while retaining the authored
// profile for diagnostics and provenance.
func Compile(validated ValidationResult, rev int64, ctx ValidationContext) (domain.WorkflowPlan, error) {
	if len(validated.Diagnostics) > 0 {
		return domain.WorkflowPlan{}, validationError(validated.Diagnostics, ErrSchemaInvalid)
	}
	if validated.Definition.ID == "" || len(validated.Canonical) == 0 || validated.Hash == "" {
		return domain.WorkflowPlan{}, validationError(
			[]Diagnostic{diagnostic("schema", "", "workflow must be validated before compilation")},
			ErrSchemaInvalid,
		)
	}

	order := validated.Order
	if len(order) == 0 {
		var err error
		validated, err = ValidateDefinition(validated.Definition, validated.Canonical, ctx)
		if err != nil {
			return domain.WorkflowPlan{}, err
		}
		order = validated.Order
	}
	byID := make(map[string]Node, len(validated.Definition.Nodes))
	for _, node := range validated.Definition.Nodes {
		byID[node.ID] = node
	}
	if len(order) != len(byID) {
		return domain.WorkflowPlan{}, validationError(
			[]Diagnostic{diagnostic("topology", "nodes", "validated workflow has no complete topological order")},
			ErrTopologyInvalid,
		)
	}

	plan := domain.WorkflowPlan{
		DefinitionID: validated.Definition.ID,
		Rev:          rev,
		Hash:         validated.Hash,
		Edges:        append([]domain.Edge(nil), validated.Definition.Edges...),
		Outputs:      append([]domain.OutputBinding(nil), validated.Definition.Outputs...),
		Budget: domain.BudgetEnvelope{
			MaxNodes:           ctx.Limits.MaxNodes,
			Parallelism:        ctx.Limits.MaxParallelism,
			MaxNodeOutputBytes: ctx.Limits.MaxNodeOutputBytes,
			DefinitionMaxBytes: ctx.Limits.DefinitionMaxBytes,
		},
	}
	plan.Nodes = make([]domain.PlanNode, 0, len(order))
	for _, id := range order {
		node, ok := byID[id]
		if !ok {
			return domain.WorkflowPlan{}, validationError(
				[]Diagnostic{diagnostic("topology", "nodes", fmt.Sprintf("topological order references unknown node %q", id))},
				ErrTopologyInvalid,
			)
		}
		spec, err := compileNodeSpec(node, ctx)
		if err != nil {
			return domain.WorkflowPlan{}, err
		}
		plan.Nodes = append(plan.Nodes, domain.PlanNode{
			ID:        node.ID,
			Kind:      node.Kind,
			Spec:      spec,
			TimeoutMS: node.TimeoutMS,
		})
	}
	return plan, nil
}

func compileNodeSpec(node Node, ctx ValidationContext) (json.RawMessage, error) {
	spec := cloneJSON(node.Config)
	if node.Kind != NodeKindModel {
		return spec, nil
	}
	var config map[string]json.RawMessage
	if err := json.Unmarshal(node.Config, &config); err != nil {
		return nil, validationError(
			[]Diagnostic{diagnostic("capability", "nodes."+node.ID+".config", "model config cannot be decoded")},
			ErrCapabilityUnavailable,
		)
	}
	var profile string
	if err := json.Unmarshal(config["profile"], &profile); err != nil {
		return nil, validationError(
			[]Diagnostic{diagnostic("capability", "nodes."+node.ID+".config.profile", "model profile cannot be decoded")},
			ErrCapabilityUnavailable,
		)
	}
	capability, ok := ctx.ModelProfiles[profile]
	if !ok || !capability.Configured || !capability.Active || capability.Provider == "" || capability.Model == "" {
		return nil, validationError(
			[]Diagnostic{diagnostic("capability", "nodes."+node.ID+".config.profile", fmt.Sprintf("model profile %q has no concrete active provider identity", profile))},
			ErrCapabilityUnavailable,
		)
	}
	provider, err := json.Marshal(capability.Provider)
	if err != nil {
		return nil, err
	}
	model, err := json.Marshal(capability.Model)
	if err != nil {
		return nil, err
	}
	config["provider"] = provider
	config["model"] = model
	resolved, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("compile node %q spec: %w", node.ID, err)
	}
	return resolved, nil
}

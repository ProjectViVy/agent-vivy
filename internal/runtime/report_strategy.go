package runtime

import (
	"context"
	"fmt"

	inofy "github.com/ProjectViVy/inofy"
)

// TrustedStrategyReport is the host-bound report strategy id. The sealed
// four-node program is code-owned and content-hashed; callers never submit
// definitions, prompts, node implementations, or tools.
const TrustedStrategyReport = "report/v1"

// Report node type ids are the only effect verbs the report executor
// honors. R0 admits the sealed program but binds no effects: every node
// replies capability_unavailable, so a committed Run can never advertise a
// generated report through placeholder output.
const (
	reportNodeCollect        = "vivy.report.collect@1"
	reportNodeNarrate        = "vivy.report.narrate@1"
	reportNodeValidateRender = "vivy.report.validate-render@1"
	reportNodePersist        = "vivy.report.persist@1"
	reportImplementationID   = "vivy-report/0"
	reportStageCollectID     = "collect"
	reportStageNarrateID     = "narrate"
	reportStageValidateID    = "validate-render"
	reportStagePersistID     = "persist"
)

// reportDefinition returns the fixed four-stage sealed graph. Node inputs
// chain the previous stage packet; the run input carries the admission
// pins the executor re-verifies per effect.
func reportDefinition() (inofy.Definition, error) {
	stages := []struct {
		id  string
		typ string
	}{
		{reportStageCollectID, reportNodeCollect},
		{reportStageNarrateID, reportNodeNarrate},
		{reportStageValidateID, reportNodeValidateRender},
		{reportStagePersistID, reportNodePersist},
	}
	nodes := make([]inofy.Node, 0, len(stages))
	edges := make([]inofy.Edge, 0, len(stages)-1)
	for i, stage := range stages {
		node := inofy.Node{ID: stage.id, Kind: inofy.NodeKindCall, Type: stage.typ}
		source := "input"
		if i > 0 {
			source = stages[i-1].id
			edges = append(edges, inofy.Edge{From: stages[i-1].id, To: stage.id})
		}
		node.Inputs = map[string]inofy.Binding{"input": {Source: source}}
		nodes = append(nodes, node)
	}
	return inofy.Definition{
		SchemaVersion: inofy.SchemaVersionV1,
		Graph: inofy.Graph{
			Nodes: nodes,
			Edges: edges,
			Exits: []string{reportStagePersistID},
			Outputs: map[string]inofy.Binding{
				"outcome": {Source: reportStagePersistID},
			},
		},
	}, nil
}

// reportDescriptors registers the four sealed node types under this
// build's implementation identity.
func reportDescriptors() []inofy.NodeDescriptor {
	types := []string{reportNodeCollect, reportNodeNarrate, reportNodeValidateRender, reportNodePersist}
	titles := []string{"collect", "narrate", "validate-render", "persist"}
	out := make([]inofy.NodeDescriptor, 0, len(types))
	for i, typeID := range types {
		out = append(out, inofy.NodeDescriptor{
			TypeID:           typeID,
			ImplementationID: reportImplementationID,
			Display:          inofy.DisplayMeta{Title: "vivy report " + titles[i]},
		})
	}
	return out
}

// reportStrategyCatalog is the closed report node catalog: authored graphs
// cannot mint these types and the report catalog cannot mint child-task or
// strategy calls.
func reportStrategyCatalog() (inofy.Catalog, error) {
	return inofy.NewCatalog(reportDescriptors())
}

// reportStrategyAdmission compiles the sealed report program against its
// dedicated catalog. The definition bytes come from code, so a caller
// payload can never alter the admitted graph.
func reportStrategyAdmission(ctx context.Context) (inofyAdmission, error) {
	def, err := reportDefinition()
	if err != nil {
		return inofyAdmission{}, fmt.Errorf("runtime: build report strategy: %w", err)
	}
	catalog, err := reportStrategyCatalog()
	if err != nil {
		return inofyAdmission{}, err
	}
	program, diags, err := inofy.Compile(ctx, def, catalog, inofy.CompileOptions{Limits: inofyWorkflowLimits()})
	if err != nil {
		return inofyAdmission{}, fmt.Errorf("runtime: compile report strategy: %w", err)
	}
	if len(diags) != 0 {
		return inofyAdmission{}, fmt.Errorf("runtime: invalid report strategy: %v", diags)
	}
	canonical, err := inofy.Normalize(def)
	if err != nil {
		return inofyAdmission{}, fmt.Errorf("runtime: normalize report strategy: %w", err)
	}
	return inofyAdmission{Definition: def, CanonicalJSON: canonical, Meta: program.Meta(), Program: program}, nil
}

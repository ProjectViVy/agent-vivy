package domain

import (
	"context"
	"encoding/json"
)

// WorkflowNodeKind is the closed WF-1 node vocabulary. New kinds require a
// Port/Host contract and are not introduced by a definition document.
type NodeKind string

// WorkflowNodeKind is retained as a descriptive alias for callers that need
// to distinguish workflow node kinds from other domain enums.
type WorkflowNodeKind = NodeKind

const (
	WorkflowNodeModel NodeKind = "model"
	WorkflowNodeAgent NodeKind = "agent"
	WorkflowNodeIO    NodeKind = "io"
)

func (kind NodeKind) Valid() bool {
	return kind == WorkflowNodeModel || kind == WorkflowNodeAgent || kind == WorkflowNodeIO
}

// Edge and OutputBinding are data-only seam types. The internal/workflow
// package aliases them for definition validation without making the domain
// layer depend on an implementation package.
type Edge struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type OutputBinding struct {
	Name     string `json:"name"`
	Template string `json:"template"`
}

type WorkflowEdge = Edge
type WorkflowOutputBinding = OutputBinding

// BudgetEnvelope carries ceilings already checked by the workflow validator.
// It intentionally has no runtime or Eino-specific fields.
type BudgetEnvelope struct {
	MaxNodes           int `json:"max_nodes"`
	Parallelism        int `json:"parallelism"`
	MaxNodeOutputBytes int `json:"max_node_output_bytes"`
	DefinitionMaxBytes int `json:"definition_max_bytes"`
}

type WorkflowPlan struct {
	DefinitionID string
	Rev          int64
	Hash         string
	Nodes        []PlanNode
	Edges        []Edge
	Outputs      []OutputBinding
	Budget       BudgetEnvelope
}

type PlanNode struct {
	ID        string
	Kind      NodeKind
	Spec      json.RawMessage
	TimeoutMS int64
}

type NodeOutcome struct {
	NodeID string
	Kind   NodeKind
	Status string
	Output string
	Error  string
}

type PlanResult struct {
	Outcomes []NodeOutcome
	Outputs  map[string]string
}

type PlanEventSink interface {
	NodeStarted(context.Context, string, NodeKind) error
	NodeCompleted(context.Context, NodeOutcome) error
}

type PlanExecutor interface {
	ExecutePlan(context.Context, RunID, WorkflowPlan, PlanEventSink) (PlanResult, error)
}

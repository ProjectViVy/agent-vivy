package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/ProjectViVy/inofy"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/orchestration"
)

const workflowChildType = "vivy.child-task@1"
const rejectedWorkflowSchema = `{"not":{}}`

const childConfigSchema = `{"type":"object","additionalProperties":false,"required":["task"],"properties":{"task":{"type":"string","minLength":1,"maxLength":4096},"tool_names":{"type":"array","maxItems":32,"items":{"type":"string"}}}}`

type inofyAdmission struct {
	Definition    inofy.Definition
	CanonicalJSON []byte
	Meta          inofy.ProgramMeta
	Program       *inofy.Program
}

type WorkflowDefinitionProposal struct {
	Definition json.RawMessage `json:"definition"`
	Digest     string          `json:"digest"`
}

var ErrINOFYStorageUnavailable = errors.New("runtime: INOFY workflow execution awaits atomic RunStore admission")
var ErrINOFYInvalidDefinition = errors.New("runtime: invalid INOFY definition")

// ProposeINOFYWorkflow checks the author's live authority without persisting
// or executing anything. The same check is repeated at Run admission.
func (s *Service) ProposeINOFYWorkflow(ctx context.Context, parentRunID domain.RunID, raw json.RawMessage) (WorkflowDefinitionProposal, error) {
	if s == nil || s.engine == nil || s.deps.Runs == nil {
		return WorkflowDefinitionProposal{}, errors.New("runtime: workflow validation is not wired")
	}
	parent, _, _, parentTools, err := s.currentChildAuthorizer(ctx, parentRunID)
	if err != nil {
		return WorkflowDefinitionProposal{}, err
	}
	if err := s.validateWorkflowDepth(parent); err != nil {
		return WorkflowDefinitionProposal{}, err
	}
	admitted, err := validateINOFYDefinition(ctx, raw, s.workflowChildTools(parentTools))
	if err != nil {
		return WorkflowDefinitionProposal{}, fmt.Errorf("%w: %w", ErrINOFYInvalidDefinition, err)
	}
	return WorkflowDefinitionProposal{Definition: admitted.CanonicalJSON, Digest: admitted.Meta.DefinitionDigest}, nil
}

// validateINOFYDefinition freezes the only executable graph vocabulary at
// the runtime boundary. The caller supplies the already narrowed host tools;
// a tool-facing JSON Schema never grants authority.
func validateINOFYDefinition(ctx context.Context, raw json.RawMessage, allowedTools []string) (inofyAdmission, error) {
	if !json.Valid(raw) || len(raw) > 64<<10 || len(bytes.TrimSpace(raw)) == 0 {
		return inofyAdmission{}, errors.New("runtime: invalid or oversized workflow definition")
	}
	artifactBytes := make([]byte, 0, len(raw)+16)
	artifactBytes = append(artifactBytes, `{"definition":`...)
	artifactBytes = append(artifactBytes, raw...)
	artifactBytes = append(artifactBytes, '}')
	artifact, diags, err := inofy.DecodeArtifact(artifactBytes)
	if err != nil {
		return inofyAdmission{}, fmt.Errorf("runtime: invalid INOFY definition %v: %w", diags, err)
	}
	def := artifact.Definition
	if len(def.Graph.Nodes) == 0 || len(def.Graph.Nodes) > orchestration.MaxNodes ||
		len(def.Graph.Edges) > orchestration.MaxEdges || len(def.Graph.Outputs) == 0 ||
		len(def.Graph.Outputs) > orchestration.MaxOutputs {
		return inofyAdmission{}, errors.New("runtime: workflow nodes, edges or outputs exceed host limits")
	}
	allowed := make(map[string]bool, len(allowedTools))
	for _, name := range allowedTools {
		allowed[name] = true
	}
	totalTaskBytes := 0
	for _, node := range def.Graph.Nodes {
		if node.Kind != inofy.NodeKindCall || node.Type != workflowChildType {
			return inofyAdmission{}, fmt.Errorf("runtime: unsupported workflow node kind/type %q/%q", node.Kind, node.Type)
		}
		if node.Retry != nil || node.OnError != nil {
			return inofyAdmission{}, fmt.Errorf("runtime: node %q retry and error fallback are unsupported for child effects", node.ID)
		}
		literalBytes := 0
		for _, binding := range node.Inputs {
			literalBytes += len(binding.Literal)
		}
		if literalBytes > orchestration.MaxOutputBytes {
			return inofyAdmission{}, fmt.Errorf("runtime: node %q literal input exceeds host limit", node.ID)
		}
		var config struct {
			Task      string   `json:"task"`
			ToolNames []string `json:"tool_names"`
		}
		decoder := json.NewDecoder(bytes.NewReader(node.Config))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&config); err != nil {
			return inofyAdmission{}, fmt.Errorf("runtime: node %q config: %w", node.ID, err)
		}
		if strings.TrimSpace(config.Task) == "" || len(config.Task) > orchestration.MaxTaskBytes {
			return inofyAdmission{}, fmt.Errorf("runtime: node %q task exceeds host limit or is empty", node.ID)
		}
		totalTaskBytes += len(config.Task)
		if totalTaskBytes > orchestration.MaxTotalTaskBytes {
			return inofyAdmission{}, errors.New("runtime: combined workflow task text exceeds host limit")
		}
		for _, name := range config.ToolNames {
			if !allowed[name] {
				return inofyAdmission{}, fmt.Errorf("runtime: node %q tool %q widens parent authority", node.ID, name)
			}
		}
	}
	catalog, err := trustedINOFYCatalog()
	if err != nil {
		return inofyAdmission{}, err
	}
	limits := inofyWorkflowLimits()
	program, diags, err := inofy.Compile(ctx, def, catalog, inofy.CompileOptions{Limits: limits})
	if err != nil {
		return inofyAdmission{}, fmt.Errorf("runtime: compile INOFY workflow: %w", err)
	}
	if len(diags) != 0 {
		return inofyAdmission{}, fmt.Errorf("runtime: invalid INOFY workflow: %v", diags)
	}
	canonical, err := inofy.Normalize(def)
	if err != nil {
		return inofyAdmission{}, fmt.Errorf("runtime: normalize INOFY workflow: %w", err)
	}
	return inofyAdmission{Definition: def, CanonicalJSON: canonical, Meta: program.Meta(), Program: program}, nil
}

// trustedINOFYCatalog is the single host node catalog: VIVY admits only the
// governed child-task node type. Draft validation, publishing and admission
// all compile against this one catalog so a published revision can never
// reference a node type admission would reject.
func trustedINOFYCatalog() (inofy.Catalog, error) {
	return inofy.NewCatalog([]inofy.NodeDescriptor{{
		TypeID: workflowChildType, ImplementationID: workflowChildType,
		ConfigSchema: json.RawMessage(childConfigSchema),
		InputSchema:  json.RawMessage(`{"type":"object"}`),
		OutputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"required":["result"],"properties":{"result":{"type":"string","maxLength":8192}}}`),
		Replay:       inofy.ReplayNonReplayable,
	}})
}

// WorkflowDefinitionSchema narrows INOFY's schema to the trusted host catalog
// for model-facing authoring. Admission still executes the strict decoder.
func WorkflowDefinitionSchema() json.RawMessage {
	var schema map[string]any
	if err := json.Unmarshal(inofy.DefinitionSchema(), &schema); err != nil {
		return json.RawMessage(rejectedWorkflowSchema)
	}
	object := func(value any) map[string]any { m, _ := value.(map[string]any); return m }
	defs := object(schema["$defs"])
	graph := object(object(defs["graph"])["properties"])
	nodes, edges, outputs := object(graph["nodes"]), object(graph["edges"]), object(graph["outputs"])
	variants, ok := object(defs["node"])["oneOf"].([]any)
	if defs == nil || graph == nil || nodes == nil || edges == nil || outputs == nil || !ok {
		return json.RawMessage(rejectedWorkflowSchema)
	}
	nodes["maxItems"] = orchestration.MaxNodes
	edges["maxItems"] = orchestration.MaxEdges
	outputs["maxProperties"] = orchestration.MaxOutputs
	outputs["minProperties"] = 1
	found := false
	for _, candidate := range variants {
		props := object(object(candidate)["properties"])
		if object(props["kind"])["const"] != "call" {
			continue
		}
		found = true
		props["type"] = map[string]any{"const": workflowChildType}
		var config any
		if err := json.Unmarshal([]byte(childConfigSchema), &config); err != nil {
			return json.RawMessage(rejectedWorkflowSchema)
		}
		props["config"] = config
		delete(props, "retry")
		delete(props, "on_error")
		object(defs["node"])["oneOf"] = []any{candidate}
		break
	}
	if !found {
		return json.RawMessage(rejectedWorkflowSchema)
	}
	raw, err := json.Marshal(schema)
	if err != nil {
		return json.RawMessage(rejectedWorkflowSchema)
	}
	return raw
}

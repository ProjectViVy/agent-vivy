// Package workflow owns the build-selected vivy/workflow module and the
// generated std/tool@v1 provider identities for the WF-1 product slice.
package workflow

import (
	"context"
	"encoding/json"
	"errors"

	"agent-vivy/sdk/module"
	toolport "agent-vivy/sdk/port/tool"
)

const (
	ID   = "vivy/workflow"
	Port = "core/workflow-host@v1"
)

const zeroDigest = "0000000000000000000000000000000000000000000000000000000000000000"

var toolIDs = []string{
	"workflow_list",
	"workflow_get",
	"workflow_validate",
	"workflow_define",
	"workflow_run",
	"workflow_runs",
}

// ToolIDs returns the module-owned provider identities in generated order.
func ToolIDs() []string { return append([]string(nil), toolIDs...) }

// NewModule constructs the generation-scoped lifecycle owner. WorkflowHost
// runtime dependencies are composed by internal/app after the sealed Assembly
// has been checked.
func NewModule() module.Module { return ownerModule{} }

func (ownerModule) Descriptor() module.Descriptor {
	provides := []module.PortRef{{Port: Port, ID: "vivy.workflow-host"}}
	for _, id := range toolIDs {
		provides = append(provides, module.PortRef{Port: "std/tool@v1", ID: id})
	}
	return module.Descriptor{
		APIVersion: module.APIVersionV1,
		Module:     module.Identity{ID: ID, Version: "1.0.0"},
		Source:     module.Source{Ref: "file:internal", SHA256: zeroDigest},
		Provides:   provides,
		Requires: []module.Requirement{{
			PortRef:  module.PortRef{Port: "core/tool-host@v1"},
			Provider: "vivy/tool-host",
		}},
		Lifecycle: module.Lifecycle{Scope: module.ScopeGeneration},
	}
}

func (ownerModule) Construct(context.Context, module.Host) (module.Instance, error) {
	return ownerInstance{}, nil
}

type ownerModule struct{}

type ownerInstance struct{}

func (ownerInstance) Start(context.Context) error { return nil }
func (ownerInstance) Ready(context.Context) error { return nil }
func (ownerInstance) Stop(context.Context) error  { return nil }
func (ownerInstance) Close(context.Context) error { return nil }

// ToolProviders returns the exact provider collection emitted into the
// generated Assembly. Provider Invoke delegates through the generated Host;
// actual implementations are injected by app.bindGeneratedTools so this
// package does not create a second registry or policy path.
func ToolProviders() []toolport.ToolProvider {
	providers := make([]toolport.ToolProvider, 0, len(toolIDs))
	for _, id := range toolIDs {
		providers = append(providers, workflowToolProvider{definition: toolDefinition(id)})
	}
	return providers
}

// ToolDefinition returns the declarative std/tool@v1 contract for one
// workflow provider. Unknown names receive the empty-object shape so callers
// cannot accidentally invent a second provider identity.
func ToolDefinition(id string) toolport.Definition {
	return toolDefinition(id)
}

type workflowToolProvider struct {
	definition toolport.Definition
}

func (provider workflowToolProvider) Definition() toolport.Definition { return provider.definition }

func (provider workflowToolProvider) Invoke(ctx context.Context, host toolport.Host, args json.RawMessage) (toolport.Result, error) {
	if host == nil {
		return toolport.Result{}, errors.New("workflow tool provider: host is required")
	}
	text, err := host.InvokeTool(ctx, provider.definition.ID, args)
	return toolport.Result{Text: text}, err
}

func toolDefinition(id string) toolport.Definition {
	effect := toolport.EffectRead
	if id == "workflow_define" || id == "workflow_run" {
		effect = toolport.EffectWrite
	}
	return toolport.Definition{
		ID:          id,
		Description: "Vivy workflow operation " + id,
		Effect:      effect,
		Schema:      json.RawMessage(toolSchema(id)),
	}
}

func toolSchema(id string) string {
	switch id {
	case "workflow_list":
		return `{"type":"object","additionalProperties":false,"properties":{}}`
	case "workflow_get":
		return `{"type":"object","additionalProperties":false,"required":["id"],"properties":{"id":{"type":"string","minLength":1,"maxLength":64},"rev":{"type":"integer","minimum":1}}}`
	case "workflow_validate", "workflow_define":
		return `{"type":"object","additionalProperties":false,"required":["definition"],"properties":{"definition":{"type":"object"}}}`
	case "workflow_run":
		return `{"type":"object","additionalProperties":false,"required":["id","inputs"],"properties":{"id":{"type":"string","minLength":1,"maxLength":64},"rev":{"type":"integer","minimum":1},"inputs":{"type":"object"},"session_id":{"type":"string","minLength":1,"maxLength":128}}}`
	case "workflow_runs":
		return `{"type":"object","additionalProperties":false,"required":["id"],"properties":{"id":{"type":"string","minLength":1,"maxLength":64},"limit":{"type":"integer","minimum":1,"maximum":100}}}`
	default:
		return `{"type":"object","additionalProperties":false,"properties":{}}`
	}
}

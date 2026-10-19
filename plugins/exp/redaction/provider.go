package redaction

import (
	"context"
	"encoding/json"
	"fmt"

	"agent-vivy/sdk/module"
	"agent-vivy/sdk/port/tool"
)

type vivyModule struct{}
type instance struct{}

func New() module.Module { return vivyModule{} }
func (vivyModule) Descriptor() module.Descriptor {
	return module.Descriptor{
		APIVersion: module.APIVersionV1,
		Module:     module.Identity{ID: "vivy/exp-redaction", Version: "0.1.0"},
		Source:     module.Source{Ref: "repo:plugins/exp/redaction", SHA256: "fe6eafb4c40347a182e9848a4aed105bceb24594db4fe5498be023eb9046f5bb"},
		Provides:   []module.PortRef{{Port: "std/tool@v1", ID: "vivy.exp.redact_text"}},
		Requires:   []module.Requirement{{PortRef: module.PortRef{Port: "core/tool-host@v1"}, Provider: "vivy/tool-host"}},
		Lifecycle:  module.Lifecycle{Scope: module.ScopeGeneration},
	}
}
func (vivyModule) Construct(context.Context, module.Host) (module.Instance, error) {
	return instance{}, nil
}
func (instance) Start(context.Context) error { return nil }
func (instance) Ready(context.Context) error { return nil }
func (instance) Stop(context.Context) error  { return nil }
func (instance) Close(context.Context) error { return nil }

type Provider struct{}

func NewProvider() tool.ToolProvider { return Provider{} }
func (Provider) Definition() tool.Definition {
	return tool.Definition{ID: "vivy.exp.redact_text", Effect: tool.EffectRead, Description: "Explicit experimental text redaction", Schema: json.RawMessage(`{"type":"object","properties":{"text":{"type":"string"}},"required":["text"],"additionalProperties":false}`)}
}
func (Provider) Invoke(ctx context.Context, _ tool.Host, args json.RawMessage) (tool.Result, error) {
	if err := ctx.Err(); err != nil {
		return tool.Result{}, err
	}
	var input map[string]json.RawMessage
	if err := json.Unmarshal(args, &input); err != nil {
		return tool.Result{}, err
	}
	var value string
	if len(input) != 1 || input["text"] == nil || string(input["text"]) == "null" {
		return tool.Result{}, fmt.Errorf("text must be a string")
	}
	if err := json.Unmarshal(input["text"], &value); err != nil {
		return tool.Result{}, err
	}
	return tool.Result{Text: Redact(value)}, nil
}

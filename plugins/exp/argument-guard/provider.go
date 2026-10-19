package argumentguard

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"agent-vivy/sdk/module"
	"agent-vivy/sdk/port/pretool"
)

type vivyModule struct{}
type instance struct{}

func New() module.Module { return vivyModule{} }
func (vivyModule) Descriptor() module.Descriptor {
	return module.Descriptor{
		APIVersion: module.APIVersionV1,
		Module:     module.Identity{ID: "vivy/exp-argument-guard", Version: "0.1.0"},
		Source:     module.Source{Ref: "repo:plugins/exp/argument-guard", SHA256: "47717b6c964ce413e87d914e588856d7bf2eb357d8e330d88d54c0cabc38e1ae"},
		Provides:   []module.PortRef{{Port: "std/middleware/pre-tool@v1", ID: "vivy.exp.argument_guard"}},
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

func NewProvider() pretool.Provider { return Provider{} }
func (Provider) ID() string         { return "vivy.exp.argument_guard" }
func (Provider) Evaluate(ctx context.Context, request pretool.Request) (pretool.Decision, error) {
	if err := ctx.Err(); err != nil {
		return pretool.Decision{}, err
	}
	if err := validate(request.ToolID, request.Arguments); err != nil {
		return pretool.Decision{Kind: pretool.Deny, ReasonCode: "experimental_argument_guard", SafeMessage: err.Error()}, nil
	}
	return pretool.Decision{Kind: pretool.Pass}, nil
}

func validate(toolID string, args json.RawMessage) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(args, &fields); err != nil || fields == nil {
		return nil
	}
	for name, raw := range fields {
		field := strings.ToLower(strings.TrimSpace(name))
		if field != "path" && field != "filepath" && field != "file_path" && field != "command" && field != "cmd" {
			continue
		}
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			continue
		}
		if strings.ContainsRune(value, '\x00') {
			return fmt.Errorf("tool argument %q: contains a NUL byte", name)
		}
		if (field == "command" || field == "cmd") && toolID != "bash" {
			lower := strings.ToLower(value)
			for _, token := range []string{"&&", "||", ";", "|", "`", "$(", "powershell", "cmd.exe"} {
				if strings.Contains(lower, token) {
					return fmt.Errorf("tool argument %q: contains blocked command syntax %q", name, token)
				}
			}
		}
		if strings.Contains(value, "..\\") || strings.Contains(value, "../") || strings.HasPrefix(value, "\\\\") {
			return fmt.Errorf("tool argument %q: path traversal or UNC paths are not allowed", name)
		}
	}
	return nil
}

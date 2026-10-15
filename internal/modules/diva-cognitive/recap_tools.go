package divacognitive

import (
	"context"
	"encoding/json"

	"agent-vivy/internal/cognitivecontract"
	"agent-vivy/internal/domain"
	"agent-vivy/internal/modules/diva-cognitive/catalog"
	"agent-vivy/internal/tools"
	"agent-vivy/sdk/module"
	toolport "agent-vivy/sdk/port/tool"
)

// A separate typed provider collection is required by the existing assembler's
// one constructor per Module. This adapter owns no state or owner lifecycle.
func NewRecapToolsModule() module.Module { return recapToolsModule{} }

type recapToolsModule struct{ ownerModule }

func (recapToolsModule) Descriptor() module.Descriptor {
	provides := []module.PortRef{}
	for _, id := range catalog.RecapToolIDs {
		provides = append(provides, module.PortRef{Port: "std/tool@v1", ID: id})
	}
	return module.Descriptor{APIVersion: module.APIVersionV1, Module: module.Identity{ID: catalog.RecapToolsModuleID, Version: "1.0.0"}, Source: module.Source{Ref: "file:internal", SHA256: zeroDigest}, Provides: provides, Requires: []module.Requirement{{PortRef: module.PortRef{Port: "core/tool-host@v1"}, Provider: "vivy/tool-host"}, {PortRef: module.PortRef{Port: Port}, Provider: ID}}, Lifecycle: module.Lifecycle{Scope: module.ScopeGeneration}}
}

type recapTool struct{ definition toolport.Definition }

func (t recapTool) Definition() toolport.Definition { return t.definition }
func (t recapTool) Invoke(ctx context.Context, host toolport.Host, raw json.RawMessage) (toolport.Result, error) {
	result, err := host.InvokeTool(ctx, t.definition.ID, raw)
	return toolport.Result{Text: result}, err
}
func RecapToolProviders() []toolport.ToolProvider {
	out := []toolport.ToolProvider{}
	for _, id := range catalog.RecapToolIDs {
		name := recapToolAction(id)
		for _, def := range recapActionDefinitions() {
			if def.ID != "vivy.recap."+name {
				continue
			}
			var input map[string]any
			_ = json.Unmarshal(def.InputSchema, &input)
			delete(input["properties"].(map[string]any), "session_id")
			required := []any{}
			for _, field := range input["required"].([]any) {
				if field != "session_id" {
					required = append(required, field)
				}
			}
			input["required"] = required
			schema, _ := json.Marshal(input)
			effect := toolport.EffectRead
			if def.Effect == "write" {
				effect = toolport.EffectWrite
			}
			out = append(out, recapTool{toolport.Definition{ID: id, Description: def.Description, Effect: effect, Schema: schema}})
		}
	}
	return out
}
func recapToolAction(id string) string {
	switch id {
	case "actmem_read":
		return "read"
	case "actmem_update":
		return "edit"
	case "actmem_clear":
		return "clear"
	case "actmem_history":
		return "history"
	case "actmem_restore":
		return "restore"
	}
	return ""
}

// Runtime implementations are per-composition references. The generated provider
// can reach only its exact host-bound implementation; there is no global Active.
type recapRuntimeTool struct {
	definition toolport.Definition
	resolve    func() cognitivecontract.Bundle
}

func NewRecapRuntimeTools(resolve func() cognitivecontract.Bundle) []tools.Tool {
	out := []tools.Tool{}
	for _, provider := range RecapToolProviders() {
		out = append(out, &recapRuntimeTool{definition: provider.Definition(), resolve: resolve})
	}
	return out
}
func (t *recapRuntimeTool) Spec() domain.ToolSpec {
	return domain.ToolSpec{Name: t.definition.ID, Description: t.definition.Description, Readonly: t.definition.Effect != toolport.EffectWrite, Exposure: domain.ToolExposureDeferred, Schema: t.definition.Schema}
}
func (t *recapRuntimeTool) owner() (dispatch, error) {
	if t.resolve == nil {
		return dispatch{}, cognitivecontract.ErrUnarmed
	}
	b, ok := t.resolve().(*bundle)
	if !ok || b == nil {
		return dispatch{}, cognitivecontract.ErrUnarmed
	}
	return dispatch{bundle: b}, nil
}
func (t *recapRuntimeTool) AuthorizeTool(ctx context.Context) error {
	d, err := t.owner()
	if err != nil {
		return err
	}
	_, err = d.authorizedRecap(ctx, tools.SessionIDFromContext(ctx), t.definition.ID)
	return err
}
func (t *recapRuntimeTool) InvokableRun(ctx context.Context, raw json.RawMessage) (string, error) {
	d, err := t.owner()
	if err != nil {
		return "", err
	}
	if err = t.AuthorizeTool(ctx); err != nil {
		return "", err
	}
	if err = strictRecapJSON(raw); err != nil {
		return "", err
	}
	if err = tools.ValidateArgs(t.Spec(), raw); err != nil {
		return "", err
	}
	var input map[string]json.RawMessage
	if err = json.Unmarshal(raw, &input); err != nil {
		return "", err
	}
	sid, _ := json.Marshal(tools.SessionIDFromContext(ctx))
	input["session_id"] = sid
	stamped, _ := json.Marshal(input)
	result, err := d.invokeRecap(ctx, "vivy.recap."+recapToolAction(t.definition.ID), stamped, t.definition.ID)
	return string(result), err
}

func IsRecapTool(id string) bool { return recapToolAction(id) != "" }

// IsRecapToolProvider identifies the compiled T1 adapter, not a claimed tool ID.
func IsRecapToolProvider(provider toolport.ToolProvider) bool {
	_, ok := provider.(recapTool)
	return ok
}

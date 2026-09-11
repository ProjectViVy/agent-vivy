package fulluimodule

import (
	"context"
	"encoding/json"

	"agent-vivy/sdk/module"
	controlaction "agent-vivy/sdk/port/controlaction"
)

const (
	ModuleID = "fixture/full-ui"
	ActionID = "fixture.full-ui.echo"
)

type owner struct{}
type instance struct{}
type actionProvider struct{}

func New() module.Module { return owner{} }

func (owner) Descriptor() module.Descriptor {
	return module.Descriptor{
		APIVersion: module.APIVersionV1,
		Module:     module.Identity{ID: ModuleID, Version: "1.0.0"},
		Provides: []module.PortRef{
			{Port: "std/ui-root@v1", ID: "fixture.full-ui.root"},
			{Port: "std/ui-extension@v1", ID: "fixture.full-ui.extension"},
			{Port: controlaction.Port, ID: ActionID},
		},
		RequestedGrants: []module.Grant{module.GrantRPCClient},
		I18N:            &module.I18N{Catalog: "i18n/catalog.json", DefaultLocale: "en", Locales: []string{"en"}},
		Lifecycle:       module.Lifecycle{Scope: module.ScopeGeneration},
	}
}

func (owner) Construct(context.Context, module.Host) (module.Instance, error) { return instance{}, nil }
func (instance) Start(context.Context) error                                  { return nil }
func (instance) Ready(context.Context) error                                  { return nil }
func (instance) Stop(context.Context) error                                   { return nil }
func (instance) Close(context.Context) error                                  { return nil }

func NewProvider() controlaction.Provider { return actionProvider{} }

func (actionProvider) Definition() controlaction.Definition {
	return controlaction.Definition{
		ID:           ActionID,
		Owner:        ModuleID,
		ModuleID:     ModuleID,
		Description:  "Echo one bounded fixture message",
		Effect:       controlaction.EffectRead,
		InputSchema:  json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"message":{"type":"string","minLength":1,"maxLength":128}},"required":["message"]}`),
		ResultSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"accepted":{"type":"boolean"},"message":{"type":"string"}},"required":["accepted","message"]}`),
	}
}

func (actionProvider) Invoke(ctx context.Context, _ controlaction.Host, input json.RawMessage) (json.RawMessage, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	var request struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(input, &request); err != nil {
		return nil, controlaction.ErrInvalidInput
	}
	return json.Marshal(struct {
		Accepted bool   `json:"accepted"`
		Message  string `json:"message"`
	}{Accepted: true, Message: request.Message})
}

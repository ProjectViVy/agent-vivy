// Package localllm is the local-LLM compatibility Module (VCP-I1).
//
// It contributes local OpenAI-compatible model servers — llama.cpp's
// llama-server, Ollama, LM Studio, and vLLM — to the sealed Vivy body
// without any kernel or wire-protocol change: every server speaks the
// already-sealed openai-completions adapter, so selection rides the
// ordinary settings provider/base_url path.
//
// The Module owns two public Ports:
//
//   - std/control-action@v1 `local_llm.manage`: one op-dispatched action
//     (status | discover | models | start | stop | pull) for lifecycle
//     control. External modules bind exactly one provider constructor per
//     Port and every action provider carries a single Definition, so the
//     six lifecycle verbs compose into one action rather than six module
//     entries — the control surface is identical.
//   - std/status-source@v1 `vivy.local-llm.status`: a read-only snapshot
//     of the last discovery result plus supervised-process state. Status
//     reads never probe or start anything; they report the cached truth
//     from the most recent discover/status op.
//
// No std/provider-profile is contributed: an external Module's single
// NewProvider() cannot implement both providerprofile.Provider and
// controlaction.Provider (colliding Definition() signatures), and a
// declarative profile carries no endpoint URL, so it could not drive
// model selection anyway — the settings provider registry already owns
// that path (see the story log for the boundary note).
package localllm

import (
	"context"

	"agent-vivy/sdk/module"
)

const (
	// ModuleID is this Module's identity; it must match vivy-module.yaml.
	ModuleID = "vivy/local-llm"
	// ActionID is the sole control action.
	ActionID = "local_llm.manage"
	// StatusSourceID is the read-only status source identity.
	StatusSourceID = "vivy.local-llm.status"
)

type owner struct{}

// New returns the Module owner.
func New() module.Module { return owner{} }

// NewProvider returns the Module's provider: one value implementing both
// controlaction.Provider and status.Provider.
func NewProvider() *Provider { return newProvider() }

func (owner) Descriptor() module.Descriptor {
	return module.Descriptor{
		APIVersion: module.APIVersionV1,
		Module:     module.Identity{ID: ModuleID, Version: "0.1.0"},
		Source:     module.Source{Ref: "repo:plugins/infra/llm", SHA256: "9a8e049ac4d8a1ddc673b28ae492a6401639bd73916d31f74d1bf2688e13a90d"},
		Provides: []module.PortRef{
			{Port: "std/control-action@v1", ID: ActionID},
			{Port: "std/status-source@v1", ID: StatusSourceID},
		},
		Requires: []module.Requirement{
			{PortRef: module.PortRef{Port: "core/action-host@v1"}, Provider: "vivy/action-host"},
			{PortRef: module.PortRef{Port: "core/status-host@v1"}, Provider: "vivy/status-host"},
		},
		RequestedGrants: []module.Grant{module.GrantNetClient},
		Lifecycle:       module.Lifecycle{Scope: module.ScopeGeneration},
	}
}

func (owner) Construct(context.Context, module.Host) (module.Instance, error) {
	return instance{}, nil
}

type instance struct{}

func (instance) Start(context.Context) error { return nil }
func (instance) Ready(context.Context) error { return nil }
func (instance) Stop(context.Context) error  { return nil }
func (instance) Close(context.Context) error { return nil }

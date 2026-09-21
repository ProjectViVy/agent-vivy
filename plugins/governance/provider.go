package governance

import (
	"context"

	"agent-vivy/sdk/module"
	"agent-vivy/sdk/port/observer"
	"agent-vivy/sdk/port/pretool"
	statusport "agent-vivy/sdk/port/status"
)

type vivyModule struct{}
type moduleInstance struct{}

func New() module.Module { return vivyModule{} }

func (vivyModule) Descriptor() module.Descriptor {
	return module.Descriptor{
		APIVersion: module.APIVersionV1,
		Module:     module.Identity{ID: "vivy/governance-reference", Version: "0.1.0"},
		Source:     module.Source{Ref: "repo:plugins/governance", SHA256: "8e0d6e287288eff1e0a81a9458cd5e04e3e97d8c39589c29b749cb741eefa5b0"},
		Provides: []module.PortRef{
			{Port: "std/middleware/pre-tool@v1", ID: "vivy.governance.pre-tool"},
			{Port: "std/observer/run@v1", ID: "vivy.governance.run"},
			{Port: "std/observer/diagnostic@v1", ID: "vivy.governance.diagnostic"},
			{Port: "std/status-source@v1", ID: "vivy.governance.status"},
		},
		Requires: []module.Requirement{
			{PortRef: module.PortRef{Port: "core/tool-host@v1"}, Provider: "vivy/tool-host"},
			{PortRef: module.PortRef{Port: "core/observer-host@v1"}, Provider: "vivy/observer-host"},
			{PortRef: module.PortRef{Port: "core/status-host@v1"}, Provider: "vivy/status-host"},
		},
		Lifecycle: module.Lifecycle{Scope: module.ScopeGeneration},
	}
}

func (vivyModule) Construct(context.Context, module.Host) (module.Instance, error) {
	return moduleInstance{}, nil
}

func (moduleInstance) Start(context.Context) error { return nil }
func (moduleInstance) Ready(context.Context) error { return nil }
func (moduleInstance) Stop(context.Context) error  { return nil }
func (moduleInstance) Close(context.Context) error { return nil }

// Provider is a no-side-effect reference implementation for the public P3
// governance Ports. It is shipped for conformance recipes and is not selected
// by the default generation.
type Provider struct{}

func NewProvider() *Provider { return &Provider{} }

func (*Provider) ID() string { return "vivy.governance.reference" }

func (*Provider) Evaluate(context.Context, pretool.Request) (pretool.Decision, error) {
	return pretool.Decision{Kind: pretool.Pass}, nil
}

func (*Provider) ObserveRun(context.Context, observer.RunEvent) error { return nil }

func (*Provider) ObserveDiagnostic(context.Context, observer.Diagnostic) {}

func (*Provider) Status(context.Context, statusport.Request) (statusport.Snapshot, error) {
	return statusport.NewSnapshot("reference-v1", "", []statusport.Item{{ID: "provider", State: "ready"}}), nil
}

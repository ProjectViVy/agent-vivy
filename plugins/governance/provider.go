package governance

import (
	"context"

	"agent-vivy/sdk/port/observer"
	"agent-vivy/sdk/port/pretool"
	statusport "agent-vivy/sdk/port/status"
)

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

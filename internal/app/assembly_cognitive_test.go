package app

import (
	"context"
	"path/filepath"
	"testing"

	"agent-vivy/internal/cognitivecontract"
	"agent-vivy/internal/config"
	"agent-vivy/internal/domain"
	genassembly "agent-vivy/internal/generated/assembly"
	"agent-vivy/internal/observerhost"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/sdk/generation"
	"agent-vivy/sdk/port/observer"

	laputaevolution "github.com/ProjectViVy/laputa/laputa/evolution"
)

func genassemblyForCognitive(modules []string) *genassembly.RuntimeAssembly {
	return &genassembly.RuntimeAssembly{Manifest: generation.Manifest{Modules: modules}}
}

func TestCognitiveBundleForAssemblyIsAbsentWhenCapabilityIsOmitted(t *testing.T) {
	assembly := genassemblyForCognitive([]string{"vivy/storage"})
	bundle, err := cognitiveBundleForAssembly(context.Background(), assembly, config.Config{}, "generation-1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if bundle != nil {
		t.Fatal("cognitive bundle was constructed for an omitted capability")
	}
}

func TestCognitiveBundleForAssemblyFailsClosedWithoutTypedFactory(t *testing.T) {
	// An unpopulated Assembly carries a nil factory binding —
	// a selected module whose binding never emitted the factory fails init
	// instead of degrading to a second construction path.
	assembly := genassemblyForCognitive([]string{"vivy/diva-cognitive"})
	if _, err := cognitiveBundleForAssembly(context.Background(), assembly, config.Config{}, "generation-1", nil); err == nil {
		t.Fatal("selected cognitive capability without a typed factory was accepted")
	}
}

type cognitiveRunObserver struct{}

func (cognitiveRunObserver) ID() string { return "fixture.cognitive" }
func (cognitiveRunObserver) ObserveRun(context.Context, observer.RunEvent) error {
	return nil
}

func TestObserverHostCompositionAppendsOwnedSubscriptions(t *testing.T) {
	backend, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "obs.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = backend.Close() })

	// An owned subscription without the compiled ObserverHost fails.
	assembly := genassemblyForCognitive([]string{"vivy/storage"})
	if _, err := observerHostForAssembly(context.Background(), *assembly, backend,
		ownedSubscription()); err == nil {
		t.Fatal("owned capture subscription without ObserverHost was accepted")
	}

	// With the host compiled, the owned subscription composes — one durable
	// cursor lane, no unsealed late Subscribe path.
	assembly = genassemblyForCognitive([]string{"vivy/observer-host"})
	host, err := observerHostForAssembly(context.Background(), *assembly, backend,
		ownedSubscription())
	if err != nil {
		t.Fatal(err)
	}
	if host == nil {
		t.Fatal("observer host missing for an owned capture subscription")
	}
	host.Close()
}

func ownedSubscription() observerhost.RunSubscription {
	return observerhost.RunSubscription{Provider: cognitiveRunObserver{}, EventTypes: []string{"run.completed"}, AllowedPayloadFields: []string{"outcome"}}
}

// ControlPort wiring: the armed cell drives state/CAS/trigger/cancel
// against the runtime service seam. The fake service-side dependency is the
// durable snapshot store — the real callbacks live on runtime.Service.
func TestControlPortMapsRuntimeSeams(t *testing.T) {
	bundle := &fakeBundle{policy: laputaevolution.TriggerPolicy{Enabled: true, MinIntervalMS: 60_000}}
	port := &cognitiveControlPort{svc: nil, bundle: bundle}
	if _, err := port.GetState(context.Background()); err == nil {
		t.Fatal("control port reported state without a service")
	}
	if _, err := port.Cancel(context.Background(), domain.RunID("run-1")); err == nil {
		t.Fatal("control port cancelled without a service")
	}
}

// fakeBundle satisfies the bundle seam for the control-port mapping test;
// the real bundle is exercised end-to-end in the module's factory tests.
type fakeBundle struct {
	policy laputaevolution.TriggerPolicy
}

func (b *fakeBundle) Prepare(context.Context, cognitivecontract.PrimaryContextInput) (cognitivecontract.PreparedPrimaryContext, error) {
	return cognitivecontract.PreparedPrimaryContext{}, nil
}
func (b *fakeBundle) ResolveBinding(context.Context) (laputaevolution.RunBinding, error) {
	return laputaevolution.RunBinding{SubjectID: "diva", DestinationID: "mentle"}, nil
}
func (b *fakeBundle) BoundDomain(context.Context, laputaevolution.RunBinding) (laputaevolution.Domain, error) {
	return nil, nil
}
func (b *fakeBundle) SourceID() string                         { return "diva/personal/mentle" }
func (b *fakeBundle) Source() cognitivecontract.Source         { return nil }
func (b *fakeBundle) Sink() cognitivecontract.CaptureSink      { return nil }
func (b *fakeBundle) Mission() cognitivecontract.MissionSource { return nil }
func (b *fakeBundle) Policy() laputaevolution.TriggerPolicy    { return b.policy }
func (b *fakeBundle) AttachRuntime(cognitivecontract.ControlPort) error {
	return nil
}
func (b *fakeBundle) Close() error { return nil }

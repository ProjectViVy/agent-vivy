package app

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"agent-vivy/internal/config"
	"agent-vivy/internal/domain"
	genassembly "agent-vivy/internal/generated/assembly"
	"agent-vivy/internal/observerhost"
	"agent-vivy/internal/runtime"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/sdk/generation"
	"agent-vivy/sdk/port/observer"
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
	port := &cognitiveControlPort{svc: nil}
	if _, err := port.GetState(context.Background()); err == nil {
		t.Fatal("control port reported state without a service")
	}
	if _, err := port.Cancel(context.Background(), domain.RunID("run-1")); err == nil {
		t.Fatal("control port cancelled without a service")
	}
}

func TestControlPortUsesCoherentSnapshotAndScopedCancel(t *testing.T) {
	backend, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "control.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = backend.Close() })
	ctx := context.Background()
	snapshots := backend.Snapshot()
	if err := snapshots.Put(ctx, "cognitive/state", []byte(`{"state_schema":2,"phase":"running","active_run_id":"run-7","pending_through":12,"watermark":8,"policy":{"enabled":true,"min_interval_ms":900},"policy_revision":4,"blocked":"cancelled"}`), 0); err != nil {
		t.Fatal(err)
	}
	svc := runtime.NewService(nil, "", "", runtime.ServiceDeps{
		Cognitive:         &runtime.CognitiveBinding{Store: snapshots, SourceID: "app-source"},
		Runs:              backend,
		WorkflowRevisions: backend,
	})
	port := &cognitiveControlPort{svc: svc}
	state, err := port.GetState(ctx)
	if err != nil {
		t.Fatalf("control state: %v", err)
	}
	if !state.Enabled || state.MinIntervalMS != 900 || state.PolicyRevision != 4 || state.SourceID != "app-source" ||
		state.ActiveRunID != "run-7" || state.Watermark != 8 || state.PendingThrough != 12 ||
		state.Phase != "running" || state.BlockReason != "cancelled" {
		t.Fatalf("control projection = %+v", state)
	}
	if _, err := port.Cancel(ctx, domain.RunID("foreground-run")); !errors.Is(err, runtime.ErrCognitiveRunMismatch) {
		t.Fatalf("unrelated cancellation = %v", err)
	}
}

package app

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"

	"agent-vivy/internal/cognitivecontract"
	"agent-vivy/internal/config"
	"agent-vivy/internal/domain"
	genassembly "agent-vivy/internal/generated/assembly"
	storagemodule "agent-vivy/internal/modules/storage"
	"agent-vivy/internal/observerhost"
	"agent-vivy/internal/runtime"
	"agent-vivy/internal/storage"
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

type compositionCognitiveBundle struct {
	closeErr              error
	resolveErr            error
	attachErr             error
	closeCalls            int
	attachCalls           int
	backendClosed         func() bool
	closeSawBackendClosed bool
}

func (b *compositionCognitiveBundle) Prepare(context.Context, cognitivecontract.PrimaryContextInput) (cognitivecontract.PreparedPrimaryContext, error) {
	return cognitivecontract.PreparedPrimaryContext{}, nil
}
func (b *compositionCognitiveBundle) ResolveBinding(context.Context, laputaevolution.TriggerPolicy) (laputaevolution.RunBinding, error) {
	if b.resolveErr != nil {
		return laputaevolution.RunBinding{}, b.resolveErr
	}
	return laputaevolution.RunBinding{SubjectID: "subject", WorkspaceID: "workspace", DestinationID: "mentle"}, nil
}
func (b *compositionCognitiveBundle) BoundDomain(context.Context, laputaevolution.RunBinding) (laputaevolution.Domain, error) {
	return embeddedCognitiveDomain{}, nil
}
func (*compositionCognitiveBundle) SourceID() string { return "fixture.source" }
func (*compositionCognitiveBundle) Source() cognitivecontract.Source {
	return compositionCognitiveSource{}
}
func (*compositionCognitiveBundle) Sink() cognitivecontract.CaptureSink {
	return compositionCognitiveSink{}
}
func (*compositionCognitiveBundle) Mission() cognitivecontract.MissionSource {
	return compositionCognitiveMission{}
}
func (*compositionCognitiveBundle) Policy() laputaevolution.TriggerPolicy {
	return laputaevolution.TriggerPolicy{Enabled: true}
}
func (b *compositionCognitiveBundle) AttachRuntime(cognitivecontract.ControlPort) error {
	b.attachCalls++
	return b.attachErr
}
func (b *compositionCognitiveBundle) Close() error {
	b.closeCalls++
	if b.backendClosed != nil {
		b.closeSawBackendClosed = b.backendClosed()
	}
	return b.closeErr
}

type compositionCognitiveSource struct{}

func (compositionCognitiveSource) HighWatermark(context.Context) (uint64, error) { return 0, nil }

type compositionCognitiveMission struct{}

func (compositionCognitiveMission) MissionRevision(context.Context) (uint64, error) { return 0, nil }

type compositionCognitiveSink struct{}

func (compositionCognitiveSink) Capture(context.Context, cognitivecontract.Capture) (cognitivecontract.CaptureReceipt, error) {
	return cognitivecontract.CaptureReceipt{Status: "accepted"}, nil
}

type compositionCognitiveDispatchBundle struct{ *compositionCognitiveBundle }

func (compositionCognitiveDispatchBundle) Dispatcher() cognitivecontract.Dispatcher {
	return compositionCognitiveDispatcher{}
}

type compositionCognitiveDispatcher struct{}

func (compositionCognitiveDispatcher) Invoke(context.Context, string, json.RawMessage) (json.RawMessage, error) {
	return nil, errors.New("fixture dispatcher is not used during composition")
}

func compositionAssembly(bundle cognitivecontract.Bundle) genassembly.RuntimeAssembly {
	assembly := genassembly.BuildDefault()
	assembly.CognitiveFactory = func(context.Context, cognitivecontract.FactoryInput) (cognitivecontract.Bundle, error) {
		return bundle, nil
	}
	return assembly
}

func minimalCognitiveAdmissionAssembly(bundle cognitivecontract.Bundle) genassembly.RuntimeAssembly {
	return genassembly.RuntimeAssembly{
		GenerationID: "fixture-generation",
		CognitiveFactory: func(context.Context, cognitivecontract.FactoryInput) (cognitivecontract.Bundle, error) {
			return bundle, nil
		},
		Manifest: generation.Manifest{
			Modules: []string{"vivy/diva-cognitive", "vivy/observer-host"},
			Face:    "kernel-headless",
		},
	}
}

func withoutCognitiveObserverHost(assembly genassembly.RuntimeAssembly) genassembly.RuntimeAssembly {
	modules := assembly.Manifest.Modules[:0]
	for _, id := range assembly.Manifest.Modules {
		if id != "vivy/observer-host" {
			modules = append(modules, id)
		}
	}
	assembly.Manifest.Modules = modules
	assembly.RunObservers = nil
	assembly.Manifest.RunObservers = nil
	assembly.Manifest.RunObserverPolicies = nil
	return assembly
}

func TestCognitiveBundleClosedOnEveryCompositionFailure(t *testing.T) {
	tests := []struct {
		name        string
		assembly    func(cognitivecontract.Bundle) genassembly.RuntimeAssembly
		bundle      func() *compositionCognitiveBundle
		wantAttach  int
		wantMessage string
	}{
		{
			name: "observer host construction",
			assembly: func(bundle cognitivecontract.Bundle) genassembly.RuntimeAssembly {
				return withoutCognitiveObserverHost(compositionAssembly(bundle))
			},
			bundle:      func() *compositionCognitiveBundle { return &compositionCognitiveBundle{} },
			wantMessage: "without compiled ObserverHost",
		},
		{
			name:     "binding resolution",
			assembly: compositionAssembly,
			bundle: func() *compositionCognitiveBundle {
				return &compositionCognitiveBundle{resolveErr: errors.New("binding resolution failed")}
			},
			wantMessage: "binding resolution failed",
		},
		{
			name:     "control attachment",
			assembly: compositionAssembly,
			bundle: func() *compositionCognitiveBundle {
				return &compositionCognitiveBundle{attachErr: errors.New("control attachment failed")}
			},
			wantAttach:  1,
			wantMessage: "control attachment failed",
		},
		{
			name:        "action dispatcher binding",
			assembly:    compositionAssembly,
			bundle:      func() *compositionCognitiveBundle { return &compositionCognitiveBundle{} },
			wantAttach:  1,
			wantMessage: "does not expose the action dispatcher",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runtime.SetEngineVersionOverride(pinnedEinoVersion)
			t.Cleanup(func() { runtime.SetEngineVersionOverride("") })
			cfg := newDeepSeekTestConfig(t)
			for attempt := 0; attempt < 2; attempt++ {
				bundle := test.bundle()
				_, err := NewWithAssembly(context.Background(), cfg, test.assembly(bundle),
					WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))), WithoutEars(), WithoutGateway())
				if err == nil || !strings.Contains(err.Error(), test.wantMessage) {
					t.Fatalf("attempt %d error=%v, want message containing %q", attempt+1, err, test.wantMessage)
				}
				if bundle.closeCalls != 1 {
					t.Fatalf("attempt %d bundle Close calls=%d, want 1", attempt+1, bundle.closeCalls)
				}
				if bundle.attachCalls != test.wantAttach {
					t.Fatalf("attempt %d AttachRuntime calls=%d, want %d", attempt+1, bundle.attachCalls, test.wantAttach)
				}
			}
		})
	}
	t.Run("primary admission", func(t *testing.T) {
		runtime.SetEngineVersionOverride(pinnedEinoVersion)
		t.Cleanup(func() { runtime.SetEngineVersionOverride("") })
		previousOpen := openStorageBackend
		var lastBackend *cognitiveAdmissionMissingBackend
		openStorageBackend = func(ctx context.Context, cfg config.Config) (storage.Engine, error) {
			backend, err := storagemodule.Open(ctx, cfg)
			if err != nil {
				return nil, err
			}
			lastBackend = &cognitiveAdmissionMissingBackend{
				Engine: backend, WorkStore: backend.(storage.WorkStore),
				GoalRunStore: backend.(storage.GoalRunStore), PrimaryRunStore: backend.(storage.PrimaryRunStore),
			}
			return lastBackend, nil
		}
		t.Cleanup(func() { openStorageBackend = previousOpen })
		cfg := newDeepSeekTestConfig(t)
		for attempt := 0; attempt < 2; attempt++ {
			lastBackend = nil
			bundle := &compositionCognitiveBundle{backendClosed: func() bool { return lastBackend != nil && lastBackend.closed }}
			_, err := NewWithAssembly(context.Background(), cfg, minimalCognitiveAdmissionAssembly(bundle),
				WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))), WithoutEars(), WithoutGateway())
			if err == nil || !strings.Contains(err.Error(), "requires Core Storage RunAdmissionStore") {
				t.Fatalf("attempt %d error=%v, want primary admission failure", attempt+1, err)
			}
			if bundle.closeCalls != 1 {
				t.Fatalf("attempt %d bundle Close calls=%d, want 1", attempt+1, bundle.closeCalls)
			}
			if bundle.closeSawBackendClosed {
				t.Fatalf("attempt %d closed storage before releasing the cognitive owner", attempt+1)
			}
			if lastBackend == nil || !lastBackend.closed {
				t.Fatalf("attempt %d did not close the failed composition storage", attempt+1)
			}
		}
	})
}

type cognitiveAdmissionMissingBackend struct {
	storage.Engine
	storage.WorkStore
	storage.GoalRunStore
	storage.PrimaryRunStore
	closed bool
}

func (b *cognitiveAdmissionMissingBackend) Close() error {
	b.closed = true
	return b.Engine.Close()
}

func TestCognitiveBundleFailurePreservesCleanupError(t *testing.T) {
	runtime.SetEngineVersionOverride(pinnedEinoVersion)
	t.Cleanup(func() { runtime.SetEngineVersionOverride("") })
	cfg := newDeepSeekTestConfig(t)
	factoryErr := errors.New("binding resolution sentinel")
	closeErr := errors.New("bundle close sentinel")
	bundle := &compositionCognitiveBundle{resolveErr: factoryErr, closeErr: closeErr}
	_, err := NewWithAssembly(context.Background(), cfg, compositionAssembly(bundle),
		WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))), WithoutEars(), WithoutGateway())
	if !errors.Is(err, factoryErr) || !errors.Is(err, closeErr) {
		t.Fatalf("composition error=%v, want both binding and cleanup causes", err)
	}
	if bundle.closeCalls != 1 {
		t.Fatalf("bundle Close calls=%d, want 1", bundle.closeCalls)
	}
}

func TestCognitiveBundleOwnershipTransfersOnce(t *testing.T) {
	runtime.SetEngineVersionOverride(pinnedEinoVersion)
	t.Cleanup(func() { runtime.SetEngineVersionOverride("") })
	cfg := newDeepSeekTestConfig(t)
	bundle := &compositionCognitiveBundle{}
	app, err := NewWithAssembly(context.Background(), cfg, compositionAssembly(compositionCognitiveDispatchBundle{bundle}),
		WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))), WithoutEars(), WithoutGateway())
	if err != nil {
		t.Fatal(err)
	}
	if bundle.closeCalls != 0 {
		t.Fatalf("bundle closed before ownership transfer: %d", bundle.closeCalls)
	}
	if bundle.attachCalls != 1 {
		t.Fatalf("AttachRuntime calls=%d, want 1", bundle.attachCalls)
	}
	if err := app.Close(); err != nil {
		t.Fatalf("App.Close: %v", err)
	}
	if err := app.Close(); err != nil {
		t.Fatalf("second App.Close: %v", err)
	}
	if bundle.closeCalls != 1 {
		t.Fatalf("bundle Close calls after App.Close=%d, want 1", bundle.closeCalls)
	}
}

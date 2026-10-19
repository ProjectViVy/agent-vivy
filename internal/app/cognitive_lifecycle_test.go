package app

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	laputaevolution "github.com/ProjectViVy/laputa/laputa/evolution"

	"agent-vivy/internal/config"
	"agent-vivy/internal/runtime"
	"agent-vivy/internal/storage/sqlite"
)

// lifecycleCognitiveDomain is a no-op bound Domain so the wake loop's
// admission gate opens; with no committed activity it stays ineligible.
type lifecycleCognitiveDomain struct{}

func (lifecycleCognitiveDomain) Collect(context.Context, laputaevolution.Window) (laputaevolution.EvidenceBatch, error) {
	return laputaevolution.EvidenceBatch{}, nil
}
func (lifecycleCognitiveDomain) Apply(context.Context, laputaevolution.Effect) (laputaevolution.EffectReceipt, error) {
	return laputaevolution.EffectReceipt{}, &laputaevolution.ContractError{Code: laputaevolution.ErrEffectNotFound}
}
func (lifecycleCognitiveDomain) Lookup(context.Context, string) (laputaevolution.EffectReceipt, error) {
	return laputaevolution.EffectReceipt{}, &laputaevolution.ContractError{Code: laputaevolution.ErrEffectNotFound}
}

// TestRunOwnsCognitiveLifecycle proves App.Run starts the cognitive wake loop
// and stops admission before draining the app on cancellation.
func TestRunOwnsCognitiveLifecycle(t *testing.T) {
	backend, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "lifecycle.db"))
	if err != nil {
		t.Fatal(err)
	}
	svc := runtime.NewService(nil, "test", "test-model", runtime.ServiceDeps{
		Cognitive: &runtime.CognitiveBinding{
			Domain:   lifecycleCognitiveDomain{},
			Store:    backend.Snapshot(),
			SourceID: "activity",
			Binding: laputaevolution.RunBinding{
				SubjectID: "profile-1", WorkspaceID: "ws-1", DestinationID: "mentle",
			},
			Policy: laputaevolution.TriggerPolicy{Enabled: true},
		},
	})
	a := &App{
		cfg:     config.Config{},
		service: svc,
		backend: backend,
		logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	runCtx, cancel := context.WithCancel(context.Background())
	runDone := make(chan error, 1)
	go func() { runDone <- a.Run(runCtx) }()
	waitFor(t, 2*time.Second, svc.CognitiveLoopActive)
	cancel()
	select {
	case err := <-runDone:
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
	case <-time.After(shutdownGrace + 10*time.Second):
		t.Fatal("Run did not stop after context cancellation")
	}
	if svc.CognitiveLoopActive() {
		t.Fatal("Run left the cognitive loop running")
	}
}

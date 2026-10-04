package app

import (
	"context"
	"path/filepath"
	"testing"

	laputaevolution "github.com/dashimaki/laputa/evolution"

	"agent-vivy/internal/config"
	"agent-vivy/internal/runtime"
	"agent-vivy/internal/storage/sqlite"
)

// embeddedCognitiveDomain is a no-op bound Domain so the wake loop's
// admission gate opens; with no committed activity it stays ineligible.
type embeddedCognitiveDomain struct{}

func (embeddedCognitiveDomain) Collect(context.Context, laputaevolution.Window) (laputaevolution.EvidenceBatch, error) {
	return laputaevolution.EvidenceBatch{}, nil
}
func (embeddedCognitiveDomain) Apply(context.Context, laputaevolution.Effect) (laputaevolution.EffectReceipt, error) {
	return laputaevolution.EffectReceipt{}, &laputaevolution.ContractError{Code: laputaevolution.ErrEffectNotFound}
}
func (embeddedCognitiveDomain) Lookup(context.Context, string) (laputaevolution.EffectReceipt, error) {
	return laputaevolution.EffectReceipt{}, &laputaevolution.ContractError{Code: laputaevolution.ErrEffectNotFound}
}

// TestEmbeddedCognitionStartsOnce proves the embedded lifecycle owner starts
// the cognitive wake loop exactly once (repeated StartEmbeddedServices stay
// idempotent) and that Close stops admission before drains — matching the
// ordering Run's shutdown already enforces.
func TestEmbeddedCognitionStartsOnce(t *testing.T) {
	backend, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "embedded.db"))
	if err != nil {
		t.Fatal(err)
	}
	svc := runtime.NewService(nil, "test", "test-model", runtime.ServiceDeps{
		Cognitive: &runtime.CognitiveBinding{
			Domain:   embeddedCognitiveDomain{},
			Store:    backend.Snapshot(),
			SourceID: "activity",
			Binding: laputaevolution.RunBinding{
				SubjectID: "profile-1", WorkspaceID: "ws-1", DestinationID: "mentle",
			},
			Policy: laputaevolution.TriggerPolicy{Enabled: true},
		},
	})
	a := &App{cfg: config.Config{}, service: svc, backend: backend}

	a.StartEmbeddedServices()
	if !svc.CognitiveLoopActive() {
		t.Fatal("embedded start did not start the cognitive loop")
	}
	a.StartEmbeddedServices()
	if !svc.CognitiveLoopActive() {
		t.Fatal("second embedded start closed the loop")
	}

	if err := a.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if svc.CognitiveLoopActive() {
		t.Fatal("close left the cognitive loop running")
	}
	// Close is idempotent; cognition never restarts.
	if err := a.Close(); err != nil {
		t.Fatalf("second close: %v", err)
	}
}

package divacognitive

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"agent-vivy/internal/cognitivecontract"
	"agent-vivy/internal/config"
	"agent-vivy/internal/domain"
	controlaction "agent-vivy/sdk/port/controlaction"

	laputaevolution "github.com/dashimaki/laputa/evolution"
)

func testFactoryInput(t *testing.T) cognitivecontract.FactoryInput {
	t.Helper()
	return cognitivecontract.FactoryInput{
		Config:       config.Config{Storage: config.Storage{DataDir: t.TempDir()}},
		GenerationID: "generation-test",
	}
}

func openBundle(t *testing.T) (context.Context, cognitivecontract.Bundle) {
	t.Helper()
	ctx := context.Background()
	bundle, err := Open(ctx, testFactoryInput(t))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = bundle.Close() })
	return ctx, bundle
}

type fakeControl struct {
	state cognitivecontract.ControlState
}

func (f *fakeControl) GetState(context.Context) (cognitivecontract.ControlState, error) {
	return f.state, nil
}
func (f *fakeControl) SetPolicyCAS(context.Context, laputaevolution.TriggerPolicy, uint64) (cognitivecontract.ControlState, error) {
	return f.state, nil
}
func (f *fakeControl) Trigger(context.Context) (cognitivecontract.ControlState, error) {
	return f.state, nil
}
func (f *fakeControl) Cancel(context.Context, domain.RunID) (cognitivecontract.ControlState, error) {
	return f.state, nil
}

func TestOpenRejectsRelativeDataDirectory(t *testing.T) {
	in := testFactoryInput(t)
	in.Config.Storage.DataDir = "relative/dir"
	if _, err := Open(context.Background(), in); err == nil {
		t.Fatal("factory accepted a non-absolute host data directory")
	}
}

func TestOpenArmsSourceSinkMissionWithoutBackend(t *testing.T) {
	ctx, bundle := openBundle(t)
	if bundle.SourceID() == "" {
		t.Fatal("armed bundle has no source identity")
	}
	if _, err := bundle.Source().HighWatermark(ctx); err != nil {
		t.Fatalf("source watermark: %v", err)
	}
	if _, err := bundle.Mission().MissionRevision(ctx); err != nil {
		t.Fatalf("mission revision: %v", err)
	}
	binding, err := bundle.ResolveBinding(ctx)
	if err != nil {
		t.Fatalf("ResolveBinding: %v", err)
	}
	if binding.SubjectID != profileID || binding.DestinationID != destinationID {
		t.Fatalf("binding = %#v", binding)
	}
	if binding.StrategyDigest == "" {
		t.Fatal("binding does not pin the trusted strategy digest")
	}
}

func TestCaptureIsDurableAndDeduped(t *testing.T) {
	ctx, bundle := openBundle(t)
	capture := cognitivecontract.Capture{
		SessionID: "session-a", RunID: "run-a", EventID: "run-a:3",
		Phase: "completed", Content: `{"outcome":"completed"}`, OccurredAt: 1700000000000,
	}
	first, err := bundle.Sink().Capture(ctx, capture)
	if err != nil {
		t.Fatalf("capture: %v", err)
	}
	if first.Seq == 0 || first.IngestionID == "" {
		t.Fatalf("receipt = %#v", first)
	}
	again, err := bundle.Sink().Capture(ctx, capture)
	if err != nil {
		t.Fatalf("redelivery: %v", err)
	}
	if again.Seq != first.Seq || again.IngestionID != first.IngestionID {
		t.Fatalf("deduped redelivery minted a new record: %#v vs %#v", again, first)
	}
	if _, err := bundle.Sink().Capture(ctx, cognitivecontract.Capture{
		SessionID: "session-a", RunID: "run-b", EventID: "run-b:1", Phase: "nonsense",
	}); err == nil {
		t.Fatal("unknown capture phase accepted")
	}
}

func TestBoundDomainRejectsForeignBinding(t *testing.T) {
	ctx, bundle := openBundle(t)
	binding, err := bundle.ResolveBinding(ctx)
	if err != nil {
		t.Fatal(err)
	}
	binding.WorkspaceID = "someone-else"
	if _, err := bundle.BoundDomain(ctx, binding); err == nil {
		t.Fatal("foreign binding admitted")
	}
}

func TestAttachRuntimeIsSingleUse(t *testing.T) {
	_, bundle := openBundle(t)
	if err := bundle.AttachRuntime(&fakeControl{}); err != nil {
		t.Fatalf("first attach: %v", err)
	}
	if err := bundle.AttachRuntime(&fakeControl{}); err == nil {
		t.Fatal("second attach replaced the armed cell")
	}
}

func TestDispatcherFailsClosedWhenUnarmed(t *testing.T) {
	ctx, bundle := openBundle(t)
	dispatcher := bundle.(cognitivecontract.DispatcherProvider).Dispatcher()
	if _, err := dispatcher.Invoke(ctx, ActionStatus, json.RawMessage(`{"session_id":"s"}`)); !errors.Is(err, cognitivecontract.ErrUnarmed) {
		t.Fatalf("unarmed dispatch error = %v", err)
	}
}

func TestDispatcherRoutesArmedControlActions(t *testing.T) {
	ctx, bundle := openBundle(t)
	if err := bundle.AttachRuntime(&fakeControl{state: cognitivecontract.ControlState{Enabled: true, SourceID: "src"}}); err != nil {
		t.Fatal(err)
	}
	dispatcher := bundle.(cognitivecontract.DispatcherProvider).Dispatcher()
	raw, err := dispatcher.Invoke(ctx, ActionStatus, json.RawMessage(`{"session_id":"s"}`))
	if err != nil {
		t.Fatalf("status invoke: %v", err)
	}
	var state cognitivecontract.ControlState
	if err := json.Unmarshal(raw, &state); err != nil || !state.Enabled {
		t.Fatalf("status payload = %s err=%v", raw, err)
	}
	if _, err := dispatcher.Invoke(ctx, ActionMemorySearch, json.RawMessage(`{"session_id":"s","query":"x"}`)); err == nil {
		t.Fatal("unhandled action reported success")
	}
}

func TestActionProvidersInventoryMatchesContract(t *testing.T) {
	providers := ActionProviders()
	if len(providers) != len(ActionIDs) {
		t.Fatalf("providers = %d, want %d", len(providers), len(ActionIDs))
	}
	for i, provider := range providers {
		def := provider.Definition()
		if def.ID != ActionIDs[i] || def.ModuleID != ID || def.Owner != ID {
			t.Fatalf("definition %d = %#v", i, def)
		}
		if def.MaxInputBytes != maxCognitiveInput || def.MaxOutputBytes != maxCognitiveOutput {
			t.Fatalf("definition %d bounds = %#v", i, def)
		}
	}
	// The sealed manifest must equal the emitted ActionSets exactly — a
	// provider invoke against a host without the private facade fails closed.
	for _, provider := range providers {
		if _, err := provider.Invoke(context.Background(), fakeHost{}, json.RawMessage(`{}`)); !errors.Is(err, cognitivecontract.ErrUnarmed) {
			t.Fatalf("%s against foreign host = %v", provider.Definition().ID, err)
		}
	}
}

type fakeHost struct{ controlaction.Host }

func (fakeHost) ModuleID() string { return "fixture" }

package divacognitive

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"testing"

	"agent-vivy/internal/cognitivecontract"
	"agent-vivy/internal/config"
	"agent-vivy/internal/domain"
	controlaction "agent-vivy/sdk/port/controlaction"

	laputaevolution "github.com/ProjectViVy/laputa/laputa/evolution"
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
	state     cognitivecontract.ControlState
	triggers  bool
	cancelled string
}

func (f *fakeControl) GetState(context.Context) (cognitivecontract.ControlState, error) {
	return f.state, nil
}
func (f *fakeControl) SetPolicyCAS(context.Context, laputaevolution.TriggerPolicy, uint64) (cognitivecontract.ControlState, error) {
	return f.state, nil
}
func (f *fakeControl) Trigger(context.Context) (cognitivecontract.ControlState, error) {
	f.triggers = true
	return f.state, nil
}
func (f *fakeControl) Cancel(_ context.Context, runID domain.RunID) (cognitivecontract.ControlState, error) {
	f.cancelled = string(runID)
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
	binding, err := bundle.ResolveBinding(ctx, laputaevolution.TriggerPolicy{})
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

func TestResolveBindingPinsSuppliedPolicy(t *testing.T) {
	ctx, bundle := openBundle(t)
	policy := laputaevolution.TriggerPolicy{Enabled: true, MinIntervalMS: 91_000}
	binding, err := bundle.ResolveBinding(ctx, policy)
	if err != nil {
		t.Fatalf("ResolveBinding: %v", err)
	}
	raw, err := json.Marshal(policy)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(raw)
	want := hex.EncodeToString(sum[:])
	if binding.PolicyRevision != want {
		t.Fatalf("policy pin = %q, want digest of supplied durable policy %q", binding.PolicyRevision, want)
	}
	if got := bundle.Policy(); got == policy {
		t.Fatal("immutable first-load policy seed changed when resolving an admission policy")
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
	binding, err := bundle.ResolveBinding(ctx, laputaevolution.TriggerPolicy{})
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
	// Unarmed control reports an unavailable envelope, never a success.
	raw, err := dispatcher.Invoke(ctx, ActionStatus, json.RawMessage(`{"session_id":"s"}`))
	if err != nil {
		t.Fatalf("unarmed dispatch transport error = %v", err)
	}
	var out struct {
		Status string `json:"status"`
		Error  struct {
			Code      string `json:"code"`
			Retryable bool   `json:"retryable"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("status payload = %s err=%v", raw, err)
	}
	if out.Status != "unavailable" || out.Error.Code != "capability_unavailable" || !out.Error.Retryable {
		t.Fatalf("unarmed outcome = %s", raw)
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
	var out struct {
		Status string `json:"status"`
		Value  struct {
			Cognition struct {
				Enabled  bool   `json:"enabled"`
				SourceID string `json:"source_id"`
			} `json:"cognition"`
			Persona struct {
				State string `json:"state"`
			} `json:"persona"`
		} `json:"value"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("status payload = %s err=%v", raw, err)
	}
	if out.Status != "ok" || !out.Value.Cognition.Enabled || out.Value.Cognition.SourceID != "src" {
		t.Fatalf("status outcome = %s", raw)
	}
	if out.Value.Persona.State == "" {
		t.Fatalf("status outcome missing persona state = %s", raw)
	}
	// A handled action without a selected backend reports an honest
	// unavailable envelope, never a success and never a transport failure.
	raw, err = dispatcher.Invoke(ctx, ActionMemorySearch, json.RawMessage(`{"session_id":"s","query":"x","limit":10,"budget_chars":1000}`))
	if err != nil {
		t.Fatalf("search invoke transport error = %v", err)
	}
	var searchOut struct {
		Status string `json:"status"`
		Error  struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &searchOut); err != nil {
		t.Fatalf("search payload = %s err=%v", raw, err)
	}
	if searchOut.Status != "unavailable" || searchOut.Error.Code == "" {
		t.Fatalf("search outcome = %s", raw)
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

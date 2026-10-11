package toolkits

import (
	"context"
	"encoding/json"
	"testing"

	"agent-vivy/internal/bundlecontract"
	"agent-vivy/internal/bundles"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/toolkitcontract"
	controlaction "agent-vivy/sdk/port/controlaction"
)

func TestToolkitActionInventoryHasStrictSchemasAndStaticEffects(t *testing.T) {
	providers := ActionProviders()
	if len(providers) != len(ControlActionIDs()) {
		t.Fatalf("provider count %d, IDs %d", len(providers), len(ControlActionIDs()))
	}
	seen := map[string]bool{}
	for _, provider := range providers {
		definition, err := provider.Definition().Normalize()
		if err != nil {
			t.Fatalf("Normalize(%s): %v", provider.Definition().ID, err)
		}
		if definition.Owner != OwnerID {
			t.Fatalf("owner = %q", definition.Owner)
		}
		if seen[definition.ID] {
			t.Fatalf("duplicate action %s", definition.ID)
		}
		seen[definition.ID] = true
		if definition.Effect != controlaction.EffectRead && definition.Effect != controlaction.EffectWrite && definition.Effect != controlaction.EffectExternalEffect {
			t.Fatalf("action %s has invalid effect %q", definition.ID, definition.Effect)
		}
		if len(definition.InputSchema) == 0 || len(definition.ResultSchema) == 0 || definition.MaxInputBytes != MaxActionInput || definition.MaxOutputBytes != MaxActionOutput {
			t.Fatalf("action %s missing bounded schema", definition.ID)
		}
	}
	for _, id := range ControlActionIDs() {
		if !seen[id] {
			t.Errorf("action ID %q has no provider", id)
		}
	}
}

func TestToolkitPreviewActionsPinSourceKind(t *testing.T) {
	host := &actionHostFixture{}
	local, ok := providerFor(ActionPreview)
	if !ok {
		t.Fatal("local preview missing")
	}
	input := json.RawMessage(`{"operation_id":"10000000-0000-4000-8000-000000000001","expected_catalog":{"epoch":0,"config_revision":0,"effective_digest":"","generation_id":""},"location":"/host/toolkit"}`)
	if _, err := local.Invoke(context.Background(), host, input); err != nil {
		t.Fatal(err)
	}
	if host.preview.Source.Kind != "local" {
		t.Fatalf("local preview source kind = %q", host.preview.Source.Kind)
	}
	git, ok := providerFor(ActionPreviewGit)
	if !ok {
		t.Fatal("git preview missing")
	}
	input = json.RawMessage(`{"operation_id":"10000000-0000-4000-8000-000000000002","expected_catalog":{"epoch":0,"config_revision":0,"effective_digest":"","generation_id":""},"location":"https://example.invalid/repo","ref":"main"}`)
	if _, err := git.Invoke(context.Background(), host, input); err != nil {
		t.Fatal(err)
	}
	if host.preview.Source.Kind != "git" {
		t.Fatalf("git preview source kind = %q", host.preview.Source.Kind)
	}
}

func TestToolkitActionInputRejectsDuplicateUnknownAndNullFields(t *testing.T) {
	for _, raw := range []string{
		`{"operation_id":"a","operation_id":"b","expected_catalog":{"epoch":0,"config_revision":0,"effective_digest":"","generation_id":""},"location":"/x"}`,
		`{"operation_id":"a","expected_catalog":{"epoch":0,"config_revision":0,"effective_digest":"","generation_id":""},"location":"/x","unexpected":true}`,
		`{"operation_id":null,"expected_catalog":{"epoch":0,"config_revision":0,"effective_digest":"","generation_id":""},"location":"/x"}`,
	} {
		var input previewInput
		if err := decode([]byte(raw), &input); err == nil {
			t.Errorf("decode accepted %s", raw)
		}
	}
}

func TestHumanExecutableConsentCannotUseRunBoundIdentity(t *testing.T) {
	provider, ok := providerFor(ActionHookArm)
	if !ok {
		t.Fatal("Hook arm action missing")
	}
	host := &actionHostFixture{runBound: true}
	_, err := provider.Invoke(context.Background(), host, json.RawMessage(`{"operation_id":"10000000-0000-4000-8000-000000000003","expected_catalog":{"epoch":0,"config_revision":0,"effective_digest":"","generation_id":""},"install_id":"20000000-0000-4000-8000-000000000003","component_id":"before-tool","release_digest":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","declaration_digest":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","expected_consent_revision":0}`))
	if !toolkitcontract.IsCode(err, toolkitcontract.CodeAuthorizationDenied) {
		t.Fatalf("consent error = %v", err)
	}
	if host.consentCalls != 0 {
		t.Fatalf("run-bound consent calls = %d", host.consentCalls)
	}
}

func providerFor(id string) (toolkitAction, bool) {
	for _, item := range ActionProviders() {
		action, ok := item.(toolkitAction)
		if ok && action.definition.ID == id {
			return action, true
		}
	}
	return toolkitAction{}, false
}

type actionHostFixture struct {
	controlaction.Host
	preview      bundles.PreviewRequest
	runBound     bool
	consentCalls int
}

func (*actionHostFixture) Capabilities() toolkitcontract.Capabilities {
	return toolkitcontract.Capabilities{Owners: map[bundlecontract.ComponentKind]bool{bundlecontract.Skill: true}}
}
func (h *actionHostFixture) HumanSubject() (string, error) {
	if h.runBound {
		return "", toolkitcontract.NewError(toolkitcontract.CodeAuthorizationDenied, nil)
	}
	return "operator-1", nil
}
func (h *actionHostFixture) RunBound() bool { return h.runBound }
func (*actionHostFixture) ListInstallations(context.Context) (storage.BundleState, error) {
	return storage.BundleState{}, nil
}
func (*actionHostFixture) InspectInstallation(context.Context, string) (bundles.InstallationInspection, error) {
	return bundles.InstallationInspection{}, nil
}
func (*actionHostFixture) ReadComponent(context.Context, string, bundlecontract.ComponentKind, string, string, int, int) (bundles.ComponentPage, error) {
	return bundles.ComponentPage{}, nil
}
func (*actionHostFixture) GetOperation(context.Context, string) (bundlecontract.Operation, error) {
	return bundlecontract.Operation{}, nil
}
func (h *actionHostFixture) Preview(_ context.Context, in bundles.PreviewRequest) (bundles.PreparedSource, error) {
	h.preview = in
	return bundles.PreparedSource{OperationID: in.OperationID}, nil
}
func (*actionHostFixture) Install(context.Context, bundles.InstallRequest) (bundlecontract.Operation, error) {
	return bundlecontract.Operation{}, nil
}
func (*actionHostFixture) Prepare(context.Context, bundles.PrepareRequest) (bundles.PreparedCatalog, error) {
	return bundles.PreparedCatalog{}, nil
}
func (*actionHostFixture) PreparedCandidate(string) (string, bundlecontract.CatalogIdentity, bool) {
	return "", bundlecontract.CatalogIdentity{}, false
}
func (*actionHostFixture) Publish(context.Context, string) (bundlecontract.Operation, error) {
	return bundlecontract.Operation{}, nil
}
func (*actionHostFixture) CancelOperation(context.Context, string, int64) (bundlecontract.Operation, error) {
	return bundlecontract.Operation{}, nil
}
func (*actionHostFixture) CancelPreview(string) bool { return true }
func (h *actionHostFixture) SetExecutableConsent(context.Context, bundles.ExecutableConsentRequest) (bundlecontract.Operation, error) {
	h.consentCalls++
	return bundlecontract.Operation{}, nil
}
func (*actionHostFixture) SetBindings(context.Context, bundles.BindingUpdateRequest) (bundlecontract.Operation, error) {
	return bundlecontract.Operation{}, nil
}

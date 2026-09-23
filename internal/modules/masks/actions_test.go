package masks

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/maskcontract"
	controlaction "agent-vivy/sdk/port/controlaction"
)

type actionHostFixture struct {
	manager maskcontract.Manager
}

func (host actionHostFixture) ModuleID() string                  { return ID }
func (host actionHostFixture) Grant(controlaction.Grant) error   { return nil }
func (host actionHostFixture) HasGrant(controlaction.Grant) bool { return true }
func (host actionHostFixture) Secret(string) (string, error)     { return "", errors.New("not available") }
func (host actionHostFixture) Settings() json.RawMessage         { return json.RawMessage(`{}`) }
func (host actionHostFixture) StartRun(context.Context, controlaction.RunRequest) (controlaction.RunResult, error) {
	return controlaction.RunResult{}, errors.New("not available")
}
func (host actionHostFixture) InvokeTool(context.Context, string, json.RawMessage) (json.RawMessage, error) {
	return nil, errors.New("not available")
}
func (host actionHostFixture) ListMasks(ctx context.Context, in maskcontract.ListRequest) (maskcontract.Page, error) {
	return host.manager.ListMasks(ctx, in)
}
func (host actionHostFixture) GetMask(ctx context.Context, in maskcontract.GetRequest) (maskcontract.Definition, error) {
	return host.manager.GetMask(ctx, in)
}
func (host actionHostFixture) CreateMask(ctx context.Context, in maskcontract.CreateRequest) (maskcontract.Definition, error) {
	return host.manager.CreateMask(ctx, in)
}
func (host actionHostFixture) UpdateMask(ctx context.Context, in maskcontract.UpdateRequest) (maskcontract.Definition, error) {
	return host.manager.UpdateMask(ctx, in)
}
func (host actionHostFixture) DeleteMask(ctx context.Context, in maskcontract.DeleteRequest) (maskcontract.DeleteResult, error) {
	return host.manager.DeleteMask(ctx, in)
}
func (host actionHostFixture) GetMaskSelection(ctx context.Context, in maskcontract.SelectionRequest) (maskcontract.SelectionView, error) {
	return host.manager.GetMaskSelection(ctx, in)
}
func (host actionHostFixture) SetMaskSelection(ctx context.Context, in maskcontract.SetSelectionRequest) (maskcontract.SelectionView, error) {
	return host.manager.SetMaskSelection(ctx, in)
}

type actionManagerFixture struct{}

func (actionManagerFixture) ListMasks(context.Context, maskcontract.ListRequest) (maskcontract.Page, error) {
	return maskcontract.Page{Items: []maskcontract.Metadata{}}, nil
}
func (actionManagerFixture) GetMask(context.Context, maskcontract.GetRequest) (maskcontract.Definition, error) {
	return maskcontract.Definition{}, nil
}
func (actionManagerFixture) CreateMask(context.Context, maskcontract.CreateRequest) (maskcontract.Definition, error) {
	return maskcontract.Definition{}, nil
}
func (actionManagerFixture) UpdateMask(context.Context, maskcontract.UpdateRequest) (maskcontract.Definition, error) {
	return maskcontract.Definition{}, nil
}
func (actionManagerFixture) DeleteMask(context.Context, maskcontract.DeleteRequest) (maskcontract.DeleteResult, error) {
	return maskcontract.DeleteResult{}, nil
}
func (actionManagerFixture) GetMaskSelection(context.Context, maskcontract.SelectionRequest) (maskcontract.SelectionView, error) {
	return maskcontract.SelectionView{Selection: maskcontract.Selection{SessionID: domain.SessionID("session-1")}, Available: true}, nil
}
func (actionManagerFixture) SetMaskSelection(context.Context, maskcontract.SetSelectionRequest) (maskcontract.SelectionView, error) {
	return maskcontract.SelectionView{Selection: maskcontract.Selection{SessionID: domain.SessionID("session-1"), Revision: 1}, Available: true}, nil
}

func TestActionProvidersHaveClosedOwnerInventory(t *testing.T) {
	providers := ActionProviders()
	if len(providers) != 7 {
		t.Fatalf("provider count = %d, want 7", len(providers))
	}
	want := map[string]bool{
		ActionCatalogList: true, ActionCatalogGet: true, ActionCatalogCreate: true,
		ActionCatalogUpdate: true, ActionCatalogDelete: true,
		ActionSelectionGet: true, ActionSelectionSet: true,
	}
	for _, provider := range providers {
		if provider == nil {
			t.Fatal("nil mask action provider")
		}
		definition, err := provider.Definition().Normalize()
		if err != nil {
			t.Fatalf("invalid definition: %v", err)
		}
		if definition.Owner != ID || !want[definition.ID] {
			t.Fatalf("unexpected action definition: %#v", definition)
		}
		delete(want, definition.ID)
	}
	if len(want) != 0 {
		t.Fatalf("missing action definitions: %#v", want)
	}
}

func TestSelectionActionUsesFlatWireResultAndRejectsExtraInput(t *testing.T) {
	var provider maskAction
	for _, candidate := range ActionProviders() {
		if candidate.Definition().ID == ActionSelectionGet {
			provider = candidate.(maskAction)
			break
		}
	}
	host := actionHostFixture{manager: actionManagerFixture{}}
	result, err := provider.Invoke(context.Background(), host, json.RawMessage(`{"session_id":"session-1"}`))
	if err != nil {
		t.Fatal(err)
	}
	var wire selectionWire
	if err := json.Unmarshal(result, &wire); err != nil {
		t.Fatal(err)
	}
	if wire.SessionID != "session-1" || !wire.Available || wire.Revision != 0 {
		t.Fatalf("selection wire = %#v", wire)
	}
	if _, err := provider.Invoke(context.Background(), host, json.RawMessage(`{"session_id":"session-1","extra":true}`)); err == nil {
		t.Fatal("extra action input property was accepted")
	}
}

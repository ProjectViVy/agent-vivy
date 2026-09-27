package actionhost

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"agent-vivy/internal/maskcontract"
	action "agent-vivy/sdk/port/controlaction"
)

type maskManagerFixture struct{}

func (maskManagerFixture) ListMasks(context.Context, maskcontract.ListRequest) (maskcontract.Page, error) {
	return maskcontract.Page{}, nil
}
func (maskManagerFixture) GetMask(context.Context, maskcontract.GetRequest) (maskcontract.Definition, error) {
	return maskcontract.Definition{}, nil
}
func (maskManagerFixture) CreateMask(context.Context, maskcontract.CreateRequest) (maskcontract.Definition, error) {
	return maskcontract.Definition{}, nil
}
func (maskManagerFixture) UpdateMask(context.Context, maskcontract.UpdateRequest) (maskcontract.Definition, error) {
	return maskcontract.Definition{}, nil
}
func (maskManagerFixture) DeleteMask(context.Context, maskcontract.DeleteRequest) (maskcontract.DeleteResult, error) {
	return maskcontract.DeleteResult{}, nil
}
func (maskManagerFixture) GetMaskSelection(context.Context, maskcontract.SelectionRequest) (maskcontract.SelectionView, error) {
	return maskcontract.SelectionView{Selection: maskcontract.Selection{MaskID: maskcontract.BuiltinWriterID}}, nil
}
func (maskManagerFixture) SetMaskSelection(context.Context, maskcontract.SetSelectionRequest) (maskcontract.SelectionView, error) {
	return maskcontract.SelectionView{}, nil
}

type contextRecordingMaskManager struct {
	maskManagerFixture
	seen context.Context
}

func (manager *contextRecordingMaskManager) CreateMask(ctx context.Context, _ maskcontract.CreateRequest) (maskcontract.Definition, error) {
	manager.seen = ctx
	return maskcontract.Definition{}, nil
}

func TestMaskOwnerReceivesPrivateManagerFacade(t *testing.T) {
	definition := action.Definition{
		ID: "vivy.masks.test", Owner: maskModuleOwner, ModuleID: maskModuleOwner,
		Effect: action.EffectRead, InputSchema: testInputSchema, ResultSchema: testOutputSchema,
	}
	provider := providerFunc{definition: definition, invoke: func(ctx context.Context, host action.Host, _ json.RawMessage) (json.RawMessage, error) {
		private, ok := host.(maskcontract.ActionHost)
		if !ok {
			return nil, errors.New("mask manager facade missing")
		}
		view, err := private.GetMaskSelection(ctx, maskcontract.SelectionRequest{SessionID: "session-1"})
		if err != nil || view.Selection.MaskID != maskcontract.BuiltinWriterID {
			return nil, errors.New("mask manager call failed")
		}
		return json.RawMessage(`{"ok":true}`), nil
	}}
	deps := readyDeps(provider)
	deps.Bindings[0].ModuleID = maskModuleOwner
	deps.Bindings[0].ActionID = definition.ID
	deps.MaskManager = maskManagerFixture{}
	host, err := New(deps)
	if err != nil {
		t.Fatal(err)
	}
	result, err := host.Invoke(context.Background(), readyCaller(), maskModuleOwner, definition.ID, json.RawMessage(`{"value":"x"}`))
	if err != nil {
		t.Fatal(err)
	}
	if string(result) != `{"ok":true}` {
		t.Fatalf("result = %s", result)
	}
}

func TestNonMaskOwnerDoesNotReceivePrivateManagerFacade(t *testing.T) {
	definition := testDefinition("example.action.facade", action.EffectRead)
	provider := providerFunc{definition: definition, invoke: func(_ context.Context, host action.Host, _ json.RawMessage) (json.RawMessage, error) {
		if _, ok := host.(maskcontract.ActionHost); ok {
			return nil, errors.New("ordinary provider received mask facade")
		}
		return json.RawMessage(`{"ok":true}`), nil
	}}
	deps := readyDeps(provider)
	deps.MaskManager = maskManagerFixture{}
	host, err := New(deps)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := host.Invoke(context.Background(), readyCaller(), testModule, definition.ID, json.RawMessage(`{"value":"x"}`)); err != nil {
		t.Fatal(err)
	}
}

func TestMaskFacadeBindsSelectionToAuthenticatedSession(t *testing.T) {
	definition := action.Definition{
		ID: "vivy.masks.session", Owner: maskModuleOwner, ModuleID: maskModuleOwner,
		Effect: action.EffectRead, InputSchema: testInputSchema, ResultSchema: testOutputSchema,
	}
	provider := providerFunc{definition: definition, invoke: func(ctx context.Context, host action.Host, _ json.RawMessage) (json.RawMessage, error) {
		private := host.(maskcontract.ActionHost)
		_, err := private.GetMaskSelection(ctx, maskcontract.SelectionRequest{SessionID: "other-session"})
		if err == nil {
			return nil, errors.New("unbound session was accepted")
		}
		var maskErr *maskcontract.Error
		if !errors.As(err, &maskErr) || maskErr.Code != maskcontract.CodeAuthorizationDenied {
			return nil, errors.New("wrong session error")
		}
		return json.RawMessage(`{"ok":true}`), nil
	}}
	deps := readyDeps(provider)
	deps.Bindings[0].ModuleID = maskModuleOwner
	deps.Bindings[0].ActionID = definition.ID
	deps.MaskManager = maskManagerFixture{}
	host, err := New(deps)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := host.Invoke(context.Background(), readyCaller(), maskModuleOwner, definition.ID, json.RawMessage(`{"value":"x"}`)); err != nil {
		t.Fatal(err)
	}
}

func TestMaskFacadeRootsManagerCallsInHostContext(t *testing.T) {
	definition := action.Definition{
		ID: "vivy.masks.context", Owner: maskModuleOwner, ModuleID: maskModuleOwner,
		Effect: action.EffectRead, InputSchema: testInputSchema, ResultSchema: testOutputSchema,
	}
	manager := &contextRecordingMaskManager{}
	provider := providerFunc{definition: definition, invoke: func(_ context.Context, host action.Host, _ json.RawMessage) (json.RawMessage, error) {
		private := host.(maskcontract.ActionHost)
		_, err := private.CreateMask(context.Background(), maskcontract.CreateRequest{})
		if err != nil {
			return nil, err
		}
		if manager.seen == nil {
			return nil, errors.New("manager did not receive a context")
		}
		if _, ok := manager.seen.Deadline(); !ok {
			return nil, errors.New("manager context escaped host deadline")
		}
		return json.RawMessage(`{"ok":true}`), nil
	}}
	deps := readyDeps(provider)
	deps.Bindings[0].ModuleID = maskModuleOwner
	deps.Bindings[0].ActionID = definition.ID
	deps.MaskManager = manager
	deps.Timeout = time.Second
	host, err := New(deps)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := host.Invoke(context.Background(), readyCaller(), maskModuleOwner, definition.ID, json.RawMessage(`{"value":"x"}`)); err != nil {
		t.Fatal(err)
	}
}

package actionhost

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"agent-vivy/internal/cognitivecontract"
	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	action "agent-vivy/sdk/port/controlaction"
)

const cognitiveTestAction = "diva.cognitive.status"

type recordingDispatcher struct {
	called   bool
	actionID string
	input    json.RawMessage
	result   json.RawMessage
	err      error
}

func (d *recordingDispatcher) Invoke(_ context.Context, actionID string, input json.RawMessage) (json.RawMessage, error) {
	d.called = true
	d.actionID = actionID
	d.input = append(json.RawMessage(nil), input...)
	if d.err != nil {
		return nil, d.err
	}
	return d.result, nil
}

type dispatcherProviderFixture struct {
	dispatcher cognitivecontract.Dispatcher
}

func (p dispatcherProviderFixture) Dispatcher() cognitivecontract.Dispatcher { return p.dispatcher }

var cognitiveInputSchema = json.RawMessage(`{"type":"object","properties":{"session_id":{"type":"string"}},"required":["session_id"],"additionalProperties":false}`)

func cognitiveDefinition() action.Definition {
	return action.Definition{
		ID: cognitiveTestAction, Owner: cognitiveModuleOwner, ModuleID: cognitiveModuleOwner,
		Effect: action.EffectRead, InputSchema: cognitiveInputSchema, ResultSchema: testOutputSchema,
	}
}

func cognitiveDeps(provider action.Provider, dispatcher cognitivecontract.Dispatcher, check func(context.Context, domain.SessionID) error) Deps {
	definition := provider.Definition()
	deps := readyDeps(provider)
	deps.Bindings[0].ModuleID = cognitiveModuleOwner
	deps.Bindings[0].ActionID = definition.ID
	deps.Cognitive = dispatcherProviderFixture{dispatcher: dispatcher}
	deps.CognitiveSessionCheck = check
	return deps
}

func TestCognitiveOwnerReceivesGuardedDispatcher(t *testing.T) {
	dispatcher := &recordingDispatcher{result: json.RawMessage(`{"ok":true}`)}
	provider := providerFunc{definition: cognitiveDefinition(), invoke: func(ctx context.Context, host action.Host, input json.RawMessage) (json.RawMessage, error) {
		private, ok := host.(cognitivecontract.ActionHost)
		if !ok {
			return nil, errors.New("cognitive facade missing")
		}
		armed, err := private.Cognitive()
		if err != nil {
			return nil, err
		}
		return armed.Invoke(ctx, cognitiveTestAction, input)
	}}
	check := func(_ context.Context, id domain.SessionID) error {
		if id != "session-1" {
			return errors.New("unexpected session")
		}
		return nil
	}
	host, err := New(cognitiveDeps(provider, dispatcher, check))
	if err != nil {
		t.Fatal(err)
	}
	result, err := host.Invoke(context.Background(), readyCaller(), cognitiveModuleOwner, cognitiveTestAction, json.RawMessage(`{"session_id":"session-1"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !dispatcher.called || dispatcher.actionID != cognitiveTestAction {
		t.Fatalf("dispatcher = %+v", dispatcher)
	}
	if string(result) != `{"ok":true}` {
		t.Fatalf("result = %s", result)
	}
}

func TestNonCognitiveOwnerDoesNotReceiveCognitiveFacade(t *testing.T) {
	definition := testDefinition("example.action.plain", action.EffectRead)
	provider := providerFunc{definition: definition, invoke: func(_ context.Context, host action.Host, _ json.RawMessage) (json.RawMessage, error) {
		if _, ok := host.(cognitivecontract.ActionHost); ok {
			return nil, errors.New("ordinary provider received cognitive facade")
		}
		return json.RawMessage(`{"ok":true}`), nil
	}}
	deps := readyDeps(provider)
	deps.Cognitive = dispatcherProviderFixture{dispatcher: &recordingDispatcher{}}
	host, err := New(deps)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := host.Invoke(context.Background(), readyCaller(), testModule, definition.ID, json.RawMessage(`{"value":"x"}`)); err != nil {
		t.Fatal(err)
	}
}

func TestCognitiveDispatcherRejectsForeignSession(t *testing.T) {
	dispatcher := &recordingDispatcher{result: json.RawMessage(`{"ok":true}`)}
	provider := providerFunc{definition: cognitiveDefinition(), invoke: func(ctx context.Context, host action.Host, _ json.RawMessage) (json.RawMessage, error) {
		armed, err := host.(cognitivecontract.ActionHost).Cognitive()
		if err != nil {
			return nil, err
		}
		return armed.Invoke(ctx, cognitiveTestAction, json.RawMessage(`{"session_id":"other-session"}`))
	}}
	check := func(context.Context, domain.SessionID) error { return nil }
	host, err := New(cognitiveDeps(provider, dispatcher, check))
	if err != nil {
		t.Fatal(err)
	}
	_, err = host.Invoke(context.Background(), readyCaller(), cognitiveModuleOwner, cognitiveTestAction, json.RawMessage(`{"session_id":"session-1"}`))
	if !errors.Is(err, action.ErrGrantDenied) {
		t.Fatalf("foreign session err = %v", err)
	}
	if dispatcher.called {
		t.Fatal("dispatcher ran for a foreign session")
	}
}

func TestCognitiveDispatcherRejectsUnboundSession(t *testing.T) {
	dispatcher := &recordingDispatcher{result: json.RawMessage(`{"ok":true}`)}
	provider := providerFunc{definition: cognitiveDefinition(), invoke: func(ctx context.Context, host action.Host, _ json.RawMessage) (json.RawMessage, error) {
		armed, err := host.(cognitivecontract.ActionHost).Cognitive()
		if err != nil {
			return nil, err
		}
		return armed.Invoke(ctx, cognitiveTestAction, json.RawMessage(`{"session_id":"session-1","extra":1}`))
	}}
	check := func(context.Context, domain.SessionID) error { return nil }
	// Identity carries no session binding (peer never called session/new|load).
	deps := cognitiveDeps(provider, dispatcher, check)
	deps.Authenticate = func(context.Context, Caller) (Identity, error) {
		return Identity{ID: "operator"}, nil
	}
	host, err := New(deps)
	if err != nil {
		t.Fatal(err)
	}
	_, err = host.Invoke(context.Background(), readyCaller(), cognitiveModuleOwner, cognitiveTestAction, json.RawMessage(`{"session_id":"session-1"}`))
	if !errors.Is(err, action.ErrGrantDenied) {
		t.Fatalf("unbound session err = %v", err)
	}
	if dispatcher.called {
		t.Fatal("dispatcher ran without a bound session")
	}
}

func TestCognitiveDispatcherChecksSessionRegistry(t *testing.T) {
	dispatcher := &recordingDispatcher{result: json.RawMessage(`{"ok":true}`)}
	provider := providerFunc{definition: cognitiveDefinition(), invoke: func(ctx context.Context, host action.Host, input json.RawMessage) (json.RawMessage, error) {
		armed, err := host.(cognitivecontract.ActionHost).Cognitive()
		if err != nil {
			return nil, err
		}
		return armed.Invoke(ctx, cognitiveTestAction, input)
	}}
	check := func(context.Context, domain.SessionID) error { return storage.ErrNotFound }
	host, err := New(cognitiveDeps(provider, dispatcher, check))
	if err != nil {
		t.Fatal(err)
	}
	_, err = host.Invoke(context.Background(), readyCaller(), cognitiveModuleOwner, cognitiveTestAction, json.RawMessage(`{"session_id":"session-1"}`))
	if !errors.Is(err, action.ErrGrantDenied) {
		t.Fatalf("registry-miss err = %v", err)
	}
	if dispatcher.called {
		t.Fatal("dispatcher ran for a session missing from the registry")
	}
}

func TestCognitiveFacadeFailsClosedWithoutDispatcher(t *testing.T) {
	provider := providerFunc{definition: cognitiveDefinition(), invoke: func(_ context.Context, host action.Host, _ json.RawMessage) (json.RawMessage, error) {
		private, ok := host.(cognitivecontract.ActionHost)
		if !ok {
			return nil, errors.New("cognitive facade missing")
		}
		if _, err := private.Cognitive(); !errors.Is(err, cognitivecontract.ErrUnarmed) {
			return nil, errors.New("unarmed dispatcher did not fail closed")
		}
		return json.RawMessage(`{"ok":true}`), nil
	}}
	deps := cognitiveDeps(provider, nil, func(context.Context, domain.SessionID) error { return nil })
	deps.Cognitive = nil
	host, err := New(deps)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := host.Invoke(context.Background(), readyCaller(), cognitiveModuleOwner, cognitiveTestAction, json.RawMessage(`{"session_id":"session-1"}`)); err != nil {
		t.Fatal(err)
	}
}

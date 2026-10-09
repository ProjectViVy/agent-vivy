package actionhost

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	nb "agent-vivy/internal/notebookcontract"
	action "agent-vivy/sdk/port/controlaction"
)

const notebookTestAction = "vivy.notebook.entries.create"

var notebookTestSchema = json.RawMessage(`{"type":"object","properties":{"operation_key":{"type":"string"},"request":{"type":"object"}},"required":["operation_key","request"],"additionalProperties":false}`)

// fakeNotebookBundle records the scope and actor the host binds so the test
// can assert provenance arrives from identity, not provider JSON.
type fakeNotebookBundle struct {
	scope nb.ScopeID
	actor nb.Actor
	err   error
}

func (b *fakeNotebookBundle) Service() nb.Store { return nil }
func (b *fakeNotebookBundle) Actions(scope nb.ScopeID, actor nb.Actor) nb.ScopedActions {
	b.scope = scope
	b.actor = actor
	return fakeScopedActions{}
}
func (b *fakeNotebookBundle) Close(context.Context) error { return nil }

type fakeScopedActions struct{ nb.ScopedActions }

type fakeScopeResolver struct {
	home      nb.ScopeID
	workspace nb.ScopeID
	sessionID string
	err       error
}

func (r *fakeScopeResolver) Home() nb.ScopeID { return r.home }
func (r *fakeScopeResolver) ForSession(_ context.Context, sessionID string) (nb.ScopeID, error) {
	if r.err != nil {
		return "", r.err
	}
	r.sessionID = sessionID
	return r.workspace, nil
}

// notebookFacadeProvider surfaces the scope/actor captured by the bound
// facade so the test can read them off the recorded bundle.
func notebookFacadeProvider() action.Provider {
	return providerFunc{
		definition: action.Definition{
			ID:           notebookTestAction,
			Owner:        notebookModuleOwner,
			ModuleID:     notebookModuleOwner,
			Effect:       action.EffectWrite,
			InputSchema:  notebookTestSchema,
			ResultSchema: testOutputSchema,
		},
		invoke: func(ctx context.Context, host action.Host, input json.RawMessage) (json.RawMessage, error) {
			private, ok := host.(nb.ActionHost)
			if !ok {
				return nil, errors.New("notebook facade missing")
			}
			if _, err := private.Notebook(); err != nil {
				return nil, err
			}
			return json.RawMessage(`{"ok":true}`), nil
		},
	}
}

func notebookDeps(provider action.Provider, bundle nb.Bundle, scopes nb.ScopeResolver) Deps {
	deps := readyDeps(provider)
	deps.Bindings[0].ModuleID = notebookModuleOwner
	deps.Bindings[0].ActionID = notebookTestAction
	deps.Notebook = bundle
	deps.NotebookScopes = scopes
	return deps
}

func invokeNotebook(t *testing.T, host *Host, caller Caller, ctx context.Context) (json.RawMessage, error) {
	t.Helper()
	return host.Invoke(ctx, caller, notebookModuleOwner, notebookTestAction, json.RawMessage(`{"operation_key":"k","request":{"title":"t"}}`))
}

func TestNotebookActionHostBindsHumanHomeOrigin(t *testing.T) {
	bundle := &fakeNotebookBundle{}
	scopes := &fakeScopeResolver{home: nb.HomeScopeID}
	host, err := New(notebookDeps(notebookFacadeProvider(), bundle, scopes))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = host.Close() })
	out, err := invokeNotebook(t, host, readyCaller(), context.Background())
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	if string(out) != `{"ok":true}` {
		t.Fatalf("result = %s", out)
	}
	if bundle.scope != nb.HomeScopeID {
		t.Fatalf("human invocation scope = %q, want %q", bundle.scope, nb.HomeScopeID)
	}
	if bundle.actor.Kind != nb.ActorHuman || bundle.actor.Ref != "peer:operator" {
		t.Fatalf("human actor = %+v", bundle.actor)
	}
}

func TestNotebookActionHostBindsAgentWorkspaceOrigin(t *testing.T) {
	bundle := &fakeNotebookBundle{}
	scopes := &fakeScopeResolver{home: nb.HomeScopeID, workspace: "ws.v1:session-1"}
	deps := notebookDeps(notebookFacadeProvider(), bundle, scopes)
	deps.Authenticate = func(_ context.Context, caller Caller) (Identity, error) {
		if caller.Opaque() != "transport-ok" {
			return Identity{}, ErrUnauthenticated
		}
		return Identity{ID: "operator", SessionID: "session-1", RunID: "run-9"}, nil
	}
	host, err := New(deps)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = host.Close() })
	if _, err := invokeNotebook(t, host, readyCaller(), context.Background()); err != nil {
		t.Fatalf("invoke: %v", err)
	}
	if scopes.sessionID != "session-1" {
		t.Fatalf("resolver got session %q", scopes.sessionID)
	}
	if bundle.scope != "ws.v1:session-1" {
		t.Fatalf("agent invocation scope = %q", bundle.scope)
	}
	if bundle.actor.Kind != nb.ActorAgent || bundle.actor.Ref != "run:run-9" {
		t.Fatalf("agent actor = %+v", bundle.actor)
	}
}

func TestNotebookActionHostUnarmedFailsClosed(t *testing.T) {
	host, err := New(notebookDeps(notebookFacadeProvider(), nil, &fakeScopeResolver{}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = host.Close() })
	if _, err := invokeNotebook(t, host, readyCaller(), context.Background()); err == nil {
		t.Fatal("unarmed facade must fail closed")
	}
}

func TestNotebookActionHostUnauthenticatedWritesNothing(t *testing.T) {
	bundle := &fakeNotebookBundle{}
	host, err := New(notebookDeps(notebookFacadeProvider(), bundle, &fakeScopeResolver{home: nb.HomeScopeID}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = host.Close() })
	if _, err := invokeNotebook(t, host, NewCaller("wrong-token"), context.Background()); err == nil {
		t.Fatal("unauthenticated invocation must fail")
	}
	if bundle.scope != "" || bundle.actor.Kind != "" {
		t.Fatalf("unauthenticated invocation reached facade: %+v", bundle)
	}
}

func TestNotebookActionHostForeignSessionDenied(t *testing.T) {
	bundle := &fakeNotebookBundle{}
	scopes := &fakeScopeResolver{err: &nb.Error{Code: nb.CodeNotFound, Message: "unknown session"}}
	deps := notebookDeps(notebookFacadeProvider(), bundle, scopes)
	deps.Authenticate = func(_ context.Context, caller Caller) (Identity, error) {
		if caller.Opaque() != "transport-ok" {
			return Identity{}, ErrUnauthenticated
		}
		return Identity{ID: "operator", SessionID: "session-foreign", RunID: "run-1"}, nil
	}
	host, err := New(deps)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = host.Close() })
	if _, err := invokeNotebook(t, host, readyCaller(), context.Background()); err == nil {
		t.Fatal("foreign session scope must be denied")
	}
	if bundle.scope != "" {
		t.Fatalf("denied invocation bound scope %q", bundle.scope)
	}
}

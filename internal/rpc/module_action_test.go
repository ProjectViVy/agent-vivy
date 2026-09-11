package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"agent-vivy/internal/actionhost"
	"agent-vivy/sdk/port/controlaction"
)

const (
	moduleActionModule = "example/module"
	moduleActionID     = "example.action.read"
	moduleActionToken  = "face-connection-token"
)

type moduleActionFixture struct {
	env       *controlTestEnv
	host      *actionhost.Host
	calls     atomic.Int32
	audit     []actionhost.AuditRecord
	auditMu   atomic.Int32
	lastInput atomic.Value
}

func newModuleActionFixture(t *testing.T, invoke func(context.Context, controlaction.Host, json.RawMessage) (json.RawMessage, error)) *moduleActionFixture {
	t.Helper()
	fixture := &moduleActionFixture{}
	provider := controlaction.ProviderFunc{
		Def: controlaction.Definition{
			ID:           moduleActionID,
			Owner:        moduleActionModule,
			Effect:       controlaction.EffectRead,
			InputSchema:  json.RawMessage(`{"type":"object","properties":{"value":{"type":"string","maxLength":64}},"required":["value"],"additionalProperties":false}`),
			ResultSchema: json.RawMessage(`{"type":"object","properties":{"ok":{"type":"boolean"}},"required":["ok"],"additionalProperties":false}`),
		},
		Fn: func(ctx context.Context, host controlaction.Host, input json.RawMessage) (json.RawMessage, error) {
			fixture.calls.Add(1)
			fixture.lastInput.Store(string(input))
			if invoke != nil {
				return invoke(ctx, host, input)
			}
			return json.RawMessage(`{"ok":true}`), nil
		},
	}
	audit := actionhost.AuditSinkFunc(func(_ context.Context, record actionhost.AuditRecord) error {
		fixture.auditMu.Add(1)
		fixture.audit = append(fixture.audit, record)
		return nil
	})
	host, err := actionhost.New(actionhost.Deps{
		Bindings:            []actionhost.ProviderBinding{{ModuleID: moduleActionModule, ActionID: moduleActionID, Provider: provider}},
		GenerationAvailable: true,
		GenerationID:        "generation-active",
		Authenticate: func(_ context.Context, caller actionhost.Caller) (actionhost.Identity, error) {
			if caller.Opaque() != moduleActionToken {
				return actionhost.Identity{}, controlaction.ErrUnauthenticated
			}
			return actionhost.Identity{ID: "face-1", Face: "web"}, nil
		},
		Authorize: func(_ context.Context, _ actionhost.Identity, _ controlaction.Definition, _ json.RawMessage) error {
			return nil
		},
		Audit:   audit,
		Timeout: 100 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("build action host: %v", err)
	}
	fixture.host = host
	t.Cleanup(func() { _ = host.Close() })
	fixture.env = newControlTestEnv(t, func(deps *ControlDeps) {
		deps.ActionHost = host
	})
	return fixture
}

func moduleActionRequest(input string) Request {
	return Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"module-action"`),
		Method:  "module.action.invoke",
		Params:  json.RawMessage(`{"module_id":"` + moduleActionModule + `","action_id":"` + moduleActionID + `","input":` + input + `}`),
	}
}

func moduleActionContext() context.Context {
	return WithAuthenticatedCaller(context.Background(), actionhost.NewCaller(moduleActionToken))
}

func invokeModuleActionTest(t *testing.T, fixture *moduleActionFixture, request Request) (any, *Error) {
	t.Helper()
	return fixture.env.handler.Handle(moduleActionContext(), nil, request)
}

func TestModuleActionRPCUsesAuthenticatedCallerAndReturnsBoundResult(t *testing.T) {
	fixture := newModuleActionFixture(t, nil)
	result, rpcErr := invokeModuleActionTest(t, fixture, moduleActionRequest(`{"value":"ok"}`))
	if rpcErr != nil {
		t.Fatalf("module.action.invoke error = %v", rpcErr)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if string(encoded) != `{"ok":true}` {
		t.Fatalf("result = %s, want bounded provider result", encoded)
	}
	if fixture.calls.Load() != 1 {
		t.Fatalf("provider calls = %d, want one", fixture.calls.Load())
	}
	if got := fixture.lastInput.Load(); got == nil || got.(string) != `{"value":"ok"}` {
		t.Fatalf("provider input = %v", got)
	}
	initialized, rpcErr := fixture.env.handler.Handle(moduleActionContext(), nil, Request{JSONRPC: "2.0", Method: "initialize"})
	if rpcErr != nil {
		t.Fatalf("initialize error = %v", rpcErr)
	}
	initJSON, _ := json.Marshal(initialized)
	if !strings.Contains(string(initJSON), "module.action.invoke") {
		t.Fatalf("capabilities = %s, want module.action.invoke", initJSON)
	}
}

func TestModuleActionRPCUsesPeerBoundCaller(t *testing.T) {
	fixture := newModuleActionFixture(t, nil)
	peer := NewPeer(nil, fixture.env.handler, Options{Caller: actionhost.NewCaller(moduleActionToken)})
	result, rpcErr := fixture.env.handler.Handle(context.Background(), peer, moduleActionRequest(`{"value":"ok"}`))
	if rpcErr != nil {
		t.Fatalf("peer-bound module.action.invoke error = %v", rpcErr)
	}
	if encoded, _ := json.Marshal(result); string(encoded) != `{"ok":true}` {
		t.Fatalf("result = %s, want bounded provider result", encoded)
	}
}

func TestModuleActionRPCRequiresAuthenticatedTransportContext(t *testing.T) {
	fixture := newModuleActionFixture(t, nil)
	_, rpcErr := fixture.env.handler.Handle(context.Background(), nil, moduleActionRequest(`{"value":"ok"}`))
	if rpcErr == nil {
		t.Fatal("unauthenticated action unexpectedly succeeded")
	}
	if fixture.calls.Load() != 0 {
		t.Fatalf("provider calls = %d, want zero", fixture.calls.Load())
	}
}

func TestModuleActionRPCRejectsSpoofedModuleIDAndForgedAuthorityClaims(t *testing.T) {
	fixture := newModuleActionFixture(t, nil)
	spoofed := moduleActionRequest(`{"value":"ok"}`)
	spoofed.Params = json.RawMessage(`{"module_id":"attacker/module","action_id":"` + moduleActionID + `","input":{"value":"ok"}}`)
	if _, rpcErr := invokeModuleActionTest(t, fixture, spoofed); rpcErr == nil {
		t.Fatal("spoofed Module ID unexpectedly succeeded")
	}
	for _, field := range []string{"approval", "trust", "authority", "grants", "caller", "identity", "token", "generation_id", "instance_id", "Module_ID"} {
		request := moduleActionRequest(`{"value":"ok"}`)
		request.Params = json.RawMessage(`{"module_id":"` + moduleActionModule + `","action_id":"` + moduleActionID + `","input":{"value":"ok"},"` + field + `":true}`)
		if _, rpcErr := invokeModuleActionTest(t, fixture, request); rpcErr == nil || rpcErr.Code != InvalidParams {
			t.Fatalf("forged %s error = %v, want InvalidParams", field, rpcErr)
		}
	}
	if fixture.calls.Load() != 0 {
		t.Fatalf("provider calls = %d, want zero for forged requests", fixture.calls.Load())
	}
}

func TestModuleActionRPCRejectsOversizedDeepAndInvalidInput(t *testing.T) {
	fixture := newModuleActionFixture(t, nil)
	oversized := `{"value":"` + strings.Repeat("x", 1<<20) + `"}`
	if _, rpcErr := invokeModuleActionTest(t, fixture, moduleActionRequest(oversized)); rpcErr == nil || rpcErr.Code != InvalidParams {
		t.Fatalf("oversized input error = %v, want InvalidParams", rpcErr)
	}
	deep := strings.Repeat("[", 40) + `{"value":"ok"}` + strings.Repeat("]", 40)
	if _, rpcErr := invokeModuleActionTest(t, fixture, moduleActionRequest(deep)); rpcErr == nil || rpcErr.Code != InvalidParams {
		t.Fatalf("deep input error = %v, want InvalidParams", rpcErr)
	}
	if _, rpcErr := invokeModuleActionTest(t, fixture, moduleActionRequest(`{"value":false}`)); rpcErr == nil || rpcErr.Code != InvalidParams {
		t.Fatalf("schema-invalid input error = %v, want InvalidParams", rpcErr)
	}
	malformed := moduleActionRequest(`{"value":"ok"}`)
	malformed.Params = json.RawMessage(`{"module_id":"` + moduleActionModule + `","action_id":"` + moduleActionID + `","input":{"value":}`)
	if _, rpcErr := invokeModuleActionTest(t, fixture, malformed); rpcErr == nil || rpcErr.Code != InvalidParams {
		t.Fatalf("malformed input error = %v, want InvalidParams", rpcErr)
	}
	if fixture.calls.Load() != 0 {
		t.Fatalf("provider calls = %d, want zero for bounded-invalid requests", fixture.calls.Load())
	}
}

func TestModuleActionRPCRejectsUnknownActionAndPluginDefinedMethods(t *testing.T) {
	fixture := newModuleActionFixture(t, nil)
	unknown := moduleActionRequest(`{"value":"ok"}`)
	unknown.Params = json.RawMessage(`{"module_id":"` + moduleActionModule + `","action_id":"example.action.missing","input":{"value":"ok"}}`)
	if _, rpcErr := invokeModuleActionTest(t, fixture, unknown); rpcErr == nil {
		t.Fatal("unknown action unexpectedly succeeded")
	}
	for _, method := range []string{"module.action.unknown", "plugin.example/module.action", "plugin.example/module/custom"} {
		request := moduleActionRequest(`{"value":"ok"}`)
		request.Method = method
		if _, rpcErr := fixture.env.handler.Handle(moduleActionContext(), nil, request); rpcErr == nil || rpcErr.Code != MethodNotFound {
			t.Fatalf("method %q error = %v, want MethodNotFound", method, rpcErr)
		}
	}
	if fixture.calls.Load() != 0 {
		t.Fatalf("provider calls = %d, want zero for unknown routes", fixture.calls.Load())
	}
}

func TestModuleActionRPCRejectsCancellationAndTimeout(t *testing.T) {
	cancelled := newModuleActionFixture(t, func(ctx context.Context, _ controlaction.Host, _ json.RawMessage) (json.RawMessage, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	})
	ctx, cancel := context.WithCancel(moduleActionContext())
	cancel()
	_, rpcErr := cancelled.env.handler.Handle(ctx, nil, moduleActionRequest(`{"value":"ok"}`))
	if rpcErr == nil {
		t.Fatal("cancelled action unexpectedly succeeded")
	}

	timedOut := newModuleActionFixture(t, func(ctx context.Context, _ controlaction.Host, _ json.RawMessage) (json.RawMessage, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	})
	_, rpcErr = invokeModuleActionTest(t, timedOut, moduleActionRequest(`{"value":"ok"}`))
	if rpcErr == nil {
		t.Fatal("timed-out action unexpectedly succeeded")
	}
	if timedOut.calls.Load() != 1 {
		t.Fatalf("timed-out provider calls = %d, want one", timedOut.calls.Load())
	}
}

func TestModuleActionRPCRedactsProviderErrorsAndSecrets(t *testing.T) {
	secret := "super-secret-action-token"
	fixture := newModuleActionFixture(t, func(context.Context, controlaction.Host, json.RawMessage) (json.RawMessage, error) {
		return nil, errors.New("provider failed with " + secret)
	})
	_, rpcErr := invokeModuleActionTest(t, fixture, moduleActionRequest(`{"value":"ok"}`))
	if rpcErr == nil {
		t.Fatal("provider failure unexpectedly succeeded")
	}
	if strings.Contains(rpcErr.Message, secret) || strings.Contains(string(rpcErr.Data), secret) {
		t.Fatalf("provider secret leaked through RPC error: %+v", rpcErr)
	}
	if strings.Contains(strings.ToLower(rpcErr.Message), "provider failed") {
		t.Fatalf("provider detail leaked through RPC error: %q", rpcErr.Message)
	}
}

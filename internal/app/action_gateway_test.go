package app

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	genassembly "agent-vivy/internal/generated/assembly"
	"agent-vivy/internal/runtime"
	"agent-vivy/sdk/port/controlaction"
)

// gatewayActionProvider is deliberately small: the test is proving that a
// browser WebSocket can reach the compiler-emitted action only after the
// server has bound a session identity, and that the action's StartRun bridge
// receives that same connection-bound session.
var gatewayActionProvider = controlaction.ProviderFunc{
	Def: controlaction.Definition{
		ID:           "fixture.action",
		Owner:        "fixture/actions",
		Effect:       controlaction.EffectRead,
		InputSchema:  json.RawMessage(`{"type":"object","properties":{"session_id":{"type":"string","minLength":1}},"required":["session_id"],"additionalProperties":false}`),
		ResultSchema: json.RawMessage(`{"type":"object","properties":{"id":{"type":"string"},"status":{"type":"string"}},"required":["id","status"],"additionalProperties":false}`),
	},
	Fn: func(ctx context.Context, host controlaction.Host, input json.RawMessage) (json.RawMessage, error) {
		var params struct {
			SessionID string `json:"session_id"`
		}
		if err := json.Unmarshal(input, &params); err != nil {
			return nil, err
		}
		result, err := host.StartRun(ctx, controlaction.RunRequest{SessionID: params.SessionID, Text: "gateway action bridge"})
		if err != nil {
			return nil, err
		}
		return json.Marshal(result)
	},
}

func TestGatewayControlActionUsesConnectionBoundSession(t *testing.T) {
	runtime.SetEngineVersionOverride(pinnedEinoVersion)
	t.Cleanup(func() { runtime.SetEngineVersionOverride("") })
	t.Setenv("ANTHROPIC_API_KEY", "gateway-action-test-key")

	assembly := genassembly.BuildDefault()
	assembly.ActionSets = []controlaction.ProviderSet{{
		ModuleID:   "fixture/actions",
		AllowedIDs: []string{"fixture.action"},
		Providers:  []controlaction.Provider{gatewayActionProvider},
	}}
	assembly.GenerationID = "generation-gateway-action-test"
	assembly.Manifest.Modules = append(assembly.Manifest.Modules, "fixture/actions")
	assembly.Manifest.Actions = []string{"fixture.action"}

	a, err := NewWithAssembly(context.Background(), newAnthropicTestConfig(t), assembly)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = a.Close() })

	server := httptest.NewServer(a.httpServer.Handler)
	t.Cleanup(server.Close)
	client := connectSmokeRPC(t, server.URL, a.rpcToken)
	t.Cleanup(func() { _ = client.conn.Close() })

	callSmoke(t, client, "initialize", map[string]any{"protocol_version": "vivy.rpc.v1"})
	created := callSmoke(t, client, "session/create", map[string]string{"title": "gateway action"})
	var session struct {
		ID string `json:"id"`
	}
	decodeSmoke(t, created, &session)
	if session.ID == "" {
		t.Fatal("session/create returned no id")
	}

	result := callSmoke(t, client, "module.action.invoke", map[string]any{
		"module_id": "fixture/actions",
		"action_id": "fixture.action",
		"input":     map[string]string{"session_id": session.ID},
	})
	var run controlaction.RunResult
	decodeSmoke(t, result, &run)
	if run.ID == "" || run.Status != "accepted" {
		t.Fatalf("gateway action StartRun result = %+v", run)
	}
}

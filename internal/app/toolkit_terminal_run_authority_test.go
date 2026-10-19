package app

import (
	"context"
	"fmt"
	"testing"
	"time"

	"agent-vivy/internal/config"
	"agent-vivy/internal/domain"
	genassembly "agent-vivy/internal/generated/assembly"
	"agent-vivy/internal/runtime"
)

func TestToolkitPreviewAfterViewingTerminalRun(t *testing.T) {
	runtime.SetEngineVersionOverride(pinnedEinoVersion)
	t.Cleanup(func() { runtime.SetEngineVersionOverride("") })

	cfg := newDeepSeekTestConfig(t)
	cfg.Governance = config.Default().Governance
	a, err := NewWithAssembly(context.Background(), cfg, genassembly.BuildDefault(), WithoutEars(), WithoutGateway())
	if err != nil {
		t.Fatalf("compose app: %v", err)
	}
	t.Cleanup(func() { _ = a.Close() })
	client, err := a.DialControl(context.Background(), nil)
	if err != nil {
		t.Fatalf("dial control: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	callControl(t, client, "initialize", map[string]any{"protocol_version": "vivy.rpc.v1"})
	session := callControl(t, client, "session/create", map[string]any{"title": "terminal run authority"})
	sessionID, _ := session["session_id"].(string)
	if sessionID == "" {
		sessionID, _ = session["id"].(string)
	}
	if sessionID == "" {
		t.Fatalf("session/create returned no session id: %v", session)
	}

	state := toolkitAction(t, client, "vivy.bundles.list", map[string]any{})
	source := writeToolkitLifecycleFixture(t)
	for index, status := range []domain.RunStatus{
		domain.RunCompleted,
		domain.RunFailed,
		domain.RunCancelled,
		domain.RunActive,
	} {
		runID := domain.RunID(fmt.Sprintf("run_toolkit_authority_%d", index))
		if err := a.backend.CreateRun(context.Background(), domain.Run{
			ID: runID, SessionID: domain.SessionID(sessionID), Status: status, CreatedAt: time.Now().UnixMilli(),
		}); err != nil {
			t.Fatalf("create %s run: %v", status, err)
		}
		callControl(t, client, "run/get", map[string]any{"run_id": string(runID)})

		operationID := fmt.Sprintf("30000000-0000-4000-8000-%012d", index+10)
		_, err := client.Call(context.Background(), "module.action.invoke", map[string]any{
			"module_id": "vivy/toolkits",
			"action_id": "vivy.bundles.preview",
			"input": map[string]any{
				"operation_id":     operationID,
				"expected_catalog": state["catalog"],
				"location":         source,
			},
		})
		if status.Terminal() && err != nil {
			t.Errorf("preview after viewing %s run: %v", status, err)
		}
		if status == domain.RunActive && err == nil {
			t.Error("preview during an active run was admitted as direct human control")
		}
	}
}

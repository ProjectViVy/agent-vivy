package app

// Lifetime evidence for the embedded host owner (agent-diva DN-L /
// GATEWAYLESS-LIFETIME-EVIDENCE): a gateway-less App.Run must block on its
// context and must not return just because no listener exists, and the
// embedded lifecycle owner must keep the interaction sweeper running so
// approvals expire durably.

import (
	"context"
	"testing"
	"time"

	"agent-vivy/internal/config"
	"agent-vivy/internal/runtime"
)

// TestGatewaylessRunBlocksUntilContextCancel demonstrates the premature
// return: with no HTTP listener there is no server error to report, so Run
// must wait on ctx alone. The baseline closed errCh immediately and
// returned nil while the host was still expected to be live.
func TestGatewaylessRunBlocksUntilContextCancel(t *testing.T) {
	a, _ := composeGatewayless(t)

	runCtx, cancel := context.WithCancel(context.Background())
	runDone := make(chan error, 1)
	go func() { runDone <- a.Run(runCtx) }()

	select {
	case err := <-runDone:
		t.Fatalf("gateway-less Run returned before ctx cancel: %v", err)
	case <-time.After(500 * time.Millisecond):
	}

	cancel()
	select {
	case err := <-runDone:
		if err != nil {
			t.Fatalf("gateway-less Run returned %v, want nil", err)
		}
	case <-time.After(shutdownGrace + 10*time.Second):
		t.Fatalf("gateway-less Run did not return after cancel")
	}
}

// TestStartEmbeddedServicesExpiresApprovals proves the embedded lifecycle
// owner starts the interaction sweeper Run used to own: a pending approval
// under a short expiration resolves to expired without any Run call.
func TestStartEmbeddedServicesExpiresApprovals(t *testing.T) {
	runtime.SetEngineVersionOverride(pinnedEinoVersion)
	t.Cleanup(func() { runtime.SetEngineVersionOverride("") })
	t.Setenv("DEEPSEEK_API_KEY", "facehost-test-key")
	t.Setenv("VIVY_PROVIDER", "deepseek")
	srv := scriptedDeepSeekServer(t, "loopback note", "loopback done")
	t.Setenv("VIVY_API_BASE", srv.URL)

	cfg := newDeepSeekTestConfig(t)
	cfg.Tools.Approval = config.Approval{Expiration: 400 * time.Millisecond}
	a, err := New(context.Background(), cfg, WithoutEars(), WithoutGateway())
	if err != nil {
		t.Fatalf("compose: %v", err)
	}
	t.Cleanup(func() { _ = a.Close() })
	a.StartEmbeddedServices()

	dialCtx, cancelDial := context.WithCancel(context.Background())
	t.Cleanup(cancelDial)
	client, err := a.DialControl(dialCtx, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	session := callControl(t, client, "session/create", map[string]any{"title": "sweeper"})
	sessionID, _ := session["id"].(string)
	turn := callControl(t, client, "turn/start", map[string]any{"session_id": sessionID, "text": "needs approval"})
	runID, _ := turn["run_id"].(string)

	var approvalID string
	waitFor(t, 10*time.Second, func() bool {
		list := callControl(t, client, "approval/list", nil)
		for _, item := range list["approvals"].([]any) {
			m, _ := item.(map[string]any)
			approvalID, _ = m["id"].(string)
		}
		return approvalID != ""
	})

	// The approval must expire on its own — no respond call.
	waitFor(t, 10*time.Second, func() bool {
		list := callControl(t, client, "approval/list", nil)
		return len(list["approvals"].([]any)) == 0
	})

	// And the run terminates durably (failed or cancelled, not active).
	waitFor(t, 10*time.Second, func() bool {
		run := callControl(t, client, "run/get", map[string]any{"run_id": runID})
		status, _ := run["status"].(string)
		return status == "failed" || status == "cancelled"
	})
}

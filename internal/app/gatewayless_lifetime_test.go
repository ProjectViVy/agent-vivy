package app

// A gateway-less App.Run must block on its context, keep lifecycle services
// running while faces drive the in-process control plane, and stop them on
// cancellation.

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
// returned nil while the App was still expected to be live.
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

// TestGatewaylessRunExpiresApprovals proves App.Run owns the interaction
// sweeper: a pending approval under a short expiration resolves without a
// direct service-start call.
func TestGatewaylessRunExpiresApprovals(t *testing.T) {
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
	runCtx, cancelRun := context.WithCancel(context.Background())
	runDone := make(chan error, 1)
	go func() { runDone <- a.Run(runCtx) }()
	t.Cleanup(func() {
		cancelRun()
		select {
		case err := <-runDone:
			if err != nil {
				t.Errorf("Run: %v", err)
			}
		case <-time.After(shutdownGrace + 10*time.Second):
			t.Error("Run did not stop after context cancellation")
		}
	})

	dialCtx, cancelDial := context.WithCancel(context.Background())
	t.Cleanup(cancelDial)
	client, err := a.DialControl(dialCtx, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

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

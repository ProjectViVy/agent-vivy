package acp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	acp "github.com/eino-contrib/acp"
)

func TestOwnedRunCleanupOnEOF(t *testing.T) {
	old := ownedCleanupBudget
	ownedCleanupBudget = 2 * time.Second
	t.Cleanup(func() { ownedCleanupBudget = old })

	status := "active"
	host := &fakeHost{callFn: func(_ context.Context, method string, _ any) (json.RawMessage, error) {
		switch method {
		case "run/cancel":
			status = "cancelled"
			return json.RawMessage(`{"status":"cancelling"}`), nil
		case "run/get":
			return json.RawMessage(fmt.Sprintf(`{"id":"run_1","status":%q}`, status)), nil
		}
		return json.RawMessage(`{}`), nil
	}}
	a := newAgent(host)
	p := newTestPrompt("run_1")
	a.sessions["sess_test"] = &sessionState{root: "/tmp", active: p}
	if err := a.closeOwned(); err != nil {
		t.Fatalf("closeOwned: %v", err)
	}
	if n := host.callCount("run/cancel"); n != 1 {
		t.Fatalf("run/cancel calls = %d, want exactly one owned run", n)
	}
	if !a.draining {
		t.Fatal("teardown did not stop admission")
	}
}

func TestCleanupCancellationFailureIsHonest(t *testing.T) {
	old := ownedCleanupBudget
	ownedCleanupBudget = 300 * time.Millisecond
	t.Cleanup(func() { ownedCleanupBudget = old })

	host := &fakeHost{callFn: func(_ context.Context, method string, _ any) (json.RawMessage, error) {
		switch method {
		case "run/cancel":
			return nil, errors.New("cancel rejected")
		case "run/get":
			return json.RawMessage(`{"id":"run_1","status":"active"}`), nil
		}
		return json.RawMessage(`{}`), nil
	}}
	a := newAgent(host)
	p := newTestPrompt("run_1")
	a.sessions["sess_test"] = &sessionState{root: "/tmp", active: p}
	err := a.closeOwned()
	if err == nil {
		t.Fatal("closeOwned claimed success while the run stayed active")
	}
	if !strings.Contains(err.Error(), "run_1") || !strings.Contains(err.Error(), "not terminal") {
		t.Fatalf("cleanup error = %v", err)
	}
	if strings.Contains(err.Error(), "cancelled") {
		t.Fatalf("cleanup error claims cancellation: %v", err)
	}
}

func TestBrokenPipeWithPendingReviewNeverApproves(t *testing.T) {
	host := approvalHost(approvalJSON("sess_test", "run_1", ""), nil)
	h := newInteractionHarness(t, host)
	h.client.gate = make(chan struct{})
	p := newTestPrompt("run_1")
	proj := newRunProjection(h.agent.nonce, p.scope)
	h.agent.startInteraction(context.Background(), p, proj, approvalEvent(t, "apr_1", "tc_1", "write_file", time.Minute))

	select {
	case <-h.client.asked:
	case <-time.After(10 * time.Second):
		t.Fatal("client was never asked for permission")
	}
	h.close() // client transport disappears while the dialog is pending
	if err := h.agent.closeOwned(); err != nil {
		t.Fatalf("closeOwned after broken pipe: %v", err)
	}
	time.Sleep(200 * time.Millisecond)
	if host.callCount("approval/respond") != 0 {
		var responded struct {
			Decision string `json:"decision"`
		}
		host.callParams(t, "approval/respond", &responded)
		if responded.Decision == decisionApproved {
			t.Fatal("broken pipe fabricated an approval")
		}
	}
}

func TestOrphanedOwnedRunIsCancelled(t *testing.T) {
	old := ownedCleanupBudget
	ownedCleanupBudget = 2 * time.Second
	t.Cleanup(func() { ownedCleanupBudget = old })

	status := "active"
	host := &fakeHost{callFn: func(_ context.Context, method string, _ any) (json.RawMessage, error) {
		switch method {
		case "run/cancel":
			status = "cancelled"
			return json.RawMessage(`{"status":"cancelling"}`), nil
		case "run/get":
			return json.RawMessage(fmt.Sprintf(`{"id":"run_orphan","status":%q}`, status)), nil
		}
		return json.RawMessage(`{}`), nil
	}}
	a := newAgent(host)
	a.trackRun("run_orphan")
	if err := a.closeOwned(); err != nil {
		t.Fatalf("closeOwned: %v", err)
	}
	if n := host.callCount("run/cancel"); n != 1 {
		t.Fatalf("run/cancel calls = %d, want the orphaned owned run cancelled once", n)
	}
	if len(a.ownedRuns) != 0 {
		t.Fatalf("owned runs after settle = %v", a.ownedRuns)
	}
}

func TestSessionCancelDoesNotWaitForPendingDialog(t *testing.T) {
	host := approvalHost(approvalJSON("sess_test", "run_1", ""), nil)
	h := newInteractionHarness(t, host)
	h.client.gate = make(chan struct{}) // dialog stays unanswered
	p := newTestPrompt("run_1")
	h.agent.sessions["sess_test"] = &sessionState{root: "/tmp", active: p}
	proj := newRunProjection(h.agent.nonce, p.scope)
	h.agent.startInteraction(context.Background(), p, proj, approvalEvent(t, "apr_1", "tc_1", "write_file", time.Minute))
	select {
	case <-h.client.asked:
	case <-time.After(10 * time.Second):
		t.Fatal("client was never asked for permission")
	}

	started := time.Now()
	if err := h.agent.SessionCancel(context.Background(), acp.CancelNotification{SessionID: "sess_test"}); err != nil {
		t.Fatalf("SessionCancel: %v", err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("SessionCancel blocked for %s behind the pending dialog", elapsed)
	}
	if !p.isClientCancelled() {
		t.Fatal("client cancel was not recorded on the prompt")
	}
	waitFor(t, "run/cancel", func() bool { return host.callCount("run/cancel") == 1 })
}

func TestPermissionUnsupportedClientDenies(t *testing.T) {
	host := approvalHost(approvalJSON("sess_test", "run_1", ""), nil)
	h := newInteractionHarness(t, host)
	h.client.permissionErr = errors.New("method not supported")
	p := newTestPrompt("run_1")
	proj := newRunProjection(h.agent.nonce, p.scope)
	h.agent.startInteraction(context.Background(), p, proj, approvalEvent(t, "apr_1", "tc_1", "write_file", time.Minute))

	waitFor(t, "approval/respond", func() bool { return host.callCount("approval/respond") == 1 })
	var responded struct {
		Decision string `json:"decision"`
		Reason   string `json:"reason"`
	}
	host.callParams(t, "approval/respond", &responded)
	if responded.Decision != decisionDenied || responded.Reason == "" {
		t.Fatalf("unsupported client response = %+v, want denied with a reason", responded)
	}
	if strings.Contains(responded.Reason, "method not supported") {
		t.Fatalf("reason leaked the client error: %q", responded.Reason)
	}
}

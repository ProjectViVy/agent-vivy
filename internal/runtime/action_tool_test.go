package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/testsupport"
	"agent-vivy/internal/tools"
)

func TestInvokeActionToolUsesPinnedServiceToolHostAndJournal(t *testing.T) {
	ctx := context.Background()
	svc, backend, _ := newTestService(t, testsupport.NewEchoModel())
	sessionID := domain.SessionID("action-session")
	runID := domain.RunID("action-run")
	if err := backend.CreateSession(ctx, domain.Session{
		ID: sessionID, CreatedAt: time.Now().UnixMilli(),
		SandboxMode:    string(domain.SandboxModeWorkspaceWrite),
		ApprovalPolicy: string(domain.ApprovalPolicyNever),
	}); err != nil {
		t.Fatalf("create session: %v", err)
	}
	if err := backend.CreateRun(ctx, domain.Run{ID: runID, SessionID: sessionID, Status: domain.RunActive, CreatedAt: time.Now().UnixMilli()}); err != nil {
		t.Fatalf("create run: %v", err)
	}
	snapshot, err := svc.engine.cfg.Policy.Snapshot(domain.PolicyProfileDefault)
	if err != nil {
		t.Fatalf("snapshot policy: %v", err)
	}
	ledger, err := NewBudgetLedger(DefaultBudgetPolicy())
	if err != nil {
		t.Fatalf("new budget ledger: %v", err)
	}
	svc.mu.Lock()
	svc.active[runID] = func() {}
	svc.runSessions[runID] = sessionID
	svc.runTools[runID] = map[string]struct{}{tools.EchoInfoName: {}}
	svc.snapshots[runID] = snapshot
	svc.ledgers[runID] = ledger
	svc.mu.Unlock()
	t.Cleanup(func() { svc.cleanupRunState(runID) })

	// The wrong session is rejected by the service-owned run/session fence;
	// it must not create a journal lifecycle event or reach the ToolHost.
	if _, err := svc.InvokeActionTool(ctx, "forged-session", runID, tools.EchoInfoName, json.RawMessage(`{"text":"must not run"}`)); !errors.Is(err, ErrPolicyDenied) {
		t.Fatalf("wrong-session result = %v, want policy denial", err)
	}
	if _, err := svc.InvokeActionTool(ctx, sessionID, runID, "not-selected", json.RawMessage(`{}`)); !errors.Is(err, ErrPolicyDenied) {
		t.Fatalf("unselected-tool result = %v, want policy denial", err)
	}

	result, err := svc.InvokeActionTool(ctx, sessionID, runID, tools.EchoInfoName, json.RawMessage(`{"text":"from action"}`))
	if err != nil {
		t.Fatalf("invoke action tool: %v", err)
	}
	if !strings.Contains(result, "from action") {
		t.Fatalf("result = %q, want authoritative echo output", result)
	}

	iter, err := backend.Replay(ctx, runID, 0)
	if err != nil {
		t.Fatalf("replay run journal: %v", err)
	}
	defer func() { _ = iter.Close() }()
	var events []domain.RunEvent
	for iter.Next() {
		events = append(events, iter.Value().Event)
	}
	if err := iter.Err(); err != nil {
		t.Fatalf("replay iterator: %v", err)
	}
	if len(events) < 3 {
		t.Fatalf("journal events = %d, want policy + started + finished: %+v", len(events), events)
	}
	var sawPolicy, sawStarted, sawFinished bool
	for _, event := range events {
		switch event.Type {
		case domain.EventPolicyEvaluated:
			sawPolicy = true
		case domain.EventToolStarted:
			sawStarted = true
		case domain.EventToolFinished:
			sawFinished = true
			var payload payloadToolFinished
			if err := json.Unmarshal(event.Payload, &payload); err != nil {
				t.Fatalf("decode tool.finished: %v", err)
			}
			if !strings.Contains(payload.Result, "from action") {
				t.Fatalf("tool.finished result = %q, want authoritative result", payload.Result)
			}
		}
	}
	if !sawPolicy || !sawStarted || !sawFinished {
		t.Fatalf("journal lifecycle = policy:%v started:%v finished:%v; events=%+v", sawPolicy, sawStarted, sawFinished, events)
	}

	run, err := backend.GetRun(ctx, runID)
	if err != nil {
		t.Fatalf("get run: %v", err)
	}
	if run.Status != domain.RunActive {
		t.Fatalf("action tool changed run status to %q, want active", run.Status)
	}

	// A missing authoritative session store is fail-closed even when all
	// process-local run maps look active; action bridges must not fall back to
	// defaults outside the real Service/Journal/ToolHost composition.
	sessions := svc.deps.Sessions
	svc.deps.Sessions = nil
	if _, err := svc.InvokeActionTool(ctx, sessionID, runID, tools.EchoInfoName, json.RawMessage(`{"text":"no fallback"}`)); !errors.Is(err, ErrPolicyDenied) {
		t.Fatalf("missing session store result = %v, want policy denial", err)
	}
	svc.deps.Sessions = sessions
}

package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/sqlite"
)

type brokerTestTool struct {
	spec   domain.ToolSpec
	called bool
}

func (t *brokerTestTool) Spec() domain.ToolSpec { return t.spec }

func (t *brokerTestTool) InvokableRun(_ context.Context, args json.RawMessage) (string, error) {
	t.called = true
	return string(args), nil
}

type brokerReplayProbeTool struct {
	spec  domain.ToolSpec
	calls int
}

func (t *brokerReplayProbeTool) Spec() domain.ToolSpec { return t.spec }

func (t *brokerReplayProbeTool) InvokableRun(_ context.Context, _ json.RawMessage) (string, error) {
	t.calls++
	return "fixture invoked", nil
}

func TestBrokerOperationIdentityReusesResultAndRejectsConflict(t *testing.T) {
	service, backend, runID := newBrokerOperationService(t)
	policy, err := NewPolicyEngine(nil)
	if err != nil {
		t.Fatal(err)
	}
	tool := &brokerReplayProbeTool{spec: domain.ToolSpec{
		Name: "write_note", Params: map[string]domain.ToolParam{"text": {Required: true}},
	}}
	hook := &countingRewriteHook{}
	hooks := NewToolHookChain(time.Second, hook)
	args := map[string]string{"text": "same direct input"}

	for attempt := 0; attempt < 2; attempt++ {
		if _, err := service.ExecuteBrokerToolOperation(context.Background(), "call-stable", tool, policy, hooks, runID, domain.PolicyProfileFullAuto, args, 1024, false); err != nil {
			t.Fatalf("broker operation attempt %d: %v", attempt+1, err)
		}
	}

	if tool.calls != 1 || hook.preCalls != 1 {
		t.Fatalf("same operation invoked %d times and ran pre-hook %d times, want one each", tool.calls, hook.preCalls)
	}
	conflict := map[string]string{"text": "changed payload"}
	if _, err := service.ExecuteBrokerToolOperation(context.Background(), "call-stable", tool, policy, hooks, runID, domain.PolicyProfileFullAuto, conflict, 1024, false); !errors.Is(err, storage.ErrToolOperationConflict) {
		t.Fatalf("same operation id with changed payload = %v, want conflict", err)
	}
	if _, err := service.ExecuteBrokerToolOperation(context.Background(), "call-distinct", tool, policy, hooks, runID, domain.PolicyProfileFullAuto, args, 1024, false); err != nil {
		t.Fatalf("distinct same-argument operation: %v", err)
	}
	if tool.calls != 2 || hook.preCalls != 2 {
		t.Fatalf("distinct operation calls=%d pre-hooks=%d, want two each", tool.calls, hook.preCalls)
	}
	if _, err := service.ExecuteBrokerToolOperation(context.Background(), "call-approved", tool, policy, nil, runID, domain.PolicyProfileDefault, args, 1024, true); err != nil {
		t.Fatalf("approved broker operation: %v", err)
	}

	operationStore := storage.ToolOperationStore(backend)
	completed, err := operationStore.GetToolOperation(context.Background(), runID, "call-stable")
	if err != nil || completed.State != domain.ToolOperationCompleted || completed.Result == "" {
		t.Fatalf("durable broker operation = %+v err=%v", completed, err)
	}
}

func TestBrokerOperationCompletionWriteFailureBlocksReplay(t *testing.T) {
	service, backend, runID := newBrokerOperationService(t)
	service.deps.ToolOperations = failingToolOperationCompletionStore{ToolOperationStore: backend, err: errors.New("completion disk full")}
	policy, err := NewPolicyEngine(nil)
	if err != nil {
		t.Fatal(err)
	}
	tool := &brokerReplayProbeTool{spec: domain.ToolSpec{
		Name: "write_note", Params: map[string]domain.ToolParam{"text": {Required: true}},
	}}
	args := map[string]string{"text": "one effect"}
	if _, err := service.ExecuteBrokerToolOperation(context.Background(), "call-disk-failure", tool, policy, nil, runID, domain.PolicyProfileFullAuto, args, 1024, false); err == nil || !strings.Contains(err.Error(), "persist tool operation completion") {
		t.Fatalf("completion persistence error = %v, want surfaced storage failure", err)
	}
	if tool.calls != 1 {
		t.Fatalf("tool calls after failed completion = %d, want one", tool.calls)
	}
	if _, err := service.ExecuteBrokerToolOperation(context.Background(), "call-disk-failure", tool, policy, nil, runID, domain.PolicyProfileFullAuto, args, 1024, false); !errors.Is(err, ErrToolOperationUnknown) {
		t.Fatalf("retry after uncertain completion = %v, want unknown outcome fence", err)
	}
	if tool.calls != 1 {
		t.Fatalf("tool replayed after completion write failure: calls=%d", tool.calls)
	}
}

func TestBrokerApprovalCannotOverridePolicyDeny(t *testing.T) {
	makePolicy := func(decision domain.PolicyDecision) *PolicyEngine {
		policy, err := NewPolicyEngine(map[domain.PolicyProfile]PolicyDefinition{
			domain.PolicyProfileDefault: {
				Rules: []PolicyRule{{Tool: "write_note", Decision: decision}},
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		return policy
	}
	args := map[string]string{"text": "approved earlier"}

	t.Run("deny on initial approved execution", func(t *testing.T) {
		service, _, runID := newBrokerOperationService(t)
		tool := &brokerReplayProbeTool{spec: domain.ToolSpec{
			Name: "write_note", Params: map[string]domain.ToolParam{"text": {Required: true}},
		}}
		_, err := service.ExecuteBrokerToolOperation(context.Background(), "call-initial-denied", tool, makePolicy(domain.PolicyDeny), nil, runID, domain.PolicyProfileDefault, args, 1024, true)
		if !errors.Is(err, ErrPolicyDenied) {
			t.Fatalf("approved broker call under deny policy = %v, want ErrPolicyDenied", err)
		}
		if tool.calls != 0 {
			t.Fatalf("denied approved broker call invoked tool %d times, want zero", tool.calls)
		}
	})

	t.Run("deny after approval and operation completion", func(t *testing.T) {
		service, _, runID := newBrokerOperationService(t)
		tool := &brokerReplayProbeTool{spec: domain.ToolSpec{
			Name: "write_note", Params: map[string]domain.ToolParam{"text": {Required: true}},
		}}
		operationID := "call-policy-changed-after-approval"
		if _, err := service.ExecuteBrokerToolOperation(context.Background(), operationID, tool, makePolicy(domain.PolicyPrompt), nil, runID, domain.PolicyProfileDefault, args, 1024, true); err != nil {
			t.Fatalf("approved prompt-policy execution: %v", err)
		}
		if _, err := service.ExecuteBrokerToolOperation(context.Background(), operationID, tool, makePolicy(domain.PolicyDeny), nil, runID, domain.PolicyProfileDefault, args, 1024, true); !errors.Is(err, ErrPolicyDenied) {
			t.Fatalf("approved operation replay after policy changed to deny = %v, want ErrPolicyDenied", err)
		}
		if tool.calls != 1 {
			t.Fatalf("policy-denied replay invoked tool again: calls=%d, want one", tool.calls)
		}
	})
}

type failingToolOperationCompletionStore struct {
	storage.ToolOperationStore
	err error
}

func (store failingToolOperationCompletionStore) CompleteToolOperation(context.Context, domain.RunID, string, string, string, string) (domain.ToolOperation, domain.RunEvent, error) {
	return domain.ToolOperation{}, domain.RunEvent{}, store.err
}

type countingRewriteHook struct{ preCalls int }

func (h *countingRewriteHook) Name() string { return "counting-rewrite" }
func (h *countingRewriteHook) PreToolUse(context.Context, ToolHookCall) (PreToolUseResult, error) {
	h.preCalls++
	return PreToolUseResult{Decision: hookRewrite, UpdatedArgs: json.RawMessage(`{"text":"effective input"}`)}, nil
}
func (h *countingRewriteHook) PostToolUse(context.Context, ToolHookCall, string, error) error {
	return nil
}

func newBrokerOperationService(t *testing.T) (*Service, *sqlite.Backend, domain.RunID) {
	t.Helper()
	ctx := context.Background()
	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "broker-operations.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = backend.Close() })
	sessionID := domain.SessionID("session-broker-operations")
	runID := domain.RunID("run-broker-operations")
	if err := backend.CreateSession(ctx, domain.Session{ID: sessionID, Title: "broker", CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	if err := backend.CreateRun(ctx, domain.Run{ID: runID, SessionID: sessionID, Status: domain.RunActive, CreatedAt: 2}); err != nil {
		t.Fatal(err)
	}
	if _, err := backend.Append(ctx, storage.Commit{RunID: runID, Events: []domain.RunEvent{{Type: domain.EventRunStarted, CreatedAt: 2, PayloadVersion: 1, Payload: []byte(`{}`)}}}); err != nil {
		t.Fatal(err)
	}
	engine := &Engine{cfg: EngineConfig{MaxEventPayloadBytes: 64 << 10}}
	service := NewService(engine, "test", "test-model", ServiceDeps{Journal: backend, Runs: backend, Sink: newTestSink()})
	return service, backend, runID
}

func TestBrokerOperationKeepsParentPolicyAuthority(t *testing.T) {
	service, _, runID := newBrokerOperationService(t)
	policy, err := NewPolicyEngine(nil)
	if err != nil {
		t.Fatal(err)
	}
	tool := &brokerTestTool{spec: domain.ToolSpec{
		Name: "write_note", Params: map[string]domain.ToolParam{"text": {Required: true}},
	}}
	_, err = service.ExecuteBrokerToolOperation(context.Background(), "call-blocked", tool, policy, nil, runID, domain.PolicyProfileDefault, map[string]string{"text": "blocked"}, 1024, false)
	if err == nil {
		t.Fatal("default profile must not let a worker bypass approval")
	}
	if tool.called {
		t.Fatal("policy rejection must happen before invocation")
	}

	result, err := service.ExecuteBrokerToolOperation(context.Background(), "call-allowed", tool, policy, nil, runID, domain.PolicyProfileFullAuto, map[string]string{"text": "allowed"}, 1024, false)
	if err != nil {
		t.Fatalf("full_auto broker call: %v", err)
	}
	if !tool.called || result == "" {
		t.Fatalf("broker result = %q, called = %v", result, tool.called)
	}
}

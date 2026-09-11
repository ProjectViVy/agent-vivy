package actionhost

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"agent-vivy/sdk/module"
	action "agent-vivy/sdk/port/controlaction"
)

const testModule = "example/module"

var (
	testInputSchema  = json.RawMessage(`{"type":"object","properties":{"value":{"type":"string"}},"required":["value"],"additionalProperties":false}`)
	testOutputSchema = json.RawMessage(`{"type":"object","properties":{"ok":{"type":"boolean"}},"required":["ok"],"additionalProperties":false}`)
)

type providerFunc struct {
	definition action.Definition
	invoke     func(context.Context, action.Host, json.RawMessage) (json.RawMessage, error)
}

type panicDefinitionProvider struct{}

func (panicDefinitionProvider) Definition() action.Definition { panic("definition panic") }
func (panicDefinitionProvider) Invoke(context.Context, action.Host, json.RawMessage) (json.RawMessage, error) {
	return nil, nil
}

func (provider providerFunc) Definition() action.Definition { return provider.definition }
func (provider providerFunc) Invoke(ctx context.Context, host action.Host, input json.RawMessage) (json.RawMessage, error) {
	return provider.invoke(ctx, host, input)
}

func testDefinition(id string, effect action.Effect) action.Definition {
	return action.Definition{
		ID:           id,
		Owner:        testModule,
		Effect:       effect,
		InputSchema:  append(json.RawMessage(nil), testInputSchema...),
		ResultSchema: append(json.RawMessage(nil), testOutputSchema...),
	}
}

type auditRecorder struct {
	mu      sync.Mutex
	records []AuditRecord
}

func (recorder *auditRecorder) Record(_ context.Context, record AuditRecord) error {
	recorder.mu.Lock()
	recorder.records = append(recorder.records, record)
	recorder.mu.Unlock()
	return nil
}

func (recorder *auditRecorder) snapshot() []AuditRecord {
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	return append([]AuditRecord(nil), recorder.records...)
}

func readyDeps(providers ...action.Provider) Deps {
	bindings := make([]ProviderBinding, 0, len(providers))
	for _, provider := range providers {
		binding := ProviderBinding{ModuleID: testModule, Provider: provider}
		if typed, ok := provider.(providerFunc); ok {
			binding.ActionID = typed.definition.ID
		}
		bindings = append(bindings, binding)
	}
	return Deps{
		Bindings:            bindings,
		GenerationAvailable: true,
		GenerationID:        "generation-a",
		Authenticate: func(_ context.Context, caller Caller) (Identity, error) {
			if caller.Opaque() != "transport-ok" {
				return Identity{}, ErrUnauthenticated
			}
			return Identity{ID: "operator", SessionID: "session-1"}, nil
		},
		Authorize: func(_ context.Context, _ Identity, definition action.Definition, _ json.RawMessage) error {
			if definition.RequiresApproval || definition.ApprovalRequired {
				return ErrApprovalRequired
			}
			return nil
		},
		AuthorizeBridge: func(context.Context, Identity, BridgeRequest) error { return nil },
		Audit:           AuditSinkFunc(func(context.Context, AuditRecord) error { return nil }),
		AllowedTools:    map[string]struct{}{"example.tool": {}},
	}
}

func readyCaller() Caller {
	return NewCaller("transport-ok")
}

func TestActionHostDistrustsBrowserAuthorityClaims(t *testing.T) {
	called := false
	definition := testDefinition("example.action.write", action.EffectWrite)
	definition.RequiresApproval = true
	provider := providerFunc{definition: definition, invoke: func(context.Context, action.Host, json.RawMessage) (json.RawMessage, error) {
		called = true
		return json.RawMessage(`{"ok":true}`), nil
	}}
	host, err := New(readyDeps(provider))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	caller := NewCaller("forged-browser-claims")
	_, err = host.Invoke(context.Background(), caller, testModule, definition.ID, json.RawMessage(`{"value":"x"}`))
	if !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("Invoke() error = %v, want ErrUnauthenticated", err)
	}
	if called {
		t.Fatal("provider ran on caller-supplied approval claim")
	}
}

func TestActionHostValidatesInputAndOutputSchemas(t *testing.T) {
	provider := providerFunc{definition: testDefinition("example.action.read", action.EffectRead), invoke: func(context.Context, action.Host, json.RawMessage) (json.RawMessage, error) {
		return json.RawMessage(`{"wrong":true}`), nil
	}}
	host, err := New(readyDeps(provider))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	_, err = host.Invoke(context.Background(), readyCaller(), testModule, provider.definition.ID, json.RawMessage(`{"nope":1}`))
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid input error = %v, want ErrInvalidInput", err)
	}
	_, err = host.Invoke(context.Background(), readyCaller(), testModule, provider.definition.ID, json.RawMessage(`{"value":1}`))
	if !errors.Is(err, ErrInvalidInput) || strings.Contains(err.Error(), "1") {
		t.Fatalf("invalid input error = %v, want bounded schema error", err)
	}
	_, err = host.Invoke(context.Background(), readyCaller(), testModule, provider.definition.ID, json.RawMessage(`{"value":"x"}`))
	if !errors.Is(err, ErrInvalidOutput) {
		t.Fatalf("invalid output error = %v, want ErrInvalidOutput", err)
	}
}

func TestActionHostEnforcesExactProviderOwner(t *testing.T) {
	called := false
	provider := providerFunc{definition: testDefinition("example.action.read", action.EffectRead), invoke: func(context.Context, action.Host, json.RawMessage) (json.RawMessage, error) {
		called = true
		return json.RawMessage(`{"ok":true}`), nil
	}}
	recorder := &auditRecorder{}
	deps := readyDeps(provider)
	deps.Audit = recorder
	host, err := New(deps)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	_, err = host.Invoke(context.Background(), readyCaller(), "example/other", provider.definition.ID, json.RawMessage(`{"value":"x"}`))
	if !errors.Is(err, ErrOwnerMismatch) {
		t.Fatalf("Invoke() error = %v, want ErrOwnerMismatch", err)
	}
	if called {
		t.Fatal("provider ran for a mismatched owner")
	}
	records := recorder.snapshot()
	if len(records) != 1 || records[0].Event != "action.read" || records[0].Effect != action.EffectRead {
		t.Fatalf("wrong-owner audit = %+v, want action.read effect", records)
	}
}

func TestActionHostAuditInvocationIDsAreStableAndUnique(t *testing.T) {
	provider := providerFunc{definition: testDefinition("example.action.audit-ids", action.EffectRead), invoke: func(context.Context, action.Host, json.RawMessage) (json.RawMessage, error) {
		return json.RawMessage(`{"ok":true}`), nil
	}}
	recorder := &auditRecorder{}
	deps := readyDeps(provider)
	deps.Audit = recorder
	host, err := New(deps)
	if err != nil {
		t.Fatal(err)
	}
	defer host.Close()
	const count = 24
	var wait sync.WaitGroup
	for i := 0; i < count; i++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			if _, invokeErr := host.Invoke(context.Background(), readyCaller(), testModule, provider.definition.ID, json.RawMessage(`{"value":"ok"}`)); invokeErr != nil {
				t.Errorf("Invoke() error = %v", invokeErr)
			}
		}()
	}
	wait.Wait()
	records := recorder.snapshot()
	seen := make(map[string]struct{}, count)
	for _, record := range records {
		if record.Phase != "pre" {
			continue
		}
		if record.InvocationID == "" {
			t.Fatal("pre-audit record has no invocation ID")
		}
		if _, exists := seen[record.InvocationID]; exists {
			t.Fatalf("duplicate pre-audit invocation ID %q", record.InvocationID)
		}
		seen[record.InvocationID] = struct{}{}
	}
	if len(seen) != count {
		t.Fatalf("pre-audit invocation IDs = %d, want %d (records=%d)", len(seen), count, len(records))
	}
	for _, record := range records {
		if record.Phase != "completion" {
			continue
		}
		if _, exists := seen[record.InvocationID]; !exists {
			t.Fatalf("completion record %q has no matching pre-audit", record.InvocationID)
		}
	}
}

func TestActionHostRejectsUnavailableGenerationAndInstance(t *testing.T) {
	provider := providerFunc{definition: testDefinition("example.action.read", action.EffectRead), invoke: func(context.Context, action.Host, json.RawMessage) (json.RawMessage, error) {
		return json.RawMessage(`{"ok":true}`), nil
	}}
	deps := readyDeps(provider)
	deps.GenerationAvailable = false
	if _, err := New(deps); !errors.Is(err, ErrGenerationUnavailable) {
		t.Fatalf("New() generation error = %v, want ErrGenerationUnavailable", err)
	}

	definition := testDefinition("example.action.instance", action.EffectRead)
	definition.InstanceID = "live"
	provider.definition = definition
	deps = readyDeps(provider)
	deps.Instances = map[string]InstanceStatus{"example/module\x00live": {ModuleID: testModule, ID: "live", State: InstanceUnavailable}}
	host, err := New(deps)
	if err != nil {
		t.Fatalf("New(instance) error = %v", err)
	}
	if _, err := host.Invoke(context.Background(), readyCaller(), testModule, definition.ID, json.RawMessage(`{"value":"x"}`)); !errors.Is(err, ErrInstanceUnavailable) {
		t.Fatalf("instance error = %v, want ErrInstanceUnavailable", err)
	}
}

func TestActionHostDeniesMissingEffectiveGrant(t *testing.T) {
	definition := testDefinition("example.action.write", action.EffectWrite)
	definition.RequiredGrants = []module.Grant{module.GrantRPCClient}
	provider := providerFunc{definition: definition, invoke: func(context.Context, action.Host, json.RawMessage) (json.RawMessage, error) {
		return json.RawMessage(`{"ok":true}`), nil
	}}
	deps := readyDeps(provider)
	deps.EffectiveGrants = map[string][]module.GrantBinding{testModule: {}}
	host, err := New(deps)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if _, err := host.Invoke(context.Background(), readyCaller(), testModule, definition.ID, json.RawMessage(`{"value":"x"}`)); !errors.Is(err, ErrGrantDenied) {
		t.Fatalf("Invoke() error = %v, want ErrGrantDenied", err)
	}
}

func TestActionHostDoesNotWidenAnExplicitEmptyBinding(t *testing.T) {
	definition := testDefinition("example.action.bound", action.EffectRead)
	definition.RequiredGrants = []module.Grant{module.GrantRPCClient}
	provider := providerFunc{definition: definition, invoke: func(context.Context, action.Host, json.RawMessage) (json.RawMessage, error) {
		return json.RawMessage(`{"ok":true}`), nil
	}}
	deps := readyDeps()
	deps.Providers = nil
	deps.EffectiveGrants = map[string][]module.GrantBinding{testModule: {{Name: module.GrantRPCClient}}}
	deps.Bindings = []ProviderBinding{{
		ModuleID:        testModule,
		ActionID:        definition.ID,
		Provider:        provider,
		EffectiveGrants: []module.GrantBinding{},
	}}
	host, err := New(deps)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if _, err := host.Invoke(context.Background(), readyCaller(), testModule, definition.ID, json.RawMessage(`{"value":"x"}`)); !errors.Is(err, ErrGrantDenied) {
		t.Fatalf("Invoke() error = %v, want ErrGrantDenied", err)
	}
}

func TestActionHostRejectsSecretLeakAndRedactsProviderError(t *testing.T) {
	const secret = "action-secret-canary-123456"
	definition := testDefinition("example.action.secret", action.EffectRead)
	definition.RequiredGrants = []module.Grant{module.GrantSecretRead}
	provider := providerFunc{definition: definition, invoke: func(_ context.Context, host action.Host, _ json.RawMessage) (json.RawMessage, error) {
		value, err := host.Secret("API_TOKEN")
		if err != nil {
			return nil, err
		}
		return json.RawMessage(`{"ok":true,"token":"` + value + `"}`), nil
	}}
	deps := readyDeps(provider)
	deps.Bindings[0].EffectiveGrants = []module.GrantBinding{{Name: module.GrantSecretRead, Constraints: map[string][]string{"names": {"API_TOKEN"}}}}
	deps.ResolveSecret = func(context.Context, string, string, string) (string, error) { return secret, nil }
	host, err := New(deps)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	_, err = host.Invoke(context.Background(), readyCaller(), testModule, definition.ID, json.RawMessage(`{"value":"x"}`))
	if !errors.Is(err, ErrSecretLeak) {
		t.Fatalf("Invoke() error = %v, want ErrSecretLeak", err)
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("secret leaked through error: %v", err)
	}

	definition.ID = "example.action.error"
	provider.definition = definition
	provider.invoke = func(context.Context, action.Host, json.RawMessage) (json.RawMessage, error) {
		return nil, errors.New("provider failed with " + secret)
	}
	host, err = New(readyDeps(provider))
	if err != nil {
		t.Fatalf("New(error) error = %v", err)
	}
	_, err = host.Invoke(context.Background(), readyCaller(), testModule, definition.ID, json.RawMessage(`{"value":"x"}`))
	if err == nil || strings.Contains(err.Error(), secret) {
		t.Fatalf("provider error = %v, secret must be redacted", err)
	}
}

func TestActionHostRejectsCredentialShapedOutput(t *testing.T) {
	definition := testDefinition("example.action.credential", action.EffectRead)
	provider := providerFunc{definition: definition, invoke: func(context.Context, action.Host, json.RawMessage) (json.RawMessage, error) {
		return json.RawMessage(`{"ok":true,"token":"sk-live-abcdefghijkl"}`), nil
	}}
	host, err := New(readyDeps(provider))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if _, err := host.Invoke(context.Background(), readyCaller(), testModule, definition.ID, json.RawMessage(`{"value":"x"}`)); !errors.Is(err, ErrSecretLeak) {
		t.Fatalf("credential output error = %v, want ErrSecretLeak", err)
	}
}

func TestActionHostTimeoutAndCancellation(t *testing.T) {
	definition := testDefinition("example.action.slow", action.EffectRead)
	provider := providerFunc{definition: definition, invoke: func(ctx context.Context, _ action.Host, _ json.RawMessage) (json.RawMessage, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}}
	deps := readyDeps(provider)
	deps.Timeout = 10 * time.Millisecond
	host, err := New(deps)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	_, err = host.Invoke(context.Background(), readyCaller(), testModule, definition.ID, json.RawMessage(`{"value":"x"}`))
	if !errors.Is(err, ErrActionTimeout) {
		t.Fatalf("timeout error = %v, want ErrActionTimeout", err)
	}
	definition.Timeout = time.Millisecond
	deps = readyDeps(providerFunc{definition: definition, invoke: provider.invoke})
	deps.Timeout = time.Second
	host, err = New(deps)
	if err != nil {
		t.Fatalf("New(action timeout) error = %v", err)
	}
	_, err = host.Invoke(context.Background(), readyCaller(), testModule, definition.ID, json.RawMessage(`{"value":"x"}`))
	if !errors.Is(err, ErrActionTimeout) {
		t.Fatalf("action timeout error = %v, want ErrActionTimeout", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = host.Invoke(ctx, readyCaller(), testModule, definition.ID, json.RawMessage(`{"value":"x"}`))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel error = %v, want context.Canceled", err)
	}
}

func TestActionHostDoesNotExposeCapabilitiesAfterTimeout(t *testing.T) {
	definition := testDefinition("example.action.timeout-capability", action.EffectWrite)
	var startCalls, toolCalls int
	provider := providerFunc{definition: definition, invoke: func(ctx context.Context, host action.Host, _ json.RawMessage) (json.RawMessage, error) {
		<-ctx.Done()
		if _, err := host.StartRun(context.Background(), action.RunRequest{SessionID: "session-1", Text: "late"}); err == nil {
			t.Error("StartRun succeeded after action timeout")
		}
		if _, err := host.InvokeTool(context.Background(), "example.tool", json.RawMessage(`{"value":"late"}`)); err == nil {
			t.Error("InvokeTool succeeded after action timeout")
		}
		return nil, ctx.Err()
	}}
	deps := readyDeps(provider)
	deps.Timeout = 10 * time.Millisecond
	deps.StartRun = func(context.Context, action.RunRequest) (action.RunResult, error) {
		startCalls++
		return action.RunResult{ID: "late"}, nil
	}
	deps.InvokeTool = func(context.Context, string, json.RawMessage) (json.RawMessage, error) {
		toolCalls++
		return json.RawMessage(`{"ok":true}`), nil
	}
	host, err := New(deps)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if _, err := host.Invoke(context.Background(), readyCaller(), testModule, definition.ID, json.RawMessage(`{"value":"x"}`)); !errors.Is(err, ErrActionTimeout) {
		t.Fatalf("Invoke() error = %v, want ErrActionTimeout", err)
	}
	if startCalls != 0 || toolCalls != 0 {
		t.Fatalf("late capabilities called StartRun=%d InvokeTool=%d", startCalls, toolCalls)
	}
}

func TestActionHostRequiresExactInstanceGeneration(t *testing.T) {
	definition := testDefinition("example.action.generation", action.EffectRead)
	definition.InstanceID = "live"
	provider := providerFunc{definition: definition, invoke: func(context.Context, action.Host, json.RawMessage) (json.RawMessage, error) {
		return json.RawMessage(`{"ok":true}`), nil
	}}
	deps := readyDeps(provider)
	deps.GenerationID = "generation-a"
	deps.Instances = map[string]InstanceStatus{
		testModule + "\x00live": {ModuleID: testModule, ID: "live", State: InstanceUnavailable, Available: true, GenerationID: "generation-a"},
	}
	host, err := New(deps)
	if err != nil {
		t.Fatalf("New(unavailable) error = %v", err)
	}
	if _, err := host.Invoke(context.Background(), readyCaller(), testModule, definition.ID, json.RawMessage(`{"value":"x"}`)); !errors.Is(err, ErrInstanceUnavailable) {
		t.Fatalf("unavailable instance error = %v, want ErrInstanceUnavailable", err)
	}
	deps.Instances[testModule+"\x00live"] = InstanceStatus{ModuleID: testModule, ID: "live", State: InstanceReady, GenerationID: "generation-b"}
	host, err = New(deps)
	if err != nil {
		t.Fatalf("New(generation) error = %v", err)
	}
	if _, err := host.Invoke(context.Background(), readyCaller(), testModule, definition.ID, json.RawMessage(`{"value":"x"}`)); !errors.Is(err, ErrInstanceUnavailable) {
		t.Fatalf("generation mismatch error = %v, want ErrInstanceUnavailable", err)
	}
}

func TestActionHostBoundsBridgeContexts(t *testing.T) {
	definition := testDefinition("example.action.bridge-timeout", action.EffectWrite)
	provider := providerFunc{definition: definition, invoke: func(_ context.Context, host action.Host, _ json.RawMessage) (json.RawMessage, error) {
		_, err := host.StartRun(context.Background(), action.RunRequest{SessionID: "session-1", Text: "hello"})
		return nil, err
	}}
	bridgeCancelled := make(chan struct{})
	deps := readyDeps(provider)
	deps.Timeout = 10 * time.Millisecond
	deps.StartRun = func(ctx context.Context, _ action.RunRequest) (action.RunResult, error) {
		<-ctx.Done()
		close(bridgeCancelled)
		return action.RunResult{}, ctx.Err()
	}
	host, err := New(deps)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if _, err := host.Invoke(context.Background(), readyCaller(), testModule, definition.ID, json.RawMessage(`{"value":"x"}`)); !errors.Is(err, ErrActionTimeout) {
		t.Fatalf("bridge timeout error = %v, want ErrActionTimeout", err)
	}
	select {
	case <-bridgeCancelled:
	case <-time.After(time.Second):
		t.Fatal("bridge callback outlived the Host timeout")
	}
}

func TestActionHostEmitsEffectSpecificAudit(t *testing.T) {
	for _, effect := range []action.Effect{action.EffectRead, action.EffectWrite, action.EffectExternalEffect} {
		definition := testDefinition("example.action."+string(effect), effect)
		provider := providerFunc{definition: definition, invoke: func(context.Context, action.Host, json.RawMessage) (json.RawMessage, error) {
			return json.RawMessage(`{"ok":true}`), nil
		}}
		recorder := &auditRecorder{}
		deps := readyDeps(provider)
		deps.Audit = recorder
		host, err := New(deps)
		if err != nil {
			t.Fatalf("New(%s) error = %v", effect, err)
		}
		if _, err := host.Invoke(context.Background(), readyCaller(), testModule, definition.ID, json.RawMessage(`{"value":"x"}`)); err != nil {
			t.Fatalf("Invoke(%s) error = %v", effect, err)
		}
		records := recorder.snapshot()
		if len(records) != 2 || records[0].Phase != "pre" || records[0].Outcome != AuditOutcomeStarted || records[1].Phase != "completion" || records[1].Effect != effect || records[1].Outcome != AuditOutcomeSucceeded {
			t.Fatalf("audit(%s) = %+v", effect, records)
		}
	}
}

func TestActionHostReentersRunAndToolPaths(t *testing.T) {
	definition := testDefinition("example.action.bridge", action.EffectWrite)
	definition.ResultSchema = json.RawMessage(`{"type":"object","properties":{"ok":{"type":"boolean"},"run_id":{"type":"string"},"tool":{"type":"object"}},"required":["ok","run_id","tool"],"additionalProperties":false}`)
	provider := providerFunc{definition: definition, invoke: func(ctx context.Context, host action.Host, _ json.RawMessage) (json.RawMessage, error) {
		run, err := host.StartRun(ctx, action.RunRequest{SessionID: "session-1", Text: "hello"})
		if err != nil {
			return nil, err
		}
		toolResult, err := host.InvokeTool(ctx, "example.tool", json.RawMessage(`{"value":"x"}`))
		if err != nil {
			return nil, err
		}
		output, err := json.Marshal(struct {
			OK    bool            `json:"ok"`
			RunID string          `json:"run_id"`
			Tool  json.RawMessage `json:"tool"`
		}{OK: true, RunID: run.ID, Tool: toolResult})
		return output, err
	}}
	deps := readyDeps(provider)
	deps.StartRun = func(context.Context, action.RunRequest) (action.RunResult, error) {
		return action.RunResult{ID: "run-1"}, nil
	}
	deps.InvokeTool = func(context.Context, string, json.RawMessage) (json.RawMessage, error) {
		return json.RawMessage(`{"ok":true}`), nil
	}
	host, err := New(deps)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if _, err := host.Invoke(context.Background(), readyCaller(), testModule, definition.ID, json.RawMessage(`{"value":"x"}`)); err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
}

func TestActionHostContainsProviderPanicAndDuplicateDefinitions(t *testing.T) {
	definition := testDefinition("example.action.panic", action.EffectRead)
	provider := providerFunc{definition: definition, invoke: func(context.Context, action.Host, json.RawMessage) (json.RawMessage, error) { panic("boom") }}
	host, err := New(readyDeps(provider))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if _, err := host.Invoke(context.Background(), readyCaller(), testModule, definition.ID, json.RawMessage(`{"value":"x"}`)); !errors.Is(err, ErrProviderPanic) {
		t.Fatalf("panic error = %v, want ErrProviderPanic", err)
	}
	if _, err := New(readyDeps(provider, provider)); !errors.Is(err, ErrDuplicateAction) {
		t.Fatalf("duplicate error = %v, want ErrDuplicateAction", err)
	}
	if _, err := New(readyDeps(panicDefinitionProvider{})); !errors.Is(err, ErrProviderPanic) {
		t.Fatalf("definition panic error = %v, want ErrProviderPanic", err)
	}
}

func TestActionHostDoesNotReflectUntrackedProviderError(t *testing.T) {
	const canary = "untracked-provider-secret-canary"
	definition := testDefinition("example.action.error-canary", action.EffectRead)
	providerErr := errors.New("provider failed with " + canary)
	provider := providerFunc{definition: definition, invoke: func(context.Context, action.Host, json.RawMessage) (json.RawMessage, error) {
		return nil, providerErr
	}}
	host, err := New(readyDeps(provider))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	_, err = host.Invoke(context.Background(), readyCaller(), testModule, definition.ID, json.RawMessage(`{"value":"x"}`))
	if err == nil || strings.Contains(err.Error(), canary) {
		t.Fatalf("provider error = %v, canary must not be reflected", err)
	}
	if !errors.Is(err, ErrProviderFailed) {
		t.Fatalf("provider failure was not classified: %v", err)
	}
	if errors.Is(err, providerErr) {
		t.Fatalf("provider cause was exposed through errors.Is: %v", err)
	}
}

func TestActionHostCloseIsIdempotent(t *testing.T) {
	host, err := New(Deps{GenerationAvailable: true})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if err := host.Close(); err != nil {
		t.Fatalf("first Close() error = %v", err)
	}
	if err := host.Close(); err != nil {
		t.Fatalf("second Close() error = %v", err)
	}
}

func TestActionHostConstructionRequiresTrustedBoundaries(t *testing.T) {
	provider := providerFunc{definition: testDefinition("example.action.boundaries", action.EffectRead), invoke: func(context.Context, action.Host, json.RawMessage) (json.RawMessage, error) {
		return json.RawMessage(`{"ok":true}`), nil
	}}
	tests := []struct {
		name string
		edit func(*Deps)
		want error
	}{
		{name: "authentication", edit: func(deps *Deps) { deps.Authenticate = nil }, want: ErrAuthenticationUnavailable},
		{name: "authorization", edit: func(deps *Deps) { deps.Authorize = nil }, want: ErrAuthorizationUnavailable},
		{name: "audit", edit: func(deps *Deps) { deps.Audit = nil }, want: ErrAuditUnavailable},
		{name: "generation", edit: func(deps *Deps) { deps.GenerationID = "" }, want: ErrGenerationUnavailable},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			deps := readyDeps(provider)
			test.edit(&deps)
			if _, err := New(deps); !errors.Is(err, test.want) {
				t.Fatalf("New() error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestActionHostAuditFailurePreventsProviderExecution(t *testing.T) {
	called := false
	definition := testDefinition("example.action.audit-required", action.EffectRead)
	provider := providerFunc{definition: definition, invoke: func(context.Context, action.Host, json.RawMessage) (json.RawMessage, error) {
		called = true
		return json.RawMessage(`{"ok":true}`), nil
	}}
	deps := readyDeps(provider)
	deps.Audit = AuditSinkFunc(func(context.Context, AuditRecord) error { return errors.New("audit backend secret") })
	host, err := New(deps)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	_, err = host.Invoke(context.Background(), readyCaller(), testModule, definition.ID, json.RawMessage(`{"value":"x"}`))
	if !errors.Is(err, ErrAuditUnavailable) || called {
		t.Fatalf("Invoke() error = %v called=%v, want audit failure before Provider", err, called)
	}
	if strings.Contains(err.Error(), "audit backend secret") {
		t.Fatalf("audit cause leaked: %v", err)
	}
}

func TestActionHostSecretProviderFailureCannotLeakCauseOrAudit(t *testing.T) {
	const secret = "action-provider-secret-canary"
	definition := testDefinition("example.action.secret-error", action.EffectRead)
	definition.RequiredGrants = []module.Grant{module.GrantSecretRead}
	provider := providerFunc{definition: definition, invoke: func(_ context.Context, host action.Host, _ json.RawMessage) (json.RawMessage, error) {
		value, err := host.Secret("API_TOKEN")
		if err != nil {
			return nil, err
		}
		return nil, errors.New("provider cause contains " + value + " and " + secret)
	}}
	recorder := &auditRecorder{}
	deps := readyDeps(provider)
	deps.Audit = recorder
	deps.Bindings[0].EffectiveGrants = []module.GrantBinding{{Name: module.GrantSecretRead, Constraints: map[string][]string{"names": {"API_TOKEN"}}}}
	deps.ResolveSecret = func(context.Context, string, string, string) (string, error) { return secret, nil }
	host, err := New(deps)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	_, err = host.Invoke(context.Background(), readyCaller(), testModule, definition.ID, json.RawMessage(`{"value":"x"}`))
	if !errors.Is(err, ErrProviderFailed) || errors.Is(err, errors.New("provider cause contains "+secret)) || strings.Contains(err.Error(), secret) {
		t.Fatalf("provider failure = %v, cause/secret must be hidden", err)
	}
	for _, record := range recorder.snapshot() {
		encoded, _ := json.Marshal(record)
		if strings.Contains(string(encoded), secret) {
			t.Fatalf("secret leaked in audit record: %s", encoded)
		}
	}
}

func TestActionHostRejectsSecretRunAndToolBridgeData(t *testing.T) {
	const secret = "bridge-secret-canary"
	for _, bridge := range []string{"run", "tool"} {
		t.Run(bridge, func(t *testing.T) {
			definition := testDefinition("example.action.bridge-secret-"+bridge, action.EffectWrite)
			var calls int
			provider := providerFunc{definition: definition, invoke: func(_ context.Context, host action.Host, _ json.RawMessage) (json.RawMessage, error) {
				value, err := host.Secret("API_TOKEN")
				if err != nil {
					return nil, err
				}
				if bridge == "run" {
					_, err = host.StartRun(context.Background(), action.RunRequest{SessionID: "session-1", Text: value})
				} else {
					_, err = host.InvokeTool(context.Background(), "example.tool", json.RawMessage(`{"value":"`+value+`"}`))
				}
				return nil, err
			}}
			deps := readyDeps(provider)
			deps.Bindings[0].EffectiveGrants = []module.GrantBinding{{Name: module.GrantSecretRead, Constraints: map[string][]string{"names": {"API_TOKEN"}}}}
			deps.ResolveSecret = func(context.Context, string, string, string) (string, error) { return secret, nil }
			deps.StartRun = func(context.Context, action.RunRequest) (action.RunResult, error) {
				calls++
				return action.RunResult{ID: "run"}, nil
			}
			deps.InvokeTool = func(context.Context, string, json.RawMessage) (json.RawMessage, error) {
				calls++
				return json.RawMessage(`{"ok":true}`), nil
			}
			host, err := New(deps)
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}
			_, err = host.Invoke(context.Background(), readyCaller(), testModule, definition.ID, json.RawMessage(`{"value":"x"}`))
			if err == nil || calls != 0 || strings.Contains(err.Error(), secret) {
				t.Fatalf("Invoke() error=%v bridge calls=%d, secret must be rejected before callback", err, calls)
			}
		})
	}
}

func TestActionHostRechecksInstanceBeforeBridge(t *testing.T) {
	definition := testDefinition("example.action.instance-bridge", action.EffectWrite)
	definition.InstanceID = "live"
	var resolves, callbacks int
	provider := providerFunc{definition: definition, invoke: func(_ context.Context, host action.Host, _ json.RawMessage) (json.RawMessage, error) {
		_, err := host.StartRun(context.Background(), action.RunRequest{SessionID: "session-1", Text: "hello"})
		return nil, err
	}}
	deps := readyDeps(provider)
	deps.ResolveInstance = func(context.Context, string, string) (InstanceStatus, error) {
		resolves++
		state := InstanceReady
		if resolves > 1 {
			state = InstanceUnavailable
		}
		return InstanceStatus{ModuleID: testModule, ID: "live", State: state, GenerationID: "generation-a"}, nil
	}
	deps.StartRun = func(context.Context, action.RunRequest) (action.RunResult, error) {
		callbacks++
		return action.RunResult{ID: "run"}, nil
	}
	host, err := New(deps)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	_, err = host.Invoke(context.Background(), readyCaller(), testModule, definition.ID, json.RawMessage(`{"value":"x"}`))
	if !errors.Is(err, ErrProviderFailed) || callbacks != 0 {
		t.Fatalf("Invoke() error=%v callbacks=%d, want bridge denial after instance transition", err, callbacks)
	}
}

func TestActionHostCloseWaitIsBoundedForUncooperativeProvider(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	definition := testDefinition("example.action.uncooperative", action.EffectRead)
	provider := providerFunc{definition: definition, invoke: func(context.Context, action.Host, json.RawMessage) (json.RawMessage, error) {
		close(started)
		<-release
		return json.RawMessage(`{"ok":true}`), nil
	}}
	host, err := New(readyDeps(provider))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	done := make(chan error, 1)
	go func() {
		_, invokeErr := host.Invoke(context.Background(), readyCaller(), testModule, definition.ID, json.RawMessage(`{"value":"x"}`))
		done <- invokeErr
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("provider did not start")
	}
	closeErr := host.CloseContext(context.Background())
	if !errors.Is(closeErr, ErrActionTimeout) {
		t.Fatalf("CloseContext() error = %v, want bounded timeout", closeErr)
	}
	close(release)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Invoke() did not drain after provider release")
	}
}

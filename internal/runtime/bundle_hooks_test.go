package runtime

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"agent-vivy/internal/bundlecontract"
	"agent-vivy/internal/bundles"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/sqlite"
)

type bundleHookTestGate struct{}

func (bundleHookTestGate) BeginDrain(context.Context, string) error { return nil }
func (bundleHookTestGate) WaitIdle(context.Context, string) error   { return nil }
func (bundleHookTestGate) EndDrain(string)                          {}

func newBundleHookFixture(t *testing.T, command string) (*bundles.Manager, *sqlite.Backend, bundlecontract.View, bundlecontract.ComponentRecord) {
	t.Helper()
	ctx := context.Background()
	root := t.TempDir()
	backend, err := sqlite.Open(ctx, filepath.Join(root, "hook.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = backend.Close() })
	if owned, err := backend.Acquire(ctx, "vivy/organism", "hook-test-owner", time.Minute); err != nil || !owned {
		t.Fatalf("acquire bundle lease = %v, %v", owned, err)
	}
	source := t.TempDir()
	if err := os.MkdirAll(filepath.Join(source, "resources"), 0700); err != nil {
		t.Fatal(err)
	}
	manifest := bundlecontract.Manifest{
		APIVersion: bundlecontract.APIVersion, ID: "example/guards", Version: "1", Name: "Guard Hooks",
		Resources: []string{"resources/policy.txt"},
		Components: bundlecontract.Components{
			Hooks: []bundlecontract.HookEntry{{
				ID: "guard", Event: "PreToolUse", Command: command, Matcher: "bash", Cwd: "bundle", TimeoutMs: 1000,
				EnvFrom: map[string]string{"HOOK_ALLOWED": "VIVY_TOOLKIT_HOOK_TEST_BINDING"},
			}},
			MCP: []bundlecontract.MCPEntry{{ID: "remote", Endpoint: "https://mcp.example.test/mcp", Args: []string{}, ResourceBridge: false}},
		},
	}
	manifestBytes, err := bundlecontract.CanonicalManifest(manifest)
	if err != nil {
		t.Fatalf("canonical hook/MCP fixture: %v; component=%+v", err, manifest.Components.MCP)
	}
	if err := os.WriteFile(filepath.Join(source, bundlecontract.ManifestFile), manifestBytes, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "resources", "policy.txt"), []byte("captured resource"), 0600); err != nil {
		t.Fatal(err)
	}
	limits := bundlecontract.DefaultLimits()
	manager, err := bundles.NewManager(bundles.Config{
		StateRoot: root, Limits: limits, LeaseOwner: "hook-test-owner",
		Capabilities: bundles.Capabilities{Owners: map[bundlecontract.ComponentKind]bool{bundlecontract.Hook: true, bundlecontract.MCP: true}, Limits: limits},
		Gate:         bundleHookTestGate{},
	}, backend.BundleStore())
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Recover(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = manager.Close(context.Background()) })
	preview, err := manager.Preview(ctx, bundles.PreviewRequest{OperationID: "81000000-0000-4000-8000-000000000001", Source: bundlecontract.SourceSpec{Kind: "local", Location: source}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Install(ctx, bundles.InstallRequest{OperationID: "81000000-0000-4000-8000-000000000002", PreviewOperationID: preview.OperationID, ExpectedReleaseDigest: preview.ReleaseDigest}); err != nil {
		t.Fatal(err)
	}
	prepared, err := manager.Prepare(ctx, bundles.PrepareRequest{OperationID: "81000000-0000-4000-8000-000000000003", Mutation: bundles.CatalogMutation{Kind: "enable", InstallID: preview.InstallID}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Publish(ctx, prepared.OperationID); err != nil {
		t.Fatal(err)
	}
	handle, err := manager.PinCurrent()
	if err != nil {
		t.Fatal(err)
	}
	view := handle.View()
	handle.Release()
	components := view.Components()
	if len(components) != 2 || components[0].Hook == nil || components[0].Available || components[0].Status != "unarmed" {
		t.Fatalf("installed Hook did not remain visibly inert: %#v", components)
	}
	return manager, backend, view, components[0]
}

func TestBundleHookConsentCapturesExactReleaseAndRevocation(t *testing.T) {
	ctx := context.Background()
	manager, backend, view, hook := newBundleHookFixture(t, "test \"$HOOK_ALLOWED\" = allowed")
	unarmedHandle, unarmedSnapshot, unarmedCheck, err := manager.CaptureRun(ctx, "run-hook-unarmed")
	if err != nil {
		t.Fatal(err)
	}
	unarmedHandle.Release()
	unarmedPayload, err := storage.ValidateRunBundleSnapshot(unarmedSnapshot)
	if err != nil {
		t.Fatal(err)
	}
	if len(unarmedCheck.Executables) != 0 || len(unarmedCheck.Consents) != 2 || unarmedCheck.Consents[0].Kind != bundlecontract.Hook ||
		unarmedCheck.Consents[0].Present || unarmedCheck.Consents[1].Kind != bundlecontract.MCP || unarmedCheck.Consents[1].Present {
		t.Fatalf("unarmed Hook entered executable run state: %+v", unarmedCheck)
	}
	if unarmedPayload.Components[0].ConsentPresent {
		t.Fatalf("unarmed Hook unexpectedly has durable consent evidence: %+v", unarmedPayload.Components[0])
	}

	arm, err := manager.SetHookConsent(ctx, bundles.HookConsentRequest{
		OperationID: "81000000-0000-4000-8000-000000000004", Expected: view.Identity(), Ref: hook.Ref,
		ExpectedRevision: 0, Subject: "test-operator", Allowed: true,
	})
	if err != nil || arm.State != "committed" {
		t.Fatalf("arm exact Hook declaration = %+v, %v", arm, err)
	}
	consent, found, err := backend.ReadBundleConsent(ctx, hook.Ref.InstallID, hook.Ref.Kind, hook.Ref.LocalID)
	if err != nil || !found || !consent.Allowed || consent.Revision != 1 {
		t.Fatalf("stored Hook consent = %+v found=%v err=%v", consent, found, err)
	}
	expectedConsentDigest, err := bundlecontract.ExecutableConsentDigest(hook.Ref.InstallID, hook.Release.ReleaseDigest, hook.ExecutableDigest, view.Identity().ConfigRevision)
	if err != nil || consent.DeclarationDigest != expectedConsentDigest {
		t.Fatalf("consent digest = %s, want exact release/config digest %s (%v)", consent.DeclarationDigest, expectedConsentDigest, err)
	}

	armedHandle, armedSnapshot, armedCheck, err := manager.CaptureRun(ctx, "run-hook-armed")
	if err != nil {
		t.Fatal(err)
	}
	armedHandle.Release()
	armedPayload, err := storage.ValidateRunBundleSnapshot(armedSnapshot)
	if err != nil {
		t.Fatal(err)
	}
	if len(armedCheck.Executables) != 1 || armedCheck.Executables[0].ConsentDigest != expectedConsentDigest || armedCheck.Executables[0].ConsentRevision != 1 ||
		!armedPayload.Components[0].ConsentAllowed || armedPayload.Components[0].ConsentRevision != 1 {
		t.Fatalf("armed Hook capture lost exact consent evidence: %+v %+v", armedCheck, armedPayload.Components)
	}

	disarm, err := manager.SetHookConsent(ctx, bundles.HookConsentRequest{
		OperationID: "81000000-0000-4000-8000-000000000005", Expected: view.Identity(), Ref: hook.Ref,
		ExpectedRevision: 1, Subject: "test-operator", Allowed: false,
	})
	if err != nil || disarm.State != "committed" {
		t.Fatalf("disarm Hook = %+v, %v", disarm, err)
	}
	if err := backend.CheckBundleRevocations(ctx, armedCheck); !errors.Is(err, storage.ErrBundleRevoked) {
		t.Fatalf("captured Hook after disarm = %v, want revoked fence", err)
	}
	newHandle, _, newCheck, err := manager.CaptureRun(ctx, "run-hook-disarmed")
	if err != nil {
		t.Fatal(err)
	}
	newHandle.Release()
	if len(newCheck.Executables) != 0 || len(newCheck.Consents) != 2 || !newCheck.Consents[0].Present || newCheck.Consents[0].Allowed ||
		newCheck.Consents[1].Kind != bundlecontract.MCP || newCheck.Consents[1].Present {
		t.Fatalf("new run failed to capture disarmed state: %+v", newCheck)
	}
	if _, err := manager.SetHookConsent(ctx, bundles.HookConsentRequest{
		OperationID: "81000000-0000-4000-8000-000000000006", Expected: view.Identity(), Ref: hook.Ref,
		ExpectedRevision: 1, Subject: "test-operator", Allowed: true,
	}); err == nil {
		t.Fatal("stale consent revision was accepted")
	}
}

func TestBundleMCPConsentCapturesInstalledReleaseAndRevocation(t *testing.T) {
	ctx := context.Background()
	manager, backend, view, _ := newBundleHookFixture(t, "test true")
	var record bundlecontract.ComponentRecord
	for _, component := range view.Components() {
		if component.Ref.Kind == bundlecontract.MCP {
			record = component
			break
		}
	}
	if record.MCP == nil {
		t.Fatal("fixture did not publish its native MCP declaration")
	}
	arm, err := manager.SetExecutableConsent(ctx, bundles.ExecutableConsentRequest{
		OperationID: "81000000-0000-4000-8000-000000000009", Expected: view.Identity(), Ref: record.Ref,
		ExpectedRevision: 0, Subject: "test-operator", Allowed: true,
	})
	if err != nil || arm.State != "committed" {
		t.Fatalf("arm exact MCP declaration = %+v, %v", arm, err)
	}
	handle, _, captured, err := manager.CaptureRun(ctx, "run-mcp-armed")
	if err != nil {
		t.Fatal(err)
	}
	handle.Release()
	if len(captured.Executables) != 1 || captured.Executables[0].Kind != bundlecontract.MCP || captured.Executables[0].ConsentRevision != 1 {
		t.Fatalf("MCP capture omitted exact consent: %+v", captured)
	}
	if err := backend.CheckBundleRevocations(ctx, captured); err != nil {
		t.Fatalf("current armed MCP failed live fence: %v", err)
	}
	disarm, err := manager.SetExecutableConsent(ctx, bundles.ExecutableConsentRequest{
		OperationID: "81000000-0000-4000-8000-00000000000a", Expected: view.Identity(), Ref: record.Ref,
		ExpectedRevision: 1, Subject: "test-operator", Allowed: false,
	})
	if err != nil || disarm.State != "committed" {
		t.Fatalf("disarm exact MCP declaration = %+v, %v", disarm, err)
	}
	if err := backend.CheckBundleRevocations(ctx, captured); !errors.Is(err, storage.ErrBundleRevoked) {
		t.Fatalf("captured MCP after disarm = %v, want revoked fence", err)
	}
}

func TestBundleHookDispatcherRequiresCapturedArmingAndCurrentConsent(t *testing.T) {
	ctx := context.Background()
	t.Setenv("VIVY_TOOLKIT_HOOK_TEST_BINDING", "allowed")
	t.Setenv("VIVY_HOOK_PARENT_SECRET", "must-not-be-inherited")
	manager, backend, view, hook := newBundleHookFixture(t, "test \"$HOOK_ALLOWED\" = allowed && test -z \"${VIVY_HOOK_PARENT_SECRET+x}\" && test -d .")
	resolver := BundleHookExecutionResolverFunc(manager.ResolveHookExecution)
	dispatcher := NewBundleHookDispatcher(backend.BundleStore(), resolver, time.Second)
	workspace := t.TempDir()
	unarmedHandle, _, unarmedCheck, err := manager.CaptureRun(ctx, "run-hook-dispatch-unarmed")
	if err != nil {
		t.Fatal(err)
	}
	unarmedCtx := withToolkitWorkspaceRoot(withBundleCapture(withBundleView(ctx, unarmedHandle.View()), unarmedCheck), workspace, true, nil)
	if dispatcher.MatchesToolContext(unarmedCtx, "bash") {
		t.Fatal("unarmed Hook matched for execution")
	}
	unarmedHandle.Release()
	if _, err := dispatcher.PreToolUse(unarmedCtx, ToolHookCall{ToolName: "bash", Arguments: []byte(`{"command":"true"}`)}); err != nil {
		t.Fatalf("unarmed Hook should remain inert: %v", err)
	}

	arm, err := manager.SetHookConsent(ctx, bundles.HookConsentRequest{
		OperationID: "81000000-0000-4000-8000-000000000007", Expected: view.Identity(), Ref: hook.Ref,
		ExpectedRevision: 0, Subject: "test-operator", Allowed: true,
	})
	if err != nil || arm.State != "committed" {
		t.Fatalf("arm Hook = %+v, %v", arm, err)
	}
	armedHandle, _, armedCheck, err := manager.CaptureRun(ctx, "run-hook-dispatch-armed")
	if err != nil {
		t.Fatal(err)
	}
	armedCtx := withToolkitWorkspaceRoot(withBundleCapture(withBundleView(ctx, armedHandle.View()), armedCheck), workspace, true, nil)
	if !dispatcher.MatchesToolContext(armedCtx, "bash") {
		t.Fatal("armed Hook did not match its declared tool")
	}
	identity := dispatcher.GovernanceIdentityFor(armedCtx)
	if strings.Contains(identity, "must-not-be-inherited") || identity == "bundle-hooks:v1:none" {
		t.Fatalf("governance identity leaks values or misses the captured Hook: %q", identity)
	}
	if result, err := dispatcher.PreToolUse(armedCtx, ToolHookCall{ToolName: "bash", Arguments: []byte(`{"command":"true"}`)}); err != nil || result.Decision != hookAllow {
		t.Fatalf("armed Hook execution = %+v, %v", result, err)
	}
	armedHandle.Release()

	if _, err := manager.SetHookConsent(ctx, bundles.HookConsentRequest{
		OperationID: "81000000-0000-4000-8000-000000000008", Expected: view.Identity(), Ref: hook.Ref,
		ExpectedRevision: 1, Subject: "test-operator", Allowed: false,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := dispatcher.PreToolUse(armedCtx, ToolHookCall{ToolName: "bash", Arguments: []byte(`{"command":"true"}`)}); !errors.Is(err, ErrHookBlocked) {
		t.Fatalf("revoked captured Hook execution = %v, want blocked", err)
	}
}

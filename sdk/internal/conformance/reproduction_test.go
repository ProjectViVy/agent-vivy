package conformance_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"agent-vivy/internal/moduleport"
	providerconformance "agent-vivy/sdk/conformance"
	assemblyv1 "agent-vivy/sdk/internal/assembly"
	"agent-vivy/sdk/module"
	"agent-vivy/sdk/port"
)

type releaseSuiteCase struct {
	Port, ProviderID, SourceRoot, SourceSHA256, EvidenceID string
	ProviderTests                                          []releaseTestCommand
}

type releaseTestCommand struct {
	Directory, Package, Run string
	Executable              string
	Arguments               []string
}

// TestCheckedInProviderConformanceMatchesExecutedSuites is the producer gate
// for conformance_results.json. The checked-in artifact is expected data only:
// this independent table recomputes every source digest, executes the owning
// Provider and Host tests, runs the common compiler/lifecycle checks through
// RunProviderSuite, and byte-compares the canonical results.
func TestCheckedInProviderConformanceMatchesExecutedSuites(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	expected := assemblyv1.SupportedPortConformance()
	// The "internal" suites all share one canonical content identity. Derive it
	// from the checked-in artifact and use that value for circular-content
	// normalization. internal/sourcehash hashes every file under internal/
	// (excluding generated/assembly/zz_default.go); seeding the normalizer from
	// the artifact keeps any embedded digest fields stable while the digest is
	// still verified against the live tree.
	var internalDigest string
	for _, result := range expected {
		if result.ProviderID == "vivy/protected-tools" {
			internalDigest = result.SourceSHA256
			break
		}
	}
	if internalDigest == "" {
		t.Fatal("checked-in conformance results do not contain an internal source digest")
	}
	computedInternalDigest, err := assemblyv1.HashSourceTree(filepath.Join(repoRoot, "internal"), internalDigest)
	if err != nil {
		t.Fatalf("hash internal source tree: %v", err)
	}
	if computedInternalDigest != internalDigest {
		t.Fatalf("checked-in internal source digest = %s, want %s", computedInternalDigest, internalDigest)
	}
	actual := make([]providerconformance.ConformanceResult, 0)
	for _, suite := range releaseSuiteCases(internalDigest) {
		suite := suite
		t.Run(suite.Port+"/"+suite.ProviderID, func(t *testing.T) {
			definition, ok := port.PublicCatalog().Lookup(module.PortRef{Port: suite.Port})
			if !ok {
				t.Fatalf("release suite names unknown Port %s", suite.Port)
			}
			checks := releaseChecks(t, repoRoot, suite, definition)
			results, runErr := providerconformance.RunProviderSuite(context.Background(), definition, suite.ProviderID, suite.SourceSHA256, suite.EvidenceID, checks)
			if runErr != nil {
				t.Fatal(runErr)
			}
			for _, result := range results {
				if !result.Passed {
					t.Errorf("%s failed: %v; evidence artifact must not record it as passing", result.Suite, checks[result.Suite](context.Background()))
				}
			}
			actual = append(actual, results...)
		})
	}
	actual, err = providerconformance.CanonicalResults(actual)
	if err != nil {
		t.Fatal(err)
	}
	actualJSON, err := json.Marshal(actual)
	if err != nil {
		t.Fatal(err)
	}
	expectedJSON, err := json.Marshal(expected)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(actualJSON, expectedJSON) {
		t.Fatalf("checked-in conformance results differ from executed suites\nactual: %s\nexpected: %s", actualJSON, expectedJSON)
	}
}

func releaseChecks(t *testing.T, repoRoot string, suite releaseSuiteCase, definition port.Definition) map[string]providerconformance.Check {
	t.Helper()
	return map[string]providerconformance.Check{
		providerconformance.CheckRegistration: func(context.Context) error {
			return checkRegistration(repoRoot, suite)
		},
		providerconformance.CheckMissingProvider: func(context.Context) error {
			return checkMissingProvider(definition)
		},
		providerconformance.CheckDuplicateProvider: func(context.Context) error {
			return checkDuplicateProvider(definition)
		},
		providerconformance.CheckVersion: func(context.Context) error {
			return checkVersion(definition)
		},
		providerconformance.CheckCycle: func(context.Context) error {
			return checkCycle(definition)
		},
		providerconformance.CheckGrant: func(context.Context) error {
			return checkGrant(definition)
		},
		providerconformance.CheckTimeout: func(context.Context) error {
			return checkTimeout(definition)
		},
		providerconformance.CheckCancellation: func(context.Context) error {
			return checkCancellation(definition)
		},
		providerconformance.CheckStartup: func(context.Context) error {
			return checkStartup(repoRoot, suite)
		},
		providerconformance.CheckUnavailable: func(context.Context) error {
			return checkUnavailable(repoRoot, suite)
		},
		providerconformance.CheckCleanup: func(context.Context) error {
			return checkCleanup(repoRoot, suite)
		},
		providerconformance.CheckRedaction: func(context.Context) error {
			return checkRedaction(definition)
		},
		providerconformance.CheckProvenance: func(context.Context) error {
			return checkProvenance(suite, definition)
		},
		providerconformance.CheckDefault: func(context.Context) error {
			return checkDefault(repoRoot, suite)
		},
		providerconformance.CheckRealFailure: func(context.Context) error {
			return checkRealFailure(repoRoot, suite, definition)
		},
	}
}

func checkRegistration(repoRoot string, suite releaseSuiteCase) error {
	digest, err := assemblyv1.HashSourceTree(filepath.Join(repoRoot, filepath.FromSlash(suite.SourceRoot)), suite.SourceSHA256)
	if err != nil {
		return err
	}
	if digest != suite.SourceSHA256 {
		return fmt.Errorf("source digest = %s, want %s", digest, suite.SourceSHA256)
	}
	return runReleaseCommands(repoRoot, suite.ProviderTests)
}

func checkMissingProvider(definition port.Definition) error {
	host := releaseHostDescriptor(definition)
	host.Requires = []module.Requirement{{PortRef: definition.Ref, Provider: "fixture/missing"}}
	_, err := releaseCompiler(definition, []module.Descriptor{host}).Compile(context.Background(), assemblyv1.Recipe{APIVersion: assemblyv1.RecipeAPIVersionV1, Modules: []string{host.Module.ID}})
	return requireDiagnostic(err, "missing provider")
}

func checkDuplicateProvider(definition port.Definition) error {
	provider, host := releaseProviderAndHost(definition)
	other := provider
	other.Module.ID = "fixture/other"
	other.Source.Ref = "test:fixture/other"
	descriptors := []module.Descriptor{provider, other, host}
	recipe := releaseRecipe(definition, descriptors)
	_, err := releaseCompiler(definition, descriptors).Compile(context.Background(), recipe)
	return requireDiagnostic(err, "duplicate provider")
}

func checkVersion(definition port.Definition) error {
	provider, host := releaseProviderAndHost(definition)
	provider.Provides[0].Port = strings.TrimSuffix(definition.Ref.Port, "@v1") + "@v2"
	descriptors := []module.Descriptor{provider, host}
	_, err := releaseCompiler(definition, descriptors).Compile(context.Background(), releaseRecipe(definition, descriptors))
	return requireDiagnostic(err, "unknown Port")
}

func checkCycle(definition port.Definition) error {
	provider, host := releaseProviderAndHost(definition)
	provider.Lifecycle.After = []string{host.Module.ID}
	host.Lifecycle.After = []string{provider.Module.ID}
	descriptors := []module.Descriptor{provider, host}
	_, err := releaseCompiler(definition, descriptors).Compile(context.Background(), releaseRecipe(definition, descriptors))
	return requireDiagnostic(err, "dependency cycle")
}

func checkGrant(definition port.Definition) error {
	provider, host := releaseProviderAndHost(definition)
	grant := module.GrantRPCClient
	if len(definition.AllowedGrants) > 0 {
		grant = definition.AllowedGrants[0]
	}
	provider.RequestedGrants = []module.Grant{grant}
	descriptors := []module.Descriptor{provider, host}
	_, err := releaseCompiler(definition, descriptors).Compile(context.Background(), releaseRecipe(definition, descriptors))
	return requireDiagnostic(err, "grant")
}

func checkTimeout(definition port.Definition) error {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	_, err := releaseCompiler(definition, nil).Compile(ctx, assemblyv1.Recipe{APIVersion: assemblyv1.RecipeAPIVersionV1})
	if !errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("compiler timeout = %v", err)
	}
	return nil
}

func checkCancellation(definition port.Definition) error {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := releaseCompiler(definition, nil).Compile(ctx, assemblyv1.Recipe{APIVersion: assemblyv1.RecipeAPIVersionV1})
	if !errors.Is(err, context.Canceled) {
		return fmt.Errorf("compiler cancellation = %v", err)
	}
	return nil
}

func checkStartup(repoRoot string, suite releaseSuiteCase) error {
	if err := runReleaseCommands(repoRoot, suite.ProviderTests); err != nil {
		return err
	}
	return runReleaseCommands(repoRoot, []releaseTestCommand{{Package: "./sdk/internal/assembly", Run: "^TestGeneratedBinderCompilesAndRollsBackLifecycle$"}})
}

func checkUnavailable(repoRoot string, suite releaseSuiteCase) error {
	commands := append([]releaseTestCommand(nil), suite.ProviderTests...)
	commands = append(commands, releaseHostCommand(suite.Port))
	return runReleaseCommands(repoRoot, commands)
}

func checkCleanup(repoRoot string, suite releaseSuiteCase) error {
	if err := runReleaseCommands(repoRoot, suite.ProviderTests); err != nil {
		return err
	}
	return runReleaseCommands(repoRoot, []releaseTestCommand{{Package: "./sdk/internal/assembly", Run: "^TestGeneratedBinderCompilesAndRollsBackLifecycle$"}})
}

func checkRedaction(definition port.Definition) error {
	provider, host := releaseProviderAndHost(definition)
	grant := module.GrantRPCClient
	if len(definition.AllowedGrants) > 0 {
		grant = definition.AllowedGrants[0]
	}
	provider.RequestedGrants = []module.Grant{grant}
	provider.Conflicts = []module.Conflict{{Module: host.Module.ID}}
	descriptors := []module.Descriptor{provider, host}
	recipe := releaseRecipe(definition, descriptors)
	const secret = "p9-conformance-secret-canary"
	recipe.GrantApprovals = []assemblyv1.GrantApproval{{Module: provider.Module.ID, Name: grant, Constraints: map[string][]string{"token": {secret}}}}
	_, err := releaseCompiler(definition, descriptors).Compile(context.Background(), recipe)
	if err == nil {
		return errors.New("redaction fixture unexpectedly compiled")
	}
	if strings.Contains(err.Error(), secret) {
		return errors.New("compiler diagnostic exposed Grant secret")
	}
	return nil
}

func checkProvenance(suite releaseSuiteCase, definition port.Definition) error {
	descriptor := releaseDescriptor(suite.ProviderID)
	descriptor.Source = module.Source{Ref: "release:" + suite.ProviderID, SHA256: suite.SourceSHA256}
	descriptor.Provides = []module.PortRef{{Port: definition.Ref.Port, ID: "release.provider"}}
	plan := assemblyv1.AssemblyPlan{Modules: []assemblyv1.ResolvedModule{{Descriptor: descriptor, Trust: assemblyv1.TrustT1}}, LifecycleOrder: []string{descriptor.Module.ID}}
	manifest, raw, err := assemblyv1.SealManifest(plan, assemblyv1.SealInputs{SpecificationVersion: "p9", CompilerVersion: "p9", SDKVersion: "p9", CanonicalRecipe: []byte("{}")})
	if err != nil {
		return err
	}
	inspected, err := assemblyv1.InspectManifest(raw)
	if err != nil {
		return err
	}
	if len(manifest.Modules) != 1 || len(inspected.Modules) != 1 || inspected.Modules[0].Source.SHA256 != suite.SourceSHA256 || inspected.GenerationID != manifest.GenerationID {
		return errors.New("sealed Manifest lost exact Provider source provenance")
	}
	return nil
}

func checkDefault(repoRoot string, suite releaseSuiteCase) error {
	commands := append([]releaseTestCommand(nil), suite.ProviderTests...)
	commands = append(commands, releaseHostCommand(suite.Port))
	return runReleaseCommands(repoRoot, commands)
}

func checkRealFailure(repoRoot string, suite releaseSuiteCase, definition port.Definition) error {
	descriptor := releaseDescriptor(suite.ProviderID)
	descriptor.Source = module.Source{Ref: "release:" + suite.ProviderID, SHA256: strings.Repeat("0", 64)}
	descriptor.Provides = []module.PortRef{{Port: definition.Ref.Port, ID: "release.provider"}}
	record := assemblyv1.SourceRecord{Descriptor: descriptor, Trust: assemblyv1.TrustT1, Root: filepath.Join(repoRoot, filepath.FromSlash(suite.SourceRoot)), Ref: descriptor.Source.Ref}
	catalog, err := assemblyv1.NewSourceCatalog([]assemblyv1.SourceRecord{record})
	if err != nil {
		return err
	}
	plan := assemblyv1.AssemblyPlan{Modules: []assemblyv1.ResolvedModule{{Descriptor: descriptor}}}
	err = assemblyv1.VerifyBoundSourceHashes(plan, catalog)
	return requireDiagnostic(err, "source changed during pack")
}

func releaseCompiler(definition port.Definition, descriptors []module.Descriptor) assemblyv1.Compiler {
	records := make([]assemblyv1.SourceRecord, 0, len(descriptors))
	for _, descriptor := range descriptors {
		records = append(records, assemblyv1.SourceRecord{Descriptor: descriptor, Trust: assemblyv1.TrustT1})
	}
	catalog, err := assemblyv1.NewSourceCatalog(records)
	if err != nil {
		panic(err)
	}
	results := make([]providerconformance.ConformanceResult, 0, len(providerconformance.RequiredProviderChecks()))
	for _, check := range providerconformance.RequiredProviderChecks() {
		results = append(results, providerconformance.ConformanceResult{Port: definition.Ref, ProviderID: "fixture/support", SourceSHA256: strings.Repeat("a", 64), Suite: check, Passed: true, EvidenceID: "sdk/internal/conformance/reproduction_test.go#releaseCompiler"})
	}
	return assemblyv1.Compiler{Ports: port.PublicCatalog(), Sources: catalog, PortEvidence: assemblyv1.SupportedPortEvidence(), ConformanceResults: results}
}

func releaseProviderAndHost(definition port.Definition) (module.Descriptor, module.Descriptor) {
	provider := releaseDescriptor("fixture/provider")
	provider.Provides = []module.PortRef{{Port: definition.Ref.Port, ID: "fixture.provider"}}
	host := releaseHostDescriptor(definition)
	provider.Requires = []module.Requirement{{PortRef: definition.Consumer, Provider: host.Module.ID}}
	if definition.Ref.Port == assemblyv1.UIExtensionPort || definition.Ref.Port == assemblyv1.UIRootPort {
		provider.I18N = &module.I18N{Catalog: "i18n/catalog.json", DefaultLocale: "en", Locales: []string{"en"}}
	}
	return provider, host
}

func releaseHostDescriptor(definition port.Definition) module.Descriptor {
	hostDefinition, ok := moduleport.Catalog().Lookup(definition.Consumer)
	if !ok {
		legacyOwners := map[string]string{"core/channel-host@v1": "vivy/channel-host", "core/face-host@v1": "vivy/face-host"}
		owner := legacyOwners[definition.Consumer.Port]
		if owner == "" {
			panic("missing closed Host definition for " + definition.Ref.Port)
		}
		hostDefinition.Owner = owner
	}
	host := releaseDescriptor(hostDefinition.Owner)
	host.Provides = []module.PortRef{{Port: definition.Consumer.Port, ID: strings.ReplaceAll(hostDefinition.Owner, "/", ".")}}
	return host
}

func releaseDescriptor(id string) module.Descriptor {
	return module.Descriptor{APIVersion: module.APIVersionV1, Module: module.Identity{ID: id, Version: "1.0.0"}, Source: module.Source{Ref: "test:" + id, SHA256: strings.Repeat("a", 64)}, Lifecycle: module.Lifecycle{Scope: module.ScopeGeneration}}
}

func releaseRecipe(definition port.Definition, descriptors []module.Descriptor) assemblyv1.Recipe {
	recipe := assemblyv1.Recipe{APIVersion: assemblyv1.RecipeAPIVersionV1}
	for _, descriptor := range descriptors {
		recipe.Modules = append(recipe.Modules, descriptor.Module.ID)
	}
	if definition.Cardinality == port.CardinalityExclusive {
		recipe.Exclusive = map[string]string{definition.Ref.Port: "fixture/provider"}
	}
	if definition.Ordered {
		recipe.Order = map[string][]string{definition.Ref.Port: {"fixture/provider"}}
	}
	return recipe
}

func requireDiagnostic(err error, fragment string) error {
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(fragment)) {
		return fmt.Errorf("diagnostic = %v, want %q", err, fragment)
	}
	return nil
}

var releaseCommandCache = struct {
	sync.Mutex
	results map[string]error
}{results: make(map[string]error)}

func runReleaseCommands(repoRoot string, commands []releaseTestCommand) error {
	for _, command := range commands {
		key := command.Directory + "\x00" + command.Package + "\x00" + command.Run + "\x00" + command.Executable + "\x00" + strings.Join(command.Arguments, "\x00")
		releaseCommandCache.Lock()
		cached, exists := releaseCommandCache.results[key]
		releaseCommandCache.Unlock()
		if exists {
			if cached != nil {
				return cached
			}
			continue
		}
		executable := filepath.Join(runtime.GOROOT(), "bin", "go")
		args := []string{"test", command.Package, "-count=1"}
		if command.Executable != "" {
			executable = command.Executable
			args = append([]string(nil), command.Arguments...)
		} else if command.Run != "" {
			args = append(args, "-run", command.Run)
		}
		process := exec.Command(executable, args...)
		process.Dir = repoRoot
		if command.Directory != "" {
			process.Dir = filepath.Join(repoRoot, filepath.FromSlash(command.Directory))
		}
		output, err := process.CombinedOutput()
		if err != nil {
			err = fmt.Errorf("%s: %w: %s", strings.Join(args, " "), err, output)
		}
		releaseCommandCache.Lock()
		releaseCommandCache.results[key] = err
		releaseCommandCache.Unlock()
		if err != nil {
			return err
		}
	}
	return nil
}

func releaseHostCommand(portID string) releaseTestCommand {
	switch portID {
	case "std/tool@v1", "std/tool-world@v1", "std/channel@v1", "std/face@v1":
		return releaseTestCommand{Package: "./sdk/internal/assembly", Run: "^TestP1P2PortConformanceSuite/" + portID + "$"}
	case "std/provider-profile@v1":
		return releaseTestCommand{Package: "./internal/modelhost"}
	case "std/context-source@v1":
		return releaseTestCommand{Package: "./internal/contexthost"}
	case "std/skill-source@v1":
		return releaseTestCommand{Package: "./internal/skillhost"}
	case "std/middleware/pre-tool@v1":
		return releaseTestCommand{Package: "./internal/toolhost", Run: "^TestMiddleware"}
	case "std/observer/run@v1", "std/observer/diagnostic@v1":
		return releaseTestCommand{Package: "./internal/observerhost"}
	case "std/status-source@v1":
		return releaseTestCommand{Package: "./internal/statushost"}
	case "std/ui-extension@v1", "std/ui-root@v1":
		return releaseTestCommand{Package: "./sdk/internal", Run: "^TestPackAndInspectSealUIAssemblyIdentity$"}
	case "std/control-action@v1":
		return releaseTestCommand{Package: "./internal/rpc", Run: "^TestModuleActionRPC"}
	default:
		panic("missing release Host command for " + portID)
	}
}

// releaseSuiteCases builds the independent expected table. internalDigest is
// the canonical identity of the "internal" source root, computed by the caller
// from the live tree (see the test above) so the table cannot drift from it.
func releaseSuiteCases(internalDigest string) []releaseSuiteCase {
	goTest := func(pkg, run string) []releaseTestCommand { return []releaseTestCommand{{Package: pkg, Run: run}} }
	nested := func(dir string) []releaseTestCommand { return []releaseTestCommand{{Directory: dir, Package: "./..."}} }
	uiConformance := releaseTestCommand{Directory: "ui", Executable: "pnpm", Arguments: []string{"exec", "vitest", "run", "src/plugins/conformance.test.tsx"}}
	cases := []releaseSuiteCase{
		{"std/tool@v1", "vivy/protected-tools", "internal", internalDigest, "internal/app/assembly_governance_e2e_test.go#TestToolEnvelopeConformanceAcrossAllSourceClasses", goTest("./internal/app", "^TestToolEnvelopeConformanceAcrossAllSourceClasses$")},
		{"std/tool-world@v1", "vivy/mcp-host", "internal", internalDigest, "internal/mcphost/conformance_test.go#TestMCPToolBridgeEntersSoleToolHost", goTest("./internal/mcphost", "^TestMCPToolBridgeEntersSoleToolHost$")},
		{"std/tool-world@v1", "vivy/hello-fs", "plugins/hello-fs", "40439b91831bda45ff7fe7fab1ccfbb5a89c8a613d37bedb6878e35e8fdab41e", "plugins/hello-fs/plugin_test.go#TestHelloStatReadsThroughEnv", goTest("./plugins/hello-fs", "^TestHelloStatReadsThroughEnv$")},
		{"std/tool-world@v1", "vivy/lsp", "plugins/lsp", "217790956559c42a0efd318df712c01d1a7f88032db291c5af33fd2e213b320b", "plugins/lsp/plugin_test.go#TestDiagnosticsToolEndToEnd", nested("plugins/lsp")},
		{"std/channel@v1", "vivy/dingtalk", "plugins/dingtalk", "9def70cefa5bda9e3c5a1e65c76165a74d730b0d71ebbdf426df5ad8982b197f", "plugins/dingtalk/plugin_test.go#TestStartStopFullLoop", nested("plugins/dingtalk")},
		{"std/channel@v1", "vivy/discord", "plugins/discord", "e360a7405bd03a2185154cd3992c4b49a998cdecffd6111ae5518fbdd890420e", "plugins/discord/plugin_test.go#TestStartSuccessWiring", nested("plugins/discord")},
		{"std/channel@v1", "vivy/feishu", "plugins/feishu", "ba6bd61c9532ab2ae8c179e7c3a4f066e676f00073a60af21b668b559d2b6ab5", "plugins/feishu/plugin_test.go#TestStartStopFullLoop", nested("plugins/feishu")},
		{"std/channel@v1", "vivy/qq", "plugins/qq", "e4f1767624d707d9e7387d4831a0d4aeed3af31f74f6a34d203070df9e068b3c", "plugins/qq/plugin_test.go#TestSendPassiveReplyLoopback", nested("plugins/qq")},
		{"std/channel@v1", "vivy/telegram", "plugins/telegram", "41df19a635440f6aedbe4e3870edef35204455a0d0d841d81042fef8a0accf1d", "plugins/telegram/plugin_test.go#TestStartStopFullLoop", nested("plugins/telegram")},
		{"std/face@v1", "vivy/headless", "faces/headless", "948c24ec5c2a923e13940c6c548d36ee4d6db2f30b3f602bc3c57e9d003c17aa", "faces/headless/headless_test.go#TestCompletedRunStreamsAndReturnsStatus", nested("faces/headless")},
		{"std/face@v1", "vivy/tui", "faces/tui", "1e9ff549d8b1ca4e6bcd76050e236d3be3c2f16a06e920dd5814ddeca397d464", "faces/tui/face_test.go#TestNewDelegatesCanonicalTUI", nested("faces/tui")},
		{"std/provider-profile@v1", "vivy/provider-profiles", "internal", internalDigest, "internal/modules/defaults/providers_test.go#TestDefaultProviderProfilesMatchExistingRuntimeFamilies", goTest("./internal/modules/defaults", "^TestDefaultProviderProfilesMatchExistingRuntimeFamilies$")},
		{"std/context-source@v1", "vivy/context-source", "internal", internalDigest, "internal/contexthost/conformance_test.go#TestContextSourceConformance", goTest("./internal/contexthost", "^TestContextSourceConformance$")},
		{"std/context-source@v1", "scx/reference-fixtures", "plugins/scx-reference", "5c39ce4d73b0e1cb47fb9a3bf6e317a32191830cea87ed554e83c3c9db1fdfe4", "internal/contexthost/scx_conformance_test.go#TestSCXExactVersionResourceResolutionIsScopedBoundedAndReplayable", append(nested("plugins/scx-reference"), releaseTestCommand{Package: "./internal/contexthost", Run: "^TestSCXExactVersionResourceResolutionIsScopedBoundedAndReplayable$"})},
		{"std/skill-source@v1", "vivy/skill-source", "internal", internalDigest, "internal/skillhost/conformance_test.go#TestSkillSourceConformance", goTest("./internal/skillhost", "^TestSkillSourceConformance$")},
		{"std/middleware/pre-tool@v1", "vivy/governance-reference", "plugins/governance", "8e0d6e287288eff1e0a81a9458cd5e04e3e97d8c39589c29b749cb741eefa5b0", "plugins/governance/provider_test.go#TestReferenceProviderConformance", nested("plugins/governance")},
		{"std/observer/run@v1", "vivy/governance-reference", "plugins/governance", "8e0d6e287288eff1e0a81a9458cd5e04e3e97d8c39589c29b749cb741eefa5b0", "plugins/governance/provider_test.go#TestReferenceProviderConformance", nested("plugins/governance")},
		{"std/observer/run@v1", "scx/reference-fixtures", "plugins/scx-reference", "5c39ce4d73b0e1cb47fb9a3bf6e317a32191830cea87ed554e83c3c9db1fdfe4", "internal/observerhost/scx_conformance_test.go#TestSCXObserverWorkerResumesPendingDeliveryAfterReconnect", append(nested("plugins/scx-reference"), releaseTestCommand{Package: "./internal/observerhost", Run: "^TestSCXObserverWorkerResumesPendingDeliveryAfterReconnect$"})},
		{"std/observer/diagnostic@v1", "vivy/governance-reference", "plugins/governance", "8e0d6e287288eff1e0a81a9458cd5e04e3e97d8c39589c29b749cb741eefa5b0", "plugins/governance/provider_test.go#TestReferenceProviderConformance", nested("plugins/governance")},
		{"std/status-source@v1", "vivy/governance-reference", "plugins/governance", "8e0d6e287288eff1e0a81a9458cd5e04e3e97d8c39589c29b749cb741eefa5b0", "plugins/governance/provider_test.go#TestReferenceProviderConformance", nested("plugins/governance")},
		{"std/ui-extension@v1", "fixture/full-ui", "sdk/internal/testdata/full-ui-module", "c106d108fd1510eba19351b3eed7d87338fe777188d75ed942db61443d72b8cc", "ui/src/plugins/conformance.test.tsx#full UI module conformance", append(goTest("./sdk/internal", "^TestPackAndInspectSealUIAssemblyIdentity$"), uiConformance)},
		{"std/ui-root@v1", "fixture/full-ui", "sdk/internal/testdata/full-ui-module", "c106d108fd1510eba19351b3eed7d87338fe777188d75ed942db61443d72b8cc", "ui/src/plugins/conformance.test.tsx#builds and presents default, extension, replacement-root, and minimal generations", append(goTest("./sdk/internal", "^TestPackAndInspectSealUIAssemblyIdentity$"), uiConformance)},
		{"std/control-action@v1", "fixture/full-ui", "sdk/internal/testdata/full-ui-module", "c106d108fd1510eba19351b3eed7d87338fe777188d75ed942db61443d72b8cc", "internal/rpc/module_action_test.go#TestModuleActionRPCUsesAuthenticatedCallerAndReturnsBoundResult", goTest("./internal/rpc", "^TestModuleActionRPCUsesAuthenticatedCallerAndReturnsBoundResult$")},
	}
	sort.Slice(cases, func(i, j int) bool {
		left := cases[i].Port + "\x00" + cases[i].ProviderID
		right := cases[j].Port + "\x00" + cases[j].ProviderID
		return left < right
	})
	return cases
}

package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"agent-vivy/internal/bundlecontract"
	"agent-vivy/internal/storage"
	toolworldport "agent-vivy/sdk/port/toolworld"
)

func toolkitMCPTestView(t *testing.T, entries ...bundlecontract.MCPEntry) bundlecontract.View {
	t.Helper()
	const installID = "81000000-0000-4000-8000-000000000001"
	release := bundlecontract.ReleaseRef{InstallID: installID, BundleID: "example/tools", ReleaseDigest: strings.Repeat("a", 64), ReleaseSequence: 1}
	components := make([]bundlecontract.ComponentRecord, 0, len(entries))
	for _, entry := range entries {
		id, err := bundlecontract.MCPInstanceID(installID, entry.ID)
		if err != nil {
			t.Fatal(err)
		}
		canonical, err := bundlecontract.CanonicalJSON(entry)
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(canonical)
		declaration := hex.EncodeToString(sum[:])
		copy := entry
		components = append(components, bundlecontract.ComponentRecord{
			Ref:         bundlecontract.ComponentRef{InstallID: installID, Kind: bundlecontract.MCP, LocalID: entry.ID},
			ProjectedID: id, Release: release, ContentDigest: release.ReleaseDigest,
			MCP: &copy, ExecutableDigest: declaration,
		})
	}
	view, err := bundlecontract.NewView(bundlecontract.CatalogIdentity{
		Epoch: 1, EffectiveDigest: strings.Repeat("b", 64), GenerationID: "test",
	}, []bundlecontract.ReleaseRef{release}, components, nil)
	if err != nil {
		t.Fatal(err)
	}
	return view
}

func TestToolkitMCPMergePreservesUserAndUsesStableBundleIDs(t *testing.T) {
	userEnabled := false
	user := []MCPServerConfig{{Name: "user_docs", Endpoint: "https://docs.example.test/mcp", Enabled: &userEnabled}}
	view := toolkitMCPTestView(t, bundlecontract.MCPEntry{ID: "docs", Endpoint: "https://bundle.example.test/mcp", Args: []string{}, ResourceBridge: false})
	bundled, err := PrepareBundleMCPConfigs(view, nil, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(bundled) != 1 || bundled[0].Name != "b_81000000000040008000000000000001_docs" {
		t.Fatalf("bundle MCP projection=%+v", bundled)
	}
	if bundled[0].Enabled == nil || *bundled[0].Enabled || bundled[0].ResourceBridge {
		t.Fatalf("unconsented MCP must be explicitly inactive with ResourceBridge=false: %+v", bundled[0])
	}
	merged, err := MergeMCPConfigs(user, bundled)
	if err != nil {
		t.Fatal(err)
	}
	if len(merged) != 2 || merged[0].Name != bundled[0].Name || merged[1].Name != "user_docs" {
		t.Fatalf("effective MCP catalog=%+v", merged)
	}
	if merged[1].Enabled == nil || *merged[1].Enabled {
		t.Fatalf("user configuration was not preserved: %+v", merged[1])
	}
}

func TestToolkitMCPConsentAndBindingsGateActivation(t *testing.T) {
	const installID = "81000000-0000-4000-8000-000000000001"
	t.Setenv("VIVY_TOOLKIT_MCP_TOKEN", "secret-value")
	entry := bundlecontract.MCPEntry{
		ID: "private", Endpoint: "https://private.example.test/mcp", Args: []string{},
		AuthEnv: "VIVY_TOOLKIT_MCP_TOKEN", ResourceBridge: false,
	}
	view := toolkitMCPTestView(t, entry)
	record, _ := view.Lookup(bundlecontract.MCP, "b_81000000000040008000000000000001_private")
	consentDigest, err := bundlecontract.ExecutableConsentDigest(installID, record.Release.ReleaseDigest, record.ExecutableDigest, view.Identity().ConfigRevision)
	if err != nil {
		t.Fatal(err)
	}
	consents := map[bundlecontract.ComponentRef]BundleMCPConsent{
		record.Ref: {Allowed: true, Digest: consentDigest, Revision: 1},
	}
	configs, err := PrepareBundleMCPConfigs(view, consents, func(name string) bool { return name == "VIVY_TOOLKIT_MCP_TOKEN" }, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(configs) != 1 || configs[0].Enabled == nil || !*configs[0].Enabled {
		t.Fatalf("authorized and bound MCP should be enabled: %+v", configs)
	}
	if configs[0].AuthEnv != "VIVY_TOOLKIT_MCP_TOKEN" {
		t.Fatalf("bundle auth binding name was not preserved: %+v", configs[0])
	}
	if strings.Contains(configs[0].ToolkitConsentDigest, "secret-value") {
		t.Fatal("MCP consent metadata contains a secret value")
	}

	missingBinding, err := PrepareBundleMCPConfigs(view, consents, func(string) bool { return false }, false)
	if err != nil {
		t.Fatal(err)
	}
	if *missingBinding[0].Enabled || missingBinding[0].DeferredReason == "" {
		t.Fatalf("missing binding did not stay explicitly inactive: %+v", missingBinding[0])
	}
	wrongConsent := map[bundlecontract.ComponentRef]BundleMCPConsent{
		record.Ref: {Allowed: true, Digest: strings.Repeat("c", 64), Revision: 1},
	}
	untrusted, err := PrepareBundleMCPConfigs(view, wrongConsent, func(string) bool { return true }, true)
	if err != nil {
		t.Fatal(err)
	}
	if *untrusted[0].Enabled || untrusted[0].DeferredReason == "" {
		t.Fatalf("stale declaration consent was accepted: %+v", untrusted[0])
	}
}

func TestToolkitMCPMergeRejectsDuplicateOrUnsafeNames(t *testing.T) {
	for name, bundled := range map[string]MCPServerConfig{
		"duplicate": {Name: "shared", Endpoint: "https://bundle.example.test/mcp", Enabled: boolPointer(true)},
		"unsafe":    {Name: "bad/name", Endpoint: "https://bundle.example.test/mcp", Enabled: boolPointer(true)},
		"transport": {Name: "bundle", Endpoint: "https://bundle.example.test/mcp", Command: "server", Enabled: boolPointer(true)},
	} {
		t.Run(name, func(t *testing.T) {
			user := []MCPServerConfig{{Name: "shared", Endpoint: "https://user.example.test/mcp"}}
			if name != "duplicate" {
				user = nil
			}
			if _, err := MergeMCPConfigs(user, []MCPServerConfig{bundled}); err == nil {
				t.Fatal("invalid MCP catalog was accepted")
			}
		})
	}
}

type bundleMCPTestWorld struct{ invokes int }

func (*bundleMCPTestWorld) Definition() toolworldport.Definition {
	return toolworldport.Definition{ID: "mcp"}
}
func (*bundleMCPTestWorld) Discover(context.Context, toolworldport.Host) ([]toolworldport.ToolDefinition, error) {
	return nil, nil
}
func (world *bundleMCPTestWorld) Invoke(context.Context, toolworldport.Host, string, json.RawMessage) (toolworldport.Result, error) {
	world.invokes++
	return toolworldport.Result{Text: "invoked"}, nil
}
func (*bundleMCPTestWorld) Close(context.Context) error { return nil }

func TestToolkitMCPInvocationRequiresExactCapturedConsentAndLiveFence(t *testing.T) {
	const installID = "81000000-0000-4000-8000-000000000001"
	entry := bundlecontract.MCPEntry{ID: "remote", Endpoint: "https://mcp.example.test/mcp", Args: []string{}, ResourceBridge: false}
	view := toolkitMCPTestView(t, entry)
	var record bundlecontract.ComponentRecord
	for _, component := range view.Components() {
		if component.Ref.Kind == bundlecontract.MCP {
			record = component
		}
	}
	consentDigest, err := bundlecontract.ExecutableConsentDigest(installID, record.Release.ReleaseDigest, record.ExecutableDigest, view.Identity().ConfigRevision)
	if err != nil {
		t.Fatal(err)
	}
	consent := BundleMCPConsent{Allowed: true, Digest: consentDigest, Revision: 4}
	configs, err := PrepareBundleMCPConfigs(view, map[bundlecontract.ComponentRef]BundleMCPConsent{record.Ref: consent}, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	config := configs[0]
	provider := &bundleMCPTestWorld{}
	backend := &MCPBackend{servers: map[string]*mcpServer{config.Name: {config: config}}}
	guarded := &bundleGuardedMCPWorld{backend: backend, delegate: provider}
	call := func(ctx context.Context) error {
		_, err := guarded.Invoke(ctx, nil, "mcp."+config.Name+".ping", json.RawMessage(`{}`))
		return err
	}

	if err := call(context.Background()); !errors.Is(err, ErrBundleMCPNotCaptured) {
		t.Fatalf("MCP call without run evidence = %v", err)
	}
	if provider.invokes != 0 {
		t.Fatalf("uncaptured call reached MCP owner %d times", provider.invokes)
	}

	check := bundlecontract.CaptureCheck{Catalog: view.Identity(), Executables: []bundlecontract.ExecutableRef{{
		ComponentRef: record.Ref, DeclarationDigest: record.ExecutableDigest,
		ConsentDigest: consent.Digest, ConsentRevision: consent.Revision,
	}}}
	ctx := withBundleFence(withBundleCapture(withBundleView(context.Background(), view), check), func(context.Context) error { return nil })
	if err := call(ctx); err != nil {
		t.Fatalf("exact captured MCP call = %v", err)
	}
	if provider.invokes != 1 {
		t.Fatalf("exactly captured MCP call reached owner %d times", provider.invokes)
	}
	ctx = withBundleFence(ctx, func(context.Context) error { return storage.ErrBundleRevoked })
	if err := call(ctx); !errors.Is(err, storage.ErrBundleRevoked) {
		t.Fatalf("revoked MCP call = %v", err)
	}
	if provider.invokes != 1 {
		t.Fatalf("revoked call reached MCP owner %d times", provider.invokes)
	}
}

func boolPointer(value bool) *bool { return &value }

package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"sort"
	"strings"

	"agent-vivy/internal/bundlecontract"
	toolworldport "agent-vivy/sdk/port/toolworld"
)

var ErrBundleMCPNotCaptured = errors.New("runtime: Toolkit MCP instance is outside the admitted run snapshot")

// BundleMCPConsent is the secret-free authorization evidence used to prepare
// an MCP configuration. Digest and revision are read from Core Storage.
type BundleMCPConsent struct {
	Allowed  bool
	Digest   string
	Revision int64
}

// MCPBindingCheck reports whether a host-managed environment binding exists;
// it never returns the bound value.
type MCPBindingCheck func(string) bool

// PrepareBundleMCPConfigs projects captured MCP declarations into inert
// MCPBackend configs. It performs no connection, process launch, or secret
// resolution. Missing authorization, bindings, or compiled Resource support
// remains visible as an explicitly inactive entry.
func PrepareBundleMCPConfigs(view bundlecontract.View, consents map[bundlecontract.ComponentRef]BundleMCPConsent, bindingExists MCPBindingCheck, resourceBridgeCompiled bool) ([]MCPServerConfig, error) {
	if bindingExists == nil {
		bindingExists = func(name string) bool {
			value, ok := os.LookupEnv(name)
			return ok && value != ""
		}
	}
	configs := make([]MCPServerConfig, 0)
	for _, record := range view.Components() {
		if record.Ref.Kind != bundlecontract.MCP {
			continue
		}
		if record.MCP == nil || record.ProjectedID == "" || record.ExecutableDigest == "" {
			return nil, errors.New("runtime: invalid captured Toolkit MCP declaration")
		}
		entry := *record.MCP
		config := MCPServerConfig{
			Name: record.ProjectedID, Endpoint: strings.TrimSpace(entry.Endpoint), Command: strings.TrimSpace(entry.Command),
			Args: append([]string(nil), entry.Args...), EnvFrom: cloneStringMap(entry.EnvFrom),
			AuthEnv: strings.TrimSpace(entry.AuthEnv), ResourceBridge: entry.ResourceBridge,
			ToolkitRef: componentRefPointer(record.Ref), ToolkitReleaseDigest: record.Release.ReleaseDigest,
			ToolkitDeclarationDigest: record.ExecutableDigest, ToolkitCatalogEpoch: view.Identity().Epoch,
		}
		enabled := false
		deferred := ""
		if !safeMCPServerName(config.Name) || (config.Endpoint == "") == (config.Command == "") {
			return nil, errors.New("runtime: invalid Toolkit MCP identity or transport")
		}
		if config.Endpoint != "" {
			parsed, err := url.Parse(config.Endpoint)
			if err != nil || parsed.Host == "" || parsed.User != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || entry.Cwd != "" || len(entry.Args) > 0 || len(entry.EnvFrom) > 0 {
				return nil, errors.New("runtime: invalid Toolkit MCP endpoint declaration")
			}
		} else {
			switch entry.Cwd {
			case "", "workspace":
				// Empty means the existing process owner resolves its configured
				// workspace root directly.
				config.Cwd = ""
			case "bundle":
				// Bundle-root process execution needs a private artifact-root
				// capability from the app owner. Do not reinterpret it as a
				// workspace-relative path.
				deferred = "toolkit_bundle_cwd_unavailable"
			default:
				return nil, errors.New("runtime: invalid Toolkit MCP cwd")
			}
			if err := validateMCPRuntimeConfig(config); err != nil {
				return nil, fmt.Errorf("runtime: invalid Toolkit MCP process declaration: %w", err)
			}
		}
		if entry.AuthEnv != "" && !bindingExists(entry.AuthEnv) {
			deferred = "toolkit_binding_unavailable"
		}
		for _, hostName := range config.EnvFrom {
			if !bindingExists(hostName) {
				deferred = "toolkit_binding_unavailable"
				break
			}
		}
		consent := consents[record.Ref]
		expectedConsent, err := bundlecontract.ExecutableConsentDigest(record.Ref.InstallID, record.Release.ReleaseDigest, record.ExecutableDigest, view.Identity().ConfigRevision)
		if err != nil {
			return nil, err
		}
		if !consent.Allowed || consent.Revision <= 0 || consent.Digest != expectedConsent {
			if deferred == "" {
				deferred = "toolkit_mcp_consent_required"
			}
		} else {
			config.ToolkitConsentDigest = consent.Digest
			config.ToolkitConsentRevision = consent.Revision
		}
		if entry.ResourceBridge && !resourceBridgeCompiled {
			deferred = "toolkit_resource_bridge_unavailable"
		}
		if deferred == "" {
			enabled = true
		}
		enabledValue := enabled
		config.Enabled = &enabledValue
		config.DeferredReason = deferred
		configs = append(configs, normalizeMCPServerConfig(config))
	}
	return configs, nil
}

func componentRefPointer(ref bundlecontract.ComponentRef) *bundlecontract.ComponentRef {
	copy := ref
	return &copy
}

func (backend *MCPBackend) toolkitMCPConfig(name string) (MCPServerConfig, bool) {
	if backend == nil {
		return MCPServerConfig{}, false
	}
	backend.mu.RLock()
	entry := backend.servers[name]
	backend.mu.RUnlock()
	if entry == nil {
		return MCPServerConfig{}, false
	}
	return normalizeMCPServerConfig(entry.config), true
}

func validateToolkitMCPInvocation(ctx context.Context, config MCPServerConfig) error {
	if config.ToolkitRef == nil {
		return nil
	}
	check, captured := bundleCapture(ctx)
	view, hasView := bundleView(ctx)
	if !captured || !hasView || check.Catalog.Epoch != config.ToolkitCatalogEpoch || view.Identity() != check.Catalog ||
		config.ToolkitRef.Kind != bundlecontract.MCP || config.ToolkitRef.InstallID == "" || config.ToolkitRef.LocalID == "" ||
		config.ToolkitConsentDigest == "" || config.ToolkitConsentRevision <= 0 {
		return ErrBundleMCPNotCaptured
	}
	if err := checkBundleFence(ctx); err != nil {
		return err
	}
	record, ok := view.Lookup(bundlecontract.MCP, config.Name)
	if !ok || record.Ref != *config.ToolkitRef || record.Release.ReleaseDigest != config.ToolkitReleaseDigest ||
		record.ExecutableDigest != config.ToolkitDeclarationDigest {
		return ErrBundleMCPNotCaptured
	}
	for _, executable := range check.Executables {
		if executable.ComponentRef == record.Ref && executable.DeclarationDigest == config.ToolkitDeclarationDigest &&
			executable.ConsentDigest == config.ToolkitConsentDigest && executable.ConsentRevision == config.ToolkitConsentRevision {
			return nil
		}
	}
	return ErrBundleMCPNotCaptured
}

type bundleGuardedMCPWorld struct {
	backend  *MCPBackend
	delegate toolworldport.Provider
}

func (world *bundleGuardedMCPWorld) Definition() toolworldport.Definition {
	return world.delegate.Definition()
}
func (world *bundleGuardedMCPWorld) Discover(ctx context.Context, host toolworldport.Host) ([]toolworldport.ToolDefinition, error) {
	if world.backend != nil {
		for _, config := range world.backend.ConfiguredServers() {
			if config.ToolkitRef == nil || config.Enabled == nil || !*config.Enabled {
				continue
			}
			if err := world.backend.validateToolkitMCPDiscovery(ctx, config); err != nil {
				return nil, err
			}
		}
	}
	return world.delegate.Discover(ctx, host)
}
func (world *bundleGuardedMCPWorld) Invoke(ctx context.Context, host toolworldport.Host, id string, args json.RawMessage) (toolworldport.Result, error) {
	instanceID, ok := mcpInstanceFromProjectedTool(id)
	if ok {
		if config, found := world.backend.toolkitMCPConfig(instanceID); found {
			if err := validateToolkitMCPInvocation(ctx, config); err != nil {
				return toolworldport.Result{}, err
			}
		}
	}
	return world.delegate.Invoke(ctx, host, id, args)
}
func (world *bundleGuardedMCPWorld) Close(ctx context.Context) error {
	return world.delegate.Close(ctx)
}

func mcpInstanceFromProjectedTool(id string) (string, bool) {
	if !strings.HasPrefix(id, "mcp.") {
		return "", false
	}
	rest := strings.TrimPrefix(id, "mcp.")
	separator := strings.IndexByte(rest, '.')
	if separator <= 0 || !safeMCPServerName(rest[:separator]) {
		return "", false
	}
	return rest[:separator], true
}

// MergeMCPConfigs constructs one effective MCP catalog without touching the
// live backend. User entries keep their existing settings semantics; bundle
// entries retain their derived identity and explicit Enabled state.
func MergeMCPConfigs(user, bundled []MCPServerConfig) ([]MCPServerConfig, error) {
	byName := make(map[string]MCPServerConfig, len(user)+len(bundled))
	appendGroup := func(configs []MCPServerConfig, isBundle bool) error {
		for _, raw := range configs {
			if !safeMCPServerName(raw.Name) {
				return errors.New("runtime: invalid MCP server identity")
			}
			config := normalizeMCPServerConfig(raw)
			if _, exists := byName[config.Name]; exists {
				return fmt.Errorf("runtime: duplicate MCP server identity %q", config.Name)
			}
			if config.Endpoint != "" && config.Command != "" {
				return fmt.Errorf("runtime: MCP server %q has conflicting transports", config.Name)
			}
			if isBundle {
				if config.Enabled == nil || config.ToolkitRef == nil || config.ToolkitRef.Kind != bundlecontract.MCP ||
					config.ToolkitRef.LocalID == "" || config.ToolkitRef.InstallID == "" ||
					!bundlecontract.ValidDigest(config.ToolkitDeclarationDigest) || !bundlecontract.ValidDigest(config.ToolkitReleaseDigest) {
					return fmt.Errorf("runtime: MCP server %q is missing explicit Toolkit provenance", config.Name)
				}
				if (config.Endpoint == "") == (config.Command == "") {
					return fmt.Errorf("runtime: Toolkit MCP server %q requires exactly one transport", config.Name)
				}
			}
			if config.Endpoint != "" {
				parsed, err := url.Parse(config.Endpoint)
				if err != nil || parsed.Host == "" || parsed.User != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
					return fmt.Errorf("runtime: MCP server %q has an invalid endpoint", config.Name)
				}
			}
			byName[config.Name] = config
		}
		return nil
	}
	if err := appendGroup(user, false); err != nil {
		return nil, err
	}
	if err := appendGroup(bundled, true); err != nil {
		return nil, err
	}
	names := make([]string, 0, len(byName))
	for name := range byName {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]MCPServerConfig, 0, len(names))
	for _, name := range names {
		out = append(out, normalizeMCPServerConfig(byName[name]))
	}
	return out, nil
}

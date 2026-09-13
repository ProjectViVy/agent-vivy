package app

import (
	"slices"
	"strings"
	"testing"

	"agent-vivy/internal/config"
	genassembly "agent-vivy/internal/generated/assembly"
	"agent-vivy/internal/runtime"
	"agent-vivy/sdk/generation"
)

func TestContextHostForAssemblyDoesNotReconstructOmittedHost(t *testing.T) {
	assembly := genassembly.BuildDefault()
	assembly.Manifest.Modules = slices.DeleteFunc(assembly.Manifest.Modules, func(id string) bool {
		return id == "vivy/context-host"
	})
	backend := runtime.NewMCPBackend([]runtime.MCPServerConfig{{
		Name:           "docs",
		Endpoint:       "https://example.invalid/mcp",
		ResourceBridge: true,
	}}, nil)
	t.Cleanup(func() { _ = backend.Close() })

	got, err := contextHostForAssembly(assembly, backend)
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatal("MCP resource bridge reconstructed ContextHost without compiled vivy/context-host")
	}
}

func TestBuildGeneratedContextHostRejectsUnsealedOrRepeatedRequiredSourcePolicy(t *testing.T) {
	assembly := genassembly.BuildDefault()
	assembly.Manifest.ContextSourcePolicies = []generation.ContextSourcePolicy{{ProviderID: "mcp.resources", Required: true}}
	if _, err := buildGeneratedContextHost(assembly); err == nil || !strings.Contains(err.Error(), "unknown provider") {
		t.Fatalf("unknown required Source policy error = %v", err)
	}

	providerID := assembly.Manifest.ContextSources[0]
	assembly.Manifest.ContextSourcePolicies = []generation.ContextSourcePolicy{
		{ProviderID: providerID, Required: true},
		{ProviderID: providerID, Required: true},
	}
	if _, err := buildGeneratedContextHost(assembly); err == nil || !strings.Contains(err.Error(), "repeats") {
		t.Fatalf("repeated required Source policy error = %v", err)
	}
}

func TestValidateRuntimeAssemblyConfigRejectsResourceBridgeWithoutContextHost(t *testing.T) {
	assembly := genassembly.BuildDefault()
	assembly.Manifest.Modules = slices.DeleteFunc(assembly.Manifest.Modules, func(id string) bool {
		return id == "vivy/context-host"
	})
	cfg := config.Config{Runtime: config.Runtime{MCPServers: []config.MCPServer{{
		Name:           "docs",
		Endpoint:       "https://example.invalid/mcp",
		ResourceBridge: true,
	}}}}

	err := validateRuntimeAssemblyConfig(assembly, cfg)
	if err == nil || !strings.Contains(err.Error(), "compiled ContextHost") {
		t.Fatalf("validateRuntimeAssemblyConfig() error = %v, want compiled ContextHost rejection", err)
	}
}

func TestValidateRuntimeAssemblyConfigRejectsMCPConfigWithoutMCPHost(t *testing.T) {
	assembly := genassembly.BuildDefault()
	assembly.Manifest.Modules = slices.DeleteFunc(assembly.Manifest.Modules, func(id string) bool {
		return id == "vivy/mcp-host"
	})
	cfg := config.Config{Runtime: config.Runtime{MCPServers: []config.MCPServer{{
		Name:     "docs",
		Endpoint: "https://example.invalid/mcp",
	}}}}

	err := validateRuntimeAssemblyConfig(assembly, cfg)
	if err == nil || !strings.Contains(err.Error(), "compiled MCPHost") {
		t.Fatalf("validateRuntimeAssemblyConfig() error = %v, want compiled MCPHost rejection", err)
	}
}

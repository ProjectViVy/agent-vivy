package runtime

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"agent-vivy/internal/mcphost"
)

func TestMCPHostPrivateStdioDeathCannotReopenSession(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "spawn-count")
	t.Setenv("VIVY_MCP_HELPER", "1")
	t.Setenv("VIVY_MCP_MARKER", marker)
	t.Setenv("VIVY_MCP_EXIT_AFTER_LIST", "1")
	host, err := mcphost.New(mcphost.Config{
		Factory:        NewMCPHostSessionFactory(nil, MCPBackendOptions{ProcessRoot: t.TempDir()}),
		MaxSafeRetries: 3,
		Instances: []mcphost.InstanceConfig{{
			ID: "dies", Command: mcpHelperCommand(t),
			Args: []string{"-test.run=TestMCPStdioHelperProcess"},
			EnvFrom: map[string]string{
				"MCP_HELPER": "VIVY_MCP_HELPER", "MCP_MARKER": "VIVY_MCP_MARKER",
				"MCP_EXIT_AFTER_LIST": "VIVY_MCP_EXIT_AFTER_LIST",
			},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := host.DiscoverTools(context.Background(), "dies"); err != nil {
		t.Fatalf("initial MCPHost discovery: %v", err)
	}
	if _, err := host.DiscoverTools(context.Background(), "dies"); err == nil {
		t.Fatal("dead private stdio session unexpectedly rediscovered")
	}
	count, err := os.ReadFile(marker)
	if err != nil {
		t.Fatal(err)
	}
	if string(count) != "1" {
		t.Fatalf("MCPHost reopened dead stdio process: spawn count=%q", count)
	}
	status := host.Status()
	if len(status) != 1 || status[0].State != mcphost.StateUnavailable {
		t.Fatalf("dead private host status = %#v", status)
	}
	_ = host.Close()
}

func TestMCPHostSessionFactoryCloseCleansUpOwnedBackend(t *testing.T) {
	session, err := NewMCPHostSessionFactory(nil, MCPBackendOptions{}).Open(context.Background(), mcphost.InstanceConfig{
		ID:       "private",
		Endpoint: "https://example.invalid/mcp",
	})
	if err != nil {
		t.Fatal(err)
	}
	adapter := session.(*mcpHostSession)

	if err := session.Close(); err != nil {
		t.Fatalf("close owned session: %v", err)
	}
	if !adapter.backend.closed {
		t.Fatal("owned session close must close its private backend")
	}
	if configured := adapter.backend.ConfiguredServers(); len(configured) != 0 {
		t.Fatalf("owned backend still has configured servers after close: %#v", configured)
	}
}

func TestExistingMCPBackendSessionClosePreservesSharedFacade(t *testing.T) {
	backend := NewMCPBackend([]MCPServerConfig{{Name: "shared", Endpoint: "https://example.invalid/mcp"}}, nil)
	t.Cleanup(func() { _ = backend.Close() })

	session, err := (existingMCPBackendSessionFactory{backend: backend}).Open(context.Background(), mcphost.InstanceConfig{
		ID:       "shared",
		Endpoint: "https://example.invalid/mcp",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatalf("close compatibility Host session: %v", err)
	}
	if backend.closed {
		t.Fatal("compatibility Host session close must not close the shared facade")
	}
	configured := backend.ConfiguredServers()
	if len(configured) != 1 || configured[0].Name != "shared" {
		t.Fatalf("compatibility Host session close removed shared server: %#v", configured)
	}
}

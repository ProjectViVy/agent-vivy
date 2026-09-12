package sandbox

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"agent-vivy/internal/config"
	"agent-vivy/internal/domain"
	"agent-vivy/internal/runtime"
)

func TestSandboxModulePreservesPathAndCommandPolicy(t *testing.T) {
	root := t.TempDir()
	cfg := config.Default()
	cfg.Runtime.WorkspaceRoot = root
	cfg.Runtime.World = "local"
	cfg.Runtime.Sandbox.WorkspaceRoot = root
	cfg.Runtime.Sandbox.DefaultMode = string(domain.SandboxModeWorkspaceWrite)
	cfg.Runtime.ExecuteAllowedCommands = []string{"go"}
	provider, err := Compose(cfg)
	if err != nil {
		t.Fatal(err)
	}
	inside := filepath.Join(root, "inside.txt")
	if err := provider.Sandbox().ValidatePath(inside, runtime.FileOpWrite); err != nil {
		t.Fatalf("inside path denied: %v", err)
	}
	outside := filepath.Join(filepath.Dir(root), "outside.txt")
	if err := provider.Sandbox().ValidatePath(outside, runtime.FileOpWrite); !errors.Is(err, runtime.ErrSandboxDenied) {
		t.Fatalf("outside path error = %v, want ErrSandboxDenied", err)
	}
	if err := provider.Sandbox().ConfineCommand("sh", nil); !errors.Is(err, runtime.ErrSandboxDenied) {
		t.Fatalf("shell escape error = %v, want ErrSandboxDenied", err)
	}
	if provider.Filesystem() == nil || provider.Commands() == nil || provider.Workspaces() == nil {
		t.Fatal("composed sandbox backend is incomplete")
	}
	if _, err := os.Stat(root); err != nil {
		t.Fatal(err)
	}
}

func TestSandboxModuleDisabledWithoutWorkspace(t *testing.T) {
	cfg := config.Default()
	cfg.Runtime.WorkspaceRoot = ""
	provider, err := Compose(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if provider.Enabled() || provider.Sandbox() != nil || provider.Filesystem() != nil || provider.Commands() != nil {
		t.Fatal("workspace-free process unexpectedly enabled sandbox capabilities")
	}
}

func TestSandboxModuleOwnsCanonicalCorePort(t *testing.T) {
	descriptor := NewModule().Descriptor()
	if descriptor.Module.ID != ID || len(descriptor.Provides) != 1 || descriptor.Provides[0].Port != Port {
		t.Fatalf("descriptor = %#v", descriptor)
	}
}

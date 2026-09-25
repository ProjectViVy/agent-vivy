package rpc

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProjectInitStatusUsesAuthoritativeRootAndFailsClosed(t *testing.T) {
	root := t.TempDir()
	env := newControlTestEnv(t, func(deps *ControlDeps) { deps.ProjectRoot = root })
	initialized, rpcErr := callControl(t, env.handler, "initialize", map[string]any{})
	if rpcErr != nil || !containsCapability(initialized, "project.init.status") {
		t.Fatalf("init capability = %+v, %v", initialized, rpcErr)
	}
	status, rpcErr := callControl(t, env.handler, "project/init/status", map[string]any{})
	if rpcErr != nil || status.(map[string]any)["exists"] != false {
		t.Fatalf("absent AGENTS.md status = %+v, %v", status, rpcErr)
	}
	file := filepath.Join(root, "AGENTS.md")
	if err := os.WriteFile(file, []byte("existing instructions"), 0o600); err != nil {
		t.Fatal(err)
	}
	status, rpcErr = callControl(t, env.handler, "project/init/status", map[string]any{})
	if rpcErr != nil || status.(map[string]any)["exists"] != true {
		t.Fatalf("existing AGENTS.md status = %+v, %v", status, rpcErr)
	}
	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "outside"), file); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	status, rpcErr = callControl(t, env.handler, "project/init/status", map[string]any{})
	if rpcErr == nil || status != nil {
		t.Fatalf("symlink status must fail closed: %+v, %v", status, rpcErr)
	}
	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(file, 0o700); err != nil {
		t.Fatal(err)
	}
	status, rpcErr = callControl(t, env.handler, "project/init/status", map[string]any{})
	if rpcErr == nil || status != nil {
		t.Fatalf("directory status must fail closed: %+v, %v", status, rpcErr)
	}
}

func TestProjectInitStatusUnavailableWithoutProjectRoot(t *testing.T) {
	env := newControlTestEnv(t)
	initialized, rpcErr := callControl(t, env.handler, "initialize", map[string]any{})
	if rpcErr != nil || containsCapability(initialized, "project.init.status") {
		t.Fatalf("unexpected init capability = %+v, %v", initialized, rpcErr)
	}
	status, rpcErr := callControl(t, env.handler, "project/init/status", map[string]any{})
	if rpcErr == nil || status != nil {
		t.Fatalf("missing root status must fail: %+v, %v", status, rpcErr)
	}
}

package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

func decodeSessionResult(t *testing.T, value any) sessionResult {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var result sessionResult
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	return result
}

// Removing workspace_path from the session projection must break this test:
// the browser would otherwise group a conversation under a directory the
// runtime cannot recover after refresh.
func TestSessionWorkspaceCreateSetAndStartedConflict(t *testing.T) {
	env := newControlTestEnv(t)
	first := t.TempDir()
	second := t.TempDir()

	created, rpcErr := callControl(t, env.handler, "session/create", map[string]any{
		"title": "workspace", "workspace_path": first,
	})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	session := decodeSessionResult(t, created)
	wantFirst, _ := filepath.EvalSymlinks(first)
	if session.WorkspacePath != wantFirst {
		t.Fatalf("created workspace = %q, want %q", session.WorkspacePath, wantFirst)
	}

	updated, rpcErr := callControl(t, env.handler, "session/set_workspace", map[string]any{
		"session_id": session.ID, "workspace_path": second,
	})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	session = decodeSessionResult(t, updated)
	wantSecond, _ := filepath.EvalSymlinks(second)
	if session.WorkspacePath != wantSecond {
		t.Fatalf("updated workspace = %q, want %q", session.WorkspacePath, wantSecond)
	}

	if err := env.backend.CreateRun(context.Background(), domain.Run{
		ID: "run-workspace-lock", SessionID: session.ID, Status: domain.RunCompleted, CreatedAt: 2,
	}); err != nil {
		t.Fatal(err)
	}
	if _, rpcErr := callControl(t, env.handler, "session/set_workspace", map[string]any{
		"session_id": session.ID, "workspace_path": first,
	}); rpcErr == nil || rpcErr.Code != CodeConflict {
		t.Fatalf("started session update error = %+v, want conflict", rpcErr)
	}
}

func TestSessionWorkspaceRejectsInvalidDirectoryAndAllowsDefault(t *testing.T) {
	env := newControlTestEnv(t)
	created, rpcErr := callControl(t, env.handler, "session/create", map[string]any{
		"title": "default", "workspace_path": "",
	})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	session := decodeSessionResult(t, created)
	if session.WorkspacePath != "" {
		t.Fatalf("default workspace = %q, want empty", session.WorkspacePath)
	}
	missing := filepath.Join(t.TempDir(), "missing")
	if _, rpcErr := callControl(t, env.handler, "session/set_workspace", map[string]any{
		"session_id": session.ID, "workspace_path": missing,
	}); rpcErr == nil || rpcErr.Code != InvalidParams {
		t.Fatalf("missing directory error = %+v, want invalid params", rpcErr)
	}
}

func TestSessionWorkspaceCapabilityRequiresRuntimeMutationWiring(t *testing.T) {
	wired := newControlTestEnv(t)
	result, rpcErr := callControl(t, wired.handler, "initialize", nil)
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	if !containsCapability(result, "session.set_workspace") {
		t.Fatalf("wired handler omitted session workspace capability: %v", result)
	}

	withoutWorkspaceStore := newControlTestEnv(t, func(deps *ControlDeps) {
		deps.Sessions = struct{ storage.SessionStore }{deps.Sessions}
	})
	result, rpcErr = callControl(t, withoutWorkspaceStore.handler, "initialize", nil)
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	if containsCapability(result, "session.set_workspace") {
		t.Fatalf("unwired handler advertised session workspace capability: %v", result)
	}
}

// Returning files, symlinks, or unsorted directory rows from workspace/browse
// would make the picker both misleading and unsafe; this exercises the real
// filesystem boundary rather than a mocked listing.
func TestWorkspaceBrowseReturnsBoundedSortedRealDirectories(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"Zulu", "alpha"} {
		if err := os.Mkdir(filepath.Join(root, name), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "file.txt"), []byte("no"), 0o600); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		if err := os.Symlink(filepath.Join(root, "alpha"), filepath.Join(root, "linked")); err != nil {
			t.Fatal(err)
		}
	}

	env := newControlTestEnv(t)
	value, rpcErr := callControl(t, env.handler, "workspace/browse", map[string]any{"path": root})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	raw, _ := json.Marshal(value)
	var result workspaceBrowseResult
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Directories) != 2 || result.Directories[0].Name != "alpha" || result.Directories[1].Name != "Zulu" {
		t.Fatalf("directories = %+v, want alpha then Zulu", result.Directories)
	}
	if result.Path == "" || result.Parent == "" || len(result.Roots) == 0 {
		t.Fatalf("browse navigation incomplete: %+v", result)
	}
}

func TestWorkspaceBrowseRejectsMissingPath(t *testing.T) {
	env := newControlTestEnv(t)
	if _, rpcErr := callControl(t, env.handler, "workspace/browse", map[string]any{
		"path": filepath.Join(t.TempDir(), "missing"),
	}); rpcErr == nil || rpcErr.Code != InvalidParams {
		t.Fatalf("missing browse path error = %+v, want invalid params", rpcErr)
	}
}

func TestWorkspaceBrowseBoundsScannedEntries(t *testing.T) {
	root := t.TempDir()
	for index := 0; index <= maxWorkspaceBrowseScannedEntries; index++ {
		name := filepath.Join(root, fmt.Sprintf("file-%05d", index))
		if err := os.WriteFile(name, nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	result, err := browseWorkspaceDirectories(root)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Truncated {
		t.Fatal("large directory scan was not marked truncated")
	}
}

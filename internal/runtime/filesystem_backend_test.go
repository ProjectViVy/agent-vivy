package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudwego/eino/adk/filesystem"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/tools"
)

func newFilesystemTestBackend(t *testing.T) (*EinoFilesystemBackend, Workspace, domain.RunID) {
	t.Helper()
	manager, err := NewWorkspaceManager(filepath.Join(t.TempDir(), "workspaces"))
	if err != nil {
		t.Fatalf("new workspace manager: %v", err)
	}
	runID := domain.RunID("run_filesystem_test")
	workspace, err := manager.Ensure(context.Background(), runID)
	if err != nil {
		t.Fatalf("ensure workspace: %v", err)
	}
	return NewEinoFilesystemBackend(manager), workspace, runID
}

func TestEinoFilesystemBackendReadWriteAndPatch(t *testing.T) {
	backend, workspace, runID := newFilesystemTestBackend(t)
	ctx := context.Background()

	write, err := backend.WriteFile(ctx, runID, tools.FileWriteRequest{
		Path: "src/example.txt", Content: "one\ntwo\nthree", CreateParents: true,
	})
	if err != nil {
		t.Fatalf("write file: %v", err)
	}
	if !write.Changed || write.Path != "src/example.txt" || write.Bytes != len("one\ntwo\nthree") {
		t.Fatalf("unexpected write result: %+v", write)
	}
	hash := sha256.Sum256([]byte("one\ntwo\nthree"))
	if write.SHA256 != hex.EncodeToString(hash[:]) {
		t.Fatalf("write hash = %q", write.SHA256)
	}

	read, err := backend.ReadFile(ctx, runID, tools.FileReadRequest{Path: "src/example.txt", StartLine: 2, EndLine: 2})
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if read.Content != "two" || read.StartLine != 2 || read.EndLine != 2 || read.TotalLines != 3 {
		t.Fatalf("unexpected read result: %+v", read)
	}

	patch, err := backend.PatchFile(ctx, runID, tools.FilePatchRequest{
		Path: "src/example.txt", OldString: "two", NewString: "TWO",
	})
	if err != nil {
		t.Fatalf("patch file: %v", err)
	}
	if !patch.Changed || !strings.Contains(patch.Diff, "-one\ntwo\nthree") || !strings.Contains(patch.Diff, "+one\nTWO\nthree") {
		t.Fatalf("unexpected patch result: %+v", patch)
	}

	data, err := os.ReadFile(filepath.Join(workspace.Path, "src", "example.txt"))
	if err != nil {
		t.Fatalf("read patched fixture: %v", err)
	}
	if string(data) != "one\nTWO\nthree" {
		t.Fatalf("patched content = %q", data)
	}

	unchanged, err := backend.WriteFile(ctx, runID, tools.FileWriteRequest{
		Path: "src/example.txt", Content: "one\nTWO\nthree", CreateParents: true,
	})
	if err != nil {
		t.Fatalf("idempotent write: %v", err)
	}
	if unchanged.Changed {
		t.Fatalf("same content should not be marked changed: %+v", unchanged)
	}
}

func TestEinoFilesystemBackendSearchAndEinoMethods(t *testing.T) {
	backend, workspace, runID := newFilesystemTestBackend(t)
	for name, content := range map[string]string{
		"a.txt":        "Needle here\nother line",
		"nested/b.go":  "needle again\nno match",
		"nested/c.bin": "\x00needle",
	} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(workspace.Path, name)), 0o700); err != nil {
			t.Fatalf("mkdir fixture %s: %v", name, err)
		}
		if err := os.WriteFile(filepath.Join(workspace.Path, name), []byte(content), 0o600); err != nil {
			t.Fatalf("write fixture %s: %v", name, err)
		}
	}

	search, err := backend.SearchFiles(context.Background(), runID, tools.FileSearchRequest{
		Query: "needle", Glob: "*.txt", MaxResults: 10, CaseSensitive: false,
	})
	if err != nil {
		t.Fatalf("search files: %v", err)
	}
	if len(search.Matches) != 1 || search.Matches[0].Path != "a.txt" || search.Matches[0].Line != 1 {
		t.Fatalf("unexpected search result: %+v", search)
	}

	ctx := tools.WithRunID(context.Background(), runID)
	listed, err := backend.LsInfo(ctx, &filesystem.LsInfoRequest{Path: "nested"})
	if err != nil {
		t.Fatalf("list info: %v", err)
	}
	if len(listed) != 2 {
		t.Fatalf("list count = %d, want 2", len(listed))
	}
	globbed, err := backend.GlobInfo(ctx, &filesystem.GlobInfoRequest{Path: ".", Pattern: "*.txt"})
	if err != nil {
		t.Fatalf("glob info: %v", err)
	}
	if len(globbed) != 1 || globbed[0].Path != "a.txt" {
		t.Fatalf("unexpected glob result: %+v", globbed)
	}
	grep, err := backend.GrepRaw(ctx, &filesystem.GrepRequest{Path: ".", Pattern: "(?i)needle"})
	if err != nil {
		t.Fatalf("grep raw: %v", err)
	}
	if len(grep) != 2 {
		t.Fatalf("grep count = %d, want 2", len(grep))
	}
}

func TestEinoFilesystemBackendRejectsUnsafePathsAndPatchAmbiguity(t *testing.T) {
	backend, workspace, runID := newFilesystemTestBackend(t)
	outside := filepath.Join(t.TempDir(), "outside.txt")
	unsafe := []string{"../outside.txt", outside, "keys.txt", ".env", "secrets/token"}
	for _, path := range unsafe {
		if _, err := backend.WriteFile(context.Background(), runID, tools.FileWriteRequest{Path: path, Content: "blocked", CreateParents: true}); err == nil {
			t.Errorf("path %q was accepted", path)
		}
	}

	if err := os.WriteFile(filepath.Join(workspace.Path, "repeat.txt"), []byte("repeat\nrepeat"), 0o600); err != nil {
		t.Fatalf("write repeat fixture: %v", err)
	}
	if _, err := backend.PatchFile(context.Background(), runID, tools.FilePatchRequest{
		Path: "repeat.txt", OldString: "repeat", NewString: "changed",
	}); err == nil || !strings.Contains(err.Error(), "matched 2 times") {
		t.Fatalf("ambiguous patch error = %v", err)
	}
	if _, err := backend.PatchFile(context.Background(), runID, tools.FilePatchRequest{
		Path: "repeat.txt", OldString: "repeat", NewString: "changed", ReplaceAll: true,
	}); err != nil {
		t.Fatalf("replace all patch: %v", err)
	}

	link := filepath.Join(workspace.Path, "link.txt")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlink unavailable on this Windows account: %v", err)
	}
	if _, err := backend.ReadFile(context.Background(), runID, tools.FileReadRequest{Path: "link.txt"}); err == nil {
		t.Fatal("symlink read should be rejected")
	}
}

func TestEinoFilesystemBackendHonorsCancellation(t *testing.T) {
	backend, _, runID := newFilesystemTestBackend(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := backend.ReadFile(ctx, runID, tools.FileReadRequest{Path: "missing.txt"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("read error = %v, want context cancellation", err)
	}
}

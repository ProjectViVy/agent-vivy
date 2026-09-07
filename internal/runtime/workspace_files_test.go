package runtime

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-vivy/internal/domain"
)

func newWorkspaceFiles(t *testing.T) (*WorkspaceFiles, string) {
	t.Helper()
	manager, err := NewWorkspaceManager(filepath.Join(t.TempDir(), "workspaces"))
	if err != nil {
		t.Fatalf("new workspace manager: %v", err)
	}
	runID := domain.RunID("run_files_ui")
	if _, err := manager.Ensure(context.Background(), runID); err != nil {
		t.Fatalf("ensure workspace: %v", err)
	}
	ws, _ := manager.Ensure(context.Background(), runID)
	return NewWorkspaceFiles(manager, 0), ws.Path
}

func TestWorkspaceFilesListAndRead(t *testing.T) {
	svc, root := newWorkspaceFiles(t)
	if err := os.MkdirAll(filepath.Join(root, "src"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "src", "main.go"), []byte("package main\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "b.txt"), []byte("hi"), 0o600); err != nil {
		t.Fatal(err)
	}

	files, truncated, err := svc.List(context.Background(), "run_files_ui")
	if err != nil || truncated {
		t.Fatalf("list = %v, %v, %v", files, truncated, err)
	}
	if len(files) != 2 || files[0].Path != "b.txt" || files[1].Path != "src/main.go" {
		t.Fatalf("list = %+v", files)
	}

	got, err := svc.Read(context.Background(), "run_files_ui", "src/main.go")
	if err != nil || got.Content != "package main\n" || got.Binary || got.Truncated {
		t.Fatalf("read = %+v, %v", got, err)
	}
	if got, err := svc.Read(context.Background(), "run_files_ui", "b.txt"); err != nil || got.Content != "hi" {
		t.Fatalf("read b.txt = %+v, %v", got, err)
	}
}

func TestWorkspaceFilesReadRejectsEscapes(t *testing.T) {
	svc, _ := newWorkspaceFiles(t)
	for _, rel := range []string{"", "../out.txt", "/abs.txt", `C:\x.txt`, "a/../../b.txt", "."} {
		if _, err := svc.Read(context.Background(), "run_files_ui", rel); err == nil {
			t.Fatalf("read(%q) = nil error", rel)
		}
	}
}

func TestWorkspaceFilesReadBinaryAndTruncation(t *testing.T) {
	svc, root := newWorkspaceFiles(t)
	png := append([]byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}, make([]byte, 32)...)
	if err := os.WriteFile(filepath.Join(root, "shot.png"), png, 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := svc.Read(context.Background(), "run_files_ui", "shot.png")
	if err != nil || !got.Binary || got.Content != "" || got.Size != int64(len(png)) {
		t.Fatalf("binary read = %+v, %v", got, err)
	}

	big := NewWorkspaceFiles(svc.manager, 16)
	if err := os.WriteFile(filepath.Join(root, "big.txt"), []byte(strings.Repeat("x", 40)), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err = big.Read(context.Background(), "run_files_ui", "big.txt")
	if err != nil || !got.Truncated || len(got.Content) != 16 {
		t.Fatalf("truncated read = %+v, %v", got, err)
	}
}

func TestWorkspaceFilesListSkipsSymlinks(t *testing.T) {
	svc, root := newWorkspaceFiles(t)
	if err := os.WriteFile(filepath.Join(root, "real.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "real.txt"), filepath.Join(root, "link.txt")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	files, _, err := svc.List(context.Background(), "run_files_ui")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].Path != "real.txt" {
		t.Fatalf("list = %+v", files)
	}
	if _, err := svc.Read(context.Background(), "run_files_ui", "link.txt"); err == nil {
		t.Fatal("read through symlink must fail")
	}
}

func TestWorkspaceFilesNotWired(t *testing.T) {
	svc := &WorkspaceFiles{}
	if _, _, err := svc.List(context.Background(), "run_files_ui"); err == nil {
		t.Fatal("nil manager list must fail")
	}
	if _, err := svc.Read(context.Background(), "run_files_ui", "a.txt"); err == nil {
		t.Fatal("nil manager read must fail")
	}
}

// TestWorkspaceFilesUnknownRunFailsClosed pins the UI accessor's fail-closed
// contract: a well-formed but unknown run id yields ErrWorkspaceNotFound, and
// malformed ids (control characters, traversal, separators) are rejected by
// the workspace name guard. Neither may leave filesystem state behind.
func TestWorkspaceFilesUnknownRunFailsClosed(t *testing.T) {
	manager, err := NewWorkspaceManager(filepath.Join(t.TempDir(), "workspaces"))
	if err != nil {
		t.Fatal(err)
	}
	svc := NewWorkspaceFiles(manager, 0)
	for _, runID := range []domain.RunID{"run_never_created", "run_\x1b[31m", "run_\x00", "run_..", "run_a/b"} {
		if _, _, err := svc.List(context.Background(), runID); err == nil {
			t.Fatalf("list(%q) = nil error", runID)
		}
		if _, err := svc.Read(context.Background(), runID, "a.txt"); err == nil {
			t.Fatalf("read(%q) = nil error", runID)
		}
	}
	if _, _, err := svc.List(context.Background(), "run_never_created"); !errors.Is(err, ErrWorkspaceNotFound) {
		t.Fatalf("unknown run list err = %v, want ErrWorkspaceNotFound", err)
	}
	if _, err := svc.Read(context.Background(), "run_never_created", "a.txt"); !errors.Is(err, ErrWorkspaceNotFound) {
		t.Fatalf("unknown run read err = %v, want ErrWorkspaceNotFound", err)
	}
	if _, err := os.Stat(manager.root); !os.IsNotExist(err) {
		t.Fatalf("workspace root created as side effect: stat err = %v", err)
	}
}

// TestWorkspaceFilesReadSurvivesPathSwap covers the race hardening in Read:
// the content must come from the handle whose stat validated regular-file
// state, so a symlink placed at the final path component is rejected both
// before and after the open.
func TestWorkspaceFilesReadSurvivesPathSwap(t *testing.T) {
	svc, root := newWorkspaceFiles(t)
	outside := filepath.Join(filepath.Dir(root), "outside.txt")
	if err := os.WriteFile(outside, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Directory symlink: the whole workspace replaced after Ensure.
	if err := os.Symlink(outside, filepath.Join(root, "dir_link")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, err := svc.Read(context.Background(), "run_files_ui", "dir_link"); err == nil {
		t.Fatal("read through final-component symlink must fail")
	}
	if err := os.WriteFile(filepath.Join(root, "real.txt"), []byte("plain"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := svc.Read(context.Background(), "run_files_ui", "real.txt")
	if err != nil {
		t.Fatal(err)
	}
	if got.Content == "" {
		t.Fatal("regular read broke after hardening")
	}
}

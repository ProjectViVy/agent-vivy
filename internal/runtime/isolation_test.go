package runtime

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"agent-vivy/internal/domain"
)

func TestWorkspaceManagerAllocatesPerRunInsideRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "sandboxes")
	m, err := NewWorkspaceManager(root)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	one, err := m.Ensure(context.Background(), domain.RunID("run_alpha"))
	if err != nil {
		t.Fatalf("ensure first: %v", err)
	}
	two, err := m.Ensure(context.Background(), domain.RunID("run_beta"))
	if err != nil {
		t.Fatalf("ensure second: %v", err)
	}
	if one.Path == two.Path || one.ID == two.ID {
		t.Fatalf("workspaces are not isolated: %+v %+v", one, two)
	}
	if err := m.ValidatePath(one.Path); err != nil {
		t.Fatalf("validate child path: %v", err)
	}
	if err := m.ValidatePath(filepath.Join(root, "..", "outside")); err == nil {
		t.Fatal("want path escape rejection")
	}
	if info, err := os.Stat(one.Path); err != nil || !info.IsDir() {
		t.Fatalf("workspace stat = %v/%v", info, err)
	}
}

func TestWorkspaceManagerRejectsTraversalAndSymlink(t *testing.T) {
	root := t.TempDir()
	m, err := NewWorkspaceManager(root)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	for _, id := range []domain.RunID{"..", `run\escape`, "run/escape", "run..escape"} {
		if _, err := m.Ensure(context.Background(), id); err == nil {
			t.Errorf("run id %q: want rejection", id)
		}
	}
	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.Mkdir(outside, 0o700); err != nil {
		t.Fatalf("mkdir outside: %v", err)
	}
	link := filepath.Join(root, "run_link")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlink unavailable on this Windows account: %v", err)
	}
	if _, err := m.Ensure(context.Background(), domain.RunID("run_link")); err == nil {
		t.Fatal("want symlink workspace rejection")
	}
}

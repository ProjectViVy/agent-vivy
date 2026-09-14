package runtime

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

type workspaceSessionLookup map[domain.SessionID]domain.Session

func (lookup workspaceSessionLookup) GetSession(_ context.Context, id domain.SessionID) (domain.Session, error) {
	if session, ok := lookup[id]; ok {
		return session, nil
	}
	return domain.Session{}, storage.ErrNotFound
}

type workspaceRunLookup map[domain.RunID]domain.Run

func (lookup workspaceRunLookup) GetRun(_ context.Context, id domain.RunID) (domain.Run, error) {
	if run, ok := lookup[id]; ok {
		return run, nil
	}
	return domain.Run{}, storage.ErrNotFound
}

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

func TestWorkspaceManagerExistingNeverCreates(t *testing.T) {
	root := filepath.Join(t.TempDir(), "sandboxes")
	manager, err := NewWorkspaceManager(root)
	if err != nil {
		t.Fatal(err)
	}
	if workspace, exists, err := manager.Existing(context.Background(), "run_missing"); err != nil || exists || workspace.Path != "" {
		t.Fatalf("missing existing workspace = %+v/%v/%v", workspace, exists, err)
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatalf("status lookup created workspace root: %v", err)
	}
	want, err := manager.Ensure(context.Background(), "run_present")
	if err != nil {
		t.Fatal(err)
	}
	got, exists, err := manager.Existing(context.Background(), "run_present")
	if err != nil || !exists || got != want {
		t.Fatalf("existing workspace = %+v/%v/%v, want %+v", got, exists, err, want)
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

func TestLocalWorkspaceManagerMountsProjectRoot(t *testing.T) {
	root := t.TempDir()
	m, err := NewLocalWorkspaceManager(root)
	if err != nil {
		t.Fatal(err)
	}
	one, err := m.Ensure(context.Background(), domain.RunID("run_one"))
	if err != nil {
		t.Fatal(err)
	}
	two, err := m.Ensure(context.Background(), domain.RunID("run_two"))
	if err != nil {
		t.Fatal(err)
	}
	if one.ID != "local" || one.Path != root || two != one {
		t.Fatalf("local workspaces = %+v %+v, want shared root %q", one, two, root)
	}
	if err := m.ValidatePath(filepath.Join(root, "child.txt")); err != nil {
		t.Fatalf("validate child: %v", err)
	}
	if err := m.ValidatePath(filepath.Join(root, "..", "outside.txt")); err == nil {
		t.Fatal("want local world escape rejection")
	}
}

// Replacing the session lookup with the process-wide root must fail this
// test: two conversations selected in different projects may never share the
// same filesystem authority.
func TestSessionWorkspaceManagerResolvesDefaultAndSelectedRoots(t *testing.T) {
	defaultRoot := filepath.Join(t.TempDir(), "default")
	alpha := t.TempDir()
	beta := t.TempDir()
	sessions := workspaceSessionLookup{
		"sess-default": {ID: "sess-default"},
		"sess-alpha":   {ID: "sess-alpha", WorkspacePath: alpha},
		"sess-beta":    {ID: "sess-beta", WorkspacePath: beta},
	}
	runs := workspaceRunLookup{
		"run-default": {ID: "run-default", SessionID: "sess-default"},
		"run-alpha":   {ID: "run-alpha", SessionID: "sess-alpha"},
		"run-beta":    {ID: "run-beta", SessionID: "sess-beta"},
	}
	manager, err := NewSessionWorkspaceManager(defaultRoot, sessions, runs)
	if err != nil {
		t.Fatal(err)
	}

	defaultWorkspace, err := manager.Ensure(context.Background(), "run-default")
	if err != nil {
		t.Fatal(err)
	}
	if defaultWorkspace.Path != filepath.Join(defaultRoot, "run-default") {
		t.Fatalf("default workspace = %q", defaultWorkspace.Path)
	}
	alphaWorkspace, err := manager.Ensure(context.Background(), "run-alpha")
	if err != nil {
		t.Fatal(err)
	}
	betaWorkspace, err := manager.Ensure(context.Background(), "run-beta")
	if err != nil {
		t.Fatal(err)
	}
	if alphaWorkspace.Path != alpha || betaWorkspace.Path != beta || alphaWorkspace.ID == betaWorkspace.ID {
		t.Fatalf("selected workspaces = %+v / %+v", alphaWorkspace, betaWorkspace)
	}

	// A new run is allocated before its durable Run row exists. The service
	// puts the already-authoritative session id in context for that boundary.
	fromContext, err := manager.Ensure(withSessionID(context.Background(), "sess-alpha"), "run-new")
	if err != nil || fromContext.Path != alpha {
		t.Fatalf("pre-persist selected workspace = %+v, err=%v", fromContext, err)
	}
	durableOwner, err := manager.Ensure(withSessionID(context.Background(), "sess-beta"), "run-alpha")
	if err != nil || durableOwner.Path != alpha {
		t.Fatalf("durable run owner lost to stale context = %+v, err=%v", durableOwner, err)
	}
}

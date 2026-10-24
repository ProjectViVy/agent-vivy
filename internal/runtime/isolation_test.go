package runtime

import (
	"context"
	"errors"
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

func TestWorkspaceManagerReleaseRemovesPrivateRunWorkspace(t *testing.T) {
	root := filepath.Join(t.TempDir(), "sandboxes")
	manager, err := NewWorkspaceManager(root)
	if err != nil {
		t.Fatal(err)
	}
	workspace, err := manager.Ensure(context.Background(), "run_release")
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Release(context.Background(), workspace); err != nil {
		t.Fatalf("release workspace: %v", err)
	}
	if _, err := os.Stat(workspace.Path); !os.IsNotExist(err) {
		t.Fatalf("released workspace still exists: %v", err)
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
	canonicalAlpha, err := filepath.EvalSymlinks(alpha)
	if err != nil {
		t.Fatal(err)
	}
	canonicalBeta, err := filepath.EvalSymlinks(beta)
	if err != nil {
		t.Fatal(err)
	}
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
	if alphaWorkspace.Path != canonicalAlpha || betaWorkspace.Path != canonicalBeta || alphaWorkspace.ID == betaWorkspace.ID {
		t.Fatalf("selected workspaces = %+v / %+v", alphaWorkspace, betaWorkspace)
	}

	// A new run is allocated before its durable Run row exists. The service
	// puts the already-authoritative session id in context for that boundary.
	fromContext, err := manager.Ensure(withSessionID(context.Background(), "sess-alpha"), "run-new")
	if err != nil || fromContext.Path != canonicalAlpha {
		t.Fatalf("pre-persist selected workspace = %+v, err=%v", fromContext, err)
	}
	durableOwner, err := manager.Ensure(withSessionID(context.Background(), "sess-beta"), "run-alpha")
	if err != nil || durableOwner.Path != canonicalAlpha {
		t.Fatalf("durable run owner lost to stale context = %+v, err=%v", durableOwner, err)
	}
}

// ---------------------------------------------------------------------
// AdmissionWorkspaceAllocator (SC-D4 §7): admission allocation reports
// whether it created a fresh private directory; rollback removes only an
// empty, root-contained, run-ID-named private dir and never a user dir.

func TestEnsureForAdmissionPrivateLifecycle(t *testing.T) {
	root := filepath.Join(t.TempDir(), "ws")
	manager, err := NewWorkspaceManager(root)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	workspace, newly, err := manager.EnsureForAdmission(ctx, "run-admission")
	if err != nil {
		t.Fatalf("EnsureForAdmission: %v", err)
	}
	if !newly {
		t.Fatal("fresh private allocation must report newly=true")
	}
	if workspace.Path != filepath.Join(root, "run-admission") {
		t.Fatalf("workspace path = %q", workspace.Path)
	}
	if _, newly, err = manager.EnsureForAdmission(ctx, "run-admission"); err != nil || newly {
		t.Fatalf("pre-existing private dir: newly=%v err=%v, want preserved", newly, err)
	}
	if err := manager.DiscardNewPrivateAdmission(ctx, "run-admission"); err != nil {
		t.Fatalf("discard: %v", err)
	}
	if _, err := os.Stat(workspace.Path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("discarded workspace still exists: %v", err)
	}
	if err := manager.DiscardNewPrivateAdmission(ctx, "run-admission"); err != nil {
		t.Fatalf("repeat discard must be a no-op: %v", err)
	}
}

func TestDiscardNewPrivateAdmissionPreservesNonEmptyDir(t *testing.T) {
	root := filepath.Join(t.TempDir(), "ws")
	manager, err := NewWorkspaceManager(root)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	workspace, newly, err := manager.EnsureForAdmission(ctx, "run-content")
	if err != nil || !newly {
		t.Fatalf("ensure: newly=%v err=%v", newly, err)
	}
	if err := os.WriteFile(filepath.Join(workspace.Path, "artifact.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := manager.DiscardNewPrivateAdmission(ctx, "run-content"); err == nil {
		t.Fatal("discard of a non-empty workspace must fail, not remove content")
	}
	if _, err := os.Stat(filepath.Join(workspace.Path, "artifact.txt")); err != nil {
		t.Fatalf("workspace content was destroyed: %v", err)
	}
}

func TestAdmissionAllocatorNeverRemovesUserDirectories(t *testing.T) {
	ctx := context.Background()

	local := t.TempDir()
	localManager, err := NewLocalWorkspaceManager(local)
	if err != nil {
		t.Fatal(err)
	}
	if _, newly, err := localManager.EnsureForAdmission(ctx, "run-local"); err != nil || newly {
		t.Fatalf("local workspace: newly=%v err=%v, want reused user dir", newly, err)
	}
	if err := localManager.DiscardNewPrivateAdmission(ctx, "run-local"); err != nil {
		t.Fatalf("local discard must be a no-op: %v", err)
	}
	if _, err := os.Stat(local); err != nil {
		t.Fatalf("local workspace was removed: %v", err)
	}

	selected := t.TempDir()
	canonicalSelected, err := filepath.EvalSymlinks(selected)
	if err != nil {
		t.Fatal(err)
	}
	defaultRoot := filepath.Join(t.TempDir(), "default")
	sessions := workspaceSessionLookup{"sess-sel": {ID: "sess-sel", WorkspacePath: selected}}
	manager, err := NewSessionWorkspaceManager(defaultRoot, sessions, workspaceRunLookup{})
	if err != nil {
		t.Fatal(err)
	}
	selCtx := withSessionID(ctx, "sess-sel")
	workspace, newly, err := manager.EnsureForAdmission(selCtx, "run-selected")
	if err != nil || newly {
		t.Fatalf("selected workspace: newly=%v err=%v, want reused user dir", newly, err)
	}
	if workspace.Path != canonicalSelected {
		t.Fatalf("selected workspace path = %q, want %q", workspace.Path, canonicalSelected)
	}
	if err := manager.DiscardNewPrivateAdmission(selCtx, "run-selected"); err != nil {
		t.Fatalf("selected discard must be a no-op: %v", err)
	}
	if _, err := os.Stat(selected); err != nil {
		t.Fatalf("selected workspace was removed: %v", err)
	}
}

// ---------------------------------------------------------------------
// Child runs resolve to their root run's private directory, so a
// delegated task can read and write what the run that delegated it
// produced. RootID rather than ParentID keeps a whole bounded descendant
// tree in one directory instead of chaining a fresh one per level.

// newChildWorkspaceManager builds a sandbox-world manager whose session has no
// explicit selection, so requests reach the private per-run branch.
func newChildWorkspaceManager(t *testing.T, runs workspaceRunLookup) (*WorkspaceManager, string) {
	t.Helper()
	root := filepath.Join(t.TempDir(), "sandboxes")
	sessions := workspaceSessionLookup{"sess-parent": {ID: "sess-parent"}}
	manager, err := NewSessionWorkspaceManager(root, sessions, runs)
	if err != nil {
		t.Fatal(err)
	}
	return manager, root
}

func TestChildRunSharesRootWorkspace(t *testing.T) {
	manager, root := newChildWorkspaceManager(t, workspaceRunLookup{
		"run_root":  {ID: "run_root", SessionID: "sess-parent"},
		"child_one": {ID: "child_one", SessionID: "sess-parent", Kind: domain.RunKindChild, ParentID: "run_root", RootID: "run_root", Depth: 1},
		"child_two": {ID: "child_two", SessionID: "sess-parent", Kind: domain.RunKindChild, ParentID: "child_one", RootID: "run_root", Depth: 2},
	})
	ctx := withSessionID(context.Background(), "sess-parent")

	rootWorkspace, err := manager.Ensure(ctx, "run_root")
	if err != nil {
		t.Fatal(err)
	}
	if rootWorkspace.redirected {
		t.Fatalf("a root run must own its workspace: %+v", rootWorkspace)
	}
	if rootWorkspace.Path != filepath.Join(root, "run_root") || rootWorkspace.ID != "run_root" {
		t.Fatalf("root workspace = %+v", rootWorkspace)
	}
	proof := filepath.Join(rootWorkspace.Path, "proof.txt")
	if err := os.WriteFile(proof, []byte("written by the parent"), 0o600); err != nil {
		t.Fatal(err)
	}

	for _, childID := range []domain.RunID{"child_one", "child_two"} {
		child, err := manager.Ensure(ctx, childID)
		if err != nil {
			t.Fatalf("ensure %s: %v", childID, err)
		}
		if child.Path != rootWorkspace.Path || child.ID != rootWorkspace.ID {
			t.Fatalf("%s workspace = %+v, want the root's %+v", childID, child, rootWorkspace)
		}
		if !child.redirected {
			t.Fatalf("%s must be marked redirected so its lifecycle cannot reclaim the shared directory", childID)
		}
		got, err := os.ReadFile(filepath.Join(child.Path, "proof.txt"))
		if err != nil || string(got) != "written by the parent" {
			t.Fatalf("%s cannot see the parent's file: %q/%v", childID, got, err)
		}
		// Existing and ValidateRunPath resolve the same directory.
		existing, ok, err := manager.Existing(ctx, childID)
		if err != nil || !ok || existing.Path != rootWorkspace.Path {
			t.Fatalf("existing %s = %+v/%v/%v", childID, existing, ok, err)
		}
		if err := manager.ValidateRunPath(ctx, childID, proof); err != nil {
			t.Fatalf("validate %s path: %v", childID, err)
		}
	}

	// One directory for the whole descendant tree: no per-child directories.
	if names := workspaceDirNames(t, root); len(names) != 1 || names[0] != "run_root" {
		t.Fatalf("root holds %v, want only run_root", names)
	}
}

// workspaceDirNames lists a workspace root so tests can assert which private
// directories actually exist.
func workspaceDirNames(t *testing.T, root string) []string {
	t.Helper()
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}

func TestRedirectedChildWorkspaceIsNeverReleased(t *testing.T) {
	manager, root := newChildWorkspaceManager(t, workspaceRunLookup{
		"run_root":  {ID: "run_root", SessionID: "sess-parent"},
		"child_one": {ID: "child_one", SessionID: "sess-parent", Kind: domain.RunKindChild, ParentID: "run_root", RootID: "run_root", Depth: 1},
	})
	ctx := withSessionID(context.Background(), "sess-parent")
	rootWorkspace, err := manager.Ensure(ctx, "run_root")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rootWorkspace.Path, "proof.txt"), []byte("keep me"), 0o600); err != nil {
		t.Fatal(err)
	}
	child, err := manager.Ensure(ctx, "child_one")
	if err != nil {
		t.Fatal(err)
	}
	// The child's admission rollback must not remove the shared directory:
	// its path has the private root/<ID> shape, so the shape check alone
	// would let the removal through.
	if err := manager.Release(context.WithoutCancel(ctx), child); err != nil {
		t.Fatalf("release redirected workspace: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "run_root", "proof.txt")); err != nil {
		t.Fatalf("the root run's workspace was destroyed by a child release: %v", err)
	}
	// The owner's own release still reclaims it.
	if err := manager.Release(ctx, rootWorkspace); err != nil {
		t.Fatalf("release root workspace: %v", err)
	}
	if _, err := os.Stat(rootWorkspace.Path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("root workspace survived its own release: %v", err)
	}
}

func TestChildAdmissionNeverReclaimsTheSharedWorkspace(t *testing.T) {
	manager, root := newChildWorkspaceManager(t, workspaceRunLookup{
		"child_one": {ID: "child_one", SessionID: "sess-parent", Kind: domain.RunKindChild, ParentID: "run_root", RootID: "run_root", Depth: 1},
	})
	ctx := withSessionID(context.Background(), "sess-parent")

	// The crash window: the root's directory does not exist yet, so this call
	// really does create it. It must still not be reported as reclaimable.
	workspace, newly, err := manager.EnsureForAdmission(ctx, "child_one")
	if err != nil {
		t.Fatalf("EnsureForAdmission: %v", err)
	}
	if newly {
		t.Fatal("a redirected child admission must never report newly=true")
	}
	if workspace.Path != filepath.Join(root, "run_root") || !workspace.redirected {
		t.Fatalf("child admission workspace = %+v", workspace)
	}
	if err := manager.DiscardNewPrivateAdmission(ctx, "child_one"); err != nil {
		t.Fatalf("child discard must be a no-op: %v", err)
	}
	if _, err := os.Stat(workspace.Path); err != nil {
		t.Fatalf("the shared workspace was discarded by a child admission: %v", err)
	}
}

func TestWorkspaceOwnerRunContextBridgeBeforePersist(t *testing.T) {
	// Child admission allocates the workspace before CreateRun persists the
	// child row, so the durable lookup cannot answer yet and the caller carries
	// the already-computed root id in context.
	manager, root := newChildWorkspaceManager(t, workspaceRunLookup{
		"run_root": {ID: "run_root", SessionID: "sess-parent"},
	})
	bridged := withWorkspaceOwnerRun(withSessionID(context.Background(), "sess-parent"), "run_root")
	workspace, err := manager.Ensure(bridged, "child_new")
	if err != nil {
		t.Fatalf("ensure pre-persist child: %v", err)
	}
	if workspace.Path != filepath.Join(root, "run_root") || !workspace.redirected {
		t.Fatalf("pre-persist child workspace = %+v, want the root's directory", workspace)
	}

	// Without the bridge an unpersisted run keeps its own directory, and a
	// durable row wins over a stale context value.
	plain, err := manager.Ensure(withSessionID(context.Background(), "sess-parent"), "child_plain")
	if err != nil {
		t.Fatal(err)
	}
	if plain.Path != filepath.Join(root, "child_plain") || plain.redirected {
		t.Fatalf("unbridged pre-persist workspace = %+v", plain)
	}
	durable := withWorkspaceOwnerRun(withSessionID(context.Background(), "sess-parent"), "child_plain")
	resolved, err := manager.Ensure(durable, "run_root")
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Path != filepath.Join(root, "run_root") || resolved.redirected {
		t.Fatalf("durable run lost to a stale owner context = %+v", resolved)
	}
}

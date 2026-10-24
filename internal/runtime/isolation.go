package runtime

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

// Workspace is the only filesystem identity a background run may receive.
// The path is deterministic per run so restart recovery can reattach without
// persisting an absolute host path in the Journal.
type Workspace struct {
	ID   string
	Path string
	// redirected reports that this workspace belongs to a different run than
	// the one that asked for it: a child run resolves to its root run's
	// directory. Such a workspace is shared state, so the requesting run's
	// lifecycle must never release or discard it.
	redirected bool
}

// WorkspaceManager allocates private directory sandboxes under one validated
// root. It intentionally does not execute git or shell commands; a future
// filesystem/subagent tool must be wired to this boundary explicitly.
type WorkspaceManager struct {
	root     string
	local    bool
	sessions SessionWorkspaceLookup
	runs     RunWorkspaceLookup
}

// SessionWorkspaceLookup resolves the durable project directory attached to
// a conversation. It is deliberately narrower than storage.SessionStore.
type SessionWorkspaceLookup interface {
	GetSession(context.Context, domain.SessionID) (domain.Session, error)
}

// RunWorkspaceLookup recovers session ownership when a workspace is inspected
// outside an active run context (restart, file preview, LSP status).
type RunWorkspaceLookup interface {
	GetRun(context.Context, domain.RunID) (domain.Run, error)
}

// NewWorkspaceManager validates and normalizes the isolation root. The root
// itself is not created until the first run needs a workspace.
func NewWorkspaceManager(root string) (*WorkspaceManager, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, errors.New("runtime: workspace root must not be empty")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("runtime: resolve workspace root: %w", err)
	}
	return &WorkspaceManager{root: filepath.Clean(abs)}, nil
}

// NewSessionWorkspaceManager preserves private per-run workspaces for
// sessions without an explicit selection and mounts a selected project root
// for every run in that session.
func NewSessionWorkspaceManager(root string, sessions SessionWorkspaceLookup, runs RunWorkspaceLookup) (*WorkspaceManager, error) {
	m, err := NewWorkspaceManager(root)
	if err != nil {
		return nil, err
	}
	if sessions == nil || runs == nil {
		return nil, errors.New("runtime: session workspace lookups must be wired")
	}
	m.sessions = sessions
	m.runs = runs
	return m, nil
}

// NewLocalWorkspaceManager mounts root itself as the workspace for every
// run. It is the code-face world: commands and file tools operate on the
// project the user launched Vivy from, while path validation, sandbox policy,
// approvals, protected-file checks, and the Journal remain unchanged.
func NewLocalWorkspaceManager(root string) (*WorkspaceManager, error) {
	m, err := NewWorkspaceManager(root)
	if err != nil {
		return nil, err
	}
	m.local = true
	return m, nil
}

// Ensure creates or verifies the private workspace for a run. IDs are
// constrained before joining paths, and symlinks are rejected so a malicious
// run ID cannot redirect writes outside the root. A child run resolves to its
// root run's directory rather than a directory of its own.
func (m *WorkspaceManager) Ensure(ctx context.Context, runID domain.RunID) (Workspace, error) {
	if m == nil || m.root == "" {
		return Workspace{}, errors.New("runtime: workspace manager not wired")
	}
	if err := ctx.Err(); err != nil {
		return Workspace{}, err
	}
	run, sessionID, err := m.workspaceRunOwner(ctx, runID)
	if err != nil {
		return Workspace{}, err
	}
	if selected, ok, err := m.selectedWorkspace(ctx, sessionID); err != nil {
		return Workspace{}, err
	} else if ok {
		return selected, nil
	}
	if m.local {
		info, err := os.Lstat(m.root)
		if err != nil {
			return Workspace{}, fmt.Errorf("runtime: inspect local workspace: %w", err)
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return Workspace{}, errors.New("runtime: local workspace root is not a directory")
		}
		return Workspace{ID: "local", Path: m.root}, nil
	}
	owner := workspaceOwnerRunID(ctx, runID, run)
	name := string(owner)
	if !validWorkspaceName(name) {
		return Workspace{}, errors.New("runtime: invalid workspace run id")
	}
	path := filepath.Join(m.root, name)
	if err := m.ensureUnderRoot(path); err != nil {
		return Workspace{}, err
	}
	if err := os.MkdirAll(m.root, 0o700); err != nil {
		return Workspace{}, fmt.Errorf("runtime: create workspace root: %w", err)
	}
	if info, err := os.Lstat(path); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return Workspace{}, errors.New("runtime: workspace path is not a private directory")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return Workspace{}, fmt.Errorf("runtime: inspect workspace: %w", err)
	} else if err := os.Mkdir(path, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
		return Workspace{}, fmt.Errorf("runtime: create workspace: %w", err)
	}
	return Workspace{ID: name, Path: path, redirected: owner != runID}, nil
}

// Release removes only a private per-run workspace allocated beneath this
// manager's root. Local, selected and redirected workspaces are shared
// resources and therefore remain untouched. The runtime calls this only before
// an admission commit, so a failed startup cannot strand an unowned directory.
func (m *WorkspaceManager) Release(ctx context.Context, workspace Workspace) error {
	if m == nil || m.root == "" || workspace.ID == "" || workspace.Path == "" {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if workspace.redirected {
		// The directory belongs to the root run and is shared with every
		// descendant. Its shape matches root/<ID>, so the path check below
		// would pass — the child's admission rollback must still not remove it.
		return nil
	}
	if m.local || !validWorkspaceName(workspace.ID) {
		return nil
	}
	expected := filepath.Join(m.root, workspace.ID)
	if filepath.Clean(workspace.Path) != filepath.Clean(expected) {
		// A selected project workspace does not have the private per-run shape.
		return nil
	}
	if err := m.ensureUnderRoot(expected); err != nil {
		return err
	}
	info, err := os.Lstat(expected)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("runtime: inspect workspace for release: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return errors.New("runtime: refusing to release a non-private workspace")
	}
	if err := os.RemoveAll(expected); err != nil {
		return fmt.Errorf("runtime: release workspace: %w", err)
	}
	return nil
}

// Existing resolves a run workspace only when it already exists. Unlike
// Ensure it never creates filesystem state, which makes it safe for status
// and inspection paths.
func (m *WorkspaceManager) Existing(ctx context.Context, runID domain.RunID) (Workspace, bool, error) {
	if m == nil || m.root == "" {
		return Workspace{}, false, errors.New("runtime: workspace manager not wired")
	}
	if err := ctx.Err(); err != nil {
		return Workspace{}, false, err
	}
	run, sessionID, err := m.workspaceRunOwner(ctx, runID)
	if err != nil {
		return Workspace{}, false, err
	}
	if selected, ok, err := m.selectedWorkspace(ctx, sessionID); err != nil {
		return Workspace{}, false, err
	} else if ok {
		return selected, true, nil
	}
	if m.local {
		info, err := os.Lstat(m.root)
		if errors.Is(err, os.ErrNotExist) {
			return Workspace{}, false, nil
		}
		if err != nil {
			return Workspace{}, false, fmt.Errorf("runtime: inspect local workspace: %w", err)
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return Workspace{}, false, errors.New("runtime: local workspace root is not a directory")
		}
		return Workspace{ID: "local", Path: m.root}, true, nil
	}
	owner := workspaceOwnerRunID(ctx, runID, run)
	name := string(owner)
	if !validWorkspaceName(name) {
		return Workspace{}, false, errors.New("runtime: invalid workspace run id")
	}
	workspacePath := filepath.Join(m.root, name)
	if err := m.ensureUnderRoot(workspacePath); err != nil {
		return Workspace{}, false, err
	}
	info, err := os.Lstat(workspacePath)
	if errors.Is(err, os.ErrNotExist) {
		return Workspace{}, false, nil
	}
	if err != nil {
		return Workspace{}, false, fmt.Errorf("runtime: inspect workspace: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return Workspace{}, false, errors.New("runtime: workspace path is not a private directory")
	}
	return Workspace{ID: name, Path: workspacePath, redirected: owner != runID}, true, nil
}

// workspaceRunOwner resolves the durable run behind a workspace request once,
// for both the session selection and the child-to-root redirect. A nil run with
// no error means the row does not exist yet: the runtime allocates a child
// workspace before CreateRun persists its owner, and the context carries the
// session identity across that short gap.
func (m *WorkspaceManager) workspaceRunOwner(ctx context.Context, runID domain.RunID) (*domain.Run, domain.SessionID, error) {
	sessionID := contextSessionID(ctx)
	if m == nil || m.runs == nil {
		return nil, sessionID, nil
	}
	run, err := m.runs.GetRun(ctx, runID)
	switch {
	case err == nil:
		// A durable run is authoritative. The context is only a pre-persist
		// bridge and may be stale when a caller reuses it for inspection.
		return &run, run.SessionID, nil
	case errors.Is(err, storage.ErrNotFound):
		return nil, sessionID, nil
	default:
		return nil, "", fmt.Errorf("runtime: resolve workspace run owner: %w", err)
	}
}

// workspaceOwnerRunID returns the run whose private directory a workspace
// request resolves to. Every descendant of a root run shares that root's
// directory, so a child can read and write what its parent produced. RootID
// rather than ParentID keeps a whole bounded descendant tree in one directory
// instead of chaining a fresh one per level.
func workspaceOwnerRunID(ctx context.Context, runID domain.RunID, run *domain.Run) domain.RunID {
	if run != nil {
		if run.Kind == domain.RunKindChild && run.RootID != "" {
			return run.RootID
		}
		return runID
	}
	if owner := contextWorkspaceOwnerRun(ctx); owner != "" {
		return owner
	}
	return runID
}

func (m *WorkspaceManager) selectedWorkspace(ctx context.Context, sessionID domain.SessionID) (Workspace, bool, error) {
	if m == nil || m.sessions == nil {
		return Workspace{}, false, nil
	}
	if sessionID == "" {
		return Workspace{}, false, fmt.Errorf("runtime: resolve workspace run owner: %w", storage.ErrNotFound)
	}
	session, err := m.sessions.GetSession(ctx, sessionID)
	if err != nil {
		return Workspace{}, false, fmt.Errorf("runtime: resolve session workspace: %w", err)
	}
	root := strings.TrimSpace(session.WorkspacePath)
	if root == "" {
		return Workspace{}, false, nil
	}
	info, err := os.Lstat(root)
	if err != nil {
		return Workspace{}, false, fmt.Errorf("runtime: inspect selected workspace: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return Workspace{}, false, errors.New("runtime: selected workspace root is not a directory")
	}
	canonical, err := filepath.EvalSymlinks(root)
	if err != nil {
		return Workspace{}, false, fmt.Errorf("runtime: canonicalize selected workspace: %w", err)
	}
	// The durable session owns this selection and its root is locked after
	// the first Run. Keep the canonical filesystem authority private in Path;
	// public workspace identity does not need to encode the user's host path.
	return Workspace{ID: "selected:" + string(sessionID), Path: filepath.Clean(canonical)}, true, nil
}

// AdmissionWorkspaceAllocator is the workspace seam atomic task admission
// needs: allocation reports whether it created a fresh private directory so a
// failed admission can roll that directory back without ever touching a
// selected or local user directory.
type AdmissionWorkspaceAllocator interface {
	// EnsureForAdmission resolves the run workspace exactly like Ensure but
	// reports true only for a newly created, root-validated, owner-named,
	// non-symlink private directory this admission may reclaim. Selected, local
	// and redirected (child-to-root) workspaces return false; an
	// already-existing private directory also returns false so an uncertain
	// earlier attempt is preserved, never reaped.
	EnsureForAdmission(context.Context, domain.RunID) (Workspace, bool, error)
	// DiscardNewPrivateAdmission removes the requesting run's own private
	// directory only when it is an empty directory under the manager root. It is
	// a no-op for local, selected and redirected workspaces. Callers invoke it
	// only after a receipt check confirms the admission did not commit.
	DiscardNewPrivateAdmission(context.Context, domain.RunID) error
}

var _ AdmissionWorkspaceAllocator = (*WorkspaceManager)(nil)

// EnsureForAdmission implements AdmissionWorkspaceAllocator.
func (m *WorkspaceManager) EnsureForAdmission(ctx context.Context, runID domain.RunID) (Workspace, bool, error) {
	if m == nil || m.root == "" {
		return Workspace{}, false, errors.New("runtime: workspace manager not wired")
	}
	if err := ctx.Err(); err != nil {
		return Workspace{}, false, err
	}
	run, sessionID, err := m.workspaceRunOwner(ctx, runID)
	if err != nil {
		return Workspace{}, false, err
	}
	if selected, ok, err := m.selectedWorkspace(ctx, sessionID); err != nil {
		return Workspace{}, false, err
	} else if ok {
		return selected, false, nil
	}
	if m.local {
		info, err := os.Lstat(m.root)
		if err != nil {
			return Workspace{}, false, fmt.Errorf("runtime: inspect local workspace: %w", err)
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return Workspace{}, false, errors.New("runtime: local workspace root is not a directory")
		}
		return Workspace{ID: "local", Path: m.root}, false, nil
	}
	owner := workspaceOwnerRunID(ctx, runID, run)
	name := string(owner)
	if !validWorkspaceName(name) {
		return Workspace{}, false, errors.New("runtime: invalid workspace run id")
	}
	// A redirected workspace is the root run's directory, so it is never this
	// admission's to reclaim — not even when the crash window left this call
	// creating it.
	redirected := owner != runID
	workspace := Workspace{ID: name, Path: filepath.Join(m.root, name), redirected: redirected}
	if err := m.ensureUnderRoot(workspace.Path); err != nil {
		return Workspace{}, false, err
	}
	if err := os.MkdirAll(m.root, 0o700); err != nil {
		return Workspace{}, false, fmt.Errorf("runtime: create workspace root: %w", err)
	}
	if info, err := os.Lstat(workspace.Path); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return Workspace{}, false, errors.New("runtime: workspace path is not a private directory")
		}
		// Pre-existing run directory: it may belong to an earlier attempt
		// whose commit outcome is still uncertain, so it is preserved.
		return workspace, false, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return Workspace{}, false, fmt.Errorf("runtime: inspect workspace: %w", err)
	}
	if err := os.Mkdir(workspace.Path, 0o700); err != nil {
		if errors.Is(err, os.ErrExist) {
			return workspace, false, nil
		}
		return Workspace{}, false, fmt.Errorf("runtime: create workspace: %w", err)
	}
	return workspace, !redirected, nil
}

// DiscardNewPrivateAdmission implements AdmissionWorkspaceAllocator.
func (m *WorkspaceManager) DiscardNewPrivateAdmission(ctx context.Context, runID domain.RunID) error {
	if m == nil || m.root == "" {
		return errors.New("runtime: workspace manager not wired")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	// Local and selected workspaces are user directories; nothing was
	// privately allocated, so nothing may be removed.
	if m.local {
		return nil
	}
	run, sessionID, err := m.workspaceRunOwner(ctx, runID)
	if err != nil {
		return err
	}
	if workspaceOwnerRunID(ctx, runID, run) != runID {
		// A child resolves to its root run's directory. That directory is
		// shared, so the child's admission rollback must never remove it.
		return nil
	}
	if selected, ok, err := m.selectedWorkspace(ctx, sessionID); err != nil {
		return err
	} else if ok {
		_ = selected
		return nil
	}
	name := string(runID)
	if !validWorkspaceName(name) {
		return errors.New("runtime: invalid workspace run id")
	}
	path := filepath.Join(m.root, name)
	if err := m.ensureUnderRoot(path); err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("runtime: inspect workspace for discard: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return errors.New("runtime: refusing to discard non-private workspace path")
	}
	// os.Remove refuses non-empty directories, so a workspace that received
	// content — for example an uncertain commit the engine then used — is
	// preserved rather than destroyed.
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("runtime: discard admission workspace: %w", err)
	}
	return nil
}

// ValidateRunPath checks a path against the root selected for this run rather
// than the process's default root.
func (m *WorkspaceManager) ValidateRunPath(ctx context.Context, runID domain.RunID, path string) error {
	workspace, err := m.Ensure(ctx, runID)
	if err != nil {
		return err
	}
	return ensurePathUnderRoot(workspace.Path, path)
}

func ensurePathUnderRoot(root, path string) error {
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return fmt.Errorf("runtime: resolve workspace root: %w", err)
	}
	realPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("runtime: resolve workspace path: %w", err)
		}
		realParent, parentErr := filepath.EvalSymlinks(filepath.Dir(path))
		if parentErr != nil {
			return fmt.Errorf("runtime: resolve workspace parent: %w", parentErr)
		}
		realPath = filepath.Join(realParent, filepath.Base(path))
	}
	rel, err := filepath.Rel(realRoot, realPath)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return errors.New("runtime: workspace path escapes selected root")
	}
	return nil
}

// ValidatePath reports whether a path stays inside the manager root. It is
// used by future tool adapters before opening any user-controlled path.
func (m *WorkspaceManager) ValidatePath(path string) error {
	if m == nil || m.root == "" {
		return errors.New("runtime: workspace manager not wired")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("runtime: resolve workspace path: %w", err)
	}
	return m.ensureUnderRoot(abs)
}

func (m *WorkspaceManager) ensureUnderRoot(path string) error {
	rel, err := filepath.Rel(m.root, filepath.Clean(path))
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return errors.New("runtime: workspace path escapes isolation root")
	}
	return nil
}

func validWorkspaceName(name string) bool {
	if name == "" || filepath.Base(name) != name || strings.Contains(name, "..") {
		return false
	}
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			continue
		}
		return false
	}
	return true
}

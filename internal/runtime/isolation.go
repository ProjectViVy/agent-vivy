package runtime

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"agent-vivy/internal/domain"
)

// Workspace is the only filesystem identity a background run may receive.
// The path is deterministic per run so restart recovery can reattach without
// persisting an absolute host path in the Journal.
type Workspace struct {
	ID   string
	Path string
}

// WorkspaceManager allocates private directory sandboxes under one validated
// root. It intentionally does not execute git or shell commands; a future
// filesystem/subagent tool must be wired to this boundary explicitly.
type WorkspaceManager struct {
	root  string
	local bool
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
// run ID cannot redirect writes outside the root.
func (m *WorkspaceManager) Ensure(ctx context.Context, runID domain.RunID) (Workspace, error) {
	if m == nil || m.root == "" {
		return Workspace{}, errors.New("runtime: workspace manager not wired")
	}
	if err := ctx.Err(); err != nil {
		return Workspace{}, err
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
	name := string(runID)
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
	return Workspace{ID: name, Path: path}, nil
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
	name := string(runID)
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
	return Workspace{ID: name, Path: workspacePath}, true, nil
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

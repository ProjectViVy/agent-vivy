package runtime

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"agent-vivy/internal/domain"
)

// workspaceListCap bounds one workspace/list reply; bigger trees report
// truncated instead of streaming an unbounded listing to the UI.
const workspaceListCap = 2000

// WorkspaceFileInfo is one file in a run workspace, path relative to the
// workspace root with forward slashes.
type WorkspaceFileInfo struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
}

// WorkspaceFiles exposes the run workspace to the control plane's UI
// preview (workspace/list, workspace/read). It is read-only, symlink-free,
// and bounded by the same byte cap as the filesystem tools.
type WorkspaceFiles struct {
	manager      *WorkspaceManager
	maxFileBytes int
}

// NewWorkspaceFiles wires the UI-facing read-only workspace accessor.
func NewWorkspaceFiles(manager *WorkspaceManager, maxFileBytes int) *WorkspaceFiles {
	if maxFileBytes <= 0 {
		maxFileBytes = defaultFilesystemMaxBytes
	}
	return &WorkspaceFiles{manager: manager, maxFileBytes: maxFileBytes}
}

// List returns every regular file under the run workspace, sorted by path.
// Symlinks are skipped (the workspace must stay symlink-free) and a listing
// past the cap reports truncated.
func (s *WorkspaceFiles) List(ctx context.Context, runID domain.RunID) ([]WorkspaceFileInfo, bool, error) {
	root, err := s.workspace(ctx, runID)
	if err != nil {
		return nil, false, err
	}
	files := make([]WorkspaceFileInfo, 0, 16)
	truncated := false
	err = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type()&fs.ModeSymlink != 0 {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if len(files) >= workspaceListCap {
			truncated = true
			return fs.SkipAll
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		files = append(files, WorkspaceFileInfo{Path: filepath.ToSlash(rel), Size: info.Size()})
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files, truncated, nil
}

// ReadFileResult is one workspace/read reply body.
type ReadFileResult struct {
	Path      string
	Content   string
	Size      int64
	Truncated bool
	Binary    bool
}

// Read returns one workspace file's content. Text over the byte cap is
// truncated (flagged); binaries are flagged and returned without content —
// the UI preview is text-only.
func (s *WorkspaceFiles) Read(ctx context.Context, runID domain.RunID, rel string) (ReadFileResult, error) {
	root, err := s.workspace(ctx, runID)
	if err != nil {
		return ReadFileResult{}, err
	}
	clean, err := workspaceRelPath(rel)
	if err != nil {
		return ReadFileResult{}, err
	}
	full := filepath.Join(root, filepath.FromSlash(clean))
	if err := s.manager.ValidatePath(full); err != nil {
		return ReadFileResult{}, err
	}
	info, err := os.Lstat(full)
	if err != nil {
		return ReadFileResult{}, err
	}
	if info.Mode()&fs.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return ReadFileResult{}, errors.New("runtime: workspace file is not a regular file")
	}
	data, err := os.ReadFile(full)
	if err != nil {
		return ReadFileResult{}, err
	}
	result := ReadFileResult{Path: clean, Size: info.Size()}
	if isBinary(data) {
		result.Binary = true
		return result, nil
	}
	if len(data) > s.maxFileBytes {
		data = data[:s.maxFileBytes]
		result.Truncated = true
	}
	result.Content = string(data)
	return result, nil
}

func (s *WorkspaceFiles) workspace(ctx context.Context, runID domain.RunID) (string, error) {
	if s == nil || s.manager == nil {
		return "", errors.New("runtime: workspace files not wired")
	}
	ws, err := s.manager.Ensure(ctx, runID)
	if err != nil {
		return "", err
	}
	return ws.Path, nil
}

// workspaceRelPath validates a UI-supplied workspace-relative path: no
// absolute forms, no drive letters, no escapes, forward slashes only.
func workspaceRelPath(rel string) (string, error) {
	if rel == "" || strings.Contains(rel, "\\") || strings.Contains(rel, ":") || strings.HasPrefix(rel, "/") {
		return "", errors.New("runtime: invalid workspace path")
	}
	clean := path.Clean(rel)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", errors.New("runtime: workspace path escapes the run workspace")
	}
	return clean, nil
}

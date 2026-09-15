package rpc

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

const maxWorkspaceBrowseDirectories = 512
const maxWorkspaceBrowseScannedEntries = 4096

type workspaceDirectoryResult struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

type workspaceBrowseResult struct {
	Path        string                     `json:"path"`
	Parent      string                     `json:"parent,omitempty"`
	Roots       []string                   `json:"roots"`
	Directories []workspaceDirectoryResult `json:"directories"`
	Truncated   bool                       `json:"truncated"`
}

// canonicalWorkspacePath applies the same selected-root contract as
// vivy-code: the directory must already exist, the selected entry itself may
// not be a symlink, and the stored value is a canonical absolute path.
func canonicalWorkspacePath(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	if !filepath.IsAbs(value) {
		return "", errors.New("workspace path must be absolute")
	}
	abs, err := filepath.Abs(value)
	if err != nil {
		return "", errors.New("workspace path cannot be resolved")
	}
	info, err := os.Lstat(abs)
	if err != nil {
		return "", errors.New("workspace directory does not exist")
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return "", errors.New("workspace path is not a directory")
	}
	canonical, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", errors.New("workspace directory cannot be resolved")
	}
	return filepath.Clean(canonical), nil
}

func workspaceRoots() []string {
	if runtime.GOOS != "windows" {
		return []string{string(filepath.Separator)}
	}
	roots := make([]string, 0, 4)
	for drive := 'A'; drive <= 'Z'; drive++ {
		root := string(drive) + `:\`
		if info, err := os.Stat(root); err == nil && info.IsDir() {
			roots = append(roots, root)
		}
	}
	return roots
}

func browseWorkspaceDirectories(value string) (workspaceBrowseResult, error) {
	if strings.TrimSpace(value) == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return workspaceBrowseResult{}, errors.New("user home directory is unavailable")
		}
		value = home
	}
	current, err := canonicalWorkspacePath(value)
	if err != nil {
		return workspaceBrowseResult{}, err
	}
	directory, err := os.Open(current)
	if err != nil {
		return workspaceBrowseResult{}, errors.New("workspace directory cannot be opened")
	}
	defer func() { _ = directory.Close() }()
	directories := make([]workspaceDirectoryResult, 0, maxWorkspaceBrowseDirectories)
	truncated := false
	scanned := 0
	for scanned < maxWorkspaceBrowseScannedEntries && len(directories) < maxWorkspaceBrowseDirectories {
		batchSize := min(128, maxWorkspaceBrowseScannedEntries-scanned)
		entries, readErr := directory.ReadDir(batchSize)
		for _, entry := range entries {
			scanned++
			if !entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
				continue
			}
			directories = append(directories, workspaceDirectoryResult{
				Name: entry.Name(), Path: filepath.Join(current, entry.Name()),
			})
			if len(directories) == maxWorkspaceBrowseDirectories {
				break
			}
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return workspaceBrowseResult{}, errors.New("workspace directory cannot be read")
		}
		if scanned >= maxWorkspaceBrowseScannedEntries || len(directories) >= maxWorkspaceBrowseDirectories {
			truncated = true
			break
		}
	}
	sort.Slice(directories, func(i, j int) bool {
		left, right := strings.ToLower(directories[i].Name), strings.ToLower(directories[j].Name)
		if left == right {
			return directories[i].Name < directories[j].Name
		}
		return left < right
	})
	parent := filepath.Dir(current)
	if filepath.Clean(parent) == filepath.Clean(current) {
		parent = ""
	}
	return workspaceBrowseResult{
		Path: current, Parent: parent, Roots: workspaceRoots(), Directories: directories, Truncated: truncated,
	}, nil
}

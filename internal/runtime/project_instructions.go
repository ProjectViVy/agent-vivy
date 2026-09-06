package runtime

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// ProjectInstructions is the launch-directory scan result fed into the
// existing Eino agentsmd / skill middlewares. Eino does not discover cwd
// itself; this adapter finds AGENTS.md and conventional skill packages.
type ProjectInstructions struct {
	// Root is the backend root for AGENTS.md reads: the git worktree root
	// when one exists, otherwise the launch directory. All AgentsMDFiles
	// are relative to Root.
	Root string
	// AgentsMDFiles is the ordered list passed to agentsmd.Config.
	// Git-root files come first so crate-local files sit closer to the
	// user turn. Always contains at least "AGENTS.md" so a missing file
	// stays a non-fatal Eino warning.
	AgentsMDFiles []string
	// SkillRoots are existing first-level skill directories in closer-first
	// order (cwd, then git root). Read-only overlays on EinoSkillBackend.
	SkillRoots []string
}

var projectSkillSubdirs = []string{
	filepath.Join(".agents", "skills"),
	filepath.Join(".vivy", "skills"),
}

// CanonicalInstructionRoot validates and canonicalizes a launch directory
// used as the project instruction scan root. The root itself must be a
// real directory, not a symlink; parent junctions are resolved so later
// containment checks compare physical paths.
func CanonicalInstructionRoot(dir string) (string, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return "", errors.New("runtime: instruction root must not be empty")
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("runtime: resolve instruction root: %w", err)
	}
	info, err := os.Lstat(abs)
	if err != nil {
		return "", fmt.Errorf("runtime: inspect instruction root: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return "", errors.New("runtime: instruction root is not a directory")
	}
	abs, err = filepath.EvalSymlinks(abs)
	if err != nil {
		return "", fmt.Errorf("runtime: canonicalize instruction root: %w", err)
	}
	abs, err = filepath.Abs(abs)
	if err != nil {
		return "", fmt.Errorf("runtime: resolve canonical instruction root: %w", err)
	}
	return filepath.Clean(abs), nil
}

// DiscoverProjectInstructions walks from launchDir up to the git root
// (when present) collecting AGENTS.md and conventional skill directories.
// launchDir should already be CanonicalInstructionRoot output.
func DiscoverProjectInstructions(launchDir string) (ProjectInstructions, error) {
	root, err := CanonicalInstructionRoot(launchDir)
	if err != nil {
		return ProjectInstructions{}, err
	}
	gitRoot := findGitRoot(root)
	backendRoot := root
	if gitRoot != "" {
		backendRoot = gitRoot
	}
	files, err := collectAgentsMDFiles(root, backendRoot)
	if err != nil {
		return ProjectInstructions{}, err
	}
	return ProjectInstructions{
		Root:          backendRoot,
		AgentsMDFiles: files,
		SkillRoots:    collectProjectSkillRoots(root, gitRoot),
	}, nil
}

func findGitRoot(dir string) string {
	cur := dir
	for {
		info, err := os.Lstat(filepath.Join(cur, ".git"))
		if err == nil && info.Mode()&os.ModeSymlink == 0 && (info.IsDir() || info.Mode().IsRegular()) {
			return cur
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return ""
		}
		cur = parent
	}
}

func collectAgentsMDFiles(launchDir, backendRoot string) ([]string, error) {
	chain, err := ancestorChain(launchDir, backendRoot)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]struct{}, len(chain))
	files := make([]string, 0, len(chain))
	// Git root first (chain is launch → … → backend root).
	for i := len(chain) - 1; i >= 0; i-- {
		rel, err := filepath.Rel(backendRoot, chain[i])
		if err != nil {
			return nil, fmt.Errorf("runtime: agents.md relative path: %w", err)
		}
		name := AgentsMDFileName
		if rel != "." {
			name = filepath.ToSlash(filepath.Join(rel, AgentsMDFileName))
		}
		if _, ok := seen[strings.ToLower(name)]; ok {
			continue
		}
		path := filepath.Join(chain[i], AgentsMDFileName)
		if !regularFile(path) {
			continue
		}
		seen[strings.ToLower(name)] = struct{}{}
		files = append(files, name)
	}
	if len(files) == 0 {
		return []string{AgentsMDFileName}, nil
	}
	return files, nil
}

func ancestorChain(from, stop string) ([]string, error) {
	var chain []string
	cur := from
	for {
		chain = append(chain, cur)
		if samePath(cur, stop) {
			return chain, nil
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return nil, errors.New("runtime: instruction root is not inside the scan backend root")
		}
		cur = parent
	}
}

func collectProjectSkillRoots(launchDir, gitRoot string) []string {
	var roots []string
	seen := make(map[string]struct{})
	add := func(base string) {
		if base == "" {
			return
		}
		for _, sub := range projectSkillSubdirs {
			path := filepath.Join(base, sub)
			key := strings.ToLower(filepath.Clean(path))
			if _, ok := seen[key]; ok {
				continue
			}
			if !realDirectory(path) {
				continue
			}
			seen[key] = struct{}{}
			roots = append(roots, path)
		}
	}
	add(launchDir)
	if gitRoot != "" && !samePath(gitRoot, launchDir) {
		add(gitRoot)
	}
	return roots
}

func regularFile(path string) bool {
	info, err := os.Lstat(path)
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeSymlink == 0 && info.Mode().IsRegular()
}

func realDirectory(path string) bool {
	info, err := os.Lstat(path)
	if err != nil {
		return false
	}
	return info.Mode()&fs.ModeSymlink == 0 && info.IsDir()
}

func samePath(a, b string) bool {
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}

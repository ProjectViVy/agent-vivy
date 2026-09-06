package runtime

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/cloudwego/eino/adk/middlewares/agentsmd"
)

// ProjectAgentsMDBackend reads AGENTS.md (and @imports) from a host
// instruction root. It is independent of the run workspace so sandbox
// compositions can still inject launch-directory instructions without
// mounting the host tree as the file-tool world.
type ProjectAgentsMDBackend struct {
	root string
}

var _ AgentsMDBackend = (*ProjectAgentsMDBackend)(nil)

// NewProjectAgentsMDBackend binds AGENTS.md reads to a canonical host root.
func NewProjectAgentsMDBackend(root string) (*ProjectAgentsMDBackend, error) {
	root, err := CanonicalInstructionRoot(root)
	if err != nil {
		return nil, err
	}
	return &ProjectAgentsMDBackend{root: root}, nil
}

// Read implements agentsmd.Backend. Missing files return os.ErrNotExist so
// Eino treats them as non-fatal warnings. Other errors abort the load.
func (b *ProjectAgentsMDBackend) Read(ctx context.Context, req *agentsmd.ReadRequest) (*agentsmd.FileContent, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if b == nil || b.root == "" {
		return nil, errors.New("runtime: agentsmd instruction root is not wired")
	}
	if req == nil {
		return nil, errors.New("runtime: agentsmd read request is nil")
	}
	path, err := b.resolve(req.FilePath)
	if err != nil {
		return nil, err
	}
	info, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		return nil, fmt.Errorf("runtime: inspect %s: %w", req.FilePath, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, os.ErrNotExist
	}
	if info.Size() > int64(agentsMDMaxBytes) {
		return nil, fmt.Errorf("runtime: %s exceeds the %d byte agentsmd cap", req.FilePath, agentsMDMaxBytes)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		return nil, fmt.Errorf("runtime: read %s: %w", req.FilePath, err)
	}
	if !utf8.Valid(data) {
		return nil, fmt.Errorf("runtime: %s is not valid UTF-8", req.FilePath)
	}
	content := string(data)
	if req.Offset > 1 || req.Limit > 0 {
		content = sliceLines(content, req.Offset, req.Limit)
	}
	return &agentsmd.FileContent{Content: content}, nil
}

func (b *ProjectAgentsMDBackend) resolve(filePath string) (string, error) {
	filePath = strings.TrimSpace(filePath)
	if filePath == "" || strings.ContainsRune(filePath, '\x00') {
		return "", errors.New("runtime: agentsmd path is invalid")
	}
	if filepath.IsAbs(filePath) {
		clean := filepath.Clean(filePath)
		if err := containPath(b.root, clean); err != nil {
			return "", err
		}
		return clean, nil
	}
	clean := filepath.Clean(filepath.FromSlash(filePath))
	if clean == "." || filepath.IsAbs(clean) || filepath.VolumeName(clean) != "" {
		return "", errors.New("runtime: agentsmd path is invalid")
	}
	full := filepath.Join(b.root, clean)
	if err := containPath(b.root, full); err != nil {
		return "", err
	}
	return full, nil
}

func containPath(root, path string) error {
	rel, err := filepath.Rel(root, filepath.Clean(path))
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return errors.New("runtime: path escapes instruction root")
	}
	return nil
}

func sliceLines(text string, offset, limit int) string {
	lines := strings.Split(text, "\n")
	start := offset
	if start <= 0 {
		start = 1
	}
	if start > len(lines) {
		return ""
	}
	end := len(lines)
	if limit > 0 && start-1+limit < end {
		end = start - 1 + limit
	}
	return strings.Join(lines[start-1:end], "\n")
}

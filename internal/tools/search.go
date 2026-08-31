package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"agent-vivy/internal/domain"
)

const (
	GrepName = "grep"
	GlobName = "glob"
)

// GrepRequest is one content search over the run workspace.
type GrepRequest struct {
	Pattern string // Go/rg regular expression
	Path    string // workspace-relative root; empty means workspace root
	Include string // optional glob filter on file names, e.g. `*.go`
}

// GrepMatch is one matching line.
type GrepMatch struct {
	Path    string `json:"path"`
	Line    int    `json:"line"`
	Content string `json:"content"`
}

// GrepResult carries the bounded match list.
type GrepResult struct {
	Matches   []GrepMatch `json:"matches"`
	Truncated bool        `json:"truncated,omitempty"`
}

// GlobRequest is one `**`-capable file pattern match.
type GlobRequest struct {
	Pattern string // doublestar pattern, e.g. `src/**/*.go`
	Path    string // workspace-relative base; empty means workspace root
}

// GlobFileInfo describes one matched file.
type GlobFileInfo struct {
	Path       string `json:"path"`
	Size       int64  `json:"size"`
	ModifiedAt string `json:"modified_at"`
}

// GlobResult carries the newest-first match list.
type GlobResult struct {
	Files     []GlobFileInfo `json:"files"`
	Truncated bool           `json:"truncated,omitempty"`
}

// GrepOperations is the search seam behind the grep/glob tools. The
// filesystem backend implements it: ripgrep subprocess when available
// (native .gitignore awareness), pure-Go walk otherwise.
type GrepOperations interface {
	Grep(ctx context.Context, req GrepRequest) (GrepResult, error)
	Glob(ctx context.Context, req GlobRequest) (GlobResult, error)
}

type grepTool struct{ ops GrepOperations }

func NewGrep(ops GrepOperations) Tool { return &grepTool{ops: ops} }

func (t *grepTool) Spec() domain.ToolSpec {
	return domain.ToolSpec{
		Name: GrepName,
		Description: "Searches workspace file contents with a regular expression. " +
			"Hidden files are skipped; .gitignore is respected when ripgrep is available on the host. " +
			"Output is bounded; prefer narrow patterns and the include filter.",
		Readonly: true,
		Keywords: []string{"grep", "search", "regex", "content", "code", "ripgrep"},
		Params: map[string]domain.ToolParam{
			"pattern": {Desc: "Regular expression, e.g. `func \\(\\w+ \\*Service\\)`.", Required: true},
			"path":    {Desc: "Optional workspace-relative directory or file to search.", Required: false},
			"include": {Desc: "Optional glob filter on file names, e.g. `*.go`.", Required: false},
		},
	}
}

func (t *grepTool) InvokableRun(ctx context.Context, args json.RawMessage) (string, error) {
	req, err := decodeGrepRequest(args)
	if err != nil {
		return "", err
	}
	if t.ops == nil {
		return "", fmt.Errorf("tools: %s backend not wired", GrepName)
	}
	result, err := t.ops.Grep(ctx, req)
	if err != nil {
		return "", err
	}
	return marshalToolResult(result)
}

func (t *grepTool) PrepareProposal(ctx context.Context, args json.RawMessage) (domain.ToolProposal, error) {
	req, err := decodeGrepRequest(args)
	if err != nil {
		return domain.ToolProposal{}, err
	}
	return domain.ToolProposal{Action: GrepName, Target: req.Path, Preview: fmt.Sprintf("grep %q in %s", req.Pattern, orDot(req.Path))}, nil
}

type globTool struct{ ops GrepOperations }

func NewGlob(ops GrepOperations) Tool { return &globTool{ops: ops} }

func (t *globTool) Spec() domain.ToolSpec {
	return domain.ToolSpec{
		Name: GlobName,
		Description: "Matches workspace file paths against a glob pattern with `**` recursion support, " +
			"newest first. Use it to find files by name before reading them.",
		Readonly: true,
		Keywords: []string{"glob", "find", "files", "pattern", "wildcard"},
		Params: map[string]domain.ToolParam{
			"pattern": {Desc: "Glob pattern with `**` recursion, e.g. `internal/**/*.go`.", Required: true},
			"path":    {Desc: "Optional workspace-relative base directory.", Required: false},
		},
	}
}

func (t *globTool) InvokableRun(ctx context.Context, args json.RawMessage) (string, error) {
	req, err := decodeGlobRequest(args)
	if err != nil {
		return "", err
	}
	if t.ops == nil {
		return "", fmt.Errorf("tools: %s backend not wired", GlobName)
	}
	result, err := t.ops.Glob(ctx, req)
	if err != nil {
		return "", err
	}
	return marshalToolResult(result)
}

func (t *globTool) PrepareProposal(ctx context.Context, args json.RawMessage) (domain.ToolProposal, error) {
	req, err := decodeGlobRequest(args)
	if err != nil {
		return domain.ToolProposal{}, err
	}
	return domain.ToolProposal{Action: GlobName, Target: req.Path, Preview: fmt.Sprintf("glob %q in %s", req.Pattern, orDot(req.Path))}, nil
}

func decodeGrepRequest(args json.RawMessage) (GrepRequest, error) {
	var req GrepRequest
	if err := decodeStringArgs(args, &req); err != nil {
		return GrepRequest{}, err
	}
	req.Pattern = strings.TrimSpace(req.Pattern)
	if req.Pattern == "" {
		return GrepRequest{}, &ArgError{Field: "pattern", Reason: "is required"}
	}
	req.Path = strings.TrimSpace(req.Path)
	req.Include = strings.TrimSpace(req.Include)
	return req, nil
}

func decodeGlobRequest(args json.RawMessage) (GlobRequest, error) {
	var req GlobRequest
	if err := decodeStringArgs(args, &req); err != nil {
		return GlobRequest{}, err
	}
	req.Pattern = strings.TrimSpace(req.Pattern)
	if req.Pattern == "" {
		return GlobRequest{}, &ArgError{Field: "pattern", Reason: "is required"}
	}
	req.Path = strings.TrimSpace(req.Path)
	return req, nil
}

func orDot(path string) string {
	if path == "" {
		return "."
	}
	return path
}

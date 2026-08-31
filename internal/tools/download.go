package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"agent-vivy/internal/domain"
)

const DownloadName = "download"

// DownloadRequest pins the target inside the current run workspace; Path is
// workspace-relative and never absolute. An empty TimeoutSeconds lets the
// backend pick its default.
type DownloadRequest struct {
	URL            string
	Path           string
	TimeoutSeconds int
}

// DownloadResult reports a finished download. The body never enters the
// conversation, so unlike web_fetch the result is not marked untrusted.
type DownloadResult struct {
	Path        string `json:"path"`
	Bytes       int64  `json:"bytes"`
	SHA256      string `json:"sha256"`
	ContentType string `json:"content_type,omitempty"`
	Overwritten bool   `json:"overwritten"`
}

type DownloadOperations interface {
	Download(context.Context, domain.RunID, DownloadRequest) (DownloadResult, error)
}

type downloadProposalOperations interface {
	PrepareDownload(context.Context, domain.RunID, DownloadRequest) (domain.ToolProposal, error)
}

type downloadTool struct{ ops DownloadOperations }

func NewDownload(ops DownloadOperations) Tool { return &downloadTool{ops: ops} }

func (t *downloadTool) Spec() domain.ToolSpec {
	return domain.ToolSpec{
		Name: DownloadName,
		Description: "Downloads a URL to a file in the current run workspace (binary-safe, streaming); " +
			"runs after approval and overwrites an existing file without warning. For reading content into the conversation use web_fetch.",
		Readonly: false,
		Keywords: []string{"download", "save", "file", "url", "fetch", "binary"},
		Params: map[string]domain.ToolParam{
			"url":     {Desc: "Absolute public HTTP(S) URL.", Required: true},
			"path":    {Desc: "Workspace-relative destination path; parents are created.", Required: true},
			"timeout": {Desc: "Optional timeout in seconds (5-600).", Type: "integer"},
		},
	}
}

func (t *downloadTool) InvokableRun(ctx context.Context, args json.RawMessage) (string, error) {
	request, err := decodeDownloadRequest(args)
	if err != nil {
		return "", err
	}
	if t.ops == nil {
		return "", fmt.Errorf("tools: download backend not wired")
	}
	result, err := t.ops.Download(ctx, RunIDFromContext(ctx), request)
	if err != nil {
		return "", err
	}
	return marshalToolResult(result)
}

// PrepareProposal mirrors the write_file HITL flow: describe the mutation for
// human review without touching the network or the target.
func (t *downloadTool) PrepareProposal(ctx context.Context, args json.RawMessage) (domain.ToolProposal, error) {
	request, err := decodeDownloadRequest(args)
	if err != nil {
		return domain.ToolProposal{}, err
	}
	if planner, ok := t.ops.(downloadProposalOperations); ok {
		return planner.PrepareDownload(ctx, RunIDFromContext(ctx), request)
	}
	payload, _ := json.Marshal(request)
	return domain.ToolProposal{
		Action: DownloadName, Target: request.Path,
		Preview:      fmt.Sprintf("download %s to %s", request.URL, request.Path),
		RiskFindings: []string{"writes remote content into the workspace"},
		Data:         payload,
	}, nil
}

func decodeDownloadRequest(args json.RawMessage) (DownloadRequest, error) {
	var input struct {
		URL     string `json:"url"`
		Path    string `json:"path"`
		Timeout int    `json:"timeout"`
	}
	if err := json.Unmarshal(args, &input); err != nil {
		return DownloadRequest{}, fmt.Errorf("download: invalid arguments: %w", err)
	}
	if strings.TrimSpace(input.URL) == "" {
		return DownloadRequest{}, &ArgError{Field: "url", Reason: "is required"}
	}
	if strings.TrimSpace(input.Path) == "" {
		return DownloadRequest{}, &ArgError{Field: "path", Reason: "is required"}
	}
	if input.Timeout < 0 {
		return DownloadRequest{}, &ArgError{Field: "timeout", Reason: "must not be negative"}
	}
	return DownloadRequest{URL: input.URL, Path: input.Path, TimeoutSeconds: input.Timeout}, nil
}

package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"agent-vivy/internal/domain"
)

const WebFetchName = "web_fetch"

// WebFetchRequest is the Vivy-owned fetch contract. Format mirrors the CRUSH
// fetch tool (markdown/text/html); an empty format means markdown. An empty
// TimeoutSeconds lets the backend pick its default.
type WebFetchRequest struct {
	URL            string
	Format         string
	TimeoutSeconds int
}

// WebFetchResult carries remote text into the conversation. Content is always
// derived data (converted or truncated); the runtime adapter folds it behind
// the untrusted-output header regardless of the Untrusted field.
type WebFetchResult struct {
	URL         string `json:"url"`
	StatusCode  int    `json:"status_code"`
	ContentType string `json:"content_type,omitempty"`
	Format      string `json:"format"`
	Content     string `json:"content"`
	Truncated   bool   `json:"truncated"`
	Bytes       int    `json:"bytes"`
	Untrusted   bool   `json:"untrusted"`
}

type WebFetchOperations interface {
	Fetch(context.Context, domain.RunID, WebFetchRequest) (WebFetchResult, error)
}

type webFetchTool struct{ ops WebFetchOperations }

func NewWebFetch(ops WebFetchOperations) Tool { return &webFetchTool{ops: ops} }

func (t *webFetchTool) Spec() domain.ToolSpec {
	return domain.ToolSpec{
		Name: WebFetchName,
		Description: "Fetches a public web page or API response as markdown, plain text, or html into the conversation; no AI processing. " +
			"Credential-bearing URLs are denied; binary content must use download instead. Large output is truncated.",
		Readonly: true,
		Keywords: []string{"web", "fetch", "url", "page", "read", "html", "markdown", "network"},
		Params: map[string]domain.ToolParam{
			"url":     {Desc: "Absolute public HTTP(S) URL.", Required: true},
			"format":  {Desc: "Optional output format; omitted means markdown.", Enum: []string{"markdown", "text", "html"}},
			"timeout": {Desc: "Optional timeout in seconds (5-120).", Type: "integer"},
		},
	}
}

func (t *webFetchTool) InvokableRun(ctx context.Context, args json.RawMessage) (string, error) {
	var input struct {
		URL     string `json:"url"`
		Format  string `json:"format"`
		Timeout int    `json:"timeout"`
	}
	if err := json.Unmarshal(args, &input); err != nil {
		return "", fmt.Errorf("web_fetch: invalid arguments: %w", err)
	}
	if strings.TrimSpace(input.URL) == "" {
		return "", &ArgError{Field: "url", Reason: "is required"}
	}
	switch input.Format {
	case "", "markdown", "text", "html":
	default:
		return "", &ArgError{Field: "format", Reason: "must be markdown, text, or html"}
	}
	if input.Timeout < 0 {
		return "", &ArgError{Field: "timeout", Reason: "must not be negative"}
	}
	if t.ops == nil {
		return "", fmt.Errorf("tools: web fetch backend not wired")
	}
	result, err := t.ops.Fetch(ctx, RunIDFromContext(ctx), WebFetchRequest{URL: input.URL, Format: input.Format, TimeoutSeconds: input.Timeout})
	if err != nil {
		return "", err
	}
	return marshalToolResult(result)
}

package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"agent-vivy/internal/domain"
)

const HTTPRequestName = "http_request"

type HTTPRequest struct {
	Method  string
	URL     string
	Headers map[string]string
}

type HTTPResponse struct {
	StatusCode  int               `json:"status_code"`
	URL         string            `json:"url"`
	Headers     map[string]string `json:"headers,omitempty"`
	Body        string            `json:"body"`
	ContentType string            `json:"content_type,omitempty"`
	Untrusted   bool              `json:"untrusted"`
}

type HTTPOperations interface {
	Request(context.Context, domain.RunID, HTTPRequest) (HTTPResponse, error)
}

type httpRequestTool struct{ ops HTTPOperations }

func NewHTTPRequest(ops HTTPOperations) Tool { return &httpRequestTool{ops: ops} }

func (t *httpRequestTool) Spec() domain.ToolSpec {
	return domain.ToolSpec{
		Name:        HTTPRequestName,
		Description: "Fetches untrusted text from an allowlisted HTTP host using GET or HEAD.",
		Readonly:    true,
		Keywords:    []string{"http", "request", "fetch", "url", "network"},
		Params: map[string]domain.ToolParam{
			"method":  {Desc: "GET or HEAD; omitted means GET.", Enum: []string{"GET", "HEAD"}},
			"url":     {Desc: "Absolute HTTP(S) URL on a configured allowlist.", Required: true},
			"headers": {Desc: "Optional non-sensitive request headers.", Type: "object"},
		},
	}
}

func (t *httpRequestTool) InvokableRun(ctx context.Context, args json.RawMessage) (string, error) {
	var input struct {
		Method  string            `json:"method"`
		URL     string            `json:"url"`
		Headers map[string]string `json:"headers"`
	}
	if err := json.Unmarshal(args, &input); err != nil {
		return "", fmt.Errorf("http_request: invalid arguments: %w", err)
	}
	if strings.TrimSpace(input.URL) == "" {
		return "", &ArgError{Field: "url", Reason: "is required"}
	}
	if t.ops == nil {
		return "", fmt.Errorf("tools: http request backend not wired")
	}
	result, err := t.ops.Request(ctx, RunIDFromContext(ctx), HTTPRequest{Method: input.Method, URL: input.URL, Headers: input.Headers})
	if err != nil {
		return "", err
	}
	return marshalToolResult(result)
}

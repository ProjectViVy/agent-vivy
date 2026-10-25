package provider

// Provider wire recorder. The provider owns the HTTP contract, so this file is
// the only place that inspects the bytes actually sent. It exists to answer one
// question after a rejected call: what did the wire carry? The summary is
// deliberately content-free (roles, shapes, identifiers, top-level keys) — it
// is journaled, and D-010 forbids message text in event payloads.
//
// Capture never changes behavior: the request body is replayed byte-for-byte,
// capture errors are ignored, and nothing is recorded for a 2xx response.
// Outbound payload normalization (outbound.go) runs above this transport, so a
// summary always describes the bytes actually sent, not the pre-normalized
// ACL conversion.

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"sort"
	"strings"
)

const (
	// wireBodyLimit bounds how much of a request body is buffered for a
	// summary. Larger bodies are still sent in full; only the summary prefix
	// is inspected and the summary reports itself truncated.
	wireBodyLimit = 8 << 20
	// wireListLimit and wireStringLimit bound every recorded list and string.
	wireListLimit   = 64
	wireStringLimit = 128
)

// WireSummary is the content-free description of one rejected provider
// request. ContentForms holds one entry per message: "string", "array",
// "empty", or "other" for a form this recorder does not model.
type WireSummary struct {
	Status       int
	Model        string
	Stream       bool
	Messages     int
	Roles        []string
	ContentForms []string
	CallIDs      []string
	ResultIDs    []string
	ToolCount    int
	ToolNames    []string
	ToolChoice   string
	ExtraKeys    []string
	BodyBytes    int
	Truncated    bool
	ParseError   string
}

type wireSinkKey struct{}

// WithWireSink attaches the per-call summary sink. A nil sink is a no-op so
// callers never need a nil check at the call site.
func WithWireSink(ctx context.Context, sink func(WireSummary)) context.Context {
	if sink == nil {
		return ctx
	}
	return context.WithValue(ctx, wireSinkKey{}, sink)
}

func wireSinkFrom(ctx context.Context) func(WireSummary) {
	sink, _ := ctx.Value(wireSinkKey{}).(func(WireSummary))
	return sink
}

// wireHTTPClient is the HTTP client handed to the provider chat model. It is
// transport-equivalent to today's default client (Timeout stays unset) with a
// recording RoundTripper layered on http.DefaultTransport.
func wireHTTPClient() *http.Client {
	return &http.Client{Transport: &wireTransport{base: http.DefaultTransport}}
}

type wireTransport struct {
	base http.RoundTripper
}

type replayBody struct {
	io.Reader
	io.Closer
}

func (t *wireTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	sink := wireSinkFrom(req.Context())
	if sink == nil || req.Body == nil {
		return t.base.RoundTrip(req)
	}
	original := req.Body
	// Read one byte past the limit so truncation is detectable without
	// consuming the whole body.
	buffered, readErr := io.ReadAll(io.LimitReader(original, wireBodyLimit+1))
	// Replay every byte that was consumed, then the unread remainder. The
	// ContentLength stays untouched, so the provider sees the same body.
	req.Body = replayBody{Reader: io.MultiReader(bytes.NewReader(buffered), original), Closer: original}
	if readErr != nil {
		return t.base.RoundTrip(req)
	}
	truncated := len(buffered) > wireBodyLimit
	inspected := buffered
	if truncated {
		inspected = buffered[:wireBodyLimit]
	}
	resp, err := t.base.RoundTrip(req)
	if err == nil && resp != nil && (resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices) {
		summary := summarizeWireRequest(inspected, truncated)
		summary.Status = resp.StatusCode
		sink(summary)
	}
	return resp, err
}

type wireRequest struct {
	Model    string          `json:"model"`
	Stream   bool            `json:"stream"`
	Messages []wireMessage   `json:"messages"`
	Tools    []wireTool      `json:"tools"`
	Choice   json.RawMessage `json:"tool_choice"`
}

type wireMessage struct {
	Role       string          `json:"role"`
	Content    json.RawMessage `json:"content"`
	ToolCalls  []wireToolCall  `json:"tool_calls"`
	ToolCallID string          `json:"tool_call_id"`
}

type wireToolCall struct {
	ID string `json:"id"`
}

type wireTool struct {
	Function struct {
		Name string `json:"name"`
	} `json:"function"`
}

// wireCoreKeys are the top-level fields the summary reports explicitly.
// Everything else is listed in ExtraKeys so routing/sampling knobs the model
// does not understand are visible without dumping values.
var wireCoreKeys = map[string]struct{}{
	"model": {}, "messages": {}, "tools": {}, "tool_choice": {},
	"stream": {}, "stream_options": {},
}

func summarizeWireRequest(body []byte, truncated bool) WireSummary {
	summary := WireSummary{BodyBytes: len(body), Truncated: truncated}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		summary.ParseError = clampWireString(err.Error())
		return summary
	}
	for key := range fields {
		if _, core := wireCoreKeys[key]; !core {
			summary.ExtraKeys = append(summary.ExtraKeys, clampWireString(key))
		}
	}
	sort.Strings(summary.ExtraKeys)
	summary.ExtraKeys = clampWireList(summary.ExtraKeys)

	var request wireRequest
	if err := json.Unmarshal(body, &request); err != nil {
		summary.ParseError = clampWireString(err.Error())
		return summary
	}
	summary.Model = clampWireString(request.Model)
	summary.Stream = request.Stream
	summary.Messages = len(request.Messages)
	for _, message := range request.Messages {
		summary.Roles = append(summary.Roles, clampWireString(message.Role))
		summary.ContentForms = append(summary.ContentForms, contentForm(message.Content))
		for _, call := range message.ToolCalls {
			summary.CallIDs = append(summary.CallIDs, clampWireString(call.ID))
		}
		if message.ToolCallID != "" {
			summary.ResultIDs = append(summary.ResultIDs, clampWireString(message.ToolCallID))
		}
	}
	summary.Roles = clampWireList(summary.Roles)
	summary.ContentForms = clampWireList(summary.ContentForms)
	summary.CallIDs = clampWireList(summary.CallIDs)
	summary.ResultIDs = clampWireList(summary.ResultIDs)
	summary.ToolCount = len(request.Tools)
	for _, tool := range request.Tools {
		summary.ToolNames = append(summary.ToolNames, clampWireString(tool.Function.Name))
	}
	summary.ToolNames = clampWireList(summary.ToolNames)
	summary.ToolChoice = summarizeToolChoice(request.Choice)
	return summary
}

func contentForm(raw json.RawMessage) string {
	trimmed := bytes.TrimSpace(raw)
	switch {
	case len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")):
		return "empty"
	case trimmed[0] == '"':
		return "string"
	case trimmed[0] == '[':
		return "array"
	default:
		return "other"
	}
}

func summarizeToolChoice(raw json.RawMessage) string {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return ""
	}
	var name string
	if err := json.Unmarshal(trimmed, &name); err == nil {
		return clampWireString(name)
	}
	return "object"
}

func clampWireList(values []string) []string {
	if len(values) > wireListLimit {
		return values[:wireListLimit]
	}
	return values
}

func clampWireString(value string) string {
	if len(value) <= wireStringLimit {
		return value
	}
	return strings.ToValidUTF8(value[:wireStringLimit], "")
}

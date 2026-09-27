package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"agent-vivy/internal/domain"
)

const (
	HistorySearchName = "history_search"
	HistoryReadName   = "history_read"
	HistoryTraceName  = "history_trace"
)

// HistoryOperations is the narrow runtime boundary shared by model tools and
// inspection RPC. It carries no storage or actor implementation details.
type HistoryOperations interface {
	Search(context.Context, domain.HistorySearchRequest) (domain.HistoryPage, error)
	Read(context.Context, domain.HistoryReadRequest) (domain.HistoryPage, error)
	Trace(context.Context, domain.HistoryTraceRequest) (domain.HistoryPage, error)
}

type HistoryCapabilityLimits struct {
	SearchQueryBytes  int `json:"search_query_bytes"`
	SearchPageDefault int `json:"search_page_default"`
	SearchPageMax     int `json:"search_page_max"`
	ReadPageDefault   int `json:"read_page_default"`
	ReadPageMax       int `json:"read_page_max"`
	CandidateRecords  int `json:"candidate_records"`
	CandidateBytes    int `json:"candidate_bytes"`
	ResultItemBytes   int `json:"result_item_bytes"`
	ResultPageBytes   int `json:"result_page_bytes"`
}

type HistoryCapabilities struct {
	Kinds   []string                `json:"kinds"`
	Filters []string                `json:"filters"`
	Limits  HistoryCapabilityLimits `json:"limits"`
}

type HistoryCapabilitiesOperations interface {
	Capabilities(context.Context) (HistoryCapabilities, error)
}

type historySearchTool struct{ ops HistoryOperations }
type historyReadTool struct{ ops HistoryOperations }
type historyTraceTool struct{ ops HistoryOperations }

func NewHistorySearch(ops HistoryOperations) Tool { return &historySearchTool{ops: ops} }
func NewHistoryRead(ops HistoryOperations) Tool   { return &historyReadTool{ops: ops} }
func NewHistoryTrace(ops HistoryOperations) Tool  { return &historyTraceTool{ops: ops} }

func (t *historySearchTool) Spec() domain.ToolSpec {
	return domain.ToolSpec{
		Name:        HistorySearchName,
		Description: "Searches authorized session history with bounded literal matching and redacted snippets.",
		Readonly:    true,
		Keywords:    []string{"history", "search", "session", "inspect"},
		Schema:      json.RawMessage(historySearchSchema),
	}
}

func (t *historyReadTool) Spec() domain.ToolSpec {
	return domain.ToolSpec{
		Name:        HistoryReadName,
		Description: "Reads a bounded, explicitly selected set of authorized history records.",
		Readonly:    true,
		Keywords:    []string{"history", "read", "source", "records"},
		Schema:      json.RawMessage(historyReadSchema),
	}
}

func (t *historyTraceTool) Spec() domain.ToolSpec {
	return domain.ToolSpec{
		Name:        HistoryTraceName,
		Description: "Traces immediate provenance for one authorized history record.",
		Readonly:    true,
		Keywords:    []string{"history", "trace", "provenance", "source"},
		Schema:      json.RawMessage(historyTraceSchema),
	}
}

func (t *historySearchTool) InvokableRun(ctx context.Context, args json.RawMessage) (string, error) {
	if t.ops == nil {
		return "", fmt.Errorf("tools: %s backend not wired", HistorySearchName)
	}
	var request domain.HistorySearchRequest
	if err := decodeStringArgs(args, &request); err != nil {
		return "", err
	}
	page, err := t.ops.Search(ctx, request)
	if err != nil {
		return "", err
	}
	return marshalToolResult(page)
}

func (t *historyReadTool) InvokableRun(ctx context.Context, args json.RawMessage) (string, error) {
	if t.ops == nil {
		return "", fmt.Errorf("tools: %s backend not wired", HistoryReadName)
	}
	var request domain.HistoryReadRequest
	if err := decodeStringArgs(args, &request); err != nil {
		return "", err
	}
	page, err := t.ops.Read(ctx, request)
	if err != nil {
		return "", err
	}
	return marshalToolResult(page)
}

func (t *historyTraceTool) InvokableRun(ctx context.Context, args json.RawMessage) (string, error) {
	if t.ops == nil {
		return "", fmt.Errorf("tools: %s backend not wired", HistoryTraceName)
	}
	var request domain.HistoryTraceRequest
	if err := decodeStringArgs(args, &request); err != nil {
		return "", err
	}
	page, err := t.ops.Trace(ctx, request)
	if err != nil {
		return "", err
	}
	return marshalToolResult(page)
}

// WithHistory appends the three readonly adapters as one cohesive registry
// addition. Existing constructors remain source-compatible for embedders.
func (r *Registry) WithHistory(ops HistoryOperations) *Registry {
	if ops == nil {
		return r
	}
	return r.WithAdditional(NewHistorySearch(ops), NewHistoryRead(ops), NewHistoryTrace(ops))
}

const historySearchSchema = `{
  "type":"object",
  "additionalProperties":false,
  "properties":{
    "query":{"type":"string","maxLength":512},
    "session_ids":{"type":"array","items":{"type":"string"},"maxItems":20},
    "from":{"type":"integer","minimum":0},
    "to":{"type":"integer","minimum":0},
    "kinds":{"type":"array","items":{"type":"string"}},
    "artifact_id":{"type":"string"},
    "task_id":{"type":"string"},
    "cursor":{"type":"string"},
    "limit":{"type":"integer","minimum":0,"maximum":50}
  }
}`

const historyReadSchema = `{
  "type":"object",
  "additionalProperties":false,
  "properties":{
    "selection":{"type":"object"},
    "reference_id":{"type":"string"},
    "cursor":{"type":"string"},
    "limit":{"type":"integer","minimum":0,"maximum":100}
  }
}`

const historyTraceSchema = `{
  "type":"object",
  "additionalProperties":false,
  "properties":{
    "source_ref":{"type":"object"},
    "reference_id":{"type":"string"},
    "deliverable_id":{"type":"string"}
  }
}`

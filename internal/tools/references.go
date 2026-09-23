package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"agent-vivy/internal/domain"
)

const ReferenceContextName = "reference_context"

// ReferenceOperations is the narrow runtime boundary for explicit context
// references. Preview serves trusted RPC composition; Attach serves the model
// tool and commits the destination-owned sanitized snapshot.
type ReferenceOperations interface {
	Preview(context.Context, domain.HistorySelection) (domain.ReferencePreview, error)
	Attach(context.Context, domain.ReferenceSelection) (domain.ContextReference, error)
	// Get reads one destination-owned snapshot plus its live source and
	// feed status for operator-facing surfaces.
	Get(context.Context, domain.SessionID, string) (domain.ReferenceView, error)
}

type referenceContextTool struct{ ops ReferenceOperations }

func NewReferenceContext(ops ReferenceOperations) Tool { return &referenceContextTool{ops: ops} }

func (t *referenceContextTool) Spec() domain.ToolSpec {
	return domain.ToolSpec{
		Name:        ReferenceContextName,
		Description: "Attaches a bounded, sanitized snapshot of explicitly selected authorized history records to this run's context.",
		Readonly:    false,
		Keywords:    []string{"reference", "context", "attach", "history"},
		Schema:      json.RawMessage(referenceContextSchema),
	}
}

type referenceContextArgs struct {
	Selection      domain.HistorySelection `json:"selection"`
	ExpectedDigest string                  `json:"expected_digest"`
	Description    string                  `json:"description"`
}

type referenceContextResult struct {
	ReferenceID string               `json:"reference_id"`
	Digest      string               `json:"digest"`
	Origin      string               `json:"origin"`
	ItemCount   int                  `json:"item_count"`
	Excerpt     []domain.HistoryItem `json:"excerpt"`
}

func (t *referenceContextTool) InvokableRun(ctx context.Context, args json.RawMessage) (string, error) {
	if t.ops == nil {
		return "", fmt.Errorf("tools: %s backend not wired", ReferenceContextName)
	}
	var input referenceContextArgs
	if err := decodeStringArgs(args, &input); err != nil {
		return "", err
	}
	reference, err := t.ops.Attach(ctx, domain.ReferenceSelection{
		Selection:      input.Selection,
		ExpectedDigest: input.ExpectedDigest,
	})
	if err != nil {
		return "", err
	}
	return marshalToolResult(referenceContextResult{
		ReferenceID: reference.ID,
		Digest:      reference.Digest,
		Origin:      reference.Origin,
		ItemCount:   len(reference.Items),
		Excerpt:     reference.Items,
	})
}

// WithReferences appends the model attach adapter. GUI task attachment goes
// through admission and does not route here.
func (r *Registry) WithReferences(ops ReferenceOperations) *Registry {
	if ops == nil {
		return r
	}
	return r.WithAdditional(NewReferenceContext(ops))
}

const referenceContextSchema = `{
  "type":"object",
  "additionalProperties":false,
  "properties":{
    "selection":{
      "type":"object",
      "additionalProperties":false,
      "properties":{
        "source_session_id":{"type":"string"},
        "refs":{"type":"array","items":{"type":"object"},"maxItems":100},
        "run_range":{"type":"object"}
      },
      "required":["source_session_id"]
    },
    "expected_digest":{"type":"string","maxLength":128},
    "description":{"type":"string","maxLength":512}
  },
  "required":["selection"]
}`

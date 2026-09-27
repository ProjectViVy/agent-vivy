package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"agent-vivy/internal/domain"
)

const PresentFilesName = "present_files"

// DeliverableOperations is the narrow runtime boundary for explicit file
// presentation. Present commits an immutable delivery set from the trusted
// run workspace; List/Get/Read/CloseTransfer serve operator inspection and
// verified download.
type DeliverableOperations interface {
	Present(context.Context, domain.PresentRequest) (domain.DeliverySet, error)
	List(context.Context, domain.SessionID, string, int) (domain.DeliverySetPage, error)
	Get(context.Context, domain.SessionID, string) (domain.DeliverySet, error)
	Read(context.Context, domain.DeliveryReadRequest) (domain.DeliveryChunk, error)
	CloseTransfer(context.Context, string) error
}

type presentFilesTool struct{ ops DeliverableOperations }

func NewPresentFiles(ops DeliverableOperations) Tool { return &presentFilesTool{ops: ops} }

func (t *presentFilesTool) Spec() domain.ToolSpec {
	return domain.ToolSpec{
		Name:        PresentFilesName,
		Description: "Presents workspace files as an immutable, verified delivery set the user can inspect and download.",
		Readonly:    false,
		Keywords:    []string{"deliver", "present", "file", "artifact", "download"},
		Schema:      json.RawMessage(presentFilesSchema),
	}
}

type presentFilesArgs struct {
	Files []domain.PresentFile `json:"files"`
	Title string               `json:"title"`
}

type presentFilesResult struct {
	SetID    string                   `json:"set_id"`
	Status   string                   `json:"status"`
	Items    []domain.Deliverable     `json:"items"`
	Failures []domain.DeliveryFailure `json:"failures,omitempty"`
}

func (t *presentFilesTool) InvokableRun(ctx context.Context, args json.RawMessage) (string, error) {
	if t.ops == nil {
		return "", fmt.Errorf("tools: %s backend not wired", PresentFilesName)
	}
	var input presentFilesArgs
	if err := decodeStringArgs(args, &input); err != nil {
		return "", err
	}
	set, err := t.ops.Present(ctx, domain.PresentRequest{Files: input.Files, Title: input.Title})
	if err != nil {
		return "", err
	}
	return marshalToolResult(presentFilesResult{
		SetID:    set.ID,
		Status:   set.Status,
		Items:    set.Items,
		Failures: set.Failures,
	})
}

// WithDeliverables appends the model presentation adapter. GUI inspection
// and download go through the deliverables/* RPCs and do not route here.
func (r *Registry) WithDeliverables(ops DeliverableOperations) *Registry {
	if ops == nil {
		return r
	}
	return r.WithAdditional(NewPresentFiles(ops))
}

const presentFilesSchema = `{
  "type":"object",
  "additionalProperties":false,
  "properties":{
    "files":{
      "type":"array",
      "minItems":1,
      "maxItems":20,
      "items":{
        "type":"object",
        "additionalProperties":false,
        "properties":{
          "path":{"type":"string","maxLength":1024},
          "description":{"type":"string","maxLength":512}
        },
        "required":["path","description"]
      }
    },
    "title":{"type":"string","maxLength":512}
  },
  "required":["files"]
}`

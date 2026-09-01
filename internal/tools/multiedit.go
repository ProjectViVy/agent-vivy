package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"agent-vivy/internal/domain"
)

const MultiEditName = "multiedit"

// FileMultiEditItem is one string replacement inside a multiedit call.
type FileMultiEditItem struct {
	OldString  string
	NewString  string
	ReplaceAll bool
}

// FileMultiEditRequest applies several replacements to one file atomically.
type FileMultiEditRequest struct {
	Path  string
	Edits []FileMultiEditItem
}

// MultiPatchOperations is implemented by filesystem backends that support
// atomic multi-edit; the multiedit tool registers only when present.
type MultiPatchOperations interface {
	MultiPatchFile(context.Context, domain.RunID, FileMultiEditRequest) (FileMutationResult, error)
}

type multiEditPreviewer interface {
	PrepareMultiPatchFile(context.Context, domain.RunID, FileMultiEditRequest) (domain.ToolProposal, error)
}

type multiEditTool struct{ ops MultiPatchOperations }

func NewMultiEdit(ops MultiPatchOperations) Tool { return &multiEditTool{ops: ops} }

func (t *multiEditTool) Spec() domain.ToolSpec {
	return domain.ToolSpec{
		Name: MultiEditName,
		Description: "Applies multiple exact string replacements to one file atomically: every edit must apply or nothing is written. " +
			"When exact matching fails, a whitespace-tolerant match may apply the edit with the file's own indentation.",
		Readonly: false,
		Keywords: []string{"multiedit", "edit", "patch", "replace", "batch"},
		Params: map[string]domain.ToolParam{
			"path":  {Desc: "Workspace-relative file path.", Required: true},
			"edits": {Desc: "Array of {old_string, new_string, replace_all} replacements applied in order.", Required: true, Type: "array"},
		},
	}
}

type multiEditItemArgs struct {
	OldString  string `json:"old_string"`
	NewString  string `json:"new_string"`
	ReplaceAll string `json:"replace_all"`
}

func decodeMultiEditRequest(args json.RawMessage) (FileMultiEditRequest, error) {
	var input struct {
		Path  string              `json:"path"`
		Edits []multiEditItemArgs `json:"edits"`
	}
	if err := decodeStringArgs(args, &input); err != nil {
		return FileMultiEditRequest{}, err
	}
	req := FileMultiEditRequest{Path: input.Path}
	if len(input.Edits) == 0 {
		return FileMultiEditRequest{}, &ArgError{Field: "edits", Reason: "must contain at least one edit"}
	}
	for i, raw := range input.Edits {
		replaceAll, err := optionalBool("edits.replace_all", raw.ReplaceAll)
		if err != nil {
			return FileMultiEditRequest{}, err
		}
		if raw.OldString == "" {
			return FileMultiEditRequest{}, &ArgError{Field: fmt.Sprintf("edits[%d].old_string", i), Reason: "is required"}
		}
		if raw.OldString == raw.NewString {
			return FileMultiEditRequest{}, &ArgError{Field: fmt.Sprintf("edits[%d].old_string", i), Reason: "must differ from new_string"}
		}
		req.Edits = append(req.Edits, FileMultiEditItem{OldString: raw.OldString, NewString: raw.NewString, ReplaceAll: replaceAll})
	}
	return req, nil
}

func (t *multiEditTool) InvokableRun(ctx context.Context, args json.RawMessage) (string, error) {
	req, err := decodeMultiEditRequest(args)
	if err != nil {
		return "", err
	}
	if t.ops == nil {
		return "", fmt.Errorf("tools: %s backend not wired", MultiEditName)
	}
	result, err := t.ops.MultiPatchFile(ctx, RunIDFromContext(ctx), req)
	if err != nil {
		return "", err
	}
	return marshalToolResult(attachWriteDiagnostics(ctx, t.ops, result, req.Path))
}

func (t *multiEditTool) PrepareProposal(ctx context.Context, args json.RawMessage) (domain.ToolProposal, error) {
	req, err := decodeMultiEditRequest(args)
	if err != nil {
		return domain.ToolProposal{}, err
	}
	if previewer, ok := t.ops.(multiEditPreviewer); ok {
		return previewer.PrepareMultiPatchFile(ctx, RunIDFromContext(ctx), req)
	}
	total := 0
	for _, item := range req.Edits {
		total += len(item.OldString)
	}
	return domain.ToolProposal{Action: MultiEditName, Target: req.Path, Preview: fmt.Sprintf("apply %d edits (%d old bytes)", len(req.Edits), total)}, nil
}

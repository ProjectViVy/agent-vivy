package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"agent-vivy/internal/domain"
)

const (
	SkillsListName  = "skills_list"
	SkillViewName   = "skill_view"
	SkillManageName = "skill_manage"
)

type SkillSummary struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Context     string   `json:"context,omitempty"`
	Agent       string   `json:"agent,omitempty"`
	Model       string   `json:"model,omitempty"`
	Hash        string   `json:"hash"`
	Warnings    []string `json:"warnings,omitempty"`
}

type SkillView struct {
	SkillSummary
	Content         string   `json:"content"`
	RelativePath    string   `json:"relative_path"`
	SupportingFiles []string `json:"supporting_files,omitempty"`
}

type SkillManageRequest struct {
	Action     string
	RevisionID string
	SkillName  string
	Path       string
	Content    string
	OldString  string
	NewString  string
	ReplaceAll bool
}

type SkillManageResult struct {
	RevisionID string   `json:"revision_id,omitempty"`
	Status     string   `json:"status"`
	SkillName  string   `json:"skill_name"`
	Action     string   `json:"action"`
	TargetPath string   `json:"target_path"`
	Hash       string   `json:"hash,omitempty"`
	Warnings   []string `json:"warnings,omitempty"`
}

type SkillOperations interface {
	ListSkills(context.Context, domain.RunID) ([]SkillSummary, error)
	ViewSkill(context.Context, domain.RunID, string, string) (SkillView, error)
	ManageSkill(context.Context, domain.RunID, SkillManageRequest) (SkillManageResult, error)
	PrepareSkillProposal(context.Context, domain.RunID, SkillManageRequest) (domain.ToolProposal, error)
	ApplySkillRevision(context.Context, domain.RunID, string) (SkillManageResult, error)
}

type skillsListTool struct{ ops SkillOperations }
type skillViewTool struct{ ops SkillOperations }
type skillManageTool struct{ ops SkillOperations }

func NewSkillsList(ops SkillOperations) Tool  { return &skillsListTool{ops: ops} }
func NewSkillView(ops SkillOperations) Tool   { return &skillViewTool{ops: ops} }
func NewSkillManage(ops SkillOperations) Tool { return &skillManageTool{ops: ops} }

func (t *skillsListTool) Spec() domain.ToolSpec {
	return domain.ToolSpec{Name: SkillsListName, Description: "Lists metadata for trusted local Skills.", Readonly: true,
		Keywords: []string{"skills", "skill", "capability", "procedure"}}
}

func (t *skillsListTool) InvokableRun(ctx context.Context, _ json.RawMessage) (string, error) {
	if t.ops == nil {
		return "", fmt.Errorf("tools: skills backend not wired")
	}
	items, err := t.ops.ListSkills(ctx, RunIDFromContext(ctx))
	if err != nil {
		return "", err
	}
	return marshalToolResult(items)
}

func (t *skillViewTool) Spec() domain.ToolSpec {
	return domain.ToolSpec{Name: SkillViewName, Description: "Reads one trusted Skill or a bounded supporting file as untrusted data.", Readonly: true,
		Keywords: []string{"skill", "view", "read", "procedure"}, Params: map[string]domain.ToolParam{
			"name": {Desc: "Skill name.", Required: true},
			"path": {Desc: "Optional relative supporting file path.", Required: false},
		}}
}

func (t *skillViewTool) InvokableRun(ctx context.Context, args json.RawMessage) (string, error) {
	var input struct {
		Name string `json:"name"`
		Path string `json:"path"`
	}
	if err := decodeStringArgs(args, &input); err != nil {
		return "", err
	}
	if strings.TrimSpace(input.Name) == "" {
		return "", &ArgError{Field: "name", Reason: "is required"}
	}
	if t.ops == nil {
		return "", fmt.Errorf("tools: skills backend not wired")
	}
	view, err := t.ops.ViewSkill(ctx, RunIDFromContext(ctx), input.Name, input.Path)
	if err != nil {
		return "", err
	}
	return marshalToolResult(view)
}

func (t *skillManageTool) Spec() domain.ToolSpec {
	return domain.ToolSpec{Name: SkillManageName, Description: "Stages a Skill create/edit/patch/delete mutation for human approval.", Readonly: false,
		Keywords: []string{"skill", "manage", "create", "edit", "delete", "procedure"}, Params: map[string]domain.ToolParam{
			"action":      {Desc: "create, edit, patch, delete, write_file, remove_file, or rollback.", Required: true},
			"revision_id": {Desc: "Applied revision id for rollback.", Required: false},
			"name":        {Desc: "Skill name.", Required: true},
			"path":        {Desc: "Supporting file path for write_file/remove_file.", Required: false},
			"content":     {Desc: "New SKILL.md or supporting-file content.", Required: false},
			"old_string":  {Desc: "Exact patch source.", Required: false},
			"new_string":  {Desc: "Patch replacement.", Required: false},
			"replace_all": {Desc: "Optional true/false for patch.", Required: false},
		}}
}

func (t *skillManageTool) InvokableRun(ctx context.Context, args json.RawMessage) (string, error) {
	req, err := decodeSkillManageRequest(args)
	if err != nil {
		return "", err
	}
	if t.ops == nil {
		return "", fmt.Errorf("tools: skills backend not wired")
	}
	var result SkillManageResult
	proposalData := ProposalDataFromContext(ctx)
	if len(proposalData) > 0 {
		var ref struct {
			RevisionID string `json:"revision_id"`
		}
		if json.Unmarshal(proposalData, &ref) == nil && ref.RevisionID != "" {
			result, err = t.ops.ApplySkillRevision(ctx, RunIDFromContext(ctx), ref.RevisionID)
		} else {
			result, err = t.ops.ManageSkill(ctx, RunIDFromContext(ctx), req)
		}
	} else {
		result, err = t.ops.ManageSkill(ctx, RunIDFromContext(ctx), req)
	}
	if err != nil {
		return "", err
	}
	return marshalToolResult(result)
}

func (t *skillManageTool) PrepareProposal(ctx context.Context, args json.RawMessage) (domain.ToolProposal, error) {
	req, err := decodeSkillManageRequest(args)
	if err != nil {
		return domain.ToolProposal{}, err
	}
	if t.ops == nil {
		return domain.ToolProposal{}, fmt.Errorf("tools: skills backend not wired")
	}
	return t.ops.PrepareSkillProposal(ctx, RunIDFromContext(ctx), req)
}

func decodeSkillManageRequest(args json.RawMessage) (SkillManageRequest, error) {
	var input struct {
		Action     string `json:"action"`
		RevisionID string `json:"revision_id"`
		Name       string `json:"name"`
		Path       string `json:"path"`
		Content    string `json:"content"`
		OldString  string `json:"old_string"`
		NewString  string `json:"new_string"`
		ReplaceAll string `json:"replace_all"`
	}
	if err := decodeStringArgs(args, &input); err != nil {
		return SkillManageRequest{}, err
	}
	replaceAll, err := optionalBool("replace_all", input.ReplaceAll)
	if err != nil {
		return SkillManageRequest{}, err
	}
	if strings.TrimSpace(input.Action) == "" {
		return SkillManageRequest{}, &ArgError{Field: "action", Reason: "is required"}
	}
	if strings.TrimSpace(input.Name) == "" {
		return SkillManageRequest{}, &ArgError{Field: "name", Reason: "is required"}
	}
	return SkillManageRequest{Action: input.Action, RevisionID: input.RevisionID, SkillName: input.Name, Path: input.Path, Content: input.Content,
		OldString: input.OldString, NewString: input.NewString, ReplaceAll: replaceAll}, nil
}

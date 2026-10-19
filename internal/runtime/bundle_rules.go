package runtime

import (
	"context"
	"errors"
	"os"
	"sort"
	"strings"

	"github.com/cloudwego/eino/adk/middlewares/agentsmd"

	"agent-vivy/internal/bundlecontract"
	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

const ToolkitAgentsMDFileName = "toolkits/AGENTS.md"

// PrepareBundleRules builds the exact Rule projection for a face from one
// immutable catalog view. It reads no filesystem paths and returns nil when
// the view has no Rules applicable to the requested face.
func PrepareBundleRules(view bundlecontract.View, face domain.Face) (*storage.RunRulesSnapshot, error) {
	if !face.Valid() {
		return nil, errors.New("runtime: invalid face for Toolkit Rules")
	}
	components := view.Components()
	blocks := make([]storage.RunRuleBlock, 0)
	for _, component := range components {
		if component.Ref.Kind != bundlecontract.Rule || !ruleAppliesToFace(component.Faces, face) {
			continue
		}
		if !component.Available || strings.TrimSpace(component.Body) == "" {
			return nil, errors.New("runtime: applicable Toolkit Rule is unavailable")
		}
		blocks = append(blocks, storage.RunRuleBlock{
			Ref: component.Ref, Release: component.Release,
			ContentDigest: component.ContentDigest, Body: component.Body,
		})
	}
	if len(blocks) == 0 {
		return nil, nil
	}
	sort.Slice(blocks, func(i, j int) bool {
		left, _ := bundlecontract.SkillID(blocks[i].Ref.InstallID, blocks[i].Ref.LocalID)
		right, _ := bundlecontract.SkillID(blocks[j].Ref.InstallID, blocks[j].Ref.LocalID)
		return left < right
	})
	snapshot := &storage.RunRulesSnapshot{Face: string(face), Blocks: blocks}
	snapshot.Body = storage.RenderRunRules(blocks)
	digest, err := storage.RunRulesDigest(*snapshot)
	if err != nil {
		return nil, err
	}
	snapshot.Digest = digest
	if err := storage.ValidateRunRulesSnapshot(snapshot); err != nil {
		return nil, err
	}
	return snapshot, nil
}

func ruleAppliesToFace(faces []string, face domain.Face) bool {
	if len(faces) == 0 {
		return true
	}
	for _, candidate := range faces {
		if candidate == string(face) {
			return true
		}
	}
	return false
}

// ToolkitAgentsMDBackend serves the durable Rules snapshot at one synthetic
// path and delegates all project/workspace reads to the existing backend.
// Eino's agentsmd parser remains the only injection mechanism.
type ToolkitAgentsMDBackend struct {
	base AgentsMDBackend
}

var _ AgentsMDBackend = (*ToolkitAgentsMDBackend)(nil)

func NewToolkitAgentsMDBackend(base AgentsMDBackend) *ToolkitAgentsMDBackend {
	return &ToolkitAgentsMDBackend{base: base}
}

func (*ToolkitAgentsMDBackend) toolkitRulesEnabled() {}

func (b *ToolkitAgentsMDBackend) Read(ctx context.Context, req *agentsmd.ReadRequest) (*agentsmd.FileContent, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if req == nil {
		return nil, errors.New("runtime: agentsmd read request is nil")
	}
	requested := strings.TrimPrefix(strings.ReplaceAll(req.FilePath, "\\", "/"), "./")
	if requested == ToolkitAgentsMDFileName {
		snapshot, ok := runPrompt(ctx)
		if !ok {
			return nil, os.ErrNotExist
		}
		payload, err := storage.ValidateRunPromptSnapshot(snapshot)
		if err != nil {
			return nil, err
		}
		if payload.Rules == nil {
			return nil, os.ErrNotExist
		}
		return &agentsmd.FileContent{Content: payload.Rules.Body}, nil
	}
	if b == nil || b.base == nil {
		return nil, os.ErrNotExist
	}
	return b.base.Read(ctx, req)
}

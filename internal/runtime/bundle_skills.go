package runtime

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"agent-vivy/internal/bundlecontract"
	"agent-vivy/internal/bundles"
	"agent-vivy/internal/domain"
	"agent-vivy/internal/skillhost"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/tools"
	"agent-vivy/sdk/port/skillsource"
)

const (
	bundleSkillSourceID = "vivy.bundle-skills"
	BundleSkillOrigin   = "bundle"
)

// BundleSkillSource projects only immutable, admitted Skill records into the
// existing SkillHost. An absent run View is an error; the source never reads
// the coordinator's latest catalog implicitly.
type BundleSkillSource struct{}

func NewBundleSkillSource() *BundleSkillSource { return &BundleSkillSource{} }

func (*BundleSkillSource) ID() string { return bundleSkillSourceID }

func (*BundleSkillSource) List(ctx context.Context, _ skillsource.Request) ([]skillsource.Summary, error) {
	view, ok := bundleView(ctx)
	if !ok {
		return nil, errors.New("runtime: captured toolkit view unavailable for Skills")
	}
	items := make([]skillsource.Summary, 0)
	for _, record := range view.Components() {
		if record.Ref.Kind != bundlecontract.Skill {
			continue
		}
		items = append(items, bundleSkill(record).Summary())
	}
	return items, nil
}

func (*BundleSkillSource) Get(ctx context.Context, _ skillsource.Request, id string) (skillsource.Skill, error) {
	view, ok := bundleView(ctx)
	if !ok {
		return skillsource.Skill{}, errors.New("runtime: captured toolkit view unavailable for Skills")
	}
	record, found := view.Lookup(bundlecontract.Skill, id)
	if !found || !record.Available {
		return skillsource.Skill{}, fmt.Errorf("runtime: toolkit Skill %q is unavailable in the captured view", id)
	}
	return bundleSkill(record), nil
}

func bundleSkill(record bundlecontract.ComponentRecord) skillsource.Skill {
	version := fmt.Sprintf("%s#%d", record.Release.BundleID, record.Release.ReleaseSequence)
	return skillsource.Skill{
		ID: record.ProjectedID, Name: record.ProjectedID, Description: record.Description,
		Version: version, SourceHash: record.ContentDigest, Content: record.Body,
		Available: record.Available,
		Metadata: map[string]string{
			"origin": BundleSkillOrigin, "display_name": record.Name,
			"display_ref": record.DisplayRef, "release_digest": record.Release.ReleaseDigest,
			"install_id": record.Ref.InstallID, "component_id": record.Ref.LocalID,
		},
	}
}

var _ skillsource.Provider = (*BundleSkillSource)(nil)

// BundleSkillOperations combines the existing local management owner with
// read-only Toolkit projections. The manager callback is late-bound because
// app composition creates the runtime tool set before the coordinator.
type BundleSkillOperations struct {
	local   tools.SkillOperations
	manager func() *bundles.Manager
	hosted  *HostedSkillBackend
	hostErr error
}

func NewBundleSkillOperations(local tools.SkillOperations, manager func() *bundles.Manager) *BundleSkillOperations {
	return newBundleSkillOperations(local, manager, skillhost.Config{})
}

func newBundleSkillOperations(local tools.SkillOperations, manager func() *bundles.Manager, hostConfig skillhost.Config) *BundleSkillOperations {
	var localBackend *EinoSkillBackend
	if backend, ok := local.(*EinoSkillBackend); ok {
		localBackend = backend
	}
	hosted, err := newHostedSkillBackend(localBackend, hostConfig, NewBundleSkillSource())
	return &BundleSkillOperations{local: local, manager: manager, hosted: hosted, hostErr: err}
}

func (ops *BundleSkillOperations) pinnedView(ctx context.Context, runID domain.RunID) (bundlecontract.View, func(), error) {
	if view, ok := bundleView(ctx); ok {
		return view, func() {}, nil
	}
	manager := (*bundles.Manager)(nil)
	if ops.manager != nil {
		manager = ops.manager()
	}
	if manager == nil {
		return bundlecontract.View{}, func() {}, nil
	}
	var handle *bundles.ViewHandle
	var err error
	if runID != "" {
		handle, err = manager.PinRun(ctx, runID)
		// Runs admitted before Toolkit support have no snapshot and retain the
		// established local Skill surface. New runs fail admission if their
		// captured snapshot is absent or corrupt.
		if errors.Is(err, storage.ErrNotFound) {
			return bundlecontract.View{}, func() {}, nil
		}
	} else {
		handle, err = manager.PinCurrent()
	}
	if err != nil {
		return bundlecontract.View{}, nil, err
	}
	return handle.View(), handle.Release, nil
}

func (ops *BundleSkillOperations) ListSkills(ctx context.Context, runID domain.RunID) ([]tools.SkillSummary, error) {
	var out []tools.SkillSummary
	if ops.local != nil {
		items, err := ops.local.ListSkills(ctx, runID)
		if err != nil {
			return nil, err
		}
		out = append(out, items...)
	}
	view, release, err := ops.pinnedView(ctx, runID)
	if err != nil {
		return nil, err
	}
	defer release()
	seen := make(map[string]struct{}, len(out))
	for _, item := range out {
		seen[item.Name] = struct{}{}
	}
	if ops.hostErr != nil {
		return nil, ops.hostErr
	}
	if ops.hosted != nil {
		ctx = withBundleView(ctx, view)
		items, err := ops.hosted.host.List(ctx, hostedSkillRequest(ctx))
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			if item.SourceID != bundleSkillSourceID {
				continue
			}
			if _, duplicate := seen[item.ID]; duplicate {
				return nil, fmt.Errorf("runtime: duplicate Skill ID %q across local and Toolkit sources", item.ID)
			}
			seen[item.ID] = struct{}{}
			record, ok := view.Lookup(bundlecontract.Skill, item.ID)
			if !ok {
				return nil, fmt.Errorf("runtime: SkillHost returned an unknown Toolkit Skill %q", item.ID)
			}
			out = append(out, toolkitSkillSummary(record))
		}
	}
	return out, nil
}

func toolkitSkillSummary(record bundlecontract.ComponentRecord) tools.SkillSummary {
	return tools.SkillSummary{
		Name: record.ProjectedID, DisplayName: record.Name, Description: record.Description,
		Origin: BundleSkillOrigin, Source: record.DisplayRef, Enabled: record.Available,
		Hash: record.ContentDigest,
	}
}

func (ops *BundleSkillOperations) ViewSkill(ctx context.Context, runID domain.RunID, name, relativePath string) (tools.SkillView, error) {
	view, release, err := ops.pinnedView(ctx, runID)
	if err != nil {
		return tools.SkillView{}, err
	}
	defer release()
	record, found := view.Lookup(bundlecontract.Skill, strings.TrimSpace(name))
	if found {
		if !record.Available {
			return tools.SkillView{}, fmt.Errorf("runtime: toolkit Skill %q is disabled", name)
		}
		if ops.hostErr != nil {
			return tools.SkillView{}, ops.hostErr
		}
		if ops.hosted != nil {
			ctx = withBundleView(ctx, view)
			if _, err := ops.hosted.host.Get(ctx, hostedSkillRequest(ctx), record.ProjectedID); err != nil {
				return tools.SkillView{}, err
			}
		}
		summary := toolkitSkillSummary(record)
		path := strings.TrimSpace(relativePath)
		if path == "" || path == "SKILL.md" {
			return tools.SkillView{SkillSummary: summary, Content: record.Body, RelativePath: "SKILL.md", SupportingFiles: append([]string{}, record.Resources...)}, nil
		}
		manager := (*bundles.Manager)(nil)
		if ops.manager != nil {
			manager = ops.manager()
		}
		if manager == nil {
			return tools.SkillView{}, errors.New("runtime: Toolkit resource reader unavailable")
		}
		content, err := manager.ReadSkillResource(ctx, view, record.ProjectedID, path)
		if err != nil {
			return tools.SkillView{}, err
		}
		return tools.SkillView{SkillSummary: summary, Content: content, RelativePath: path}, nil
	}
	if ops.local == nil {
		return tools.SkillView{}, fmt.Errorf("runtime: Skill %q not found", name)
	}
	return ops.local.ViewSkill(ctx, runID, name, relativePath)
}

func (ops *BundleSkillOperations) rejectBundleMutation(name string) error {
	if isProjectedBundleSkillID(strings.TrimSpace(name)) {
		return errors.New("read_only_component: Toolkit Skills cannot be modified through skill_manage")
	}
	return nil
}

func isProjectedBundleSkillID(id string) bool {
	// Projected IDs are `b_` + 32 lowercase hex digits + `_` + native local ID.
	if len(id) < 2+32+1+1 || !strings.HasPrefix(id, "b_") || id[34] != '_' {
		return false
	}
	if _, err := hex.DecodeString(id[2:34]); err != nil {
		return false
	}
	return bundlecontract.ValidLocalID(id[35:])
}

func (ops *BundleSkillOperations) ManageSkill(ctx context.Context, runID domain.RunID, request tools.SkillManageRequest) (tools.SkillManageResult, error) {
	if err := ops.rejectBundleMutation(request.SkillName); err != nil {
		return tools.SkillManageResult{}, err
	}
	if ops.local == nil {
		return tools.SkillManageResult{}, errors.New("tools: skills backend not wired")
	}
	return ops.local.ManageSkill(ctx, runID, request)
}

func (ops *BundleSkillOperations) PrepareSkillProposal(ctx context.Context, runID domain.RunID, request tools.SkillManageRequest) (domain.ToolProposal, error) {
	if err := ops.rejectBundleMutation(request.SkillName); err != nil {
		return domain.ToolProposal{}, err
	}
	if ops.local == nil {
		return domain.ToolProposal{}, errors.New("tools: skills backend not wired")
	}
	return ops.local.PrepareSkillProposal(ctx, runID, request)
}

func (ops *BundleSkillOperations) ApplySkillRevision(ctx context.Context, runID domain.RunID, revisionID string) (tools.SkillManageResult, error) {
	if isProjectedBundleSkillID(strings.TrimSpace(revisionID)) {
		return tools.SkillManageResult{}, errors.New("read_only_component: Toolkit Skills cannot be modified through skill_manage")
	}
	if ops.local == nil {
		return tools.SkillManageResult{}, errors.New("tools: skills backend not wired")
	}
	return ops.local.ApplySkillRevision(ctx, runID, revisionID)
}

func (ops *BundleSkillOperations) SetSkillEnabled(ctx context.Context, name string, enabled bool, expectedHash string) (tools.SkillSummary, error) {
	if err := ops.rejectBundleMutation(name); err != nil {
		return tools.SkillSummary{}, err
	}
	if ops.local == nil {
		return tools.SkillSummary{}, errors.New("tools: skills backend not wired")
	}
	return ops.local.SetSkillEnabled(ctx, name, enabled, expectedHash)
}

func (ops *BundleSkillOperations) GetSkillRevision(ctx context.Context, id string) (domain.SkillRevision, error) {
	lifecycle, ok := ops.local.(tools.SkillRevisionLifecycle)
	if !ok {
		return domain.SkillRevision{}, tools.ErrSkillRevisionHistoryUnavailable
	}
	return lifecycle.GetSkillRevision(ctx, id)
}

func (ops *BundleSkillOperations) SetSkillRevisionStatus(ctx context.Context, id string, status domain.SkillRevisionStatus) error {
	lifecycle, ok := ops.local.(tools.SkillRevisionLifecycle)
	if !ok {
		return tools.ErrSkillRevisionHistoryUnavailable
	}
	return lifecycle.SetSkillRevisionStatus(ctx, id, status)
}

func (ops *BundleSkillOperations) ListPendingSkillRevisions(ctx context.Context) ([]domain.SkillRevision, error) {
	lifecycle, ok := ops.local.(tools.SkillRevisionLifecycle)
	if !ok {
		return nil, tools.ErrSkillRevisionHistoryUnavailable
	}
	return lifecycle.ListPendingSkillRevisions(ctx)
}

func (ops *BundleSkillOperations) CheckSkillRevision(ctx context.Context, id string) error {
	lifecycle, ok := ops.local.(tools.SkillRevisionLifecycle)
	if !ok {
		return tools.ErrSkillRevisionHistoryUnavailable
	}
	return lifecycle.CheckSkillRevision(ctx, id)
}

var _ tools.SkillOperations = (*BundleSkillOperations)(nil)

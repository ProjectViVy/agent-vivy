package masks

import (
	"context"
	"errors"
	"fmt"

	"agent-vivy/internal/bundlecontract"
	"agent-vivy/internal/maskcontract"
)

// ToolkitViewManager is the immutable catalog pin seam shared with the
// coordinator. Mask ownership stays here; the coordinator only supplies a
// versioned view and its lease release.
type ToolkitViewManager interface {
	CurrentView(context.Context) (bundlecontract.View, func(), error)
}

// SetToolkitManager attaches the shared coordinator after process composition
// has recovered its published catalog. The Mask module keeps its existing
// owner boundary and does not own a second Toolkit catalog.
func (s *Service) SetToolkitManager(manager ToolkitViewManager) {
	if s != nil {
		s.toolkits = manager
	}
}

func getBundleMaskFromView(view bundlecontract.View, id string) (maskcontract.Definition, error) {
	record, ok := view.Lookup(bundlecontract.Mask, id)
	if !ok || !record.Available {
		return maskcontract.Definition{}, maskcontract.NewError(maskcontract.CodeMaskUnavailable, errors.New("selected toolkit Mask is unavailable"))
	}
	definition, err := bundleMaskDefinition(view, record)
	if err != nil {
		return maskcontract.Definition{}, maskcontract.NewError(maskcontract.CodeSnapshotCorrupt, err)
	}
	return definition, nil
}

func bundleMaskDefinition(view bundlecontract.View, record bundlecontract.ComponentRecord) (maskcontract.Definition, error) {
	if record.Ref.Kind != bundlecontract.Mask || record.ProjectedID == "" || record.Ref.InstallID != record.Release.InstallID {
		return maskcontract.Definition{}, errors.New("captured Mask component identity is invalid")
	}
	provenance := &maskcontract.BundleProvenance{Release: record.Release, LocalID: record.Ref.LocalID}
	recommendations := make([]maskcontract.SkillRecommendation, 0, len(record.RecommendedSkills))
	for _, ref := range record.RecommendedSkills {
		id, err := bundlecontract.SkillID(ref.InstallID, ref.LocalID)
		if err != nil || ref.Kind != bundlecontract.Skill || ref.InstallID != record.Ref.InstallID {
			return maskcontract.Definition{}, errors.New("captured Mask recommendation identity is invalid")
		}
		recommendation := maskcontract.SkillRecommendation{Ref: ref, SkillID: id, Name: ref.LocalID, InactiveReason: "skill_unavailable"}
		if skill, found := view.Lookup(bundlecontract.Skill, id); found && skill.Available {
			recommendation.Name = skill.Name
			recommendation.Available = true
			recommendation.InactiveReason = ""
		}
		recommendations = append(recommendations, recommendation)
	}
	definition := maskcontract.Definition{
		ID: record.ProjectedID, Name: record.Name, Description: record.Description, Body: record.Body,
		Revision: record.Release.ReleaseSequence,
		Digest:   maskcontract.DefinitionDigest(record.ProjectedID, record.Name, record.Body),
		Origin:   maskcontract.OriginBundle, Bundle: provenance, RecommendedSkills: recommendations,
	}
	if err := maskcontract.ValidateDefinition(definition); err != nil {
		return maskcontract.Definition{}, fmt.Errorf("invalid captured Mask: %w", err)
	}
	return definition, nil
}

func cloneProvenance(in *maskcontract.BundleProvenance) *maskcontract.BundleProvenance {
	if in == nil {
		return nil
	}
	out := *in
	out.Release = in.Release
	return &out
}

func cloneRecommendations(in []maskcontract.SkillRecommendation) []maskcontract.SkillRecommendation {
	if in == nil {
		return nil
	}
	out := append([]maskcontract.SkillRecommendation{}, in...)
	for i := range out {
		out[i].Ref = in[i].Ref
	}
	return out
}

package studio

import (
	"context"

	"agent-vivy/internal/buildinfo"
	"agent-vivy/internal/domain"
)

const BuiltinGenerationID = "builtin"

// LiveView is the running process's first-party identity. Paths and
// secrets must not appear here.
type LiveView struct {
	Provider      string
	PolicyProfile domain.PolicyProfile
	PolicyHash    string
	Tools         []domain.ToolSpec
}

// Report is the species/inspect payload.
type Report struct {
	ProtocolVersion string                `json:"protocol_version"`
	BinaryID        string                `json:"binary_id"`
	GenerationID    string                `json:"generation_id"`
	ArtifactSHA256  string                `json:"artifact_sha256,omitempty"`
	Recipe          domain.AssemblyRecipe `json:"recipe"`
	PolicyProfile   string                `json:"policy_profile"`
	PolicyHash      string                `json:"policy_hash"`
	Tools           []ToolReport          `json:"tools"`
	Grants          []string              `json:"grants"`
}

// ToolReport is a path-free tool identity.
type ToolReport struct {
	Name     string `json:"name"`
	Readonly bool   `json:"readonly"`
}

// Inspect reports the live species. A missing studio store still returns
// the builtin process view.
func (s *Service) Inspect(ctx context.Context, live LiveView) (Report, error) {
	rep := Report{
		BinaryID:      buildinfo.Version,
		GenerationID:  BuiltinGenerationID,
		PolicyProfile: string(live.PolicyProfile),
		PolicyHash:    live.PolicyHash,
		Tools:         []ToolReport{},
		Grants:        []string{},
		Recipe: domain.AssemblyRecipe{
			Loop:      "eino",
			World:     "sandbox",
			Providers: compactStrings(live.Provider),
			Tools:     []string{},
		},
	}
	for _, spec := range live.Tools {
		rep.Tools = append(rep.Tools, ToolReport{Name: spec.Name, Readonly: spec.Readonly})
		rep.Recipe.Tools = append(rep.Recipe.Tools, spec.Name)
	}
	if s == nil || s.store == nil {
		return rep, nil
	}
	promos, err := s.store.ListPromotions(ctx)
	if err != nil {
		return Report{}, err
	}
	for _, p := range promos {
		if p.Phase != domain.PromotionAccepted || p.AppliesAt != domain.PromotionAppliesNextLaunch {
			continue
		}
		gen, genErr := s.store.GetGeneration(ctx, p.ToID)
		if genErr != nil {
			return Report{}, genErr
		}
		rep.GenerationID = gen.ID
		rep.ArtifactSHA256 = gen.ArtifactSHA256
		if hasRecipe(gen.Recipe) {
			rep.Recipe = gen.Recipe
		}
		break
	}
	return rep, nil
}

func compactStrings(v string) []string {
	if v == "" {
		return nil
	}
	return []string{v}
}

func hasRecipe(r domain.AssemblyRecipe) bool {
	return r.Loop != "" || r.World != "" || len(r.Providers) > 0 || len(r.Tools) > 0 || len(r.Plugins) > 0
}

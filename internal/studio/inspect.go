package studio

import (
	"context"

	"agent-vivy/internal/buildinfo"
	"agent-vivy/internal/domain"
	"agent-vivy/sdk/generation"
)

const BuiltinGenerationID = "builtin"

// LiveView is the running process's first-party identity. Paths and
// secrets must not appear here.
type LiveView struct {
	Provider      string
	PolicyProfile domain.PolicyProfile
	Tools         []domain.ToolSpec
}

// Report is the species/inspect payload.
type Report struct {
	ProtocolVersion string                `json:"protocol_version"`
	BinaryID        string                `json:"binary_id"`
	GenerationID    string                `json:"generation_id"`
	Recipe          domain.AssemblyRecipe `json:"recipe"`
	PolicyProfile   string                `json:"policy_profile"`
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
		Tools:         []ToolReport{},
		Grants:        []string{},
		Recipe: domain.AssemblyRecipe{
			Loop:      "eino",
			World:     "sandbox",
			Providers: compactStrings(live.Provider),
			Tools:     []string{},
		},
	}
	// A packed species carries the compiler-sealed Generation Manifest in the
	// executable. Project only final identities from that manifest; never
	// derive them from planned Recipe inputs or from browser-generated source.
	sealedGenerationID := sealedIdentity()
	if sealedGenerationID != "" {
		rep.GenerationID = sealedGenerationID
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
		if hasRecipe(gen.Recipe) {
			rep.Recipe = gen.Recipe
		}
		break
	}
	return rep, nil
}

func sealedIdentity() string {
	raw, err := generation.EmbeddedManifest()
	if err != nil {
		return ""
	}
	id, err := generation.InspectManifestIdentity(raw)
	if err != nil {
		return ""
	}
	return id
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

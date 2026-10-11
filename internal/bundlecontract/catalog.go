package bundlecontract

import (
	"encoding/json"
	"sort"
)

// ComponentRecord holds literal prepared data. Authority, processes, clients,
// credentials and mutable filesystem paths never enter a captured View.
type ComponentRecord struct {
	Ref               ComponentRef   `json:"ref"`
	ProjectedID       string         `json:"projected_id"`
	Release           ReleaseRef     `json:"release"`
	ContentDigest     string         `json:"content_digest"`
	DisplayRef        string         `json:"display_ref"`
	Name              string         `json:"name"`
	Description       string         `json:"description"`
	Body              string         `json:"body"`
	Resources         []string       `json:"resources"`
	RecommendedSkills []ComponentRef `json:"recommended_skills"`
	Faces             []string       `json:"faces"`
	Hook              *HookEntry     `json:"hook,omitempty"`
	MCP               *MCPEntry      `json:"mcp,omitempty"`
	ExecutableDigest  string         `json:"executable_digest,omitempty"`
	Available         bool           `json:"available"`
	Status            string         `json:"status"`
}
type Diagnostic struct {
	Code string        `json:"code"`
	Ref  *ComponentRef `json:"ref,omitempty"`
}
type View struct {
	identity    CatalogIdentity
	releases    []ReleaseRef
	components  []ComponentRecord
	diagnostics []Diagnostic
}

func NewView(identity CatalogIdentity, releases []ReleaseRef, components []ComponentRecord, diagnostics []Diagnostic) (View, error) {
	known := map[string]ReleaseRef{}
	for _, r := range releases {
		if !ValidInstallID(r.InstallID) || !bundleIDPattern.MatchString(r.BundleID) || !validDigest(r.ReleaseDigest) || r.ReleaseSequence <= 0 {
			return View{}, Invalid("release")
		}
		if _, ok := known[r.InstallID]; ok {
			return View{}, Invalid("duplicate_release")
		}
		known[r.InstallID] = r
	}
	seen := map[string]bool{}
	for _, c := range components {
		r, ok := known[c.Ref.InstallID]
		if !ok || r != c.Release || !localIDPattern.MatchString(c.Ref.LocalID) {
			return View{}, Invalid("component_ref")
		}
		var id string
		var err error
		switch c.Ref.Kind {
		case Skill, MCP:
			id, err = SkillID(c.Ref.InstallID, c.Ref.LocalID)
		case Mask:
			id, err = MaskID(c.Ref.InstallID, c.Ref.LocalID)
		case Rule, Hook:
			id, err = SkillID(c.Ref.InstallID, c.Ref.LocalID)
		default:
			return View{}, Invalid("component_kind")
		}
		key := string(c.Ref.Kind) + "/" + c.ProjectedID
		if err != nil || id != c.ProjectedID || seen[key] {
			return View{}, Invalid("duplicate_projection")
		}
		seen[key] = true
	}
	v := View{identity: identity, releases: cloneValues(releases), components: cloneValues(components), diagnostics: cloneValues(diagnostics)}
	sort.Slice(v.releases, func(i, j int) bool { return v.releases[i].InstallID < v.releases[j].InstallID })
	sort.Slice(v.components, func(i, j int) bool {
		a, b := v.components[i], v.components[j]
		if a.Ref.Kind != b.Ref.Kind {
			return a.Ref.Kind < b.Ref.Kind
		}
		return a.ProjectedID < b.ProjectedID
	})
	return v, nil
}
func cloneValues[T any](in []T) []T {
	if in == nil {
		return []T{}
	}
	b, _ := json.Marshal(in)
	var out []T
	_ = json.Unmarshal(b, &out)
	return out
}
func (v View) Identity() CatalogIdentity     { return v.identity }
func (v View) Releases() []ReleaseRef        { return cloneValues(v.releases) }
func (v View) Components() []ComponentRecord { return cloneValues(v.components) }
func (v View) Diagnostics() []Diagnostic     { return cloneValues(v.diagnostics) }
func (v View) Lookup(kind ComponentKind, id string) (ComponentRecord, bool) {
	for _, c := range v.components {
		if c.Ref.Kind == kind && c.ProjectedID == id {
			return cloneValues([]ComponentRecord{c})[0], true
		}
	}
	return ComponentRecord{}, false
}

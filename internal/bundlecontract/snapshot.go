package bundlecontract

import (
	"bytes"
	"encoding/json"
	"sort"
)

// CapturedComponent records stable provenance for one component without
// copying large Skill bodies or executable data into a model-visible record.
type CapturedComponent struct {
	Ref              ComponentRef `json:"ref"`
	ProjectedID      string       `json:"projected_id"`
	Release          ReleaseRef   `json:"release"`
	ContentDigest    string       `json:"content_digest"`
	ExecutableDigest string       `json:"executable_digest,omitempty"`
	FaceScope        []string     `json:"face_scope,omitempty"`
	ConsentPresent   bool         `json:"consent_present,omitempty"`
	ConsentDigest    string       `json:"consent_digest,omitempty"`
	ConsentAllowed   bool         `json:"consent_allowed,omitempty"`
	ConsentRevision  int64        `json:"consent_revision,omitempty"`
	Bindings         []BindingRef `json:"bindings,omitempty"`
}

// RunSnapshotPayload is immutable execution evidence for one published
// catalog. Prompt-specific Mask and Rule bytes remain in RunPromptSnapshot.
type RunSnapshotPayload struct {
	Catalog            CatalogIdentity     `json:"catalog"`
	RevocationRevision int64               `json:"revocation_revision"`
	Releases           []ReleaseRef        `json:"releases"`
	Components         []CapturedComponent `json:"components"`
}

func SnapshotPayload(view View, revocationRevision int64) RunSnapshotPayload {
	p := RunSnapshotPayload{
		Catalog: view.Identity(), RevocationRevision: revocationRevision,
		Releases: view.Releases(), Components: []CapturedComponent{},
	}
	for _, c := range view.Components() {
		p.Components = append(p.Components, CapturedComponent{
			Ref: c.Ref, ProjectedID: c.ProjectedID, Release: c.Release,
			ContentDigest: c.ContentDigest, ExecutableDigest: c.ExecutableDigest,
			FaceScope: append([]string(nil), c.Faces...), Bindings: append([]BindingRef(nil), c.Bindings...),
		})
	}
	sort.Slice(p.Components, func(i, j int) bool {
		a, b := p.Components[i], p.Components[j]
		if a.Ref.Kind != b.Ref.Kind {
			return a.Ref.Kind < b.Ref.Kind
		}
		return a.ProjectedID < b.ProjectedID
	})
	return p
}

func CaptureCheckFor(payload RunSnapshotPayload) CaptureCheck {
	check := CaptureCheck{Catalog: payload.Catalog, RevocationRevision: payload.RevocationRevision, Executables: []ExecutableRef{}, Consents: []ConsentRef{}}
	for _, c := range payload.Components {
		if c.Ref.Kind == Hook || c.Ref.Kind == MCP {
			check.Consents = append(check.Consents, ConsentRef{
				ComponentRef: c.Ref, Present: c.ConsentPresent, DeclarationDigest: c.ConsentDigest,
				Allowed: c.ConsentAllowed, Revision: c.ConsentRevision,
			})
			if c.ExecutableDigest != "" && c.ConsentPresent && c.ConsentAllowed {
				consentDigest, err := ExecutableConsentDigest(c.Ref.InstallID, c.Release.ReleaseDigest, c.ExecutableDigest, payload.Catalog.ConfigRevision)
				if err == nil && c.ConsentDigest == consentDigest {
					check.Executables = append(check.Executables, ExecutableRef{
						ComponentRef: c.Ref, DeclarationDigest: c.ExecutableDigest,
						ConsentDigest: c.ConsentDigest, ConsentRevision: c.ConsentRevision,
					})
				}
			}
		} else if c.ExecutableDigest != "" {
			check.Executables = append(check.Executables, ExecutableRef{ComponentRef: c.Ref, DeclarationDigest: c.ExecutableDigest})
		}
	}
	sort.Slice(check.Executables, func(i, j int) bool {
		a, b := check.Executables[i], check.Executables[j]
		if a.InstallID != b.InstallID {
			return a.InstallID < b.InstallID
		}
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		if a.LocalID != b.LocalID {
			return a.LocalID < b.LocalID
		}
		return a.DeclarationDigest < b.DeclarationDigest
	})
	sort.Slice(check.Consents, func(i, j int) bool {
		a, b := check.Consents[i], check.Consents[j]
		if a.InstallID != b.InstallID {
			return a.InstallID < b.InstallID
		}
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		return a.LocalID < b.LocalID
	})
	return check
}

func EncodeRunSnapshotPayload(payload RunSnapshotPayload) ([]byte, error) {
	if err := ValidateRunSnapshotPayload(payload); err != nil {
		return nil, err
	}
	return CanonicalJSON(payload)
}

func DecodeRunSnapshotPayload(raw []byte) (RunSnapshotPayload, error) {
	var payload RunSnapshotPayload
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&payload); err != nil {
		return RunSnapshotPayload{}, Invalid("snapshot_json")
	}
	canonical, err := EncodeRunSnapshotPayload(payload)
	if err != nil || !bytes.Equal(canonical, raw) {
		return RunSnapshotPayload{}, Invalid("snapshot_canonical")
	}
	return payload, nil
}

func ValidateRunSnapshotPayload(p RunSnapshotPayload) error {
	if p.RevocationRevision < 0 || p.Releases == nil || p.Components == nil {
		return Invalid("snapshot_shape")
	}
	if p.Catalog.Epoch < 0 || p.Catalog.ConfigRevision < 0 ||
		(p.Catalog.Epoch == 0 && (p.Catalog.EffectiveDigest != "" || p.Catalog.GenerationID != "")) ||
		(p.Catalog.Epoch > 0 && !validDigest(p.Catalog.EffectiveDigest)) {
		return Invalid("snapshot_catalog")
	}
	releases := map[string]ReleaseRef{}
	for i, r := range p.Releases {
		if !ValidInstallID(r.InstallID) || !bundleIDPattern.MatchString(r.BundleID) || !validDigest(r.ReleaseDigest) || r.ReleaseSequence <= 0 {
			return Invalid("snapshot_release")
		}
		if i > 0 && p.Releases[i-1].InstallID >= r.InstallID {
			return Invalid("snapshot_release_order")
		}
		releases[r.InstallID] = r
	}
	seen := map[string]struct{}{}
	for i, c := range p.Components {
		if !localIDPattern.MatchString(c.Ref.LocalID) || c.ProjectedID == "" || !validDigest(c.ContentDigest) {
			return Invalid("snapshot_component")
		}
		if c.ExecutableDigest != "" && !validDigest(c.ExecutableDigest) {
			return Invalid("snapshot_executable")
		}
		if c.Ref.Kind != Hook && c.Ref.Kind != MCP && (c.ConsentPresent || c.ConsentDigest != "" || c.ConsentAllowed || c.ConsentRevision != 0) {
			return Invalid("snapshot_component_consent")
		}
		if c.ConsentPresent {
			if (c.Ref.Kind != Hook && c.Ref.Kind != MCP) || c.ConsentRevision <= 0 || !validDigest(c.ConsentDigest) {
				return Invalid("snapshot_component_consent")
			}
		} else if c.ConsentDigest != "" || c.ConsentAllowed || c.ConsentRevision != 0 {
			return Invalid("snapshot_component_consent")
		}
		if c.Ref.Kind != Rule && len(c.FaceScope) > 0 {
			return Invalid("snapshot_component_face_scope")
		}
		if c.Ref.Kind != Hook && c.Ref.Kind != MCP && len(c.Bindings) > 0 {
			return Invalid("snapshot_component_bindings")
		}
		for bindingIndex, binding := range c.Bindings {
			if !ValidBindingName(binding.Name) || !ValidBindingReference(binding.Reference) ||
				(bindingIndex > 0 && c.Bindings[bindingIndex-1].Name >= binding.Name) {
				return Invalid("snapshot_component_bindings")
			}
		}
		for faceIndex, face := range c.FaceScope {
			if face != "web" && face != "code" && face != "tui" && face != "headless" {
				return Invalid("snapshot_component_face_scope")
			}
			if faceIndex > 0 && c.FaceScope[faceIndex-1] >= face {
				return Invalid("snapshot_component_face_scope_order")
			}
		}
		if release, ok := releases[c.Ref.InstallID]; !ok || release != c.Release {
			return Invalid("snapshot_component_release")
		}
		var expected string
		switch c.Ref.Kind {
		case Skill, Rule, Hook, MCP:
			expected, _ = SkillID(c.Ref.InstallID, c.Ref.LocalID)
		case Mask:
			expected, _ = MaskID(c.Ref.InstallID, c.Ref.LocalID)
		default:
			return Invalid("snapshot_component_kind")
		}
		if expected != c.ProjectedID {
			return Invalid("snapshot_component_identity")
		}
		key := string(c.Ref.Kind) + "/" + c.ProjectedID
		if _, dup := seen[key]; dup {
			return Invalid("snapshot_component_duplicate")
		}
		seen[key] = struct{}{}
		if i > 0 {
			prev := p.Components[i-1]
			if prev.Ref.Kind > c.Ref.Kind || (prev.Ref.Kind == c.Ref.Kind && prev.ProjectedID >= c.ProjectedID) {
				return Invalid("snapshot_component_order")
			}
		}
	}
	return nil
}

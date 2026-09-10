package assembly

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"sort"

	"agent-vivy/sdk/module"
)

type CapabilityState string

const (
	CapabilityNotCompiled  CapabilityState = "NOT_COMPILED"
	CapabilityUnconfigured CapabilityState = "UNCONFIGURED"
	CapabilityInactive     CapabilityState = "INACTIVE"
	CapabilityReady        CapabilityState = "READY"
	CapabilityUnavailable  CapabilityState = "UNAVAILABLE"
	CapabilitySpecified    CapabilityState = "SPECIFIED"
	CapabilityDeferred     CapabilityState = "DEFERRED_INDEFINITE"
)

func CapabilityStates() []CapabilityState {
	return []CapabilityState{
		CapabilityNotCompiled,
		CapabilityUnconfigured,
		CapabilityInactive,
		CapabilityReady,
		CapabilityUnavailable,
		CapabilitySpecified,
		CapabilityDeferred,
	}
}

type CatalogManifest struct {
	Module        string            `json:"module"`
	SchemaVersion string            `json:"schemaVersion"`
	Path          string            `json:"path"`
	Digest        string            `json:"digest"`
	DefaultLocale string            `json:"defaultLocale"`
	Locales       []string          `json:"locales"`
	Completeness  map[string]string `json:"completeness,omitempty"`
	Evidence      []string          `json:"evidence,omitempty"`
}

type SealInputs struct {
	SpecificationVersion string
	CompilerVersion      string
	SDKVersion           string
	CanonicalRecipe      []byte
	DependencyLocks      map[string]string
	UIArtifacts          map[string]string
	Catalogs             []CatalogManifest
	CapabilityStates     map[string]CapabilityState
}

type ManifestModule struct {
	ID              string               `json:"id"`
	Version         string               `json:"version"`
	Source          module.Source        `json:"source"`
	Trust           Trust                `json:"trust"`
	Provides        []module.PortRef     `json:"provides"`
	Requires        []module.Requirement `json:"requires"`
	EffectiveGrants []EffectiveGrant     `json:"effectiveGrants"`
}

type GenerationManifest struct {
	GenerationID         string                     `json:"generationId"`
	SpecificationVersion string                     `json:"specificationVersion"`
	CompilerVersion      string                     `json:"compilerVersion"`
	SDKVersion           string                     `json:"sdkVersion"`
	RecipeDigest         string                     `json:"recipeDigest"`
	Modules              []ManifestModule           `json:"modules"`
	PortEdges            []PortEdge                 `json:"portEdges"`
	LifecycleOrder       []string                   `json:"lifecycleOrder"`
	OrderedContributions map[string][]string        `json:"orderedContributions"`
	DependencyLocks      map[string]string          `json:"dependencyLocks"`
	UIArtifacts          map[string]string          `json:"uiArtifacts"`
	Catalogs             []CatalogManifest          `json:"catalogs"`
	CapabilityStates     map[string]CapabilityState `json:"capabilityStates,omitempty"`
}

func CanonicalRecipe(recipe Recipe) ([]byte, error) {
	canonical := recipe
	canonical.Modules = append([]string(nil), recipe.Modules...)
	sort.Strings(canonical.Modules)
	canonical.Exclusive = cloneStringMap(recipe.Exclusive)
	canonical.Order = cloneStringSliceMap(recipe.Order)
	canonical.GrantApprovals = append([]GrantApproval(nil), recipe.GrantApprovals...)
	for index := range canonical.GrantApprovals {
		constraints, err := canonicalConstraints(canonical.GrantApprovals[index].Constraints)
		if err != nil {
			return nil, err
		}
		canonical.GrantApprovals[index].Constraints = constraints
		canonical.GrantApprovals[index].Evidence = canonicalStrings(canonical.GrantApprovals[index].Evidence)
	}
	sort.Slice(canonical.GrantApprovals, func(i, j int) bool {
		left := canonical.GrantApprovals[i]
		right := canonical.GrantApprovals[j]
		if left.Module == right.Module {
			return left.Name < right.Name
		}
		return left.Module < right.Module
	})
	return json.Marshal(canonical)
}

func SealManifest(plan AssemblyPlan, inputs SealInputs) (GenerationManifest, []byte, error) {
	if inputs.SpecificationVersion == "" || inputs.CompilerVersion == "" || inputs.SDKVersion == "" {
		return GenerationManifest{}, nil, fmt.Errorf("seal inputs require specification, compiler, and SDK versions")
	}
	if !json.Valid(inputs.CanonicalRecipe) {
		return GenerationManifest{}, nil, fmt.Errorf("canonical Recipe is not valid JSON")
	}
	recipeDigest := sha256.Sum256(inputs.CanonicalRecipe)
	manifest := GenerationManifest{
		SpecificationVersion: inputs.SpecificationVersion,
		CompilerVersion:      inputs.CompilerVersion,
		SDKVersion:           inputs.SDKVersion,
		RecipeDigest:         hex.EncodeToString(recipeDigest[:]),
		PortEdges:            append([]PortEdge(nil), plan.PortEdges...),
		LifecycleOrder:       append([]string(nil), plan.LifecycleOrder...),
		OrderedContributions: cloneStringSliceMap(plan.OrderedContributions),
		DependencyLocks:      cloneStringMap(inputs.DependencyLocks),
		UIArtifacts:          cloneStringMap(inputs.UIArtifacts),
		Catalogs:             append([]CatalogManifest(nil), inputs.Catalogs...),
		CapabilityStates:     cloneStringMap(inputs.CapabilityStates),
	}
	if err := validateCapabilityStates(manifest.CapabilityStates); err != nil {
		return GenerationManifest{}, nil, err
	}
	for _, resolved := range plan.Modules {
		descriptor := canonicalDescriptor(resolved.Descriptor)
		manifest.Modules = append(manifest.Modules, ManifestModule{
			ID: descriptor.Module.ID, Version: descriptor.Module.Version,
			Source: descriptor.Source, Trust: resolved.Trust,
			Provides: descriptor.Provides, Requires: descriptor.Requires,
			EffectiveGrants: cloneEffectiveGrants(resolved.EffectiveGrants),
		})
	}
	sort.Slice(manifest.Modules, func(i, j int) bool { return manifest.Modules[i].ID < manifest.Modules[j].ID })
	sort.Slice(manifest.PortEdges, func(i, j int) bool {
		left := manifest.PortEdges[i]
		right := manifest.PortEdges[j]
		return left.Port.Port+"\x00"+left.Provider+"\x00"+left.Consumer < right.Port.Port+"\x00"+right.Provider+"\x00"+right.Consumer
	})
	for index := range manifest.Catalogs {
		manifest.Catalogs[index].Locales = canonicalStrings(manifest.Catalogs[index].Locales)
		manifest.Catalogs[index].Evidence = canonicalStrings(manifest.Catalogs[index].Evidence)
		manifest.Catalogs[index].Completeness = cloneStringMap(manifest.Catalogs[index].Completeness)
	}
	sort.Slice(manifest.Catalogs, func(i, j int) bool { return manifest.Catalogs[i].Module < manifest.Catalogs[j].Module })

	identityBytes, err := json.Marshal(manifest)
	if err != nil {
		return GenerationManifest{}, nil, err
	}
	identity := sha256.Sum256(identityBytes)
	manifest.GenerationID = hex.EncodeToString(identity[:])
	canonicalBytes, err := json.Marshal(manifest)
	if err != nil {
		return GenerationManifest{}, nil, err
	}
	return manifest, canonicalBytes, nil
}

// InspectManifest parses a sealed manifest and recomputes its content address
// before exposing any provenance or capability state.
func InspectManifest(raw []byte) (GenerationManifest, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var manifest GenerationManifest
	if err := decoder.Decode(&manifest); err != nil {
		return GenerationManifest{}, fmt.Errorf("inspect Generation Manifest: %w", err)
	}
	if err := requireManifestEOF(decoder); err != nil {
		return GenerationManifest{}, fmt.Errorf("inspect Generation Manifest: %w", err)
	}
	if err := validateCapabilityStates(manifest.CapabilityStates); err != nil {
		return GenerationManifest{}, err
	}
	want := manifest.GenerationID
	manifest.GenerationID = ""
	identityBytes, err := json.Marshal(manifest)
	if err != nil {
		return GenerationManifest{}, err
	}
	identity := sha256.Sum256(identityBytes)
	got := hex.EncodeToString(identity[:])
	if want == "" || got != want {
		return GenerationManifest{}, fmt.Errorf("Generation Manifest identity mismatch")
	}
	manifest.GenerationID = want
	return manifest, nil
}

func validateCapabilityStates(states map[string]CapabilityState) error {
	valid := make(map[CapabilityState]struct{})
	for _, state := range CapabilityStates() {
		valid[state] = struct{}{}
	}
	keys := make([]string, 0, len(states))
	for key := range states {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if _, ok := valid[states[key]]; !ok {
			return fmt.Errorf("unknown capability state %q for %s", states[key], key)
		}
	}
	return nil
}

func requireManifestEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("multiple JSON values are forbidden")
		}
		return err
	}
	return nil
}

func canonicalDescriptor(descriptor module.Descriptor) module.Descriptor {
	descriptor.Provides = append([]module.PortRef(nil), descriptor.Provides...)
	sort.Slice(descriptor.Provides, func(i, j int) bool {
		return descriptor.Provides[i].Port+"\x00"+descriptor.Provides[i].ID < descriptor.Provides[j].Port+"\x00"+descriptor.Provides[j].ID
	})
	descriptor.Requires = append([]module.Requirement(nil), descriptor.Requires...)
	sort.Slice(descriptor.Requires, func(i, j int) bool {
		return requirementKey(descriptor.Requires[i]) < requirementKey(descriptor.Requires[j])
	})
	descriptor.Optional = append([]module.Requirement(nil), descriptor.Optional...)
	sort.Slice(descriptor.Optional, func(i, j int) bool {
		return requirementKey(descriptor.Optional[i]) < requirementKey(descriptor.Optional[j])
	})
	descriptor.Conflicts = append([]module.Conflict(nil), descriptor.Conflicts...)
	sort.Slice(descriptor.Conflicts, func(i, j int) bool { return descriptor.Conflicts[i].Module < descriptor.Conflicts[j].Module })
	descriptor.RequestedGrants = append([]module.Grant(nil), descriptor.RequestedGrants...)
	sort.Slice(descriptor.RequestedGrants, func(i, j int) bool { return descriptor.RequestedGrants[i] < descriptor.RequestedGrants[j] })
	descriptor.Lifecycle.After = canonicalStrings(descriptor.Lifecycle.After)
	return descriptor
}

func requirementKey(requirement module.Requirement) string {
	return requirement.Port + "\x00" + requirement.Provider + "\x00" + requirement.ID
}

func cloneEffectiveGrants(grants []EffectiveGrant) []EffectiveGrant {
	cloned := append([]EffectiveGrant(nil), grants...)
	for index := range cloned {
		cloned[index].Constraints = cloneStringSliceMap(cloned[index].Constraints)
	}
	sort.Slice(cloned, func(i, j int) bool { return cloned[i].Name < cloned[j].Name })
	return cloned
}

func canonicalStrings(values []string) []string {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		set[value] = struct{}{}
	}
	return sortedSet(set)
}

func cloneStringMap[T ~string](source map[string]T) map[string]T {
	if source == nil {
		return nil
	}
	cloned := make(map[string]T, len(source))
	for key, value := range source {
		cloned[key] = value
	}
	return cloned
}

func cloneStringSliceMap(source map[string][]string) map[string][]string {
	if source == nil {
		return nil
	}
	cloned := make(map[string][]string, len(source))
	for key, value := range source {
		cloned[key] = append([]string(nil), value...)
	}
	return cloned
}

func canonicalJSON(raw []byte) ([]byte, error) {
	var value any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	return json.Marshal(value)
}

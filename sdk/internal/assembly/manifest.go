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
	Module string `json:"module"`
	// APIVersion and Units are the compiler-owned catalog projection consumed
	// by the generated Web/TUI hosts. The metadata fields remain available to
	// Inspect and the digest binds this exact canonical projection.
	APIVersion    string                 `json:"apiVersion,omitempty"`
	SchemaVersion string                 `json:"schemaVersion"`
	Path          string                 `json:"path"`
	Digest        string                 `json:"digest"`
	DefaultLocale string                 `json:"defaultLocale"`
	Locales       []string               `json:"locales"`
	Completeness  map[string]string      `json:"completeness,omitempty"`
	Evidence      []string               `json:"evidence,omitempty"`
	Units         map[string]CatalogUnit `json:"units,omitempty"`
}

// CatalogUnit is the shared Web/TUI translation-unit shape. It deliberately
// contains no runtime loading metadata: generation embeds the validated
// projection and PresentationHost only reads these values.
type CatalogUnit struct {
	Description  string            `json:"description"`
	Placeholders []string          `json:"placeholders"`
	Messages     map[string]string `json:"messages"`
	Short        map[string]string `json:"short,omitempty"`
	Long         map[string]string `json:"long,omitempty"`
}

type SealInputs struct {
	SpecificationVersion string
	CompilerVersion      string
	SDKVersion           string
	CanonicalRecipe      []byte
	DependencyLocks      map[string]string
	UIArtifacts          map[string]string
	// UI is the generated full-code UI projection. Keeping it in the sealed
	// manifest (rather than only returning it from GenerateUIAssembly) binds
	// root selection, extension order, replacement relationships, SDK pin, and
	// all UI content identities to the Generation ID consumed by Inspect.
	UI               *UIAssemblyManifest
	Catalogs         []CatalogManifest
	CapabilityStates map[string]CapabilityState
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
	UI                   *UIAssemblyManifest        `json:"ui,omitempty"`
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
	canonical.UI = cloneUIAssemblyInput(recipe.UI)
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
		UI:                   cloneUIAssemblyManifest(inputs.UI),
		Catalogs:             cloneCatalogManifests(inputs.Catalogs),
		CapabilityStates:     cloneStringMap(inputs.CapabilityStates),
	}
	manifest.Catalogs = canonicalizeCatalogManifests(manifest.Catalogs)
	if manifest.UI != nil {
		manifest.UI.Catalogs = canonicalizeCatalogManifests(manifest.UI.Catalogs)
	}
	if manifest.UI != nil {
		if len(manifest.Catalogs) == 0 && len(manifest.UI.Catalogs) > 0 {
			manifest.Catalogs = cloneCatalogManifests(manifest.UI.Catalogs)
		} else if len(manifest.Catalogs) != len(manifest.UI.Catalogs) {
			return GenerationManifest{}, nil, fmt.Errorf("Generation Manifest catalog projection differs from UI Assembly")
		} else if len(manifest.UI.Catalogs) > 0 {
			leftCatalogs, leftErr := normalizeCatalogManifests(manifest.Catalogs)
			rightCatalogs, rightErr := normalizeCatalogManifests(manifest.UI.Catalogs)
			left, marshalLeftErr := json.Marshal(leftCatalogs)
			right, marshalRightErr := json.Marshal(rightCatalogs)
			if leftErr != nil || rightErr != nil || marshalLeftErr != nil || marshalRightErr != nil || !bytes.Equal(left, right) {
				return GenerationManifest{}, nil, fmt.Errorf("Generation Manifest catalog projection differs from UI Assembly")
			}
			manifest.Catalogs = leftCatalogs
			manifest.UI.Catalogs = rightCatalogs
		}
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

// canonicalizeCatalogManifests makes the projection stable before it is used
// in either the Generation Manifest or its UI mirror. It intentionally does
// not rewrite the digest-bound translation body; body validation remains the
// compiler-owned responsibility of normalizeCatalogManifests.
func canonicalizeCatalogManifests(source []CatalogManifest) []CatalogManifest {
	cloned := cloneCatalogManifests(source)
	for index := range cloned {
		cloned[index].Locales = canonicalStrings(cloned[index].Locales)
		cloned[index].Evidence = canonicalStrings(cloned[index].Evidence)
		cloned[index].Completeness = cloneStringMap(cloned[index].Completeness)
	}
	sort.SliceStable(cloned, func(left, right int) bool {
		return cloned[left].Module < cloned[right].Module
	})
	return cloned
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
	if err := validateInspectedCatalogs(manifest); err != nil {
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

// validateInspectedCatalogs protects the trust boundary used by studio
// Inspect and embedded-manifest provenance. A syntactically valid JSON object
// is not enough: every catalog body must still be the compiler-validated,
// namespace-confined projection whose digest is sealed in the manifest.
func validateInspectedCatalogs(manifest GenerationManifest) error {
	if len(manifest.Catalogs) > 0 {
		if _, err := normalizeCatalogManifests(manifest.Catalogs); err != nil {
			return fmt.Errorf("inspect Generation Manifest catalogs: %w", err)
		}
	}
	if manifest.UI == nil || len(manifest.UI.Catalogs) == 0 {
		return nil
	}
	if _, err := normalizeCatalogManifests(manifest.UI.Catalogs); err != nil {
		return fmt.Errorf("inspect UI Assembly catalogs: %w", err)
	}
	if len(manifest.Catalogs) == 0 {
		return fmt.Errorf("inspect Generation Manifest catalogs: UI catalog projection is not embedded in Generation Manifest")
	}
	left, leftErr := json.Marshal(manifest.Catalogs)
	right, rightErr := json.Marshal(manifest.UI.Catalogs)
	if leftErr != nil || rightErr != nil || !bytes.Equal(left, right) {
		return fmt.Errorf("inspect Generation Manifest catalogs: UI and Generation projections differ")
	}
	return nil
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

func cloneCatalogUnits(source map[string]CatalogUnit) map[string]CatalogUnit {
	if source == nil {
		return nil
	}
	cloned := make(map[string]CatalogUnit, len(source))
	for key, unit := range source {
		cloned[key] = CatalogUnit{
			Description:  unit.Description,
			Placeholders: cloneStringSlice(unit.Placeholders),
			Messages:     cloneStringMap(unit.Messages),
			Short:        cloneStringMap(unit.Short),
			Long:         cloneStringMap(unit.Long),
		}
	}
	return cloned
}

func cloneStringSlice(source []string) []string {
	if source == nil {
		return nil
	}
	return append([]string{}, source...)
}

func cloneCatalogManifest(source CatalogManifest) CatalogManifest {
	return CatalogManifest{
		Module:        source.Module,
		APIVersion:    source.APIVersion,
		SchemaVersion: source.SchemaVersion,
		Path:          source.Path,
		Digest:        source.Digest,
		DefaultLocale: source.DefaultLocale,
		Locales:       append([]string(nil), source.Locales...),
		Completeness:  cloneStringMap(source.Completeness),
		Evidence:      append([]string(nil), source.Evidence...),
		Units:         cloneCatalogUnits(source.Units),
	}
}

func cloneUIAssemblyManifest(source *UIAssemblyManifest) *UIAssemblyManifest {
	if source == nil {
		return nil
	}
	cloned := &UIAssemblyManifest{
		Root:                 source.Root,
		Extensions:           append([]string(nil), source.Extensions...),
		Replacements:         cloneStringSliceMap(source.Replacements),
		SourceHashes:         cloneStringMap(source.SourceHashes),
		DependencyLockHashes: cloneStringMap(source.DependencyLockHashes),
		SDKPackage:           source.SDKPackage,
		SDKVersion:           source.SDKVersion,
		AssetHashes:          cloneStringMap(source.AssetHashes),
		Catalogs:             cloneCatalogManifests(source.Catalogs),
	}
	return cloned
}

func cloneCatalogManifests(source []CatalogManifest) []CatalogManifest {
	if source == nil {
		return nil
	}
	cloned := make([]CatalogManifest, len(source))
	for index, catalog := range source {
		cloned[index] = cloneCatalogManifest(catalog)
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

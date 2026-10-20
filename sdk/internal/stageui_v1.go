package sdk

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	assemblyv1 "agent-vivy/sdk/internal/assembly"
	"agent-vivy/sdk/port"
	"gopkg.in/yaml.v3"
)

// stagedUIAssemblyFile is the generated entry consumed by the checked-in Vite
// configuration. It uses the same projection as Pack with an authoring identity.
const stagedUIAssemblyFile = "assembly.ts"

// StageUIReport is the machine-readable outcome of one staging run.
type StageUIReport struct {
	Recipe     string   `json:"recipe"`
	Output     string   `json:"output"`
	Assembly   string   `json:"assembly"`
	SDKVersion string   `json:"sdkVersion"`
	Extensions []string `json:"extensions"`
}

// StageUI renders the repository's dev/build UI projection from one Recipe.
//
// It is the unsealed twin of Pack's UI input: the same Recipe, the same
// Assembly generator, and the same flat source staging, so a Module entry path
// means the same thing in the inner loop and in a sealed Generation. It builds
// no Go artifact and runs no Vite build; its authoring identity stays stable
// until Pack supplies a fresh build-owned identity.
//
// A Recipe that selects no UI Module still writes the canonical empty
// Assembly, so a UI-less profile remains buildable instead of failing.
func StageUI(repoRoot, recipePath, outputDir string, generationIDs ...string) (StageUIReport, error) {
	root, err := filepath.Abs(repoRoot)
	if err != nil {
		return StageUIReport{}, err
	}
	recipeFile, err := filepath.Abs(recipePath)
	if err != nil {
		return StageUIReport{}, err
	}
	output, err := filepath.Abs(outputDir)
	if err != nil {
		return StageUIReport{}, err
	}
	raw, err := os.ReadFile(recipeFile)
	if err != nil {
		return StageUIReport{}, err
	}
	decoder := yaml.NewDecoder(bytes.NewReader(raw))
	decoder.KnownFields(true)
	var recipe assemblyv1.Recipe
	if err := decoder.Decode(&recipe); err != nil {
		return StageUIReport{}, fmt.Errorf("sdk: parse Recipe: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return StageUIReport{}, errors.New("sdk: Recipe must contain one YAML document")
	}

	records, err := sourceRecords(root, nil, recipe.Sources)
	if err != nil {
		return StageUIReport{}, err
	}
	catalog, err := assemblyv1.NewSourceCatalog(records)
	if err != nil {
		return StageUIReport{}, err
	}
	evidence := assemblyv1.SupportedPortEvidence()
	conformanceResults := assemblyv1.SupportedPortConformance()
	plan, err := (assemblyv1.Compiler{Ports: port.PublicCatalog(), Sources: catalog, PortEvidence: evidence, ConformanceResults: conformanceResults}).Compile(context.Background(), recipe)
	if err != nil {
		return StageUIReport{}, err
	}

	var catalogs []assemblyv1.CatalogManifest
	for _, resolved := range plan.Modules {
		record, resolveErr := catalog.Resolve(resolved.Descriptor.Module.ID)
		if resolveErr != nil {
			return StageUIReport{}, resolveErr
		}
		compiled, catalogErr := loadCatalog(record.Root, resolved.Descriptor)
		if catalogErr != nil {
			return StageUIReport{}, catalogErr
		}
		if compiled.State == catalogNotApplicable {
			continue
		}
		completeness := make(map[string]string, len(compiled.Completeness))
		for locale, state := range compiled.Completeness {
			completeness[locale] = string(state)
		}
		var projection struct {
			Units map[string]assemblyv1.CatalogUnit `json:"units"`
		}
		if err := json.Unmarshal(compiled.Canonical, &projection); err != nil {
			return StageUIReport{}, fmt.Errorf("sdk: decode i18n catalog projection for %s: %w", resolved.Descriptor.Module.ID, err)
		}
		catalogs = append(catalogs, assemblyv1.CatalogManifest{Module: resolved.Descriptor.Module.ID, APIVersion: compiled.SchemaVersion, SchemaVersion: compiled.SchemaVersion, Path: compiled.Path, DefaultLocale: compiled.DefaultLocale, Locales: compiled.Locales, Completeness: completeness, CompilationState: string(compiled.State), Evidence: []string{string(compiled.State)}, Units: projection.Units})
	}

	uiInput := assemblyv1.UIAssemblyInput{SDKVersion: assemblyv1.UIAssemblySDKVersion, Catalogs: catalogs}
	if recipe.UI != nil {
		uiInput = *recipe.UI
		uiInput.Catalogs = catalogs
	}
	uiInput.GenerationID = "authoring-" + recipe.Profile
	if len(generationIDs) > 1 {
		return StageUIReport{}, fmt.Errorf("one Generation ID is required")
	}
	if len(generationIDs) == 1 {
		uiInput.GenerationID = generationIDs[0]
	}
	if err := applyRecipeUIOrder(&uiInput, recipe.Order); err != nil {
		return StageUIReport{}, fmt.Errorf("sdk: configure UI Assembly order: %w", err)
	}
	if err := bindUIBuildDependencies(&uiInput, root, plan, catalog); err != nil {
		return StageUIReport{}, err
	}
	uiAssembly, err := assemblyv1.GenerateUIAssembly(uiInput)
	if err != nil {
		return StageUIReport{}, fmt.Errorf("sdk: generate UI Assembly: %w", err)
	}
	if err := cleanStagedOutput(output); err != nil {
		return StageUIReport{}, err
	}
	if err := os.MkdirAll(output, 0o755); err != nil {
		return StageUIReport{}, err
	}
	// Staging copies every selected Module source into one flat boundary, so
	// the Recipe entry path resolves identically here and in `pack`.
	if err := stageUIBuildSources(output, uiInput, plan, catalog); err != nil {
		return StageUIReport{}, err
	}
	entry := filepath.Join(output, stagedUIAssemblyFile)
	if err := os.WriteFile(entry, uiAssembly.Source, 0o644); err != nil {
		return StageUIReport{}, err
	}
	staged := make([]string, 0, len(uiInput.Extensions))
	for _, extension := range uiInput.Extensions {
		id := strings.TrimSpace(extension.ID)
		if id == "" {
			id = strings.TrimSpace(extension.ModuleID)
		}
		staged = append(staged, id)
	}
	return StageUIReport{Recipe: recipeFile, Output: output, Assembly: entry, SDKVersion: uiInput.SDKVersion, Extensions: staged}, nil
}

// cleanStagedOutput removes every previously staged entry so a Module removed
// from the Recipe cannot leave a stale contribution behind. The output
// directory is dedicated to generated content.
func cleanStagedOutput(output string) error {
	if _, err := os.Stat(output); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	entries, err := os.ReadDir(output)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.Name() == stagedUIAssemblyFile {
			continue
		}
		if err := os.RemoveAll(filepath.Join(output, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

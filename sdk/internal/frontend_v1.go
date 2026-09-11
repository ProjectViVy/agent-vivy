package sdk

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"agent-vivy/internal/modules/defaults"
	"agent-vivy/sdk/generation"
	assemblyv1 "agent-vivy/sdk/internal/assembly"
	"agent-vivy/sdk/module"
	"agent-vivy/sdk/port"
	"gopkg.in/yaml.v3"
)

type VerifyReport struct {
	Module string `json:"module"`
	OK     bool   `json:"ok"`
}
type Artifact struct {
	Directory string                        `json:"directory"`
	Binary    string                        `json:"binary"`
	Manifest  assemblyv1.GenerationManifest `json:"manifest"`
}

const zeroDigest = "0000000000000000000000000000000000000000000000000000000000000000"

var uiArtifactAssetValuePattern = regexp.MustCompile(`(:\s*")[0-9a-f]{64}(")`)
var uiArtifactHashedFilenamePattern = regexp.MustCompile(`-[A-Za-z0-9_-]{8}(\.[^./]+)$`)
var uiExactDependencyPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$`)
var uiDependencyNamePattern = regexp.MustCompile(`^(?:@[^/\s]+/)?[^/\s]+$`)

func Verify(dir string) (VerifyReport, error) {
	descriptor, _, err := loadDescriptor(dir)
	if err != nil {
		return VerifyReport{}, err
	}
	if _, err := loadCatalog(dir, descriptor); err != nil {
		return VerifyReport{}, err
	}
	if err := verifySource(dir, descriptor); err != nil {
		return VerifyReport{}, err
	}
	if _, _, err := sourceGoBinding(dir); err != nil {
		return VerifyReport{}, err
	}
	return VerifyReport{Module: descriptor.Module.ID, OK: true}, nil
}

type packOptions struct {
	Recipe, Output string
	Sources        []string
}

func parsePackArgs(args []string) (packOptions, error) {
	var o packOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--recipe":
			i++
			if i >= len(args) {
				return o, errors.New("--recipe requires a file")
			}
			o.Recipe = args[i]
		case "--source":
			i++
			if i >= len(args) {
				return o, errors.New("--source requires a directory")
			}
			o.Sources = append(o.Sources, args[i])
		case "--output", "--out":
			i++
			if i >= len(args) {
				return o, errors.New("--output requires a directory")
			}
			o.Output = args[i]
		default:
			return o, fmt.Errorf("unknown pack argument %q", args[i])
		}
	}
	if o.Recipe == "" || o.Output == "" {
		return o, errors.New("pack requires --recipe and --output")
	}
	return o, nil
}

func snapshotSourceDirs(repoRoot string, sources []string) (string, []string, error) {
	if len(sources) == 0 {
		return "", nil, nil
	}
	known := make(map[string]bool)
	for _, rel := range []string{"plugins/dingtalk", "plugins/discord", "plugins/feishu", "plugins/qq", "plugins/telegram", "plugins/hello-fs", "plugins/lsp", "faces/headless", "faces/tui", "sdk/internal/testdata/full-ui-module"} {
		abs, _ := filepath.Abs(filepath.Join(repoRoot, rel))
		known[abs] = true
	}
	root, err := os.MkdirTemp("", "vivy-source-snapshot-")
	if err != nil {
		return "", nil, err
	}
	cleanup := func(cause error) (string, []string, error) {
		_ = os.RemoveAll(root)
		return "", nil, cause
	}
	out := make([]string, 0, len(sources))
	for index, source := range sources {
		abs, err := filepath.Abs(source)
		if err != nil {
			return cleanup(err)
		}
		if known[abs] {
			out = append(out, abs)
			continue
		}
		destination := filepath.Join(root, fmt.Sprintf("source-%d", index))
		if err := copySourceTree(abs, destination); err != nil {
			return cleanup(fmt.Errorf("sdk: snapshot source %s: %w", source, err))
		}
		out = append(out, destination)
	}
	return root, out, nil
}

func copySourceTree(source, destination string) error {
	return filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symbolic link %s is not allowed", path)
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o700)
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("non-regular source entry %s is not allowed", path)
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, body, 0o600)
	})
}

func Pack(ctx context.Context, o packOptions) (Artifact, error) {
	recipeRaw, err := os.ReadFile(o.Recipe)
	if err != nil {
		return Artifact{}, err
	}
	decoder := yaml.NewDecoder(bytes.NewReader(recipeRaw))
	decoder.KnownFields(true)
	var recipe assemblyv1.Recipe
	if err := decoder.Decode(&recipe); err != nil {
		return Artifact{}, fmt.Errorf("sdk: parse Recipe: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return Artifact{}, errors.New("sdk: Recipe must contain one YAML document")
	}
	repoRoot, err := findRepoRoot(filepath.Dir(o.Recipe))
	if err != nil {
		repoRoot, err = findRepoRoot(".")
		if err != nil {
			return Artifact{}, err
		}
	}
	snapshotRoot, buildSources, err := snapshotSourceDirs(repoRoot, o.Sources)
	if err != nil {
		return Artifact{}, err
	}
	if snapshotRoot != "" {
		defer os.RemoveAll(snapshotRoot)
	}
	records, err := sourceRecords(repoRoot, buildSources, recipe.Sources)
	if err != nil {
		return Artifact{}, err
	}
	catalog, err := assemblyv1.NewSourceCatalog(records)
	if err != nil {
		return Artifact{}, err
	}
	evidence := assemblyv1.P1P2PortEvidence()
	for portName, portEvidence := range assemblyv1.P6UIPortEvidence() {
		evidence[portName] = portEvidence
	}
	plan, err := (assemblyv1.Compiler{Ports: port.PublicCatalog(), Sources: catalog, PortEvidence: evidence}).Compile(ctx, recipe)
	if err != nil {
		return Artifact{}, err
	}
	binder, err := assemblyv1.GenerateBinder(plan, "assembly")
	if err != nil {
		return Artifact{}, err
	}
	runtimeSource, err := assemblyv1.GenerateRuntimeAssembly(plan, "assembly")
	if err != nil {
		return Artifact{}, err
	}
	overlayDir, err := os.MkdirTemp("", "vivy-pack-overlay-")
	if err != nil {
		return Artifact{}, err
	}
	defer os.RemoveAll(overlayDir)
	replacement := filepath.Join(overlayDir, "zz_default.go")
	if err := os.WriteFile(replacement, runtimeSource, 0o600); err != nil {
		return Artifact{}, err
	}
	original, err := filepath.Abs(filepath.Join(repoRoot, "internal/generated/assembly/zz_default.go"))
	if err != nil {
		return Artifact{}, err
	}
	overlayRaw, err := json.Marshal(map[string]any{"Replace": map[string]string{original: replacement}})
	if err != nil {
		return Artifact{}, err
	}
	overlayFile := filepath.Join(overlayDir, "overlay.json")
	if err := os.WriteFile(overlayFile, overlayRaw, 0o600); err != nil {
		return Artifact{}, err
	}
	modfile, err := prepareBuildModfile(repoRoot, overlayDir, buildSources)
	if err != nil {
		return Artifact{}, err
	}
	dependencyLocks, err := resolveDependencyLocks(repoRoot, modfile)
	if err != nil {
		return Artifact{}, err
	}
	var catalogs []assemblyv1.CatalogManifest
	for _, resolved := range plan.Modules {
		record, resolveErr := catalog.Resolve(resolved.Descriptor.Module.ID)
		if resolveErr != nil {
			return Artifact{}, resolveErr
		}
		compiled, catalogErr := loadCatalog(record.Root, resolved.Descriptor)
		if catalogErr != nil {
			return Artifact{}, catalogErr
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
			return Artifact{}, fmt.Errorf("sdk: decode i18n catalog projection for %s: %w", resolved.Descriptor.Module.ID, err)
		}
		catalogs = append(catalogs, assemblyv1.CatalogManifest{Module: resolved.Descriptor.Module.ID, APIVersion: compiled.SchemaVersion, SchemaVersion: compiled.SchemaVersion, Path: compiled.Path, Digest: compiled.Digest, DefaultLocale: compiled.DefaultLocale, Locales: compiled.Locales, Completeness: completeness, Evidence: []string{string(compiled.State)}, Units: projection.Units})
	}
	uiInput := assemblyv1.UIAssemblyInput{SDKVersion: assemblyv1.UIAssemblySDKVersion, Catalogs: catalogs}
	if recipe.UI != nil {
		uiInput = *recipe.UI
		uiInput.Catalogs = catalogs
	}
	if err := applyRecipeUIOrder(&uiInput, recipe.Order); err != nil {
		return Artifact{}, fmt.Errorf("sdk: configure UI Assembly order: %w", err)
	}
	if err := bindUIContentHashes(&uiInput, plan, catalog); err != nil {
		return Artifact{}, err
	}
	if err := bindUIBuildDependencies(&uiInput, repoRoot, plan, catalog); err != nil {
		return Artifact{}, err
	}
	// Asset hashes are outputs of the frontend compiler. Generate once with a
	// masked value so the compiler can run, hash the resulting dist, then bind
	// that digest into the emitted manifest and bundle. The canonical digest
	// intentionally masks these self-referential manifest values.
	setUIAssetHashes(&uiInput, zeroDigest)
	uiAssembly, err := assemblyv1.GenerateUIAssembly(uiInput)
	if err != nil {
		return Artifact{}, fmt.Errorf("sdk: generate UI Assembly: %w", err)
	}
	if _, err := os.Stat(o.Output); err == nil {
		return Artifact{}, fmt.Errorf("sdk: output already exists: %s", o.Output)
	} else if !os.IsNotExist(err) {
		return Artifact{}, err
	}
	uiBuild, err := buildWebUI(ctx, repoRoot, uiInput, plan, catalog, uiAssembly.Source)
	if err != nil {
		return Artifact{}, err
	}
	defer os.RemoveAll(uiBuild.Root)
	if err := rewriteUIArtifactAssetHashes(uiBuild.Dist, uiBuild.Digest); err != nil {
		return Artifact{}, fmt.Errorf("sdk: bind authoritative UI artifact hash: %w", err)
	}
	if finalDigest, digestErr := hashUIArtifactTree(uiBuild.Dist); digestErr != nil {
		return Artifact{}, fmt.Errorf("sdk: verify authoritative UI artifact hash: %w", digestErr)
	} else if finalDigest != uiBuild.Digest {
		return Artifact{}, fmt.Errorf("sdk: UI artifact hash changed while binding provenance: first %s, final %s", uiBuild.Digest, finalDigest)
	}
	setUIAssetHashes(&uiInput, uiBuild.Digest)
	uiAssembly, err = assemblyv1.GenerateUIAssembly(uiInput)
	if err != nil {
		return Artifact{}, fmt.Errorf("sdk: generate authoritative UI Assembly: %w", err)
	}
	// Seal the canonical Recipe only after the build-owned UI hashes have been
	// bound. This prevents a caller-supplied per-provider asset claim from
	// changing Generation identity without changing the selected output.
	recipe.UI = &uiInput
	canonical, err := assemblyv1.CanonicalRecipe(recipe)
	if err != nil {
		return Artifact{}, err
	}
	if err := overlaySelectedUIDist(overlayFile, filepath.Join(repoRoot, "ui", "dist"), uiBuild.Dist); err != nil {
		return Artifact{}, fmt.Errorf("sdk: bind selected UI to executable embed: %w", err)
	}
	uiArtifacts := map[string]string{"ui/dist": uiBuild.Digest}
	for id, digest := range uiAssembly.Manifest.AssetHashes {
		uiArtifacts["ui/provider/"+id] = digest
	}
	capabilityStates := map[string]assemblyv1.CapabilityState{
		"channels": assemblyv1.CapabilityNotCompiled,
		"mcp":      assemblyv1.CapabilityNotCompiled,
	}
	for _, resolved := range plan.Modules {
		for _, provided := range resolved.Descriptor.Provides {
			if provided.Port == "std/channel@v1" {
				capabilityStates["channels"] = assemblyv1.CapabilityUnconfigured
			}
			if provided.Port == "std/tool-world@v1" && provided.ID == "mcp" {
				capabilityStates["mcp"] = assemblyv1.CapabilityUnconfigured
			}
		}
	}
	manifest, manifestRaw, err := assemblyv1.SealManifest(plan, assemblyv1.SealInputs{SpecificationVersion: "vivy.module/v1", CompilerVersion: "plg-p1", SDKVersion: "v1", CanonicalRecipe: canonical, DependencyLocks: dependencyLocks, UIArtifacts: uiArtifacts, UI: &uiAssembly.Manifest, Catalogs: catalogs, CapabilityStates: capabilityStates})
	if err != nil {
		return Artifact{}, err
	}
	parent := filepath.Dir(o.Output)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return Artifact{}, err
	}
	stage, err := os.MkdirTemp(parent, ".vivy-pack-")
	if err != nil {
		return Artifact{}, err
	}
	published := false
	defer func() {
		if !published {
			_ = os.RemoveAll(stage)
		}
	}()
	if err := os.WriteFile(filepath.Join(stage, "zz_assembly.go"), binder, 0o644); err != nil {
		return Artifact{}, err
	}
	if err := os.WriteFile(filepath.Join(stage, "ui-assembly.ts"), uiAssembly.Source, 0o644); err != nil {
		return Artifact{}, err
	}
	if err := os.WriteFile(filepath.Join(stage, "generation.json"), manifestRaw, 0o644); err != nil {
		return Artifact{}, err
	}
	// Keep the exact Vite-emitted UI tree beside the executable so inspection
	// recomputes the same sealed asset identity. The generated Assembly source
	// is a compiler input, never a substitute for this final artifact.
	if err := copySourceTree(uiBuild.Dist, filepath.Join(stage, "ui", "dist")); err != nil {
		return Artifact{}, fmt.Errorf("sdk: stage final UI artifact: %w", err)
	}
	binary := filepath.Join(stage, "vivy")
	embedded := generation.FrameEmbeddedManifest(manifestRaw)
	cmd := exec.CommandContext(ctx, "go", "build", "-p=2", "-modfile", modfile, "-mod=readonly", "-overlay", overlayFile, "-ldflags", "-X=agent-vivy/sdk/generation.EmbeddedManifestBase64="+embedded, "-o", binary, "./cmd/vivy")
	cmd.Dir = repoRoot
	if output, buildErr := cmd.CombinedOutput(); buildErr != nil {
		return Artifact{}, fmt.Errorf("build generation: %w: %s", buildErr, output)
	}
	if _, err := assemblyv1.NewSourceCatalog(records); err != nil {
		return Artifact{}, fmt.Errorf("sdk: source changed during build: %w", err)
	}
	if err := verifyDependencyLocks(repoRoot, modfile, dependencyLocks); err != nil {
		return Artifact{}, err
	}
	if err := os.Rename(stage, o.Output); err != nil {
		return Artifact{}, fmt.Errorf("publish generation: %w", err)
	}
	published = true
	return Artifact{Directory: o.Output, Binary: filepath.Join(o.Output, "vivy"), Manifest: manifest}, nil
}

type builtWebUI struct {
	Root   string
	Dist   string
	Digest string
}

// buildWebUI compiles the checked-in Web UI against the generated Assembly in
// an isolated source/output boundary. It never writes ui/src/generated or the
// repository's ui/dist, and it only uses the already-installed local Vite
// toolchain.
func buildWebUI(ctx context.Context, repoRoot string, input assemblyv1.UIAssemblyInput, plan assemblyv1.AssemblyPlan, sources assemblyv1.SourceCatalog, generated []byte) (builtWebUI, error) {
	uiRoot := filepath.Join(repoRoot, "ui")
	viteEntry := filepath.Join(uiRoot, "node_modules", "vite", "bin", "vite.js")
	if _, err := os.Stat(viteEntry); err != nil {
		return builtWebUI{}, fmt.Errorf("sdk: build Web UI: checked-in Vite tool is unavailable at %s: %w", viteEntry, err)
	}
	buildRoot, err := os.MkdirTemp("", "vivy-ui-build-")
	if err != nil {
		return builtWebUI{}, fmt.Errorf("sdk: create isolated Web UI build boundary: %w", err)
	}
	cleanup := func(cause error) (builtWebUI, error) {
		_ = os.RemoveAll(buildRoot)
		return builtWebUI{}, cause
	}
	if err := stageUIBuildSources(buildRoot, input, plan, sources); err != nil {
		return cleanup(err)
	}
	if err := os.Symlink(filepath.Join(uiRoot, "node_modules"), filepath.Join(buildRoot, "node_modules")); err != nil {
		return cleanup(fmt.Errorf("sdk: link checked-in Web UI dependencies into isolated build boundary: %w", err))
	}
	assemblyEntry := filepath.Join(buildRoot, "assembly.ts")
	if err := os.WriteFile(assemblyEntry, generated, 0o600); err != nil {
		return cleanup(fmt.Errorf("sdk: write generated Web UI Assembly entry: %w", err))
	}
	dist := filepath.Join(buildRoot, "dist")
	cmd := exec.CommandContext(ctx, "node", viteEntry, "build", "--outDir", dist)
	cmd.Dir = uiRoot
	cmd.Env = append(os.Environ(), "VIVY_UI_ASSEMBLY_ENTRY="+assemblyEntry)
	output, buildErr := cmd.CombinedOutput()
	if buildErr != nil {
		if ctx.Err() != nil {
			return cleanup(fmt.Errorf("sdk: build Web UI with generated Assembly: %w", ctx.Err()))
		}
		return cleanup(fmt.Errorf("sdk: build Web UI with generated Assembly: %w: %s", buildErr, strings.TrimSpace(string(output))))
	}
	if err := ctx.Err(); err != nil {
		return cleanup(fmt.Errorf("sdk: build Web UI with generated Assembly: %w", err))
	}
	if info, statErr := os.Stat(dist); statErr != nil || !info.IsDir() {
		if statErr == nil {
			statErr = errors.New("output is not a directory")
		}
		return cleanup(fmt.Errorf("sdk: build Web UI did not emit dist: %w", statErr))
	}
	digest, err := hashUIArtifactTree(dist)
	if err != nil {
		return cleanup(fmt.Errorf("sdk: hash final Web UI artifact: %w", err))
	}
	return builtWebUI{Root: buildRoot, Dist: dist, Digest: digest}, nil
}

// overlaySelectedUIDist makes the Go embed pattern resolve the exact Vite
// output produced for this Pack invocation. The repository dist is only a
// build placeholder; mapping every old file to an empty backing path removes
// it, while mapping every selected file (including new hashed asset names)
// makes the packed executable carry the selected Generation UI.
func overlaySelectedUIDist(overlayFile, repositoryDist, selectedDist string) error {
	raw, err := os.ReadFile(overlayFile)
	if err != nil {
		return err
	}
	var overlay struct {
		Replace map[string]string
	}
	if err := json.Unmarshal(raw, &overlay); err != nil {
		return fmt.Errorf("decode Go overlay: %w", err)
	}
	if overlay.Replace == nil {
		overlay.Replace = make(map[string]string)
	}
	mapTree := func(root string, replacement func(string, string) (string, string, error)) error {
		return filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				return nil
			}
			if entry.Type()&os.ModeSymlink != 0 || !entry.Type().IsRegular() {
				return fmt.Errorf("UI dist contains non-regular entry %s", path)
			}
			rel, err := filepath.Rel(root, path)
			if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				return fmt.Errorf("invalid UI dist relative path %q", rel)
			}
			diskPath, backingPath, err := replacement(rel, path)
			if err != nil {
				return err
			}
			overlay.Replace[diskPath] = backingPath
			return nil
		})
	}
	if err := mapTree(repositoryDist, func(rel, _ string) (string, string, error) {
		return filepath.Join(repositoryDist, rel), "", nil
	}); err != nil {
		return fmt.Errorf("hide repository UI dist: %w", err)
	}
	if err := mapTree(selectedDist, func(rel, path string) (string, string, error) {
		return filepath.Join(repositoryDist, rel), path, nil
	}); err != nil {
		return fmt.Errorf("map selected UI dist: %w", err)
	}
	encoded, err := json.Marshal(struct {
		Replace map[string]string
	}{Replace: overlay.Replace})
	if err != nil {
		return fmt.Errorf("encode Go overlay: %w", err)
	}
	return os.WriteFile(overlayFile, encoded, 0o600)
}

// bindUIContentHashes replaces Recipe-provided UI provenance with hashes
// derived from the selected, sealed Module source. A stale claim cannot make
// it into the generated Assembly: SourceHash is the Module's verified source
// digest and DependencyLockHash is the digest of the supported lock files in
// that same source tree.
func bindUIContentHashes(input *assemblyv1.UIAssemblyInput, plan assemblyv1.AssemblyPlan, sources assemblyv1.SourceCatalog) error {
	if input == nil {
		return nil
	}
	bind := func(contribution *assemblyv1.UIModule) error {
		if contribution == nil || !uiModuleInputPresent(*contribution) {
			return nil
		}
		moduleID := strings.TrimSpace(contribution.ModuleID)
		if moduleID == "" {
			moduleID = strings.TrimSpace(contribution.ID)
		}
		root, err := resolveUISourceRoot(*contribution, plan, sources)
		if err != nil {
			return err
		}
		record, err := sources.Resolve(moduleID)
		if err != nil {
			// ModuleID is optional in the Recipe adapter. When a contribution
			// identifies only its Provider ID, recover the owning Module from
			// the already-compiled Port edge rather than treating the Provider
			// ID as an arbitrary source lookup key.
			for _, resolved := range plan.Modules {
				if descriptorProvidesUI(resolved.Descriptor, contribution.Port, contribution.ID) {
					moduleID = resolved.Descriptor.Module.ID
					record, err = sources.Resolve(moduleID)
					break
				}
			}
		}
		if err != nil {
			return fmt.Errorf("sdk: resolve UI source %s: %w", contribution.ID, err)
		}
		contribution.ModuleID = moduleID
		sourceHash := strings.TrimSpace(record.Descriptor.Source.SHA256)
		if sourceHash == "" {
			sourceHash, err = assemblyv1.HashSourceTree(root, "")
			if err != nil {
				return fmt.Errorf("sdk: hash UI source %s: %w", contribution.ID, err)
			}
		}
		if claimed, claimErr := claimedUIHash("source", contribution.SourceHash, contribution.SourceSHA256, []string{moduleID, contribution.ID}, input.SourceHashes); claimErr != nil {
			return fmt.Errorf("sdk: UI provider %s: %w", contribution.ID, claimErr)
		} else if claimed != "" && claimed != sourceHash {
			return fmt.Errorf("sdk: UI provider %s source hash mismatch: got %s, want %s", contribution.ID, claimed, sourceHash)
		}
		lockHash, err := hashUIDependencyLocks(root)
		if err != nil {
			return fmt.Errorf("sdk: UI provider %s: %w", contribution.ID, err)
		}
		if claimed, claimErr := claimedUIHash("dependency lock", contribution.LockHash, contribution.DependencyLockHash, []string{moduleID, contribution.ID}, input.DependencyLockHashes, input.LockHashes); claimErr != nil {
			return fmt.Errorf("sdk: UI provider %s: %w", contribution.ID, claimErr)
		} else if claimed != "" && claimed != lockHash {
			return fmt.Errorf("sdk: UI provider %s dependency lock hash mismatch: got %s, want %s", contribution.ID, claimed, lockHash)
		}
		contribution.SourceHash = sourceHash
		contribution.SourceSHA256 = ""
		contribution.DependencyLockHash = lockHash
		contribution.LockHash = ""
		return nil
	}
	for index := range input.Roots {
		if err := bind(&input.Roots[index]); err != nil {
			return err
		}
	}
	if uiModuleInputPresent(input.Root) {
		if err := bind(&input.Root); err != nil {
			return err
		}
	}
	for index := range input.Extensions {
		if err := bind(&input.Extensions[index]); err != nil {
			return err
		}
	}
	// Hash maps are adapter aliases. Once each selected contribution carries
	// its authoritative field, retaining arbitrary omitted entries would still
	// perturb the canonical Recipe without affecting the built composition.
	input.SourceHashes = nil
	input.DependencyLockHashes = nil
	input.LockHashes = nil
	return nil
}

// bindUIBuildDependencies turns package.json declarations into build-owned
// exact dependency pins. Pack intentionally reuses the repository's
// pre-installed Vite toolchain instead of running a package manager or
// reaching the network, so every selected Module dependency must be both an
// exact version and present at the version the compiler will resolve. The
// selected Module's lockfile is hashed separately by bindUIContentHashes and
// remains part of the sealed provenance.
func bindUIBuildDependencies(input *assemblyv1.UIAssemblyInput, repoRoot string, plan assemblyv1.AssemblyPlan, sources assemblyv1.SourceCatalog) error {
	if input == nil {
		return nil
	}
	bind := func(contribution *assemblyv1.UIModule) error {
		if contribution == nil || !uiModuleInputPresent(*contribution) {
			return nil
		}
		root, err := resolveUISourceRoot(*contribution, plan, sources)
		if err != nil {
			return err
		}
		declared := make(map[string]string, len(contribution.Dependencies)+len(contribution.PackageDependencies))
		merge := func(values map[string]string, source string) error {
			for name, version := range values {
				name = strings.TrimSpace(name)
				version = strings.TrimSpace(version)
				if name == "" || version == "" {
					return fmt.Errorf("sdk: UI provider %s has an empty %s dependency pin", contribution.ID, source)
				}
				if previous, exists := declared[name]; exists && previous != version {
					return fmt.Errorf("sdk: UI provider %s has conflicting dependency pins for %s", contribution.ID, name)
				}
				declared[name] = version
			}
			return nil
		}
		if err := merge(contribution.Dependencies, "dependencies"); err != nil {
			return err
		}
		if err := merge(contribution.PackageDependencies, "packageDependencies"); err != nil {
			return err
		}
		manifestDependencies, err := readUIBuildDependencies(root)
		if err != nil {
			return fmt.Errorf("sdk: read UI provider %s package metadata: %w", contribution.ID, err)
		}
		for name, version := range manifestDependencies {
			if previous, exists := declared[name]; exists && previous != version {
				return fmt.Errorf("sdk: UI provider %s dependency pin for %s disagrees with package.json", contribution.ID, name)
			}
			declared[name] = version
		}
		for name, version := range declared {
			if !uiExactDependencyPattern.MatchString(version) {
				return fmt.Errorf("sdk: UI provider %s has floating package dependency %s@%s", contribution.ID, name, version)
			}
			if err := verifyUIHostDependency(repoRoot, name, version); err != nil {
				return fmt.Errorf("sdk: UI provider %s: %w", contribution.ID, err)
			}
		}
		if len(declared) == 0 {
			contribution.PackageDependencies = nil
		} else {
			contribution.PackageDependencies = declared
		}
		contribution.Dependencies = nil
		return nil
	}
	for index := range input.Roots {
		if err := bind(&input.Roots[index]); err != nil {
			return err
		}
	}
	if uiModuleInputPresent(input.Root) {
		if err := bind(&input.Root); err != nil {
			return err
		}
	}
	for index := range input.Extensions {
		if err := bind(&input.Extensions[index]); err != nil {
			return err
		}
	}
	return nil
}

type uiPackageManifest struct {
	Dependencies         map[string]string `json:"dependencies"`
	DevDependencies      map[string]string `json:"devDependencies"`
	OptionalDependencies map[string]string `json:"optionalDependencies"`
}

func readUIBuildDependencies(root string) (map[string]string, error) {
	paths := make([]string, 0, 2)
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == "node_modules" || entry.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Name() == "package.json" && entry.Type().IsRegular() {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	dependencies := make(map[string]string)
	for _, path := range paths {
		body, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		var manifest uiPackageManifest
		if err := json.Unmarshal(body, &manifest); err != nil {
			return nil, fmt.Errorf("decode %s: %w", filepath.ToSlash(path), err)
		}
		for _, group := range []map[string]string{manifest.Dependencies, manifest.DevDependencies, manifest.OptionalDependencies} {
			for name, version := range group {
				if previous, exists := dependencies[name]; exists && strings.TrimSpace(previous) != strings.TrimSpace(version) {
					return nil, fmt.Errorf("conflicting package.json pins for %s", name)
				}
				dependencies[name] = strings.TrimSpace(version)
			}
		}
	}
	return dependencies, nil
}

func verifyUIHostDependency(repoRoot, name, version string) error {
	invalidName := !uiDependencyNamePattern.MatchString(name) || strings.ContainsAny(name, "\\\x00\r\n\t")
	for _, component := range strings.Split(name, "/") {
		if component == "." || component == ".." {
			invalidName = true
		}
	}
	if invalidName {
		return fmt.Errorf("invalid package dependency name %q", name)
	}
	if !uiExactDependencyPattern.MatchString(version) {
		return fmt.Errorf("floating package dependency %s@%s", name, version)
	}
	packageJSON := filepath.Join(repoRoot, "ui", "node_modules", filepath.FromSlash(name), "package.json")
	if name == "@vivy/ui-sdk" {
		// Vite's alias intentionally resolves the SDK from the checked-in source;
		// inspect that same package metadata rather than trusting node_modules.
		packageJSON = filepath.Join(repoRoot, "sdk", "ui", "package.json")
	}
	body, err := os.ReadFile(packageJSON)
	if err != nil {
		return fmt.Errorf("pinned package %s@%s is not installed in the compiler toolchain: %w", name, version, err)
	}
	var manifest struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}
	if err := json.Unmarshal(body, &manifest); err != nil {
		return fmt.Errorf("decode installed package %s: %w", name, err)
	}
	if manifest.Name != "" && manifest.Name != name {
		return fmt.Errorf("installed package metadata for %s names %s", name, manifest.Name)
	}
	if manifest.Version != version {
		return fmt.Errorf("installed package %s is %s, want pinned %s", name, manifest.Version, version)
	}
	return nil
}

func claimedUIHash(label, first, second string, keys []string, maps ...map[string]string) (string, error) {
	claimed, err := chooseUIHashAlias(label, first, second)
	if err != nil {
		return "", err
	}
	mapClaim, err := lookupUIHashAliases(label, keys, maps...)
	if err != nil {
		return "", err
	}
	if claimed != "" && mapClaim != "" && claimed != mapClaim {
		return "", fmt.Errorf("conflicting %s hash aliases", label)
	}
	if claimed != "" {
		return claimed, nil
	}
	return mapClaim, nil
}

func lookupUIHashAliases(label string, keys []string, values ...map[string]string) (string, error) {
	claims := make([]string, 0, len(keys)*len(values))
	for _, valuesMap := range values {
		for _, key := range keys {
			if value := strings.TrimSpace(valuesMap[key]); value != "" {
				claims = append(claims, value)
			}
		}
	}
	if len(claims) == 0 {
		return "", nil
	}
	first := claims[0]
	for _, claim := range claims[1:] {
		if claim != first {
			return "", fmt.Errorf("conflicting %s hash aliases", label)
		}
	}
	return first, nil
}

func chooseUIHashAlias(label, first, second string) (string, error) {
	first = strings.TrimSpace(first)
	second = strings.TrimSpace(second)
	if first != "" && second != "" && first != second {
		return "", fmt.Errorf("conflicting %s hash aliases", label)
	}
	return firstNonEmptyString(first, second), nil
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

var uiDependencyLockNames = map[string]struct{}{
	"npm-shrinkwrap.json": {},
	"package-lock.json":   {},
	"pnpm-lock.yaml":      {},
	"yarn.lock":           {},
}

func isUIDependencyLockFile(relative string) bool {
	_, ok := uiDependencyLockNames[filepath.Base(relative)]
	return ok
}

func hashUIDependencyLocks(root string) (string, error) {
	var paths []string
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == "node_modules" || entry.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("source tree contains symbolic link %s", path)
		}
		if entry.Type().IsRegular() {
			rel, relErr := filepath.Rel(root, path)
			if relErr != nil {
				return relErr
			}
			if isUIDependencyLockFile(rel) {
				paths = append(paths, path)
			}
		}
		return nil
	}); err != nil {
		return "", err
	}
	if len(paths) == 0 {
		return "", errors.New("no supported dependency lock file is present")
	}
	sort.Strings(paths)
	h := sha256.New()
	for _, path := range paths {
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return "", err
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		_, _ = io.WriteString(h, filepath.ToSlash(rel))
		_, _ = h.Write([]byte{0})
		_, _ = h.Write(body)
		_, _ = h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func setUIAssetHashes(input *assemblyv1.UIAssemblyInput, digest string) {
	if input == nil {
		return
	}
	set := func(contribution *assemblyv1.UIModule) {
		if contribution == nil || !uiModuleInputPresent(*contribution) {
			return
		}
		contribution.AssetHash = digest
		contribution.FinalAssetHash = ""
	}
	for index := range input.Roots {
		set(&input.Roots[index])
	}
	if uiModuleInputPresent(input.Root) {
		set(&input.Root)
	}
	for index := range input.Extensions {
		set(&input.Extensions[index])
	}
	input.AssetHashes = nil
	input.FinalAssetHashes = nil
}

// hashUIArtifactTree computes the final UI artifact identity while masking the
// per-provider asset values that are embedded in UI_ASSEMBLY_MANIFEST itself.
// This removes the otherwise circular dependency between a bundle's hash and
// the provenance value embedded in that same bundle.
func hashUIArtifactTree(root string) (string, error) {
	var paths []string
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("UI artifact contains symbolic link %s", path)
		}
		if entry.Type().IsRegular() {
			paths = append(paths, path)
		}
		return nil
	}); err != nil {
		return "", err
	}
	sort.Strings(paths)
	references := make(map[string]string, len(paths)*2)
	for _, path := range paths {
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return "", err
		}
		rawPath := filepath.ToSlash(rel)
		canonicalPath := uiArtifactHashedFilenamePattern.ReplaceAllString(rawPath, "-"+zeroDigest[:8]+"$1")
		if rawPath != canonicalPath {
			references[rawPath] = canonicalPath
			references[filepath.Base(rawPath)] = filepath.Base(canonicalPath)
		}
	}
	h := sha256.New()
	for _, path := range paths {
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return "", err
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		body = maskUIArtifactReferences(body, references)
		body = maskUIArtifactAssetHashes(body)
		canonicalPath := filepath.ToSlash(rel)
		canonicalPath = uiArtifactHashedFilenamePattern.ReplaceAllString(canonicalPath, "-"+zeroDigest[:8]+"$1")
		_, _ = io.WriteString(h, canonicalPath)
		_, _ = h.Write([]byte{0})
		_, _ = h.Write(body)
		_, _ = h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func maskUIArtifactReferences(body []byte, references map[string]string) []byte {
	keys := make([]string, 0, len(references))
	for raw := range references {
		keys = append(keys, raw)
	}
	// Replace longer paths first. A basename can be contained in its full
	// asset path; sorting makes canonicalization deterministic and prevents a
	// shorter replacement from changing a later match's input.
	sort.Slice(keys, func(i, j int) bool {
		if len(keys[i]) != len(keys[j]) {
			return len(keys[i]) > len(keys[j])
		}
		return keys[i] < keys[j]
	})
	for _, raw := range keys {
		body = bytes.ReplaceAll(body, []byte(raw), []byte(references[raw]))
	}
	return body
}

func maskUIArtifactAssetHashes(body []byte) []byte {
	return rewriteUIArtifactAssetHashesInBody(body, zeroDigest)
}

func rewriteUIArtifactAssetHashes(root, digest string) error {
	if !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(digest) {
		return fmt.Errorf("invalid UI artifact digest")
	}
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 || !entry.Type().IsRegular() {
			return fmt.Errorf("UI artifact contains non-regular entry %s", path)
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rewritten := rewriteUIArtifactAssetHashesInBody(body, digest)
		if !bytes.Equal(body, rewritten) {
			if err := os.WriteFile(path, rewritten, 0o600); err != nil {
				return err
			}
		}
		return nil
	})
}

func rewriteUIArtifactAssetHashesInBody(body []byte, digest string) []byte {
	key := []byte(`assetHashes`)
	keyStart := bytes.Index(body, key)
	if keyStart < 0 {
		return body
	}
	openOffset := bytes.IndexByte(body[keyStart+len(key):], '{')
	if openOffset < 0 {
		return body
	}
	open := keyStart + len(key) + openOffset
	depth := 0
	inString, escaped := false, false
	close := -1
	for index := open; index < len(body); index++ {
		char := body[index]
		if inString {
			if escaped {
				escaped = false
			} else if char == '\\' {
				escaped = true
			} else if char == '"' {
				inString = false
			}
			continue
		}
		switch char {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				close = index
			}
		}
		if close >= 0 {
			break
		}
	}
	if close < 0 {
		return body
	}
	masked := append([]byte(nil), body...)
	section := masked[open : close+1]
	replaced := uiArtifactAssetValuePattern.ReplaceAll(section, []byte("${1}"+digest+"${2}"))
	result := make([]byte, 0, len(masked))
	result = append(result, masked[:open]...)
	result = append(result, replaced...)
	result = append(result, masked[close+1:]...)
	return result
}

func stageUIBuildSources(destination string, input assemblyv1.UIAssemblyInput, plan assemblyv1.AssemblyPlan, sources assemblyv1.SourceCatalog) error {
	contributions := make([]assemblyv1.UIModule, 0, len(input.Roots)+len(input.Extensions)+1)
	contributions = append(contributions, input.Roots...)
	if uiModuleInputPresent(input.Root) {
		contributions = append(contributions, input.Root)
	}
	contributions = append(contributions, input.Extensions...)
	for _, contribution := range contributions {
		root, err := resolveUISourceRoot(contribution, plan, sources)
		if err != nil {
			return err
		}
		if err := copyUISourceTree(root, destination); err != nil {
			return fmt.Errorf("sdk: stage selected UI source %s: %w", contribution.ID, err)
		}
	}
	return nil
}

func resolveUISourceRoot(contribution assemblyv1.UIModule, plan assemblyv1.AssemblyPlan, sources assemblyv1.SourceCatalog) (string, error) {
	moduleID := strings.TrimSpace(contribution.ModuleID)
	if moduleID == "" {
		moduleID = strings.TrimSpace(contribution.ID)
	}
	if record, err := sources.Resolve(moduleID); err == nil && record.Root != "" {
		return record.Root, nil
	}
	for _, resolved := range plan.Modules {
		if resolved.Descriptor.Module.ID == moduleID || descriptorProvidesUI(resolved.Descriptor, contribution.Port, contribution.ID) {
			record, err := sources.Resolve(resolved.Descriptor.Module.ID)
			if err == nil && record.Root != "" {
				return record.Root, nil
			}
		}
	}
	return "", fmt.Errorf("sdk: selected UI provider %s has no source root in the sealed Source Catalog", contribution.ID)
}

func descriptorProvidesUI(descriptor module.Descriptor, port, id string) bool {
	for _, provided := range descriptor.Provides {
		if (port == "" || provided.Port == port) && (id == "" || provided.ID == id) && (provided.Port == assemblyv1.UIRootPort || provided.Port == assemblyv1.UIExtensionPort) {
			return true
		}
	}
	return false
}

func copyUISourceTree(source, destination string) error {
	return filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return os.MkdirAll(destination, 0o700)
		}
		if entry.IsDir() {
			if entry.Name() == "node_modules" || entry.Name() == ".git" {
				return filepath.SkipDir
			}
			return os.MkdirAll(filepath.Join(destination, rel), 0o700)
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symbolic link %s is not allowed", path)
		}
		if !entry.Type().IsRegular() || (!uiSourceFile(filepath.ToSlash(rel)) && !isUIDependencyLockFile(filepath.ToSlash(rel))) {
			return nil
		}
		target := filepath.Join(destination, rel)
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if existing, readErr := os.ReadFile(target); readErr == nil {
			if !bytes.Equal(existing, body) {
				return fmt.Errorf("conflicting selected UI source path %s", filepath.ToSlash(rel))
			}
			return nil
		} else if !os.IsNotExist(readErr) {
			return readErr
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return err
		}
		return os.WriteFile(target, body, 0o600)
	})
}

func uiSourceFile(relative string) bool {
	base := filepath.Base(relative)
	switch base {
	case "go.mod", "go.sum", "vivy-module.yaml":
		return false
	}
	switch strings.ToLower(filepath.Ext(relative)) {
	case ".go", ".yaml", ".yml", ".toml", ".md":
		return false
	default:
		return true
	}
}

func uiModuleInputPresent(module assemblyv1.UIModule) bool {
	return module.ID != "" || module.ProviderID != "" || module.ModuleID != "" || module.Port != "" ||
		module.Entry != "" || module.ImportPath != "" || module.Export != "" || module.ExportName != "" ||
		module.SourceHash != "" || module.SourceSHA256 != "" || module.LockHash != "" || module.DependencyLockHash != "" ||
		module.AssetHash != "" || module.FinalAssetHash != "" || len(module.Dependencies) > 0 || len(module.PackageDependencies) > 0 ||
		len(module.Before) > 0 || len(module.After) > 0 || len(module.Replaces) > 0
}

// applyRecipeUIOrder bridges the canonical ordered-Port Recipe field to the
// UI composition adapter. UIAssemblyInput keeps a nested field so callers can
// use the generator directly, but Pack must honor the same top-level order
// sequence used by the Assembly Compiler for std/ui-extension@v1.
func applyRecipeUIOrder(input *assemblyv1.UIAssemblyInput, recipeOrder map[string][]string) error {
	order, exists := recipeOrder[assemblyv1.UIExtensionPort]
	if !exists {
		return nil
	}
	if input.ExtensionOrder != nil && !equalStringSlices(input.ExtensionOrder, order) {
		return errors.New("Recipe order disagrees with UI ExtensionOrder")
	}
	if input.Order != nil && !equalStringSlices(input.Order, order) {
		return errors.New("Recipe order disagrees with UI Order alias")
	}
	input.ExtensionOrder = append([]string(nil), order...)
	return nil
}

func equalStringSlices(first, second []string) bool {
	if len(first) != len(second) {
		return false
	}
	for index := range first {
		if first[index] != second[index] {
			return false
		}
	}
	return true
}

func prepareBuildModfile(repoRoot, temporaryRoot string, sources []string) (string, error) {
	modfile := filepath.Join(temporaryRoot, "vivy-pack.mod")
	rootMod, err := os.ReadFile(filepath.Join(repoRoot, "go.mod"))
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(modfile, rootMod, 0o600); err != nil {
		return "", err
	}
	if rootSum, readErr := os.ReadFile(filepath.Join(repoRoot, "go.sum")); readErr == nil {
		if err := os.WriteFile(filepath.Join(temporaryRoot, "vivy-pack.sum"), rootSum, 0o600); err != nil {
			return "", err
		}
	}
	rootModule, err := goModulePath(repoRoot)
	if err != nil {
		return "", err
	}
	seen := map[string]bool{rootModule: true}
	for _, source := range sources {
		absolute, err := filepath.Abs(source)
		if err != nil {
			return "", err
		}
		modulePath, err := goModulePath(absolute)
		if err != nil {
			return "", fmt.Errorf("sdk: source %s: %w", source, err)
		}
		if seen[modulePath] {
			continue
		}
		seen[modulePath] = true
		version := "v0.0.0"
		parts := strings.Split(modulePath, "/")
		last := parts[len(parts)-1]
		if len(last) > 1 && last[0] == 'v' && strings.Trim(last[1:], "0123456789") == "" && last != "v0" && last != "v1" {
			version = last + ".0.0"
		}
		for _, edit := range []string{"-require=" + modulePath + "@" + version, "-replace=" + modulePath + "=" + absolute} {
			cmd := exec.Command("go", "mod", "edit", "-modfile="+modfile, edit)
			cmd.Dir = repoRoot
			if output, editErr := cmd.CombinedOutput(); editErr != nil {
				return "", fmt.Errorf("sdk: bind source module %s: %w: %s", modulePath, editErr, strings.TrimSpace(string(output)))
			}
		}
	}
	return modfile, nil
}

func resolveDependencyLocks(repoRoot, modfile string) (map[string]string, error) {
	download := exec.Command("go", "mod", "download", "-modfile="+modfile)
	download.Dir = repoRoot
	if output, err := download.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("sdk: resolve dependency sums: %w: %s", err, strings.TrimSpace(string(output)))
	}
	list := exec.Command("go", "list", "-mod=mod", "-modfile="+modfile, "-m", "-f", "{{.Path}} {{.Version}}", "all")
	list.Dir = repoRoot
	if output, err := list.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("sdk: resolve dependency build list: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return snapshotDependencyLocks(repoRoot, modfile)
}

func snapshotDependencyLocks(repoRoot, modfile string) (map[string]string, error) {
	readonly := exec.Command("go", "list", "-mod=readonly", "-modfile="+modfile, "-m", "-f", "{{.Path}} {{.Version}}", "all")
	readonly.Dir = repoRoot
	buildList, err := readonly.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("sdk: freeze dependency build list: %w: %s", err, strings.TrimSpace(string(buildList)))
	}
	locks, err := sealedFileInputs(repoRoot, "go.mod", "go.sum")
	if err != nil {
		return nil, err
	}
	canonicalMod, err := canonicalResolvedModfile(repoRoot, modfile)
	if err != nil {
		return nil, err
	}
	locks["resolved-go.mod"] = sha256Hex(canonicalMod)
	sumPath := strings.TrimSuffix(modfile, ".mod") + ".sum"
	resolvedSum, err := os.ReadFile(sumPath)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	locks["resolved-go.sum"] = sha256Hex(resolvedSum)
	locks["resolved-build-list"] = sha256Hex(bytes.TrimSpace(buildList))
	return locks, nil
}

func canonicalResolvedModfile(repoRoot, modfile string) ([]byte, error) {
	command := exec.Command("go", "mod", "edit", "-json", "-modfile="+modfile)
	command.Dir = repoRoot
	raw, err := command.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("sdk: canonicalize resolved modfile: %w: %s", err, strings.TrimSpace(string(raw)))
	}
	var document map[string]any
	if err := json.Unmarshal(raw, &document); err != nil {
		return nil, fmt.Errorf("sdk: decode resolved modfile: %w", err)
	}
	// Local replacement paths are build-machine locations. Their Old.Path
	// already identifies the dependency while each selected source tree is
	// sealed separately, so normalize only the location-bearing New.Path.
	if replacements, ok := document["Replace"].([]any); ok {
		for _, value := range replacements {
			replacement, _ := value.(map[string]any)
			oldModule, _ := replacement["Old"].(map[string]any)
			newModule, _ := replacement["New"].(map[string]any)
			version, _ := newModule["Version"].(string)
			oldPath, _ := oldModule["Path"].(string)
			if version == "" && oldPath != "" {
				newModule["Path"] = "local:" + oldPath
			}
		}
	}
	canonical, err := json.Marshal(document)
	if err != nil {
		return nil, fmt.Errorf("sdk: encode resolved modfile: %w", err)
	}
	return canonical, nil
}

func verifyDependencyLocks(repoRoot, modfile string, expected map[string]string) error {
	actual, err := snapshotDependencyLocks(repoRoot, modfile)
	if err != nil {
		return err
	}
	if len(actual) != len(expected) {
		return errors.New("sdk: dependency locks changed during build")
	}
	for name, digest := range expected {
		if actual[name] != digest {
			return fmt.Errorf("sdk: dependency lock %s changed during build", name)
		}
	}
	return nil
}

func sha256Hex(raw []byte) string {
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:])
}

func goModulePath(dir string) (string, error) {
	cmd := exec.Command("go", "list", "-m", "-f", "{{.Path}}")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("resolve Go module: %w: %s", err, strings.TrimSpace(string(out)))
	}
	modulePath := strings.TrimSpace(string(out))
	if modulePath == "" || modulePath == "command-line-arguments" {
		return "", errors.New("resolve Go module: empty module path")
	}
	return modulePath, nil
}

func sealedFileInputs(root string, names ...string) (map[string]string, error) {
	inputs := make(map[string]string, len(names))
	for _, name := range names {
		body, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			return nil, fmt.Errorf("sdk: read sealed input %s: %w", name, err)
		}
		digest := sha256.Sum256(body)
		inputs[name] = hex.EncodeToString(digest[:])
	}
	return inputs, nil
}

func InspectArtifact(dir string) (Artifact, error) {
	raw, err := os.ReadFile(filepath.Join(dir, "generation.json"))
	if err != nil {
		return Artifact{}, err
	}
	manifest, err := assemblyv1.InspectManifest(raw)
	if err != nil {
		return Artifact{}, err
	}
	binary := filepath.Join(dir, "vivy")
	if _, err := os.Stat(binary); err != nil {
		return Artifact{}, err
	}
	binaryRaw, err := os.ReadFile(binary)
	if err != nil {
		return Artifact{}, fmt.Errorf("read executable Generation Manifest: %w", err)
	}
	embeddedRaw, err := generation.ExtractEmbeddedManifest(binaryRaw)
	if err != nil {
		return Artifact{}, fmt.Errorf("inspect embedded Generation Manifest: %w", err)
	}
	embedded, err := assemblyv1.InspectManifest(embeddedRaw)
	if err != nil {
		return Artifact{}, err
	}
	if embedded.GenerationID != manifest.GenerationID || !bytes.Equal(embeddedRaw, raw) {
		return Artifact{}, fmt.Errorf("Generation Manifest is not bound to executable")
	}
	if expected := manifest.UIArtifacts["ui/dist"]; expected != "" {
		actual, hashErr := hashUIArtifactTree(filepath.Join(dir, "ui", "dist"))
		if hashErr != nil {
			return Artifact{}, fmt.Errorf("inspect final UI artifact: %w", hashErr)
		}
		if actual != expected {
			return Artifact{}, fmt.Errorf("UI artifact hash mismatch: got %s, want %s", actual, expected)
		}
	}
	return Artifact{Directory: dir, Binary: binary, Manifest: manifest}, nil
}

func sourceRecords(repoRoot string, sources []string, pins map[string]module.Source) ([]assemblyv1.SourceRecord, error) {
	internal, err := defaults.Catalog(repoRoot)
	if err != nil {
		return nil, err
	}
	records := make([]assemblyv1.SourceRecord, 0, len(internal)+len(sources)+8)
	for _, r := range internal {
		records = append(records, assemblyv1.SourceRecord{Descriptor: r.Descriptor, Trust: assemblyv1.TrustT1, Root: filepath.Join(repoRoot, "internal"), Ref: "file:internal", Binding: assemblyv1.GoBinding{ImportPath: r.Binding.ImportPath, Package: r.Binding.Package, Constructor: r.Binding.Constructor, ProviderConstructor: r.Binding.ProviderConstructor, ProviderCollection: r.Binding.ProviderCollection}})
	}
	known := []struct {
		dir, importPath, pkg                string
		diagnostics, languageServerStatuses bool
	}{
		{dir: "plugins/dingtalk", importPath: "example.com/vivy/plugins/dingtalk", pkg: "dingtalk"},
		{dir: "plugins/discord", importPath: "example.com/vivy/plugins/discord", pkg: "discord"},
		{dir: "plugins/feishu", importPath: "example.com/vivy/plugins/feishu", pkg: "feishu"},
		{dir: "plugins/qq", importPath: "example.com/vivy/plugins/qq", pkg: "qq"},
		{dir: "plugins/telegram", importPath: "example.com/vivy/plugins/telegram", pkg: "telegram"},
		{dir: "plugins/hello-fs", importPath: "agent-vivy/plugins/hello-fs", pkg: "hellofs"},
		{dir: "plugins/lsp", importPath: "example.com/vivy/plugins/lsp", pkg: "lsp", diagnostics: true, languageServerStatuses: true},
		{dir: "faces/headless", importPath: "example.com/vivy/faces/headless", pkg: "headless"},
		{dir: "faces/tui", importPath: "example.com/vivy/faces/tui", pkg: "tui"},
	}
	seen := map[string]bool{}
	for _, k := range known {
		dir := filepath.Join(repoRoot, k.dir)
		if _, err := os.Stat(filepath.Join(dir, descriptorFilename)); err != nil {
			continue
		}
		d, _, err := loadDescriptor(dir)
		if err != nil {
			return nil, err
		}
		if err := verifySource(dir, d); err != nil {
			return nil, fmt.Errorf("sdk: source %s: %w", dir, err)
		}
		records = append(records, assemblyv1.SourceRecord{Descriptor: d, Trust: assemblyv1.TrustT1, Root: dir, Ref: "repo:" + k.dir, Binding: assemblyv1.GoBinding{ImportPath: k.importPath, Package: k.pkg, Constructor: "New", ProviderConstructor: "NewProvider", DiagnosticObserver: k.diagnostics, LanguageServerStatusProvider: k.languageServerStatuses}})
		seen[dir] = true
	}
	for _, dir := range sources {
		abs, _ := filepath.Abs(dir)
		if seen[abs] {
			continue
		}
		d, _, err := loadDescriptor(dir)
		if err != nil {
			return nil, err
		}
		if err := verifySource(dir, d); err != nil {
			return nil, fmt.Errorf("sdk: source %s: %w", dir, err)
		}
		pin, ok := pins[d.Module.ID]
		if !ok {
			return nil, fmt.Errorf("sdk: external source %s requires an authoritative Recipe sources pin", d.Module.ID)
		}
		if pin.Ref != d.Source.Ref || pin.SHA256 != d.Source.SHA256 {
			return nil, fmt.Errorf("sdk: external source pin mismatch for %s", d.Module.ID)
		}
		importPath, packageName, err := sourceGoBinding(dir)
		if err != nil {
			return nil, fmt.Errorf("sdk: source %s: %w", dir, err)
		}
		records = append(records, assemblyv1.SourceRecord{Descriptor: d, Trust: assemblyv1.TrustT2, Root: abs, Ref: pin.Ref, Binding: assemblyv1.GoBinding{ImportPath: importPath, Package: packageName, Constructor: "New", ProviderConstructor: "NewProvider"}})
	}
	return records, nil
}

func sourceGoBinding(dir string) (string, string, error) {
	cmd := exec.Command("go", "list", "-f", "{{.ImportPath}} {{.Name}}", ".")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", "", fmt.Errorf("package is not linkable: %w: %s", err, strings.TrimSpace(string(out)))
	}
	fields := strings.Fields(string(out))
	if len(fields) != 2 || fields[0] == "" || fields[1] == "" || fields[1] == "main" {
		return "", "", fmt.Errorf("expected one importable Go package, got %q", strings.TrimSpace(string(out)))
	}
	return fields[0], fields[1], nil
}
func findRepoRoot(start string) (string, error) {
	abs, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(abs, "go.mod")); err == nil {
			return abs, nil
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			return "", errors.New("sdk: repository root not found")
		}
		abs = parent
	}
}

func Run(args []string) int { return run(args, os.Stdout, os.Stderr) }
func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: vivy-sdk verify <module-dir> | pack --recipe <file> --output <dir> [--source <dir>] | inspect-artifact <dir>")
		return 2
	}
	switch args[0] {
	case "verify":
		if len(args) != 2 {
			fmt.Fprintln(stderr, "usage: vivy-sdk verify <module-dir>")
			return 2
		}
		r, err := Verify(args[1])
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		fmt.Fprintf(stdout, "ok %s\n", r.Module)
		return 0
	case "pack":
		o, err := parsePackArgs(args[1:])
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		a, err := Pack(context.Background(), o)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		_ = json.NewEncoder(stdout).Encode(a)
		return 0
	case "inspect-artifact":
		if len(args) != 2 {
			return 2
		}
		a, err := InspectArtifact(args[1])
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		_ = json.NewEncoder(stdout).Encode(a)
		return 0
	default:
		fmt.Fprintf(stderr, "unknown sdk command %q\n", args[0])
		return 2
	}
}

var _ = module.APIVersionV1

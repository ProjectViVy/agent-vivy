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
	for _, rel := range []string{"plugins/dingtalk", "plugins/discord", "plugins/feishu", "plugins/qq", "plugins/telegram", "plugins/hello-fs", "plugins/lsp", "faces/headless", "faces/tui"} {
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
	evidence := assemblyv1.SupportedPortEvidence()
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
	canonical, err := assemblyv1.CanonicalRecipe(recipe)
	if err != nil {
		return Artifact{}, err
	}
	dependencyLocks, err := resolveDependencyLocks(repoRoot, modfile)
	if err != nil {
		return Artifact{}, err
	}
	uiArtifacts := map[string]string{}
	if digest, hashErr := assemblyv1.HashSourceTree(filepath.Join(repoRoot, "ui", "dist"), ""); hashErr == nil {
		uiArtifacts["ui/dist"] = digest
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
		catalogs = append(catalogs, assemblyv1.CatalogManifest{Module: resolved.Descriptor.Module.ID, SchemaVersion: compiled.SchemaVersion, Path: compiled.Path, Digest: compiled.Digest, DefaultLocale: compiled.DefaultLocale, Locales: compiled.Locales, Completeness: completeness, Evidence: []string{string(compiled.State)}})
	}
	capabilityStates := capabilityStatesForPlan(plan)
	manifest, manifestRaw, err := assemblyv1.SealManifest(plan, assemblyv1.SealInputs{SpecificationVersion: "vivy.module/v1", CompilerVersion: "plg-p1", SDKVersion: "v1", CanonicalRecipe: canonical, DependencyLocks: dependencyLocks, UIArtifacts: uiArtifacts, Catalogs: catalogs, CapabilityStates: capabilityStates})
	if err != nil {
		return Artifact{}, err
	}
	if _, err := os.Stat(o.Output); err == nil {
		return Artifact{}, fmt.Errorf("sdk: output already exists: %s", o.Output)
	} else if !os.IsNotExist(err) {
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
	if err := os.WriteFile(filepath.Join(stage, "generation.json"), manifestRaw, 0o644); err != nil {
		return Artifact{}, err
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

// capabilityStatesForPlan derives compiled-capability signals from the
// build-owned typed bindings, not from a Module's public Port claims alone.
// In particular, a public mcp ToolWorld is not a compiled MCPHost unless its
// Source Catalog record carries the authoritative MCPHostProvider binding.
func capabilityStatesForPlan(plan assemblyv1.AssemblyPlan) map[string]assemblyv1.CapabilityState {
	states := map[string]assemblyv1.CapabilityState{
		"channels": assemblyv1.CapabilityNotCompiled,
		"mcp":      assemblyv1.CapabilityNotCompiled,
	}
	for _, resolved := range plan.Modules {
		for _, provided := range resolved.Descriptor.Provides {
			if provided.Port == "std/channel@v1" {
				states["channels"] = assemblyv1.CapabilityUnconfigured
			}
			if provided.Port == "std/tool-world@v1" && provided.ID == "mcp" && resolved.Binding.MCPHostProvider {
				states["mcp"] = assemblyv1.CapabilityUnconfigured
			}
		}
	}
	return states
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
	return Artifact{Directory: dir, Binary: binary, Manifest: manifest}, nil
}

func sourceRecords(repoRoot string, sources []string, pins map[string]module.Source) ([]assemblyv1.SourceRecord, error) {
	internal, err := defaults.Catalog(repoRoot)
	if err != nil {
		return nil, err
	}
	records := make([]assemblyv1.SourceRecord, 0, len(internal)+len(sources)+8)
	for _, r := range internal {
		records = append(records, assemblyv1.SourceRecord{Descriptor: r.Descriptor, Trust: assemblyv1.TrustT1, Root: filepath.Join(repoRoot, "internal"), Ref: "file:internal", Binding: assemblyv1.GoBinding{ImportPath: r.Binding.ImportPath, Package: r.Binding.Package, Constructor: r.Binding.Constructor, ProviderConstructor: r.Binding.ProviderConstructor, ProviderCollection: r.Binding.ProviderCollection, ContextSourceProvider: r.Binding.ContextSourceProvider, SkillSourceProvider: r.Binding.SkillSourceProvider, MCPHostProvider: r.Binding.MCPHostProvider}})
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

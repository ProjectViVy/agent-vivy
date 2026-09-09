package sdk

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"testing"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/generated/presentation"
	"agent-vivy/internal/i18n"
)

func TestPackRequiresWith(t *testing.T) {
	if _, err := Pack(packOptions{}); err == nil || !strings.Contains(err.Error(), "--with") {
		t.Fatalf("err = %v", err)
	}
}

func TestPackRejectsFailedVerify(t *testing.T) {
	_, err := Pack(packOptions{With: []string{filepath.Join("testdata", "bad-seam-journal")}})
	if err == nil || !strings.Contains(err.Error(), "verify") {
		t.Fatalf("err = %v", err)
	}
}

func TestPackRejectsInvalidDeveloperLocale(t *testing.T) {
	t.Setenv(i18n.DefaultLocaleEnv, "fr")
	if _, err := Pack(packOptions{With: []string{"hello-fs"}, Out: t.TempDir()}); err == nil ||
		!strings.Contains(err.Error(), "unsupported locale") {
		t.Fatalf("Pack invalid locale error = %v", err)
	}
}

func TestCommittedPresentationSettingsUseUnsealedEnglish(t *testing.T) {
	if presentation.DefaultLocale != i18n.English {
		t.Fatalf("DefaultLocale = %q, want %q", presentation.DefaultLocale, i18n.English)
	}
	if presentation.SealedGeneration {
		t.Fatal("committed development body must be unsealed")
	}
}

func TestPackResolvedLocaleMatchesRecipeAndOverlay(t *testing.T) {
	t.Setenv(i18n.DefaultLocaleEnv, "zh")
	root := t.TempDir()
	tmp := t.TempDir()
	locale, err := i18n.DeveloperDefault(filepath.Join(root, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	replacements := make(map[string]string)
	recipe := domain.AssemblyRecipe{}
	if err := embedPresentationSettings(replacements, root, tmp, locale, &recipe); err != nil {
		t.Fatal(err)
	}
	if recipe.Settings.Locale != "zh" {
		t.Fatalf("recipe locale = %q, want zh", recipe.Settings.Locale)
	}
	live := filepath.Join(root, "internal", "generated", "presentation", "zz_settings.go")
	overlay, ok := replacements[live]
	if !ok {
		t.Fatalf("presentation settings replacement missing: %+v", replacements)
	}
	raw, err := os.ReadFile(overlay)
	if err != nil {
		t.Fatal(err)
	}
	src := string(raw)
	if !strings.Contains(src, `const DefaultLocale i18n.Locale = "zh"`) ||
		!strings.Contains(src, "const SealedGeneration = true") {
		t.Fatalf("generated presentation settings do not seal zh:\n%s", src)
	}
}

func TestGenerateRegisterImportPaths(t *testing.T) {
	src := generateRegister([]packedPluginForGen{
		{pkg: "hellofs", impPath: "agent-vivy/plugins/hello-fs"},
		{pkg: "fakechannel", impPath: "example.com/vivy/fake-channel"},
	})
	if !strings.Contains(src, `hellofs "agent-vivy/plugins/hello-fs"`) {
		t.Fatalf("in-species plugin import missing: %s", src)
	}
	if !strings.Contains(src, `fakechannel "example.com/vivy/fake-channel"`) {
		t.Fatalf("standalone plugin module-path import missing: %s", src)
	}
	if !strings.Contains(src, "hellofs.New()") || !strings.Contains(src, "fakechannel.New()") {
		t.Fatalf("register calls missing: %s", src)
	}
}

func TestStandaloneModulePath(t *testing.T) {
	dir := filepath.Join("testdata", "fake-channel")
	mod, ok := standaloneModulePath(dir)
	if !ok || mod != "example.com/vivy/fake-channel" {
		t.Fatalf("module path = %q ok=%v", mod, ok)
	}
	if _, ok := standaloneModulePath(filepath.Join("..", "..", "plugins", "hello-fs")); ok {
		t.Fatal("hello-fs has no own go.mod; must not be standalone")
	}
}

// TestPackFakeChannelStandaloneModule packs a seam-channel plugin that
// lives in its own go.mod. The real `go build` with the double overlay
// (zz_register.go + root go.mod) is the acceptance.
func TestPackFakeChannelStandaloneModule(t *testing.T) {
	t.Setenv(i18n.DefaultLocaleEnv, "zh")
	root, err := findModuleRoot(".")
	if err != nil {
		t.Fatal(err)
	}
	live := filepath.Join(root, "internal", "generated", "plugins", "zz_register.go")
	before, err := os.ReadFile(live)
	if err != nil {
		t.Fatal(err)
	}
	liveGoMod := filepath.Join(root, "go.mod")
	goModBefore, err := os.ReadFile(liveGoMod)
	if err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	art, err := Pack(packOptions{With: []string{filepath.Join("sdk", "internal", "testdata", "fake-channel")}, Out: out})
	if err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(live)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("pack mutated the live zz_register.go")
	}
	goModAfter, err := os.ReadFile(liveGoMod)
	if err != nil {
		t.Fatal(err)
	}
	if string(goModBefore) != string(goModAfter) {
		t.Fatal("pack mutated the live go.mod")
	}
	if len(art.Recipe.Plugins) != 1 || art.Recipe.Plugins[0] != "fake-channel" {
		t.Fatalf("recipe = %+v", art.Recipe)
	}
	if art.Recipe.Settings.Locale != "zh" {
		t.Fatalf("recipe locale = %q, want zh", art.Recipe.Settings.Locale)
	}
	if len(art.Tools) != 0 {
		t.Fatalf("channel plugin must not contribute tools: %+v", art.Tools)
	}
	if len(art.Plugins) != 1 {
		t.Fatalf("plugins = %+v, want exactly one entry", art.Plugins)
	}
	wantDir, err := filepath.Abs(filepath.Join(root, "sdk", "internal", "testdata", "fake-channel"))
	if err != nil {
		t.Fatal(err)
	}
	assertPluginEntry(t, art.Plugins[0], artifactPlugin{
		Name:      "fake-channel",
		Version:   "0.1.0",
		Seam:      "channel",
		Grants:    []string{"channel.poll", "secret.read"},
		Transport: "poll",
		SourceRef: fileRefPrefix + wantDir,
	})
	exe := strings.TrimPrefix(art.SourceRef, fileRefPrefix)
	if _, err := os.Stat(exe); err != nil {
		t.Fatalf("packed exe missing: %v", err)
	}
	inspected, err := InspectArtifact(out)
	if err != nil {
		t.Fatal(err)
	}
	if inspected.ID != art.ID || inspected.ArtifactSHA256 != art.ArtifactSHA256 {
		t.Fatalf("inspect-artifact = %+v", inspected)
	}
}

func TestPackHelloFSWritesArtifactAndLeavesLiveRegister(t *testing.T) {
	t.Setenv(i18n.DefaultLocaleEnv, "en")
	root, err := findModuleRoot(".")
	if err != nil {
		t.Fatal(err)
	}
	live := filepath.Join(root, "internal", "generated", "plugins", "zz_register.go")
	before, err := os.ReadFile(live)
	if err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	art, err := Pack(packOptions{With: []string{"hello-fs"}, Out: out})
	if err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(live)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("pack mutated the live zz_register.go")
	}
	if art.ID == "" || art.ArtifactSHA256 == "" || !strings.Contains(art.SourceRef, "file:") {
		t.Fatalf("artifact = %+v", art)
	}
	if len(art.Recipe.Plugins) != 1 || art.Recipe.Plugins[0] != "hello-fs" {
		t.Fatalf("recipe = %+v", art.Recipe)
	}
	if art.Recipe.Settings.Locale != "en" {
		t.Fatalf("recipe locale = %q, want en", art.Recipe.Settings.Locale)
	}
	if len(art.Plugins) != 1 {
		t.Fatalf("plugins = %+v, want exactly one entry", art.Plugins)
	}
	wantDir, err := filepath.Abs(filepath.Join(root, "plugins", "hello-fs"))
	if err != nil {
		t.Fatal(err)
	}
	assertPluginEntry(t, art.Plugins[0], artifactPlugin{
		Name:      "hello-fs",
		Version:   "0.1.0",
		Seam:      "tool-world",
		Grants:    []string{"fs.read"},
		SourceRef: fileRefPrefix + wantDir,
	})
	hasStat := false
	for _, tool := range art.Tools {
		if tool.Name == "hello_stat" && tool.Readonly {
			hasStat = true
		}
	}
	if !hasStat {
		t.Fatalf("tools = %+v", art.Tools)
	}
	exe := strings.TrimPrefix(art.SourceRef, fileRefPrefix)
	if _, err := os.Stat(exe); err != nil {
		t.Fatalf("packed exe missing: %v", err)
	}
	inspected, err := InspectArtifact(out)
	if err != nil {
		t.Fatal(err)
	}
	if inspected.ID != art.ID || inspected.ArtifactSHA256 != art.ArtifactSHA256 {
		t.Fatalf("inspect-artifact = %+v", inspected)
	}
	if inspected.Recipe.Settings.Locale != art.Recipe.Settings.Locale {
		t.Fatalf("generation.json locale = %q, want %q", inspected.Recipe.Settings.Locale, art.Recipe.Settings.Locale)
	}
}

// TestParseRequireLines covers the go.mod line parser the overlay merge
// relies on: single-line directives, parenthesized blocks, comments, and
// `// indirect` annotations (kept as written).
func TestParseRequireLines(t *testing.T) {
	data := `module example.com/vivy/plugins/telegram

go 1.26.4

require agent-vivy v0.0.0

require (
	github.com/mymmrac/telego v1.10.0

	// a comment line inside the block
	github.com/valyala/fasthttp v1.71.0 // indirect
)

replace agent-vivy => ../..
`
	specs := parseRequireLines(data)
	want := []requireSpec{
		{path: "agent-vivy", version: "v0.0.0"},
		{path: "github.com/mymmrac/telego", version: "v1.10.0"},
		{path: "github.com/valyala/fasthttp", version: "v1.71.0", comment: "// indirect"},
	}
	if len(specs) != len(want) {
		t.Fatalf("specs = %+v, want %+v", specs, want)
	}
	for i, spec := range specs {
		if spec != want[i] {
			t.Fatalf("specs[%d] = %+v, want %+v", i, spec, want[i])
		}
	}
}

// telegramStandalonePlugin describes the real plugins/telegram module for
// the overlay tests.
func telegramStandalonePlugin(root string) packedPlugin {
	return packedPlugin{
		dir:        filepath.Join(root, "plugins", "telegram"),
		name:       "telegram",
		impPath:    "example.com/vivy/plugins/telegram",
		standalone: true,
	}
}

// TestOverlayGoModMergesPluginRequires: packing a standalone plugin with
// third-party dependencies merges its require closure into the overlaid
// root go.mod (the CH-C2 gap), while the species module is never
// self-required.
// TestOverlayGoModMergesPluginRequires: a standalone plugin the root
// go.mod does NOT carry gets its require+replace pair appended, and its
// third-party closure merges as new single-line requires. (Root-carried
// plugins take the idempotent path instead — see the idempotency test.)
func TestOverlayGoModMergesPluginRequires(t *testing.T) {
	root, err := findModuleRoot(".")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	synth := "module example.com/vivy/synth\n\ngo 1.26.4\n\nrequire agent-vivy v0.0.0\n\nrequire gopkg.in/yaml.v3 v3.0.0\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(synth), 0o600); err != nil {
		t.Fatal(err)
	}
	p := packedPlugin{dir: dir, name: "synth", impPath: "example.com/vivy/synth", standalone: true}
	out, err := overlayGoModForStandalone(root, t.TempDir(), []packedPlugin{p})
	if err != nil {
		t.Fatal(err)
	}
	merged, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(merged), "require gopkg.in/yaml.v3 v3.0.0") {
		t.Fatalf("overlay go.mod misses the plugin's yaml.v3 require:\n%s", merged)
	}
	if !strings.Contains(string(merged), "require example.com/vivy/synth v0.0.0") {
		t.Fatalf("overlay go.mod misses the plugin module require:\n%s", merged)
	}
	if !strings.Contains(string(merged), "replace example.com/vivy/synth => ") {
		t.Fatalf("overlay go.mod misses the plugin module replace:\n%s", merged)
	}
	// The species module must not require itself (the plugin's own
	// `require agent-vivy v0.0.0` is dropped).
	if strings.Contains(string(merged), "require agent-vivy v0.0.0") {
		t.Fatalf("overlay go.mod self-requires the species module:\n%s", merged)
	}
}

// TestParseReplaceTargets covers single-line and block replace forms,
// including a versioned left side and comment noise.
func TestParseReplaceTargets(t *testing.T) {
	data := `module agent-vivy

go 1.26.4

require example.com/vivy/plugins/telegram v0.0.0

replace example.com/vivy/plugins/telegram => ./plugins/telegram

replace (
	example.com/vivy/plugins/dingtalk v0.0.0 => ./plugins/dingtalk
	// a comment line inside the block
	example.com/vivy/plugins/qq => ../elsewhere
)
`
	targets := parseReplaceTargets(data)
	want := []string{
		"example.com/vivy/plugins/telegram",
		"example.com/vivy/plugins/dingtalk",
		"example.com/vivy/plugins/qq",
	}
	if len(targets) != len(want) {
		t.Fatalf("targets = %v, want exactly %v", targets, want)
	}
	for _, path := range want {
		if !targets[path] {
			t.Fatalf("targets misses %s: %v", path, targets)
		}
	}
}

// TestOverlayGoModIdempotentWhenRootCarriesPlugin: the full committed
// species body already requires and replaces the plugin module in the
// root go.mod. The merged overlay must keep exactly one require and one
// replace line for it — a second replace for the same module is a
// conflicting-replacement build error — while staying go-command valid.
func TestOverlayGoModIdempotentWhenRootCarriesPlugin(t *testing.T) {
	root, err := findModuleRoot(".")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	if !parseReplaceTargets(string(data))["example.com/vivy/plugins/telegram"] {
		t.Skip("root go.mod does not carry the telegram module (narrow species body)")
	}
	out, err := overlayGoModForStandalone(root, t.TempDir(), []packedPlugin{telegramStandalonePlugin(root)})
	if err != nil {
		t.Fatal(err)
	}
	merged, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	const pluginPath = "example.com/vivy/plugins/telegram"
	if n := strings.Count(string(merged), "replace "+pluginPath+" "); n != 1 {
		t.Fatalf("overlay go.mod has %d replace lines for %s:\n%s", n, pluginPath, merged)
	}
	// Count the require by path+version so the assertion holds whether the
	// root carries it as a single-line or a block require.
	if n := strings.Count(string(merged), pluginPath+" v0.0.0"); n != 1 {
		t.Fatalf("overlay go.mod has %d require entries for %s:\n%s", n, pluginPath, merged)
	}
	// The merged file must stay parseable by the go command (same check
	// as the MVS drift test: `go mod graph` parses without the replace
	// target's directory context, but local-dir replaces from the root
	// go.mod must be absolutized for the scratch copy).
	checkDir := t.TempDir()
	absRoot := filepath.ToSlash(root)
	patched := strings.ReplaceAll(string(merged), "=> ./plugins/", "=> "+absRoot+"/plugins/")
	if err := os.WriteFile(filepath.Join(checkDir, "go.mod"), []byte(patched), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(goToolchainPath(t), "mod", "graph")
	cmd.Dir = checkDir
	cmd.Env = os.Environ()
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("merged go.mod is not go-command valid: %v: %s", err, output)
	}
}

// TestOverlayGoModAllowsMVSVersionDrift: a plugin may require a module
// the root already requires at a different version. Both require lines
// land in the overlay and Go's minimal-version selection resolves to the
// higher one — the merged file must stay parseable by the go command.
func TestOverlayGoModAllowsMVSVersionDrift(t *testing.T) {
	root, err := findModuleRoot(".")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	drift := "module example.com/vivy/drifted\n\ngo 1.26.4\n\nrequire gopkg.in/yaml.v3 v3.0.0\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(drift), 0o600); err != nil {
		t.Fatal(err)
	}
	p := packedPlugin{dir: dir, name: "drifted", impPath: "example.com/vivy/drifted", standalone: true}
	out, err := overlayGoModForStandalone(root, t.TempDir(), []packedPlugin{p})
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "require gopkg.in/yaml.v3 v3.0.0\n") {
		t.Fatalf("overlay go.mod misses the plugin's drifted require:\n%s", data)
	}
	// The merged file is validated by the go command: copy it as go.mod
	// into a scratch dir and check that MVS selected the higher of the
	// two yaml.v3 versions. The root may carry local-dir replaces
	// (`./plugins/...`, full species body); a scratch copy cannot resolve
	// those relative targets, so they are absolutized to the root first.
	checkDir := t.TempDir()
	data2, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	absRoot := filepath.ToSlash(root)
	patched := strings.ReplaceAll(string(data2), "=> ./plugins/", "=> "+absRoot+"/plugins/")
	if err := os.WriteFile(filepath.Join(checkDir, "go.mod"), []byte(patched), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(goToolchainPath(t), "mod", "graph")
	cmd.Dir = checkDir
	cmd.Env = os.Environ()
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("merged go.mod is not go-command valid: %v: %s", err, output)
	}
	if !strings.Contains(string(output), "gopkg.in/yaml.v3@v3.0.1") {
		t.Fatalf("MVS did not select the higher version: %s", output)
	}
}

// goToolchainPath returns the go binary path for subprocess checks.
func goToolchainPath(t *testing.T) string {
	t.Helper()
	goBin := filepath.Join(runtime.GOROOT(), "bin", "go")
	if runtime.GOOS == "windows" {
		goBin += ".exe"
	}
	if _, err := os.Stat(goBin); err != nil {
		t.Fatalf("go toolchain missing: %v", err)
	}
	return goBin
}

// TestOverlayGoSumMergesPluginChecksums: the overlaid go.sum gains the
// telego closure checksums while keeping every root entry.
func TestOverlayGoSumMergesPluginChecksums(t *testing.T) {
	root, err := findModuleRoot(".")
	if err != nil {
		t.Fatal(err)
	}
	tmp := t.TempDir()
	out, err := overlayGoSumForStandalone(root, tmp, []packedPlugin{telegramStandalonePlugin(root)})
	if err != nil {
		t.Fatal(err)
	}
	if out == "" {
		t.Fatal("the telegram plugin brings go.sum entries; an overlay must exist")
	}
	merged, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(merged), "github.com/mymmrac/telego v1.10.0 h1:") {
		t.Fatalf("overlay go.sum misses the telego module hash:\n%s", merged)
	}
	rootSum, err := os.ReadFile(filepath.Join(root, "go.sum"))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(rootSum), "\n") {
		if line != "" && !strings.Contains(string(merged), line) {
			t.Fatalf("overlay go.sum lost the root entry %q", line)
		}
	}
}

// TestOverlaySkipsDepFreeStandalonePlugin: a standalone plugin without a
// go.sum (or with only the species-module require) produces no overlay
// entries — the C2 behavior is unchanged for dependency-free plugins.
func TestOverlaySkipsDepFreeStandalonePlugin(t *testing.T) {
	root, err := findModuleRoot(".")
	if err != nil {
		t.Fatal(err)
	}
	fake := packedPlugin{
		dir:        filepath.Join(root, "sdk", "internal", "testdata", "fake-channel"),
		name:       "fake-channel",
		impPath:    "example.com/vivy/fake-channel",
		standalone: true,
	}
	tmp := t.TempDir()
	goModOut, err := overlayGoModForStandalone(root, tmp, []packedPlugin{fake})
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(goModOut)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "require agent-vivy v0.0.0") {
		t.Fatalf("overlay go.mod self-requires the species module:\n%s", data)
	}
	goSumOut, err := overlayGoSumForStandalone(root, tmp, []packedPlugin{fake})
	if err != nil {
		t.Fatal(err)
	}
	if goSumOut == "" {
		t.Fatal("the pack.sum seed must exist for the -modfile build pair")
	}
	seeded, err := os.ReadFile(goSumOut)
	if err != nil {
		t.Fatal(err)
	}
	rootSum, err := os.ReadFile(filepath.Join(root, "go.sum"))
	if err != nil {
		t.Fatal(err)
	}
	if string(seeded) != string(rootSum) {
		t.Fatal("a dependency-free standalone plugin must not change the go.sum seed")
	}
}

// TestPackTelegramStandaloneModule is the CH-C4 acceptance: the real
// `pack --with telegram` builds a candidate EXE that links telego through
// the go.mod+go.sum overlay, and the live species go.mod AND go.sum stay
// byte-identical.
func TestPackTelegramStandaloneModule(t *testing.T) {
	root, err := findModuleRoot(".")
	if err != nil {
		t.Fatal(err)
	}
	liveRegister := filepath.Join(root, "internal", "generated", "plugins", "zz_register.go")
	liveGoMod := filepath.Join(root, "go.mod")
	liveGoSum := filepath.Join(root, "go.sum")
	before := map[string]string{}
	for _, path := range []string{liveRegister, liveGoMod, liveGoSum} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		before[path] = string(data)
	}
	out := t.TempDir()
	art, err := Pack(packOptions{With: []string{"telegram"}, Out: out})
	if err != nil {
		t.Fatalf("pack --with telegram: %v", err)
	}
	for path, want := range before {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != want {
			t.Fatalf("pack mutated the live %s", path)
		}
	}
	if len(art.Recipe.Plugins) != 1 || art.Recipe.Plugins[0] != "telegram" {
		t.Fatalf("recipe = %+v", art.Recipe)
	}
	if len(art.Tools) != 0 {
		t.Fatalf("channel plugin must not contribute tools: %+v", art.Tools)
	}
	exe := strings.TrimPrefix(art.SourceRef, fileRefPrefix)
	if _, err := os.Stat(exe); err != nil {
		t.Fatalf("packed exe missing: %v", err)
	}
	// The candidate EXE links the telego closure: the importable symbol
	// must be present in the binary's dependency table.
	linked, err := exeContainsTelego(exe)
	if err != nil {
		t.Fatalf("inspect exe: %v", err)
	}
	if !linked {
		t.Fatal("the packed exe does not link the telego closure")
	}
	inspected, err := InspectArtifact(out)
	if err != nil {
		t.Fatal(err)
	}
	if inspected.ID != art.ID || inspected.ArtifactSHA256 != art.ArtifactSHA256 {
		t.Fatalf("inspect-artifact = %+v", inspected)
	}
}

// TestPackStandaloneRejectsForkReplace: a standalone plugin go.mod with a
// fork replace (or any exclude) fails the pack loudly with the offending
// line — pack must not silently rewrite the merged build's dependency
// graph (review L4). The species-module replace stays legal in every form.
func TestPackStandaloneRejectsForkReplace(t *testing.T) {
	root, err := findModuleRoot(".")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	goMod := `module example.com/vivy/forked

go 1.26.4

require agent-vivy v0.0.0

replace agent-vivy => ../..

replace github.com/some/dep v1.0.0 => example.com/fork/dep v1.0.1
`
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goMod), 0o600); err != nil {
		t.Fatal(err)
	}
	p := packedPlugin{dir: dir, name: "forked", impPath: "example.com/vivy/forked", standalone: true}
	_, err = overlayGoModForStandalone(root, t.TempDir(), []packedPlugin{p})
	want := "sdk: plugin forked go.mod has replace/exclude directives pack cannot merge " +
		"(line 9: replace github.com/some/dep v1.0.0 => example.com/fork/dep v1.0.1)"
	if err == nil || err.Error() != want {
		t.Fatalf("err = %v, want %q", err, want)
	}

	// exclude (single line and block form) is equally loud.
	if err := checkMergeableDirectives("forked", "module example.com/vivy/forked\n\nexclude github.com/some/dep v1.0.0\n"); err == nil ||
		!strings.Contains(err.Error(), "pack cannot merge (line 3: exclude github.com/some/dep v1.0.0)") {
		t.Fatalf("exclude err = %v", err)
	}
	if err := checkMergeableDirectives("forked", "module example.com/vivy/forked\n\nexclude (\n\tgithub.com/some/dep v1.0.0\n)\n"); err == nil ||
		!strings.Contains(err.Error(), "pack cannot merge (line 4: github.com/some/dep v1.0.0)") {
		t.Fatalf("block exclude err = %v", err)
	}

	// The species-module replace is legal in single-line and block form.
	legal := "module example.com/vivy/ok\n\ngo 1.26.4\n\nreplace agent-vivy => ../..\n"
	if err := checkMergeableDirectives("ok", legal); err != nil {
		t.Fatalf("species replace must stay legal: %v", err)
	}
	if err := checkMergeableDirectives("ok", "module example.com/vivy/ok\n\nreplace (\n\tagent-vivy => ../..\n)\n"); err != nil {
		t.Fatalf("species replace in block form must stay legal: %v", err)
	}
}

// TestPackTwoStandaloneModules packs telegram AND discord — two standalone
// plugin modules with disjoint third-party closures — in one pass (review
// L4). The real build is the acceptance: the merged overlay carries both
// require+replace pairs, the live tree stays byte-identical, and the
// recipe lists both plugins.
func TestPackTwoStandaloneModules(t *testing.T) {
	root, err := findModuleRoot(".")
	if err != nil {
		t.Fatal(err)
	}
	live := map[string]string{}
	for _, name := range []string{
		filepath.Join("internal", "generated", "plugins", "zz_register.go"),
		"go.mod",
		"go.sum",
	} {
		path := filepath.Join(root, name)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		live[path] = string(data)
	}
	out := t.TempDir()
	art, err := Pack(packOptions{With: []string{"telegram", "discord"}, Out: out})
	if err != nil {
		t.Fatalf("pack telegram+discord: %v", err)
	}
	for path, want := range live {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != want {
			t.Fatalf("pack mutated the live %s", path)
		}
	}
	if len(art.Recipe.Plugins) != 2 || art.Recipe.Plugins[0] != "telegram" || art.Recipe.Plugins[1] != "discord" {
		t.Fatalf("recipe = %+v, want [telegram discord]", art.Recipe.Plugins)
	}
	if len(art.Plugins) != 2 {
		t.Fatalf("plugins = %+v, want two entries", art.Plugins)
	}
	tgDir, err := filepath.Abs(filepath.Join(root, "plugins", "telegram"))
	if err != nil {
		t.Fatal(err)
	}
	dcDir, err := filepath.Abs(filepath.Join(root, "plugins", "discord"))
	if err != nil {
		t.Fatal(err)
	}
	assertPluginEntry(t, art.Plugins[0], artifactPlugin{
		Name:      "telegram",
		Version:   "0.1.0",
		Seam:      "channel",
		Grants:    []string{"channel.poll", "secret.read"},
		Transport: "poll",
		SourceRef: fileRefPrefix + tgDir,
	})
	assertPluginEntry(t, art.Plugins[1], artifactPlugin{
		Name:      "discord",
		Version:   "0.1.0",
		Seam:      "channel",
		Grants:    []string{"channel.poll", "secret.read"},
		Transport: "poll",
		SourceRef: fileRefPrefix + dcDir,
	})
	exe := strings.TrimPrefix(art.SourceRef, fileRefPrefix)
	if _, err := os.Stat(exe); err != nil {
		t.Fatalf("packed exe missing: %v", err)
	}
	inspected, err := InspectArtifact(out)
	if err != nil {
		t.Fatal(err)
	}
	if inspected.ID != art.ID || inspected.ArtifactSHA256 != art.ArtifactSHA256 {
		t.Fatalf("inspect-artifact = %+v", inspected)
	}
}

// TestPackDedupesRepeatedWith: a repeated identical --with entry is skipped
// after the first — the plugin is verified, registered, and listed once.
func TestPackDedupesRepeatedWith(t *testing.T) {
	out := t.TempDir()
	art, err := Pack(packOptions{With: []string{"hello-fs", "hello-fs"}, Out: out})
	if err != nil {
		t.Fatal(err)
	}
	if len(art.Recipe.Plugins) != 1 || art.Recipe.Plugins[0] != "hello-fs" {
		t.Fatalf("recipe = %+v, want hello-fs exactly once", art.Recipe.Plugins)
	}
}

// exeContainsTelego scans the built exe for the telego module path, which
// survives in the binary as part of the embedded build info / string table.
func exeContainsTelego(exe string) (bool, error) {
	data, err := os.ReadFile(exe)
	if err != nil {
		return false, err
	}
	return strings.Contains(string(data), "github.com/mymmrac/telego"), nil
}

// treeHashPattern is a valid sha256 tree fingerprint as written into the
// generation manifest's seam-classified plugin entries.
var treeHashPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

// assertPluginEntry checks one seam-classified generation entry against the
// manifest projection it must carry (VIVY-CHANNEL-PACK.md §10): identity,
// grants, channel transport (empty off the channel seam), a file: source
// ref pointing at the plugin directory, and a 64-hex tree fingerprint.
func assertPluginEntry(t *testing.T, got, want artifactPlugin) {
	t.Helper()
	if got.Name != want.Name || got.Version != want.Version || got.Seam != want.Seam {
		t.Fatalf("plugin entry = %+v, want name/version/seam of %+v", got, want)
	}
	if !slices.Equal(got.Grants, want.Grants) {
		t.Fatalf("grants = %v, want %v", got.Grants, want.Grants)
	}
	if got.Transport != want.Transport {
		t.Fatalf("transport = %q, want %q", got.Transport, want.Transport)
	}
	if got.SourceRef != want.SourceRef {
		t.Fatalf("source_ref = %q, want %q", got.SourceRef, want.SourceRef)
	}
	if !treeHashPattern.MatchString(got.TreeHash) {
		t.Fatalf("tree_hash = %q, want a 64-hex digest", got.TreeHash)
	}
}

// TestHashPluginTree pins the fingerprint contract: deterministic for the
// same tree, sensitive to any content byte, and covering nested files.
func TestHashPluginTree(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "plugin.go"), []byte("package p\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sub", "note.txt"), []byte("hi"), 0o600); err != nil {
		t.Fatal(err)
	}
	first, err := hashPluginTree(dir)
	if err != nil {
		t.Fatal(err)
	}
	second, err := hashPluginTree(dir)
	if err != nil {
		t.Fatal(err)
	}
	if first != second || !treeHashPattern.MatchString(first) {
		t.Fatalf("tree hash not deterministic 64-hex: %q vs %q", first, second)
	}
	if err := os.WriteFile(filepath.Join(dir, "plugin.go"), []byte("package p // changed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	third, err := hashPluginTree(dir)
	if err != nil {
		t.Fatal(err)
	}
	if third == first {
		t.Fatal("one-byte content change must change the tree hash")
	}
}

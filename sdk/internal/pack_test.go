package sdk

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
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
	if len(art.Tools) != 0 {
		t.Fatalf("channel plugin must not contribute tools: %+v", art.Tools)
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
}

func TestPackHelloFSWritesArtifactAndLeavesLiveRegister(t *testing.T) {
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
func TestOverlayGoModMergesPluginRequires(t *testing.T) {
	root, err := findModuleRoot(".")
	if err != nil {
		t.Fatal(err)
	}
	tmp := t.TempDir()
	out, err := overlayGoModForStandalone(root, tmp, []packedPlugin{telegramStandalonePlugin(root)})
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	merged := string(data)
	if !strings.Contains(merged, "require github.com/mymmrac/telego v1.10.0") {
		t.Fatalf("overlay go.mod misses the plugin's telego require:\n%s", merged)
	}
	if !strings.Contains(merged, "require example.com/vivy/plugins/telegram v0.0.0") {
		t.Fatalf("overlay go.mod misses the plugin module require:\n%s", merged)
	}
	if !strings.Contains(merged, "replace example.com/vivy/plugins/telegram => ") {
		t.Fatalf("overlay go.mod misses the plugin module replace:\n%s", merged)
	}
	// The species module must not require itself (the plugin's own
	// `require agent-vivy v0.0.0` is dropped).
	if strings.Contains(merged, "require agent-vivy v0.0.0") {
		t.Fatalf("overlay go.mod self-requires the species module:\n%s", merged)
	}
	// Dedup: the root already requires e.g. sonic; the plugin's indirect
	// require of a sonic version must not produce a second line for the
	// same path at the same version.
	count := strings.Count(merged, "require github.com/bytedance/sonic/loader")
	if count > 1 {
		t.Fatalf("overlay go.mod duplicates sonic/loader %d times:\n%s", count, merged)
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
	// into a scratch dir (it also needs the replace target's directory
	// context only for builds; `go mod graph` parses the module graph
	// without one) and check that MVS selected the higher of the two
	// yaml.v3 versions.
	checkDir := t.TempDir()
	data2, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(checkDir, "go.mod"), data2, 0o600); err != nil {
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
	if strings.Contains(before[liveGoMod], "telego") {
		t.Fatal("the species go.mod must never gain telego")
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

// exeContainsTelego scans the built exe for the telego module path, which
// survives in the binary as part of the embedded build info / string table.
func exeContainsTelego(exe string) (bool, error) {
	data, err := os.ReadFile(exe)
	if err != nil {
		return false, err
	}
	return strings.Contains(string(data), "github.com/mymmrac/telego"), nil
}

package sdk

import (
	"os"
	"path/filepath"
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

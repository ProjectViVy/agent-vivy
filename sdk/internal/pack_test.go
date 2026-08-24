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

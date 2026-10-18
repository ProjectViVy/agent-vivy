package sdk

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// An external source module owns its third-party pins. Pack must carry its
// version replacements into the temporary build modfile, must never redirect
// the root module, and must fail closed on a conflict or a directory replace.

type packFixture struct {
	root, source, out string
}

func newPackFixture(t *testing.T) packFixture {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example/root\n\ngo 1.26.4\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(root, "source")
	out := filepath.Join(root, "out")
	for _, dir := range []string{source, out} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	return packFixture{root: root, source: source, out: out}
}

func writeSourceModule(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func readModfile(t *testing.T, path string) map[string]any {
	t.Helper()
	cmd := exec.Command("go", "mod", "edit", "-json", "-modfile="+path)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("read modfile: %v: %s", err, out)
	}
	var file map[string]any
	if err := json.Unmarshal(out, &file); err != nil {
		t.Fatal(err)
	}
	return file
}

func TestPrepareBuildModfileMergesExternalPins(t *testing.T) {
	f := newPackFixture(t)
	writeSourceModule(t, f.source, `module example/ext

go 1.26.4

require example/root v0.0.0

require github.com/eino-contrib/acp v0.0.4

replace example/root => ../..

replace github.com/eino-contrib/acp => github.com/mastwet/acp v0.0.0-20261007071707-780fb4dd4b98
`)
	modfile, err := prepareBuildModfile(f.root, f.out, []string{f.source})
	if err != nil {
		t.Fatalf("prepareBuildModfile: %v", err)
	}
	file := readModfile(t, modfile)
	replaces, _ := file["Replace"].([]any)
	var fork, rootRedirect bool
	for _, raw := range replaces {
		entry, _ := raw.(map[string]any)
		old, _ := entry["Old"].(map[string]any)
		next, _ := entry["New"].(map[string]any)
		switch old["Path"] {
		case "github.com/eino-contrib/acp":
			fork = next["Path"] == "github.com/mastwet/acp" && next["Version"] == "v0.0.0-20261007071707-780fb4dd4b98"
		case "example/root":
			rootRedirect = true
		}
	}
	if !fork {
		t.Fatalf("build modfile lacks the external fork replacement: %v", replaces)
	}
	if rootRedirect {
		t.Fatal("build modfile redirects the root module from an external source")
	}
	requires, _ := file["Require"].([]any)
	var ext, forkRequire bool
	for _, raw := range requires {
		entry, _ := raw.(map[string]any)
		switch entry["Path"] {
		case "example/ext":
			ext = true
		case "github.com/eino-contrib/acp":
			forkRequire = true
		}
	}
	if !ext || !forkRequire {
		t.Fatalf("build modfile requires = %v, want example/ext and the fork", requires)
	}
}

func TestPrepareBuildModfileRejectsConflictingPin(t *testing.T) {
	f := newPackFixture(t)
	if err := os.WriteFile(filepath.Join(f.root, "go.mod"), []byte(`module example/root

go 1.26.4

replace github.com/eino-contrib/acp => github.com/eino-contrib/acp v0.0.4
`), 0o600); err != nil {
		t.Fatal(err)
	}
	writeSourceModule(t, f.source, `module example/ext

go 1.26.4

replace github.com/eino-contrib/acp => github.com/mastwet/acp v0.0.0-20261007071707-780fb4dd4b98
`)
	_, err := prepareBuildModfile(f.root, f.out, []string{f.source})
	if err == nil || !strings.Contains(err.Error(), "already replaces") {
		t.Fatalf("conflicting pin error = %v", err)
	}
}

func TestPrepareBuildModfileRejectsDirectoryReplacement(t *testing.T) {
	f := newPackFixture(t)
	writeSourceModule(t, f.source, `module example/ext

go 1.26.4

replace example/dep => ../dep
`)
	_, err := prepareBuildModfile(f.root, f.out, []string{f.source})
	if err == nil || !strings.Contains(err.Error(), "version replacements only") {
		t.Fatalf("directory replacement error = %v", err)
	}
}

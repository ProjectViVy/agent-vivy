package app

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// D4(a) (D-007): source-level import gate. Eino is the single sanctioned
// agent engine and its surface must stay quarantined behind the two
// adapter packages; the surrounding agent-diva / .workspace reference
// material is read-only context and can never become a Go dependency.

func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("repo root: %v", err)
	}
	return root
}

// walkGoSources visits every Go source file under the given repo dirs.
func walkGoSources(t *testing.T, root string, dirs []string, visit func(relPath string, src []byte)) {
	t.Helper()
	for _, dir := range dirs {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if d.Name() == "testdata" || d.Name() == "node_modules" {
					return fs.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") {
				return nil
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			src, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			visit(filepath.ToSlash(rel), src)
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", dir, err)
		}
	}
}

// importPaths returns every import path declared in a Go source file,
// parsed from the AST so comments and string literals never confuse it.
func importPaths(t *testing.T, rel string, src []byte) []string {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, rel, src, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parse %s: %v", rel, err)
	}
	var paths []string
	for _, imp := range file.Imports {
		p, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			t.Fatalf("unquote import in %s: %v", rel, err)
		}
		paths = append(paths, p)
	}
	return paths
}

// The only code allowed to touch the Eino engine lives in internal/runtime
// (engine adapters) and internal/provider (model construction). Any import
// leaking elsewhere couples a layer to the engine that must stay behind
// those two doors (D-007).
func TestEinoImportsQuarantined(t *testing.T) {
	var violations []string
	walkGoSources(t, repoRoot(t), []string{"cmd", "internal", "sdk", "plugins"}, func(rel string, src []byte) {
		if strings.HasSuffix(rel, "_test.go") {
			return
		}
		allowed := strings.HasPrefix(rel, "internal/runtime/") || strings.HasPrefix(rel, "internal/provider/")
		for _, p := range importPaths(t, rel, src) {
			if strings.HasPrefix(p, "github.com/cloudwego/eino") && !allowed {
				violations = append(violations, rel+": "+p)
			}
		}
	})
	if len(violations) > 0 {
		t.Fatalf("eino imports outside internal/runtime and internal/provider: %v", violations)
	}
}

// agent-diva and .workspace hold the read-only reference material this
// repository was distilled from: no source file may import them or point
// file paths at them, or the build silently regains the dependency it was
// separated from.
func TestNoReferenceMaterialDependencies(t *testing.T) {
	banned := []string{"agent-diva", ".workspace"}
	var violations []string
	walkGoSources(t, repoRoot(t), []string{"cmd", "internal", "sdk", "plugins"}, func(rel string, src []byte) {
		for _, p := range importPaths(t, rel, src) {
			for _, b := range banned {
				if strings.Contains(p, b) {
					violations = append(violations, rel+" imports "+p)
				}
			}
		}
	})
	if len(violations) > 0 {
		t.Fatalf("reference material entered the dependency graph: %v", violations)
	}
}

// User plugins and the public SDK window may not import the kernel.
func TestPluginWindowCannotImportInternal(t *testing.T) {
	var violations []string
	walkGoSources(t, repoRoot(t), []string{"sdk/plugin", "plugins"}, func(rel string, src []byte) {
		for _, p := range importPaths(t, rel, src) {
			if strings.HasPrefix(p, "agent-vivy/internal/") {
				violations = append(violations, rel+" imports "+p)
			}
		}
	})
	if len(violations) > 0 {
		t.Fatalf("plugin window imported the kernel: %v", violations)
	}
}

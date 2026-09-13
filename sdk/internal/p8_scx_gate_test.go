package sdk

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	assemblyv1 "agent-vivy/sdk/internal/assembly"
	"agent-vivy/sdk/module"
)

func TestP8SCXGateARejectsDirectImplementationDependencies(t *testing.T) {
	for name, importPath := range map[string]string{
		"runtime":         "agent-vivy/internal/runtime",
		"eino":            "github.com/cloudwego/eino/schema",
		"public-provider": "agent-vivy/plugins/hello-fs",
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			source := "package scx\n\nimport _ \"" + importPath + "\"\n\nfunc New() {}\nfunc NewProvider() {}\n"
			if err := os.WriteFile(filepath.Join(dir, "provider.go"), []byte(source), 0o600); err != nil {
				t.Fatal(err)
			}
			digest, err := assemblyv1.HashSourceTree(dir, "")
			if err != nil {
				t.Fatal(err)
			}
			descriptor := module.Descriptor{Source: module.Source{SHA256: digest}}
			err = verifySource(dir, descriptor)
			if err == nil || !strings.Contains(err.Error(), "forbidden source import "+importPath) {
				t.Fatalf("verifySource() error = %v, want forbidden import %s", err, importPath)
			}
		})
	}
}

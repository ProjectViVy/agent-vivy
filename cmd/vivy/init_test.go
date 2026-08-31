package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitAgentsMDCreatesTemplate(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0o600); err != nil {
		t.Fatalf("seed project file: %v", err)
	}
	path, hints, err := initAgentsMD(dir)
	if err != nil {
		t.Fatalf("initAgentsMD: %v", err)
	}
	if path != filepath.Join(dir, "AGENTS.md") {
		t.Fatalf("path = %q", path)
	}
	if len(hints) != 0 {
		t.Fatalf("hints = %v, want none", hints)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read generated file: %v", err)
	}
	content := string(data)
	for _, want := range []string{"non-obvious", "## Project overview", "## Build, test, verify", "## Known pitfalls"} {
		if !strings.Contains(content, want) {
			t.Fatalf("template missing %q", want)
		}
	}
}

func TestInitAgentsMDRefusesEmptyDirectory(t *testing.T) {
	dir := t.TempDir()
	// A bare .git checkout is still an empty project for this purpose.
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0o700); err != nil {
		t.Fatalf("seed .git: %v", err)
	}
	if _, _, err := initAgentsMD(dir); err == nil {
		t.Fatal("empty directory accepted")
	}
}

func TestInitAgentsMDRefusesOverwrite(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("x"), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("existing"), 0o600); err != nil {
		t.Fatalf("seed AGENTS.md: %v", err)
	}
	if _, _, err := initAgentsMD(dir); err == nil {
		t.Fatal("overwrite accepted")
	}
	data, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if err != nil || string(data) != "existing" {
		t.Fatalf("existing AGENTS.md must stay untouched (err=%v, content=%q)", err, data)
	}
}

func TestInitAgentsMDProbesExistingRuleFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module x"), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".cursorrules"), []byte("rules"), 0o600); err != nil {
		t.Fatalf("seed .cursorrules: %v", err)
	}
	copilot := filepath.Join(dir, ".github", "copilot-instructions.md")
	if err := os.MkdirAll(filepath.Dir(copilot), 0o700); err != nil {
		t.Fatalf("seed .github: %v", err)
	}
	if err := os.WriteFile(copilot, []byte("rules"), 0o600); err != nil {
		t.Fatalf("seed copilot rules: %v", err)
	}
	_, hints, err := initAgentsMD(dir)
	if err != nil {
		t.Fatalf("initAgentsMD: %v", err)
	}
	if len(hints) != 2 || hints[0] != ".cursorrules" || hints[1] != filepath.Join(".github", "copilot-instructions.md") {
		t.Fatalf("hints = %v", hints)
	}
	data, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if err != nil {
		t.Fatalf("read generated file: %v", err)
	}
	if !strings.Contains(string(data), ".cursorrules") {
		t.Fatal("generated template must reference the probed rule files")
	}
}

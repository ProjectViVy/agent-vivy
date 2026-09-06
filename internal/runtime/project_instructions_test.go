package runtime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscoverProjectInstructionsRootAgentsMD(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("root rules"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "ui"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "ui", "AGENTS.md"), []byte("ui rules"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := DiscoverProjectInstructions(root)
	if err != nil {
		t.Fatal(err)
	}
	wantRoot, err := CanonicalInstructionRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	if got.Root != wantRoot {
		t.Fatalf("root = %q, want %q", got.Root, wantRoot)
	}
	if len(got.AgentsMDFiles) != 1 || got.AgentsMDFiles[0] != "AGENTS.md" {
		t.Fatalf("files = %v, want [AGENTS.md] without nested ui/AGENTS.md", got.AgentsMDFiles)
	}
}

func TestDiscoverProjectInstructionsWalksUpToGitRoot(t *testing.T) {
	repo := t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "AGENTS.md"), []byte("repo"), 0o600); err != nil {
		t.Fatal(err)
	}
	pkg := filepath.Join(repo, "pkg")
	if err := os.Mkdir(pkg, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkg, "AGENTS.md"), []byte("pkg"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := DiscoverProjectInstructions(pkg)
	if err != nil {
		t.Fatal(err)
	}
	wantRepo, err := CanonicalInstructionRoot(repo)
	if err != nil {
		t.Fatal(err)
	}
	if got.Root != wantRepo {
		t.Fatalf("backend root = %q, want git root %q", got.Root, wantRepo)
	}
	if len(got.AgentsMDFiles) != 2 || got.AgentsMDFiles[0] != "AGENTS.md" || filepath.ToSlash(got.AgentsMDFiles[1]) != "pkg/AGENTS.md" {
		t.Fatalf("files = %v, want git-root first then crate-local", got.AgentsMDFiles)
	}
}

func TestDiscoverProjectInstructionsMissingAgentsMDStillListsDefault(t *testing.T) {
	root := t.TempDir()
	got, err := DiscoverProjectInstructions(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.AgentsMDFiles) != 1 || got.AgentsMDFiles[0] != "AGENTS.md" {
		t.Fatalf("files = %v, want default AGENTS.md", got.AgentsMDFiles)
	}
}

func TestDiscoverProjectInstructionsSkillDirs(t *testing.T) {
	repo := t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}
	writeSkillPackage(t, filepath.Join(repo, ".agents", "skills", "repo-skill"))
	pkg := filepath.Join(repo, "pkg")
	if err := os.Mkdir(pkg, 0o700); err != nil {
		t.Fatal(err)
	}
	writeSkillPackage(t, filepath.Join(pkg, ".agents", "skills", "pkg-skill"))
	writeSkillPackage(t, filepath.Join(pkg, ".vivy", "skills", "vivy-skill"))
	writeSkillPackage(t, filepath.Join(pkg, ".claude", "skills", "claude-skill"))
	writeSkillPackage(t, filepath.Join(pkg, ".agents", "skills", "pkg-skill", "nested"))

	got, err := DiscoverProjectInstructions(pkg)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(got.SkillRoots, "\n")
	if !strings.Contains(joined, filepath.Join(".agents", "skills")) {
		t.Fatalf("skill roots missing .agents/skills: %v", got.SkillRoots)
	}
	if len(got.SkillRoots) != 3 {
		t.Fatalf("skill roots = %v, want cwd .agents, cwd .vivy, git-root .agents", got.SkillRoots)
	}
	for _, root := range got.SkillRoots {
		if strings.Contains(root, ".claude") {
			t.Fatalf("unexpected .claude skill root: %v", got.SkillRoots)
		}
	}
	if !strings.HasSuffix(filepath.ToSlash(got.SkillRoots[0]), "/pkg/.agents/skills") &&
		!strings.HasSuffix(filepath.ToSlash(got.SkillRoots[0]), "pkg/.agents/skills") {
		t.Fatalf("first skill root = %q, want cwd .agents/skills", got.SkillRoots[0])
	}
}

func TestDiscoverProjectInstructionsSkipsSymlinkAgentsMD(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target.md")
	if err := os.WriteFile(target, []byte("nope"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "AGENTS.md")
	if err := os.Symlink(target, link); err != nil {
		t.Skip("symlink not permitted")
	}
	got, err := DiscoverProjectInstructions(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.AgentsMDFiles) != 1 || got.AgentsMDFiles[0] != "AGENTS.md" {
		t.Fatalf("files = %v, want default placeholder after skipping symlink", got.AgentsMDFiles)
	}
	if regularFile(link) {
		t.Fatal("symlink must not count as a regular AGENTS.md")
	}
}

func TestCanonicalInstructionRootRejectsMissing(t *testing.T) {
	if _, err := CanonicalInstructionRoot(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("expected error for missing root")
	}
}

func writeSkillPackage(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	name := filepath.Base(dir)
	doc := "---\nname: " + name + "\ndescription: test\n---\n\nbody\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(doc), 0o600); err != nil {
		t.Fatal(err)
	}
}

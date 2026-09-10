package runtime

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHostedSkillBackendListAndGetTraverseSkillHost(t *testing.T) {
	base, root, _ := openSkillTestBackend(t)
	writeSkillFixture(t, root, "demo-skill", "Use this carefully.")
	hosted, err := NewHostedSkillBackend(base)
	if err != nil {
		t.Fatal(err)
	}

	items, err := hosted.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Name != "demo-skill" {
		t.Fatalf("hosted List = %#v", items)
	}
	loaded, err := hosted.Get(context.Background(), "demo-skill")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Content != "Use this carefully." || filepath.Base(loaded.BaseDirectory) != "demo-skill" {
		t.Fatalf("hosted Get = %#v", loaded)
	}
}

func TestHostedSkillBackendOmitsDisabledSkill(t *testing.T) {
	base, root, _ := openSkillTestBackend(t)
	dir := filepath.Join(root, "disabled")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	doc := "---\nname: disabled\ndescription: Disabled skill\nenabled: false\n---\n\nhidden\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(doc), 0o600); err != nil {
		t.Fatal(err)
	}
	hosted, err := NewHostedSkillBackend(base)
	if err != nil {
		t.Fatal(err)
	}
	items, err := hosted.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("disabled skill leaked through SkillHost: %#v", items)
	}
	if _, err := hosted.Get(context.Background(), "disabled"); err == nil {
		t.Fatal("disabled skill Get should fail")
	}
}

func TestHostedAlwaysSkillsUsesSkillHostResolution(t *testing.T) {
	base, root, _ := openSkillTestBackend(t)
	dir := filepath.Join(root, "always-skill")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	doc := "---\nname: always-skill\ndescription: Always skill\nalways: true\n---\n\nalways body\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(doc), 0o600); err != nil {
		t.Fatal(err)
	}
	hosted, err := NewHostedSkillBackend(base)
	if err != nil {
		t.Fatal(err)
	}
	content, err := hosted.AlwaysSkills(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(content, "### Skill: always-skill") || !strings.Contains(content, "always body") {
		t.Fatalf("always skill output = %q", content)
	}
}

func TestHostedSkillBackendRedactsSecretLikeSkillContent(t *testing.T) {
	base, root, _ := openSkillTestBackend(t)
	writeSkillFixture(t, root, "secret-skill", "Use sk-test-12345678901234567890 carefully.")
	hosted, err := NewHostedSkillBackend(base)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := hosted.Get(context.Background(), "secret-skill")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(loaded.Content, "sk-test-") {
		t.Fatalf("secret-like content leaked from SkillHost: %q", loaded.Content)
	}
}

package runtime

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/tools"
	"agent-vivy/sdk/port/skillsource"
)

type recordingSkillSource struct {
	request chan skillsource.Request
}

func (source *recordingSkillSource) ID() string { return "fixture.engine-skill" }
func (source *recordingSkillSource) List(_ context.Context, request skillsource.Request) ([]skillsource.Summary, error) {
	source.request <- request
	return []skillsource.Summary{{ID: "engine-skill", Name: "engine-display", Version: "v1", SourceHash: "source", Available: true}}, nil
}
func (source *recordingSkillSource) Get(_ context.Context, request skillsource.Request, id string) (skillsource.Skill, error) {
	source.request <- request
	return skillsource.Skill{ID: id, Name: "engine-display", Version: "v1", SourceHash: "source", Available: true, Content: "engine body"}, nil
}

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

func TestHostedSkillBackendPreservesIDDifferentFromEinoName(t *testing.T) {
	source := &recordingSkillSource{request: make(chan skillsource.Request, 4)}
	hosted, err := NewHostedSkillBackend(nil, source)
	if err != nil {
		t.Fatal(err)
	}
	items, err := hosted.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Name != "engine-skill" {
		t.Fatalf("Eino list did not expose stable ID: %#v", items)
	}
	loaded, err := hosted.Get(context.Background(), "engine-skill")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Content != "engine body" || loaded.Name != "engine-display" {
		t.Fatalf("Eino get resolved the wrong hosted skill: %#v", loaded)
	}
}

func TestHostedSkillBackendDoesNotCollapseLocalOverlayConflicts(t *testing.T) {
	base, root, _ := openSkillTestBackend(t)
	project := t.TempDir()
	writeSkillFixture(t, root, "same-skill", "user body")
	writeSkillFixture(t, project, "same-skill", "project body")
	if err := base.SetProjectSkillRoots([]string{project}); err != nil {
		t.Fatal(err)
	}
	hosted, err := NewHostedSkillBackend(base)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := hosted.List(context.Background()); err == nil || !strings.Contains(err.Error(), "duplicate skill id") {
		t.Fatalf("local conflict was collapsed before SkillHost: %v", err)
	}
}

func TestEngineSkillSourceReceivesLiveWorkspaceIdentity(t *testing.T) {
	base, _, _ := openSkillTestBackend(t)
	source := &recordingSkillSource{request: make(chan skillsource.Request, 4)}
	model := &recordingModel{inner: NewScriptedModel(schema.AssistantMessage("done", nil))}
	items, err := tools.Builtin(nil).Resolve([]string{tools.EchoInfoName})
	if err != nil {
		t.Fatal(err)
	}
	engine, err := NewEngine(context.Background(), model, items, EngineConfig{
		StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10, SkillBackend: base, SkillSources: []skillsource.Provider{source},
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx := withWorkspaceID(withSessionID(withRunID(context.Background(), domain.RunID("run-live-skill")), domain.SessionID("session-live-skill")), "workspace-live-skill")
	iter := engine.RunHistory(ctx, []*schema.Message{schema.UserMessage("hello")})
	deadline := time.After(5 * time.Second)
	for {
		event, ok := iter.Next()
		if !ok {
			break
		}
		if event != nil && event.Err != nil {
			t.Fatalf("engine run: %v", event.Err)
		}
	}
	select {
	case request := <-source.request:
		if request.SessionID != "session-live-skill" || request.WorkspaceID != "workspace-live-skill" {
			t.Fatalf("Skill Source identity = %+v, want live session/workspace", request)
		}
	case <-deadline:
		t.Fatal("Engine did not query Skill Source")
	}
}

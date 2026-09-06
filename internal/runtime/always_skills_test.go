package runtime

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	einoskill "github.com/cloudwego/eino/adk/middlewares/skill"
	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/tools"
)

func writeAlwaysSkillFixture(t *testing.T, root, name, frontmatter, body string) {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	doc := "---\n" + frontmatter + "\n---\n\n" + body + "\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(doc), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestEinoSkillBackendAlwaysSkillsSelection(t *testing.T) {
	root := t.TempDir()
	writeAlwaysSkillFixture(t, root, "alpha", "name: alpha\ndescription: always on\nalways: true\n", "alpha body")
	writeAlwaysSkillFixture(t, root, "beta", "name: beta\ndescription: always but disabled\nalways: true\nenabled: false\n", "beta body")
	writeAlwaysSkillFixture(t, root, "delta", "name: delta\ndescription: no always flag\n", "delta body")
	writeAlwaysSkillFixture(t, root, "epsilon", "name: epsilon\ndescription: oversized\nalways: true\n", strings.Repeat("o", alwaysFileMaxChars+1))
	backend, err := NewEinoSkillBackend(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	got, err := backend.AlwaysSkills(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := "### Skill: alpha\n\nalpha body"
	if got != want {
		t.Fatalf("AlwaysSkills = %q, want %q", got, want)
	}
}

func TestEinoSkillBackendAlwaysSkillsSlugOrderAndBudget(t *testing.T) {
	root := t.TempDir()
	writeAlwaysSkillFixture(t, root, "zeta", "name: zeta\ndescription: second\nalways: true\n", strings.Repeat("a", 1200))
	writeAlwaysSkillFixture(t, root, "eta", "name: eta\ndescription: first\nalways: true\n", strings.Repeat("b", 1200))
	backend, err := NewEinoSkillBackend(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	got, err := backend.AlwaysSkills(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	sections := strings.Split(got, "\n\n---\n\n")
	if len(sections) != 2 {
		t.Fatalf("sections = %d, want 2", len(sections))
	}
	if !strings.HasPrefix(sections[0], "### Skill: eta\n\n") {
		t.Fatalf("first section = %q, want eta first (slug order)", sections[0])
	}
	if !strings.HasPrefix(sections[1], "### Skill: zeta\n\n") {
		t.Fatalf("second section = %q, want zeta second", sections[1])
	}
	if got := utf8RuneCount(strings.TrimPrefix(sections[0], "### Skill: eta\n\n")); got != 1200 {
		t.Fatalf("eta body runes = %d, want 1200", got)
	}
	// zeta fills the remaining total budget: 2000 - 1200 = 800 runes.
	if got := utf8RuneCount(strings.TrimPrefix(sections[1], "### Skill: zeta\n\n")); got != 800 {
		t.Fatalf("zeta body runes = %d, want 800 (total budget)", got)
	}
}

func utf8RuneCount(s string) int { return len([]rune(s)) }

func TestRenderSkillDocumentPreservesAlways(t *testing.T) {
	front, _, content, err := parseSkillDocument([]byte("---\nname: keeper\ndescription: d\nalways: true\n---\n\nbody\n"))
	if err != nil {
		t.Fatal(err)
	}
	if !front.Always {
		t.Fatal("parseSkillDocument did not read always: true")
	}
	rendered, err := renderSkillDocument(front, content)
	if err != nil {
		t.Fatal(err)
	}
	reread, _, _, err := parseSkillDocument(rendered)
	if err != nil {
		t.Fatal(err)
	}
	if !reread.Always {
		t.Fatalf("canonical re-render dropped always: %s", rendered)
	}
	frontOff, _, contentOff, err := parseSkillDocument([]byte("---\nname: keeper\ndescription: d\nalways: false\n---\n\nbody\n"))
	if err != nil {
		t.Fatal(err)
	}
	if frontOff.Always {
		t.Fatal("always: false parsed as true")
	}
	renderedOff, err := renderSkillDocument(frontOff, contentOff)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(renderedOff), "always") {
		t.Fatalf("canonical re-render kept an explicit always: false: %s", renderedOff)
	}
}

// bareSkillBackend satisfies einoskill.Backend without the AlwaysSkills
// capability: engines wired to it must inject nothing.
type bareSkillBackend struct{}

func (bareSkillBackend) List(context.Context) ([]einoskill.FrontMatter, error) { return nil, nil }
func (bareSkillBackend) Get(context.Context, string) (einoskill.Skill, error) {
	return einoskill.Skill{}, errors.New("not found")
}

func alwaysSkillsTestEngine(t *testing.T, skillBackend einoskill.Backend, script ...*schema.Message) (*Engine, *recordingModel) {
	t.Helper()
	ctx := context.Background()
	ts, err := tools.Builtin(nil).Resolve([]string{tools.EchoInfoName})
	if err != nil {
		t.Fatalf("resolve tools: %v", err)
	}
	rec := &recordingModel{inner: NewScriptedModel(script...)}
	eng, err := NewEngine(ctx, rec, ts, EngineConfig{
		StreamBuffer:         8,
		MaxEventPayloadBytes: 64 << 10,
		SkillBackend:         skillBackend,
	})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	return eng, rec
}

func alwaysSkillsTestScript() []*schema.Message {
	return []*schema.Message{
		schema.AssistantMessage("", []schema.ToolCall{{
			ID:       "call-always-1",
			Function: schema.FunctionCall{Name: tools.EchoInfoName, Arguments: `{"text":"probe"}`},
		}}),
		schema.AssistantMessage("ALWAYS-SKILLS-TEST-DONE", nil),
	}
}

const alwaysSkillsTestMarker = "ALWAYS-SKILLS-MARKER-ONE"

func TestEngineAlwaysSkillsInjection(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	writeAlwaysSkillFixture(t, root, "always-probe", "name: always-probe\ndescription: always on\nalways: true\n", alwaysSkillsTestMarker)
	writeAlwaysSkillFixture(t, root, "quiet-skill", "name: quiet-skill\ndescription: not injected\n", "never injected body")
	backend, err := NewEinoSkillBackend(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	eng, rec := alwaysSkillsTestEngine(t, backend, alwaysSkillsTestScript()...)
	runID := domain.RunID("run-always-skills-1")
	final := drainFinalText(t, eng.RunHistory(withRunID(ctx, runID), []*schema.Message{
		schema.UserMessage("echo something"),
	}))
	if final != "ALWAYS-SKILLS-TEST-DONE" {
		t.Fatalf("final answer = %q", final)
	}
	inputs := rec.snapshot()
	if len(inputs) != 2 {
		t.Fatalf("model calls = %d, want 2", len(inputs))
	}
	for i, input := range inputs {
		count := 0
		injectedAt, firstRealUser := -1, -1
		for j, msg := range input {
			if msg == nil || msg.Role != schema.User {
				continue
			}
			// The official Eino tool-search middleware's transient reminder
			// also uses the user role; it is not the real request turn.
			if strings.HasPrefix(msg.Content, "<available-deferred-tools>") {
				continue
			}
			if msg.Extra != nil {
				if _, ok := msg.Extra[alwaysSkillsExtraKey]; ok {
					count++
					if injectedAt < 0 {
						injectedAt = j
					}
					continue
				}
			}
			if firstRealUser < 0 {
				firstRealUser = j
			}
		}
		if count != 1 {
			t.Fatalf("model call %d: injected messages = %d, want 1 (idempotent across turns)", i, count)
		}
		if injectedAt != firstRealUser-1 {
			t.Fatalf("model call %d: injection at %d, first real user message at %d", i, injectedAt, firstRealUser)
		}
		if !strings.Contains(input[injectedAt].Content, "## Active Skills") || !strings.Contains(input[injectedAt].Content, "### Skill: always-probe") || !strings.Contains(input[injectedAt].Content, alwaysSkillsTestMarker) {
			t.Fatalf("model call %d: injected content = %q", i, input[injectedAt].Content)
		}
		if strings.Contains(input[injectedAt].Content, "quiet-skill") || strings.Contains(input[injectedAt].Content, "never injected body") {
			t.Fatalf("model call %d: non-always skill leaked: %q", i, input[injectedAt].Content)
		}
	}
}

func TestEngineAlwaysSkillsRequiresCapability(t *testing.T) {
	ctx := context.Background()
	eng, rec := alwaysSkillsTestEngine(t, bareSkillBackend{}, alwaysSkillsTestScript()...)
	runID := domain.RunID("run-always-skills-bare")
	final := drainFinalText(t, eng.RunHistory(withRunID(ctx, runID), []*schema.Message{
		schema.UserMessage("echo something"),
	}))
	if final != "ALWAYS-SKILLS-TEST-DONE" {
		t.Fatalf("final answer = %q", final)
	}
	for i, input := range rec.snapshot() {
		for _, msg := range input {
			if msg == nil {
				continue
			}
			if msg.Extra != nil {
				if _, ok := msg.Extra[alwaysSkillsExtraKey]; ok {
					t.Fatalf("model call %d: bare backend must not inject", i)
				}
			}
			if strings.Contains(msg.Content, "## Active Skills") {
				t.Fatalf("model call %d: bare backend injected Active Skills", i)
			}
		}
	}
}

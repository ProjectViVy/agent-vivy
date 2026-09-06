package runtime

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/internal/tools"
)

type fakeTitleGenerator struct {
	gotUser      string
	gotAssistant string
	title        string
	err          error
}

func (f *fakeTitleGenerator) GenerateTitle(_ context.Context, userText, assistantText string) (string, error) {
	f.gotUser = userText
	f.gotAssistant = assistantText
	return f.title, f.err
}

func newTitleService(t *testing.T, gen TitleGenerator) (*Service, *sqlite.Backend) {
	t.Helper()
	ctx := context.Background()
	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "titler.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = backend.Close() })
	ts, err := tools.Builtin(backend).Resolve([]string{tools.EchoInfoName})
	if err != nil {
		t.Fatalf("resolve tools: %v", err)
	}
	eng, err := NewEngine(ctx, NewScriptedModel(schema.AssistantMessage("first answer", nil)), ts, EngineConfig{
		StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10,
	})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	svc := NewService(eng, "scripted", "scripted-v0", ServiceDeps{
		Journal: backend, Runs: backend, Messages: backend, Sessions: backend, Sink: newTestSink(),
		Titles: gen,
	})
	return svc, backend
}

// newTitleServiceWithModel wires the titler harness over a custom model so a
// test can drive runs that fail before completion.
func newTitleServiceWithModel(t *testing.T, gen TitleGenerator, m model.ToolCallingChatModel) (*Service, *sqlite.Backend) {
	t.Helper()
	ctx := context.Background()
	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "titler.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = backend.Close() })
	ts, err := tools.Builtin(backend).Resolve([]string{tools.EchoInfoName})
	if err != nil {
		t.Fatalf("resolve tools: %v", err)
	}
	eng, err := NewEngine(ctx, m, ts, EngineConfig{
		StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10,
	})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	svc := NewService(eng, "scripted", "scripted-v0", ServiceDeps{
		Journal: backend, Runs: backend, Messages: backend, Sessions: backend, Sink: newTestSink(),
		Titles: gen,
	})
	return svc, backend
}

func titleIdleCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 10*time.Second)
}

func TestAutoTitleNamesSessionAfterFirstExchange(t *testing.T) {
	gen := &fakeTitleGenerator{title: "  \"Fix the   login bug\"  "}
	svc, backend := newTitleService(t, gen)
	ctx, cancel := titleIdleCtx()
	defer cancel()
	if err := backend.CreateSession(context.Background(), domain.Session{ID: "sess-title", CreatedAt: 1}); err != nil {
		t.Fatalf("create session: %v", err)
	}
	if _, err := svc.Run(ctx, "sess-title", "the login bug is on line 42"); err != nil {
		t.Fatalf("run: %v", err)
	}
	if !svc.WaitIdle(ctx) {
		t.Fatal("run did not go idle")
	}
	sess, err := backend.GetSession(context.Background(), "sess-title")
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if sess.Title != "Fix the login bug" {
		t.Fatalf("title = %q, want sanitized auto title", sess.Title)
	}
	if !strings.Contains(gen.gotUser, "login bug") || !strings.Contains(gen.gotAssistant, "first answer") {
		t.Fatalf("generator got user=%q assistant=%q", gen.gotUser, gen.gotAssistant)
	}
}

func TestAutoTitleRespectsUserRename(t *testing.T) {
	gen := &fakeTitleGenerator{title: "model title"}
	svc, backend := newTitleService(t, gen)
	ctx, cancel := titleIdleCtx()
	defer cancel()
	if err := backend.CreateSession(context.Background(), domain.Session{ID: "sess-named", Title: "my project", CreatedAt: 1}); err != nil {
		t.Fatalf("create session: %v", err)
	}
	if _, err := svc.Run(ctx, "sess-named", "hello there"); err != nil {
		t.Fatalf("run: %v", err)
	}
	if !svc.WaitIdle(ctx) {
		t.Fatal("run did not go idle")
	}
	sess, err := backend.GetSession(context.Background(), "sess-named")
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if sess.Title != "my project" {
		t.Fatalf("title = %q, want the user rename to stand", sess.Title)
	}
	if gen.gotUser != "" {
		t.Fatal("generator must not run for a user-named session")
	}
}

func TestAutoTitleKeepsEmptyOnGeneratorFailure(t *testing.T) {
	gen := &fakeTitleGenerator{err: errors.New("no api key")}
	svc, backend := newTitleService(t, gen)
	ctx, cancel := titleIdleCtx()
	defer cancel()
	if err := backend.CreateSession(context.Background(), domain.Session{ID: "sess-fail", CreatedAt: 1}); err != nil {
		t.Fatalf("create session: %v", err)
	}
	if _, err := svc.Run(ctx, "sess-fail", "hello there"); err != nil {
		t.Fatalf("run: %v", err)
	}
	if !svc.WaitIdle(ctx) {
		t.Fatal("run did not go idle")
	}
	sess, err := backend.GetSession(context.Background(), "sess-fail")
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if sess.Title != "" {
		t.Fatalf("title = %q, want empty after generator failure", sess.Title)
	}
}

func TestAutoTitleFiresAfterFailedRun(t *testing.T) {
	// An empty scripted model fails the run at the first model call, the way
	// a broken provider or a tool error ends a real first exchange.
	gen := &fakeTitleGenerator{title: "Login Bug"}
	svc, backend := newTitleServiceWithModel(t, gen, NewScriptedModel())
	ctx, cancel := titleIdleCtx()
	defer cancel()
	if err := backend.CreateSession(context.Background(), domain.Session{ID: "sess-failed", CreatedAt: 1}); err != nil {
		t.Fatalf("create session: %v", err)
	}
	if _, err := svc.Run(ctx, "sess-failed", "fix the login bug"); err != nil {
		t.Fatalf("run submit: %v", err)
	}
	if !svc.WaitIdle(ctx) {
		t.Fatal("run did not go idle")
	}
	sess, err := backend.GetSession(context.Background(), "sess-failed")
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if sess.Title != "Login Bug" {
		t.Fatalf("title = %q, want the auto title after a failed run", sess.Title)
	}
	if !strings.Contains(gen.gotUser, "login bug") {
		t.Fatalf("generator got user=%q", gen.gotUser)
	}
}

func TestAutoTitleSkipsTitledSessionDirectly(t *testing.T) {
	backend, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "direct.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = backend.Close() })
	if err := backend.CreateSession(context.Background(), domain.Session{ID: "sess-user", Title: "user title", CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	gen := &fakeTitleGenerator{title: "should not happen"}
	svc := &Service{deps: ServiceDeps{Sessions: backend, Messages: backend, Titles: gen}}
	svc.autoTitle(context.Background(), "sess-user")
	if gen.gotUser != "" {
		t.Fatal("titled sessions must be skipped before the generator runs")
	}
}

func TestSanitizeSessionTitle(t *testing.T) {
	cases := []struct {
		raw, want string
	}{
		{raw: "  plain title  ", want: "plain title"},
		{raw: "\"quoted\"\n", want: "quoted"},
		{raw: "multi\nline\ttitle", want: "multi line title"},
		{raw: "“curly quotes”", want: "curly quotes"},
	}
	for _, tc := range cases {
		if got := sanitizeSessionTitle(tc.raw); got != tc.want {
			t.Errorf("sanitize(%q) = %q, want %q", tc.raw, got, tc.want)
		}
	}
	long := strings.Repeat("长", maxSessionTitleRunes+10)
	if got := sanitizeSessionTitle(long); len([]rune(got)) != maxSessionTitleRunes {
		t.Errorf("long title capped to %d runes, got %d", maxSessionTitleRunes, len([]rune(got)))
	}
}

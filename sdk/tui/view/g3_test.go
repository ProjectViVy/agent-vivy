package view

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"agent-vivy/sdk/tui/surface"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func g3Driver() *testDriver {
	return &testDriver{
		sessions: []surface.Session{{ID: "s1", Title: "s1"}},
		active:   "s1",
		messages: map[string][]surface.Message{
			"s1": {
				{ID: "m1", Role: surface.RoleUser, Content: "first prompt about apples"},
				{ID: "m2", Role: surface.RoleAssistant, Content: "answer one"},
				{ID: "m3", Role: surface.RoleUser, Content: "second prompt about oranges"},
				{ID: "m4", Role: surface.RoleAssistant, Content: "visit https://example.com/docs for details."},
				{ID: "m5", Tool: &surface.ToolCard{ToolName: "bash", Status: "done", Result: "needle-in-tool-result"}},
			},
		},
	}
}

// g3TallDriver pads assistant replies so the transcript outgrows the small
// test viewport and scroll positions become observable.
func g3TallDriver() *testDriver {
	d := g3Driver()
	d.messages["s1"][1].Content = "answer one\n" + strings.Repeat("filler line\n", 40)
	d.messages["s1"][3].Content = "answer two\n" + strings.Repeat("more filler\n", 40)
	return d
}

func TestSearchMatchesAcrossContentAndTools(t *testing.T) {
	m := New(g3Driver())
	m = m.openSearch()
	m.searchQuery = "needle"
	matches := m.computeSearchMatches()
	if len(matches) != 1 || matches[0] != 4 {
		t.Fatalf("tool result match = %v, want [4]", matches)
	}
	m.searchQuery = "PROMPT"
	matches = m.computeSearchMatches()
	if len(matches) != 2 || matches[0] != 0 || matches[1] != 2 {
		t.Fatalf("case-insensitive matches = %v, want [0 2]", matches)
	}
	if got := m.computeSearchMatches(); len(got) == 0 {
		t.Fatal("expected matches")
	}
}

func TestSearchJumpAndEscRestore(t *testing.T) {
	d := g3TallDriver()
	m := New(d)
	m.width, m.height = 100, 12
	m.chatFollow = true
	m = m.openSearch()
	m.searchQuery = "prompt"
	m = m.searchJump(1)
	if m.searchCursor != 0 || len(m.searchMatches) != 2 {
		t.Fatalf("cursor=%d matches=%v", m.searchCursor, m.searchMatches)
	}
	if m.chatFollow {
		t.Fatal("jump must leave follow mode")
	}
	m = m.searchJump(1)
	if m.searchCursor != 1 {
		t.Fatalf("cursor = %d, want 1", m.searchCursor)
	}
	m = m.searchJump(1)
	if m.searchCursor != 0 {
		t.Fatalf("wrap: cursor = %d, want 0", m.searchCursor)
	}
	m = m.searchJump(-1)
	if m.searchCursor != 1 {
		t.Fatalf("reverse wrap: cursor = %d, want 1", m.searchCursor)
	}
	m = m.closeSearch(true)
	if !m.chatFollow {
		t.Fatal("esc restore must return to follow mode")
	}
	if m.searchOpen {
		t.Fatal("search still open after close")
	}
}

func TestPromptJumpMovesBetweenUserMessages(t *testing.T) {
	m := New(g3TallDriver())
	m.width, m.height = 100, 12
	m.chatFollow = true
	m.clampChatScroll()
	// Follow puts the viewport at the bottom; prompt_prev jumps to the last
	// user message, then the previous one.
	m = m.jumpToUserMessage(-1)
	if m.chatFollow {
		t.Fatal("prompt jump must leave follow mode")
	}
	first := m.chatScroll
	m = m.jumpToUserMessage(-1)
	second := m.chatScroll
	if second >= first {
		t.Fatalf("prev jump did not move up: %d -> %d", first, second)
	}
	m = m.jumpToUserMessage(1)
	if m.chatScroll <= second {
		t.Fatalf("next jump did not move down: %d -> %d", second, m.chatScroll)
	}
	// Past the ends: stays put.
	top := m.chatScroll
	m = m.jumpToUserMessage(-1)
	m = m.jumpToUserMessage(-1)
	m = m.jumpToUserMessage(-1)
	if m.chatScroll != top && m.chatScroll > top {
		t.Fatalf("unexpected scroll: %d -> %d", top, m.chatScroll)
	}
}

func TestSearchKeyRoutingTypesQueryAndCloses(t *testing.T) {
	m := New(g3Driver())
	m = m.openSearch()
	km, _ := m.handleSearchKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("xyz")})
	m = km
	if m.searchQuery != "xyz" {
		t.Fatalf("query = %q", m.searchQuery)
	}
	km, _ = m.handleSearchKey(tea.KeyMsg{Type: tea.KeyBackspace})
	m = km
	if m.searchQuery != "xy" {
		t.Fatalf("backspace query = %q", m.searchQuery)
	}
	km, _ = m.handleSearchKey(tea.KeyMsg{Type: tea.KeyEsc})
	m = km
	if m.searchOpen {
		t.Fatal("esc did not close search")
	}
}

func TestResolveEditorPrefersEnvThenPATH(t *testing.T) {
	env := func(k string) string {
		if k == "EDITOR" {
			return "code --wait"
		}
		return ""
	}
	if got := resolveEditor(env, func(string) (string, error) { return "", os.ErrNotExist }); len(got) != 2 || got[0] != "code" || got[1] != "--wait" {
		t.Fatalf("EDITOR split = %v", got)
	}
	if got := resolveEditor(func(string) string { return "  " }, func(string) (string, error) { return "", os.ErrNotExist }); got != nil {
		t.Fatalf("no editor = %v", got)
	}
	if runtime.GOOS != "windows" {
		got := resolveEditor(func(string) string { return "" }, func(name string) (string, error) {
			if name == "vi" {
				return "/usr/bin/vi", nil
			}
			return "", os.ErrNotExist
		})
		if len(got) != 1 || got[0] != "vi" {
			t.Fatalf("fallback = %v", got)
		}
	}
}

func TestExternalEditorRoundTrip(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-script fake editor")
	}
	dir := t.TempDir()
	fake := filepath.Join(dir, "fake-editor.sh")
	script := "#!/bin/sh\nprintf 'edited content\\nsecond line' > \"$1\"\n"
	if err := os.WriteFile(fake, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := runExternalEditor([]string{fake}, "draft")
	if err != nil {
		t.Fatal(err)
	}
	if got != "edited content\nsecond line" {
		t.Fatalf("round trip = %q", got)
	}
	if _, err := runExternalEditor([]string{"/nonexistent-editor-x"}, "x"); err == nil {
		t.Fatal("missing binary must error")
	}
	if _, err := runExternalEditor(nil, "x"); err == nil {
		t.Fatal("empty editor must error")
	}
}

func TestLinkifyOSC8WrapsURLs(t *testing.T) {
	out := linkifyOSC8("see https://example.com/a?b=1. now")
	if !strings.Contains(out, "\x1b]8;;https://example.com/a?b=1\x07") {
		t.Fatalf("no OSC8 target: %q", out)
	}
	if !strings.HasSuffix(out, "\x1b]8;;\x07 now") {
		t.Fatalf("no terminator: %q", out)
	}
	if plain := linkifyOSC8("no link here"); strings.Contains(plain, "\x1b]8") {
		t.Fatalf("plain line touched: %q", plain)
	}
	// Styled line: escapes inside the URL text are stripped from the target.
	styled := "go \x1b[4mhttps://ex.com/x\x1b[0m end"
	out = linkifyOSC8(styled)
	if !strings.Contains(out, "\x1b]8;;https://ex.com/x\x07") {
		t.Fatalf("styled URL target: %q", out)
	}
	if stripped := ansi.Strip(out); !strings.Contains(stripped, "https://ex.com/x end") {
		t.Fatalf("display text changed: %q", stripped)
	}
}

func TestToolRendererRegistryDispatch(t *testing.T) {
	RegisterToolRenderer("Bash", func(tool *surface.ToolCard, width int, p Palette) []string {
		return []string{"CUSTOM:" + tool.Result}
	})
	defer RegisterToolRenderer("bash", nil)
	tool := &surface.ToolCard{ToolName: "bash", Status: "done", Result: "R"}
	m := New(g3Driver())
	got := m.renderToolWithOptions(tool, 80, m.palette, false)
	if len(got) != 1 || got[0] != "CUSTOM:R" {
		t.Fatalf("renderer output = %v", got)
	}
	// nil return falls back to the default card.
	RegisterToolRenderer("bash", func(tool *surface.ToolCard, width int, p Palette) []string { return nil })
	got = m.renderToolWithOptions(tool, 80, m.palette, false)
	if len(got) == 0 || strings.Contains(got[0], "CUSTOM:") {
		t.Fatalf("fallback output = %v", got)
	}
	RegisterToolRenderer("bash", nil)
}

func TestHeroCountsLineListsKnownResources(t *testing.T) {
	d := g3Driver()
	d.sidebar = surface.Sidebar{
		SkillsKnown: true,
		Skills:      []surface.SidebarSkill{{Name: "a"}, {Name: "b"}},
		MCPKnown:    true,
		MCP:         []surface.MCPServer{{Name: "m1"}},
		ToolsKnown:  true,
		ToolCount:   42,
	}
	m := New(d)
	line := m.heroCountsLine()
	for _, want := range []string{"2 skills", "42 tools", "1 MCP", "1 sessions"} {
		if !strings.Contains(line, want) {
			t.Fatalf("counts line %q missing %q", line, want)
		}
	}
	// Unknown facts are skipped, not zeroed.
	d.sidebar = surface.Sidebar{}
	m = New(d)
	line = m.heroCountsLine()
	if strings.Contains(line, "skills") || strings.Contains(line, "tools") || strings.Contains(line, "MCP") {
		t.Fatalf("unknown facts rendered: %q", line)
	}
	if !strings.Contains(line, "1 sessions") {
		t.Fatalf("sessions always listed: %q", line)
	}
}

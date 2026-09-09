package view

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"agent-vivy/sdk/tui/surface"
)

func TestRenderMessageMarkdownTypography(t *testing.T) {
	source := "# Title\n\n## Section\n\nThis is **bold** and `code`.\n\n- item one\n- item two\n\n> quoted\n\n```go\nfmt.Println(\"hi\")\n```\n\nSee [docs](https://example.com).\n"
	lines := (Model{}).renderMessage(surface.Message{Role: surface.RoleAssistant, Content: source}, 80, DefaultPalette())
	if len(lines) == 0 {
		t.Fatal("markdown message rendered no lines")
	}
	plain := ansi.Strip(strings.Join(lines, "\n"))
	joined := strings.Join(lines, "\n")
	if strings.Contains(plain, "**bold**") {
		t.Fatalf("strong leaked source markers: %q", plain)
	}
	if !strings.Contains(plain, "Title") || !strings.Contains(plain, "Section") || !strings.Contains(plain, "bold") {
		t.Fatalf("heading/emphasis text missing: %q", plain)
	}
	if strings.Contains(plain, "# Title") {
		t.Fatalf("h1 kept a raw hash prefix: %q", plain)
	}
	if !strings.Contains(plain, "## Section") {
		t.Fatalf("h2 lost its visible prefix: %q", plain)
	}
	if !strings.Contains(plain, "• item one") {
		t.Fatalf("list did not use a bullet: %q", plain)
	}
	if !strings.Contains(plain, "│") || strings.Contains(plain, "> quoted") {
		t.Fatalf("quote did not use a rail: %q", plain)
	}
	if !strings.Contains(plain, "fmt.Println") {
		t.Fatalf("fenced code lost body: %q", plain)
	}
	if !strings.Contains(joined, "\x1b[") {
		t.Fatalf("markdown render had no ANSI styling: %q", joined)
	}
	if !strings.Contains(plain, "docs") {
		t.Fatalf("link text missing: %q", plain)
	}
}

func TestRenderMessageMarkdownUserAndReasoning(t *testing.T) {
	user := strings.Join((Model{}).renderMessage(surface.Message{Role: surface.RoleUser, Content: "# Hello\n\n- one"}, 80, DefaultPalette()), "\n")
	if !strings.Contains(ansi.Strip(user), "Hello") || strings.Contains(ansi.Strip(user), "# Hello") {
		t.Fatalf("user markdown was not styled: %q", ansi.Strip(user))
	}
	thinking := strings.Join((Model{}).renderMessage(surface.Message{
		Role: surface.RoleAssistant, Content: "# Loud\n\n- quiet", Reasoning: true,
	}, 80, DefaultPalette()), "\n")
	plain := ansi.Strip(thinking)
	if !strings.Contains(plain, "Loud") || !strings.Contains(plain, "• quiet") {
		t.Fatalf("reasoning markdown lost structure: %q", plain)
	}
	if !strings.Contains(thinking, "┊") {
		t.Fatalf("reasoning lost its gutter: %q", thinking)
	}
}

func TestRenderMessageMarkdownFitsNarrowViewportAndDropsBidi(t *testing.T) {
	message := surface.Message{Role: surface.RoleAssistant, Content: "# 你e\u0301\n\n- 👨‍👩‍👧‍👦\u202eabc\u2066", Streaming: true}
	for width := 1; width <= 12; width++ {
		lines := (Model{}).renderMessage(message, width, DefaultPalette())
		if len(lines) == 0 {
			t.Fatalf("width %d rendered no message lines", width)
		}
		visible := false
		for _, line := range lines {
			if got := lipgloss.Width(line); got > width {
				t.Fatalf("width %d rendered line width %d: %q", width, got, line)
			}
			plain := ansi.Strip(line)
			visible = visible || strings.TrimSpace(plain) != ""
			if strings.ContainsAny(plain, "\u202e\u2066") {
				t.Fatalf("width %d retained bidi controls: %q", width, plain)
			}
		}
		if !visible {
			t.Fatalf("width %d silently erased the message", width)
		}
	}
}

func TestFinishedMarkdownMessageCacheHits(t *testing.T) {
	driver := &testDriver{
		active:   "active",
		sessions: []surface.Session{{ID: "active", Title: "one"}},
		messages: map[string][]surface.Message{
			"active": {{ID: "m1", Role: surface.RoleAssistant, Content: "# Cached\n\n- item"}},
		},
	}
	m := New(driver)
	first := m.chatLines(80, DefaultPalette())
	second := m.chatLines(80, DefaultPalette())
	if strings.Join(first, "\n") != strings.Join(second, "\n") {
		t.Fatalf("cached markdown render changed:\n%s\n---\n%s", first, second)
	}
	if len(m.mdCache.lines) == 0 {
		t.Fatal("finished markdown was not stored in the message cache")
	}
	plain := ansi.Strip(strings.Join(first, "\n"))
	if !strings.Contains(plain, "Cached") || !strings.Contains(plain, "• item") {
		t.Fatalf("cached markdown lost typography: %q", plain)
	}
}

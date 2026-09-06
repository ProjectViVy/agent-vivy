package view

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"agent-vivy/sdk/tui/surface"
)

func TestMiddleTruncateKeepsShortStringsUnchanged(t *testing.T) {
	for _, tc := range []struct {
		in    string
		width int
	}{
		{"main.go", 20},
		{"main.go", 7},
		{"目录/引擎.go", 12}, // exactly 12 cells with wide runes
		{"", 5},
	} {
		if got := middleTruncate(tc.in, tc.width); got != tc.in {
			t.Fatalf("middleTruncate(%q, %d) = %q, want unchanged", tc.in, tc.width, got)
		}
	}
	if got := middleTruncate("main.go", 0); got != "" {
		t.Fatalf("middleTruncate with width 0 = %q, want empty", got)
	}
}

func TestMiddleTruncatePrefersTailAndRespectsWidth(t *testing.T) {
	const path = "internal/runtime/deeply/nested/directory/engine.go"
	if got := lipgloss.Width(path); got != 50 {
		t.Fatalf("fixture path width = %d, want 50 (runewidth table drift)", got)
	}
	for _, width := range []int{14, 16, 20, 30} {
		got := middleTruncate(path, width)
		if got == path {
			t.Fatalf("width %d did not truncate: %q", width, got)
		}
		if w := lipgloss.Width(got); w > width {
			t.Fatalf("width %d produced %d cells: %q", width, w, got)
		}
		if !strings.Contains(got, "…") {
			t.Fatalf("width %d lost the ellipsis: %q", width, got)
		}
		if !strings.HasSuffix(got, "engine.go") {
			t.Fatalf("width %d dropped the filename tail: %q", width, got)
		}
	}
}

func TestMiddleTruncateSmallWidthsDoNotPanic(t *testing.T) {
	long := strings.Repeat("a/b/", 40) + "file.go"
	if got := middleTruncate("anything", -2); got != "" {
		t.Fatalf("middleTruncate with negative width = %q, want empty", got)
	}
	for width := 1; width <= 4; width++ {
		got := middleTruncate(long, width)
		if w := lipgloss.Width(got); w > width {
			t.Fatalf("width %d produced %d cells: %q", width, w, got)
		}
	}
}

func TestMiddleTruncateCJKCutsOnGraphemeBoundaries(t *testing.T) {
	path := "目录/很长的文件名/引擎模块.go"
	if got := lipgloss.Width(path); got != 29 {
		t.Fatalf("fixture path width = %d, want 29 (runewidth table drift)", got)
	}
	if got := middleTruncate(path, 29); got != path {
		t.Fatalf("exact-fit wide path was altered: %q", got)
	}
	for width := 1; width <= 32; width++ {
		got := middleTruncate(path, width)
		if w := lipgloss.Width(got); w > width {
			t.Fatalf("width %d produced %d cells: %q", width, w, got)
		}
	}
	for _, width := range []int{10, 12, 16, 24} {
		got := middleTruncate(path, width)
		if !strings.HasSuffix(got, ".go") {
			t.Fatalf("width %d dropped the file extension: %q", width, got)
		}
	}
}

func TestSidebarModifiedFilesStyleCountsAndKeepFilenameTail(t *testing.T) {
	const longPath = "pkg/deeply/nested/directory/structure/that/is/very/long/engine.go"
	const stamp = int64(1725552000000)
	when := sidebarTime(stamp)
	driver := &testDriver{
		sessions: []surface.Session{{ID: "active", Title: "Current"}},
		active:   "active",
		sidebar: surface.Sidebar{
			Session:            surface.Session{ID: "active", Title: "Current"},
			ModifiedFilesKnown: true,
			ModifiedFiles: []surface.ModifiedFile{
				{Path: longPath, Diff: surface.SidebarDiff{Additions: 12, Deletions: 3}},
				{Path: "notes.md", Diff: surface.SidebarDiff{Additions: 1, Deletions: 0}, UpdatedAt: stamp},
				{Path: "b.go", Diff: surface.SidebarDiff{Additions: 2, Deletions: 1}, UpdatedAt: stamp},
			},
		},
	}
	m := New(driver)
	next, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m = next.(Model)
	p := DefaultPalette()

	var sawLong, sawNotes, sawShort bool
	for _, line := range m.sidebarLines(32, p) {
		plain := ansi.Strip(line)
		if w := lipgloss.Width(line); w > 31 {
			t.Fatalf("modified-file line width %d exceeds 31: %q", w, line)
		}
		switch {
		case strings.Contains(plain, "+12"):
			sawLong = true
			if !strings.Contains(plain, "…") || !strings.HasSuffix(plain, "engine.go  +12 -3") {
				t.Fatalf("long path lost its middle-truncated filename tail: %q", plain)
			}
			if !strings.Contains(line, p.DiffAdd.Render("+12")) || !strings.Contains(line, p.DiffDel.Render("-3")) {
				t.Fatalf("counts lost DiffAdd/DiffDel styling: %q", line)
			}
		case strings.Contains(plain, "notes.md"):
			sawNotes = true
			if strings.Contains(plain, when) {
				t.Fatalf("timestamp was kept although it starved the path floor: %q", plain)
			}
			if !strings.Contains(line, p.DiffAdd.Render("+1")) || !strings.Contains(line, p.DiffDel.Render("-0")) {
				t.Fatalf("notes counts lost DiffAdd/DiffDel styling: %q", line)
			}
		case strings.Contains(plain, "b.go  +2 -1"):
			sawShort = true
			if !strings.Contains(line, p.Dim.Render(" · "+when)) {
				t.Fatalf("short path dropped a fitting timestamp: %q", line)
			}
		}
	}
	if !sawLong || !sawNotes || !sawShort {
		t.Fatalf("modified-file lines missing: long=%v notes=%v short=%v", sawLong, sawNotes, sawShort)
	}

	// A roomier sidebar keeps the timestamp and shows head and tail of the
	// long path while still dropping its middle.
	roomyPlain := ansi.Strip(strings.Join(m.sidebarLines(60, p), "\n"))
	if !strings.Contains(roomyPlain, "notes.md  +1 -0 · "+when) {
		t.Fatalf("roomy sidebar dropped a fitting timestamp:\n%s", roomyPlain)
	}
	if !strings.Contains(roomyPlain, "pkg/deeply") || !strings.Contains(roomyPlain, "engine.go") || strings.Contains(roomyPlain, longPath) {
		t.Fatalf("roomy sidebar did not middle-truncate the long path:\n%s", roomyPlain)
	}

	// Degenerate tiny widths fall back to a whole-line end-truncation that
	// still fits.
	for _, line := range m.sidebarLines(8, p) {
		plain := ansi.Strip(line)
		if !strings.Contains(plain, "+") {
			continue
		}
		if w := lipgloss.Width(line); w > 7 {
			t.Fatalf("degenerate modified-file line width %d exceeds 7: %q", w, line)
		}
	}

	plainView := ansi.Strip(m.View())
	for _, want := range []string{"Modified Files", "engine.go", "+12", "-3", "notes.md", "b.go"} {
		if !strings.Contains(plainView, want) {
			t.Fatalf("sidebar view omitted %q:\n%s", want, plainView)
		}
	}
}

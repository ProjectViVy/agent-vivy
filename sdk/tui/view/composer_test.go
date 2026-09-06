package view

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"

	"agent-vivy/sdk/tui/surface"
)

func TestComposerUsesRoundedBoxAndChips(t *testing.T) {
	driver := &testDriver{
		sessions: []surface.Session{{ID: "s1", Title: "Current", PermissionPreset: "smart"}},
		active:   "s1",
		thinking: "auto",
		sidebar:  surface.Sidebar{Model: "gpt-4.1"},
	}
	m := New(driver)
	m.input = "hello"
	editor := ansi.Strip(m.renderEditor(40, DefaultPalette()))
	if !strings.Contains(editor, "╭") || !strings.Contains(editor, "╰") || !strings.Contains(editor, "╮") || !strings.Contains(editor, "╯") {
		t.Fatalf("missing rounded corners:\n%s", editor)
	}
	if !strings.Contains(editor, "gpt-4.1") || !strings.Contains(editor, "(auto)") || !strings.Contains(editor, "智能") {
		t.Fatalf("composer chips missing:\n%s", editor)
	}
	if !strings.Contains(editor, ":::") || !strings.Contains(editor, "hello") {
		t.Fatalf("crush prompt or draft missing:\n%s", editor)
	}
}

func TestComposerFallsBackWhenModelMissing(t *testing.T) {
	driver := &testDriver{
		sessions: []surface.Session{{ID: "s1", PermissionPreset: "cautious"}},
		active:   "s1",
		thinking: "off",
	}
	m := New(driver)
	editor := ansi.Strip(m.renderEditor(36, DefaultPalette()))
	if !strings.Contains(editor, "╭") || !strings.Contains(editor, "model") || !strings.Contains(editor, "只读") {
		t.Fatalf("missing fallback chips:\n%s", editor)
	}
}

func TestComposerFitsNarrowWidths(t *testing.T) {
	driver := &testDriver{
		sessions: []surface.Session{{ID: "s1", PermissionPreset: "trusted"}},
		active:   "s1",
		thinking: "on",
		sidebar:  surface.Sidebar{Model: "very-long-provider-model-id"},
	}
	m := New(driver)
	m.input = "draft"
	for _, width := range []int{8, 12, 20, 40, 80} {
		editor := m.renderEditor(width, DefaultPalette())
		for i, line := range strings.Split(editor, "\n") {
			if got := lipgloss.Width(line); got > width {
				t.Fatalf("width %d line %d overflowed %d: %q", width, i, got, ansi.Strip(line))
			}
		}
	}
}

func TestComposerKeepsGateMarkerAndAttachmentChips(t *testing.T) {
	driver := &testDriver{
		sessions:    []surface.Session{{ID: "s1", PermissionPreset: "smart"}},
		active:      "s1",
		gate:        &surface.Gate{Kind: "approval", ID: "g1", Title: "write"},
		attachments: []surface.Attachment{{Name: "shot.png", Path: "shot.png"}},
		sidebar:     surface.Sidebar{Model: "gpt-4.1"},
	}
	m := New(driver)
	editor := ansi.Strip(m.renderEditor(48, DefaultPalette()))
	if !strings.Contains(editor, "!") || !strings.Contains(editor, ":::") {
		t.Fatalf("gate marker or prompt missing:\n%s", editor)
	}
	if !strings.Contains(editor, "[image: shot.png]") {
		t.Fatalf("attachment chip missing:\n%s", editor)
	}
	if strings.Contains(editor, "data:") || strings.Contains(editor, "base64") {
		t.Fatalf("attachment payload leaked into composer:\n%s", editor)
	}
}

func TestComposerHelpRowStaysVisible(t *testing.T) {
	driver := &testDriver{
		sessions: []surface.Session{{ID: "s1", Title: "Current", PermissionPreset: "smart"}},
		active:   "s1",
		sidebar:  surface.Sidebar{Model: "gpt-4.1"},
	}
	m := New(driver)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = updated.(Model)
	view := ansi.Strip(m.View())
	if !strings.Contains(view, "shift+tab") || !strings.Contains(view, "切换模式") || !strings.Contains(view, "帮助") {
		t.Fatalf("help row clipped after taller composer:\n%s", view)
	}
	if !strings.Contains(view, "╭") || !strings.Contains(view, "gpt-4.1") {
		t.Fatalf("composer missing from frame:\n%s", view)
	}
}

func TestComposerBoxColorFollowsWorkingMode(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	driver := &testDriver{
		sessions: []surface.Session{{ID: "s1", PermissionPreset: "smart"}},
		active:   "s1",
		sidebar:  surface.Sidebar{Session: surface.Session{ID: "s1", PermissionPreset: "smart"}, Model: "gpt-4.1"},
	}
	m := New(driver)
	smart := m.renderEditor(48, DefaultPalette())
	m = paletteKey(t, m, tea.KeyMsg{Type: tea.KeyShiftTab})
	plan := m.renderEditor(48, DefaultPalette())
	m = paletteKey(t, m, tea.KeyMsg{Type: tea.KeyShiftTab})
	read := m.renderEditor(48, DefaultPalette())
	if smart == plan || plan == read || smart == read {
		t.Fatalf("composer border did not change with mode")
	}
	if !strings.Contains(ansi.Strip(smart), "智能") || !strings.Contains(ansi.Strip(plan), "计划") || !strings.Contains(ansi.Strip(read), "只读") {
		t.Fatalf("mode chips missing smart=%q plan=%q read=%q", ansi.Strip(smart), ansi.Strip(plan), ansi.Strip(read))
	}
}

func TestEditorInputLinesSplitsTruncatesAndWindows(t *testing.T) {
	if got := editorInputLines("hello", 20, 6); len(got) != 1 || got[0] != "hello" {
		t.Fatalf("single line = %q", got)
	}
	if got := editorInputLines("", 20, 6); len(got) != 1 || got[0] != "" {
		t.Fatalf("empty input = %q, want one empty line", got)
	}
	if got := editorInputLines("a\r\nbb\rccc", 20, 6); len(got) != 3 || got[0] != "a" || got[1] != "bb" || got[2] != "ccc" {
		t.Fatalf("CRLF normalization = %q", got)
	}
	if got := editorInputLines("a\nbb\nccc", 20, 6); len(got) != 3 || got[0] != "a" || got[1] != "bb" || got[2] != "ccc" {
		t.Fatalf("multi line = %q", got)
	}
	got := editorInputLines("l1\nl2\nl3\nl4\nl5\nl6\nl7\nl8", 20, 6)
	if len(got) != 6 || got[0] != "…l3" || got[1] != "l4" || got[5] != "l8" {
		t.Fatalf("window kept %q, want trailing six with upper-cut marker", got)
	}
	for _, tc := range []struct {
		name  string
		input string
		width int
	}{
		{"ascii overflow", strings.Repeat("x", 50), 10},
		{"cjk wide chars", strings.Repeat("你好世界", 5), 7},
		{"ansi payload", "\x1b[31m" + strings.Repeat("r", 30) + "\x1b[0m", 5},
	} {
		for _, line := range editorInputLines(tc.input, tc.width, 6) {
			if w := lipgloss.Width(line); w > tc.width {
				t.Fatalf("%s: line width %d exceeds %d: %q", tc.name, w, tc.width, line)
			}
		}
	}
	if !strings.Contains(editorInputLines("\x1b[31mred-red-red\x1b[0m", 20, 6)[0], "red") {
		t.Fatal("ansi payload lost its text")
	}
	if got := editorInputLines("a\nb", 10, 0); len(got) != 1 || got[0] != "…b" {
		t.Fatalf("maxLines < 1 guard = %q, want the trailing line with upper-cut marker", got)
	}
}

func TestEditorReserveGrowsWithInputLinesAndCaps(t *testing.T) {
	for _, tc := range []struct {
		attachments bool
		pasteGuard  bool
		inputLines  int
		want        int
	}{
		{false, false, 1, 4},
		{false, false, 0, 4},
		{false, false, -3, 4},
		{false, false, 3, 6},
		{true, false, 1, 5},
		{false, false, 6, 9},
		{false, false, 99, 9},
		{true, false, 99, 10},
		{false, true, 1, 5},
		{true, true, 99, 11},
	} {
		if got := editorReserve(tc.attachments, tc.pasteGuard, tc.inputLines); got != tc.want {
			t.Fatalf("editorReserve(%v, %v, %d) = %d, want %d", tc.attachments, tc.pasteGuard, tc.inputLines, got, tc.want)
		}
	}
}

func TestLayoutShrinksMainHeightForTallerDrafts(t *testing.T) {
	driver := &testDriver{sessions: []surface.Session{{ID: "s1", Title: "Current"}}, active: "s1"}
	m := New(driver)
	next, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = next.(Model)
	if l := m.layout(); l.editorH != 4 || l.mainH() != 15 {
		t.Fatalf("single-line layout editorH=%d mainH=%d, want 4/15", l.editorH, l.mainH())
	}
	m.input = "a\nb\nc"
	if l := m.layout(); l.editorH != 6 || l.mainH() != 13 {
		t.Fatalf("three-line layout editorH=%d mainH=%d, want 6/13", l.editorH, l.mainH())
	}
	// Nine lines: past maxEditorLines but below the paste-guard thresholds,
	// so only the line cap drives the reserve.
	m.input = strings.Repeat("l\n", 8)
	if l := m.layout(); l.editorH != 9 || l.mainH() != 10 {
		t.Fatalf("capped layout editorH=%d mainH=%d, want 9/10", l.editorH, l.mainH())
	}
	next, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 6})
	m = next.(Model)
	if l := m.layout(); l.mainH() != 1 {
		t.Fatalf("mainH floor = %d, want 1", l.mainH())
	}
	// A pending gate owns the composer as a single hint row, so a leftover
	// multi-line draft must not grow the reserve.
	m = New(&testDriver{
		sessions: []surface.Session{{ID: "s1", Title: "Current"}},
		active:   "s1",
		gate:     &surface.Gate{Kind: "question", ID: "q1", Title: "answer"},
	})
	next, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = next.(Model)
	m.input = "a\nb\nc"
	if l := m.layout(); l.editorH != 4 {
		t.Fatalf("gate layout editorH=%d, want 4", l.editorH)
	}
}

func TestRenderEditorShowsWholeDraftAndTrailingCaret(t *testing.T) {
	driver := &testDriver{
		sessions: []surface.Session{{ID: "s1", PermissionPreset: "smart"}},
		active:   "s1",
		sidebar:  surface.Sidebar{Model: "gpt-4.1"},
	}
	m := New(driver)
	m.input = "first line\nsecond\nthird"
	editor := m.renderEditor(48, DefaultPalette())
	plain := ansi.Strip(editor)
	for _, want := range []string{"first line", "second", "third"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("draft line %q missing:\n%s", want, plain)
		}
	}
	if strings.Count(plain, "█") != 1 {
		t.Fatalf("caret count = %d, want exactly one:\n%s", strings.Count(plain, "█"), plain)
	}
	for _, line := range strings.Split(plain, "\n") {
		if strings.Contains(line, "third") && !strings.Contains(line, "third█") {
			t.Fatalf("caret is not at the tail of the last draft row: %q", line)
		}
	}
	for i, line := range strings.Split(editor, "\n") {
		if w := lipgloss.Width(line); w > 48 {
			t.Fatalf("line %d overflowed 48: %q", i, line)
		}
	}
	m.input = "l1\nl2\nl3\nl4\nl5\nl6\nl7\nl8"
	plain = ansi.Strip(m.renderEditor(48, DefaultPalette()))
	if strings.Contains(plain, "l1") || strings.Contains(plain, "l2") {
		t.Fatalf("window did not drop the oldest lines:\n%s", plain)
	}
	if !strings.Contains(plain, "…l3") {
		t.Fatalf("upper-cut marker missing:\n%s", plain)
	}
	for _, want := range []string{"l4", "l5", "l6", "l7", "l8"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("window line %q missing:\n%s", want, plain)
		}
	}
	for _, line := range strings.Split(plain, "\n") {
		if strings.Contains(line, "l8") && !strings.Contains(line, "l8█") {
			t.Fatalf("caret lost on the windowed last row: %q", line)
		}
	}
}

func TestComposerPlaceholderShowsOnlyForEmptyUngatedInput(t *testing.T) {
	p := DefaultPalette()
	dim := p.Dim.Render(composerPlaceholder)
	driver := &testDriver{
		sessions: []surface.Session{{ID: "s1", PermissionPreset: "smart"}},
		active:   "s1",
		sidebar:  surface.Sidebar{Model: "gpt-4.1"},
	}
	m := New(driver)
	empty := m.renderEditor(60, p)
	if !strings.Contains(ansi.Strip(empty), composerPlaceholder) {
		t.Fatalf("empty composer missed the placeholder:\n%s", ansi.Strip(empty))
	}
	if !strings.Contains(empty, dim) {
		t.Fatal("placeholder was not rendered with the dim style")
	}
	m.input = "draft text"
	if strings.Contains(ansi.Strip(m.renderEditor(60, p)), composerPlaceholder) {
		t.Fatal("placeholder leaked into a non-empty draft")
	}
	gated := New(&testDriver{
		sessions: []surface.Session{{ID: "s1", PermissionPreset: "smart"}},
		active:   "s1",
		gate:     &surface.Gate{Kind: "question", ID: "q1", Title: "answer"},
	})
	if strings.Contains(ansi.Strip(gated.renderEditor(60, p)), composerPlaceholder) {
		t.Fatal("placeholder shown while a gate is pending")
	}
	m.input = ""
	m.sidebarFocused = true
	if strings.Contains(ansi.Strip(m.renderEditor(60, p)), composerPlaceholder) {
		t.Fatal("placeholder shown while the sidebar is focused")
	}
	m.sidebarFocused = false
	for _, width := range []int{12, 18, 30, 60} {
		editor := m.renderEditor(width, p)
		for i, line := range strings.Split(editor, "\n") {
			if w := lipgloss.Width(line); w > width {
				t.Fatalf("width %d line %d overflowed: %q", width, i, ansi.Strip(line))
			}
		}
	}
}

func TestPasteGuardChipThresholdBoundaries(t *testing.T) {
	if chip := pasteGuardChip(strings.Repeat("x", pasteThresholdChars)); chip != "" {
		t.Fatalf("chip at exactly %d chars = %q", pasteThresholdChars, chip)
	}
	if chip := pasteGuardChip(strings.Repeat("x", pasteThresholdChars+1)); !strings.Contains(chip, "2001 字符") || !strings.Contains(chip, "1 行") {
		t.Fatalf("chip above char threshold = %q", chip)
	}
	linesAtThreshold := strings.Repeat("l\n", pasteThresholdLines-1) + "l"
	if chip := pasteGuardChip(linesAtThreshold); chip != "" {
		t.Fatalf("chip at exactly %d lines = %q", pasteThresholdLines, chip)
	}
	over := strings.Repeat("l\n", pasteThresholdLines)
	if chip := pasteGuardChip(over); !strings.Contains(chip, fmt.Sprintf("%d 行", pasteThresholdLines+1)) {
		t.Fatalf("chip above line threshold = %q", chip)
	}
	if chip := pasteGuardChip("normal draft"); chip != "" {
		t.Fatalf("small draft raised a chip: %q", chip)
	}
}

func TestPasteGuardChipRendersBetweenAttachmentsAndInput(t *testing.T) {
	p := DefaultPalette()
	driver := &testDriver{
		sessions:    []surface.Session{{ID: "s1", PermissionPreset: "smart"}},
		active:      "s1",
		attachments: []surface.Attachment{{Name: "shot.png", Path: "shot.png"}},
		sidebar:     surface.Sidebar{Model: "gpt-4.1"},
	}
	m := New(driver)
	m.input = strings.Repeat("x", pasteThresholdChars+10)
	editor := m.renderEditor(60, p)
	plain := ansi.Strip(editor)
	chip := pasteGuardChip(m.input)
	if !strings.Contains(plain, chip) {
		t.Fatalf("paste guard chip missing:\n%s", plain)
	}
	attachmentAt := strings.Index(plain, "[image: shot.png]")
	chipAt := strings.Index(plain, "大段粘贴")
	inputAt := strings.Index(plain, ":::")
	if attachmentAt < 0 || chipAt < 0 || inputAt < 0 || !(attachmentAt < chipAt && chipAt < inputAt) {
		t.Fatalf("chip order wrong: attachment=%d chip=%d input=%d", attachmentAt, chipAt, inputAt)
	}
	// The reserve grows with the chip and shrinks back once the draft is
	// trimmed under the thresholds: border 2 + chips 1 + attachments 1 +
	// single input line 1 + paste-guard 1.
	if l := m.layout(); l.editorH != 6 {
		t.Fatalf("guarded layout editorH = %d, want 6", l.editorH)
	}
	m.input = "small again"
	if strings.Contains(ansi.Strip(m.renderEditor(60, p)), "大段粘贴") {
		t.Fatal("chip survived trimming below the thresholds")
	}
	if l := m.layout(); l.editorH != 5 {
		t.Fatalf("unguarded layout editorH = %d, want 5 (attachments keep their row)", l.editorH)
	}
}

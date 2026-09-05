package view

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

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
	if !strings.Contains(editor, "gpt-4.1") || !strings.Contains(editor, "smart") || !strings.Contains(editor, "auto") {
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
	if !strings.Contains(editor, "╭") || !strings.Contains(editor, "model") || !strings.Contains(editor, "cautious") || !strings.Contains(editor, "off") {
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
	if !strings.Contains(view, "enter") || !strings.Contains(view, "send") {
		t.Fatalf("help row clipped after taller composer:\n%s", view)
	}
	if !strings.Contains(view, "╭") || !strings.Contains(view, "gpt-4.1") {
		t.Fatalf("composer missing from frame:\n%s", view)
	}
}

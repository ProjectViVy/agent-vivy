package view

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"agent-vivy/internal/tui/demo"
)

func TestVivyRolePaletteEmitsTrueColor(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	got := defaultPalette().LogoWord.Render("VIVY")
	if !strings.Contains(got, "\x1b[") || !strings.Contains(got, "38;2;167;139;250") {
		t.Fatalf("logo color missing from %q", got)
	}
}

func TestPermissionCycle(t *testing.T) {
	if got := nextPermission("cautious"); got != "smart" {
		t.Fatalf("cautious -> %q", got)
	}
	if got := nextPermission("smart"); got != "trusted" {
		t.Fatalf("smart -> %q", got)
	}
	if got := nextPermission("trusted"); got != "cautious" {
		t.Fatalf("trusted -> %q", got)
	}
}

func TestViewContainsCrushSkeleton(t *testing.T) {
	m := New(demo.NewStore())
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 36})
	m = updated.(Model)
	got := m.View()
	for _, want := range []string{
		"VIVY CODE", // sidebar logo
		"Sessions",  // sidebar section
		"审批中",
		"write_file",
		":::", // Crush editor prompt
		"^s",  // sessions dialog shortcut
		"demo",
		"permission", // overlay title
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in view:\n%s", want, got)
		}
	}
	if strings.Contains(got, "过夜") {
		t.Fatalf("wide sidebar must not render the session collection:\n%s", got)
	}
	// Wide layout: no top "vivy tui · demo" strip; logo is in the sidebar.
	if strings.Contains(got, "vivy tui · demo") {
		t.Fatalf("wide mode should not use the old top banner:\n%s", got)
	}
}

func TestApprovalKeyClearsGate(t *testing.T) {
	store := demo.NewStore()
	m := New(store)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 36})
	m = updated.(Model)
	if store.PendingGate() == nil {
		t.Fatal("expected pending gate")
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	m = updated.(Model)
	if store.PendingGate() != nil {
		t.Fatal("gate still open after y")
	}
	if !strings.Contains(m.View(), "demo approved") && !strings.Contains(m.View(), "已按你的决定") {
		t.Fatalf("expected approval follow-up in view:\n%s", m.View())
	}
}

func TestCompactHeaderHasDiagonals(t *testing.T) {
	m := New(demo.NewStore())
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = updated.(Model)
	got := m.View()
	if !strings.Contains(got, "VIVY") {
		t.Fatalf("compact logo missing:\n%s", got)
	}
	if !strings.Contains(got, "╱") {
		t.Fatalf("compact header diagonals missing:\n%s", got)
	}
	if !strings.Contains(got, "审批中") {
		t.Fatalf("active session title missing:\n%s", got)
	}
	// Compact: no Sessions sidebar label.
	if strings.Contains(got, "Sessions") {
		t.Fatalf("compact should hide sidebar:\n%s", got)
	}
}

func TestEnterAppendsDemoReply(t *testing.T) {
	store := demo.NewStore()
	store.SelectSession("sess_empty")
	m := New(store)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 36})
	m = updated.(Model)
	for _, r := range []rune("hi") {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(Model)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	got := m.View()
	if !strings.Contains(got, "hi") || !strings.Contains(got, "未接控制面") {
		t.Fatalf("demo reply missing:\n%s", got)
	}
}

func TestEditorUsesCrushPrompt(t *testing.T) {
	store := demo.NewStore()
	store.SelectSession("sess_empty")
	m := New(store)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 36})
	m = updated.(Model)
	got := m.View()
	if !strings.Contains(got, ":::") {
		t.Fatalf("missing crush ::: prompt:\n%s", got)
	}
	if strings.Contains(got, "you>") {
		t.Fatalf("old you> prompt should be gone:\n%s", got)
	}
}

package view

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"agent-vivy/internal/tui/demo"
)

func TestViewContainsCrushSkeleton(t *testing.T) {
	m := New(demo.NewStore())
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 36})
	m = updated.(Model)
	got := m.View()
	for _, want := range []string{
		"Vivy",     // sidebar logo
		"Sessions", // sidebar section
		"审批中",
		"过夜",
		"write_file",
		":::", // Crush editor prompt
		"tab", // help keys
		"mock",
		"permission", // overlay title
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in view:\n%s", want, got)
		}
	}
	// Wide layout: no top "vivy tui · demo" strip; logo is in the sidebar.
	if strings.Contains(got, "vivy tui · demo") {
		t.Fatalf("wide mode should not use the old top banner:\n%s", got)
	}
}

func TestApprovalKeyClearsGate(t *testing.T) {
	m := New(demo.NewStore())
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 36})
	m = updated.(Model)
	if m.store.PendingGate() == nil {
		t.Fatal("expected pending gate")
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	m = updated.(Model)
	if m.store.PendingGate() != nil {
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

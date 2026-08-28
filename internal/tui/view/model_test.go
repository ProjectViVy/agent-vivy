package view

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"agent-vivy/internal/tui/demo"
)

func TestViewContainsSkeletonRegions(t *testing.T) {
	m := New(demo.NewStore())
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 36})
	m = updated.(Model)
	got := m.View()
	for _, want := range []string{
		"vivy tui · demo",
		"审批中",
		"过夜",
		"write_file",
		"mock · not connected",
		"approve? [y/n]",
		"y 批准",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in view:\n%s", want, got)
		}
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

func TestNarrowHidesSidebarLabelStillShowsHeader(t *testing.T) {
	m := New(demo.NewStore())
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = updated.(Model)
	got := m.View()
	if !strings.Contains(got, "vivy tui · demo") {
		t.Fatalf("header missing:\n%s", got)
	}
	// Sidebar title is omitted when width < breakpoint; session name stays in header.
	if !strings.Contains(got, "审批中") {
		t.Fatalf("active session title missing from header path:\n%s", got)
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

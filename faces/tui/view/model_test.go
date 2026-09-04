package view

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestSharedCommandPaletteIsVisibleThroughPackedWrapper(t *testing.T) {
	m := New(nil)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
	m = updated.(Model)
	if got := m.View(); !strings.Contains(got, "Commands") || !strings.Contains(got, "/help") {
		t.Fatalf("packed wrapper palette missing:\n%s", got)
	}
}

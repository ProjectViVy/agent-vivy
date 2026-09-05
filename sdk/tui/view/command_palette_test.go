package view

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"agent-vivy/sdk/tui/surface"
)

func paletteKey(t *testing.T, m Model, key tea.KeyMsg) Model {
	t.Helper()
	updated, _ := m.Update(key)
	return updated.(Model)
}

func TestCommandPaletteOpensFromSlashAndCtrlPWithoutSending(t *testing.T) {
	d := &testDriver{}
	m := New(d)
	m = paletteKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	if !m.commandPaletteOpen || d.sent != "" || !strings.Contains(m.View(), "/help") {
		t.Fatalf("slash palette open=%v sent=%q\n%s", m.commandPaletteOpen, d.sent, m.View())
	}
	m = paletteKey(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	m.input = "keep this draft"
	m = paletteKey(t, m, tea.KeyMsg{Type: tea.KeyCtrlP})
	m = paletteKey(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.commandPaletteOpen || m.input != "keep this draft" {
		t.Fatalf("Ctrl+P escape lost draft: open=%v input=%q", m.commandPaletteOpen, m.input)
	}
}

func TestCommandPaletteFiltersAliasesAndSelectsCanonicalDraft(t *testing.T) {
	d := &commandDriver{testDriver: &testDriver{}}
	m := New(d)
	m = paletteKey(t, m, tea.KeyMsg{Type: tea.KeyCtrlP})
	m = paletteKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("attach")})
	view := m.View()
	rows := m.filteredCommands()
	if len(rows) == 0 || rows[0].Name != "image" || !strings.Contains(view, "/image <relative-path>") || strings.Contains(view, "/status") {
		t.Fatalf("alias filter did not prioritize image command: rows=%+v\n%s", rows, view)
	}
	m = paletteKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.commandPaletteOpen || m.input != "/image" || d.commandName != "" {
		t.Fatalf("selection open=%v input=%q executed=%q", m.commandPaletteOpen, m.input, d.commandName)
	}
}

func TestCommandPaletteNavigationWrapsAndSelectedCommandUsesNormalDispatch(t *testing.T) {
	d := &commandDriver{testDriver: &testDriver{}}
	m := New(d)
	m = paletteKey(t, m, tea.KeyMsg{Type: tea.KeyCtrlP})
	m = paletteKey(t, m, tea.KeyMsg{Type: tea.KeyUp})
	rows := m.filteredCommands()
	if m.commandPaletteCursor != len(rows)-1 || rows[m.commandPaletteCursor].Name != "quit" {
		t.Fatalf("up wrap cursor=%d rows=%d", m.commandPaletteCursor, len(rows))
	}
	m = paletteKey(t, m, tea.KeyMsg{Type: tea.KeyDown})
	m = paletteKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.input != "/help" {
		t.Fatalf("selected draft = %q", m.input)
	}
	m = paletteKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.commandOverlayTitle != "Commands" || d.commandName != "" {
		t.Fatalf("normal dispatch overlay=%q driver=%q", m.commandOverlayTitle, d.commandName)
	}
}

func TestCommandPalettePreservesDoubleSlashLiteral(t *testing.T) {
	d := &testDriver{}
	m := New(d)
	m = paletteKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m = paletteKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m = paletteKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("hello")})
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if cmd == nil || d.sent != "/hello" || m.input != "" {
		t.Fatalf("literal slash sent=%q input=%q cmd=%v", d.sent, m.input, cmd != nil)
	}
}

func TestCommandPaletteEmptyStateAndBackspaceResetCursor(t *testing.T) {
	m := New(&testDriver{})
	m = paletteKey(t, m, tea.KeyMsg{Type: tea.KeyCtrlP})
	m = paletteKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("zzzz-no-command")})
	if !strings.Contains(m.View(), "no matching commands") {
		t.Fatalf("missing empty state:\n%s", m.View())
	}
	m.commandPaletteCursor = 8
	m = paletteKey(t, m, tea.KeyMsg{Type: tea.KeyBackspace})
	if m.commandPaletteCursor != 0 {
		t.Fatalf("backspace cursor=%d", m.commandPaletteCursor)
	}
}

func TestCommandPaletteSanitizesPasteCapsLengthAndAcceptsPhysicalSpace(t *testing.T) {
	m := New(&testDriver{})
	m.openCommandPalette()
	paste := "red\x1b[31m\nline\r" + strings.Repeat("x", 200)
	m = paletteKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(paste)})
	if strings.ContainsAny(m.commandPaletteFilter, "\x1b\n\r") || len([]rune(m.commandPaletteFilter)) != maxCommandPaletteFilterRunes {
		t.Fatalf("unsafe/cap filter = %q (%d runes)", m.commandPaletteFilter, len([]rune(m.commandPaletteFilter)))
	}
	m.commandPaletteFilter = "run"
	m = paletteKey(t, m, tea.KeyMsg{Type: tea.KeySpace})
	m = paletteKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("status")})
	if m.commandPaletteFilter != "run status" {
		t.Fatalf("physical space filter = %q", m.commandPaletteFilter)
	}
}

func TestCommandPaletteNeverPreemptsGateSessionOrCommandOverlay(t *testing.T) {
	d := &testDriver{gate: &surface.Gate{Kind: "approval", ID: "gate_1"}}
	m := New(d)
	m = paletteKey(t, m, tea.KeyMsg{Type: tea.KeyCtrlP})
	if m.commandPaletteOpen {
		t.Fatal("palette opened over gate")
	}

	d.gate = nil
	m.sessionsOpen = true
	m.sessionRows = []surface.Session{{ID: "a"}, {ID: "b"}}
	m = paletteKey(t, m, tea.KeyMsg{Type: tea.KeyCtrlP})
	if m.commandPaletteOpen || m.sessionCursor != 1 {
		t.Fatalf("session Ctrl+P routing palette=%v cursor=%d", m.commandPaletteOpen, m.sessionCursor)
	}

	m.sessionsOpen = false
	m.commandOverlay = "done"
	m = paletteKey(t, m, tea.KeyMsg{Type: tea.KeyCtrlP})
	if m.commandPaletteOpen || m.commandOverlay == "" {
		t.Fatal("palette preempted command overlay")
	}
}

func TestCommandPaletteClosesForAsynchronousGateAndCommandResult(t *testing.T) {
	d := &testDriver{}
	m := New(d)
	m.openCommandPalette()
	d.gate = &surface.Gate{Kind: "question", ID: "question_1"}
	updated, _ := m.Update(surface.RefreshMsg{})
	m = updated.(Model)
	if m.commandPaletteOpen {
		t.Fatal("asynchronous gate left palette open")
	}
	d.gate = nil
	updated, _ = m.Update(surface.GateResolvedMsg{Kind: "question"})
	m = updated.(Model)
	if m.commandPaletteOpen {
		t.Fatal("palette reappeared after gate resolution")
	}

	m.openCommandPalette()
	updated, _ = m.Update(surface.CommandResultMsg{Name: "stats", Output: "ready"})
	m = updated.(Model)
	if m.commandPaletteOpen || m.commandOverlay != "ready" {
		t.Fatalf("command result palette=%v overlay=%q", m.commandPaletteOpen, m.commandOverlay)
	}
}

func TestCommandPaletteRendersWithinSmallTerminal(t *testing.T) {
	m := New(&testDriver{})
	m.width, m.height = 32, 10
	m.openCommandPalette()
	got := m.View()
	if !strings.Contains(got, "Commands") || !strings.Contains(got, "enter") || !strings.Contains(got, "/help") || !strings.Contains(got, "╯") || len(strings.Split(got, "\n")) != 10 {
		t.Fatalf("small palette escaped frame (%d lines):\n%s", len(strings.Split(got, "\n")), got)
	}
}

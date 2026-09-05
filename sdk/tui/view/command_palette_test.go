package view

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"

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
	if m.commandOverlayTitle != "命令" || d.commandName != "" {
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
	if !strings.Contains(m.View(), "没有匹配的命令") {
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
	if !strings.Contains(got, "帮助") || !strings.Contains(got, "enter") || !strings.Contains(got, "/help") || !strings.Contains(got, "╯") || len(strings.Split(got, "\n")) != 10 {
		t.Fatalf("small palette escaped frame (%d lines):\n%s", len(strings.Split(got, "\n")), got)
	}
}

func TestOverlayBlanksMainContentInsteadOfMixing(t *testing.T) {
	driver := &testDriver{
		sessions: []surface.Session{{ID: "active", Title: "Current"}},
		active:   "active",
		messages: map[string][]surface.Message{
			"active": {{Role: surface.RoleAssistant, Content: "UNIQUE_CHAT_MARKER_XYZ"}},
		},
	}
	m := New(driver)
	next, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 36})
	m = next.(Model)
	if !strings.Contains(ansi.Strip(m.View()), "UNIQUE_CHAT_MARKER_XYZ") {
		t.Fatal("setup missing chat marker")
	}
	m = paletteKey(t, m, tea.KeyMsg{Type: tea.KeyCtrlX})
	plain := ansi.Strip(m.View())
	if strings.Contains(plain, "UNIQUE_CHAT_MARKER_XYZ") {
		t.Fatalf("overlay mixed with main chat:\n%s", plain)
	}
	if !strings.Contains(plain, "快捷方式") {
		t.Fatalf("shortcuts overlay missing:\n%s", plain)
	}
}

func TestInputChromeUsesShiftHHelpAndKeepsKeysOnTheRight(t *testing.T) {
	driver := &testDriver{
		sessions: []surface.Session{{ID: "active", Title: "Current", PermissionPreset: "smart"}},
		active:   "active",
		thinking: "on",
		sidebar: surface.Sidebar{
			Session:    surface.Session{ID: "active", Title: "Current", PermissionPreset: "smart"},
			Model:      "deepseek-v4-flash",
			Provider:   "openai",
			HasContext: true,
			Context:    surface.Context{FeedTokens: 1200, ModelLimitTokens: 8000, ModelLimitKnown: true},
		},
		meta: surface.Meta{Host: "127.0.0.1:8787"},
	}
	m := New(driver)
	next, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 36})
	m = next.(Model)
	plain := ansi.Strip(m.View())
	for _, want := range []string{"deepseek-v4-flash(high)", "openai", "15%", "shift+tab", "切换模式", "智能", "shift+h", "帮助", "ctrl+x", "快捷", "host · 127.0.0.1:8787"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("input chrome/sidebar omitted %q:\n%s", want, plain)
		}
	}
	if strings.Contains(plain, "^p 命令") || strings.Contains(plain, "pgup/pgdn") {
		t.Fatalf("old footer dump still on screen:\n%s", plain)
	}
	if strings.Contains(plain, "TUI") || strings.Contains(plain, " · live") || strings.Contains(plain, " model · ") || strings.Contains(plain, " provider · ") || strings.Contains(plain, " permission · ") {
		t.Fatalf("redundant TUI/live/model chrome still visible:\n%s", plain)
	}

	editor := ansi.Strip(m.renderEditor(computeLayout(120, 36).mainW(), DefaultPalette()))
	if !strings.Contains(editor, "╭") || !strings.Contains(editor, "deepseek-v4-flash(high)") || !strings.Contains(editor, "openai") || !strings.Contains(editor, "15%") {
		t.Fatalf("rounded composer missing model chips:\n%s", editor)
	}
	chrome := strings.TrimLeft(ansi.Strip(m.renderInputChrome(computeLayout(120, 36).mainW(), DefaultPalette())), " ")
	if !strings.HasPrefix(chrome, "shift+tab") || !strings.Contains(chrome, "切换模式") || strings.Index(chrome, "shift+tab") > strings.Index(chrome, "shift+h") {
		t.Fatalf("hint row is not left-aligned switch-mode chrome:\n%s", chrome)
	}

	m = paletteKey(t, m, tea.KeyMsg{Type: tea.KeyCtrlX})
	plain = ansi.Strip(m.View())
	if !m.shortcutsOpen || !strings.Contains(plain, "快捷方式") || !strings.Contains(plain, "ctrl+s") {
		t.Fatalf("ctrl+x did not open shortcuts:\n%s", plain)
	}
	m = paletteKey(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.shortcutsOpen {
		t.Fatal("esc did not close shortcuts")
	}

	m = paletteKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("H")})
	if !m.commandPaletteOpen || m.shortcutsOpen || !strings.Contains(ansi.Strip(m.View()), "帮助") {
		t.Fatalf("shift+h did not open help: palette=%v shortcuts=%v\n%s", m.commandPaletteOpen, m.shortcutsOpen, m.View())
	}
	m = paletteKey(t, m, tea.KeyMsg{Type: tea.KeyEsc})

	m = paletteKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")})
	if m.commandPaletteOpen || m.input != "h" {
		t.Fatalf("lowercase h was stolen as help: palette=%v input=%q", m.commandPaletteOpen, m.input)
	}
	m.input = "draft"
	m = paletteKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("H")})
	if m.commandPaletteOpen || m.input != "draftH" {
		t.Fatalf("shift+h stole an in-progress draft: palette=%v input=%q", m.commandPaletteOpen, m.input)
	}
}

func TestShiftTabCyclesWorkingModes(t *testing.T) {
	driver := &testDriver{
		sessions: []surface.Session{{ID: "active", Title: "Current", PermissionPreset: "smart"}},
		active:   "active",
		sidebar:  surface.Sidebar{Session: surface.Session{ID: "active", Title: "Current", PermissionPreset: "smart"}},
	}
	m := New(driver)
	next, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 36})
	m = next.(Model)
	if got := ansi.Strip(m.renderInputChrome(80, DefaultPalette())); !strings.Contains(got, "shift+tab") || !strings.Contains(got, "切换模式") {
		t.Fatalf("default mode chrome:\n%s", got)
	}
	if got := ansi.Strip(m.renderEditor(80, DefaultPalette())); !strings.Contains(got, "智能") {
		t.Fatalf("default composer mode chip:\n%s", got)
	}

	m = paletteKey(t, m, tea.KeyMsg{Type: tea.KeyShiftTab})
	if driver.runMode != "plan" || driver.sessions[0].PermissionPreset != "smart" {
		t.Fatalf("smart → plan: mode=%q perm=%q", driver.runMode, driver.sessions[0].PermissionPreset)
	}
	if got := ansi.Strip(m.renderEditor(80, DefaultPalette())); !strings.Contains(got, "计划") {
		t.Fatalf("plan composer chip:\n%s", got)
	}

	m = paletteKey(t, m, tea.KeyMsg{Type: tea.KeyShiftTab})
	if driver.runMode != "normal" || driver.sessions[0].PermissionPreset != "cautious" {
		t.Fatalf("plan → readonly: mode=%q perm=%q", driver.runMode, driver.sessions[0].PermissionPreset)
	}
	if got := ansi.Strip(m.renderEditor(80, DefaultPalette())); !strings.Contains(got, "只读") {
		t.Fatalf("readonly composer chip:\n%s", got)
	}

	m = paletteKey(t, m, tea.KeyMsg{Type: tea.KeyShiftTab})
	if driver.runMode != "normal" || driver.sessions[0].PermissionPreset != "smart" {
		t.Fatalf("readonly → smart: mode=%q perm=%q", driver.runMode, driver.sessions[0].PermissionPreset)
	}

	m.input = "draft"
	m = paletteKey(t, m, tea.KeyMsg{Type: tea.KeyShiftTab})
	if driver.sessions[0].PermissionPreset != "smart" || driver.runMode != "normal" {
		t.Fatal("shift+tab stole an in-progress draft")
	}

	driver.busy = true
	m.input = ""
	m = paletteKey(t, m, tea.KeyMsg{Type: tea.KeyShiftTab})
	if driver.runMode != "normal" {
		t.Fatal("shift+tab cycled while busy")
	}
}

func TestInputChromeUsesColorWithoutTTY(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	driver := &testDriver{
		sessions: []surface.Session{{ID: "active", Title: "Current", PermissionPreset: "smart"}},
		active:   "active",
		thinking: "on",
		messages: map[string][]surface.Message{
			"active": {{Role: surface.RoleUser, Content: "hello from the user"}},
		},
		sidebar: surface.Sidebar{
			Session:    surface.Session{ID: "active", Title: "Current", PermissionPreset: "smart"},
			Model:      "deepseek-v4-flash",
			Provider:   "openai",
			HasContext: true,
			Context:    surface.Context{FeedTokens: 7200, ModelLimitTokens: 8000, ModelLimitKnown: true},
		},
	}
	m := New(driver)
	next, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 36})
	m = next.(Model)
	view := m.View()
	if !strings.Contains(view, "\x1b[") {
		t.Fatalf("expected ANSI color in chrome/chat:\n%s", view)
	}
	if !strings.Contains(ansi.Strip(view), "90%") || !strings.Contains(ansi.Strip(view), "(high)") {
		t.Fatalf("missing hot context or high intensity:\n%s", ansi.Strip(view))
	}
}

func TestCommandPaletteHighlightsMatchesAndHidesUnselectedUsage(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	d := &commandDriver{testDriver: &testDriver{}}
	m := New(d)
	m = paletteKey(t, m, tea.KeyMsg{Type: tea.KeyCtrlP})
	m = paletteKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("只读资源")})
	view := m.View()
	plain := ansi.Strip(view)
	if !strings.Contains(plain, "/mcp") || !strings.Contains(plain, "查看 MCP") {
		t.Fatalf("palette lost short command row:\n%s", plain)
	}
	if strings.Count(plain, "[server|resources") != 1 {
		t.Fatalf("unselected rows leaked long usage:\n%s", plain)
	}
	if !strings.Contains(view, "\x1b[") {
		t.Fatalf("palette missing match highlight:\n%s", view)
	}
}

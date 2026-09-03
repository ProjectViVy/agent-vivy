package view

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"agent-vivy/sdk/tui/surface"
)

type commandDriver struct {
	*testDriver
	commandName string
	commandArgs []string
}

func (d *commandDriver) ExecuteCommand(name string, args []string) tea.Cmd {
	d.commandName = name
	d.commandArgs = append([]string(nil), args...)
	return func() tea.Msg { return surface.RefreshMsg{} }
}

func TestEnterKeepsPlainTextAndDoubleSlashLiteral(t *testing.T) {
	d := &testDriver{}
	m := New(d)
	m.input = "  ordinary  text 🙂"
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil || d.sent != "  ordinary  text 🙂" {
		t.Fatalf("plain Enter sent=%q cmd=%v", d.sent, cmd != nil)
	}
	m = updated.(Model)
	m.input = "//not-a-command"
	updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil || d.sent != "/not-a-command" {
		t.Fatalf("escaped slash sent=%q cmd=%v", d.sent, cmd != nil)
	}
}

func TestEnterUnknownSlashCommandNeverSendsToDriver(t *testing.T) {
	d := &testDriver{}
	m := New(d)
	m.input = `/not-registered "🙂"`
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if cmd != nil || d.sent != "" {
		t.Fatalf("unknown command sent=%q cmd=%v", d.sent, cmd != nil)
	}
	if !strings.Contains(m.View(), "unknown command /not-registered") {
		t.Fatalf("unknown command was not rendered locally:\n%s", m.View())
	}
	if m.input != `/not-registered "🙂"` {
		t.Fatalf("unknown command draft was discarded: %q", m.input)
	}
}

func TestAsyncCommandResultIsRendered(t *testing.T) {
	m := New(&testDriver{})
	updated, _ := m.Update(surface.CommandResultMsg{Output: "queued turns cleared"})
	m = updated.(Model)
	if !strings.Contains(m.View(), "queued turns cleared") {
		t.Fatalf("async command output was dropped:\n%s", m.View())
	}
	updated, _ = m.Update(surface.CommandResultMsg{Err: fmt.Errorf("command failed")})
	m = updated.(Model)
	if !strings.Contains(m.View(), "command failed") {
		t.Fatalf("async command error was dropped:\n%s", m.View())
	}
}

func TestCommandExecutorReceivesParsedUnicodeArguments(t *testing.T) {
	d := &commandDriver{testDriver: &testDriver{}}
	m := New(d)
	m.input = `/new "你好 世界" emoji\ 🙂`
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if cmd == nil || d.commandName != "new" {
		t.Fatalf("command name=%q cmd=%v", d.commandName, cmd != nil)
	}
	if got, want := strings.Join(d.commandArgs, "|"), "你好 世界|emoji 🙂"; got != want {
		t.Fatalf("args=%q, want %q", got, want)
	}
	if m.input != "" {
		t.Fatalf("command draft was not cleared: %q", m.input)
	}
}

func TestCommandHelpStatusAndSessionsStayInSharedTeaState(t *testing.T) {
	d := &testDriver{
		sessions: []surface.Session{{ID: "sess_1", Title: "Current", PermissionPreset: "smart"}},
		active:   "sess_1",
	}
	m := New(d)
	m.input = "/help"
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if cmd != nil || !strings.Contains(m.View(), "/sessions") {
		t.Fatalf("help command state/output invalid: cmd=%v\n%s", cmd != nil, m.View())
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.commandOverlay != "" {
		t.Fatal("escape did not close command overlay")
	}
	m.input = "/status"
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if !strings.Contains(m.View(), "sess_1") || !strings.Contains(m.View(), "permission: smart") {
		t.Fatalf("status omitted active facts:\n%s", m.View())
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	m.input = "/sessions"
	updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if !m.sessionsOpen || cmd == nil {
		t.Fatalf("/sessions did not open the existing picker: open=%v cmd=%v", m.sessionsOpen, cmd != nil)
	}
}

func TestCommandDeleteUsesExistingConfirmationAndBusyFailsClosed(t *testing.T) {
	d := &commandDriver{testDriver: &testDriver{
		sessions: []surface.Session{{ID: "active", Title: "Current"}, {ID: "other", Title: "Other"}},
		active:   "active",
	}}
	m := New(d)
	m.input = "/delete other"
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if !m.sessionsOpen || m.sessionDeleteID != "other" || cmd == nil {
		t.Fatalf("delete did not enter existing confirmation: open=%v id=%q cmd=%v", m.sessionsOpen, m.sessionDeleteID, cmd != nil)
	}
	if d.commandName != "" {
		t.Fatal("delete bypassed the existing session confirmation/controller")
	}

	busy := &commandDriver{testDriver: &testDriver{busy: true}}
	m = New(busy)
	m.input = `/new unsafe while busy`
	updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if cmd != nil || busy.commandName != "" || !strings.Contains(m.View(), "unavailable") {
		t.Fatalf("busy mutation was not fail-closed: cmd=%v command=%q\n%s", cmd != nil, busy.commandName, m.View())
	}
}

func TestGateHasPriorityOverSlashCommands(t *testing.T) {
	d := &commandDriver{testDriver: &testDriver{gate: &surface.Gate{Kind: "approval", ID: "a1"}}}
	m := New(d)
	m.input = "/help"
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if cmd != nil || d.commandName != "" || m.commandOverlay != "" {
		t.Fatalf("gate did not take priority: cmd=%v command=%q overlay=%q", cmd != nil, d.commandName, m.commandOverlay)
	}
}

func TestQuitAliasesReturnTeaQuit(t *testing.T) {
	for _, input := range []string{"/quit", "/exit", "/q"} {
		d := &testDriver{}
		m := New(d)
		m.input = input
		updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		_ = updated.(Model)
		if cmd == nil {
			t.Fatalf("%s returned nil command", input)
		}
		if _, ok := cmd().(tea.QuitMsg); !ok {
			t.Fatalf("%s returned %T, want tea.QuitMsg", input, cmd())
		}
	}
}

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

type contextCommandDriver struct {
	*testDriver
	contextText  string
	contextPaths []string
	shellScript  string
}

func (d *commandDriver) ExecuteCommand(name string, args []string) tea.Cmd {
	d.commandName = name
	d.commandArgs = append([]string(nil), args...)
	return func() tea.Msg { return surface.RefreshMsg{} }
}

func (d *contextCommandDriver) SendWithContext(text string, paths []string) tea.Cmd {
	d.contextText = text
	d.contextPaths = append([]string(nil), paths...)
	return func() tea.Msg { return surface.RefreshMsg{} }
}

func (d *contextCommandDriver) ExecuteShell(script string) tea.Cmd {
	d.shellScript = script
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

func TestEnterSingleBangAndAtFailClosedWithoutGovernedDriverSeam(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  string
	}{
		{input: "!echo hi", want: "! shell commands are unavailable"},
		{input: "@README.md", want: "@file references require a prompt"},
	} {
		d := &testDriver{}
		m := New(d)
		m.input = tc.input
		updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		m = updated.(Model)
		if cmd != nil || d.sent != "" {
			t.Fatalf("input %q escaped local guard: sent=%q cmd=%v", tc.input, d.sent, cmd != nil)
		}
		if !strings.Contains(m.View(), tc.want) {
			t.Fatalf("input %q missing diagnostic %q:\n%s", tc.input, tc.want, m.View())
		}
	}
}

func TestEnterFileReferenceUsesContextSenderAndStripsMarkers(t *testing.T) {
	d := &contextCommandDriver{testDriver: &testDriver{}}
	m := New(d)
	m.input = "summarize @README.md and @internal/app.go"
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if cmd == nil || d.contextText != "summarize  and " || strings.Join(d.contextPaths, "|") != "README.md|internal/app.go" {
		t.Fatalf("file context route text=%q paths=%q cmd=%v", d.contextText, strings.Join(d.contextPaths, "|"), cmd != nil)
	}
	if d.sent != "" || m.input != "" {
		t.Fatalf("file context leaked to plain send or retained input: sent=%q input=%q", d.sent, m.input)
	}
}

func TestFailedFileSubmissionRestoresRetryInput(t *testing.T) {
	m := New(&testDriver{})
	m.input = "new draft"
	updated, _ := m.Update(surface.RestoreInputMsg{Text: "inspect @README.md"})
	m = updated.(Model)
	if m.input != "inspect @README.md new draft" {
		t.Fatalf("restored input = %q", m.input)
	}
}

func TestEnterShellUsesGovernedShellExecutorAndPreservesScript(t *testing.T) {
	d := &contextCommandDriver{testDriver: &testDriver{}}
	m := New(d)
	m.input = "  !  printf 'hi there'  "
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if cmd == nil || d.shellScript != "  printf 'hi there'  " {
		t.Fatalf("shell route script=%q cmd=%v", d.shellScript, cmd != nil)
	}
	if d.sent != "" || m.input != "" {
		t.Fatalf("shell leaked to plain send or retained input: sent=%q input=%q", d.sent, m.input)
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

func TestImageCommandStaysLocalAndCarriesRelativePath(t *testing.T) {
	d := &commandDriver{testDriver: &testDriver{}}
	m := New(d)
	m.input = `/image assets/photo.png`
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	_ = updated.(Model)
	if cmd == nil || d.commandName != "image" || strings.Join(d.commandArgs, "|") != "assets/photo.png" {
		t.Fatalf("image command route = %q %q cmd=%v", d.commandName, strings.Join(d.commandArgs, "|"), cmd != nil)
	}
	if d.sent != "" {
		t.Fatalf("image command leaked to model text: %q", d.sent)
	}
}

func TestMCPResourceCommandsUseSharedExecutorAndNeverSendModelText(t *testing.T) {
	for _, tc := range []struct {
		input string
		args  string
	}{
		{input: "/mcp resources docs", args: "resources|docs"},
		{input: "/mcp read docs docs://guide", args: "read|docs|docs://guide"},
	} {
		d := &commandDriver{testDriver: &testDriver{}}
		m := New(d)
		m.input = tc.input
		updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		_ = updated.(Model)
		if cmd == nil || d.commandName != "mcp" || strings.Join(d.commandArgs, "|") != tc.args {
			t.Fatalf("input %q routed name=%q args=%q cmd=%v", tc.input, d.commandName, strings.Join(d.commandArgs, "|"), cmd != nil)
		}
		if d.sent != "" {
			t.Fatalf("input %q was sent as model text: %q", tc.input, d.sent)
		}
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

func TestThinkingCommandAndShortcutUseTruthfulModelCapability(t *testing.T) {
	d := &testDriver{sidebar: surface.Sidebar{HasContext: true, Context: surface.Context{ThinkingSupported: true}}}
	m := New(d)
	m.input = "/thinking on"
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if cmd != nil || d.thinking != "on" || !strings.Contains(m.View(), "thinking: on") {
		t.Fatalf("thinking command = mode %q cmd=%v\n%s", d.thinking, cmd != nil, m.View())
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyCtrlT})
	m = updated.(Model)
	if cmd != nil || d.thinking != "off" {
		t.Fatalf("Ctrl+T did not cycle snapshotted preference: %q", d.thinking)
	}

	unsupported := &testDriver{sidebar: surface.Sidebar{HasContext: true}}
	m = New(unsupported)
	m.input = "/thinking on"
	updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if cmd != nil || unsupported.thinking == "on" || !strings.Contains(m.View(), "unavailable for the active model") {
		t.Fatalf("unsupported model accepted thinking: mode=%q view=%s", unsupported.thinking, m.View())
	}
	unknown := &testDriver{}
	m = New(unknown)
	m.input = "/thinking on"
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if unknown.thinking == "on" || strings.Contains(m.renderHelp(computeLayout(120, 30), DefaultPalette()), "^t") {
		t.Fatalf("unknown capability exposed or accepted thinking: mode=%q", unknown.thinking)
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

func TestAdvancedSessionCommandsRequireConfirmation(t *testing.T) {
	d := &commandDriver{testDriver: &testDriver{sessions: []surface.Session{{ID: "sess_1", Title: "one"}}, active: "sess_1"}}
	m := New(d)
	for _, input := range []string{"/compact", "/fork msg-1", "/rewind msg-1"} {
		m.input = input
		updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		m = updated.(Model)
		if cmd != nil || m.commandConfirmName == "" || d.commandName != "" {
			t.Fatalf("%s bypassed confirmation: cmd=%v confirm=%q driver=%q", input, cmd != nil, m.commandConfirmName, d.commandName)
		}
		if !strings.Contains(m.View(), "confirm") {
			t.Fatalf("%s confirmation was not rendered:\n%s", input, m.View())
		}
		updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
		m = updated.(Model)
		if cmd != nil || m.commandConfirmName != "" {
			t.Fatalf("%s denial left confirmation active", input)
		}
	}
}

func TestAdvancedSessionCommandConfirmationGuardsSessionEpoch(t *testing.T) {
	d := &commandDriver{testDriver: &testDriver{sessions: []surface.Session{{ID: "sess_1", Title: "one"}, {ID: "sess_2", Title: "two"}}, active: "sess_1"}}
	m := New(d)
	m.input = "/rewind msg-1"
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	d.active = "sess_2"
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m = updated.(Model)
	if cmd != nil || d.commandName != "" || !strings.Contains(m.View(), "active session changed") {
		t.Fatalf("session epoch guard failed: cmd=%v driver=%q view=%s", cmd != nil, d.commandName, m.View())
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

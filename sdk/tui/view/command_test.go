package view

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	corei18n "agent-vivy/internal/i18n"
	"agent-vivy/sdk/tui/surface"
)

type commandDriver struct {
	*testDriver
	commandName string
	commandArgs []string
}

type dynamicCommandDriver struct {
	*testDriver
	commands []surface.DynamicCommand
	id       string
	args     []string
	refresh  uint64
	cancels  []uint64
}

func (*dynamicCommandDriver) SupportsCapability(name string) bool {
	return name == "commands.list" || name == "commands.expand"
}

func (d *dynamicCommandDriver) RefreshDynamicCommands(request uint64) tea.Cmd {
	d.refresh = request
	return func() tea.Msg { return surface.DynamicCommandsMsg{Request: request, Commands: d.commands} }
}

func (d *dynamicCommandDriver) DynamicCommands() []surface.DynamicCommand {
	return append([]surface.DynamicCommand(nil), d.commands...)
}

func (d *dynamicCommandDriver) ExecuteDynamicCommand(request uint64, sessionID, id string, args []string) tea.Cmd {
	d.id = id
	d.args = append([]string(nil), args...)
	return func() tea.Msg {
		return surface.DynamicCommandExpandedMsg{Request: request, SessionID: sessionID, ID: id, Text: "expanded skill input"}
	}
}

func (d *dynamicCommandDriver) CancelDynamicCommand(request uint64) {
	d.cancels = append(d.cancels, request)
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

func (d *contextCommandDriver) SupportsCapability(name string) bool {
	return name == "shell.start"
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

func TestEnterUnknownSlashCommandLocalizesFramingAndPreservesToken(t *testing.T) {
	d := &testDriver{}
	const input = `/not-registered-原样 "🙂"`
	m := New(d, Options{Locale: corei18n.Chinese})
	m.input = input

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)

	if cmd != nil || d.sent != "" {
		t.Fatalf("unknown command sent=%q cmd=%v", d.sent, cmd != nil)
	}
	if !strings.Contains(m.View(), "未知命令 /not-registered-原样") {
		t.Fatalf("unknown command framing or token was not localized safely:\n%s", m.View())
	}
	if strings.Contains(m.View(), "unknown command") {
		t.Fatalf("unknown command retained English framing:\n%s", m.View())
	}
	if m.input != input {
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

func TestDynamicSkillCommandFiltersDispatchesAndSendsExpandedInput(t *testing.T) {
	d := &dynamicCommandDriver{testDriver: &testDriver{sessions: []surface.Session{{ID: "session-1"}}, active: "session-1"}, commands: []surface.DynamicCommand{
		{ID: "skill:review", Kind: "skill", Name: "review", Usage: "/review focus=<value>", Description: "Review \u202ethe current change"},
		{ID: "skill:shadow", Kind: "skill", Name: "help", Description: "must not shadow static help"},
		{ID: "skill:unsafe", Kind: "skill", Name: "bad/name", Description: "must be ignored"},
	}}
	m := New(d)
	m.openCommandPalette()
	if d.refresh == 0 || !m.commandCatalogLoading {
		t.Fatal("opening the command palette did not start a catalog refresh")
	}
	m.commandPaletteFilter = "current change"
	rows := m.filteredCommands()
	if len(rows) != 1 || rows[0].Name != "review" {
		t.Fatalf("dynamic palette rows = %+v", rows)
	}

	m.commandPaletteOpen = false
	m.input = `/review "this patch"`
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if cmd == nil || d.id != "skill:review" || strings.Join(d.args, "|") != "this patch" || d.sent != "" {
		t.Fatalf("dynamic dispatch id=%q args=%q sent=%q cmd=%v", d.id, strings.Join(d.args, "|"), d.sent, cmd != nil)
	}
	updated, sendCmd := m.Update(surface.DynamicCommandExpandedMsg{Request: m.dynamicCommandRequest, SessionID: "session-1", ID: d.id, Text: "expanded skill input"})
	m = updated.(Model)
	if sendCmd == nil || d.sent != "expanded skill input" {
		t.Fatalf("expanded command sent=%q cmd=%v", d.sent, sendCmd != nil)
	}

	m.commandOverlay = ""
	m.input = "/help"
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if m.commandOverlayTitle != "Commands" || !strings.Contains(m.commandOverlay, "/review focus=<value>") || strings.Contains(m.commandOverlay, "\u202e") {
		t.Fatalf("effective help omitted dynamic command or static help was shadowed:\n%s", m.commandOverlay)
	}
}

func TestDynamicCommandExpansionFailureNeverSends(t *testing.T) {
	d := &dynamicCommandDriver{testDriver: &testDriver{sessions: []surface.Session{{ID: "session-1"}}, active: "session-1"}}
	m := New(d)
	m.dynamicCommandRequest = 1
	m.dynamicCommandPending = true
	m.dynamicCommandID = "skill:gone"
	m.dynamicCommandSession = "session-1"
	m.dynamicCommandDraft = "/gone"
	updated, cmd := m.Update(surface.DynamicCommandExpandedMsg{Request: 1, SessionID: "session-1", ID: "skill:gone", Err: fmt.Errorf("dynamic command is unavailable")})
	m = updated.(Model)
	if cmd != nil || d.sent != "" || !strings.Contains(m.View(), "dynamic command is unavailable") {
		t.Fatalf("failed dynamic expansion escaped guard: sent=%q cmd=%v\n%s", d.sent, cmd != nil, m.View())
	}
}

func TestDynamicCommandSendRefusalRestoresDraft(t *testing.T) {
	d := &dynamicCommandDriver{testDriver: &testDriver{sessions: []surface.Session{{ID: "session-1"}}, active: "session-1", sendBlocked: true}, commands: []surface.DynamicCommand{{ID: "skill:review", Kind: "skill", Name: "review"}}}
	m := New(d)
	m.input = `/review "this patch"`
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if cmd == nil || !m.dynamicCommandPending {
		t.Fatal("dispatch did not enter pending state")
	}
	updated, sendCmd := m.Update(surface.DynamicCommandExpandedMsg{Request: m.dynamicCommandRequest, SessionID: "session-1", ID: d.id, Text: "expanded skill input"})
	m = updated.(Model)
	if sendCmd != nil || d.sent != "" || m.input != `/review "this patch"` {
		t.Fatalf("refused send dropped the draft: sent=%q input=%q cmd=%v", d.sent, m.input, sendCmd != nil)
	}
	if m.dynamicCommandPending {
		t.Fatal("pending state was not cleared after the refused send")
	}
	if !strings.Contains(m.View(), "could not start a turn") {
		t.Fatalf("refusal was not surfaced:\n%s", m.View())
	}
}

func TestDynamicCommandPendingLocksEditorAndSurfaces(t *testing.T) {
	d := &dynamicCommandDriver{testDriver: &testDriver{sessions: []surface.Session{{ID: "session-1"}}, active: "session-1"}, commands: []surface.DynamicCommand{{ID: "skill:review", Kind: "skill", Name: "review"}}}
	m := New(d)
	m.input = `/review "first"`
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if cmd == nil || !m.dynamicCommandPending {
		t.Fatal("dispatch did not enter pending state")
	}
	draft := m.dynamicCommandDraft

	// Ctrl+C stays available while the expansion owns every other key.
	updated, quitCmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	m = updated.(Model)
	if quitCmd == nil {
		t.Fatal("ctrl+c did not respond while expansion was pending")
	}
	if _, ok := quitCmd().(tea.QuitMsg); !ok {
		t.Fatalf("ctrl+c returned %T, want tea.QuitMsg", quitCmd())
	}
	if !m.dynamicCommandPending || m.input != "" {
		t.Fatalf("ctrl+c disturbed the pending state: pending=%v input=%q", m.dynamicCommandPending, m.input)
	}

	// Typing, submitting, and secondary surfaces are swallowed while the
	// expansion is in flight.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	m = updated.(Model)
	updated, enter := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	updated, ctrlT := m.Update(tea.KeyMsg{Type: tea.KeyCtrlT})
	m = updated.(Model)
	if m.input != "" || d.sent != "" || enter != nil || ctrlT != nil || m.shortcutsOpen {
		t.Fatalf("pending state leaked input: input=%q sent=%q enter=%v ctrlT=%v shortcuts=%v", m.input, d.sent, enter != nil, ctrlT != nil, m.shortcutsOpen)
	}

	// Esc is the only exit besides Ctrl+C and restores the saved draft.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.dynamicCommandPending || m.input != draft {
		t.Fatalf("escape did not cancel: pending=%v input=%q", m.dynamicCommandPending, m.input)
	}
}

func TestDynamicCommandSerializesAndRestoresDraftAcrossCancellation(t *testing.T) {
	d := &dynamicCommandDriver{testDriver: &testDriver{sessions: []surface.Session{{ID: "session-1"}}, active: "session-1"}, commands: []surface.DynamicCommand{{ID: "skill:review", Kind: "skill", Name: "review"}}}
	m := New(d)
	m.input = `/review "first"`
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if cmd == nil || !m.dynamicCommandPending {
		t.Fatal("dynamic expansion did not enter pending state")
	}
	m.input = "ordinary text"
	updated, second := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	// The pending lock swallows keys before submitInput, so the second
	// submission is dropped without a diagnostic and without reaching Send.
	if second != nil || d.sent != "" || m.input != "ordinary text" {
		t.Fatalf("second submission was not blocked while expansion was pending: cmd=%v sent=%q input=%q", second != nil, d.sent, m.input)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.dynamicCommandPending || m.input != `/review "first"` {
		t.Fatalf("cancelled expansion pending=%v draft=%q", m.dynamicCommandPending, m.input)
	}
	updated, stale := m.Update(cmd())
	m = updated.(Model)
	if stale != nil || d.sent != "" || m.input != `/review "first"` {
		t.Fatalf("stale expansion escaped fence: sent=%q draft=%q", d.sent, m.input)
	}
}

func TestDynamicCommandEscapeCancelsInFlightExpansion(t *testing.T) {
	d := &dynamicCommandDriver{testDriver: &testDriver{sessions: []surface.Session{{ID: "session-1"}}, active: "session-1"}, commands: []surface.DynamicCommand{{ID: "skill:review", Kind: "skill", Name: "review"}}}
	m := New(d)
	m.input = `/review "first"`
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if cmd == nil || !m.dynamicCommandPending {
		t.Fatal("dispatch did not enter pending state")
	}
	request := m.dynamicCommandRequest
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.dynamicCommandPending || m.input != `/review "first"` {
		t.Fatalf("escape did not cancel: pending=%v input=%q", m.dynamicCommandPending, m.input)
	}
	if len(d.cancels) != 1 || d.cancels[0] != request {
		t.Fatalf("escape cancelled %v, want [%d]", d.cancels, request)
	}
	// The cancelled RPC still returns; the bumped request must discard it
	// silently instead of restoring a stale draft.
	updated, stale := m.Update(surface.DynamicCommandExpandedMsg{Request: request, SessionID: "session-1", ID: "skill:review", Err: fmt.Errorf("context canceled")})
	m = updated.(Model)
	if stale != nil || d.sent != "" {
		t.Fatalf("cancelled expansion leaked into the model: cmd=%v sent=%q", stale != nil, d.sent)
	}
}

func TestDynamicCommandSessionChangeCancelsAndClosesArgumentForm(t *testing.T) {
	d := &dynamicCommandDriver{testDriver: &testDriver{sessions: []surface.Session{{ID: "session-1"}, {ID: "session-2"}}, active: "session-1"}, commands: []surface.DynamicCommand{{
		ID: "mcp:prompt", Kind: "mcp_prompt", Name: "mcp-review",
		Arguments: []surface.DynamicCommandArgument{{Name: "focus", Required: true}},
	}}}
	m := New(d)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = updated.(Model)

	m.openCommandPalette()
	m.commandPaletteFilter = "mcp-review"
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if m.dynamicArgumentCommand == nil || m.dynamicArgumentSession != "session-1" {
		t.Fatalf("argument form session = %q", m.dynamicArgumentSession)
	}

	m.dynamicCommandPending = true
	m.dynamicCommandID = "skill:review"
	m.dynamicCommandSession = "session-1"
	m.dynamicCommandDraft = "/review"
	request := m.dynamicCommandRequest
	d.active = "session-2"
	updated, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = updated.(Model)
	if len(d.cancels) != 1 || d.cancels[0] != request {
		t.Fatalf("session switch cancelled %v, want [%d]", d.cancels, request)
	}
	if m.dynamicArgumentCommand != nil {
		t.Fatal("argument form survived the session switch")
	}

	// The cancelled RPC returns into the discard path: pending clears, the
	// draft is restored, and the mismatch is surfaced.
	updated, _ = m.Update(surface.DynamicCommandExpandedMsg{Request: request, SessionID: "session-1", ID: "skill:review", Err: fmt.Errorf("context canceled")})
	m = updated.(Model)
	if m.dynamicCommandPending || m.input != "/review" || !strings.Contains(m.View(), "active session changed") {
		t.Fatalf("cancelled expansion was not discarded: pending=%v input=%q", m.dynamicCommandPending, m.input)
	}
}

func TestDynamicCommandPaletteCollectsRequiredArguments(t *testing.T) {
	d := &dynamicCommandDriver{testDriver: &testDriver{sessions: []surface.Session{{ID: "session-1"}}, active: "session-1"}, commands: []surface.DynamicCommand{{
		ID: "mcp:prompt", Kind: "mcp_prompt", Name: "mcp-review", Usage: "/mcp-review focus=<value>", Description: "Review a change",
		Arguments: []surface.DynamicCommandArgument{{Name: "focus", Description: "review focus", Required: true}},
	}}}
	m := New(d)
	m.openCommandPalette()
	m.commandPaletteFilter = "mcp-review"
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if cmd != nil || m.dynamicArgumentCommand == nil || !strings.Contains(m.View(), "focus (required)") {
		t.Fatalf("argument form did not open:\n%s", m.View())
	}
	updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if cmd != nil || !strings.Contains(m.View(), "focus is required") {
		t.Fatal("required argument was not enforced")
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("security")})
	m = updated.(Model)
	updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if cmd == nil || d.id != "mcp:prompt" || strings.Join(d.args, "|") != "focus=security" {
		t.Fatalf("argument dispatch id=%q args=%q cmd=%v", d.id, d.args, cmd != nil)
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
	if unknown.thinking == "on" || strings.Contains(ansi.Strip(m.View()), "^t") {
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
	// Observe the already-busy driver once so the guarded key below is not the
	// first busy observation (which schedules a view-owned spinner tick cmd).
	updated, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = updated.(Model)
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
		if !strings.Contains(m.View(), "Confirm") {
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

// Package view is the shared Crush-style fullscreen shell for first-party
// built-in and packed terminal faces.
package view

import (
	"fmt"
	"io"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"agent-vivy/sdk/tui/command"
	"agent-vivy/sdk/tui/surface"
)

const (
	approvalApproved = "approved"
	approvalDenied   = "denied"
)

var commandRegistry = command.DefaultRegistry()

// Model is the single Bubble Tea model used by both first-party terminal
// faces. The Sessions dialog state is transient UI state; session data itself
// remains owned by the surface.Driver.
type Model struct {
	driver  surface.Driver
	width   int
	height  int
	input   string
	palette Palette

	sessionsOpen       bool
	sessionRows        []surface.Session
	sessionFilter      string
	sessionCursor      int
	sessionLoading     bool
	sessionRenaming    bool
	sessionRenameID    string
	sessionRenameInput string
	sessionDeleteID    string
	sessionActionBusy  bool
	sessionError       string
	sessionRequest     uint64

	commandOverlayTitle string
	commandOverlay      string
	commandConfirmName  string
	commandConfirmArgs  []string
	commandConfirmSID   string
}

// New returns a model bound to the given driver.
func New(driver surface.Driver) Model {
	if driver == nil {
		driver = noDriver{}
	}
	return Model{
		driver:  driver,
		width:   120,
		height:  36,
		palette: DefaultPalette(),
	}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	if m.driver == nil {
		return nil
	}
	return m.driver.Init()
}

// Update implements tea.Model. The driver sees every message first so its
// authoritative state is committed before the shared view consumes result
// messages such as SessionsMsg.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	if m.driver != nil {
		if cmd := m.driver.Handle(msg); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = max(1, msg.Width)
		m.height = max(1, msg.Height)
	case tea.KeyMsg:
		next, cmd := m.handleKey(msg)
		m = next
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	case surface.SessionsMsg:
		m.applySessionsMsg(msg)
	case surface.GateResolvedMsg:
		if msg.Kind == "question" {
			m.input = ""
		}
	case surface.CommandResultMsg:
		m.applyCommandResult(msg)
	case surface.ErrMsg:
		// The driver stores transport errors in Meta. Keep the dialog snapshot
		// and local input intact so a retry does not discard user work.
	}
	return m, tea.Batch(cmds...)
}

// View implements tea.Model.
func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "vivy tui"
	}
	return m.renderFrame()
}

func (m Model) handleKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	gate := m.driver.PendingGate()
	meta := m.driver.Meta()
	if gate != nil && m.sessionsOpen {
		// A gate may arrive asynchronously while the Sessions dialog is open.
		// Close the secondary surface before routing this key so the gate remains
		// visible and no session mutation can bypass it.
		m.sessionsOpen = false
		m.sessionRenaming = false
		m.sessionDeleteID = ""
		m.sessionActionBusy = false
	}
	if gate != nil && msg.Type == tea.KeyCtrlS {
		// A pending approval/question remains the top-most interaction. Do not
		// let a secondary dialog hide it or mutate another session through it.
		return m, nil
	}
	if gate != nil && m.commandOverlay != "" {
		// A gate arriving asynchronously is always more important than a local
		// command result. Drop the secondary overlay before routing input.
		m.commandOverlayTitle = ""
		m.commandOverlay = ""
	}
	if gate != nil && m.commandConfirmName != "" {
		m.clearCommandConfirmation()
	}
	if m.sessionsOpen {
		return m.handleSessionsKey(msg)
	}
	if gate == nil && m.commandConfirmName != "" {
		return m.handleCommandConfirmation(msg)
	}
	if gate == nil && m.commandOverlay != "" {
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit
		case tea.KeyEsc, tea.KeyEnter:
			m.commandOverlayTitle = ""
			m.commandOverlay = ""
		}
		return m, nil
	}
	if gate != nil && gate.Submitting && msg.Type != tea.KeyCtrlC && msg.Type != tea.KeyEsc {
		return m, nil
	}
	switch msg.Type {
	case tea.KeyCtrlC:
		return m, tea.Quit
	case tea.KeyCtrlS:
		return m.openSessions()
	case tea.KeyEsc:
		if gate != nil {
			return m, nil
		}
		if meta.Queued > 0 {
			m.driver.ClearQueue()
			return m, nil
		}
		if meta.Busy {
			return m, m.driver.Cancel()
		}
		m.input = ""
		return m, nil
	case tea.KeyCtrlN:
		if gate == nil && !meta.Busy {
			m.input = ""
			return m, m.driver.NewSession("")
		}
		return m, nil
	case tea.KeyCtrlY:
		if gate == nil && !meta.Busy {
			return m, m.driver.SetPermission(nextPermission(m.driver.Active().PermissionPreset))
		}
		return m, nil
	case tea.KeyCtrlT:
		if gate != nil {
			return m, nil
		}
		return m.setThinking("")
	case tea.KeyTab, tea.KeyShiftTab, tea.KeyUp, tea.KeyDown:
		// Session navigation belongs to the explicit Ctrl+S dialog. Keeping
		// arrows in the editor avoids the old hidden-session sidebar behavior.
		return m, nil
	case tea.KeyEnter:
		if gate != nil {
			if gate.Submitting {
				return m, nil
			}
			if gate.Kind == "question" {
				return m, m.driver.AnswerQuestion(m.input)
			}
			return m, nil
		}
		return m.submitInput()
	case tea.KeyBackspace:
		if gate != nil && gate.Kind == "approval" {
			return m, nil
		}
		m.input = removeLastRune(m.input)
		return m, nil
	case tea.KeyCtrlJ:
		if gate != nil && gate.Kind == "approval" {
			return m, nil
		}
		m.input += "\n"
		return m, nil
	case tea.KeyRunes:
		text := string(msg.Runes)
		if gate != nil && gate.Kind == "approval" {
			if gate.Submitting {
				return m, nil
			}
			switch strings.ToLower(strings.TrimSpace(text)) {
			case "y":
				return m, m.driver.DecideApproval(approvalApproved)
			case "n":
				return m, m.driver.DecideApproval(approvalDenied)
			}
			return m, nil
		}
		if text == "q" && m.input == "" && gate == nil && !meta.Busy {
			return m, tea.Quit
		}
		m.input += text
		return m, nil
	}
	if gate != nil && gate.Kind == "approval" {
		switch strings.ToLower(msg.String()) {
		case "y":
			return m, m.driver.DecideApproval(approvalApproved)
		case "n":
			return m, m.driver.DecideApproval(approvalDenied)
		}
	}
	return m, nil
}

func (m Model) submitInput() (Model, tea.Cmd) {
	parsed, err := commandRegistry.Parse(m.input)
	if err != nil {
		// Keep malformed/unknown command text in the editor so the operator can
		// dismiss the local error and correct it without retyping.
		return m.showCommandError(err), nil
	}
	if parsed.IsUnavailable() {
		return m.showCommandError(fmt.Errorf("%s", parsed.UnavailableReason)), nil
	}
	if !parsed.IsCommand() {
		cmd := m.driver.Send(parsed.Text)
		if cmd != nil {
			m.input = ""
		}
		return m, cmd
	}
	if err := commandRegistry.Validate(parsed.Invocation); err != nil {
		return m.showCommandError(err), nil
	}
	m.input = ""
	return m.dispatchCommand(parsed.Invocation)
}

func (m Model) dispatchCommand(invocation *command.Invocation) (Model, tea.Cmd) {
	if invocation == nil {
		return m.showCommandError(fmt.Errorf("missing command")), nil
	}
	spec, ok := commandRegistry.Lookup(invocation.Name)
	if !ok {
		// Registry.Parse already performs this check. Keep the guard here so a
		// future registry change cannot turn an unknown slash line into model
		// text.
		return m.showCommandError(fmt.Errorf("unknown command /%s", invocation.Name)), nil
	}
	name := spec.Name
	args := invocation.Args
	switch name {
	case "help":
		if len(args) != 0 {
			return m.showCommandError(fmt.Errorf("usage: /help")), nil
		}
		m.commandOverlayTitle = "Commands"
		m.commandOverlay = commandRegistry.Help()
		return m, nil
	case "status":
		if len(args) != 0 {
			return m.showCommandError(fmt.Errorf("usage: /status")), nil
		}
		m.commandOverlayTitle = "Status"
		m.commandOverlay = m.statusText()
		return m, nil
	case "sessions":
		if len(args) != 0 {
			return m.showCommandError(fmt.Errorf("usage: /sessions")), nil
		}
		return m.openSessions()
	case "new":
		if len(args) > 1 && strings.TrimSpace(strings.Join(args, " ")) == "" {
			return m.showCommandError(fmt.Errorf("usage: /new [title]")), nil
		}
		if blocked, reason := m.commandBlocked(name); blocked {
			return m.showCommandError(fmt.Errorf("%s", reason)), nil
		}
		return m.executeDriverCommand(name, args)
	case "session":
		if len(args) != 1 || strings.TrimSpace(args[0]) == "" {
			return m.showCommandError(fmt.Errorf("usage: /session <id>")), nil
		}
		if blocked, reason := m.commandBlocked(name); blocked {
			return m.showCommandError(fmt.Errorf("%s", reason)), nil
		}
		return m.executeDriverCommand(name, args)
	case "rename":
		if len(args) == 0 || strings.TrimSpace(strings.Join(args, " ")) == "" {
			return m.showCommandError(fmt.Errorf("usage: /rename <title>")), nil
		}
		if blocked, reason := m.commandBlocked(name); blocked {
			return m.showCommandError(fmt.Errorf("%s", reason)), nil
		}
		return m.executeDriverCommand(name, args)
	case "delete":
		if len(args) > 1 {
			return m.showCommandError(fmt.Errorf("usage: /delete [id]")), nil
		}
		if blocked, reason := m.commandBlocked(name); blocked {
			return m.showCommandError(fmt.Errorf("%s", reason)), nil
		}
		id := ""
		if len(args) == 1 {
			id = strings.TrimSpace(args[0])
		}
		return m.openSessionsForDelete(id)
	case "cancel":
		if len(args) != 0 {
			return m.showCommandError(fmt.Errorf("usage: /cancel")), nil
		}
		return m.executeDriverCommand(name, args)
	case "queue":
		if len(args) != 1 || !strings.EqualFold(args[0], "clear") {
			return m.showCommandError(fmt.Errorf("usage: /queue clear")), nil
		}
		return m.executeDriverCommand(name, args)
	case "permission":
		if len(args) > 1 {
			return m.showCommandError(fmt.Errorf("usage: /permission [cautious|smart|trusted]")), nil
		}
		if blocked, reason := m.commandBlocked(name); blocked {
			return m.showCommandError(fmt.Errorf("%s", reason)), nil
		}
		if len(args) == 1 {
			preset := strings.ToLower(strings.TrimSpace(args[0]))
			if preset != "cautious" && preset != "smart" && preset != "trusted" {
				return m.showCommandError(fmt.Errorf("permission must be cautious, smart, or trusted")), nil
			}
		}
		return m.executeDriverCommand(name, args)
	case "thinking":
		mode := ""
		if len(args) == 1 {
			mode = strings.ToLower(strings.TrimSpace(args[0]))
		}
		return m.setThinking(mode)
	case "compact":
		if blocked, reason := m.commandBlocked(name); blocked {
			return m.showCommandError(fmt.Errorf("%s", reason)), nil
		}
		return m.confirmCommand(name, args)
	case "fork", "rewind":
		if blocked, reason := m.commandBlocked(name); blocked {
			return m.showCommandError(fmt.Errorf("%s", reason)), nil
		}
		return m.confirmCommand(name, args)
	case "todos", "stats", "skills", "mcp", "files", "tools":
		// These commands are read-only snapshots. They remain useful while a
		// run is streaming; the driver still owns the authoritative RPC state.
		return m.executeDriverCommand(name, args)
	case "quit":
		if len(args) != 0 {
			return m.showCommandError(fmt.Errorf("usage: /quit")), nil
		}
		return m, tea.Quit
	default:
		return m.showCommandError(fmt.Errorf("unknown command /%s", invocation.Name)), nil
	}
}

func (m Model) setThinking(mode string) (Model, tea.Cmd) {
	controller, ok := m.driver.(surface.ThinkingController)
	if !ok {
		return m.showCommandError(fmt.Errorf("thinking control is unavailable")), nil
	}
	if mode == "" {
		mode = nextThinking(controller.ThinkingMode())
	}
	if mode != "auto" && mode != "on" && mode != "off" {
		return m.showCommandError(fmt.Errorf("thinking must be auto, on, or off")), nil
	}
	if err := controller.SetThinkingMode(mode); err != nil {
		return m.showCommandError(err), nil
	}
	return m.showCommandResult("Thinking", "next turn thinking: "+mode), nil
}

func nextThinking(current string) string {
	switch strings.ToLower(strings.TrimSpace(current)) {
	case "auto":
		return "on"
	case "on":
		return "off"
	default:
		return "auto"
	}
}

func (m Model) confirmCommand(name string, args []string) (Model, tea.Cmd) {
	sessionID := m.driver.Active().ID
	if sessionID == "" {
		return m.showCommandError(fmt.Errorf("no active session")), nil
	}
	m.commandConfirmName = name
	m.commandConfirmArgs = append([]string(nil), args...)
	m.commandConfirmSID = sessionID
	return m, nil
}

func (m Model) handleCommandConfirmation(msg tea.KeyMsg) (Model, tea.Cmd) {
	if msg.Type == tea.KeyCtrlC {
		return m, tea.Quit
	}
	decision := ""
	if msg.Type == tea.KeyRunes {
		decision = strings.ToLower(strings.TrimSpace(string(msg.Runes)))
	}
	if msg.Type == tea.KeyEsc || decision == "n" || decision == "no" {
		m.clearCommandConfirmation()
		return m, nil
	}
	if decision != "y" && decision != "yes" {
		return m, nil
	}
	name := m.commandConfirmName
	args := append([]string(nil), m.commandConfirmArgs...)
	sessionID := m.commandConfirmSID
	m.clearCommandConfirmation()
	if m.driver.Active().ID != sessionID {
		return m.showCommandError(fmt.Errorf("active session changed; /%s cancelled", name)), nil
	}
	if blocked, reason := m.commandBlocked(name); blocked {
		return m.showCommandError(fmt.Errorf("%s", reason)), nil
	}
	return m.executeDriverCommand(name, args)
}

func (m *Model) clearCommandConfirmation() {
	m.commandConfirmName = ""
	m.commandConfirmArgs = nil
	m.commandConfirmSID = ""
}

func (m Model) commandBlocked(name string) (bool, string) {
	if gate := m.driver.PendingGate(); gate != nil {
		return true, "a pending gate must be answered first"
	}
	if m.driver.Meta().Busy {
		return true, fmt.Sprintf("run in flight; /%s is unavailable (use /cancel)", name)
	}
	return false, ""
}

func (m Model) executeDriverCommand(name string, args []string) (Model, tea.Cmd) {
	if executor, ok := m.driver.(surface.CommandExecutor); ok {
		if cmd := executor.ExecuteCommand(name, append([]string(nil), args...)); cmd != nil {
			return m, cmd
		}
		return m.showCommandError(fmt.Errorf("/%s is unavailable", name)), nil
	}

	// Compatibility path for small/demo drivers. The same busy/gate checks
	// above apply before any mutation reaches this fallback.
	switch name {
	case "new":
		return m, m.driver.NewSession(strings.TrimSpace(strings.Join(args, " ")))
	case "session":
		controller, ok := m.driver.(surface.SessionController)
		if !ok {
			return m.showCommandError(fmt.Errorf("/session is unavailable")), nil
		}
		if cmd := controller.SelectSession(args[0]); cmd != nil {
			return m, cmd
		}
		return m.showCommandError(fmt.Errorf("/session is unavailable")), nil
	case "rename":
		controller, ok := m.driver.(surface.SessionController)
		active := m.driver.Active()
		if !ok || active.ID == "" {
			return m.showCommandError(fmt.Errorf("/rename is unavailable")), nil
		}
		if cmd := controller.RenameSession(active.ID, strings.TrimSpace(strings.Join(args, " "))); cmd != nil {
			return m, cmd
		}
		return m.showCommandError(fmt.Errorf("/rename is unavailable")), nil
	case "cancel":
		if !m.driver.Meta().Busy {
			return m.showCommandError(fmt.Errorf("nothing to cancel")), nil
		}
		if cmd := m.driver.Cancel(); cmd != nil {
			return m, cmd
		}
		return m.showCommandError(fmt.Errorf("cancel is unavailable")), nil
	case "queue":
		if m.driver.ClearQueue() {
			return m.showCommandResult("Queue", "queued turns cleared"), nil
		}
		return m.showCommandResult("Queue", "queue is already empty"), nil
	case "permission":
		preset := ""
		if len(args) == 1 {
			preset = strings.ToLower(strings.TrimSpace(args[0]))
		} else {
			preset = nextPermission(m.driver.Active().PermissionPreset)
		}
		if cmd := m.driver.SetPermission(preset); cmd != nil {
			return m, cmd
		}
		return m.showCommandError(fmt.Errorf("permission change is unavailable")), nil
	}
	return m.showCommandError(fmt.Errorf("/%s is unavailable", name)), nil
}

func (m Model) openSessionsForDelete(id string) (Model, tea.Cmd) {
	if id == "" {
		id = m.driver.Active().ID
	}
	if id == "" {
		return m.showCommandError(fmt.Errorf("no active session")), nil
	}
	m, cmd := m.openSessions()
	if len(m.sessionRows) > 0 {
		found := false
		for _, row := range m.sessionRows {
			if row.ID == id {
				found = true
				break
			}
		}
		if !found {
			m.sessionError = "session not found: " + id
			return m, cmd
		}
	}
	m.sessionDeleteID = id
	return m, cmd
}

func (m Model) showCommandResult(title, output string) Model {
	m.commandOverlayTitle = title
	m.commandOverlay = strings.TrimSpace(output)
	return m
}

func (m Model) showCommandError(err error) Model {
	if err == nil {
		return m
	}
	return m.showCommandResult("Command error", err.Error())
}

func (m *Model) applyCommandResult(msg surface.CommandResultMsg) {
	if msg.Err != nil {
		next := m.showCommandError(msg.Err)
		*m = next
		return
	}
	if strings.TrimSpace(msg.Output) != "" {
		next := m.showCommandResult("Command", msg.Output)
		*m = next
	}
}

func (m Model) statusText() string {
	meta := m.driver.Meta()
	active := m.driver.Active()
	var lines []string
	if active.ID == "" {
		lines = append(lines, "session: none")
	} else {
		title := strings.TrimSpace(active.Title)
		if title == "" {
			title = "untitled session"
		}
		lines = append(lines, "session: "+title, "id: "+active.ID)
	}
	if meta.Busy {
		line := "run: active"
		if meta.RunID != "" {
			line += " (" + meta.RunID + ")"
		}
		lines = append(lines, line)
	} else {
		lines = append(lines, "run: idle")
	}
	lines = append(lines, fmt.Sprintf("queue: %d", meta.Queued))
	if active.PermissionPreset != "" {
		lines = append(lines, "permission: "+active.PermissionPreset)
	}
	if controller, ok := m.driver.(surface.ThinkingController); ok {
		lines = append(lines, "thinking (next turn): "+controller.ThinkingMode())
	}
	if provider, ok := m.driver.(surface.SidebarProvider); ok {
		snapshot := provider.Sidebar()
		if snapshot.HasContext {
			ctx := snapshot.Context
			if ctx.ModelLimitTokens > 0 {
				lines = append(lines, fmt.Sprintf("context: %d/%d tokens", ctx.FeedTokens, ctx.ModelLimitTokens))
			} else if ctx.FeedTokens > 0 {
				lines = append(lines, fmt.Sprintf("context: %d tokens", ctx.FeedTokens))
			}
		}
	}
	if strings.TrimSpace(meta.Error) != "" {
		lines = append(lines, "error: "+meta.Error)
	}
	return strings.Join(lines, "\n")
}

func (m Model) openSessions() (Model, tea.Cmd) {
	m.sessionsOpen = true
	m.sessionRows = append([]surface.Session(nil), m.driver.Sessions()...)
	m.sessionFilter = ""
	m.sessionCursor = 0
	m.sessionLoading = false
	m.sessionRenaming = false
	m.sessionRenameID = ""
	m.sessionRenameInput = ""
	m.sessionDeleteID = ""
	m.sessionActionBusy = false
	m.sessionError = ""
	m.syncSessionCursorToActive()
	controller, ok := m.driver.(surface.SessionController)
	if ok {
		if cmd := controller.RefreshSessions(); cmd != nil {
			m.sessionLoading = true
			return m, cmd
		}
	}
	return m, nil
}

func (m Model) handleSessionsKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	if msg.Type == tea.KeyCtrlC {
		return m, tea.Quit
	}
	if m.sessionActionBusy {
		if msg.Type == tea.KeyEsc && !m.sessionRenaming && m.sessionDeleteID == "" {
			m.sessionsOpen = false
		}
		return m, nil
	}
	if m.sessionRenaming {
		switch msg.Type {
		case tea.KeyEsc:
			m.sessionRenaming = false
			m.sessionRenameID = ""
			m.sessionRenameInput = ""
			return m, nil
		case tea.KeyBackspace:
			m.sessionRenameInput = removeLastRune(m.sessionRenameInput)
			return m, nil
		case tea.KeyEnter:
			return m.submitRename()
		case tea.KeyRunes:
			m.sessionRenameInput += string(msg.Runes)
			return m, nil
		}
		return m, nil
	}
	if m.sessionDeleteID != "" {
		switch msg.Type {
		case tea.KeyEsc:
			m.sessionDeleteID = ""
			m.sessionError = ""
			return m, nil
		case tea.KeyRunes:
			switch strings.ToLower(strings.TrimSpace(string(msg.Runes))) {
			case "y":
				return m.submitDelete()
			case "n":
				m.sessionDeleteID = ""
				m.sessionError = ""
			}
		}
		return m, nil
	}

	switch msg.Type {
	case tea.KeyEsc:
		m.sessionsOpen = false
		m.sessionError = ""
		return m, nil
	case tea.KeyCtrlR:
		return m.beginRename()
	case tea.KeyCtrlX:
		return m.beginDelete()
	case tea.KeyUp, tea.KeyCtrlP:
		m.moveSessionCursor(-1)
		return m, nil
	case tea.KeyDown, tea.KeyCtrlN:
		m.moveSessionCursor(1)
		return m, nil
	case tea.KeyBackspace:
		m.sessionFilter = removeLastRune(m.sessionFilter)
		m.sessionCursor = min(m.sessionCursor, max(0, len(m.filteredSessions())-1))
		return m, nil
	case tea.KeyEnter, tea.KeyTab:
		return m.chooseSession()
	case tea.KeyCtrlY:
		return m.chooseSession()
	case tea.KeyRunes:
		m.sessionFilter += string(msg.Runes)
		m.sessionCursor = 0
		m.sessionError = ""
		return m, nil
	}
	return m, nil
}

func (m Model) beginRename() (Model, tea.Cmd) {
	rows := m.filteredSessions()
	if len(rows) == 0 {
		return m, nil
	}
	row := rows[m.sessionCursor]
	m.sessionRenaming = true
	m.sessionRenameID = row.ID
	m.sessionRenameInput = row.Title
	m.sessionError = ""
	return m, nil
}

func (m Model) beginDelete() (Model, tea.Cmd) {
	rows := m.filteredSessions()
	if len(rows) == 0 {
		return m, nil
	}
	row := rows[m.sessionCursor]
	if meta := m.driver.Meta(); meta.Busy && row.ID == m.driver.Active().ID {
		m.sessionError = "cannot delete the active session while a run is in progress"
		return m, nil
	}
	m.sessionDeleteID = row.ID
	m.sessionError = ""
	return m, nil
}

func (m Model) submitRename() (Model, tea.Cmd) {
	title := strings.TrimSpace(m.sessionRenameInput)
	if title == "" {
		m.sessionError = "title cannot be empty"
		return m, nil
	}
	controller, ok := m.driver.(surface.SessionController)
	if !ok {
		m.sessionError = "session rename is unavailable"
		return m, nil
	}
	cmd := controller.RenameSession(m.sessionRenameID, title)
	if cmd == nil {
		m.sessionError = "session rename is unavailable"
		return m, nil
	}
	m.sessionActionBusy = true
	m.sessionError = ""
	return m, cmd
}

func (m Model) submitDelete() (Model, tea.Cmd) {
	controller, ok := m.driver.(surface.SessionController)
	if !ok {
		m.sessionError = "session delete is unavailable"
		return m, nil
	}
	cmd := controller.DeleteSession(m.sessionDeleteID)
	if cmd == nil {
		m.sessionError = "session delete is unavailable"
		return m, nil
	}
	m.sessionActionBusy = true
	m.sessionError = ""
	return m, cmd
}

func (m Model) chooseSession() (Model, tea.Cmd) {
	rows := m.filteredSessions()
	if len(rows) == 0 {
		return m, nil
	}
	controller, ok := m.driver.(surface.SessionController)
	if !ok {
		m.sessionError = "session selection is unavailable"
		return m, nil
	}
	cmd := controller.SelectSession(rows[m.sessionCursor].ID)
	if cmd == nil {
		m.sessionError = "session selection is unavailable"
		return m, nil
	}
	m.sessionsOpen = false
	m.sessionError = ""
	return m, cmd
}

func (m *Model) applySessionsMsg(msg surface.SessionsMsg) {
	if !m.sessionsOpen {
		return
	}
	if msg.Request > 0 {
		if msg.Request < m.sessionRequest {
			return
		}
		m.sessionRequest = msg.Request
	}
	if msg.Err != nil {
		m.sessionLoading = false
		m.sessionActionBusy = false
		m.sessionError = shortError(msg.Err)
		return
	}
	switch msg.Action {
	case "list":
		m.sessionLoading = false
		if msg.Sessions != nil {
			m.sessionRows = append([]surface.Session(nil), msg.Sessions...)
		}
		m.syncSessionCursorToActive()
	case "rename":
		m.sessionActionBusy = false
		m.sessionRenaming = false
		m.sessionRenameID = ""
		m.sessionRenameInput = ""
		for i := range m.sessionRows {
			if m.sessionRows[i].ID == msg.ID {
				m.sessionRows[i] = msg.Session
				break
			}
		}
	case "delete":
		m.sessionActionBusy = false
		m.sessionDeleteID = ""
		rows := m.sessionRows[:0]
		for _, row := range m.sessionRows {
			if row.ID != msg.ID {
				rows = append(rows, row)
			}
		}
		m.sessionRows = rows
		m.sessionCursor = min(m.sessionCursor, max(0, len(m.filteredSessions())-1))
	}
	m.sessionError = ""
}

func (m *Model) syncSessionCursorToActive() {
	activeID := m.driver.Active().ID
	rows := m.filteredSessions()
	for i, row := range rows {
		if row.ID == activeID {
			m.sessionCursor = i
			return
		}
	}
	if len(rows) == 0 {
		m.sessionCursor = 0
		return
	}
	m.sessionCursor = min(m.sessionCursor, len(rows)-1)
}

func (m *Model) moveSessionCursor(delta int) {
	rows := m.filteredSessions()
	if len(rows) == 0 {
		m.sessionCursor = 0
		return
	}
	m.sessionCursor = (m.sessionCursor + delta) % len(rows)
	if m.sessionCursor < 0 {
		m.sessionCursor += len(rows)
	}
}

func (m Model) filteredSessions() []surface.Session {
	needle := strings.ToLower(strings.TrimSpace(m.sessionFilter))
	if needle == "" {
		return m.sessionRows
	}
	rows := make([]surface.Session, 0, len(m.sessionRows))
	for _, row := range m.sessionRows {
		if fuzzyContains(strings.ToLower(row.Title), needle) {
			rows = append(rows, row)
		}
	}
	return rows
}

// fuzzyContains implements the ordered-subsequence matching used by the
// Sessions picker. It keeps filtering deterministic and Unicode-safe without
// pulling transport or ranking concerns into the shared view.
func fuzzyContains(haystack, needle string) bool {
	if needle == "" {
		return true
	}
	remaining := []rune(needle)
	for _, r := range []rune(haystack) {
		if len(remaining) > 0 && r == remaining[0] {
			remaining = remaining[1:]
		}
	}
	return len(remaining) == 0
}

func nextPermission(current string) string {
	switch current {
	case "cautious":
		return "smart"
	case "smart":
		return "trusted"
	default:
		return "cautious"
	}
}

func removeLastRune(s string) string {
	runes := []rune(s)
	if len(runes) == 0 {
		return ""
	}
	return string(runes[:len(runes)-1])
}

func shortError(err error) string {
	if err == nil {
		return ""
	}
	s := err.Error()
	if len(s) > 80 {
		return s[:77] + "…"
	}
	return s
}

// Run starts the shared fullscreen shell on stdout.
func Run(driver surface.Driver) error { return RunWithOutput(driver, os.Stdout) }

// RunWithOutput starts the canonical shell on the launcher's output stream.
func RunWithOutput(driver surface.Driver, out io.Writer) error {
	configureColor(out)
	p := tea.NewProgram(New(driver), tea.WithAltScreen(), tea.WithOutput(out))
	_, err := p.Run()
	return err
}

func configureColor(out io.Writer) {
	if os.Getenv("NO_COLOR") != "" {
		return
	}
	f, ok := out.(*os.File)
	if !ok {
		return
	}
	stat, err := f.Stat()
	if err == nil && stat.Mode()&os.ModeCharDevice != 0 {
		lipgloss.SetColorProfile(termenv.TrueColor)
	}
}

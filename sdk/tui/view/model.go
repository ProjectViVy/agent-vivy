// Package view is the shared Crush-style fullscreen shell for first-party
// built-in and packed terminal faces.
package view

import (
	"io"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"agent-vivy/sdk/tui/surface"
)

const (
	approvalApproved = "approved"
	approvalDenied   = "denied"
)

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
	if m.sessionsOpen {
		return m.handleSessionsKey(msg)
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
		text := m.input
		cmd := m.driver.Send(text)
		if cmd == nil {
			return m, nil
		}
		m.input = ""
		return m, cmd
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

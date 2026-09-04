// Package view is the shared Crush-style fullscreen shell for first-party
// built-in and packed terminal faces.
package view

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/rivo/uniseg"

	"agent-vivy/sdk/tui/command"
	"agent-vivy/sdk/tui/surface"
)

const (
	approvalApproved = "approved"
	approvalDenied   = "denied"
)

var commandRegistry = command.DefaultRegistry()

type fileCompletionStartMsg struct {
	Request   uint64
	Query     string
	SessionID string
}

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

	commandOverlayTitle  string
	commandOverlay       string
	commandConfirmName   string
	commandConfirmArgs   []string
	commandConfirmSID    string
	commandPaletteOpen   bool
	commandPaletteFilter string
	commandPaletteCursor int

	fileCompletionOpen       bool
	fileCompletionLoading    bool
	fileCompletionQuery      string
	fileCompletionTokenStart int
	fileCompletionFiles      []surface.FileContext
	fileCompletionCursor     int
	fileCompletionTruncated  bool
	fileCompletionError      string
	fileCompletionRequest    uint64
	fileCompletionSessionID  string

	sidebarFocused bool
	sidebarScroll  int
	chatScroll     int
	chatFollow     bool
	chatSessionID  string
}

// New returns a model bound to the given driver.
func New(driver surface.Driver) Model {
	if driver == nil {
		driver = noDriver{}
	}
	return Model{
		driver:        driver,
		width:         120,
		height:        36,
		palette:       DefaultPalette(),
		chatFollow:    true,
		chatSessionID: driver.Active().ID,
	}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	if m.driver == nil {
		return nil
	}
	return m.driver.Init()
}

// Update implements tea.Model. The driver sees transport messages first so
// its authoritative state is committed before the shared view consumes result
// messages such as SessionsMsg. Mouse input belongs exclusively to this view;
// never leak it through a driver underneath a dialog or gate.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	_, mouseInput := msg.(tea.MouseMsg)
	if m.driver != nil && !mouseInput {
		if cmd := m.driver.Handle(msg); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if m.commandPaletteOpen && m.driver.PendingGate() != nil {
		// Gates arrive asynchronously from the run stream. Close the palette
		// on arrival, not only on the next key, so it cannot reappear after the
		// gate resolves.
		m.closeCommandPalette()
	}
	if m.fileCompletionOpen && m.driver.PendingGate() != nil {
		m.closeFileCompletion()
	}
	if m.fileCompletionOpen && m.fileCompletionSessionID != m.driver.Active().ID {
		m.closeFileCompletion()
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
	case tea.MouseMsg:
		m.handleMouse(msg)
	case surface.SessionsMsg:
		m.applySessionsMsg(msg)
	case surface.GateResolvedMsg:
		if msg.Kind == "question" {
			m.input = ""
		}
	case surface.CommandResultMsg:
		m.closeCommandPalette()
		m.closeFileCompletion()
		m.applyCommandResult(msg)
	case surface.ProjectFilesMsg:
		m.applyProjectFilesMsg(msg)
	case fileCompletionStartMsg:
		if m.fileCompletionOpen && msg.Request == m.fileCompletionRequest && msg.Query == m.fileCompletionQuery && msg.SessionID == m.fileCompletionSessionID {
			if completer, ok := m.driver.(surface.ProjectFileCompleter); ok {
				if cmd := completer.CompleteProjectFiles(msg.Request, msg.Query); cmd != nil {
					cmds = append(cmds, cmd)
				}
			}
		}
	case surface.ErrMsg:
		// The driver stores transport errors in Meta. Keep the dialog snapshot
		// and local input intact so a retry does not discard user work.
	case surface.RestoreInputMsg:
		if strings.TrimSpace(msg.Text) != "" {
			if strings.TrimSpace(m.input) == "" {
				m.input = msg.Text
			} else {
				m.input = msg.Text + " " + m.input
			}
		}
	}
	if m.sidebarFocused && !m.sidebarCanScroll() {
		m.sidebarFocused = false
	}
	m.clampSidebarScroll()
	if activeID := m.driver.Active().ID; activeID != m.chatSessionID {
		m.chatSessionID = activeID
		m.chatScroll = 0
		m.chatFollow = true
	}
	m.clampChatScroll()
	return m, tea.Batch(cmds...)
}

const sidebarWheelStep = 3

func (m *Model) handleMouse(msg tea.MouseMsg) {
	if m.driver.PendingGate() != nil || m.commandPaletteOpen || m.fileCompletionOpen || m.sessionsOpen || m.commandConfirmName != "" || m.commandOverlay != "" {
		return
	}
	if msg.Action != tea.MouseActionPress {
		return
	}
	l := computeLayout(m.width, m.height)
	inSidebar := m.mouseInSidebar(l, msg.X, msg.Y)
	if msg.Button == tea.MouseButtonLeft {
		m.sidebarFocused = inSidebar && m.sidebarCanScroll()
		return
	}
	// Match Crush focus routing: a click chooses the scroll owner, then wheel
	// events stay with that owner even if the pointer drifts outside its box.
	delta := 0
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		delta = -sidebarWheelStep
	case tea.MouseButtonWheelDown:
		delta = sidebarWheelStep
	default:
		// Bubble Tea keeps Type for compatibility with older terminal input
		// decoders; accept it without treating ordinary clicks as scrolling.
		switch msg.Type {
		case tea.MouseWheelUp:
			delta = -sidebarWheelStep
		case tea.MouseWheelDown:
			delta = sidebarWheelStep
		default:
			return
		}
	}
	if m.sidebarFocused && m.sidebarCanScroll() {
		m.sidebarScroll += delta
		m.clampSidebarScroll()
		return
	}
	if m.chatCanScroll() {
		m.scrollChat(delta)
	}
}

func (m Model) mouseInSidebar(l layout, x, y int) bool {
	if !l.showSidebar {
		return false
	}
	left := l.marginX + l.mainW() + 1
	top := l.marginY
	height := m.sidebarViewportHeight(l, m.palette)
	return x >= left && x < left+l.sidebarW && y >= top && y < top+height
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
	if gate != nil && m.commandPaletteOpen {
		m.closeCommandPalette()
	}
	if gate != nil && m.fileCompletionOpen {
		m.closeFileCompletion()
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
	if gate == nil && m.commandPaletteOpen {
		return m.handleCommandPaletteKey(msg)
	}
	if gate == nil && m.fileCompletionOpen {
		return m.handleFileCompletionKey(msg)
	}
	if gate == nil && m.sidebarFocused {
		var handled bool
		m, cmd, handled := m.handleSidebarKey(msg)
		if handled {
			return m, cmd
		}
		// Non-navigation input returns focus to the editor and continues
		// through the normal global/editor routing below.
		m.sidebarFocused = false
	}
	if gate == nil && m.sidebarCanScroll() && msg.Type == tea.KeyCtrlRight {
		m.sidebarFocused = true
		m.clampSidebarScroll()
		return m, nil
	}
	if gate == nil && !m.sidebarFocused {
		switch msg.Type {
		case tea.KeyPgUp:
			m.scrollChat(-m.chatViewportHeight())
			return m, nil
		case tea.KeyPgDown:
			m.scrollChat(m.chatViewportHeight())
			return m, nil
		case tea.KeyHome:
			m.chatScroll = 0
			m.chatFollow = m.chatMaxScroll() == 0
			return m, nil
		case tea.KeyEnd:
			m.chatFollow = true
			m.clampChatScroll()
			return m, nil
		}
	}
	if gate != nil && gate.Submitting && msg.Type != tea.KeyCtrlC && msg.Type != tea.KeyEsc {
		return m, nil
	}
	switch msg.Type {
	case tea.KeyCtrlC:
		return m, tea.Quit
	case tea.KeyCtrlS:
		return m.openSessions()
	case tea.KeyCtrlP:
		if gate == nil {
			m.openCommandPalette()
		}
		return m, nil
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
		m.closeFileCompletion()
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
		return m.refreshFileCompletion()
	case tea.KeySpace:
		if gate != nil && gate.Kind == "approval" {
			return m, nil
		}
		m.input += " "
		m.closeFileCompletion()
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
		if text == "/" && m.input == "" && gate == nil {
			m.openCommandPalette()
			return m, nil
		}
		m.input += text
		return m.refreshFileCompletion()
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

func (m Model) handleSidebarKey(msg tea.KeyMsg) (Model, tea.Cmd, bool) {
	switch msg.Type {
	case tea.KeyCtrlC:
		return m, tea.Quit, true
	case tea.KeyEsc, tea.KeyTab, tea.KeyShiftTab, tea.KeyLeft, tea.KeyCtrlLeft:
		m.sidebarFocused = false
		return m, nil, true
	case tea.KeyUp:
		m.sidebarScroll--
		m.clampSidebarScroll()
		return m, nil, true
	case tea.KeyDown:
		m.sidebarScroll++
		m.clampSidebarScroll()
		return m, nil, true
	case tea.KeyHome:
		m.sidebarScroll = 0
		return m, nil, true
	case tea.KeyEnd:
		m.sidebarScroll = m.sidebarMaxScroll()
		return m, nil, true
	case tea.KeyRunes:
		switch string(msg.Runes) {
		case "h":
			m.sidebarFocused = false
			return m, nil, true
		case "j":
			m.sidebarScroll++
			m.clampSidebarScroll()
			return m, nil, true
		case "k":
			m.sidebarScroll--
			m.clampSidebarScroll()
			return m, nil, true
		}
	}
	return m, nil, false
}

func (m Model) sidebarViewportHeight(l layout, p Palette) int {
	chat := m.renderChat(l.mainW(), l.mainH(), p)
	editor := m.renderEditor(l.mainW(), p)
	return max(1, lipgloss.Height(lipgloss.JoinVertical(lipgloss.Left, chat, "", editor)))
}

func (m Model) sidebarMaxScroll() int {
	l := computeLayout(m.width, m.height)
	if !l.showSidebar {
		return 0
	}
	p := m.palette
	lines := m.sidebarLines(l.sidebarW, p)
	viewport := max(1, m.sidebarViewportHeight(l, p)-len(sidebarLogoLines(p)))
	return max(0, len(lines)-viewport)
}

func (m Model) sidebarCanScroll() bool {
	return m.sidebarMaxScroll() > 0
}

func (m *Model) clampSidebarScroll() {
	maxScroll := m.sidebarMaxScroll()
	if m.sidebarScroll < 0 {
		m.sidebarScroll = 0
	}
	if m.sidebarScroll > maxScroll {
		m.sidebarScroll = maxScroll
	}
}

func (m Model) chatViewportHeight() int {
	return computeLayout(m.width, m.height).mainH()
}

func (m Model) chatMaxScroll() int {
	l := computeLayout(m.width, m.height)
	width := l.innerW()
	if l.showSidebar {
		width = l.mainW()
	}
	return max(0, len(m.chatLines(width, m.palette))-l.mainH())
}

func (m Model) chatCanScroll() bool {
	return m.chatMaxScroll() > 0
}

func (m *Model) scrollChat(delta int) {
	if delta == 0 {
		return
	}
	m.chatScroll += delta
	m.chatFollow = false
	m.clampChatScroll()
}

func (m *Model) clampChatScroll() {
	maxScroll := m.chatMaxScroll()
	if m.chatFollow {
		m.chatScroll = maxScroll
		return
	}
	if m.chatScroll < 0 {
		m.chatScroll = 0
	}
	if m.chatScroll >= maxScroll {
		m.chatScroll = maxScroll
		m.chatFollow = true
	}
}

func (m Model) refreshFileCompletion() (Model, tea.Cmd) {
	query, start, ok := activeFileCompletionToken(m.input)
	if !ok {
		m.closeFileCompletion()
		return m, nil
	}
	capabilities, capable := m.driver.(surface.CapabilityReporter)
	completer, completable := m.driver.(surface.ProjectFileCompleter)
	if !capable || !capabilities.SupportsCapability("project-context.list") || !completable {
		m.closeFileCompletion()
		return m, nil
	}
	m.fileCompletionOpen = true
	m.fileCompletionLoading = true
	m.fileCompletionQuery = query
	m.fileCompletionTokenStart = start
	m.fileCompletionFiles = nil
	m.fileCompletionCursor = 0
	m.fileCompletionTruncated = false
	m.fileCompletionError = ""
	m.fileCompletionRequest++
	m.fileCompletionSessionID = m.driver.Active().ID
	request := m.fileCompletionRequest
	sessionID := m.fileCompletionSessionID
	_ = completer // capability/interface presence is checked before scheduling.
	return m, tea.Tick(120*time.Millisecond, func(time.Time) tea.Msg {
		return fileCompletionStartMsg{Request: request, Query: query, SessionID: sessionID}
	})
}

func activeFileCompletionToken(input string) (string, int, bool) {
	runes := []rune(input)
	if len(runes) == 0 {
		return "", 0, false
	}
	start := len(runes) - 1
	for start >= 0 && !unicode.IsSpace(runes[start]) {
		start--
	}
	start++
	if start >= len(runes) || runes[start] != '@' || (start+1 < len(runes) && runes[start+1] == '@') {
		return "", 0, false
	}
	query := sanitizeFileCompletionText(string(runes[start+1:]))
	if query != string(runes[start+1:]) || strings.ContainsAny(query, "\"'") {
		return "", 0, false
	}
	return query, start, true
}

func (m *Model) closeFileCompletion() {
	m.fileCompletionOpen = false
	m.fileCompletionLoading = false
	m.fileCompletionQuery = ""
	m.fileCompletionFiles = nil
	m.fileCompletionCursor = 0
	m.fileCompletionTruncated = false
	m.fileCompletionError = ""
	m.fileCompletionSessionID = ""
	m.fileCompletionRequest++
}

func (m *Model) applyProjectFilesMsg(msg surface.ProjectFilesMsg) {
	if !m.fileCompletionOpen || msg.Request != m.fileCompletionRequest || msg.Query != m.fileCompletionQuery {
		return
	}
	m.fileCompletionLoading = false
	m.fileCompletionFiles = append([]surface.FileContext(nil), msg.Files...)
	m.fileCompletionTruncated = msg.Truncated
	m.fileCompletionCursor = 0
	if msg.Err != nil {
		m.fileCompletionError = shortCompletionError(msg.Err)
		m.fileCompletionFiles = nil
	} else {
		m.fileCompletionError = ""
	}
}

func (m Model) handleFileCompletionKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		return m, tea.Quit
	case tea.KeyEsc:
		m.closeFileCompletion()
		return m, nil
	case tea.KeyUp, tea.KeyCtrlP:
		m.moveFileCompletionCursor(-1)
		return m, nil
	case tea.KeyDown, tea.KeyCtrlN:
		m.moveFileCompletionCursor(1)
		return m, nil
	case tea.KeyEnter, tea.KeyTab:
		rows := m.filteredProjectFiles()
		if len(rows) == 0 {
			return m, nil
		}
		cursor := min(max(0, m.fileCompletionCursor), len(rows)-1)
		path := safeProjectFilePath(rows[cursor].Path)
		if path == "" || rows[cursor].Size < 0 {
			return m, nil
		}
		runes := []rune(m.input)
		start := min(max(0, m.fileCompletionTokenStart), len(runes))
		m.input = string(runes[:start]) + command.FormatFileReference(path) + " "
		m.closeFileCompletion()
		return m, nil
	case tea.KeyBackspace:
		m.input = removeLastRune(m.input)
		return m.refreshFileCompletion()
	case tea.KeySpace:
		m.input += " "
		m.closeFileCompletion()
		return m, nil
	case tea.KeyRunes:
		m.input += string(msg.Runes)
		return m.refreshFileCompletion()
	}
	return m, nil
}

func (m *Model) moveFileCompletionCursor(delta int) {
	rows := m.filteredProjectFiles()
	if len(rows) == 0 {
		m.fileCompletionCursor = 0
		return
	}
	m.fileCompletionCursor = (m.fileCompletionCursor + delta) % len(rows)
	if m.fileCompletionCursor < 0 {
		m.fileCompletionCursor += len(rows)
	}
}

func (m Model) filteredProjectFiles() []surface.FileContext {
	needle := strings.ToLower(strings.ReplaceAll(sanitizeFileCompletionText(m.fileCompletionQuery), "\\", "/"))
	rows := make([]surface.FileContext, 0, min(len(m.fileCompletionFiles), 200))
	seen := make(map[string]struct{}, min(len(m.fileCompletionFiles), 200))
	for _, file := range m.fileCompletionFiles {
		path := safeProjectFilePath(file.Path)
		if path == "" || file.Size < 0 {
			continue
		}
		if _, duplicate := seen[path]; duplicate {
			continue
		}
		if needle == "" || fuzzyContains(strings.ToLower(strings.ReplaceAll(path, "\\", "/")), needle) {
			file.Path = path
			file.Name = sanitizeFileCompletionText(file.Name)
			rows = append(rows, file)
			seen[path] = struct{}{}
			if len(rows) == 200 {
				break
			}
		}
	}
	return rows
}

func (m *Model) openCommandPalette() {
	m.closeFileCompletion()
	m.commandPaletteOpen = true
	m.commandPaletteFilter = ""
	m.commandPaletteCursor = 0
}

func (m *Model) closeCommandPalette() {
	m.commandPaletteOpen = false
	m.commandPaletteFilter = ""
	m.commandPaletteCursor = 0
}

func (m Model) handleCommandPaletteKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		return m, tea.Quit
	case tea.KeyEsc:
		m.closeCommandPalette()
		return m, nil
	case tea.KeyUp, tea.KeyCtrlP:
		m.moveCommandPaletteCursor(-1)
		return m, nil
	case tea.KeyDown, tea.KeyCtrlN:
		m.moveCommandPaletteCursor(1)
		return m, nil
	case tea.KeyBackspace:
		m.commandPaletteFilter = removeLastRune(m.commandPaletteFilter)
		m.commandPaletteCursor = 0
		return m, nil
	case tea.KeySpace:
		m.commandPaletteFilter = sanitizeCommandPaletteFilter(m.commandPaletteFilter + " ")
		m.commandPaletteCursor = 0
		return m, nil
	case tea.KeyEnter, tea.KeyCtrlY:
		rows := m.filteredCommands()
		if len(rows) == 0 {
			return m, nil
		}
		cursor := min(max(0, m.commandPaletteCursor), len(rows)-1)
		m.input = "/" + rows[cursor].Name
		m.closeCommandPalette()
		return m, nil
	case tea.KeyRunes:
		text := string(msg.Runes)
		if text == "/" && m.commandPaletteFilter == "" && m.input == "" {
			// Preserve the parser's //literal contract even though the first
			// slash opens the palette instead of entering the editor.
			m.input = "//"
			m.closeCommandPalette()
			return m, nil
		}
		m.commandPaletteFilter = sanitizeCommandPaletteFilter(m.commandPaletteFilter + text)
		m.commandPaletteCursor = 0
		return m, nil
	}
	return m, nil
}

func (m *Model) moveCommandPaletteCursor(delta int) {
	rows := m.filteredCommands()
	if len(rows) == 0 {
		m.commandPaletteCursor = 0
		return
	}
	m.commandPaletteCursor = (m.commandPaletteCursor + delta) % len(rows)
	if m.commandPaletteCursor < 0 {
		m.commandPaletteCursor += len(rows)
	}
}

func (m Model) filteredCommands() []command.Spec {
	needle := strings.ToLower(strings.TrimSpace(sanitizeCommandPaletteFilter(m.commandPaletteFilter)))
	rows := commandRegistry.Specs()
	if needle == "" {
		return rows
	}
	filtered := make([]command.Spec, 0, len(rows))
	for _, spec := range rows {
		search := strings.ToLower(strings.Join([]string{
			spec.Name,
			strings.Join(spec.Aliases, " "),
			spec.Usage,
			spec.Description,
		}, " "))
		if fuzzyContains(search, needle) {
			filtered = append(filtered, spec)
		}
	}
	return filtered
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
	if parsed.IsShell() {
		capabilities, capable := m.driver.(surface.CapabilityReporter)
		if !capable || !capabilities.SupportsCapability("shell.start") {
			return m.showCommandError(fmt.Errorf("! shell commands are unavailable")), nil
		}
		executor, ok := m.driver.(surface.ShellExecutor)
		if !ok {
			// A shell-capable input is never sent as model text. Small/demo
			// drivers that do not expose the governed control-plane seam fail
			// closed in the shared view.
			return m.showCommandError(fmt.Errorf("! shell commands are unavailable")), nil
		}
		if cmd := executor.ExecuteShell(parsed.Shell.Script); cmd != nil {
			m.input = ""
			m.chatFollow = true
			return m, cmd
		}
		return m.showCommandError(fmt.Errorf("! shell commands are unavailable")), nil
	}
	if parsed.IsFile() {
		if strings.TrimSpace(parsed.Text) == "" {
			return m.showCommandError(fmt.Errorf("@file references require a prompt")), nil
		}
		sender, ok := m.driver.(surface.ContextSender)
		if !ok {
			// The path list is an untrusted hint. It must reach a live driver
			// (and then the server resolver) before the turn is accepted; the
			// shared view has no filesystem authority of its own.
			return m.showCommandError(fmt.Errorf("@file references are unavailable")), nil
		}
		if cmd := sender.SendWithContext(parsed.Text, parsed.FilePaths()); cmd != nil {
			m.input = ""
			m.chatFollow = true
			return m, cmd
		}
		return m.showCommandError(fmt.Errorf("@file references are unavailable")), nil
	}
	if !parsed.IsCommand() {
		cmd := m.driver.Send(parsed.Text)
		if cmd != nil {
			m.input = ""
			m.chatFollow = true
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
		shellSupported := false
		if capabilities, ok := m.driver.(surface.CapabilityReporter); ok {
			shellSupported = capabilities.SupportsCapability("shell.start")
		}
		m.commandOverlay = commandRegistry.HelpFor(shellSupported)
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
	case "image":
		return m.executeImageCommand(args)
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

func (m Model) executeImageCommand(args []string) (Model, tea.Cmd) {
	if executor, ok := m.driver.(surface.CommandExecutor); ok {
		if cmd := executor.ExecuteCommand("image", append([]string(nil), args...)); cmd != nil {
			return m, cmd
		}
		return m.showCommandError(fmt.Errorf("/image is unavailable")), nil
	}
	return m.showCommandError(fmt.Errorf("/image is unavailable")), nil
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
	m.closeFileCompletion()
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
	graphemes := uniseg.NewGraphemes(s)
	for graphemes.Next() {
		start, end := graphemes.Positions()
		if end == len(s) {
			return s[:start]
		}
	}
	return s
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
	p := tea.NewProgram(New(driver), tea.WithAltScreen(), tea.WithMouseCellMotion(), tea.WithOutput(out))
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

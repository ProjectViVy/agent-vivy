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
	driver          surface.Driver
	width           int
	height          int
	input           string
	palette         Palette
	debugToolOutput bool

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

	commandOverlayTitle    string
	commandOverlay         string
	commandConfirmName     string
	commandConfirmArgs     []string
	commandConfirmSID      string
	commandPaletteOpen     bool
	commandPaletteFilter   string
	commandPaletteCursor   int
	commandCatalogRequest  uint64
	commandCatalogLoading  bool
	commandCatalogError    string
	dynamicCommandRequest  uint64
	dynamicCommandPending  bool
	dynamicCommandID       string
	dynamicCommandSession  string
	dynamicCommandDraft    string
	dynamicArgumentCommand *surface.DynamicCommand
	dynamicArgumentValues  []string
	dynamicArgumentCursor  int
	dynamicArgumentError   string

	modelPickerOpen      bool
	modelPickerLoading   bool
	modelPickerSelecting bool
	modelPickerFilter    string
	modelPickerCursor    int
	modelPickerError     string
	modelPickerRequest   uint64

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
	mdCache        *messageMarkdownCache
	shortcutsOpen  bool

	gateID           string
	gateScroll       int
	gateHorizontal   int
	gateUnified      bool
	gateViewExplicit bool
	gateFullscreen   bool
}

// Options controls presentation-only behavior of the shared terminal view.
type Options struct {
	DebugToolOutput bool
}

// New returns a model bound to the given driver.
func New(driver surface.Driver, options ...Options) Model {
	if driver == nil {
		panic("tui view: nil driver")
	}
	var opts Options
	if len(options) > 0 {
		opts = options[0]
	}
	return Model{
		driver:          driver,
		width:           120,
		height:          36,
		palette:         DefaultPalette(),
		debugToolOutput: opts.DebugToolOutput,
		chatFollow:      true,
		chatSessionID:   driver.Active().ID,
		mdCache:         newMessageMarkdownCache(),
	}
}

type messageMarkdownKey struct {
	id        string
	role      string
	content   string
	reasoning bool
	width     int
}

type messageMarkdownCache struct {
	sessionID string
	width     int
	lines     map[messageMarkdownKey][]string
}

func newMessageMarkdownCache() *messageMarkdownCache {
	return &messageMarkdownCache{lines: map[messageMarkdownKey][]string{}}
}

func cacheableMarkdownMessage(message surface.Message) bool {
	return !message.Streaming && message.Tool == nil && len(message.Attachments) == 0 && len(message.FileContexts) == 0
}

func (c *messageMarkdownCache) ensure(sessionID string, width int) {
	if c == nil {
		return
	}
	if c.lines == nil || c.sessionID != sessionID || c.width != width {
		c.sessionID = sessionID
		c.width = width
		c.lines = map[messageMarkdownKey][]string{}
	}
}

func (c *messageMarkdownCache) get(message surface.Message, width int) ([]string, bool) {
	if c == nil || c.lines == nil || !cacheableMarkdownMessage(message) {
		return nil, false
	}
	lines, ok := c.lines[messageMarkdownKey{
		id: message.ID, role: message.Role, content: message.Content,
		reasoning: message.Reasoning, width: width,
	}]
	if !ok {
		return nil, false
	}
	out := make([]string, len(lines))
	copy(out, lines)
	return out, true
}

func (c *messageMarkdownCache) put(message surface.Message, width int, lines []string) {
	if c == nil || c.lines == nil || !cacheableMarkdownMessage(message) {
		return
	}
	stored := make([]string, len(lines))
	copy(stored, lines)
	c.lines[messageMarkdownKey{
		id: message.ID, role: message.Role, content: message.Content,
		reasoning: message.Reasoning, width: width,
	}] = stored
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
	if m.modelPickerOpen && m.driver.PendingGate() != nil {
		m.closeModelPicker()
	}
	if m.shortcutsOpen && m.driver.PendingGate() != nil {
		m.shortcutsOpen = false
	}
	m.syncGateView()
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
	case surface.DynamicCommandsMsg:
		if msg.Request == m.commandCatalogRequest {
			m.commandCatalogLoading = false
			if msg.Err != nil {
				m.commandCatalogError = sanitizeCommandPaletteFilter(msg.Err.Error())
			} else {
				m.commandCatalogError = ""
				m.commandPaletteCursor = 0
			}
		}
	case surface.DynamicCommandExpandedMsg:
		if msg.Request != m.dynamicCommandRequest || !m.dynamicCommandPending || msg.ID != m.dynamicCommandID || msg.SessionID != m.dynamicCommandSession {
			break
		}
		retryDraft := m.dynamicCommandDraft
		m.dynamicCommandPending = false
		m.dynamicCommandID = ""
		m.dynamicCommandSession = ""
		m.dynamicCommandDraft = ""
		if msg.SessionID == "" || msg.SessionID != m.driver.Active().ID {
			m.input = retryDraft
			m = m.showCommandError(fmt.Errorf("dynamic command was discarded after the active session changed"))
			break
		}
		m.closeCommandPalette()
		if msg.Err != nil {
			m.input = retryDraft
			m = m.showCommandError(msg.Err)
		} else if strings.TrimSpace(msg.Text) == "" {
			m.input = retryDraft
			m = m.showCommandError(fmt.Errorf("dynamic command expanded to empty input"))
		} else if cmd := m.driver.Send(msg.Text); cmd != nil {
			m.chatFollow = true
			cmds = append(cmds, cmd)
		} else {
			m = m.showCommandError(fmt.Errorf("dynamic command could not start a turn"))
		}
	case surface.ProjectFilesMsg:
		m.applyProjectFilesMsg(msg)
	case surface.ModelsMsg:
		m.applyModelsMsg(msg)
	case surface.ModelSelectedMsg:
		m.applyModelSelectedMsg(msg)
	case fileCompletionStartMsg:
		if m.fileCompletionOpen && msg.Request == m.fileCompletionRequest && msg.Query == m.fileCompletionQuery && msg.SessionID == m.fileCompletionSessionID {
			if cmd := m.driver.CompleteProjectFiles(msg.Request, msg.Query); cmd != nil {
				cmds = append(cmds, cmd)
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
	m.clampGateScroll()
	return m, tea.Batch(cmds...)
}

const sidebarWheelStep = 3

func (m *Model) handleMouse(msg tea.MouseMsg) {
	if gate := m.driver.PendingGate(); gate != nil {
		if gate.Kind == "approval" && msg.Action == tea.MouseActionPress {
			delta := 0
			switch msg.Button {
			case tea.MouseButtonWheelUp:
				delta = -sidebarWheelStep
			case tea.MouseButtonWheelDown:
				delta = sidebarWheelStep
			default:
				switch msg.Type {
				case tea.MouseWheelUp:
					delta = -sidebarWheelStep
				case tea.MouseWheelDown:
					delta = sidebarWheelStep
				}
			}
			m.gateScroll += delta
			m.clampGateScroll()
		}
		return
	}
	if m.modelPickerOpen || m.commandPaletteOpen || m.fileCompletionOpen || m.sessionsOpen || m.commandConfirmName != "" || m.commandOverlay != "" || m.shortcutsOpen {
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
	// Match Crush pointer-region routing: wheel input belongs to the pane under
	// the pointer. Click/keyboard focus remains independent so hovering the
	// sidebar does not steal editor input or arrow-key ownership.
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
	if inSidebar {
		if m.sidebarCanScroll() {
			m.sidebarScroll += delta
			m.clampSidebarScroll()
		}
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
	if gate != nil && m.dynamicArgumentCommand != nil {
		m.clearDynamicArguments()
	}
	if gate != nil && m.fileCompletionOpen {
		m.closeFileCompletion()
	}
	if gate != nil && m.modelPickerOpen {
		m.closeModelPicker()
	}
	if gate != nil && m.shortcutsOpen {
		m.shortcutsOpen = false
	}
	if gate == nil && m.dynamicCommandPending && msg.Type == tea.KeyEsc {
		m.dynamicCommandRequest++
		m.dynamicCommandPending = false
		m.input = m.dynamicCommandDraft
		m.dynamicCommandID = ""
		m.dynamicCommandSession = ""
		m.dynamicCommandDraft = ""
		m.commandOverlayTitle = ""
		m.commandOverlay = ""
		m.closeCommandPalette()
		return m, nil
	}
	if m.sessionsOpen {
		return m.handleSessionsKey(msg)
	}
	if gate == nil && m.shortcutsOpen {
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit
		case tea.KeyEsc, tea.KeyCtrlX:
			m.shortcutsOpen = false
			return m, nil
		}
		if m.isHelpKey(msg) {
			m.shortcutsOpen = false
			return m, m.openCommandPalette()
		}
		m.shortcutsOpen = false
	}
	if gate == nil && m.modelPickerOpen {
		return m.handleModelPickerKey(msg)
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
	if gate == nil && m.dynamicArgumentCommand != nil {
		return m.handleDynamicArgumentKey(msg)
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
	if gate != nil && gate.Submitting && msg.Type != tea.KeyCtrlC {
		return m, nil
	}
	if gate != nil && gate.Kind == "approval" && !gate.Submitting {
		if next, handled := m.handleApprovalViewKey(msg); handled {
			return next, nil
		}
	}
	switch msg.Type {
	case tea.KeyCtrlC:
		return m, tea.Quit
	case tea.KeyCtrlS:
		m.shortcutsOpen = false
		return m.openSessions()
	case tea.KeyCtrlX:
		if gate != nil {
			return m, nil
		}
		m.shortcutsOpen = !m.shortcutsOpen
		if m.shortcutsOpen {
			m.closeCommandPalette()
			m.closeFileCompletion()
			m.closeModelPicker()
		}
		return m, nil
	case tea.KeyCtrlP:
		if gate == nil {
			m.shortcutsOpen = false
			return m, m.openCommandPalette()
		}
		return m, nil
	}
	if gate == nil && m.input == "" && m.isHelpKey(msg) {
		m.shortcutsOpen = false
		return m, m.openCommandPalette()
	}
	switch msg.Type {
	case tea.KeyCtrlL:
		if gate == nil {
			return m.openModelPicker("")
		}
		return m, nil
	case tea.KeyEsc:
		if gate != nil {
			if gate.Kind == "approval" && !gate.Submitting {
				return m, m.driver.DecideApproval(approvalDenied)
			}
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
		if gate != nil && gate.Kind == "approval" && !gate.Submitting {
			return m, m.driver.DecideApproval(approvalApproved)
		}
		if gate == nil && !meta.Busy {
			return m, m.driver.SetPermission(nextPermission(m.driver.Active().PermissionPreset))
		}
		return m, nil
	case tea.KeyCtrlT:
		if gate != nil {
			return m, nil
		}
		return m.setThinking("")
	case tea.KeyTab, tea.KeyUp, tea.KeyDown:
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
			return m, m.driver.DecideApproval(approvalApproved)
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
			return m, m.openCommandPalette()
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

func (m *Model) syncGateView() {
	gate := m.driver.PendingGate()
	if gate == nil {
		m.gateID = ""
		m.gateScroll = 0
		m.gateHorizontal = 0
		m.gateUnified = false
		m.gateViewExplicit = false
		m.gateFullscreen = false
		return
	}
	if gate.ID != m.gateID {
		m.gateID = gate.ID
		m.gateScroll = 0
		m.gateHorizontal = 0
		m.gateUnified = false
		m.gateViewExplicit = false
		m.gateFullscreen = false
	}
}

func (m Model) handleApprovalViewKey(msg tea.KeyMsg) (Model, bool) {
	gate := m.driver.PendingGate()
	if gate == nil || !isApprovalDiff(gate) {
		return m, false
	}
	page := max(1, m.gateViewportHeight(gate, computeLayout(m.width, m.height))-1)
	switch msg.Type {
	case tea.KeyUp:
		m.gateScroll--
	case tea.KeyDown:
		m.gateScroll++
	case tea.KeyPgUp:
		m.gateScroll -= page
	case tea.KeyPgDown:
		m.gateScroll += page
	case tea.KeyHome:
		m.gateScroll = 0
	case tea.KeyEnd:
		m.gateScroll = m.gateMaxScroll()
	default:
		raw := msg.String()
		switch raw {
		case "H":
			m.gateHorizontal = max(0, m.gateHorizontal-4)
		case "L":
			m.gateHorizontal += 4
		case "K":
			m.gateScroll--
		case "J":
			m.gateScroll++
		default:
			switch strings.ToLower(raw) {
			case "t":
				m.gateUnified = m.gateUsesSplit(gate, computeLayout(m.width, m.height))
				m.gateViewExplicit = true
			case "f":
				m.gateFullscreen = !m.gateFullscreen
			case "shift+up", "shift+k":
				m.gateScroll--
			case "shift+down", "shift+j":
				m.gateScroll++
			case "shift+left", "shift+h":
				m.gateHorizontal = max(0, m.gateHorizontal-4)
			case "shift+right", "shift+l":
				m.gateHorizontal += 4
			default:
				return m, false
			}
		}
	}
	m.clampGateScroll()
	return m, true
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
	chrome := m.renderInputChrome(l.mainW(), p)
	return max(1, lipgloss.Height(lipgloss.JoinVertical(lipgloss.Left, chat, "", editor, chrome)))
}

func (m Model) isHelpKey(msg tea.KeyMsg) bool {
	if strings.EqualFold(msg.String(), "shift+h") {
		return true
	}
	return msg.Type == tea.KeyRunes && string(msg.Runes) == "H"
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
	if !m.driver.SupportsCapability("project-context.list") {
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

func (m *Model) openCommandPalette() tea.Cmd {
	m.closeFileCompletion()
	m.closeModelPicker()
	m.shortcutsOpen = false
	m.commandPaletteOpen = true
	m.commandPaletteFilter = ""
	m.commandPaletteCursor = 0
	m.commandCatalogError = ""
	if !m.driver.SupportsCapability("commands.list") || !m.driver.SupportsCapability("commands.expand") {
		m.commandCatalogLoading = false
		return nil
	}
	m.commandCatalogRequest++
	m.commandCatalogLoading = true
	return m.driver.RefreshDynamicCommands(m.commandCatalogRequest)
}

func (m Model) modelSelectionAvailable() bool {
	return m.driver.SupportsModelSelection()
}

func (m Model) openModelPicker(filter string) (Model, tea.Cmd) {
	if !m.modelSelectionAvailable() {
		return m.showCommandError(fmt.Errorf("model selection is unavailable")), nil
	}
	meta := m.driver.Meta()
	if m.driver.PendingGate() != nil {
		return m.showCommandError(fmt.Errorf("a pending gate must be answered first")), nil
	}
	if meta.Busy || meta.Queued > 0 {
		return m.showCommandError(fmt.Errorf("finish or cancel active and queued work before changing models")), nil
	}
	m.closeCommandPalette()
	m.closeFileCompletion()
	m.shortcutsOpen = false
	m.sessionsOpen = false
	m.commandOverlayTitle = ""
	m.commandOverlay = ""
	m.modelPickerOpen = true
	m.modelPickerLoading = true
	m.modelPickerSelecting = false
	m.modelPickerFilter = sanitizeCommandPaletteFilter(filter)
	m.modelPickerCursor = 0
	m.modelPickerError = ""
	m.modelPickerRequest++
	request := m.modelPickerRequest
	if cmd := m.driver.RefreshModels(request); cmd != nil {
		return m, cmd
	}
	m.modelPickerLoading = false
	m.modelPickerError = "model catalog is unavailable"
	return m, nil
}

func (m *Model) closeModelPicker() {
	m.modelPickerOpen = false
	m.modelPickerLoading = false
	m.modelPickerSelecting = false
	m.modelPickerFilter = ""
	m.modelPickerCursor = 0
	m.modelPickerError = ""
	m.modelPickerRequest++
}

func (m *Model) applyModelsMsg(msg surface.ModelsMsg) {
	if !m.modelPickerOpen || msg.Request != m.modelPickerRequest {
		return
	}
	m.modelPickerLoading = false
	m.modelPickerCursor = 0
	if msg.Err != nil {
		m.modelPickerError = msg.Err.Error()
		return
	}
	m.modelPickerError = ""
	rows := m.filteredModels()
	for i, row := range rows {
		if row.Current {
			m.modelPickerCursor = i
			break
		}
	}
}

func (m *Model) applyModelSelectedMsg(msg surface.ModelSelectedMsg) {
	if !m.modelPickerOpen || msg.Request != m.modelPickerRequest {
		return
	}
	m.modelPickerSelecting = false
	if msg.Err != nil {
		m.modelPickerError = msg.Err.Error()
		return
	}
	m.closeModelPicker()
}

func (m Model) handleModelPickerKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	if msg.Type == tea.KeyCtrlC {
		return m, tea.Quit
	}
	if m.modelPickerSelecting {
		// Selection is a global, server-confirmed transition. Keep the modal
		// until its result arrives so Esc cannot expose the editor and race a
		// new turn against an in-flight model change.
		return m, nil
	}
	if msg.Type == tea.KeyEsc {
		m.closeModelPicker()
		return m, nil
	}
	switch msg.Type {
	case tea.KeyUp, tea.KeyCtrlP:
		m.moveModelPickerCursor(-1)
	case tea.KeyDown, tea.KeyCtrlN:
		m.moveModelPickerCursor(1)
	case tea.KeyBackspace:
		m.modelPickerFilter = removeLastRune(m.modelPickerFilter)
		m.modelPickerCursor = 0
	case tea.KeySpace:
		m.modelPickerFilter = sanitizeCommandPaletteFilter(m.modelPickerFilter + " ")
		m.modelPickerCursor = 0
	case tea.KeyEnter, tea.KeyCtrlY:
		rows := m.filteredModels()
		if m.modelPickerLoading || len(rows) == 0 {
			return m, nil
		}
		catalog := m.driver.ModelCatalog()
		if catalog.ReadOnly || catalog.Frozen {
			m.modelPickerError = "model selection is read-only in this deployment"
			return m, nil
		}
		cursor := min(max(0, m.modelPickerCursor), len(rows)-1)
		if rows[cursor].Current {
			m.closeModelPicker()
			return m, nil
		}
		m.modelPickerRequest++
		request := m.modelPickerRequest
		m.modelPickerSelecting = true
		m.modelPickerError = ""
		if cmd := m.driver.SelectModel(request, rows[cursor]); cmd != nil {
			return m, cmd
		}
		m.modelPickerSelecting = false
		m.modelPickerError = "model selection is unavailable"
	case tea.KeyRunes:
		m.modelPickerFilter = sanitizeCommandPaletteFilter(m.modelPickerFilter + string(msg.Runes))
		m.modelPickerCursor = 0
	}
	return m, nil
}

func (m *Model) moveModelPickerCursor(delta int) {
	rows := m.filteredModels()
	if len(rows) == 0 {
		m.modelPickerCursor = 0
		return
	}
	m.modelPickerCursor = (m.modelPickerCursor + delta) % len(rows)
	if m.modelPickerCursor < 0 {
		m.modelPickerCursor += len(rows)
	}
}

func (m Model) filteredModels() []surface.ModelOption {
	needle := strings.ToLower(strings.TrimSpace(m.modelPickerFilter))
	options := m.driver.ModelCatalog().Options
	rows := make([]surface.ModelOption, 0, len(options))
	for _, option := range options {
		haystack := strings.ToLower(safeModelLabel(option.DisplayName) + " " + safeModelLabel(option.Provider) + " " + safeModelLabel(option.Model))
		if needle == "" || fuzzyContains(haystack, needle) {
			rows = append(rows, option)
		}
	}
	return rows
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
		name := rows[cursor].Name
		_, dynamic := m.effectiveCommandRegistry()
		if entry, ok := dynamic[name]; ok && len(entry.Arguments) > 0 {
			copyEntry := entry
			copyEntry.Arguments = append([]surface.DynamicCommandArgument(nil), entry.Arguments...)
			m.dynamicArgumentCommand = &copyEntry
			m.dynamicArgumentValues = make([]string, len(copyEntry.Arguments))
			m.dynamicArgumentCursor = 0
			m.dynamicArgumentError = ""
			m.closeCommandPalette()
			return m, nil
		}
		m.input = "/" + name
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

func (m *Model) clearDynamicArguments() {
	m.dynamicArgumentCommand = nil
	m.dynamicArgumentValues = nil
	m.dynamicArgumentCursor = 0
	m.dynamicArgumentError = ""
}

func (m Model) handleDynamicArgumentKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	if m.dynamicArgumentCommand == nil || len(m.dynamicArgumentCommand.Arguments) == 0 {
		m.clearDynamicArguments()
		return m, nil
	}
	if msg.Type == tea.KeyCtrlC {
		return m, tea.Quit
	}
	if msg.Type == tea.KeyEsc {
		m.input = "/" + m.dynamicArgumentCommand.Name
		m.clearDynamicArguments()
		return m, nil
	}
	move := func(delta int) {
		m.dynamicArgumentCursor = (m.dynamicArgumentCursor + delta) % len(m.dynamicArgumentValues)
		if m.dynamicArgumentCursor < 0 {
			m.dynamicArgumentCursor += len(m.dynamicArgumentValues)
		}
		m.dynamicArgumentError = ""
	}
	switch msg.Type {
	case tea.KeyUp, tea.KeyShiftTab:
		move(-1)
	case tea.KeyDown, tea.KeyTab:
		move(1)
	case tea.KeyBackspace:
		m.dynamicArgumentValues[m.dynamicArgumentCursor] = removeLastRune(m.dynamicArgumentValues[m.dynamicArgumentCursor])
		m.dynamicArgumentError = ""
	case tea.KeySpace:
		m.dynamicArgumentValues[m.dynamicArgumentCursor] += " "
	case tea.KeyRunes:
		m.dynamicArgumentValues[m.dynamicArgumentCursor] += sanitizeCommandPaletteFilter(string(msg.Runes))
		m.dynamicArgumentError = ""
	case tea.KeyEnter, tea.KeyCtrlY:
		args := make([]string, 0, len(m.dynamicArgumentValues))
		for i, argument := range m.dynamicArgumentCommand.Arguments {
			value := strings.TrimSpace(m.dynamicArgumentValues[i])
			if argument.Required && value == "" {
				m.dynamicArgumentCursor = i
				m.dynamicArgumentError = argument.Name + " 为必填"
				return m, nil
			}
			if value != "" {
				args = append(args, argument.Name+"="+value)
			}
		}
		entry := *m.dynamicArgumentCommand
		raw := "/" + entry.Name + " " + strings.Join(args, " ")
		m.clearDynamicArguments()
		return m.dispatchCommand(&command.Invocation{Name: entry.Name, Args: args, Raw: strings.TrimSpace(raw)})
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
	registry, dynamic := m.effectiveCommandRegistry()
	var system, skill, mcp []command.Spec
	for _, spec := range registry.Specs() {
		search := strings.ToLower(strings.Join([]string{
			spec.Name,
			strings.Join(spec.Aliases, " "),
			spec.Usage,
			spec.Description,
		}, " "))
		if needle != "" && !fuzzyContains(search, needle) {
			continue
		}
		switch paletteGroup(spec, dynamic) {
		case "mcp":
			mcp = append(mcp, spec)
		case "skill":
			skill = append(skill, spec)
		default:
			system = append(system, spec)
		}
	}
	return append(append(system, skill...), mcp...)
}

func (m Model) effectiveCommandRegistry() (command.Registry, map[string]surface.DynamicCommand) {
	specs := commandRegistry.Specs()
	dynamic := make(map[string]surface.DynamicCommand)
	for _, candidate := range m.driver.DynamicCommands() {
		name := strings.ToLower(strings.TrimSpace(candidate.Name))
		if !safeDynamicCommandName(name) || strings.TrimSpace(candidate.ID) == "" || strings.TrimSpace(candidate.Kind) == "" {
			continue
		}
		if _, reserved := commandRegistry.Lookup(name); reserved {
			continue
		}
		if _, duplicate := dynamic[name]; duplicate {
			continue
		}
		usage := safeDynamicCommandUsage(name, candidate.Usage)
		description := strings.TrimSpace(sanitizeCommandPaletteFilter(candidate.Description))
		specs = append(specs, command.Spec{Name: name, Usage: usage, Description: description})
		candidate.Name = name
		dynamic[name] = candidate
	}
	registry, err := command.NewRegistry(specs...)
	if err != nil {
		return commandRegistry, map[string]surface.DynamicCommand{}
	}
	return registry, dynamic
}

func safeDynamicCommandUsage(name, usage string) string {
	usage = strings.TrimSpace(sanitizeCommandPaletteFilter(usage))
	fields := strings.Fields(usage)
	if len(fields) == 0 || fields[0] != "/"+name {
		return "/" + name + " [request]"
	}
	return usage
}

func safeDynamicCommandName(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		if r == '-' || r == '_' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			continue
		}
		return false
	}
	return true
}

func (m Model) submitInput() (Model, tea.Cmd) {
	if m.dynamicCommandPending {
		return m.showCommandError(fmt.Errorf("wait for the current dynamic command to finish expanding")), nil
	}
	registry, _ := m.effectiveCommandRegistry()
	parsed, err := registry.Parse(m.input)
	if err != nil {
		// Keep malformed/unknown command text in the editor so the operator can
		// dismiss the local error and correct it without retyping.
		return m.showCommandError(err), nil
	}
	if parsed.IsUnavailable() {
		return m.showCommandError(fmt.Errorf("%s", parsed.UnavailableReason)), nil
	}
	if parsed.IsShell() {
		if !m.driver.SupportsCapability("shell.start") {
			return m.showCommandError(fmt.Errorf("! shell commands are unavailable")), nil
		}
		if cmd := m.driver.ExecuteShell(parsed.Shell.Script); cmd != nil {
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
		if cmd := m.driver.SendWithContext(parsed.Text, parsed.FilePaths()); cmd != nil {
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
	if err := registry.Validate(parsed.Invocation); err != nil {
		return m.showCommandError(err), nil
	}
	m.input = ""
	return m.dispatchCommand(parsed.Invocation)
}

func (m Model) dispatchCommand(invocation *command.Invocation) (Model, tea.Cmd) {
	if invocation == nil {
		return m.showCommandError(fmt.Errorf("missing command")), nil
	}
	registry, dynamic := m.effectiveCommandRegistry()
	spec, ok := registry.Lookup(invocation.Name)
	if !ok {
		// Registry.Parse already performs this check. Keep the guard here so a
		// future registry change cannot turn an unknown slash line into model
		// text.
		return m.showCommandError(fmt.Errorf("unknown command /%s", invocation.Name)), nil
	}
	name := spec.Name
	args := invocation.Args
	if entry, ok := dynamic[name]; ok {
		if !m.driver.SupportsCapability("commands.expand") {
			return m.showCommandError(fmt.Errorf("dynamic command /%s is unavailable", name)), nil
		}
		m.dynamicCommandRequest++
		request := m.dynamicCommandRequest
		sessionID := m.driver.Active().ID
		if sessionID == "" {
			return m.showCommandError(fmt.Errorf("dynamic command /%s requires an active session", name)), nil
		}
		if cmd := m.driver.ExecuteDynamicCommand(request, sessionID, entry.ID, append([]string(nil), args...)); cmd != nil {
			m.dynamicCommandPending = true
			m.dynamicCommandID = entry.ID
			m.dynamicCommandSession = sessionID
			m.dynamicCommandDraft = invocation.Raw
			return m, cmd
		}
		return m.showCommandError(fmt.Errorf("dynamic command /%s is unavailable", name)), nil
	}
	switch name {
	case "help":
		if len(args) != 0 {
			return m.showCommandError(fmt.Errorf("usage: /help")), nil
		}
		m.commandOverlayTitle = "命令"
		m.commandOverlay = registry.HelpFor(m.driver.SupportsCapability("shell.start"))
		return m, nil
	case "status":
		if len(args) != 0 {
			return m.showCommandError(fmt.Errorf("usage: /status")), nil
		}
		m.commandOverlayTitle = "状态"
		m.commandOverlay = m.statusText()
		return m, nil
	case "sessions":
		if len(args) != 0 {
			return m.showCommandError(fmt.Errorf("usage: /sessions")), nil
		}
		return m.openSessions()
	case "model":
		return m.openModelPicker(strings.Join(args, " "))
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
	if cmd := m.driver.ExecuteCommand("image", append([]string(nil), args...)); cmd != nil {
		return m, cmd
	}
	return m.showCommandError(fmt.Errorf("/image is unavailable")), nil
}

func (m Model) setThinking(mode string) (Model, tea.Cmd) {
	if mode == "" {
		mode = nextThinking(m.driver.ThinkingMode())
	}
	if mode != "auto" && mode != "on" && mode != "off" {
		return m.showCommandError(fmt.Errorf("thinking must be auto, on, or off")), nil
	}
	if err := m.driver.SetThinkingMode(mode); err != nil {
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
	if cmd := m.driver.ExecuteCommand(name, append([]string(nil), args...)); cmd != nil {
		return m, cmd
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
	lines = append(lines, "thinking (next turn): "+m.driver.ThinkingMode())
	snapshot := m.driver.Sidebar()
	if snapshot.HasContext {
		ctx := snapshot.Context
		estimated := ""
		if ctx.TokenCountsEstimated {
			estimated = "~"
		}
		if ctx.ModelLimitKnown && ctx.ModelLimitTokens > 0 {
			percentage := int(float64(ctx.FeedTokens) / float64(ctx.ModelLimitTokens) * 100)
			lines = append(lines, fmt.Sprintf("context: %s%d/%d tokens (%s%d%%)", estimated, ctx.FeedTokens, ctx.ModelLimitTokens, estimated, percentage))
		} else if ctx.FeedTokens > 0 {
			lines = append(lines, fmt.Sprintf("context: %s%d tokens (model limit unknown)", estimated, ctx.FeedTokens))
		}
	}
	if strings.TrimSpace(meta.Error) != "" {
		lines = append(lines, "error: "+meta.Error)
	}
	return strings.Join(lines, "\n")
}

func (m Model) openSessions() (Model, tea.Cmd) {
	m.closeFileCompletion()
	m.closeModelPicker()
	m.shortcutsOpen = false
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
	if cmd := m.driver.RefreshSessions(); cmd != nil {
		m.sessionLoading = true
		return m, cmd
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
	cmd := m.driver.RenameSession(m.sessionRenameID, title)
	if cmd == nil {
		m.sessionError = "session rename is unavailable"
		return m, nil
	}
	m.sessionActionBusy = true
	m.sessionError = ""
	return m, cmd
}

func (m Model) submitDelete() (Model, tea.Cmd) {
	cmd := m.driver.DeleteSession(m.sessionDeleteID)
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
	cmd := m.driver.SelectSession(rows[m.sessionCursor].ID)
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
func Run(driver surface.Driver, options ...Options) error {
	return RunWithOutput(driver, os.Stdout, options...)
}

// RunWithOutput starts the canonical shell on the launcher's output stream.
func RunWithOutput(driver surface.Driver, out io.Writer, options ...Options) error {
	configureColor(out)
	p := tea.NewProgram(New(driver, options...), tea.WithAltScreen(), tea.WithMouseCellMotion(), tea.WithOutput(out))
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

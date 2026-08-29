// Package view is the Crush-style fullscreen TUI skeleton driven by demo data.
package view

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/tui/demo"
)

// Model is the sole Bubble Tea model for the demo skeleton.
type Model struct {
	store   *demo.Store
	width   int
	height  int
	input   string
	palette Palette
}

// New returns a model bound to the given demo store.
func New(store *demo.Store) Model {
	if store == nil {
		store = demo.NewStore()
	}
	return Model{
		store:   store,
		width:   120,
		height:  36,
		palette: defaultPalette(),
	}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd { return nil }

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = max(1, msg.Width)
		m.height = max(1, msg.Height)
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

// View implements tea.Model.
func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "vivy tui · demo"
	}
	return m.renderFrame()
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	gate := m.store.PendingGate()
	switch msg.Type {
	case tea.KeyCtrlC:
		return m, tea.Quit
	case tea.KeyEsc:
		// Overlay has no independent dismiss that keeps pending; esc is a no-op
		// on the gate body so the operator still y/n. Without a gate, clear input.
		if gate == nil {
			m.input = ""
		}
		return m, nil
	case tea.KeyCtrlN:
		if gate == nil {
			m.store.NewSession("")
			m.input = ""
		}
		return m, nil
	case tea.KeyTab:
		m.store.MoveSession(1)
		return m, nil
	case tea.KeyShiftTab:
		m.store.MoveSession(-1)
		return m, nil
	case tea.KeyUp:
		m.store.MoveSession(-1)
		return m, nil
	case tea.KeyDown:
		m.store.MoveSession(1)
		return m, nil
	case tea.KeyEnter:
		if gate != nil {
			return m, nil
		}
		m.store.AppendUser(m.input)
		m.input = ""
		return m, nil
	case tea.KeyBackspace:
		if gate != nil {
			return m, nil
		}
		if m.input != "" {
			r := []rune(m.input)
			m.input = string(r[:len(r)-1])
		}
		return m, nil
	case tea.KeyRunes:
		text := string(msg.Runes)
		if gate != nil {
			switch strings.ToLower(strings.TrimSpace(text)) {
			case "y":
				m.store.DecideApproval(domain.ApprovalApproved)
			case "n":
				m.store.DecideApproval(domain.ApprovalDenied)
			}
			return m, nil
		}
		if text == "q" && m.input == "" {
			return m, tea.Quit
		}
		m.input += text
		return m, nil
	}
	// Some terminals send y/n as KeyMsg with Type KeyRunes; also handle String().
	if gate != nil {
		switch strings.ToLower(msg.String()) {
		case "y":
			m.store.DecideApproval(domain.ApprovalApproved)
			return m, nil
		case "n":
			m.store.DecideApproval(domain.ApprovalDenied)
			return m, nil
		}
	}
	return m, nil
}

// RunDemo starts the fullscreen Bubble Tea program on the real terminal.
func RunDemo() error {
	store := demo.NewStore()
	p := tea.NewProgram(New(store), tea.WithAltScreen())
	_, err := p.Run()
	return err
}

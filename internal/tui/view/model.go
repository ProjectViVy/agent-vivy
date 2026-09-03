// Package view is the Crush-style fullscreen TUI driven by a surface.Driver
// (offline demo or live control-plane client).
package view

import (
	"io"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/tui/demo"
	"agent-vivy/internal/tui/surface"
)

// Model is the sole Bubble Tea model for the fullscreen shell.
type Model struct {
	driver  surface.Driver
	width   int
	height  int
	input   string
	palette Palette
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
		palette: defaultPalette(),
	}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	if m.driver == nil {
		return nil
	}
	return m.driver.Init()
}

// Update implements tea.Model.
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
	case surface.ErrMsg:
		// Driver already stores error in Meta; force redraw only.
	case surface.GateResolvedMsg:
		if msg.Kind == "question" {
			m.input = ""
		}
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
	if gate != nil && gate.Submitting && msg.Type != tea.KeyCtrlC && msg.Type != tea.KeyEsc {
		return m, nil
	}
	switch msg.Type {
	case tea.KeyCtrlC:
		return m, tea.Quit
	case tea.KeyEsc:
		if gate != nil {
			// Overlay has no independent dismiss that keeps pending; esc is a
			// no-op on the gate body so the operator still y/n (or types an answer).
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
	case tea.KeyTab:
		if !meta.Busy {
			return m, m.driver.MoveSession(1)
		}
		return m, nil
	case tea.KeyShiftTab:
		if !meta.Busy {
			return m, m.driver.MoveSession(-1)
		}
		return m, nil
	case tea.KeyUp:
		if m.input == "" && gate == nil && !meta.Busy {
			return m, m.driver.MoveSession(-1)
		}
		return m, nil
	case tea.KeyDown:
		if m.input == "" && gate == nil && !meta.Busy {
			return m, m.driver.MoveSession(1)
		}
		return m, nil
	case tea.KeyEnter:
		if gate != nil {
			if gate.Submitting {
				return m, nil
			}
			if gate.Kind == "question" {
				answer := m.input
				return m, m.driver.AnswerQuestion(answer)
			}
			return m, nil
		}
		text := m.input
		m.input = ""
		return m, m.driver.Send(text)
	case tea.KeyBackspace:
		if gate != nil && gate.Kind == "approval" {
			return m, nil
		}
		if m.input != "" {
			r := []rune(m.input)
			m.input = string(r[:len(r)-1])
		}
		return m, nil
	case tea.KeyCtrlJ:
		// Newline while composing (Crush ctrl+j).
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
				return m, m.driver.DecideApproval(domain.ApprovalApproved)
			case "n":
				return m, m.driver.DecideApproval(domain.ApprovalDenied)
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
			return m, m.driver.DecideApproval(domain.ApprovalApproved)
		case "n":
			return m, m.driver.DecideApproval(domain.ApprovalDenied)
		}
	}
	return m, nil
}

func nextPermission(current string) string {
	switch current {
	case string(domain.PermissionPresetCautious):
		return string(domain.PermissionPresetSmart)
	case string(domain.PermissionPresetSmart):
		return string(domain.PermissionPresetTrusted)
	default:
		return string(domain.PermissionPresetCautious)
	}
}

// RunDemo starts the fullscreen Bubble Tea program on offline demo data.
func RunDemo() error {
	return Run(demo.NewStore())
}

// Run starts the fullscreen Bubble Tea program on the given driver.
func Run(driver surface.Driver) error {
	return RunWithOutput(driver, os.Stdout)
}

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

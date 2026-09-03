// Package view keeps the packed-face import path while delegating the shell
// implementation to the shared first-party TUI view.
package view

import (
	tea "github.com/charmbracelet/bubbletea"

	"agent-vivy/sdk/tui/surface"
	shared "agent-vivy/sdk/tui/view"
)

type Model = shared.Model
type Palette = shared.Palette

func New(driver surface.Driver) Model { return shared.New(driver) }

var _ tea.Model = Model{}

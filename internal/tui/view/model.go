// Package view keeps the historical built-in import path while delegating
// the actual shell to the shared first-party TUI view.
package view

import (
	"io"

	tea "github.com/charmbracelet/bubbletea"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/tui/demo"
	"agent-vivy/sdk/tui/surface"
	shared "agent-vivy/sdk/tui/view"
)

type Model = shared.Model
type Palette = shared.Palette

func New(driver surface.Driver) Model { return shared.New(driver) }

func RunDemo() error                  { return shared.Run(demo.NewStore()) }
func Run(driver surface.Driver) error { return shared.Run(driver) }
func RunWithOutput(driver surface.Driver, out io.Writer) error {
	return shared.RunWithOutput(driver, out)
}

func defaultPalette() Palette { return shared.DefaultPalette() }

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

var _ tea.Model = Model{}

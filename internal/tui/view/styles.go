package view

import "github.com/charmbracelet/lipgloss"

// Palette is a small token set — fluorite teal accent, no rainbow.
type Palette struct {
	Header      lipgloss.Style
	Sidebar     lipgloss.Style
	Active      lipgloss.Style
	Idle        lipgloss.Style
	Chat        lipgloss.Style
	User        lipgloss.Style
	Assistant   lipgloss.Style
	Tool        lipgloss.Style
	ToolPend    lipgloss.Style
	Editor      lipgloss.Style
	Status      lipgloss.Style
	Dialog      lipgloss.Style
	DialogTitle lipgloss.Style
	Dim         lipgloss.Style
}

func defaultPalette() Palette {
	accent := lipgloss.Color("#2dd4bf")
	muted := lipgloss.Color("#64748b")
	fg := lipgloss.Color("#e2e8f0")
	warn := lipgloss.Color("#fbbf24")
	return Palette{
		Header:      lipgloss.NewStyle().Foreground(accent).Bold(true),
		Sidebar:     lipgloss.NewStyle().Foreground(fg),
		Active:      lipgloss.NewStyle().Foreground(accent).Bold(true),
		Idle:        lipgloss.NewStyle().Foreground(muted),
		Chat:        lipgloss.NewStyle().Foreground(fg),
		User:        lipgloss.NewStyle().Foreground(lipgloss.Color("#93c5fd")),
		Assistant:   lipgloss.NewStyle().Foreground(fg),
		Tool:        lipgloss.NewStyle().Foreground(muted).Border(lipgloss.RoundedBorder()).BorderForeground(muted).Padding(0, 1),
		ToolPend:    lipgloss.NewStyle().Foreground(warn).Border(lipgloss.RoundedBorder()).BorderForeground(warn).Padding(0, 1),
		Editor:      lipgloss.NewStyle().Foreground(fg),
		Status:      lipgloss.NewStyle().Foreground(muted),
		Dialog:      lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(warn).Padding(1, 2),
		DialogTitle: lipgloss.NewStyle().Foreground(warn).Bold(true),
		Dim:         lipgloss.NewStyle().Foreground(muted),
	}
}

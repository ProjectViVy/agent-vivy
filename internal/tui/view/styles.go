package view

import "github.com/charmbracelet/lipgloss"

// Palette mirrors Crush's quickStyle roles enough for a skeleton:
// base fg, subtle text, primary accent, success prompt, warning gate.
type Palette struct {
	Logo        lipgloss.Style
	LogoWord    lipgloss.Style
	Diagonals   lipgloss.Style
	HeaderMeta  lipgloss.Style
	Sidebar     lipgloss.Style
	SidebarLogo lipgloss.Style
	Active      lipgloss.Style
	Idle        lipgloss.Style
	Chat        lipgloss.Style
	User        lipgloss.Style
	UserBar     lipgloss.Style
	Assistant   lipgloss.Style
	AsstBar     lipgloss.Style
	Tool        lipgloss.Style
	ToolPend    lipgloss.Style
	ToolOK      lipgloss.Style
	Editor      lipgloss.Style
	Prompt      lipgloss.Style
	PromptWarn  lipgloss.Style
	Status      lipgloss.Style
	HelpKey     lipgloss.Style
	HelpDesc    lipgloss.Style
	Dialog      lipgloss.Style
	DialogTitle lipgloss.Style
	Dim         lipgloss.Style
	Separator   lipgloss.Style
}

func defaultPalette() Palette {
	// Crush-ish dark: charcoal base, violet primary, green success prompt,
	// fluorite teal as Vivy brand secondary on the active session.
	primary := lipgloss.Color("#A78BFA")
	secondary := lipgloss.Color("#2DD4BF")
	fg := lipgloss.Color("#E4E4E7")
	muted := lipgloss.Color("#71717A")
	subtle := lipgloss.Color("#52525B")
	success := lipgloss.Color("#4ADE80")
	warn := lipgloss.Color("#FBBF24")
	userC := lipgloss.Color("#93C5FD")
	return Palette{
		Logo:        lipgloss.NewStyle().Foreground(muted),
		LogoWord:    lipgloss.NewStyle().Foreground(primary).Bold(true),
		Diagonals:   lipgloss.NewStyle().Foreground(subtle),
		HeaderMeta:  lipgloss.NewStyle().Foreground(muted),
		Sidebar:     lipgloss.NewStyle().Foreground(fg),
		SidebarLogo: lipgloss.NewStyle().Foreground(primary).Bold(true),
		Active:      lipgloss.NewStyle().Foreground(secondary).Bold(true),
		Idle:        lipgloss.NewStyle().Foreground(muted),
		Chat:        lipgloss.NewStyle().Foreground(fg),
		User:        lipgloss.NewStyle().Foreground(userC),
		UserBar:     lipgloss.NewStyle().Foreground(userC),
		Assistant:   lipgloss.NewStyle().Foreground(fg),
		AsstBar:     lipgloss.NewStyle().Foreground(primary),
		Tool:        lipgloss.NewStyle().Foreground(muted).Border(lipgloss.NormalBorder()).BorderForeground(subtle).Padding(0, 1),
		ToolPend:    lipgloss.NewStyle().Foreground(warn).Border(lipgloss.NormalBorder()).BorderForeground(warn).Padding(0, 1),
		ToolOK:      lipgloss.NewStyle().Foreground(success),
		Editor:      lipgloss.NewStyle().Foreground(fg),
		Prompt:      lipgloss.NewStyle().Foreground(success),
		PromptWarn:  lipgloss.NewStyle().Foreground(lipgloss.Color("#1C1917")).Background(warn).Bold(true),
		Status:      lipgloss.NewStyle().Foreground(muted),
		HelpKey:     lipgloss.NewStyle().Foreground(primary),
		HelpDesc:    lipgloss.NewStyle().Foreground(subtle),
		Dialog:      lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(warn).Padding(1, 2),
		DialogTitle: lipgloss.NewStyle().Foreground(warn).Bold(true),
		Dim:         lipgloss.NewStyle().Foreground(subtle),
		Separator:   lipgloss.NewStyle().Foreground(subtle),
	}
}

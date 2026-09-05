package view

import "github.com/charmbracelet/lipgloss"

const (
	palettePrimary   = "#A78BFA"
	paletteSecondary = "#2DD4BF"
	paletteFg        = "#E4E4E7"
	paletteMuted     = "#71717A"
	paletteSubtle    = "#52525B"
	paletteSuccess   = "#4ADE80"
	paletteWarn      = "#FBBF24"
	paletteDanger    = "#FB7185"
	paletteUser      = "#93C5FD"
	paletteOnPrimary = "#1C1917"
	paletteCodeBg    = "#27272A"
	paletteString    = "#FDBA74"
	paletteLink      = "#A1A1AA"
)

// Palette is the single visual vocabulary used by the built-in and packed
// TUI faces. It intentionally contains roles, not page-specific colours.
type Palette struct {
	Logo          lipgloss.Style
	LogoWord      lipgloss.Style
	Diagonals     lipgloss.Style
	HeaderMeta    lipgloss.Style
	Sidebar       lipgloss.Style
	SidebarLogo   lipgloss.Style
	Active        lipgloss.Style
	Idle          lipgloss.Style
	Chat          lipgloss.Style
	User          lipgloss.Style
	UserBar       lipgloss.Style
	Assistant     lipgloss.Style
	AsstBar       lipgloss.Style
	Reasoning     lipgloss.Style
	ReasoningBar  lipgloss.Style
	Tool          lipgloss.Style
	ToolPend      lipgloss.Style
	ToolOK        lipgloss.Style
	ToolFail      lipgloss.Style
	DiffAdd       lipgloss.Style
	DiffDel       lipgloss.Style
	DiffHunk      lipgloss.Style
	Editor        lipgloss.Style
	EditorBox     lipgloss.Style
	Prompt        lipgloss.Style
	PromptWarn    lipgloss.Style
	Status        lipgloss.Style
	HelpKey       lipgloss.Style
	HelpDesc      lipgloss.Style
	Dialog        lipgloss.Style
	DialogTitle   lipgloss.Style
	DialogBody    lipgloss.Style
	DialogFooter  lipgloss.Style
	Dim           lipgloss.Style
	Separator     lipgloss.Style
	Selected      lipgloss.Style
	Match         lipgloss.Style
	IntensityHigh lipgloss.Style
	IntensityAuto lipgloss.Style
	ContextOK     lipgloss.Style
	ContextMid    lipgloss.Style
	ContextHot    lipgloss.Style
}

// DefaultPalette returns the stable dark palette shared by both first-party
// faces.
func DefaultPalette() Palette {
	primary := lipgloss.Color(palettePrimary)
	secondary := lipgloss.Color(paletteSecondary)
	fg := lipgloss.Color(paletteFg)
	muted := lipgloss.Color(paletteMuted)
	subtle := lipgloss.Color(paletteSubtle)
	success := lipgloss.Color(paletteSuccess)
	warn := lipgloss.Color(paletteWarn)
	danger := lipgloss.Color(paletteDanger)
	userC := lipgloss.Color(paletteUser)
	return Palette{
		Logo:          lipgloss.NewStyle().Foreground(muted),
		LogoWord:      lipgloss.NewStyle().Foreground(primary).Bold(true),
		Diagonals:     lipgloss.NewStyle().Foreground(subtle),
		HeaderMeta:    lipgloss.NewStyle().Foreground(muted),
		Sidebar:       lipgloss.NewStyle().Foreground(fg),
		SidebarLogo:   lipgloss.NewStyle().Foreground(primary).Bold(true),
		Active:        lipgloss.NewStyle().Foreground(secondary).Bold(true),
		Idle:          lipgloss.NewStyle().Foreground(muted),
		Chat:          lipgloss.NewStyle().Foreground(fg),
		User:          lipgloss.NewStyle().Foreground(userC).Bold(true),
		UserBar:       lipgloss.NewStyle().Foreground(userC).Bold(true),
		Assistant:     lipgloss.NewStyle().Foreground(fg),
		AsstBar:       lipgloss.NewStyle().Foreground(primary),
		Reasoning:     lipgloss.NewStyle().Foreground(muted).Italic(true),
		ReasoningBar:  lipgloss.NewStyle().Foreground(secondary),
		Tool:          lipgloss.NewStyle().Foreground(muted).Border(lipgloss.NormalBorder()).BorderForeground(subtle).Padding(0, 1),
		ToolPend:      lipgloss.NewStyle().Foreground(warn).Border(lipgloss.NormalBorder()).BorderForeground(warn).Padding(0, 1),
		ToolOK:        lipgloss.NewStyle().Foreground(success),
		ToolFail:      lipgloss.NewStyle().Foreground(danger),
		DiffAdd:       lipgloss.NewStyle().Foreground(success),
		DiffDel:       lipgloss.NewStyle().Foreground(danger),
		DiffHunk:      lipgloss.NewStyle().Foreground(secondary),
		Editor:        lipgloss.NewStyle().Foreground(fg),
		EditorBox:     lipgloss.NewStyle().Foreground(fg).Border(lipgloss.RoundedBorder()).BorderForeground(subtle).Padding(0, 1),
		Prompt:        lipgloss.NewStyle().Foreground(success),
		PromptWarn:    lipgloss.NewStyle().Foreground(lipgloss.Color(paletteOnPrimary)).Background(warn).Bold(true),
		Status:        lipgloss.NewStyle().Foreground(muted),
		HelpKey:       lipgloss.NewStyle().Foreground(primary),
		HelpDesc:      lipgloss.NewStyle().Foreground(subtle),
		Dialog:        lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(primary).Foreground(fg).Background(lipgloss.Color(paletteCodeBg)).Padding(1, 2),
		DialogTitle:   lipgloss.NewStyle().Foreground(primary).Bold(true).Background(lipgloss.Color(paletteCodeBg)),
		DialogBody:    lipgloss.NewStyle().Foreground(fg).Background(lipgloss.Color(paletteCodeBg)),
		DialogFooter:  lipgloss.NewStyle().Foreground(muted).Background(lipgloss.Color(paletteCodeBg)),
		Dim:           lipgloss.NewStyle().Foreground(subtle),
		Separator:     lipgloss.NewStyle().Foreground(subtle),
		Selected:      lipgloss.NewStyle().Foreground(fg).Background(lipgloss.Color(paletteCodeBg)),
		Match:         lipgloss.NewStyle().Foreground(secondary).Bold(true).Underline(true),
		IntensityHigh: lipgloss.NewStyle().Foreground(warn).Bold(true),
		IntensityAuto: lipgloss.NewStyle().Foreground(muted),
		ContextOK:     lipgloss.NewStyle().Foreground(success).Bold(true),
		ContextMid:    lipgloss.NewStyle().Foreground(warn).Bold(true),
		ContextHot:    lipgloss.NewStyle().Foreground(danger).Bold(true),
	}
}

package view

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"agent-vivy/sdk/tui/command"
	"agent-vivy/sdk/tui/surface"
)

const paletteNameCol = 14

type paletteSection struct {
	title string
	rows  []command.Spec
}

func (m Model) renderCommandPalette(l layout, p Palette) string {
	sections := m.paletteSections()
	w := max(1, min(l.width-8, 72))
	lineWidth := max(1, w-p.Dialog.GetHorizontalFrameSize())
	query := sanitizeCommandPaletteFilter(m.commandPaletteFilter)
	compact := l.height < 16
	filter := p.DialogFooter.Render("输入以筛选命令…")
	if query != "" {
		label := "筛选："
		if compact {
			label = "筛选："
		}
		filter = p.DialogFooter.Render(label) + p.Match.Render(truncate(query, max(1, lineWidth-lipgloss.Width(label))))
	}

	lines := []string{p.DialogTitle.Render("帮助"), filter}
	if m.commandCatalogLoading {
		lines = append(lines, p.DialogFooter.Render("正在刷新动态命令…"))
	} else if m.commandCatalogError != "" {
		lines = append(lines, p.DialogFooter.Render("动态刷新失败："+truncate(m.commandCatalogError, max(1, lineWidth-12))))
	}
	if !compact {
		lines = append(lines, "")
	}

	commands := m.filteredCommands()
	if len(commands) == 0 {
		lines = append(lines, p.DialogFooter.Render("没有匹配的命令"))
	} else {
		cursor := min(max(0, m.commandPaletteCursor), len(commands)-1)
		body := m.paletteBodyLines(sections, commands, cursor, query, lineWidth, !compact, p)
		windowRows := max(1, l.height-9)
		if compact {
			windowRows = max(1, l.height-7)
		}
		selectedLine := 0
		for i, line := range body {
			if strings.Contains(line, "▸") {
				selectedLine = i
				break
			}
		}
		start := max(0, selectedLine-windowRows/2)
		if start+windowRows > len(body) {
			start = max(0, len(body)-windowRows)
		}
		end := min(len(body), start+windowRows)
		lines = append(lines, body[start:end]...)
	}
	if !compact {
		lines = append(lines, "")
	}
	footer := "↑/↓ 选择 · enter 填入 · esc 关闭"
	if compact {
		footer = "↑/↓ · enter · esc"
	}
	lines = append(lines, p.DialogFooter.Render(footer))
	return p.Dialog.Width(w).Render(strings.Join(lines, "\n"))
}

func (m Model) paletteSections() []paletteSection {
	_, dynamic := m.effectiveCommandRegistry()
	var system, skill, mcp []command.Spec
	for _, spec := range m.filteredCommands() {
		switch paletteGroup(spec, dynamic) {
		case "mcp":
			mcp = append(mcp, spec)
		case "skill":
			skill = append(skill, spec)
		default:
			system = append(system, spec)
		}
	}
	sections := make([]paletteSection, 0, 3)
	if len(system) > 0 {
		sections = append(sections, paletteSection{title: "系统", rows: system})
	}
	if len(skill) > 0 {
		sections = append(sections, paletteSection{title: "技能", rows: skill})
	}
	if len(mcp) > 0 {
		sections = append(sections, paletteSection{title: "MCP", rows: mcp})
	}
	return sections
}

func paletteGroup(spec command.Spec, dynamic map[string]surface.DynamicCommand) string {
	if dyn, ok := dynamic[strings.ToLower(spec.Name)]; ok {
		if dyn.Kind == "mcp_prompt" {
			return "mcp"
		}
		return "skill"
	}
	return "system"
}

func (m Model) paletteBodyLines(sections []paletteSection, commands []command.Spec, cursor int, query string, lineWidth int, showDetail bool, p Palette) []string {
	selected := command.Spec{}
	if cursor >= 0 && cursor < len(commands) {
		selected = commands[cursor]
	}
	var lines []string
	for _, section := range sections {
		if len(sections) > 1 {
			lines = append(lines, p.Dim.Render(truncate(section.title, lineWidth)))
		}
		for _, spec := range section.rows {
			focused := spec.Name == selected.Name
			lines = append(lines, renderPaletteCommandRow(spec, query, focused, lineWidth, p))
			if focused && showDetail {
				detail := strings.TrimSpace(spec.Description)
				usage := strings.TrimSpace(spec.Usage)
				if usage == "" {
					usage = "/" + spec.Name
				}
				if detail != "" {
					lines = append(lines, p.Dim.Render(truncate("  "+detail, lineWidth)))
				}
				if usage != "/"+spec.Name {
					lines = append(lines, p.Dim.Render(truncate("  "+usage, lineWidth)))
				}
			}
		}
	}
	return lines
}

func renderPaletteCommandRow(spec command.Spec, query string, focused bool, lineWidth int, p Palette) string {
	marker := "  "
	if focused {
		marker = "▸ "
	}
	plainName := "/" + spec.Name
	if lipgloss.Width(plainName) > paletteNameCol {
		plainName = truncate(plainName, paletteNameCol)
	}
	name := highlightRunes(plainName, query, p.Match)
	namePad := max(0, paletteNameCol-lipgloss.Width(plainName))
	title := strings.TrimSpace(spec.Description)
	if title == "" {
		title = spec.Name
	}
	title = highlightRunes(truncate(title, max(1, lineWidth-lipgloss.Width(marker)-paletteNameCol-1)), query, p.Match)
	row := marker + name + strings.Repeat(" ", namePad) + " " + title
	if lipgloss.Width(row) > lineWidth {
		row = truncate(row, lineWidth)
	}
	if focused {
		return p.Selected.Width(lineWidth).MaxWidth(lineWidth).Render(row)
	}
	return p.Idle.Render(row)
}

func highlightRunes(text, query string, match lipgloss.Style) string {
	indexes := matchRuneIndexes(text, query)
	if len(indexes) == 0 {
		return text
	}
	hit := make(map[int]struct{}, len(indexes))
	for _, i := range indexes {
		hit[i] = struct{}{}
	}
	var b strings.Builder
	for i, r := range []rune(text) {
		s := string(r)
		if _, ok := hit[i]; ok {
			b.WriteString(match.Render(s))
		} else {
			b.WriteString(s)
		}
	}
	return b.String()
}

func matchRuneIndexes(haystack, needle string) []int {
	needle = strings.ToLower(strings.TrimSpace(needle))
	if needle == "" {
		return nil
	}
	want := []rune(needle)
	source := []rune(haystack)
	lower := []rune(strings.ToLower(haystack))
	indexes := make([]int, 0, len(want))
	j := 0
	for i := 0; i < len(source) && j < len(want); i++ {
		if lower[i] == want[j] {
			indexes = append(indexes, i)
			j++
		}
	}
	if j != len(want) {
		return nil
	}
	return indexes
}

package view

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/tui/demo"
)

const headerDiag = "╱"

func (m Model) renderFrame() string {
	l := computeLayout(m.width, m.height)
	p := m.palette

	var app string
	if l.showSidebar {
		app = m.renderWide(l, p)
	} else {
		app = m.renderCompact(l, p)
	}
	help := m.renderHelp(l, p)

	// Outer vertical: top margin + app + help (Crush helpRect under appRect).
	topPad := strings.Repeat(" ", l.width)
	frame := lipgloss.JoinVertical(lipgloss.Left, topPad, app, help)
	// Ensure exact height by padding/truncating.
	frame = fitHeight(frame, l.width, l.height)

	if gate := m.store.PendingGate(); gate != nil {
		return placeOverlay(frame, m.renderDialog(gate, l, p), l.width, l.height)
	}
	return frame
}

func (m Model) renderWide(l layout, p Palette) string {
	// Crush: main stack (chat + editor) | sidebar
	chat := m.renderChat(l.mainW(), l.mainH(), p)
	editor := m.renderEditor(l.mainW(), p)
	mainCol := lipgloss.JoinVertical(lipgloss.Left, chat, "", editor)
	side := m.renderSidebar(l.sidebarW, lipgloss.Height(mainCol), p)
	gap := lipgloss.NewStyle().Width(1).Height(lipgloss.Height(mainCol)).Render(" ")
	row := lipgloss.JoinHorizontal(lipgloss.Top, mainCol, gap, side)
	return padHorizontal(row, l.marginX, l.width)
}

func (m Model) renderCompact(l layout, p Palette) string {
	header := m.renderCompactHeader(l, p)
	chat := m.renderChat(l.innerW(), l.mainH(), p)
	editor := m.renderEditor(l.innerW(), p)
	col := lipgloss.JoinVertical(lipgloss.Left, header, "", chat, "", editor)
	return padHorizontal(col, l.marginX, l.width)
}

func (m Model) renderCompactHeader(l layout, p Palette) string {
	session := m.store.Active()
	logo := p.Logo.Render("Vivy™ ") + p.LogoWord.Render("VIVY") + " "
	meta := p.HeaderMeta.Render(fmt.Sprintf("demo · %s", session.Title))
	used := lipgloss.Width(logo) + lipgloss.Width(meta) + 1
	diags := max(3, l.innerW()-used)
	mid := p.Diagonals.Render(strings.Repeat(headerDiag, diags))
	line := logo + mid + " " + meta
	return truncate(line, l.innerW())
}

func (m Model) renderSidebar(width, height int, p Palette) string {
	var b strings.Builder
	// Crush: fixed logo on top of sidebar.
	b.WriteString(p.SidebarLogo.Render(" Vivy"))
	b.WriteByte('\n')
	b.WriteString(p.Dim.Render(" ─────────────"))
	b.WriteByte('\n')
	b.WriteString(p.Dim.Render(" Sessions"))
	b.WriteByte('\n')
	for _, session := range m.store.Sessions {
		mark := "  "
		style := p.Idle
		if session.ID == m.store.ActiveID {
			mark = "▸ "
			style = p.Active
		}
		line := mark + session.Title
		preset := session.PermissionPreset
		if preset != "" {
			line = truncate(line, width-2)
			// second line subtle id/preset
			b.WriteString(style.Render(truncate(line, width-1)))
			b.WriteByte('\n')
			b.WriteString(p.Dim.Render(truncate("  "+preset, width-1)))
			b.WriteByte('\n')
			continue
		}
		b.WriteString(style.Render(truncate(line, width-1)))
		b.WriteByte('\n')
	}
	b.WriteByte('\n')
	b.WriteString(p.Dim.Render(" mock"))
	b.WriteByte('\n')
	b.WriteString(p.Dim.Render(" not connected"))
	box := strings.TrimRight(b.String(), "\n")
	return p.Sidebar.Width(width).Height(height).MaxHeight(height).Render(padBlock(box, width, height))
}

func (m Model) renderChat(width, height int, p Palette) string {
	messages := m.store.ActiveMessages()
	var lines []string
	if len(messages) == 0 {
		lines = append(lines,
			p.Dim.Render(""),
			p.LogoWord.Render(" 寻找真心之旅"),
			p.Dim.Render(" demo empty session · type to draft"),
		)
	}
	for _, message := range messages {
		lines = append(lines, renderMessage(message, width, p)...)
		lines = append(lines, "")
	}
	content := strings.Join(trimTrailingEmpty(lines), "\n")
	content = tailBlock(content, height)
	return p.Chat.Width(width).Height(height).MaxHeight(height).Render(padBlock(content, width, height))
}

func renderMessage(message demo.Message, width int, p Palette) []string {
	if message.Tool != nil {
		return renderTool(message.Tool, width, p)
	}
	bar := p.AsstBar.Render("┃ ")
	style := p.Assistant
	label := ""
	switch message.Role {
	case string(domain.RoleUser):
		bar = p.UserBar.Render("┃ ")
		style = p.User
	case string(domain.RoleAssistant):
		// Crush assistant often omits a loud "assistant:" prefix; keep content.
		label = ""
	}
	text := message.Content
	if label != "" {
		text = label + text
	}
	wrapped := wrapText(text, max(8, width-3))
	out := make([]string, 0, len(wrapped))
	for _, line := range wrapped {
		out = append(out, bar+style.Render(line))
	}
	return out
}

func renderTool(tool *demo.ToolCard, width int, p Palette) []string {
	style := p.Tool
	icon := "●"
	switch tool.Status {
	case "pending":
		style = p.ToolPend
		icon = "◉"
	case "done":
		icon = p.ToolOK.Render("✔")
	case "denied", "failed":
		icon = "✖"
	}
	title := fmt.Sprintf("%s %s  %s", icon, tool.ToolName, tool.Status)
	body := tool.Preview
	if tool.Status != "pending" && tool.Result != "" {
		body = tool.Result
	}
	inner := title
	if body != "" {
		inner = title + "\n" + body
	}
	boxW := min(width-4, 56)
	box := style.Width(boxW).Render(inner)
	// Indent tool cards under the message gutter.
	indented := make([]string, 0)
	for _, line := range strings.Split(box, "\n") {
		indented = append(indented, "  "+line)
	}
	return indented
}

func (m Model) renderEditor(width int, p Palette) string {
	gate := m.store.PendingGate()
	var prompt string
	if gate != nil {
		prompt = p.PromptWarn.Render(" ! ") + p.Prompt.Render("::: ")
	} else {
		prompt = p.Prompt.Render("::: ")
	}
	line := prompt + m.input
	// Crush editor sits without a heavy double rule; a single subtle rule above.
	rule := p.Separator.Render(strings.Repeat("─", max(1, width)))
	cursor := p.Dim.Render("█")
	if gate != nil {
		cursor = ""
	}
	return p.Editor.Width(width).Render(rule + "\n" + truncate(line+cursor, width))
}

func (m Model) renderHelp(l layout, p Palette) string {
	// Crush bottom help: key + desc pairs.
	parts := []string{
		p.HelpKey.Render("tab") + p.HelpDesc.Render(" sessions"),
		p.HelpKey.Render("enter") + p.HelpDesc.Render(" send"),
		p.HelpKey.Render("y/n") + p.HelpDesc.Render(" approve"),
		p.HelpKey.Render("^n") + p.HelpDesc.Render(" new"),
		p.HelpKey.Render("^c") + p.HelpDesc.Render(" quit"),
		p.HelpDesc.Render("· mock · not connected"),
	}
	line := " " + strings.Join(parts, p.HelpDesc.Render("  "))
	return p.Status.Width(l.width).Render(truncate(line, l.width))
}

func (m Model) renderDialog(gate *demo.Gate, l layout, p Palette) string {
	title := p.DialogTitle.Render("permission  ·  " + gate.Title)
	body := p.Chat.Render(gate.Body)
	help := p.Dim.Render("y approve    n deny")
	inner := lipgloss.JoinVertical(lipgloss.Left, title, "", body, "", help)
	w := min(l.width-6, 64)
	return p.Dialog.Width(w).Render(inner)
}

func padHorizontal(content string, margin, totalWidth int) string {
	if margin <= 0 {
		return content
	}
	pad := strings.Repeat(" ", margin)
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		lines[i] = pad + line
		// right pad to total width
		w := lipgloss.Width(lines[i])
		if w < totalWidth {
			lines[i] += strings.Repeat(" ", totalWidth-w)
		}
	}
	return strings.Join(lines, "\n")
}

func fitHeight(content string, width, height int) string {
	lines := strings.Split(content, "\n")
	out := make([]string, 0, height)
	for _, line := range lines {
		out = append(out, padRight(truncate(line, width), width))
		if len(out) == height {
			break
		}
	}
	for len(out) < height {
		out = append(out, strings.Repeat(" ", width))
	}
	return strings.Join(out, "\n")
}

func placeOverlay(base, overlay string, width, height int) string {
	baseLines := padLines(strings.Split(base, "\n"), width, height)
	overLines := strings.Split(overlay, "\n")
	ow := 0
	for _, line := range overLines {
		if w := lipgloss.Width(line); w > ow {
			ow = w
		}
	}
	oh := len(overLines)
	row := max(0, (height-oh)/2)
	col := max(0, (width-ow)/2)
	for i, line := range overLines {
		r := row + i
		if r < 0 || r >= len(baseLines) {
			continue
		}
		baseLines[r] = overlayLine(baseLines[r], line, col, width)
	}
	return strings.Join(baseLines, "\n")
}

func overlayLine(base, over string, col, width int) string {
	plain := stripForPad(base)
	if len([]rune(plain)) < width {
		plain += strings.Repeat(" ", width-len([]rune(plain)))
	}
	runes := []rune(plain)
	or := []rune(stripForPad(over))
	for i := 0; i < len(or) && col+i < len(runes); i++ {
		runes[col+i] = or[i]
	}
	if col == 0 && lipgloss.Width(over) >= width {
		return over
	}
	left := string(runes[:min(col, len(runes))])
	rightStart := col + lipgloss.Width(over)
	right := ""
	if rightStart < len(runes) {
		right = string(runes[rightStart:])
	}
	return left + over + right
}

func stripForPad(s string) string {
	var b strings.Builder
	inESC := false
	for _, r := range s {
		if r == 0x1b {
			inESC = true
			continue
		}
		if inESC {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inESC = false
			}
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func padBlock(content string, width, height int) string {
	lines := padLines(strings.Split(content, "\n"), width, height)
	return strings.Join(lines, "\n")
}

func padLines(lines []string, width, height int) []string {
	out := make([]string, 0, height)
	for _, line := range lines {
		out = append(out, padRight(truncate(line, width), width))
		if len(out) == height {
			break
		}
	}
	for len(out) < height {
		out = append(out, strings.Repeat(" ", width))
	}
	return out
}

func tailBlock(content string, height int) string {
	lines := strings.Split(content, "\n")
	if len(lines) <= height {
		return content
	}
	return strings.Join(lines[len(lines)-height:], "\n")
}

func trimTrailingEmpty(lines []string) []string {
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func wrapText(text string, width int) []string {
	if width < 8 {
		width = 8
	}
	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{""}
	}
	var lines []string
	var cur string
	for _, word := range words {
		if cur == "" {
			cur = word
			continue
		}
		if lipgloss.Width(cur+" "+word) <= width {
			cur += " " + word
			continue
		}
		lines = append(lines, cur)
		cur = word
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	return lines
}

func truncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= width {
		return s
	}
	runes := []rune(s)
	if width <= 1 {
		return "…"
	}
	for len(runes) > 0 && lipgloss.Width(string(runes)+"…") > width {
		runes = runes[:len(runes)-1]
	}
	return string(runes) + "…"
}

func padRight(s string, width int) string {
	w := lipgloss.Width(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}

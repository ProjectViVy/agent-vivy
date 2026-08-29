package view

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/tui/surface"
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

	if gate := m.driver.PendingGate(); gate != nil {
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
	session := m.driver.Active()
	meta := m.driver.Meta()
	logo := p.Logo.Render("Vivy™ ") + p.LogoWord.Render("VIVY") + " "
	label := session.Title
	if meta.Mode == "live" && meta.Host != "" {
		label = fmt.Sprintf("%s · %s", session.Title, meta.Host)
	} else if meta.Mode == "demo" {
		label = fmt.Sprintf("demo · %s", session.Title)
	}
	headerMeta := p.HeaderMeta.Render(label)
	used := lipgloss.Width(logo) + lipgloss.Width(headerMeta) + 1
	diags := max(3, l.innerW()-used)
	mid := p.Diagonals.Render(strings.Repeat(headerDiag, diags))
	line := logo + mid + " " + headerMeta
	return truncate(line, l.innerW())
}

func (m Model) renderSidebar(width, height int, p Palette) string {
	meta := m.driver.Meta()
	var b strings.Builder
	// Crush: fixed logo on top of sidebar.
	b.WriteString(p.SidebarLogo.Render(" Vivy"))
	b.WriteByte('\n')
	b.WriteString(p.Dim.Render(" ─────────────"))
	b.WriteByte('\n')
	b.WriteString(p.Dim.Render(" Sessions"))
	b.WriteByte('\n')
	activeID := m.driver.Active().ID
	for _, session := range m.driver.Sessions() {
		mark := "  "
		style := p.Idle
		if session.ID == activeID {
			mark = "▸ "
			style = p.Active
		}
		line := mark + session.Title
		preset := session.PermissionPreset
		if preset != "" {
			line = truncate(line, width-2)
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
	if meta.Mode == "live" {
		b.WriteString(p.Dim.Render(" live"))
		b.WriteByte('\n')
		host := meta.Host
		if host == "" {
			host = "connected"
		}
		b.WriteString(p.Dim.Render(truncate(" "+host, width-1)))
		if meta.Busy {
			b.WriteByte('\n')
			b.WriteString(p.Dim.Render(truncate(" run…", width-1)))
		}
	} else {
		b.WriteString(p.Dim.Render(" mock"))
		b.WriteByte('\n')
		b.WriteString(p.Dim.Render(" not connected"))
	}
	if meta.Error != "" {
		b.WriteByte('\n')
		b.WriteString(p.PromptWarn.Render(truncate(" ! "+meta.Error, width-1)))
	}
	box := strings.TrimRight(b.String(), "\n")
	return p.Sidebar.Width(width).Height(height).MaxHeight(height).Render(padBlock(box, width, height))
}

func (m Model) renderChat(width, height int, p Palette) string {
	messages := m.driver.ActiveMessages()
	var lines []string
	if len(messages) == 0 {
		lines = append(lines,
			p.Dim.Render(""),
			p.LogoWord.Render(" 寻找真心之旅"),
			p.Dim.Render(" empty session · type to draft"),
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

func renderMessage(message surface.Message, width int, p Palette) []string {
	if message.Tool != nil {
		return renderTool(message.Tool, width, p)
	}
	bar := p.AsstBar.Render("┃ ")
	style := p.Assistant
	switch message.Role {
	case string(domain.RoleUser):
		bar = p.UserBar.Render("┃ ")
		style = p.User
	case string(domain.RoleAssistant):
		// Crush assistant often omits a loud "assistant:" prefix; keep content.
	}
	text := message.Content
	if message.Streaming {
		text += "▌"
	}
	wrapped := wrapText(text, max(8, width-3))
	out := make([]string, 0, len(wrapped))
	for _, line := range wrapped {
		out = append(out, bar+style.Render(line))
	}
	return out
}

func renderTool(tool *surface.ToolCard, width int, p Palette) []string {
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
	gate := m.driver.PendingGate()
	var prompt string
	if gate != nil {
		prompt = p.PromptWarn.Render(" ! ") + p.Prompt.Render("::: ")
	} else {
		prompt = p.Prompt.Render("::: ")
	}
	// Multi-line input: show last line in the single-row editor chrome.
	display := m.input
	if i := strings.LastIndex(display, "\n"); i >= 0 {
		display = display[i+1:]
	}
	line := prompt + display
	// Crush editor sits without a heavy double rule; a single subtle rule above.
	rule := p.Separator.Render(strings.Repeat("─", max(1, width)))
	cursor := p.Dim.Render("█")
	if gate != nil && gate.Kind == "approval" {
		cursor = ""
	}
	return p.Editor.Width(width).Render(rule + "\n" + truncate(line+cursor, width))
}

func (m Model) renderHelp(l layout, p Palette) string {
	meta := m.driver.Meta()
	parts := []string{
		p.HelpKey.Render("tab") + p.HelpDesc.Render(" sessions"),
		p.HelpKey.Render("enter") + p.HelpDesc.Render(" send"),
		p.HelpKey.Render("y/n") + p.HelpDesc.Render(" approve"),
		p.HelpKey.Render("^n") + p.HelpDesc.Render(" new"),
		p.HelpKey.Render("esc") + p.HelpDesc.Render(" cancel"),
		p.HelpKey.Render("^c") + p.HelpDesc.Render(" quit"),
	}
	footer := meta.Footer
	if footer == "" {
		if meta.Mode == "live" {
			footer = "live"
			if meta.Host != "" {
				footer = "live · " + meta.Host
			}
			if meta.Busy {
				footer += " · run…"
			}
		} else {
			footer = "mock · not connected"
		}
	}
	if meta.Error != "" {
		footer = "err · " + meta.Error
	}
	parts = append(parts, p.HelpDesc.Render("· "+footer))
	line := " " + strings.Join(parts, p.HelpDesc.Render("  "))
	return p.Status.Width(l.width).Render(truncate(line, l.width))
}

func (m Model) renderDialog(gate *surface.Gate, l layout, p Palette) string {
	kind := gate.Kind
	if kind == "" {
		kind = "permission"
	}
	if kind == "approval" {
		kind = "permission"
	}
	title := p.DialogTitle.Render(kind + "  ·  " + gate.Title)
	body := p.Chat.Render(gate.Body)
	help := p.Dim.Render("y approve    n deny")
	if gate.Kind == "question" {
		help = p.Dim.Render("type answer · enter submit")
	}
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
	if text == "" {
		return []string{""}
	}
	// Preserve explicit newlines from multi-line drafts / tool bodies.
	var lines []string
	for _, para := range strings.Split(text, "\n") {
		words := strings.Fields(para)
		if len(words) == 0 {
			lines = append(lines, "")
			continue
		}
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
	}
	if len(lines) == 0 {
		return []string{""}
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

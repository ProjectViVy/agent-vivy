package view

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/tui/demo"
)

func (m Model) renderFrame() string {
	l := computeLayout(m.width, m.height)
	p := m.palette

	header := m.renderHeader(l, p)
	body := m.renderBody(l, p)
	editor := m.renderEditor(l, p)
	status := m.renderStatus(l, p)

	frame := lipgloss.JoinVertical(lipgloss.Left, header, body, editor, status)
	if gate := m.store.PendingGate(); gate != nil {
		return placeOverlay(frame, m.renderDialog(gate, l, p), l.width, l.height)
	}
	return frame
}

func (m Model) renderHeader(l layout, p Palette) string {
	session := m.store.Active()
	line := fmt.Sprintf(" vivy tui · demo · %s %s", session.ID, session.Title)
	return p.Header.Width(l.width).MaxWidth(l.width).Render(truncate(line, l.width))
}

func (m Model) renderBody(l layout, p Palette) string {
	main := m.renderChat(l.mainW(), l.mainH(), p)
	if !l.showSidebar {
		return main
	}
	side := m.renderSidebar(l.sidebarW, l.mainH(), p)
	return lipgloss.JoinHorizontal(lipgloss.Top, side, main)
}

func (m Model) renderSidebar(width, height int, p Palette) string {
	var b strings.Builder
	b.WriteString(p.Dim.Render(" 会话"))
	b.WriteByte('\n')
	for _, session := range m.store.Sessions {
		mark := " "
		style := p.Idle
		if session.ID == m.store.ActiveID {
			mark = "*"
			style = p.Active
		}
		line := fmt.Sprintf("%s %s", mark, session.Title)
		b.WriteString(style.Render(truncate(line, width-1)))
		b.WriteByte('\n')
	}
	box := strings.TrimRight(b.String(), "\n")
	return p.Sidebar.Width(width).Height(height).MaxHeight(height).Render(padBlock(box, width, height))
}

func (m Model) renderChat(width, height int, p Palette) string {
	messages := m.store.ActiveMessages()
	var lines []string
	if len(messages) == 0 {
		lines = append(lines, p.Dim.Render(" 寻找真心之旅"), p.Dim.Render(" （demo 空会话）"))
	}
	for _, message := range messages {
		lines = append(lines, renderMessage(message, width, p)...)
		lines = append(lines, "")
	}
	content := strings.Join(trimTrailingEmpty(lines), "\n")
	// Keep the bottom of the transcript visible.
	content = tailBlock(content, height)
	return p.Chat.Width(width).Height(height).MaxHeight(height).Render(padBlock(content, width, height))
}

func renderMessage(message demo.Message, width int, p Palette) []string {
	if message.Tool != nil {
		return renderTool(message.Tool, width, p)
	}
	role := message.Role
	style := p.Assistant
	prefix := "vivy"
	switch message.Role {
	case string(domain.RoleUser):
		style = p.User
		prefix = "you"
	case string(domain.RoleAssistant):
		prefix = "vivy"
	}
	_ = role
	wrapped := wrapText(prefix+": "+message.Content, width-1)
	out := make([]string, 0, len(wrapped))
	for _, line := range wrapped {
		out = append(out, style.Render(line))
	}
	return out
}

func renderTool(tool *demo.ToolCard, width int, p Palette) []string {
	style := p.Tool
	if tool.Status == "pending" {
		style = p.ToolPend
	}
	title := fmt.Sprintf("tool %s  %s", tool.ToolName, tool.Status)
	body := tool.Preview
	if tool.Status != "pending" && tool.Result != "" {
		body = tool.Result
	}
	inner := title
	if body != "" {
		inner = title + "\n" + body
	}
	box := style.Width(min(width-2, 60)).Render(inner)
	return strings.Split(box, "\n")
}

func (m Model) renderEditor(l layout, p Palette) string {
	prompt := " you> "
	if m.store.PendingGate() != nil {
		prompt = " approve? [y/n] "
	}
	line := prompt + m.input
	border := strings.Repeat("─", max(1, l.width))
	return p.Editor.Width(l.width).Render(border + "\n" + truncate(line, l.width) + "\n" + border)
}

func (m Model) renderStatus(l layout, p Palette) string {
	text := " mock · not connected · tab 切会话 · enter 假回复 · y/n 审批 · ctrl+c 退出"
	return p.Status.Width(l.width).Render(truncate(text, l.width))
}

func (m Model) renderDialog(gate *demo.Gate, l layout, p Palette) string {
	title := p.DialogTitle.Render("approval · " + gate.Title)
	body := p.Chat.Render(gate.Body)
	help := p.Dim.Render("y 批准   n 拒绝   esc 关闭")
	inner := lipgloss.JoinVertical(lipgloss.Left, title, "", body, "", help)
	w := min(l.width-4, 64)
	return p.Dialog.Width(w).Render(inner)
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
	// Strip styles for placement math; keep overlay styled chunk spliced by padding.
	plain := stripForPad(base)
	if len([]rune(plain)) < width {
		plain += strings.Repeat(" ", width-len([]rune(plain)))
	}
	runes := []rune(plain)
	or := []rune(stripForPad(over))
	for i := 0; i < len(or) && col+i < len(runes); i++ {
		runes[col+i] = or[i]
	}
	// Prefer showing the styled overlay row when it fits; otherwise plain splice.
	if col == 0 && lipgloss.Width(over) >= width {
		return over
	}
	left := string(runes[:col])
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
		return string(runes[:1])
	}
	// Approximate: cut runes then rely on lipgloss width.
	for len(runes) > 0 && lipgloss.Width(string(runes)) > width-1 {
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

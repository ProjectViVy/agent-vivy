package view

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"agent-vivy/sdk/tui/surface"
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
	frame := lipgloss.JoinVertical(lipgloss.Left, strings.Repeat(" ", l.width), app, help)
	frame = fitHeight(frame, l.width, l.height)

	if gate := m.driver.PendingGate(); gate != nil {
		return placeOverlay(frame, m.renderGateDialog(gate, l, p), l.width, l.height)
	}
	if m.sessionsOpen {
		return placeOverlay(frame, m.renderSessionsDialog(l, p), l.width, l.height)
	}
	return frame
}

func (m Model) renderWide(l layout, p Palette) string {
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
	logo := p.Logo.Render("Vivy™ ") + p.LogoWord.Render("VIVY CODE") + " "
	label := session.Title
	if label == "" {
		label = "untitled session"
	}
	if meta.Mode == "live" && meta.Host != "" {
		label = fmt.Sprintf("%s · %s", label, meta.Host)
	} else if meta.Mode == "demo" {
		label = fmt.Sprintf("demo · %s", label)
	}
	headerMeta := p.HeaderMeta.Render(label)
	used := lipgloss.Width(logo) + lipgloss.Width(headerMeta) + 1
	diags := max(3, l.innerW()-used)
	line := logo + p.Diagonals.Render(strings.Repeat(headerDiag, diags)) + " " + headerMeta
	return truncate(line, l.innerW())
}

// renderSidebar intentionally contains no session collection. Crush uses the
// right rail for the active session and live context details; the collection
// is a separate Ctrl+S surface so the chat remains the primary workspace.
func (m Model) renderSidebar(width, height int, p Palette) string {
	active := m.driver.Active()
	snapshot := surface.Sidebar{Session: active}
	if provider, ok := m.driver.(surface.SidebarProvider); ok {
		provided := provider.Sidebar()
		if provided.Session.ID == "" {
			provided.Session = active
		}
		snapshot = provided
	}

	var b strings.Builder
	b.WriteString(p.SidebarLogo.Render(" VIVY CODE"))
	b.WriteByte('\n')
	b.WriteString(p.Dim.Render(" ─────────────"))
	b.WriteByte('\n')
	title := strings.TrimSpace(snapshot.Session.Title)
	if title == "" {
		title = "untitled session"
	}
	titleLines := wrapText(title, max(8, width-2))
	if len(titleLines) > 2 {
		titleLines = titleLines[:2]
	}
	for _, line := range titleLines {
		b.WriteString(p.Active.Render(" " + line))
		b.WriteByte('\n')
	}
	if preset := strings.TrimSpace(snapshot.Session.PermissionPreset); preset != "" {
		b.WriteString(p.Dim.Render(truncate(" permission · "+preset, width-1)))
		b.WriteByte('\n')
	}
	if snapshot.HasContext {
		b.WriteByte('\n')
		b.WriteString(p.Dim.Render(" Context"))
		b.WriteByte('\n')
		ctx := snapshot.Context
		switch {
		case ctx.ModelLimitTokens > 0:
			b.WriteString(p.Dim.Render(truncate(fmt.Sprintf(" %s / %s tokens", compactNumber(ctx.FeedTokens), compactNumber(ctx.ModelLimitTokens)), width-1)))
		case ctx.FeedTokens > 0:
			b.WriteString(p.Dim.Render(truncate(fmt.Sprintf(" %s tokens", compactNumber(ctx.FeedTokens)), width-1)))
		}
		if ctx.TotalMessages > 0 {
			b.WriteByte('\n')
			b.WriteString(p.Dim.Render(truncate(fmt.Sprintf(" %d messages", ctx.TotalMessages), width-1)))
		}
		if ctx.CompactionEnabled {
			b.WriteByte('\n')
			compaction := " compaction on"
			if ctx.WouldCompact {
				compaction = " compaction needed"
			}
			b.WriteString(p.Dim.Render(compaction))
		}
	}
	if errText := strings.TrimSpace(m.driver.Meta().Error); errText != "" {
		b.WriteByte('\n')
		b.WriteString(p.PromptWarn.Render(truncate(" ! "+errText, width-1)))
	}
	box := strings.TrimRight(b.String(), "\n")
	return p.Sidebar.Width(width).Height(height).MaxHeight(height).Render(padBlock(box, width, height))
}

func compactNumber(n int) string {
	if n >= 1000 {
		return fmt.Sprintf("%.1fk", float64(n)/1000)
	}
	return fmt.Sprintf("%d", n)
}

func (m Model) renderChat(width, height int, p Palette) string {
	messages := m.driver.ActiveMessages()
	var lines []string
	if len(messages) == 0 {
		lines = append(lines, p.Dim.Render(""), p.LogoWord.Render(" 寻找真心之旅"), p.Dim.Render(" empty session · type to draft"))
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
	if message.Reasoning {
		bar = p.ReasoningBar.Render("┊ ")
		style = p.Reasoning
	}
	switch message.Role {
	case surface.RoleUser:
		bar = p.UserBar.Render("┃ ")
		style = p.User
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
		icon = p.ToolFail.Render("✖")
	}
	title := fmt.Sprintf("%s %s  %s", icon, tool.ToolName, tool.Status)
	body := tool.Preview
	if tool.Status != "pending" && tool.Result != "" {
		body = tool.Result
	}
	body = renderDiffBody(body, p)
	inner := title
	if body != "" {
		inner += "\n" + body
	}
	boxW := max(1, min(width-4, 56))
	box := style.Width(boxW).Render(inner)
	indented := make([]string, 0)
	for _, line := range strings.Split(box, "\n") {
		indented = append(indented, "  "+line)
	}
	return indented
}

func renderDiffBody(body string, p Palette) string {
	if !strings.Contains(body, "@@") && !strings.Contains(body, "\n+") && !strings.Contains(body, "\n-") {
		return body
	}
	lines := strings.Split(body, "\n")
	adds, dels := 0, 0
	for i, line := range lines {
		switch {
		case strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++"):
			adds++
			lines[i] = p.DiffAdd.Render(line)
		case strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---"):
			dels++
			lines[i] = p.DiffDel.Render(line)
		case strings.HasPrefix(line, "@@"):
			lines[i] = p.DiffHunk.Render(line)
		}
	}
	if adds+dels > 0 {
		lines = append([]string{p.DiffAdd.Render(fmt.Sprintf("+%d", adds)) + " " + p.DiffDel.Render(fmt.Sprintf("-%d", dels))}, lines...)
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderEditor(width int, p Palette) string {
	gate := m.driver.PendingGate()
	prompt := p.Prompt.Render("::: ")
	if gate != nil {
		prompt = p.PromptWarn.Render(" ! ") + prompt
	}
	display := m.input
	if i := strings.LastIndex(display, "\n"); i >= 0 {
		display = display[i+1:]
	}
	cursor := p.Dim.Render("█")
	if gate != nil && gate.Kind == "approval" {
		cursor = ""
	}
	rule := p.Separator.Render(strings.Repeat("─", max(1, width)))
	return p.Editor.Width(width).Render(rule + "\n" + truncate(prompt+display+cursor, width))
}

func (m Model) renderHelp(l layout, p Palette) string {
	meta := m.driver.Meta()
	selector := " sessions"
	if l.showSidebar {
		selector = " Sessions"
	}
	parts := []string{
		p.HelpKey.Render("^s") + p.HelpDesc.Render(selector),
		p.HelpKey.Render("enter") + p.HelpDesc.Render(" send"),
		p.HelpKey.Render("y/n") + p.HelpDesc.Render(" approve"),
		p.HelpKey.Render("^n") + p.HelpDesc.Render(" new"),
		p.HelpKey.Render("^y") + p.HelpDesc.Render(" permission"),
		p.HelpKey.Render("esc") + p.HelpDesc.Render(" cancel"),
		p.HelpKey.Render("^c") + p.HelpDesc.Render(" quit"),
	}
	footer := meta.Footer
	if footer == "" {
		if meta.Mode == "live" {
			footer = "live"
			if meta.Host != "" {
				footer += " · " + meta.Host
			}
			if meta.Busy {
				footer += " · run…"
			}
		} else {
			footer = "demo · not connected"
		}
	}
	if meta.Error != "" {
		footer = "err · " + meta.Error
	}
	parts = append(parts, p.HelpDesc.Render("· "+footer))
	return p.Status.Width(l.width).Render(truncate(" "+strings.Join(parts, p.HelpDesc.Render("  ")), l.width))
}

func (m Model) renderGateDialog(gate *surface.Gate, l layout, p Palette) string {
	kind := gate.Kind
	if kind == "" || kind == "approval" {
		kind = "permission"
	}
	title := p.DialogTitle.Render(kind + "  ·  " + gate.Title)
	body := p.DialogBody.Render(gate.Body)
	help := p.DialogFooter.Render("y approve    n deny")
	if gate.Kind == "question" {
		help = p.DialogFooter.Render("type answer · enter submit")
	}
	if gate.Submitting {
		help = p.DialogFooter.Render("submitting…")
	}
	inner := lipgloss.JoinVertical(lipgloss.Left, title, "", body, "", help)
	w := max(1, min(l.width-6, 64))
	return p.Dialog.Width(w).Render(inner)
}

func (m Model) renderSessionsDialog(l layout, p Palette) string {
	rows := m.filteredSessions()
	title := p.DialogTitle.Render("Sessions")
	filter := "filter: " + m.sessionFilter
	if m.sessionFilter == "" {
		filter = "filter title…"
	}
	lines := []string{title, p.DialogFooter.Render(filter)}
	if m.sessionLoading {
		lines = append(lines, "", p.DialogFooter.Render("loading sessions…"))
	} else if len(rows) == 0 {
		lines = append(lines, "", p.DialogFooter.Render("no matching sessions"))
	} else {
		lines = append(lines, "")
		windowRows := max(1, (max(4, l.height-12))/2)
		start := max(0, m.sessionCursor-windowRows/2)
		if start+windowRows > len(rows) {
			start = max(0, len(rows)-windowRows)
		}
		end := min(len(rows), start+windowRows)
		for i := start; i < end; i++ {
			row := rows[i]
			marker := "  "
			style := p.Idle
			if i == m.sessionCursor {
				marker = "▸ "
				style = p.Active
			}
			name := strings.TrimSpace(row.Title)
			if name == "" {
				name = "untitled session"
			}
			lines = append(lines, style.Render(truncate(marker+name, max(8, l.width-14))))
			lines = append(lines, p.Dim.Render(truncate("   "+row.ID, max(8, l.width-14))))
		}
	}
	lines = append(lines, "")
	if m.sessionRenaming {
		lines = append(lines, p.DialogFooter.Render("rename: "+m.sessionRenameInput+"█"), p.DialogFooter.Render("enter confirm · esc cancel"))
	} else if m.sessionDeleteID != "" {
		name := m.sessionDeleteID
		for _, row := range rows {
			if row.ID == m.sessionDeleteID {
				name = row.Title
				break
			}
		}
		lines = append(lines, p.PromptWarn.Render(truncate("delete "+name+"?", max(8, l.width-14))), p.DialogFooter.Render("y delete · n/esc cancel"))
	} else {
		lines = append(lines, p.DialogFooter.Render("↑/↓ move · enter/tab choose · ^r rename · ^x delete"))
	}
	if m.sessionError != "" {
		lines = append(lines, p.PromptWarn.Render(truncate("! "+m.sessionError, max(8, l.width-14))))
	}
	inner := strings.Join(lines, "\n")
	w := max(1, min(l.width-8, 72))
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
		if w := lipgloss.Width(lines[i]); w < totalWidth {
			lines[i] += strings.Repeat(" ", totalWidth-w)
		}
	}
	return strings.Join(lines, "\n")
}

func fitHeight(content string, width, height int) string {
	return strings.Join(padLines(strings.Split(content, "\n"), width, height), "\n")
}

func placeOverlay(base, overlay string, width, height int) string {
	baseLines := padLines(strings.Split(base, "\n"), width, height)
	overLines := strings.Split(overlay, "\n")
	ow := 0
	for _, line := range overLines {
		ow = max(ow, lipgloss.Width(line))
	}
	row := max(0, (height-len(overLines))/2)
	col := max(0, (width-ow)/2)
	for i, line := range overLines {
		r := row + i
		if r >= 0 && r < len(baseLines) {
			baseLines[r] = overlayLine(baseLines[r], line, col, width)
		}
	}
	return strings.Join(baseLines, "\n")
}

func overlayLine(base, over string, col, width int) string {
	plain := stripForPad(base)
	if lipgloss.Width(plain) < width {
		plain += strings.Repeat(" ", width-lipgloss.Width(plain))
	}
	runes := []rune(plain)
	overPlain := stripForPad(over)
	overRunes := []rune(overPlain)
	for i := 0; i < len(overRunes) && col+i < len(runes); i++ {
		runes[col+i] = overRunes[i]
	}
	if col == 0 && lipgloss.Width(over) >= width {
		return over
	}
	left := string(runes[:min(col, len(runes))])
	rightStart := min(len(runes), col+lipgloss.Width(over))
	return left + over + string(runes[rightStart:])
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
	return strings.Join(padLines(strings.Split(content, "\n"), width, height), "\n")
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
	var lines []string
	for _, para := range strings.Split(text, "\n") {
		words := strings.Fields(para)
		if len(words) == 0 {
			lines = append(lines, "")
			continue
		}
		cur := ""
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
	if width == 1 {
		return "…"
	}
	runes := []rune(s)
	for len(runes) > 0 && lipgloss.Width(string(runes)+"…") > width {
		runes = runes[:len(runes)-1]
	}
	return string(runes) + "…"
}

func padRight(s string, width int) string {
	if w := lipgloss.Width(s); w < width {
		return s + strings.Repeat(" ", width-w)
	}
	return s
}

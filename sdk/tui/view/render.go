package view

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/rivo/uniseg"

	"agent-vivy/sdk/tui/surface"
)

const headerDiag = "╱"

func (m Model) renderFrame() string {
	l := m.layout()
	p := m.palette

	var app string
	if l.showSidebar {
		app = m.renderWide(l, p)
	} else {
		app = m.renderCompact(l, p)
	}
	frame := lipgloss.JoinVertical(lipgloss.Left, strings.Repeat(" ", l.width), app)
	frame = fitHeight(frame, l.width, l.height)

	if gate := m.driver.PendingGate(); gate != nil {
		return placeOverlay(frame, m.renderGateDialog(gate, l, p), l.width, l.height)
	}
	if m.modelPickerOpen {
		return placeOverlay(frame, m.renderModelDialog(l, p), l.width, l.height)
	}
	if m.commandPaletteOpen {
		return placeOverlay(frame, m.renderCommandPalette(l, p), l.width, l.height)
	}
	if m.shortcutsOpen {
		return placeOverlay(frame, m.renderShortcutsDialog(l, p), l.width, l.height)
	}
	if m.dynamicArgumentCommand != nil {
		return placeOverlay(frame, m.renderDynamicArguments(l, p), l.width, l.height)
	}
	if m.fileCompletionOpen {
		return placeOverlay(frame, m.renderFileCompletion(l, p), l.width, l.height)
	}
	if m.sessionsOpen {
		return placeOverlay(frame, m.renderSessionsDialog(l, p), l.width, l.height)
	}
	if m.commandConfirmName != "" || m.commandOverlay != "" {
		return placeOverlay(frame, m.renderCommandDialog(l, p), l.width, l.height)
	}
	return frame
}

func (m Model) renderDynamicArguments(l layout, p Palette) string {
	command := m.dynamicArgumentCommand
	if command == nil {
		return ""
	}
	w := max(1, min(l.width-8, 72))
	lineWidth := max(1, w-p.Dialog.GetHorizontalFrameSize())
	lines := []string{p.DialogTitle.Render("/" + command.Name + " 参数"), p.Dim.Render(truncate(command.Description, lineWidth)), ""}
	for i, argument := range command.Arguments {
		marker := "  "
		style := p.Idle
		if i == m.dynamicArgumentCursor {
			marker, style = "▸ ", p.Active
		}
		required := "可选"
		if argument.Required {
			required = "必填"
		}
		value := ""
		if i < len(m.dynamicArgumentValues) {
			value = m.dynamicArgumentValues[i]
		}
		lines = append(lines, style.Render(truncate(marker+argument.Name+" ("+required+"): "+value, lineWidth)))
		if argument.Description != "" {
			lines = append(lines, p.Dim.Render(truncate("   "+argument.Description, lineWidth)))
		}
	}
	if m.dynamicArgumentError != "" {
		lines = append(lines, "", p.PromptWarn.Render(truncate(m.dynamicArgumentError, lineWidth)))
	}
	lines = append(lines, "", p.DialogFooter.Render("tab/↑/↓ 字段 · enter 运行 · esc 保留草稿"))
	return p.Dialog.Width(w).Render(strings.Join(lines, "\n"))
}

func (m Model) renderModelDialog(l layout, p Palette) string {
	rows := m.filteredModels()
	w := max(1, min(l.width-8, 72))
	innerWidth := max(1, w-p.Dialog.GetHorizontalFrameSize())
	status := "筛选：" + sanitizeCommandPaletteFilter(m.modelPickerFilter)
	if strings.TrimSpace(m.modelPickerFilter) == "" {
		status = "筛选：全部"
	}
	if m.modelPickerLoading {
		status += "  · 加载中…"
	} else if m.modelPickerSelecting {
		status += "  · 正在应用…"
	}
	lines := []string{p.DialogTitle.Render("切换全局模型"), p.DialogFooter.Render(truncate(status, innerWidth)), ""}
	if m.modelPickerError != "" {
		lines = append(lines, p.ToolFail.Render(truncate(m.modelPickerError, innerWidth)))
	} else if !m.modelPickerLoading && len(rows) == 0 {
		lines = append(lines, p.DialogFooter.Render("没有匹配的已配置模型"))
	} else {
		windowRows := max(1, l.height-11)
		cursor := min(max(0, m.modelPickerCursor), max(0, len(rows)-1))
		start := max(0, cursor-windowRows/2)
		if start+windowRows > len(rows) {
			start = max(0, len(rows)-windowRows)
		}
		end := min(len(rows), start+windowRows)
		for i := start; i < end; i++ {
			option := rows[i]
			marker := "  "
			if option.Current {
				marker = "● "
			}
			provider := safeModelLabel(option.DisplayName)
			if provider == "" {
				provider = safeModelLabel(option.Provider)
			}
			line := marker + provider + " · " + safeModelLabel(option.Model)
			style := p.Idle
			if i == cursor {
				line = "▸ " + strings.TrimPrefix(line, "  ")
				style = p.Active
			}
			lines = append(lines, style.Render(truncate(line, innerWidth)))
		}
	}
	catalog := m.driver.ModelCatalog()
	footer := "全局 · 下一空闲回合生效 · ↑↓ 选择 · esc 关闭"
	if catalog.ReadOnly || catalog.Frozen {
		footer = "只读 · ↑↓ 浏览 · 输入筛选 · esc 关闭"
	}
	lines = append(lines, "", p.DialogFooter.Render(truncate(footer, innerWidth)))
	return p.Dialog.Width(w).Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

func safeModelLabel(text string) string {
	return strings.TrimSpace(sanitizeFileCompletionText(text))
}

func (m Model) renderFileCompletion(l layout, p Palette) string {
	rows := m.filteredProjectFiles()
	w := max(1, min(l.width-8, 72))
	compact := l.height < 16
	query := sanitizeFileCompletionText(m.fileCompletionQuery)
	status := "@" + query
	if m.fileCompletionLoading {
		status += "  loading…"
	} else if m.fileCompletionTruncated {
		status += "  partial results"
	}
	lines := []string{p.DialogTitle.Render("项目文件"), p.DialogFooter.Render(truncate(status, max(1, w-p.Dialog.GetHorizontalFrameSize())))}
	if !compact {
		lines = append(lines, "")
	}
	if m.fileCompletionError != "" {
		lines = append(lines, p.ToolFail.Render(truncate(m.fileCompletionError, max(1, w-p.Dialog.GetHorizontalFrameSize()))))
	} else if !m.fileCompletionLoading && len(rows) == 0 {
		lines = append(lines, p.DialogFooter.Render("没有匹配的项目文件"))
	} else {
		windowRows := max(1, l.height-10)
		if compact {
			windowRows = max(1, l.height-6)
		}
		cursor := min(max(0, m.fileCompletionCursor), max(0, len(rows)-1))
		start := max(0, cursor-windowRows/2)
		if start+windowRows > len(rows) {
			start = max(0, len(rows)-windowRows)
		}
		end := min(len(rows), start+windowRows)
		lineWidth := max(1, w-p.Dialog.GetHorizontalFrameSize())
		for i := start; i < end; i++ {
			marker := "  "
			style := p.Idle
			if i == cursor {
				marker = "▸ "
				style = p.Active
			}
			lines = append(lines, style.Render(truncate(marker+safeProjectFilePath(rows[i].Path), lineWidth)))
		}
	}
	if !compact {
		lines = append(lines, "")
	}
	footer := "↑/↓ 移动 · enter/tab 选择 · esc 关闭"
	if compact {
		footer = "↑/↓ · enter/tab · esc"
	}
	lines = append(lines, p.DialogFooter.Render(footer))
	return p.Dialog.Width(w).Render(strings.Join(lines, "\n"))
}

const maxFileCompletionRunes = 512

func sanitizeFileCompletionText(text string) string {
	text = ansi.Strip(text)
	clean := make([]rune, 0, min(len([]rune(text)), maxFileCompletionRunes))
	for _, r := range text {
		if unicode.IsControl(r) || isBidiControl(r) {
			continue
		}
		clean = append(clean, r)
		if len(clean) == maxFileCompletionRunes {
			break
		}
	}
	return string(clean)
}

func isBidiControl(r rune) bool {
	return r == '\u061c' || r == '\u200e' || r == '\u200f' || (r >= '\u202a' && r <= '\u202e') || (r >= '\u2066' && r <= '\u2069')
}

func safeProjectFilePath(path string) string {
	clean := sanitizeFileCompletionText(path)
	if clean == "" || clean != path {
		return ""
	}
	if strings.HasPrefix(clean, "/") || strings.HasPrefix(clean, `\`) || strings.Contains(clean, `\`) || strings.Contains(clean, ":") || strings.HasSuffix(clean, "/") || strings.Contains(clean, "//") {
		return ""
	}
	for _, part := range strings.Split(clean, "/") {
		if part == "" || part == "." || part == ".." {
			return ""
		}
	}
	return clean
}

func shortCompletionError(err error) string {
	if err == nil {
		return ""
	}
	return truncate(sanitizeFileCompletionText(err.Error()), 120)
}

const maxCommandPaletteFilterRunes = 128

func sanitizeCommandPaletteFilter(text string) string {
	text = ansi.Strip(text)
	clean := make([]rune, 0, min(len([]rune(text)), maxCommandPaletteFilterRunes))
	for _, r := range text {
		if unicode.IsControl(r) || r == '\u061c' || r == '\u200e' || r == '\u200f' || (r >= '\u202a' && r <= '\u202e') || (r >= '\u2066' && r <= '\u2069') {
			continue
		}
		clean = append(clean, r)
		if len(clean) == maxCommandPaletteFilterRunes {
			break
		}
	}
	return string(clean)
}

func (m Model) renderWide(l layout, p Palette) string {
	chat := m.renderChat(l.mainW(), l.mainH(), p)
	editor := m.renderEditor(l.mainW(), p)
	chrome := m.renderInputChrome(l.mainW(), p)
	mainCol := lipgloss.JoinVertical(lipgloss.Left, chat, "", editor, chrome)
	side := m.renderSidebar(l.sidebarW, lipgloss.Height(mainCol), p)
	gap := lipgloss.NewStyle().Width(1).Height(lipgloss.Height(mainCol)).Render(" ")
	row := lipgloss.JoinHorizontal(lipgloss.Top, mainCol, gap, side)
	return padHorizontal(row, l.marginX, l.width)
}

func (m Model) renderCompact(l layout, p Palette) string {
	header := m.renderCompactHeader(l, p)
	chat := m.renderChat(l.innerW(), l.mainH(), p)
	editor := m.renderEditor(l.innerW(), p)
	chrome := m.renderInputChrome(l.innerW(), p)
	col := lipgloss.JoinVertical(lipgloss.Left, header, "", chat, "", editor, chrome)
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
	if meta.Host != "" {
		label = fmt.Sprintf("%s · %s", label, meta.Host)
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
	lines := m.sidebarLines(width, p)
	logoLines := sidebarLogoLines(p)
	viewport := max(1, height-len(logoLines))
	maxScroll := max(0, len(lines)-viewport)
	offset := min(max(0, m.sidebarScroll), maxScroll)
	end := min(len(lines), offset+viewport)
	if offset < end {
		lines = lines[offset:end]
	} else {
		lines = nil
	}
	box := strings.Join(append(logoLines, lines...), "\n")
	return p.Sidebar.Width(width).Height(height).MaxHeight(height).Render(padBlock(box, width, height))
}

func sidebarLogoLines(p Palette) []string {
	return []string{p.SidebarLogo.Render(" VIVY CODE"), p.Dim.Render(" ─────────────")}
}

func (m Model) sidebarLines(width int, p Palette) []string {
	active := m.driver.Active()
	snapshot := m.driver.Sidebar()
	if snapshot.Session.ID == "" {
		snapshot.Session = active
	}

	lines := make([]string, 0, 24)
	title := strings.TrimSpace(snapshot.Session.Title)
	if title == "" {
		title = "untitled session"
	}
	titleLines := wrapText(title, max(8, width-2))
	if len(titleLines) > 2 {
		titleLines = titleLines[:2]
	}
	for _, line := range titleLines {
		lines = append(lines, p.Active.Render(" "+line))
	}
	if updated := sidebarTime(snapshot.Session.UpdatedAt); updated != "" {
		lines = append(lines, p.Dim.Render(truncate(" updated · "+updated, width-1)))
	}
	if cwd := strings.TrimSpace(snapshot.CWD); cwd != "" {
		lines = append(lines, p.Dim.Render(truncate(" cwd · "+cwd, width-1)))
	}
	if host := strings.TrimSpace(m.driver.Meta().Host); host != "" {
		lines = append(lines, p.Active.Render(truncate(" host · "+host, width-1)))
	}
	if snapshot.ReasoningKnown {
		reasoning := "unsupported"
		if snapshot.ReasoningSupported {
			reasoning = "supported"
		}
		lines = append(lines, p.Dim.Render(truncate(" reasoning · "+reasoning, width-1)))
	}
	if snapshot.HasContext && snapshot.Context.ThinkingSupported {
		lines = append(lines, p.Dim.Render(truncate(" draft thinking · "+m.driver.ThinkingMode(), width-1)))
	}
	if snapshot.HasContext {
		lines = append(lines, "", p.Dim.Render(" Context"))
		ctx := snapshot.Context
		switch {
		case ctx.ModelLimitKnown && ctx.ModelLimitTokens > 0:
			ratio := float64(ctx.FeedTokens) / float64(ctx.ModelLimitTokens)
			percentage := int(ratio * 100)
			estimated := ""
			if ctx.TokenCountsEstimated {
				estimated = "~"
			}
			line := fmt.Sprintf(" %s%d%% · %s%s / %s tokens", estimated, percentage, estimated, compactNumber(ctx.FeedTokens), compactNumber(ctx.ModelLimitTokens))
			style := contextPercentStyle(ratio, p)
			if ratio > 0.8 {
				line = " !" + line
			}
			lines = append(lines, style.Render(truncate(line, width-1)))
		case ctx.FeedTokens > 0:
			estimated := ""
			if ctx.TokenCountsEstimated {
				estimated = "~"
			}
			lines = append(lines, p.Dim.Render(truncate(fmt.Sprintf(" %s%s tokens · limit unknown", estimated, compactNumber(ctx.FeedTokens)), width-1)))
		}
		if ctx.TotalMessages > 0 {
			if ctx.FeedMessages > 0 && ctx.FeedMessages != ctx.TotalMessages {
				lines = append(lines, p.Dim.Render(truncate(fmt.Sprintf(" %d / %d feed messages", ctx.FeedMessages, ctx.TotalMessages), width-1)))
			} else {
				lines = append(lines, p.Dim.Render(truncate(fmt.Sprintf(" %d messages", ctx.TotalMessages), width-1)))
			}
		}
		if ctx.TriggerTokens > 0 {
			estimated := ""
			if ctx.TokenCountsEstimated {
				estimated = "~"
			}
			lines = append(lines, p.Dim.Render(truncate(fmt.Sprintf(" compact at %s%s", estimated, compactNumber(ctx.TriggerTokens)), width-1)))
		}
		if ctx.CompactionEnabled {
			compaction := " compaction on"
			if ctx.WouldCompact {
				compaction = " compaction needed"
			}
			lines = append(lines, p.Dim.Render(compaction))
		}
		if ctx.HasCompactionSummary {
			lines = append(lines, p.Dim.Render(" summary available"))
		}
	}
	if snapshot.HasUsage {
		lines = append(lines, "", p.Dim.Render(" Session Usage"))
		usage := snapshot.Usage
		lines = append(lines,
			p.Dim.Render(truncate(fmt.Sprintf(" total · %s tokens", compactNumber(usage.TotalTokens)), width-1)),
			p.Dim.Render(truncate(fmt.Sprintf(" input · %s", compactNumber(usage.PromptTokens)), width-1)),
			p.Dim.Render(truncate(fmt.Sprintf(" output · %s", compactNumber(usage.CompletionTokens)), width-1)),
		)
		if usage.ReasoningTokens > 0 {
			lines = append(lines, p.Dim.Render(truncate(fmt.Sprintf(" reasoning · %s", compactNumber(usage.ReasoningTokens)), width-1)))
		}
		if usage.CachedTokens > 0 {
			lines = append(lines, p.Dim.Render(truncate(fmt.Sprintf(" cached · %s", compactNumber(usage.CachedTokens)), width-1)))
		}
		lines = append(lines, p.Dim.Render(truncate(fmt.Sprintf(" requests · %d", usage.RequestCount), width-1)))
		cost := "unknown"
		if usage.CostKnown {
			cost = fmt.Sprintf("$%.4f", usage.CostUSD)
		}
		lines = append(lines, p.Dim.Render(truncate(" est. cost · "+cost, width-1)))
	}
	if snapshot.ModifiedFilesKnown {
		lines = append(lines, "", p.Dim.Render(" Modified Files"))
		if len(snapshot.ModifiedFiles) == 0 {
			lines = append(lines, p.Dim.Render(" None"))
		}
		for _, file := range snapshot.ModifiedFiles {
			path := strings.TrimSpace(safeProjectFilePath(file.Path))
			if path == "" {
				continue
			}
			line := fmt.Sprintf(" %s  +%d -%d", path, file.Diff.Additions, file.Diff.Deletions)
			if updated := sidebarTime(file.UpdatedAt); updated != "" {
				line += " · " + updated
			}
			lines = append(lines, p.Dim.Render(truncate(line, width-1)))
		}
	}
	if snapshot.LSPKnown {
		lspLines := make([]string, 0, len(snapshot.LSP))
		for _, server := range snapshot.LSP {
			language := strings.TrimSpace(sanitizeFileCompletionText(server.Language))
			if language == "" {
				continue
			}
			state := server.State
			style := p.Dim
			switch state {
			case "starting":
			case "initialized":
				state = "initialized"
				style = p.Active
			default:
				continue
			}
			lspLines = append(lspLines, style.Render(truncate(" "+language+" · "+state, width-1)))
		}
		lines = append(lines, "", p.Dim.Render(" LSP · live"))
		if len(lspLines) == 0 {
			lines = append(lines, p.Dim.Render(" None initialized"))
		} else {
			lines = append(lines, lspLines...)
		}
	}
	if snapshot.MCPKnown {
		lines = append(lines, "", p.Dim.Render(" MCP"))
		if len(snapshot.MCP) == 0 {
			lines = append(lines, p.Dim.Render(" None configured"))
		}
		for _, server := range snapshot.MCP {
			name := strings.TrimSpace(sanitizeFileCompletionText(server.Name))
			if name == "" {
				continue
			}
			state := "configured"
			style := p.Dim
			if server.State == "initialized" {
				state = "initialized"
				style = p.Active
			}
			line := " " + name + " · " + state
			lines = append(lines, style.Render(truncate(line, width-1)))
		}
	}
	if snapshot.SkillsKnown {
		lines = append(lines, "", p.Dim.Render(" Skills · enabled"))
		if len(snapshot.Skills) == 0 {
			lines = append(lines, p.Dim.Render(" None"))
		}
		for _, skill := range snapshot.Skills {
			name := strings.TrimSpace(sanitizeFileCompletionText(skill.Name))
			if name != "" {
				lines = append(lines, p.Dim.Render(truncate(" "+name, width-1)))
			}
		}
	}
	meta := m.driver.Meta()
	if meta.Busy {
		lines = append(lines, "", p.Dim.Render(truncate(" run · "+fallback(meta.RunID, "active"), width-1)))
	}
	if meta.Queued > 0 {
		lines = append(lines, p.Dim.Render(truncate(" queue · "+fmt.Sprintf("%d", meta.Queued), width-1)))
	}
	if errText := strings.TrimSpace(meta.Error); errText != "" {
		lines = append(lines, "", p.PromptWarn.Render(truncate(" ! "+errText, width-1)))
	}
	return lines
}

func sidebarTime(timestamp int64) string {
	if timestamp <= 0 {
		return ""
	}
	return time.UnixMilli(timestamp).Format("2006-01-02 15:04")
}

func fallback(value, otherwise string) string {
	if strings.TrimSpace(value) == "" {
		return otherwise
	}
	return value
}

func compactNumber(n int) string {
	if n >= 1000 {
		return fmt.Sprintf("%.1fk", float64(n)/1000)
	}
	return fmt.Sprintf("%d", n)
}

func (m Model) renderChat(width, height int, p Palette) string {
	lines := m.chatLines(width, p)
	maxScroll := max(0, len(lines)-height)
	offset := min(max(0, m.chatScroll), maxScroll)
	if m.chatFollow {
		offset = maxScroll
	}
	end := min(len(lines), offset+height)
	if offset < end {
		lines = lines[offset:end]
	} else {
		lines = nil
	}
	content := strings.Join(lines, "\n")
	return p.Chat.Width(width).MaxWidth(width).Height(height).MaxHeight(height).Render(padBlock(content, width, height))
}

func (m Model) chatLines(width int, p Palette) []string {
	messages := m.driver.ActiveMessages()
	m.mdCache.ensure(m.driver.Active().ID, width)
	var lines []string
	if len(messages) == 0 {
		lines = append(lines, p.Dim.Render(""), p.LogoWord.Render(" 寻找真心之旅"), p.Dim.Render(" empty session · type to draft"))
	}
	for index, message := range messages {
		rendered, ok := m.mdCache.get(message, width)
		if !ok {
			rendered = renderMessageWithOptions(message, width, p, m.debugToolOutput)
			m.mdCache.put(message, width, rendered)
		}
		lines = append(lines, rendered...)
		if index < len(messages)-1 {
			lines = append(lines, "")
		}
	}
	return lines
}

func renderMessage(message surface.Message, width int, p Palette) []string {
	return renderMessageWithOptions(message, width, p, false)
}

func renderMessageWithOptions(message surface.Message, width int, p Palette, debugToolOutput bool) []string {
	if message.Tool != nil {
		return renderToolWithOptions(message.Tool, width, p, debugToolOutput)
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
	contentWidth := width - lipgloss.Width(bar)
	if contentWidth < 1 {
		bar = ""
		contentWidth = max(1, width)
	}

	bodyLines, painted := renderMessageBody(message.Content, contentWidth, message.Reasoning)
	appendChipLines := func(chips string) {
		if chips == "" {
			return
		}
		for _, line := range wrapText(chips, contentWidth) {
			if painted {
				bodyLines = append(bodyLines, style.Render(line))
			} else {
				bodyLines = append(bodyLines, line)
			}
		}
	}
	appendChipLines(renderAttachmentChips(message.Attachments))
	appendChipLines(renderFileContextChips(message.FileContexts))
	if len(bodyLines) == 0 {
		bodyLines = []string{""}
	}
	if message.Streaming {
		bodyLines[len(bodyLines)-1] += "▌"
	}
	out := make([]string, 0, len(bodyLines))
	for _, line := range bodyLines {
		paintedLine := line
		if !painted {
			paintedLine = style.Render(line)
		}
		out = append(out, ansi.Truncate(bar+paintedLine, max(1, width), "…"))
	}
	return out
}

func renderMessageBody(content string, contentWidth int, quiet bool) ([]string, bool) {
	source := sanitizeMarkdownSource(content)
	if source == "" {
		return nil, true
	}
	wrapWidth := markdownWrapWidth(contentWidth)
	rendered, err := renderMarkdown(source, wrapWidth, quiet)
	if err != nil {
		return wrapText(source, contentWidth), false
	}
	if rendered == "" {
		return wrapText(source, contentWidth), false
	}
	lines := strings.Split(rendered, "\n")
	for i, line := range lines {
		if lipgloss.Width(line) > contentWidth {
			lines[i] = ansi.Truncate(line, contentWidth, "…")
		}
	}
	return lines, true
}

func renderTool(tool *surface.ToolCard, width int, p Palette) []string {
	return renderToolWithOptions(tool, width, p, false)
}

const compactToolResultLines = 8

func renderToolWithOptions(tool *surface.ToolCard, width int, p Palette, debugToolOutput bool) []string {
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
	name := strings.TrimSpace(sanitizeFileCompletionText(tool.ToolName))
	if name == "" {
		name = "tool"
	}
	status := strings.TrimSpace(sanitizeFileCompletionText(tool.Status))
	title := fmt.Sprintf("%s %s  %s", icon, name, status)
	body := tool.Preview
	if tool.Status != "pending" && tool.Result != "" {
		body = tool.Result
	}

	const indent = 2
	available := max(1, width-indent)
	frame := style.GetHorizontalFrameSize()
	if available <= frame {
		lines := wrapText(title, max(1, width))
		if body != "" {
			lines = append(lines, compactToolLines(wrapText(body, max(1, width)), debugToolOutput, max(1, width))...)
		}
		return lines
	}
	contentWidth := max(1, min(52, available-frame))
	innerLines := wrapText(title, contentWidth)
	if body != "" {
		bodyLines := wrapText(body, contentWidth)
		renderedBody := strings.Split(renderDiffBody(strings.Join(bodyLines, "\n"), p), "\n")
		innerLines = append(innerLines, compactToolLines(renderedBody, debugToolOutput, contentWidth)...)
	}
	box := style.Width(contentWidth).MaxWidth(available).Render(strings.Join(innerLines, "\n"))
	indented := make([]string, 0)
	for _, line := range strings.Split(box, "\n") {
		indented = append(indented, "  "+line)
	}
	return indented
}

func compactToolLines(lines []string, debug bool, width int) []string {
	if debug || len(lines) <= compactToolResultLines {
		return lines
	}
	omitted := len(lines) - compactToolResultLines
	compact := append([]string(nil), lines[:compactToolResultLines]...)
	marker := fmt.Sprintf("… %d more lines · set tui.debug: true", omitted)
	return append(compact, wrapText(marker, max(1, width))...)
}

func renderDiffBody(body string, p Palette) string {
	if !strings.Contains(body, "@@") && !strings.Contains(body, "\n+") && !strings.Contains(body, "\n-") {
		return body
	}
	lines := strings.Split(body, "\n")
	adds, dels := visibleDiffStats(lines)
	inHunk := false
	for i, line := range lines {
		switch {
		case strings.HasPrefix(line, "@@"):
			inHunk = true
			lines[i] = p.DiffHunk.Render(line)
		case inHunk && strings.HasPrefix(line, "+"):
			lines[i] = p.DiffAdd.Render(line)
		case inHunk && strings.HasPrefix(line, "-"):
			lines[i] = p.DiffDel.Render(line)
		}
	}
	if adds+dels > 0 {
		lines = append([]string{p.DiffAdd.Render(fmt.Sprintf("+%d", adds)) + " " + p.DiffDel.Render(fmt.Sprintf("-%d", dels))}, lines...)
	}
	return strings.Join(lines, "\n")
}

// composerPlaceholder is the ghost hint shown in an empty, ungated composer.
// It is display-only: it never enters m.input.
const composerPlaceholder = "问点什么…  / 命令 · @文件 · !shell"

func (m Model) renderEditor(width int, p Palette) string {
	inner := max(1, width-p.EditorBox.GetHorizontalFrameSize())
	gate := m.driver.PendingGate()
	prompt := p.Prompt.Render("::: ")
	if gate != nil {
		prompt = p.PromptWarn.Render(" ! ") + prompt
	}
	cursor := p.Dim.Render("█")
	if gate != nil && gate.Kind == "approval" {
		cursor = ""
	}
	lines := []string{m.renderComposerChips(inner, p)}
	if chips := renderAttachmentChips(m.driver.PendingAttachments()); chips != "" {
		lines = append(lines, p.Dim.Render(truncate(chips, inner)))
	}
	if chip := pasteGuardChip(m.input); chip != "" {
		lines = append(lines, p.PromptWarn.Render(truncate(chip, inner)))
	}
	if gate != nil {
		// A pending gate turns the input row into a status hint and gate keys
		// own the composer; keep the historical single-line rendering.
		display := ansi.Strip(m.input)
		if i := strings.LastIndex(display, "\n"); i >= 0 {
			display = display[i+1:]
		}
		lines = append(lines, truncate(prompt+sanitizeFileCompletionText(display)+cursor, inner))
	} else {
		if m.input == "" && !m.sidebarFocused {
			// Ghost hint for the empty state: dim text after the prompt, no
			// caret, never stored as a draft.
			lines = append(lines, truncate(prompt+p.Dim.Render(composerPlaceholder), inner))
		} else {
			// The draft always appends at the tail, so the visible window is the
			// trailing lines and the caret rides at the end of the last one. The
			// window must come from the same helper the layout reserve uses or
			// the bottom chrome would jitter while typing.
			win := editorInputLines(m.input, inner, maxEditorLines)
			indent := strings.Repeat(" ", lipgloss.Width(prompt))
			for i, row := range win {
				row = sanitizeFileCompletionText(row)
				prefix := prompt
				if i > 0 {
					prefix = indent
				}
				if i == len(win)-1 {
					body := truncate(row, max(1, inner-lipgloss.Width(prefix)-lipgloss.Width(cursor)))
					lines = append(lines, truncate(prefix+body+cursor, inner))
				} else {
					lines = append(lines, truncate(prefix+row, inner))
				}
			}
		}
	}
	// lipgloss Width is the padded content box; the rounded border is added
	// outside it. Size the content so the final block is `width` cells.
	boxWidth := max(1, width-p.EditorBox.GetHorizontalBorderSize())
	return m.composerBoxStyle(p).Width(boxWidth).Render(strings.Join(lines, "\n"))
}

func (m Model) renderComposerChips(width int, p Palette) string {
	snapshot := m.driver.Sidebar()
	if snapshot.Session.ID == "" {
		snapshot.Session = m.driver.Active()
	}
	model := strings.TrimSpace(sanitizeFileCompletionText(snapshot.Model))
	if model == "" {
		for _, option := range m.driver.ModelCatalog().Options {
			if option.Current {
				model = safeModelLabel(option.Model)
				break
			}
		}
	}
	if model == "" {
		model = "model"
	}
	sep := p.Dim.Render("  ·  ")
	mode := workingModeLabel(m.driver.RunMode(), snapshot.Session.PermissionPreset)
	parts := []string{
		p.Dim.Render(model) + renderThinkingIntensity(m.driver.ThinkingMode(), p),
		workingModeStyle(mode, p).Render(mode),
	}
	if provider := strings.TrimSpace(sanitizeFileCompletionText(snapshot.Provider)); provider != "" {
		parts = append(parts, p.Dim.Render(provider))
	}
	if label, ratio, ok := contextPercentLabel(snapshot.Context, snapshot.HasContext); ok {
		parts = append(parts, contextPercentStyle(ratio, p).Render(label))
	}
	line := strings.Join(parts, sep)
	if lipgloss.Width(line) > width && len(parts) > 2 {
		parts = parts[:len(parts)-1]
		line = strings.Join(parts, sep)
	}
	return truncate(line, width)
}

func (m Model) renderInputChrome(width int, p Palette) string {
	hints := p.HelpKey.Render("shift+tab") + p.HelpDesc.Render(" 切换模式") + p.HelpDesc.Render("  ") + p.HelpKey.Render("shift+h") + p.HelpDesc.Render(" 帮助") + p.HelpDesc.Render("  ") + p.HelpKey.Render("ctrl+x") + p.HelpDesc.Render(" 快捷")
	if gate := m.driver.PendingGate(); gate != nil && !gate.Submitting {
		if gate.Kind == "question" {
			hints = p.HelpKey.Render("enter") + p.HelpDesc.Render(" 回答")
		} else {
			hints = p.HelpKey.Render("y/n") + p.HelpDesc.Render(" 批准")
		}
	}
	if m.sidebarFocused {
		hints = p.HelpKey.Render("esc") + p.HelpDesc.Render(" 离开侧栏")
	}
	meta := m.driver.Meta()
	line := hints
	if errText := strings.TrimSpace(meta.Error); errText != "" {
		line = p.ToolFail.Render("err · "+errText) + p.HelpDesc.Render("  ") + hints
	} else if meta.Busy {
		line = p.Dim.Render("run…") + p.HelpDesc.Render("  ") + hints
	}
	return lipgloss.NewStyle().Width(width).MaxWidth(width).MaxHeight(1).Align(lipgloss.Left).Render(truncate(line, width))
}

func (m Model) composerBoxStyle(p Palette) lipgloss.Style {
	if m.driver.Meta().Busy {
		// Busy outranks the working-mode color: the border dims to signal
		// that input queues behind the running turn, and the mode color
		// returns when the run finishes. Typing stays enabled.
		return p.EditorBox.BorderForeground(p.Dim.GetForeground())
	}
	snapshot := m.driver.Sidebar()
	if snapshot.Session.ID == "" {
		snapshot.Session = m.driver.Active()
	}
	return p.EditorBox.BorderForeground(workingModeStyle(workingModeLabel(m.driver.RunMode(), snapshot.Session.PermissionPreset), p).GetForeground())
}

func workingModeStyle(label string, p Palette) lipgloss.Style {
	switch label {
	case "计划":
		return p.ModePlan
	case "只读":
		return p.ModeRead
	default:
		return p.ModeSmart
	}
}

func renderThinkingIntensity(mode string, p Palette) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "on":
		return p.IntensityHigh.Render("(high)")
	case "auto":
		return p.IntensityAuto.Render("(auto)")
	default:
		return ""
	}
}

func contextPercentLabel(ctx surface.Context, hasContext bool) (string, float64, bool) {
	if !hasContext || !ctx.ModelLimitKnown || ctx.ModelLimitTokens <= 0 {
		return "", 0, false
	}
	ratio := float64(ctx.FeedTokens) / float64(ctx.ModelLimitTokens)
	label := fmt.Sprintf("%d%%", int(ratio*100))
	if ctx.TokenCountsEstimated {
		label = "~" + label
	}
	return label, ratio, true
}

func contextPercentStyle(ratio float64, p Palette) lipgloss.Style {
	switch {
	case ratio > 0.8:
		return p.ContextHot
	case ratio >= 0.5:
		return p.ContextMid
	default:
		return p.ContextOK
	}
}

// renderAttachmentChips is metadata-only presentation. In particular, it
// never renders a data URL or any binary payload into the terminal.
func renderAttachmentChips(attachments []surface.Attachment) string {
	if len(attachments) == 0 {
		return ""
	}
	parts := make([]string, 0, len(attachments))
	for _, attachment := range attachments {
		name := strings.TrimSpace(sanitizeFileCompletionText(attachment.Name))
		if name == "" {
			name = strings.TrimSpace(sanitizeFileCompletionText(attachment.Path))
		}
		if name == "" {
			name = "image"
		}
		parts = append(parts, "[image: "+name+"]")
	}
	return strings.Join(parts, " ")
}

// renderFileContextChips renders only bounded metadata returned by the
// control plane. Context contents are never printed as part of a history
// bubble or an editor draft.
func renderFileContextChips(contexts []surface.FileContext) string {
	if len(contexts) == 0 {
		return ""
	}
	parts := make([]string, 0, len(contexts))
	for _, context := range contexts {
		name := strings.TrimSpace(sanitizeFileCompletionText(context.Name))
		if name == "" {
			name = strings.TrimSpace(sanitizeFileCompletionText(context.Path))
		}
		if name == "" {
			name = "file"
		}
		parts = append(parts, "[file: "+name+"]")
	}
	return strings.Join(parts, " ")
}

func (m Model) renderShortcutsDialog(l layout, p Palette) string {
	w := max(1, min(l.width-8, 56))
	inner := max(1, w-p.Dialog.GetHorizontalFrameSize())
	rows := []string{
		p.DialogTitle.Render("快捷方式"),
		"",
		p.HelpKey.Render("shift+tab") + p.DialogBody.Render("  切换模式"),
		p.HelpKey.Render("shift+h") + p.DialogBody.Render("    帮助"),
		p.HelpKey.Render("/") + p.DialogBody.Render("          命令面板"),
		p.HelpKey.Render("ctrl+p") + p.DialogBody.Render("     命令面板"),
		p.HelpKey.Render("ctrl+s") + p.DialogBody.Render("     会话"),
		p.HelpKey.Render("ctrl+n") + p.DialogBody.Render("     新建会话"),
		p.HelpKey.Render("ctrl+l") + p.DialogBody.Render("     全局模型"),
		p.HelpKey.Render("ctrl+y") + p.DialogBody.Render("     权限档"),
		p.HelpKey.Render("ctrl+t") + p.DialogBody.Render("     思考档"),
		p.HelpKey.Render("enter") + p.DialogBody.Render("      发送"),
		p.HelpKey.Render("y/n") + p.DialogBody.Render("        批准 / 拒绝"),
		p.HelpKey.Render("esc") + p.DialogBody.Render("        取消 / 关闭"),
		p.HelpKey.Render("pgup/pgdn") + p.DialogBody.Render("  滚动聊天"),
		p.HelpKey.Render("ctrl+→") + p.DialogBody.Render("     侧栏"),
		p.HelpKey.Render("ctrl+c") + p.DialogBody.Render("     退出"),
		"",
		p.DialogFooter.Render("esc / ctrl+x 关闭"),
	}
	for i, row := range rows {
		rows[i] = truncate(row, inner)
	}
	return p.Dialog.Width(w).Render(strings.Join(rows, "\n"))
}

func (m Model) renderGateDialog(gate *surface.Gate, l layout, p Palette) string {
	kind := gate.Kind
	if kind == "" || kind == "approval" {
		kind = "permission"
	}
	w := m.gateDialogWidth(gate, l)
	innerWidth := max(1, w-p.Dialog.GetHorizontalFrameSize())
	titleText := kind + "  ·  " + sanitizeInline(gate.Title)
	title := p.DialogTitle.Render(truncate(titleText, innerWidth))
	lines := []string{title}
	if meta := renderGateMetadata(gate, innerWidth, p); len(meta) > 0 {
		lines = append(lines, meta...)
	}
	lines = append(lines, "")
	bodyLines := m.gateBodyLines(gate, l, p, innerWidth)
	viewport := m.gateViewportHeight(gate, l)
	start := min(max(0, m.gateScroll), max(0, len(bodyLines)-viewport))
	end := min(len(bodyLines), start+viewport)
	if len(bodyLines) == 0 {
		bodyLines = []string{p.DialogBody.Render("No preview supplied.")}
		start, end = 0, 1
	}
	lines = append(lines, bodyLines[start:end]...)
	if len(bodyLines) > viewport {
		lines = append(lines, p.DialogFooter.Render(fmt.Sprintf("lines %d–%d of %d", start+1, end, len(bodyLines))))
	}
	lines = append(lines, "")
	helpText := "enter/y approve · esc/n deny"
	if isApprovalDiff(gate) {
		mode := "unified"
		if m.gateUsesSplit(gate, l) {
			mode = "split"
		}
		helpText = mode + " · t view · f fullscreen · ↑↓/pg scroll · enter/y approve · esc/n deny"
	}
	help := p.DialogFooter.Render(truncate(helpText, innerWidth))
	if gate.Kind == "question" {
		help = p.DialogFooter.Render("type answer · enter submit")
	}
	if gate.Submitting {
		help = p.DialogFooter.Render("submitting…")
	}
	lines = append(lines, help)
	inner := lipgloss.JoinVertical(lipgloss.Left, lines...)
	return p.Dialog.Width(w).Render(inner)
}

func renderGateMetadata(gate *surface.Gate, width int, p Palette) []string {
	var lines []string
	if action := sanitizeInline(gate.Action); action != "" {
		lines = append(lines, p.DialogFooter.Render(truncate("action: "+action, width)))
	}
	if target := sanitizeApprovalTarget(gate.Target); target != "" {
		lines = append(lines, p.DialogFooter.Render(truncate("target: "+target, width)))
	}
	if hash := shortPreconditionHash(gate.PreconditionHash); hash != "" {
		lines = append(lines, p.DialogFooter.Render("base: "+hash))
	}
	for i, risk := range gate.Risks {
		if i == 3 {
			lines = append(lines, p.ToolFail.Render(fmt.Sprintf("warning: %d more findings", len(gate.Risks)-i)))
			break
		}
		if risk = sanitizeInline(risk); risk != "" {
			lines = append(lines, p.ToolFail.Render(truncate("warning: "+risk, width)))
		}
	}
	return lines
}

func shortPreconditionHash(hash string) string {
	hash = strings.TrimSpace(hash)
	if len(hash) != 64 {
		return ""
	}
	for _, r := range hash {
		if !strings.ContainsRune("0123456789abcdefABCDEF", r) {
			return ""
		}
	}
	return strings.ToLower(hash[:12]) + "…"
}

func sanitizeInline(text string) string {
	return strings.TrimSpace(strings.ReplaceAll(sanitizeMultilineText(text), "\n", " "))
}

func sanitizeApprovalTarget(text string) string {
	target := sanitizeInline(text)
	if strings.Contains(target, "\\") || strings.Contains(target, ":") || strings.HasPrefix(target, "/") {
		return "[redacted target]"
	}
	return target
}

const maxGatePreviewRunes = 64 * 1024

func sanitizeMultilineText(text string) string {
	text = strings.ReplaceAll(strings.ReplaceAll(ansi.Strip(text), "\r\n", "\n"), "\r", "\n")
	clean := make([]rune, 0, min(len([]rune(text)), maxGatePreviewRunes))
	for _, r := range text {
		if r == '\n' {
			clean = append(clean, r)
		} else if r == '\t' {
			clean = append(clean, ' ', ' ', ' ', ' ')
		} else if !unicode.IsControl(r) && !isBidiControl(r) {
			clean = append(clean, r)
		}
		if len(clean) >= maxGatePreviewRunes {
			break
		}
	}
	return string(clean)
}

func isUnifiedDiff(preview string) bool {
	preview = sanitizeMultilineText(preview)
	return strings.Contains(preview, "\n--- ") && strings.Contains(preview, "\n+++ ") && strings.Contains(preview, "\n@@") ||
		strings.HasPrefix(preview, "--- ") && strings.Contains(preview, "\n+++ ") && strings.Contains(preview, "\n@@")
}

func isApprovalDiff(gate *surface.Gate) bool {
	if gate == nil || !isUnifiedDiff(gate.Preview) {
		return false
	}
	action := strings.ToLower(strings.TrimSpace(gate.Action))
	return action == "write_file" || action == "write" || action == "patch" || action == "multiedit" || action == "replace_symbol" || strings.HasPrefix(action, "skill_")
}

func (m Model) gateUsesSplit(gate *surface.Gate, l layout) bool {
	if m.gateViewExplicit {
		return !m.gateUnified
	}
	return m.gateDialogWidth(gate, l) >= 140
}

func (m Model) gateBodyLines(gate *surface.Gate, l layout, p Palette, width int) []string {
	if !isApprovalDiff(gate) {
		body := sanitizeMultilineText(gate.Preview)
		if strings.TrimSpace(body) == "" {
			body = sanitizeMultilineText(gate.Body)
		}
		return wrapText(body, width)
	}
	if m.gateUsesSplit(gate, l) {
		return renderSplitDiffLines(gate.Preview, width, m.gateHorizontal, p)
	}
	return renderUnifiedDiffLines(gate.Preview, width, m.gateHorizontal, p)
}

func renderUnifiedDiffLines(preview string, width, horizontal int, p Palette) []string {
	raw := strings.Split(sanitizeMultilineText(preview), "\n")
	adds, dels := visibleDiffStats(raw)
	lines := []string{p.Dim.Render("preview ") + p.DiffAdd.Render(fmt.Sprintf("+%d", adds)) + " " + p.DiffDel.Render(fmt.Sprintf("-%d", dels))}
	inHunk := false
	for _, original := range raw {
		line := horizontalSlice(original, horizontal, width)
		switch {
		case strings.HasPrefix(original, "@@"):
			inHunk = true
			line = p.DiffHunk.Render(line)
		case inHunk && strings.HasPrefix(original, "+"):
			line = p.DiffAdd.Render(line)
		case inHunk && strings.HasPrefix(original, "-"):
			line = p.DiffDel.Render(line)
		}
		lines = append(lines, line)
	}
	return lines
}

type splitDiffRow struct {
	kind         byte
	left, right  string
	oldNo, newNo int
}

func renderSplitDiffLines(preview string, width, horizontal int, p Palette) []string {
	raw := strings.Split(sanitizeMultilineText(preview), "\n")
	adds, dels := visibleDiffStats(raw)
	rows := parseSplitDiffRows(raw)
	lines := []string{p.Dim.Render("preview ") + p.DiffAdd.Render(fmt.Sprintf("+%d", adds)) + " " + p.DiffDel.Render(fmt.Sprintf("-%d", dels))}
	col := max(1, (width-3)/2)
	for _, row := range rows {
		if row.kind == '@' {
			lines = append(lines, p.DiffHunk.Render(ansi.Truncate(row.left, width, "…")))
			continue
		}
		left := padCells(splitNumber(row.oldNo)+horizontalSlice(row.left, horizontal, col-5), col)
		right := padCells(splitNumber(row.newNo)+horizontalSlice(row.right, horizontal, col-5), col)
		if row.kind == '-' && row.left != "" {
			left = p.DiffDel.Render(left)
		}
		if (row.kind == '+' || row.kind == '-') && row.right != "" {
			right = p.DiffAdd.Render(right)
		}
		lines = append(lines, left+p.Dim.Render(" │ ")+right)
	}
	return lines
}

func parseSplitDiffRows(lines []string) []splitDiffRow {
	rows := make([]splitDiffRow, 0, len(lines))
	oldNo, newNo := 0, 0
	inHunk := false
	for i := 0; i < len(lines); {
		line := lines[i]
		if strings.HasPrefix(line, "@@") {
			inHunk = true
			oldNo, newNo = parseHunkStart(line)
			rows = append(rows, splitDiffRow{kind: '@', left: line})
			i++
			continue
		}
		if !inHunk {
			// File headers are already represented by the typed target metadata;
			// do not duplicate ---/+++ into both split columns.
			i++
			continue
		}
		if strings.HasPrefix(line, "-") {
			var dels, adds []splitDiffRow
			for i < len(lines) && strings.HasPrefix(lines[i], "-") {
				dels = append(dels, splitDiffRow{left: lines[i], oldNo: oldNo})
				oldNo++
				i++
			}
			for i < len(lines) && strings.HasPrefix(lines[i], "+") {
				adds = append(adds, splitDiffRow{right: lines[i], newNo: newNo})
				newNo++
				i++
			}
			for n := 0; n < max(len(dels), len(adds)); n++ {
				var left, right string
				var leftNo, rightNo int
				if n < len(dels) {
					left, leftNo = dels[n].left, dels[n].oldNo
				}
				if n < len(adds) {
					right, rightNo = adds[n].right, adds[n].newNo
				}
				rows = append(rows, splitDiffRow{kind: '-', left: left, right: right, oldNo: leftNo, newNo: rightNo})
			}
			continue
		}
		switch {
		case strings.HasPrefix(line, "+"):
			rows = append(rows, splitDiffRow{kind: '+', right: line, newNo: newNo})
			newNo++
		case strings.HasPrefix(line, " "):
			rows = append(rows, splitDiffRow{kind: ' ', left: line, right: line, oldNo: oldNo, newNo: newNo})
			oldNo++
			newNo++
		default:
			rows = append(rows, splitDiffRow{kind: ' ', left: line, right: line})
		}
		i++
	}
	return rows
}

func parseHunkStart(line string) (oldNo, newNo int) {
	fields := strings.Fields(line)
	if len(fields) < 3 {
		return 0, 0
	}
	parse := func(field string) int {
		field = strings.TrimLeft(field, "+-")
		if comma := strings.IndexByte(field, ','); comma >= 0 {
			field = field[:comma]
		}
		n, _ := strconv.Atoi(field)
		return n
	}
	return parse(fields[1]), parse(fields[2])
}

func splitNumber(line int) string {
	if line <= 0 {
		return "     "
	}
	return fmt.Sprintf("%4d ", line)
}

func visibleDiffStats(lines []string) (adds, dels int) {
	inHunk := false
	for _, line := range lines {
		if strings.HasPrefix(line, "@@") {
			inHunk = true
			continue
		}
		if inHunk && strings.HasPrefix(line, "+") {
			adds++
		}
		if inHunk && strings.HasPrefix(line, "-") {
			dels++
		}
	}
	return adds, dels
}

func horizontalSlice(text string, offset, width int) string {
	if offset > 0 {
		text = ansi.TruncateLeft(text, offset, "")
	}
	return ansi.Truncate(text, max(1, width), "…")
}

func padCells(text string, width int) string {
	return text + strings.Repeat(" ", max(0, width-lipgloss.Width(text)))
}

func (m Model) gateViewportHeight(gate *surface.Gate, l layout) int {
	// Reserve dialog border/padding, title, metadata, blank separators, the
	// scroll position line, and help before assigning the remaining rows.
	available := l.height - gateMetadataLineCount(gate) - 10
	if m.gateFullscreen || l.width <= 77 || l.height <= 20 {
		return max(1, available)
	}
	return max(1, min(available, 24))
}

func gateMetadataLineCount(gate *surface.Gate) int {
	if gate == nil {
		return 0
	}
	count := 0
	if sanitizeInline(gate.Action) != "" {
		count++
	}
	if sanitizeApprovalTarget(gate.Target) != "" {
		count++
	}
	if shortPreconditionHash(gate.PreconditionHash) != "" {
		count++
	}
	count += min(len(gate.Risks), 4)
	return count
}

func (m Model) gateDialogWidth(gate *surface.Gate, l layout) int {
	w := max(1, min(l.width-6, 64))
	if isApprovalDiff(gate) {
		w = approvalDialogWidth(gate, l)
	}
	if m.gateFullscreen || l.width <= 77 || l.height <= 20 {
		w = max(1, l.width-2)
	}
	return w
}

func approvalDialogWidth(gate *surface.Gate, l layout) int {
	if !isApprovalDiff(gate) {
		return max(1, min(l.width-6, 64))
	}
	return min(min(180, max(40, l.width*4/5)), max(1, l.width-4))
}

func (m Model) gateMaxScroll() int {
	gate := m.driver.PendingGate()
	if gate == nil {
		return 0
	}
	l := computeLayout(m.width, m.height)
	w := m.gateDialogWidth(gate, l)
	inner := max(1, w-m.palette.Dialog.GetHorizontalFrameSize())
	return max(0, len(m.gateBodyLines(gate, l, m.palette, inner))-m.gateViewportHeight(gate, l))
}

func (m *Model) clampGateScroll() {
	m.gateScroll = min(max(0, m.gateScroll), m.gateMaxScroll())
	m.gateHorizontal = min(max(0, m.gateHorizontal), m.gateMaxHorizontal())
}

func (m Model) gateMaxHorizontal() int {
	gate := m.driver.PendingGate()
	if !isApprovalDiff(gate) {
		return 0
	}
	l := computeLayout(m.width, m.height)
	width := max(1, m.gateDialogWidth(gate, l)-m.palette.Dialog.GetHorizontalFrameSize())
	if m.gateUsesSplit(gate, l) {
		width = max(1, (width-3)/2-5)
	}
	longest := 0
	for _, line := range strings.Split(sanitizeMultilineText(gate.Preview), "\n") {
		longest = max(longest, lipgloss.Width(line))
	}
	return max(0, longest-width)
}

func (m Model) renderSessionsDialog(l layout, p Palette) string {
	rows := m.filteredSessions()
	title := p.DialogTitle.Render("会话")
	filter := "筛选：" + m.sessionFilter
	if m.sessionFilter == "" {
		filter = "按标题筛选…"
	}
	lines := []string{title, p.DialogFooter.Render(filter)}
	if m.sessionLoading {
		lines = append(lines, "", p.DialogFooter.Render("正在加载会话…"))
	} else if len(rows) == 0 {
		lines = append(lines, "", p.DialogFooter.Render("没有匹配的会话"))
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
				name = "未命名会话"
			}
			lines = append(lines, style.Render(truncate(marker+name, max(8, l.width-14))))
			lines = append(lines, p.Dim.Render(truncate("   "+row.ID, max(8, l.width-14))))
		}
	}
	lines = append(lines, "")
	if m.sessionRenaming {
		lines = append(lines, p.DialogFooter.Render("重命名："+m.sessionRenameInput+"█"), p.DialogFooter.Render("enter 确认 · esc 取消"))
	} else if m.sessionDeleteID != "" {
		name := m.sessionDeleteID
		for _, row := range rows {
			if row.ID == m.sessionDeleteID {
				name = row.Title
				break
			}
		}
		lines = append(lines, p.PromptWarn.Render(truncate("删除 "+name+"？", max(8, l.width-14))), p.DialogFooter.Render("y 删除 · n/esc 取消"))
	} else {
		lines = append(lines, p.DialogFooter.Render("↑/↓ 移动 · enter/tab 选择 · ^r 重命名 · ^x 删除"))
	}
	if m.sessionError != "" {
		lines = append(lines, p.PromptWarn.Render(truncate("! "+m.sessionError, max(8, l.width-14))))
	}
	inner := strings.Join(lines, "\n")
	w := max(1, min(l.width-8, 72))
	return p.Dialog.Width(w).Render(inner)
}

func (m Model) renderCommandDialog(l layout, p Palette) string {
	if m.commandConfirmName != "" {
		name := "/" + m.commandConfirmName
		body := "应用这次会话变更？"
		if m.commandConfirmName == "fork" && len(m.commandConfirmArgs) > 0 {
			body = fmt.Sprintf("在消息 %s 处分叉？", m.commandConfirmArgs[0])
		} else if m.commandConfirmName == "rewind" && len(m.commandConfirmArgs) > 0 {
			body = fmt.Sprintf("回退到消息 %s？", m.commandConfirmArgs[0])
		} else if m.commandConfirmName == "compact" {
			body = "压缩当前会话上下文？"
		}
		inner := strings.Join([]string{
			p.DialogTitle.Render("确认 " + name),
			"",
			p.DialogBody.Render(truncate(body, max(8, l.width-14))),
			"",
			p.DialogFooter.Render("y 确认 · n/esc 取消"),
		}, "\n")
		w := max(1, min(l.width-8, 72))
		return p.Dialog.Width(w).Render(inner)
	}
	title := m.commandOverlayTitle
	if title == "" {
		title = "命令"
	}
	body := m.commandOverlay
	if body == "" {
		body = "done"
	}
	lines := []string{p.DialogTitle.Render(title), ""}
	for _, line := range strings.Split(body, "\n") {
		lines = append(lines, p.DialogBody.Render(truncate(line, max(8, l.width-14))))
	}
	lines = append(lines, "", p.DialogFooter.Render("enter / esc close"))
	inner := strings.Join(lines, "\n")
	w := max(1, min(l.width-8, 72))
	return p.Dialog.Width(w).Render(inner)
}

func padHorizontal(content string, margin, totalWidth int) string {
	pad := ""
	if margin > 0 {
		pad = strings.Repeat(" ", margin)
	}
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		lines[i] = padRight(truncate(pad+line, totalWidth), totalWidth)
	}
	return strings.Join(lines, "\n")
}

func fitHeight(content string, width, height int) string {
	return strings.Join(padLines(strings.Split(content, "\n"), width, height), "\n")
}

func placeOverlay(base, overlay string, width, height int) string {
	_ = base
	blank := strings.Repeat(" ", max(1, width))
	baseLines := make([]string, height)
	for i := range baseLines {
		baseLines[i] = blank
	}
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
	base = padRight(truncate(base, width), width)
	overW := lipgloss.Width(over)
	if overW <= 0 {
		return base
	}
	if col <= 0 && overW >= width {
		return padRight(truncate(over, width), width)
	}
	left := ansi.Cut(base, 0, max(0, col))
	right := ansi.Cut(base, min(width, col+overW), width)
	return padRight(truncate(left+over+right, width), width)
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

func trimTrailingEmpty(lines []string) []string {
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func wrapText(text string, width int) []string {
	if width < 1 {
		width = 1
	}
	if text == "" {
		return []string{""}
	}
	// Model text is data, never terminal control. Strip ANSI and discard
	// controls that could move the cursor or rewrite earlier output. Tabs are
	// expanded deterministically before cell-width wrapping.
	text = sanitizeMarkdownSource(text)
	var lines []string
	for _, paragraph := range strings.Split(text, "\n") {
		lines = append(lines, wrapParagraphExact(paragraph, width)...)
	}
	return lines
}

type textCluster struct {
	text  string
	width int
	space bool
}

// wrapParagraphExact prefers word boundaries while preserving every
// printable grapheme. Whitespace that crosses a boundary remains visible at
// the beginning or end of the adjacent line instead of being synthesized or
// discarded.
func wrapParagraphExact(text string, width int) []string {
	if text == "" {
		return []string{""}
	}
	var clusters []textCluster
	iterator := uniseg.NewGraphemes(text)
	for iterator.Next() {
		cluster := iterator.Str()
		r, _ := utf8.DecodeRuneInString(cluster)
		clusters = append(clusters, textCluster{text: cluster, width: iterator.Width(), space: unicode.IsSpace(r)})
	}

	var lines []string
	var current strings.Builder
	currentWidth := 0
	flush := func() {
		lines = append(lines, current.String())
		current.Reset()
		currentWidth = 0
	}
	for start := 0; start < len(clusters); {
		end := start + 1
		for end < len(clusters) && clusters[end].space == clusters[start].space {
			end++
		}
		tokenWidth := 0
		for _, cluster := range clusters[start:end] {
			tokenWidth += cluster.width
		}
		if !clusters[start].space && currentWidth > 0 && currentWidth+tokenWidth > width {
			flush()
		}
		for _, cluster := range clusters[start:end] {
			if cluster.width > width {
				if currentWidth > 0 {
					flush()
				}
				lines = append(lines, ansi.Truncate(cluster.text, width, "…"))
				continue
			}
			if currentWidth > 0 && currentWidth+cluster.width > width {
				flush()
			}
			current.WriteString(cluster.text)
			currentWidth += cluster.width
		}
		start = end
	}
	if current.Len() > 0 {
		flush()
	}
	return lines
}

func truncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	return ansi.Truncate(s, width, "…")
}

// pasteGuardChip flags a draft that looks like a giant paste: over the char
// or the line threshold. It is derived from the current draft only, so
// trimming back under the thresholds clears the chip without bookkeeping.
func pasteGuardChip(input string) string {
	runes := len([]rune(input))
	lines := strings.Count(input, "\n") + 1
	if runes <= pasteThresholdChars && lines <= pasteThresholdLines {
		return ""
	}
	return fmt.Sprintf("⚠ 大段粘贴 · %d 行 / %d 字符 · enter 发送前请确认", lines, runes)
}

// editorInputLines splits a composer draft into the lines the editor shows.
// CRLF is normalized to "\n", every line is ANSI-safe truncated to width, and
// drafts past maxLines keep only the trailing window (input always appends at
// the tail) with a "…" marker on the first kept line. The line count it
// returns is the same accounting Model.layout uses for the editor reserve.
func editorInputLines(input string, width, maxLines int) []string {
	if width < 1 {
		width = 1
	}
	if maxLines < 1 {
		maxLines = 1
	}
	normalized := strings.ReplaceAll(strings.ReplaceAll(input, "\r\n", "\n"), "\r", "\n")
	lines := strings.Split(normalized, "\n")
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
		lines[0] = "…" + lines[0]
	}
	for i, line := range lines {
		lines[i] = ansi.Truncate(line, width, "…")
	}
	return lines
}

func padRight(s string, width int) string {
	if w := lipgloss.Width(s); w < width {
		return s + strings.Repeat(" ", width-w)
	}
	return s
}

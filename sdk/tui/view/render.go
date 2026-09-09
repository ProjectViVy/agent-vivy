package view

import (
	"bytes"
	"encoding/json"
	"fmt"
	"hash/maphash"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/rivo/uniseg"

	"agent-vivy/sdk/tui/internal/textsafe"
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
	lines := []string{p.DialogTitle.Render(m.translator.T("vivy.tui.arguments.title", map[string]any{"command": command.Name})), p.Dim.Render(truncate(command.Description, lineWidth)), ""}
	for i, argument := range command.Arguments {
		marker := "  "
		style := p.Idle
		if i == m.dynamicArgumentCursor {
			marker, style = "▸ ", p.Active
		}
		required := m.translator.T("vivy.tui.arguments.optional", nil)
		if argument.Required {
			required = m.translator.T("vivy.tui.arguments.required", nil)
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
	lines = append(lines, "", p.DialogFooter.Render(m.translator.T("vivy.tui.arguments.footer", nil)))
	return p.Dialog.Width(w).Render(strings.Join(lines, "\n"))
}

func (m Model) renderModelDialog(l layout, p Palette) string {
	rows := m.filteredModels()
	w := max(1, min(l.width-8, 72))
	innerWidth := max(1, w-p.Dialog.GetHorizontalFrameSize())
	status := m.translator.T("vivy.tui.filter.value", map[string]any{"filter": sanitizeCommandPaletteFilter(m.modelPickerFilter)})
	if strings.TrimSpace(m.modelPickerFilter) == "" {
		status = m.translator.T("vivy.tui.filter.all", nil)
	}
	if m.modelPickerLoading {
		status += m.translator.T("vivy.tui.models.loading", nil)
	} else if m.modelPickerSelecting {
		status += m.translator.T("vivy.tui.models.applying", nil)
	}
	lines := []string{p.DialogTitle.Render(truncate(m.translator.T("vivy.tui.models.title", nil), innerWidth)), p.DialogFooter.Render(truncate(status, innerWidth)), ""}
	if m.modelPickerError != "" {
		lines = append(lines, p.ToolFail.Render(truncate(m.modelPickerError, innerWidth)))
	} else if !m.modelPickerLoading && len(rows) == 0 {
		lines = append(lines, p.DialogFooter.Render(truncate(m.translator.T("vivy.tui.models.empty", nil), innerWidth)))
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
	footer := m.translator.T("vivy.tui.models.footer", nil)
	if catalog.ReadOnly || catalog.Frozen {
		footer = m.translator.T("vivy.tui.models.readOnlyFooter", nil)
		if lipgloss.Width(footer) > innerWidth {
			footer = m.translator.T("vivy.tui.models.readOnlyFooter.short", nil)
		}
	} else if lipgloss.Width(footer) > innerWidth {
		footer = m.translator.T("vivy.tui.models.footer.short", nil)
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
		status += m.translator.T("vivy.tui.files.loading", nil)
	} else if m.fileCompletionTruncated {
		status += m.translator.T("vivy.tui.files.partial", nil)
	}
	lines := []string{p.DialogTitle.Render(m.translator.T("vivy.tui.files.title", nil)), p.DialogFooter.Render(truncate(status, max(1, w-p.Dialog.GetHorizontalFrameSize())))}
	if !compact {
		lines = append(lines, "")
	}
	if m.fileCompletionError != "" {
		lines = append(lines, p.ToolFail.Render(truncate(m.fileCompletionError, max(1, w-p.Dialog.GetHorizontalFrameSize()))))
	} else if !m.fileCompletionLoading && len(rows) == 0 {
		lines = append(lines, p.DialogFooter.Render(m.translator.T("vivy.tui.files.empty", nil)))
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
	footer := m.translator.T("vivy.tui.files.footer", nil)
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
	return textsafe.IsBidiControl(r)
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
	chat, scroll := m.renderChat(l.mainW(), l.mainH(), p)
	editor := m.renderEditor(l.mainW(), p)
	chrome := m.renderChromeRow(l.mainW(), p, scroll)
	mainCol := lipgloss.JoinVertical(lipgloss.Left, chat, "", editor, chrome)
	side := m.renderSidebar(l.sidebarW, lipgloss.Height(mainCol), p)
	gap := lipgloss.NewStyle().Width(1).Height(lipgloss.Height(mainCol)).Render(" ")
	row := lipgloss.JoinHorizontal(lipgloss.Top, mainCol, gap, side)
	return padHorizontal(row, l.marginX, l.width)
}

func (m Model) renderCompact(l layout, p Palette) string {
	header := m.renderCompactHeader(l, p)
	chat, scroll := m.renderChat(l.innerW(), l.mainH(), p)
	editor := m.renderEditor(l.innerW(), p)
	chrome := m.renderChromeRow(l.innerW(), p, scroll)
	col := lipgloss.JoinVertical(lipgloss.Left, header, "", chat, "", editor, chrome)
	return padHorizontal(col, l.marginX, l.width)
}

func (m Model) renderCompactHeader(l layout, p Palette) string {
	session := m.driver.Active()
	meta := m.driver.Meta()
	logo := p.Logo.Render("Vivy™ ") + p.LogoWord.Render("VIVY CODE") + " "
	label := session.Title
	if label == "" {
		label = m.translator.T("vivy.tui.session.untitled", nil)
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

// minModifiedPathCells is the smallest path budget a modified-file row may be
// trimmed to before its timestamp is dropped instead.
const minModifiedPathCells = 10

func (m Model) sidebarLines(width int, p Palette) []string {
	active := m.driver.Active()
	snapshot := m.driver.Sidebar()
	if snapshot.Session.ID == "" {
		snapshot.Session = active
	}

	lines := make([]string, 0, 24)
	title := strings.TrimSpace(snapshot.Session.Title)
	if title == "" {
		title = m.translator.T("vivy.tui.session.untitled", nil)
	}
	titleLines := wrapText(title, max(8, width-2))
	if len(titleLines) > 2 {
		titleLines = titleLines[:2]
	}
	for _, line := range titleLines {
		lines = append(lines, p.Active.Render(" "+line))
	}
	if updated := sidebarTime(snapshot.Session.UpdatedAt); updated != "" {
		lines = append(lines, p.Dim.Render(truncate(m.translator.T("vivy.tui.sidebar.updated", map[string]any{"time": updated}), width-1)))
	}
	if cwd := strings.TrimSpace(snapshot.CWD); cwd != "" {
		lines = append(lines, p.Dim.Render(truncate(m.translator.T("vivy.tui.sidebar.cwd", map[string]any{"path": cwd}), width-1)))
	}
	if host := strings.TrimSpace(m.driver.Meta().Host); host != "" {
		lines = append(lines, p.Active.Render(truncate(m.translator.T("vivy.tui.sidebar.host", map[string]any{"host": host}), width-1)))
	}
	if snapshot.ReasoningKnown {
		reasoning := m.translator.T("vivy.tui.sidebar.unsupported", nil)
		if snapshot.ReasoningSupported {
			reasoning = m.translator.T("vivy.tui.sidebar.supported", nil)
		}
		lines = append(lines, p.Dim.Render(truncate(m.translator.T("vivy.tui.sidebar.reasoning", map[string]any{"support": reasoning}), width-1)))
	}
	if snapshot.HasContext && snapshot.Context.ThinkingSupported {
		lines = append(lines, p.Dim.Render(truncate(m.translator.T("vivy.tui.sidebar.thinking", map[string]any{"mode": m.driver.ThinkingMode()}), width-1)))
	}
	if snapshot.HasContext {
		lines = append(lines, "", p.Dim.Render(m.translator.T("vivy.tui.sidebar.context", nil)))
		ctx := snapshot.Context
		switch {
		case ctx.ModelLimitKnown && ctx.ModelLimitTokens > 0:
			ratio := float64(ctx.FeedTokens) / float64(ctx.ModelLimitTokens)
			percentage := int(ratio * 100)
			estimated := ""
			if ctx.TokenCountsEstimated {
				estimated = "~"
			}
			line := m.translator.T("vivy.tui.sidebar.contextTokens", map[string]any{"estimated": estimated, "percentage": percentage, "tokens": compactNumber(ctx.FeedTokens), "limit": compactNumber(ctx.ModelLimitTokens)})
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
			lines = append(lines, p.Dim.Render(truncate(m.translator.T("vivy.tui.sidebar.contextUnknown", map[string]any{"estimated": estimated, "tokens": compactNumber(ctx.FeedTokens)}), width-1)))
		}
		if ctx.TotalMessages > 0 {
			if ctx.FeedMessages > 0 && ctx.FeedMessages != ctx.TotalMessages {
				lines = append(lines, p.Dim.Render(truncate(m.translator.T("vivy.tui.sidebar.feedMessages", map[string]any{"feed": ctx.FeedMessages, "total": ctx.TotalMessages}), width-1)))
			} else {
				lines = append(lines, p.Dim.Render(truncate(m.translator.T("vivy.tui.sidebar.messages", map[string]any{"count": ctx.TotalMessages}), width-1)))
			}
		}
		if ctx.TriggerTokens > 0 {
			estimated := ""
			if ctx.TokenCountsEstimated {
				estimated = "~"
			}
			lines = append(lines, p.Dim.Render(truncate(m.translator.T("vivy.tui.sidebar.compactAt", map[string]any{"estimated": estimated, "tokens": compactNumber(ctx.TriggerTokens)}), width-1)))
		}
		if ctx.CompactionEnabled {
			compaction := m.translator.T("vivy.tui.sidebar.compactionOn", nil)
			if ctx.WouldCompact {
				compaction = m.translator.T("vivy.tui.sidebar.compactionNeeded", nil)
			}
			lines = append(lines, p.Dim.Render(compaction))
		}
		if ctx.HasCompactionSummary {
			lines = append(lines, p.Dim.Render(m.translator.T("vivy.tui.sidebar.summary", nil)))
		}
	}
	if snapshot.HasUsage {
		lines = append(lines, "", p.Dim.Render(m.translator.T("vivy.tui.sidebar.usage", nil)))
		usage := snapshot.Usage
		lines = append(lines,
			p.Dim.Render(truncate(m.translator.T("vivy.tui.sidebar.total", map[string]any{"tokens": compactNumber(usage.TotalTokens)}), width-1)),
			p.Dim.Render(truncate(m.translator.T("vivy.tui.sidebar.input", map[string]any{"tokens": compactNumber(usage.PromptTokens)}), width-1)),
			p.Dim.Render(truncate(m.translator.T("vivy.tui.sidebar.output", map[string]any{"tokens": compactNumber(usage.CompletionTokens)}), width-1)),
		)
		if usage.ReasoningTokens > 0 {
			lines = append(lines, p.Dim.Render(truncate(m.translator.T("vivy.tui.sidebar.reasoningTokens", map[string]any{"tokens": compactNumber(usage.ReasoningTokens)}), width-1)))
		}
		if usage.CachedTokens > 0 {
			lines = append(lines, p.Dim.Render(truncate(m.translator.T("vivy.tui.sidebar.cached", map[string]any{"tokens": compactNumber(usage.CachedTokens)}), width-1)))
		}
		lines = append(lines, p.Dim.Render(truncate(m.translator.T("vivy.tui.sidebar.requests", map[string]any{"count": usage.RequestCount}), width-1)))
		cost := m.translator.T("vivy.tui.sidebar.unknown", nil)
		if usage.CostKnown {
			cost = fmt.Sprintf("$%.4f", usage.CostUSD)
		}
		lines = append(lines, p.Dim.Render(truncate(m.translator.T("vivy.tui.sidebar.cost", map[string]any{"cost": cost}), width-1)))
	}
	if snapshot.ModifiedFilesKnown {
		lines = append(lines, "", p.Dim.Render(m.translator.T("vivy.tui.sidebar.modifiedFiles", nil)))
		if len(snapshot.ModifiedFiles) == 0 {
			lines = append(lines, p.Dim.Render(m.translator.T("vivy.tui.sidebar.none", nil)))
		}
		for _, file := range snapshot.ModifiedFiles {
			path := strings.TrimSpace(safeProjectFilePath(file.Path))
			if path == "" {
				continue
			}
			updated := sidebarTime(file.UpdatedAt)
			counts := fmt.Sprintf("  +%d -%d", file.Diff.Additions, file.Diff.Deletions)
			pathBudget := width - 2 - lipgloss.Width(counts)
			if pathBudget < 1 {
				// Degenerate tiny width: even the counts-only line overflows,
				// so end-truncate the whole plain line as before.
				line := " " + path + counts
				if updated != "" {
					line += " · " + updated
				}
				lines = append(lines, p.Dim.Render(truncate(line, width-1)))
				continue
			}
			if updated != "" {
				switch timeWidth := lipgloss.Width(" · " + updated); {
				case lipgloss.Width(path)+timeWidth <= pathBudget:
					// Short path: the timestamp fits beside it untruncated.
				case pathBudget-timeWidth >= minModifiedPathCells:
					// Trim the path to make room, never below the floor.
					pathBudget -= timeWidth
				default:
					updated = "" // the timestamp would starve the path
				}
			}
			line := p.Dim.Render(" "+middleTruncate(path, pathBudget)) + "  " +
				p.DiffAdd.Render(fmt.Sprintf("+%d", file.Diff.Additions)) + " " +
				p.DiffDel.Render(fmt.Sprintf("-%d", file.Diff.Deletions))
			if updated != "" {
				line += p.Dim.Render(" · " + updated)
			}
			lines = append(lines, line)
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
			lspLines = append(lspLines, style.Render(truncate(" "+language+" · "+m.stateLabel(state), width-1)))
		}
		lines = append(lines, "", p.Dim.Render(m.translator.T("vivy.tui.sidebar.lsp", nil)))
		if len(lspLines) == 0 {
			lines = append(lines, p.Dim.Render(m.translator.T("vivy.tui.sidebar.lspEmpty", nil)))
		} else {
			lines = append(lines, lspLines...)
		}
	}
	if snapshot.MCPKnown {
		lines = append(lines, "", p.Dim.Render(m.translator.T("vivy.tui.sidebar.mcp", nil)))
		if len(snapshot.MCP) == 0 {
			lines = append(lines, p.Dim.Render(m.translator.T("vivy.tui.sidebar.mcpEmpty", nil)))
		}
		for _, server := range snapshot.MCP {
			name := strings.TrimSpace(sanitizeFileCompletionText(server.Name))
			if name == "" {
				continue
			}
			state := ""
			style := p.Dim
			switch server.State {
			case "configured":
				state = "configured"
			case "error":
				state = "error"
				style = p.PromptWarn
			case "initialized":
				state = "initialized"
				style = p.Active
			default:
				continue
			}
			line := " " + name + " · " + m.stateLabel(state)
			if transport := strings.TrimSpace(sanitizeMCPTransport(server.Transport)); transport != "" {
				line += " · " + transport
			}
			lines = append(lines, style.Render(truncate(line, width-1)))
			if state == "error" {
				if message := sanitizeInline(server.Error); message != "" {
					lines = append(lines, p.PromptWarn.Render(truncate("   ! "+message, width-1)))
				}
			}
			if server.AuthMissing {
				lines = append(lines, p.PromptWarn.Render(truncate(m.translator.T("vivy.tui.sidebar.authMissing", nil), width-1)))
			}
			if missing := sanitizeMCPEnvMissing(server.EnvMissing); missing != "" {
				lines = append(lines, p.PromptWarn.Render(truncate(m.translator.T("vivy.tui.sidebar.envMissing", map[string]any{"names": missing}), width-1)))
			}
			if server.ToolCount >= 0 {
				lines = append(lines, p.Dim.Render(truncate(m.translator.T("vivy.tui.sidebar.tools", map[string]any{"count": server.ToolCount}), width-1)))
			}
		}
	}
	if snapshot.SkillsKnown {
		lines = append(lines, "", p.Dim.Render(m.translator.T("vivy.tui.sidebar.skills", nil)))
		if len(snapshot.Skills) == 0 {
			lines = append(lines, p.Dim.Render(m.translator.T("vivy.tui.sidebar.none", nil)))
		}
		for _, skill := range snapshot.Skills {
			name := strings.TrimSpace(sanitizeFileCompletionText(skill.Name))
			if name != "" {
				line := " " + name
				if origin := strings.TrimSpace(sanitizeFileCompletionText(skill.Origin)); origin != "" {
					line += " \u00b7 " + origin
				}
				lines = append(lines, p.Dim.Render(truncate(line, width-1)))
			}
		}
	}
	meta := m.driver.Meta()
	if meta.Busy {
		lines = append(lines, "", p.Dim.Render(truncate(m.translator.T("vivy.tui.sidebar.run", map[string]any{"id": fallback(meta.RunID, m.translator.T("vivy.tui.run.active", nil))}), width-1)))
	}
	if meta.Queued > 0 {
		lines = append(lines, p.Dim.Render(truncate(m.translator.T("vivy.tui.sidebar.queue", map[string]any{"count": meta.Queued}), width-1)))
	}
	if errText := strings.TrimSpace(meta.Error); errText != "" {
		lines = append(lines, "", p.PromptWarn.Render(truncate(" ! "+errText, width-1)))
	}
	return lines
}

func sanitizeMCPTransport(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "http" || value == "stdio" {
		return value
	}
	return ""
}

func sanitizeMCPEnvMissing(values []string) string {
	clean := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		valid := true
		for i, r := range value {
			if (i == 0 && (r < 'A' || r > 'Z')) || (i > 0 && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '_') {
				valid = false
				break
			}
		}
		if valid {
			clean = append(clean, value)
		}
	}
	return strings.Join(clean, ", ")
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

// chatScrollInfo carries the chat viewport scroll state that renderChat
// computes to the chrome row, so the scroll hint can be composed without
// writing Model state during render.
type chatScrollInfo struct {
	follow    bool
	offset    int
	maxScroll int
	viewport  int
}

// chatAssembly is the cached per-message segment table of the active chat
// history. It is allocated once per Model and mutated in place so value
// copies of Model share the cache.
type chatAssembly struct {
	sessionID string
	width     int
	collapsed bool
	debugTool bool
	loading   bool
	stamp     uint64
	segments  [][]string
	lineCount int
}

func (m Model) renderChat(width, height int, p Palette) (string, chatScrollInfo) {
	assembly := m.chatSegments(width, p)
	maxScroll := max(0, assembly.lineCount-height)
	offset := min(max(0, m.chatScroll), maxScroll)
	if m.chatFollow {
		offset = maxScroll
	}
	info := chatScrollInfo{follow: m.chatFollow, offset: offset, maxScroll: maxScroll, viewport: max(1, height)}
	end := min(assembly.lineCount, offset+height)
	lines := make([]string, 0, max(0, end-offset))
	if offset < end {
		skipped := 0
		for _, segment := range assembly.segments {
			if skipped+len(segment) <= offset {
				skipped += len(segment)
				continue
			}
			start := max(0, offset-skipped)
			stop := min(len(segment), end-skipped)
			if start < stop {
				lines = append(lines, segment[start:stop]...)
			}
			skipped += len(segment)
			if skipped >= end {
				break
			}
		}
	}
	content := strings.Join(lines, "\n")
	return p.Chat.Width(width).MaxWidth(width).Height(height).MaxHeight(height).Render(padBlock(content, width, height)), info
}

// chatSegments returns the assembled per-message line segments of the active
// history, reusing chatAssembly across the several per-frame calls (clamp,
// help overlay, render) so the full history is neither re-rendered nor
// re-flattened. The stamp fingerprints every rendered field of every message;
// separators are baked into each segment so viewport windows slice cleanly.
func (m Model) chatSegments(width int, p Palette) *chatAssembly {
	messages := m.driver.ActiveMessages()
	debugToolOutput := m.debugToolOutput || m.toolExpanded
	loading := len(messages) == 0 && m.driver.Meta().Loading
	assembly := m.chatAssembly
	if assembly == nil {
		assembly = &chatAssembly{}
	}
	if assembly.sessionID == m.driver.Active().ID && assembly.width == width && assembly.collapsed == m.reasoningCollapsed &&
		assembly.debugTool == debugToolOutput && assembly.loading == loading && assembly.stamp == chatStamp(messages) {
		return assembly
	}
	m.mdCache.ensure(m.driver.Active().ID, width, m.reasoningCollapsed)
	segments := make([][]string, 0, len(messages))
	if len(messages) == 0 {
		if loading {
			segments = append(segments, m.renderHistoryLoading(p, width))
		} else {
			segments = append(segments, m.renderEmptyHero(p, width))
		}
	}
	for index, message := range messages {
		segment, ok := m.mdCache.get(message, width)
		if !ok {
			segment = m.renderMessageWithOptions(message, width, p, debugToolOutput, m.reasoningCollapsed)
			m.mdCache.put(message, width, segment)
		}
		if index < len(messages)-1 {
			segment = append(append([]string(nil), segment...), "")
		}
		segments = append(segments, segment)
	}
	assembly.sessionID = m.driver.Active().ID
	assembly.width = width
	assembly.collapsed = m.reasoningCollapsed
	assembly.debugTool = debugToolOutput
	assembly.loading = loading
	assembly.stamp = chatStamp(messages)
	assembly.segments = segments
	assembly.lineCount = 0
	for _, segment := range segments {
		assembly.lineCount += len(segment)
	}
	return assembly
}

func (m Model) chatLines(width int, p Palette) []string {
	assembly := m.chatSegments(width, p)
	lines := make([]string, 0, assembly.lineCount)
	for _, segment := range assembly.segments {
		lines = append(lines, segment...)
	}
	return lines
}

// chatStampSeed fingerprints message content; maphash hashes without
// allocating and the per-frame cost matches the per-message mdCache lookups
// the render already performs.
var chatStampSeed = maphash.MakeSeed()

func chatStamp(messages []surface.Message) uint64 {
	stamp := uint64(len(messages))
	for _, message := range messages {
		stamp = stamp*31 + messageStamp(message)
	}
	return stamp
}

func messageStamp(message surface.Message) uint64 {
	h := maphash.String(chatStampSeed, message.ID)
	h = h*31 + maphash.String(chatStampSeed, message.Role)
	h = h*31 + maphash.String(chatStampSeed, message.Content)
	if message.Tool != nil {
		h = h*31 + maphash.String(chatStampSeed, message.Tool.ToolName)
		h = h*31 + maphash.String(chatStampSeed, message.Tool.ToolCallID)
		h = h*31 + maphash.String(chatStampSeed, message.Tool.Status)
		h = h*31 + maphash.String(chatStampSeed, message.Tool.Preview)
		h = h*31 + maphash.String(chatStampSeed, message.Tool.Result)
		h = h*31 + maphash.String(chatStampSeed, message.Tool.ApprovalID)
	}
	if message.Streaming {
		h = h*31 + 1
	}
	if message.Reasoning {
		h = h*31 + 2
	}
	for _, attachment := range message.Attachments {
		h = h*31 + maphash.String(chatStampSeed, attachment.Path)
		h = h*31 + maphash.String(chatStampSeed, attachment.Name)
		h = h*31 + maphash.String(chatStampSeed, attachment.MimeType)
		h = h*31 + uint64(attachment.Size)
	}
	for _, file := range message.FileContexts {
		h = h*31 + maphash.String(chatStampSeed, file.Path)
		h = h*31 + maphash.String(chatStampSeed, file.Name)
		h = h*31 + uint64(file.Size)
	}
	return h
}

// renderHistoryLoading replaces the empty-conversation hero while the active
// session's history projection is still loading (Meta.Loading), so a session
// switch never poses as an empty conversation.
func (m Model) renderHistoryLoading(p Palette, width int) []string {
	return []string{p.Dim.Render(truncate(m.translator.T("vivy.tui.history.loading", nil), width))}
}

func (m Model) renderEmptyHero(p Palette, width int) []string {
	lines := []string{
		p.Dim.Render(""),
		p.Logo.Render("Vivy™ ") + p.LogoWord.Render("VIVY CODE"),
		p.Dim.Render(m.translator.T("vivy.tui.hero.tagline", nil)),
		"",
	}
	if cwd := strings.TrimSpace(m.driver.Sidebar().CWD); cwd != "" {
		lines = append(lines, p.Dim.Render(truncate(m.translator.T("vivy.tui.hero.cwd", map[string]any{"path": cwd}), width)))
	}
	return append(lines,
		p.Dim.Render(truncate(m.translator.T("vivy.tui.hero.commands", nil), width)),
		p.Dim.Render(truncate(m.translator.T("vivy.tui.hero.toggles", nil), width)),
	)
}

func (m Model) renderMessage(message surface.Message, width int, p Palette) []string {
	return m.renderMessageWithOptions(message, width, p, false, false)
}

func (m Model) renderMessageWithOptions(message surface.Message, width int, p Palette, debugToolOutput, reasoningCollapsed bool) []string {
	if message.Tool != nil {
		return m.renderToolWithOptions(message.Tool, width, p, debugToolOutput)
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

	bodyLines, painted := renderMessageBody(message, contentWidth)
	if message.Reasoning && reasoningCollapsed {
		if len(bodyLines) > 0 {
			bodyLines = []string{m.translator.T("vivy.tui.reasoning.lines", map[string]any{"count": len(bodyLines)})}
		} else {
			bodyLines = []string{m.translator.T("vivy.tui.reasoning.collapsed", nil)}
		}
		painted = false
	}
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
	appendChipLines(m.renderAttachmentChips(message.Attachments))
	appendChipLines(m.renderFileContextChips(message.FileContexts))
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

func renderMessageBody(message surface.Message, contentWidth int) ([]string, bool) {
	quiet := message.Reasoning
	source := sanitizeMarkdownSource(message.Content)
	if source == "" {
		return nil, true
	}
	wrapWidth := markdownWrapWidth(contentWidth)
	var rendered string
	var err error
	if message.Streaming {
		// A growing bubble renders through the stable-prefix cache so each
		// flush re-renders only the trailing segment, not the whole document.
		rendered, err = streamMarkdownRender(message.ID, source, wrapWidth, quiet)
	} else {
		rendered, err = renderMarkdown(source, wrapWidth, quiet)
	}
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

func (m Model) renderTool(tool *surface.ToolCard, width int, p Palette) []string {
	return m.renderToolWithOptions(tool, width, p, false)
}

const compactToolResultLines = 8

func (m Model) renderToolWithOptions(tool *surface.ToolCard, width int, p Palette, debugToolOutput bool) []string {
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
		name = m.translator.T("vivy.tui.common.tool", nil)
	}
	status := strings.TrimSpace(sanitizeFileCompletionText(tool.Status))
	title := fmt.Sprintf("%s %s  %s", icon, name, m.stateLabel(status))
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
			lines = append(lines, m.compactToolLines(wrapText(body, max(1, width)), debugToolOutput, max(1, width))...)
		}
		return lines
	}
	contentWidth := max(1, min(52, available-frame))
	innerLines := wrapText(title, contentWidth)
	if body != "" {
		innerLines = append(innerLines, m.compactToolLines(renderToolBodyLines(body, contentWidth, p), debugToolOutput, contentWidth)...)
	}
	box := style.Width(contentWidth).MaxWidth(available).Render(strings.Join(innerLines, "\n"))
	indented := make([]string, 0)
	for _, line := range strings.Split(box, "\n") {
		indented = append(indented, "  "+line)
	}
	return indented
}

func (m Model) compactToolLines(lines []string, debug bool, width int) []string {
	if debug || len(lines) <= compactToolResultLines {
		return lines
	}
	omitted := len(lines) - compactToolResultLines
	compact := append([]string(nil), lines[:compactToolResultLines]...)
	marker := m.translator.T("vivy.tui.tool.omitted", map[string]any{"count": omitted})
	return append(compact, wrapText(marker, max(1, width))...)
}

type toolResultKind int

const (
	toolResultPlain toolResultKind = iota
	toolResultJSON
	toolResultDiff
	toolResultMarkdown
)

// markdownSniffPatterns is the same heuristic crush uses to decide that tool
// output carries markdown structure worth highlighting rather than wrapping.
var markdownSniffPatterns = []string{"# ", "## ", "**", "```", "- ", "1. ", "> ", "---", "***"}

// toolResultContent routes a tool body into the crush split: JSON, unified
// diff, markdown, plain (board row TUI-MD-TOOL-RESULTS). Detection order
// matters — JSON wins over diff/markdown because structured output can
// contain any marker — and JSON re-indenting is byte-preserving.
func toolResultContent(body string) (toolResultKind, string) {
	trimmed := strings.TrimSpace(body)
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		var buf bytes.Buffer
		if err := json.Indent(&buf, []byte(trimmed), "", "  "); err == nil {
			return toolResultJSON, buf.String()
		}
	}
	if isUnifiedDiffContent(body) {
		return toolResultDiff, body
	}
	for _, pattern := range markdownSniffPatterns {
		if strings.Contains(body, pattern) {
			return toolResultMarkdown, body
		}
	}
	return toolResultPlain, body
}

// isUnifiedDiffContent reports whether body looks like a real unified diff,
// following crush's internal/diffdetect rule (hunk marker plus file headers,
// or git header plus file headers). The previous substring check ("\n+"/
// "\n-") misclassified markdown lists as diffs; a bare "@@" hunk line still
// routes to the diff renderer so Vivy's own hunk-style previews keep their
// coloring.
func isUnifiedDiffContent(content string) bool {
	hasHunk, hasFileHeader, hasGitHeader := false, false, false
	for line := range strings.SplitSeq(content, "\n") {
		switch {
		case strings.HasPrefix(line, "@@"):
			hasHunk = true
		case strings.HasPrefix(line, "--- ") || strings.HasPrefix(line, "+++ "):
			hasFileHeader = true
		case strings.HasPrefix(line, "diff --git "):
			hasGitHeader = true
		}
	}
	if hasGitHeader && hasFileHeader {
		return true
	}
	return hasHunk
}

// renderToolBodyLines renders a tool card body. JSON and markdown bodies are
// syntax-highlighted through the shared markdown pipeline's chroma code
// blocks; diffs keep the unified-diff coloring; everything else stays plain
// wrapped text. Highlighted lines are truncated, never rewrapped, so ANSI
// styling survives inside the card box.
func renderToolBodyLines(body string, contentWidth int, p Palette) []string {
	kind, content := toolResultContent(body)
	switch kind {
	case toolResultJSON, toolResultMarkdown:
		lang := "markdown"
		if kind == toolResultJSON {
			lang = "json"
		}
		source := sanitizeMarkdownSource(content)
		// The quiet style carries no chroma config; the normal style provides
		// the palette chroma highlighting for the fenced code block.
		rendered, err := renderMarkdown("```"+lang+"\n"+source+"\n```", markdownWrapWidth(contentWidth), false)
		if err != nil || rendered == "" {
			return wrapText(body, contentWidth)
		}
		lines := strings.Split(rendered, "\n")
		for i, line := range lines {
			if lipgloss.Width(line) > contentWidth {
				lines[i] = ansi.Truncate(line, contentWidth, "…")
			}
		}
		return lines
	case toolResultDiff:
		wrapped := wrapText(content, contentWidth)
		return strings.Split(renderDiffBody(strings.Join(wrapped, "\n"), p), "\n")
	default:
		return wrapText(body, contentWidth)
	}
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
	if chips := m.renderAttachmentChips(m.driver.PendingAttachments()); chips != "" {
		lines = append(lines, p.Dim.Render(truncate(chips, inner)))
	}
	if chip := m.pasteGuardChip(m.input); chip != "" {
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
			lines = append(lines, truncate(prompt+p.Dim.Render(m.translator.T("vivy.tui.composer.placeholder", nil)), inner))
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
		model = m.translator.T("vivy.tui.common.model", nil)
	}
	sep := p.Dim.Render("  ·  ")
	mode := workingMode(m.driver.RunMode(), snapshot.Session.PermissionPreset)
	parts := []string{
		p.Dim.Render(model) + m.renderThinkingIntensity(m.driver.ThinkingMode(), p),
		workingModeStyle(mode, p).Render(m.workingModeLabel(mode)),
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

// spinnerFrames is the hand-rolled braille spinner for the busy chrome. The
// view owns no timer: the driver's 40ms live tick already repaints on every
// message and Update advances the frame index each time.
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func (m Model) renderInputChrome(width int, p Palette) string {
	// Dialog measurement and standalone callers see the follow-mode chrome:
	// the scroll hint belongs to the live chat frame only.
	return m.renderChromeRow(width, p, chatScrollInfo{follow: true})
}

// renderChromeRow composes the chrome row for the live frame: optional scroll
// hint, the left hints/spinner/error segment, and the right environment meta.
func (m Model) renderChromeRow(width int, p Palette, scroll chatScrollInfo) string {
	left := m.chromeLeft(p)
	if hint := m.chromeScrollHint(p, scroll); hint != "" {
		left = hint + p.HelpDesc.Render("  ") + left
	}
	right := m.chromeMeta(left, width, p)
	row := joinChromeRow(left, right, width)
	return lipgloss.NewStyle().Width(width).MaxWidth(width).MaxHeight(1).Align(lipgloss.Left).Render(truncate(row, width))
}

const chatScrollHintLines = 3 // near-bottom margin below which the hint stays hidden

// chromeScrollHint describes a paused chat viewport: how to get back to the
// bottom, or how much history remains below. It stays quiet while following,
// when there is nothing to scroll, and within a few lines of the bottom.
func (m Model) chromeScrollHint(p Palette, info chatScrollInfo) string {
	if info.follow || info.maxScroll == 0 {
		return ""
	}
	below := info.maxScroll - info.offset
	if below <= chatScrollHintLines {
		return ""
	}
	if below > info.viewport {
		return p.HelpKey.Render(m.translator.T("vivy.tui.scroll.history", nil)) + p.HelpDesc.Render(m.translator.T("vivy.tui.scroll.below", map[string]any{"count": below}))
	}
	return p.HelpKey.Render("↓ end") + p.HelpDesc.Render(m.translator.T("vivy.tui.scroll.bottom", nil))
}

// chromeLeft builds the left chrome segment with the existing priority:
// transport error first, then the busy spinner, then the hint keys.
func (m Model) chromeLeft(p Palette) string {
	hints := p.HelpKey.Render("shift+tab") + p.HelpDesc.Render(m.translator.T("vivy.tui.chrome.switchMode", nil)) + p.HelpDesc.Render("  ") + p.HelpKey.Render("shift+h") + p.HelpDesc.Render(m.translator.T("vivy.tui.chrome.help", nil)) + p.HelpDesc.Render("  ") + p.HelpKey.Render("ctrl+x") + p.HelpDesc.Render(m.translator.T("vivy.tui.chrome.shortcuts", nil))
	if gate := m.driver.PendingGate(); gate != nil && !gate.Submitting {
		if gate.Kind == "question" {
			hints = p.HelpKey.Render("enter") + p.HelpDesc.Render(m.translator.T("vivy.tui.chrome.answer", nil))
		} else {
			hints = p.HelpKey.Render("y/n") + p.HelpDesc.Render(m.translator.T("vivy.tui.chrome.approve", nil))
		}
	}
	if m.sidebarFocused {
		hints = p.HelpKey.Render("esc") + p.HelpDesc.Render(m.translator.T("vivy.tui.chrome.leaveSidebar", nil))
	}
	meta := m.driver.Meta()
	if errText := strings.TrimSpace(meta.Error); errText != "" {
		return p.ToolFail.Render(m.translator.T("vivy.tui.chrome.error", map[string]any{"error": errText})) + p.HelpDesc.Render("  ") + hints
	}
	if meta.Busy {
		return p.Dim.Render(m.busyStatus(meta)) + p.HelpDesc.Render("  ") + hints
	}
	return hints
}

const (
	chromeGap           = 2 // cells between chrome segments and meta candidates
	minChromeTitleWidth = 8 // below this the trimmed title carries no information
)

// chromeMeta builds the right chrome segment: the queued count, the host, and
// the active session title, joined left to right with two-space gaps. The
// title is tail-truncated to the room that remains next to the other
// candidates; after that candidates degrade tail-first (title → host →
// queued) until the segment fits or disappears.
func (m Model) chromeMeta(left string, width int, p Palette) string {
	meta := m.driver.Meta()
	var parts []string
	if meta.Queued > 0 {
		parts = append(parts, p.PromptWarn.Render(m.translator.T("vivy.tui.chrome.queued", map[string]any{"count": meta.Queued})))
	}
	if host := strings.TrimSpace(meta.Host); host != "" {
		parts = append(parts, p.Dim.Render(host))
	}
	title := strings.TrimSpace(sanitizeFileCompletionText(m.driver.Active().Title))
	if title != "" {
		parts = append(parts, p.Dim.Render(title))
	}
	if len(parts) == 0 {
		return ""
	}
	avail := max(0, width-lipgloss.Width(left)-chromeGap)
	if title != "" {
		last := len(parts) - 1
		room := avail - chromeMetaWidth(parts[:last]) - chromeGap
		if room < minChromeTitleWidth {
			parts = parts[:last]
		} else {
			parts[last] = p.Dim.Render(truncate(title, room))
		}
	}
	for len(parts) > 0 && chromeMetaWidth(parts) > avail {
		parts = parts[:len(parts)-1]
	}
	return strings.Join(parts, strings.Repeat(" ", chromeGap))
}

func chromeMetaWidth(parts []string) int {
	w := 0
	for i, part := range parts {
		if i > 0 {
			w += chromeGap
		}
		w += lipgloss.Width(part)
	}
	return w
}

// joinChromeRow composes one chrome row: the left segment, space padding, and
// the right segment filling the row exactly. Widths use lipgloss.Width so CJK
// cells stay aligned. An empty right segment leaves the left segment alone;
// anything that still does not fit resolves to a truncated left segment.
func joinChromeRow(left, right string, width int) string {
	if width <= 0 {
		return ""
	}
	if right == "" {
		return truncate(left, width)
	}
	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < chromeGap {
		return truncate(left, width)
	}
	return left + strings.Repeat(" ", gap) + right
}

// busyStatus renders the current spinner frame plus the elapsed run time
// measured from Meta.BusySince; a zero timestamp keeps the bare `run` label.
func (m Model) busyStatus(meta surface.Meta) string {
	frame := spinnerFrames[m.spinFrame%len(spinnerFrames)]
	if meta.BusySince.IsZero() {
		return frame + " " + m.translator.T("vivy.tui.run.label", nil)
	}
	elapsed := time.Since(meta.BusySince).Truncate(time.Second)
	if elapsed < 0 {
		elapsed = 0
	}
	if elapsed >= time.Minute {
		return frame + " " + m.translator.T("vivy.tui.run.elapsed", map[string]any{"duration": fmt.Sprintf("%dm%02ds", int(elapsed.Minutes()), int(elapsed.Seconds())%60)})
	}
	return frame + " " + m.translator.T("vivy.tui.run.elapsed", map[string]any{"duration": fmt.Sprintf("%ds", int(elapsed.Seconds()))})
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
	return p.EditorBox.BorderForeground(workingModeStyle(workingMode(m.driver.RunMode(), snapshot.Session.PermissionPreset), p).GetForeground())
}

func workingModeStyle(mode string, p Palette) lipgloss.Style {
	switch mode {
	case "plan":
		return p.ModePlan
	case "readOnly":
		return p.ModeRead
	default:
		return p.ModeSmart
	}
}

func (m Model) renderThinkingIntensity(mode string, p Palette) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "on":
		return p.IntensityHigh.Render(m.translator.T("vivy.tui.intensity.high", nil))
	case "auto":
		return p.IntensityAuto.Render(m.translator.T("vivy.tui.intensity.auto", nil))
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
func (m Model) renderAttachmentChips(attachments []surface.Attachment) string {
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
			name = m.translator.T("vivy.tui.common.image", nil)
		}
		parts = append(parts, m.translator.T("vivy.tui.attachment.image", map[string]any{"name": name}))
	}
	return strings.Join(parts, " ")
}

// renderFileContextChips renders only bounded metadata returned by the
// control plane. Context contents are never printed as part of a history
// bubble or an editor draft.
func (m Model) renderFileContextChips(contexts []surface.FileContext) string {
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
			name = m.translator.T("vivy.tui.common.file", nil)
		}
		parts = append(parts, m.translator.T("vivy.tui.attachment.file", map[string]any{"name": name}))
	}
	return strings.Join(parts, " ")
}

func (m Model) renderShortcutsDialog(l layout, p Palette) string {
	w := max(1, min(l.width-8, 56))
	inner := max(1, w-p.Dialog.GetHorizontalFrameSize())
	rows := []string{
		p.DialogTitle.Render(m.translator.T("vivy.tui.shortcuts.title", nil)),
		"",
		p.HelpKey.Render("shift+tab") + p.DialogBody.Render(m.translator.T("vivy.tui.shortcuts.switchMode", nil)),
		p.HelpKey.Render("shift+h") + p.DialogBody.Render(m.translator.T("vivy.tui.shortcuts.help", nil)),
		p.HelpKey.Render("/") + p.DialogBody.Render(m.translator.T("vivy.tui.shortcuts.slashPalette", nil)),
		p.HelpKey.Render("ctrl+p") + p.DialogBody.Render(m.translator.T("vivy.tui.shortcuts.palette", nil)),
		p.HelpKey.Render("ctrl+s") + p.DialogBody.Render(m.translator.T("vivy.tui.shortcuts.sessions", nil)),
		p.HelpKey.Render("ctrl+n") + p.DialogBody.Render(m.translator.T("vivy.tui.shortcuts.newSession", nil)),
		p.HelpKey.Render("ctrl+l") + p.DialogBody.Render(m.translator.T("vivy.tui.shortcuts.model", nil)),
		p.HelpKey.Render("ctrl+y") + p.DialogBody.Render(m.translator.T("vivy.tui.shortcuts.permission", nil)),
		p.HelpKey.Render("ctrl+t") + p.DialogBody.Render(m.translator.T("vivy.tui.shortcuts.thinking", nil)),
		p.HelpKey.Render("ctrl+o") + p.DialogBody.Render(m.translator.T("vivy.tui.shortcuts.tools", nil)),
		p.HelpKey.Render("ctrl+r") + p.DialogBody.Render(m.translator.T("vivy.tui.shortcuts.reasoning", nil)),
		p.HelpKey.Render("enter") + p.DialogBody.Render(m.translator.T("vivy.tui.shortcuts.send", nil)),
		p.HelpKey.Render("y/n") + p.DialogBody.Render(m.translator.T("vivy.tui.shortcuts.approval", nil)),
		p.HelpKey.Render("esc") + p.DialogBody.Render(m.translator.T("vivy.tui.shortcuts.cancel", nil)),
		p.HelpKey.Render("pgup/pgdn") + p.DialogBody.Render(m.translator.T("vivy.tui.shortcuts.scroll", nil)),
		p.HelpKey.Render("ctrl+→") + p.DialogBody.Render(m.translator.T("vivy.tui.shortcuts.sidebar", nil)),
		p.HelpKey.Render("ctrl+c") + p.DialogBody.Render(m.translator.T("vivy.tui.shortcuts.quit", nil)),
		"",
		p.DialogFooter.Render(m.translator.T("vivy.tui.shortcuts.footer", nil)),
	}
	for i, row := range rows {
		rows[i] = truncate(row, inner)
	}
	return p.Dialog.Width(w).Render(strings.Join(rows, "\n"))
}

func (m Model) renderGateDialog(gate *surface.Gate, l layout, p Palette) string {
	kind := gate.Kind
	if kind == "" || kind == "approval" {
		kind = m.translator.T("vivy.tui.gate.permission", nil)
	} else if kind == "question" {
		kind = m.translator.T("vivy.tui.gate.question", nil)
	}
	w := m.gateDialogWidth(gate, l)
	innerWidth := max(1, w-p.Dialog.GetHorizontalFrameSize())
	titleText := kind + "  ·  " + sanitizeInline(gate.Title)
	title := p.DialogTitle.Render(truncate(titleText, innerWidth))
	lines := []string{title}
	if meta := m.renderGateMetadata(gate, innerWidth, p); len(meta) > 0 {
		lines = append(lines, meta...)
	}
	lines = append(lines, "")
	bodyLines := m.gateBodyLines(gate, l, p, innerWidth)
	viewport := m.gateViewportHeight(gate, l)
	start := min(max(0, m.gateScroll), max(0, len(bodyLines)-viewport))
	end := min(len(bodyLines), start+viewport)
	if len(bodyLines) == 0 {
		bodyLines = []string{p.DialogBody.Render(m.translator.T("vivy.tui.gate.empty", nil))}
		start, end = 0, 1
	}
	lines = append(lines, bodyLines[start:end]...)
	if len(bodyLines) > viewport {
		lines = append(lines, p.DialogFooter.Render(m.translator.T("vivy.tui.gate.lines", map[string]any{"start": start+1, "end": end, "total": len(bodyLines)})))
	}
	lines = append(lines, "")
	helpText := m.translator.T("vivy.tui.gate.approvalFooter", nil)
	if isApprovalDiff(gate) {
		mode := m.translator.T("vivy.tui.gate.unified", nil)
		if m.gateUsesSplit(gate, l) {
			mode = m.translator.T("vivy.tui.gate.split", nil)
		}
		helpText = m.translator.T("vivy.tui.gate.diffFooter", map[string]any{"mode": mode})
	}
	help := p.DialogFooter.Render(truncate(helpText, innerWidth))
	if gate.Kind == "question" {
		help = p.DialogFooter.Render(m.translator.T("vivy.tui.gate.questionFooter", nil))
	}
	if gate.Submitting {
		help = p.DialogFooter.Render(m.translator.T("vivy.tui.gate.submitting", nil))
	}
	lines = append(lines, help)
	inner := lipgloss.JoinVertical(lipgloss.Left, lines...)
	return p.Dialog.Width(w).Render(inner)
}

func (m Model) renderGateMetadata(gate *surface.Gate, width int, p Palette) []string {
	var lines []string
	if action := sanitizeInline(gate.Action); action != "" {
		lines = append(lines, p.DialogFooter.Render(truncate(m.translator.T("vivy.tui.gate.action", map[string]any{"action": action}), width)))
	}
	if target := m.sanitizeApprovalTarget(gate.Target); target != "" {
		lines = append(lines, p.DialogFooter.Render(truncate(m.translator.T("vivy.tui.gate.target", map[string]any{"target": target}), width)))
	}
	if hash := shortPreconditionHash(gate.PreconditionHash); hash != "" {
		lines = append(lines, p.DialogFooter.Render(m.translator.T("vivy.tui.gate.base", map[string]any{"hash": hash})))
	}
	for i, risk := range gate.Risks {
		if i == 3 {
			lines = append(lines, p.ToolFail.Render(m.translator.T("vivy.tui.gate.moreRisks", map[string]any{"count": len(gate.Risks)-i})))
			break
		}
		if risk = sanitizeInline(risk); risk != "" {
			lines = append(lines, p.ToolFail.Render(truncate(m.translator.T("vivy.tui.gate.risk", map[string]any{"risk": risk}), width)))
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
	return textsafe.Inline(text)
}

func (m Model) sanitizeApprovalTarget(text string) string {
	target := sanitizeInline(text)
	if strings.Contains(target, "\\") || strings.Contains(target, ":") || strings.HasPrefix(target, "/") {
		return m.translator.T("vivy.tui.gate.redactedTarget", nil)
	}
	return target
}

func sanitizeMultilineText(text string) string {
	return textsafe.Multiline(text)
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
		return m.renderSplitDiffLines(gate.Preview, width, m.gateHorizontal, p)
	}
	return m.renderUnifiedDiffLines(gate.Preview, width, m.gateHorizontal, p)
}

func (m Model) renderUnifiedDiffLines(preview string, width, horizontal int, p Palette) []string {
	raw := strings.Split(sanitizeMultilineText(preview), "\n")
	adds, dels := visibleDiffStats(raw)
	lines := []string{p.Dim.Render(m.translator.T("vivy.tui.gate.preview", nil)) + p.DiffAdd.Render(fmt.Sprintf("+%d", adds)) + " " + p.DiffDel.Render(fmt.Sprintf("-%d", dels))}
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

func (m Model) renderSplitDiffLines(preview string, width, horizontal int, p Palette) []string {
	raw := strings.Split(sanitizeMultilineText(preview), "\n")
	adds, dels := visibleDiffStats(raw)
	rows := parseSplitDiffRows(raw)
	lines := []string{p.Dim.Render(m.translator.T("vivy.tui.gate.preview", nil)) + p.DiffAdd.Render(fmt.Sprintf("+%d", adds)) + " " + p.DiffDel.Render(fmt.Sprintf("-%d", dels))}
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
	if sanitizeInline(gate.Target) != "" {
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
	title := p.DialogTitle.Render(m.translator.T("vivy.tui.sessions.title", nil))
	filter := m.translator.T("vivy.tui.filter.value", map[string]any{"filter": m.sessionFilter})
	if m.sessionFilter == "" {
		filter = m.translator.T("vivy.tui.sessions.prompt", nil)
	}
	lines := []string{title, p.DialogFooter.Render(filter)}
	if m.sessionLoading {
		lines = append(lines, "", p.DialogFooter.Render(m.translator.T("vivy.tui.sessions.loading", nil)))
	} else if len(rows) == 0 {
		lines = append(lines, "", p.DialogFooter.Render(m.translator.T("vivy.tui.sessions.empty", nil)))
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
				name = m.translator.T("vivy.tui.session.untitled", nil)
			}
			lines = append(lines, style.Render(truncate(marker+name, max(8, l.width-14))))
			lines = append(lines, p.Dim.Render(truncate("   "+row.ID, max(8, l.width-14))))
		}
	}
	lines = append(lines, "")
	if m.sessionRenaming {
		lines = append(lines, p.DialogFooter.Render(m.translator.T("vivy.tui.sessions.rename", map[string]any{"name": m.sessionRenameInput})), p.DialogFooter.Render(m.translator.T("vivy.tui.sessions.renameFooter", nil)))
	} else if m.sessionDeleteID != "" {
		name := m.sessionDeleteID
		for _, row := range rows {
			if row.ID == m.sessionDeleteID {
				name = row.Title
				break
			}
		}
		lines = append(lines, p.PromptWarn.Render(truncate(m.translator.T("vivy.tui.sessions.delete", map[string]any{"name": name}), max(8, l.width-14))), p.DialogFooter.Render(m.translator.T("vivy.tui.sessions.deleteFooter", nil)))
	} else {
		lines = append(lines, p.DialogFooter.Render(m.translator.T("vivy.tui.sessions.footer", nil)))
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
		body := m.translator.T("vivy.tui.confirmation.sessionChange", nil)
		if m.commandConfirmName == "fork" && len(m.commandConfirmArgs) > 0 {
			body = m.translator.T("vivy.tui.confirmation.fork", map[string]any{"message": m.commandConfirmArgs[0]})
		} else if m.commandConfirmName == "rewind" && len(m.commandConfirmArgs) > 0 {
			body = m.translator.T("vivy.tui.confirmation.rewind", map[string]any{"message": m.commandConfirmArgs[0]})
		} else if m.commandConfirmName == "compact" {
			body = m.translator.T("vivy.tui.confirmation.compact", nil)
		}
		inner := strings.Join([]string{
			p.DialogTitle.Render(m.translator.T("vivy.tui.confirmation.title", map[string]any{"command": name})),
			"",
			p.DialogBody.Render(truncate(body, max(8, l.width-14))),
			"",
			p.DialogFooter.Render(m.translator.T("vivy.tui.confirmation.footer", nil)),
		}, "\n")
		w := max(1, min(l.width-8, 72))
		return p.Dialog.Width(w).Render(inner)
	}
	title := m.commandOverlayTitle
	if title == "" {
		title = m.translator.T("vivy.tui.dialog.commands", nil)
	}
	body := m.commandOverlay
	if body == "" {
		body = m.translator.T("vivy.tui.common.done", nil)
	}
	lines := []string{p.DialogTitle.Render(title), ""}
	for _, line := range strings.Split(body, "\n") {
		lines = append(lines, p.DialogBody.Render(truncate(line, max(8, l.width-14))))
	}
	lines = append(lines, "", p.DialogFooter.Render(m.translator.T("vivy.tui.dialog.close", nil)))
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

// middleTruncate shortens s to at most width display cells by keeping a head
// and a tail joined by an ellipsis. The tail receives the larger budget so a
// filename at the end of a long path stays visible. s is plain text: sidebar
// paths are sanitized before they reach this helper.
func middleTruncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= width {
		return s
	}
	const ellipsis = "…"
	budget := width - lipgloss.Width(ellipsis)
	if budget <= 0 {
		// Too narrow for head + ellipsis + tail: keep whatever tail fits.
		return tailOfWidth(s, width)
	}
	headBudget := budget / 3
	head := ansi.Truncate(s, headBudget, "")
	tail := tailOfWidth(s, budget-headBudget)
	return head + ellipsis + tail
}

// tailOfWidth returns the last at most budget display cells of s, cut on
// grapheme boundaries via ansi.TruncateLeft. TruncateLeft never splits a
// cluster, so a boundary crossing a wide rune can overshoot by one cell; the
// budget is retried one cell narrower until the tail fits.
func tailOfWidth(s string, budget int) string {
	if budget <= 0 {
		return ""
	}
	total := lipgloss.Width(s)
	if total <= budget {
		return s
	}
	tail := ansi.TruncateLeft(s, total-budget, "")
	for cells := budget; lipgloss.Width(tail) > budget; {
		cells--
		if cells <= 0 {
			return ""
		}
		tail = ansi.TruncateLeft(s, total-cells, "")
	}
	return tail
}

// pasteGuardChip flags a draft that looks like a giant paste: over the char
// or the line threshold. It is derived from the current draft only, so
// trimming back under the thresholds clears the chip without bookkeeping.
func (m Model) pasteGuardChip(input string) string {
	runes := len([]rune(input))
	lines := strings.Count(input, "\n") + 1
	if runes <= pasteThresholdChars && lines <= pasteThresholdLines {
		return ""
	}
	return m.translator.T("vivy.tui.paste.warning", map[string]any{"lines": lines, "characters": runes})
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

func (m Model) workingModeLabel(mode string) string {
	switch mode {
	case "plan":
		return m.translator.T("vivy.tui.mode.plan", nil)
	case "readOnly":
		return m.translator.T("vivy.tui.mode.readOnly", nil)
	default:
		return m.translator.T("vivy.tui.mode.smart", nil)
	}
}

// stateLabel translates known host status badges, preserving unknown values.
func (m Model) stateLabel(state string) string {
	switch state {
	case "starting":
		return m.translator.T("vivy.tui.state.starting", nil)
	case "initialized":
		return m.translator.T("vivy.tui.state.initialized", nil)
	case "configured":
		return m.translator.T("vivy.tui.state.configured", nil)
	case "error":
		return m.translator.T("vivy.tui.state.error", nil)
	case "pending":
		return m.translator.T("vivy.tui.state.pending", nil)
	case "done":
		return m.translator.T("vivy.tui.state.done", nil)
	case "denied":
		return m.translator.T("vivy.tui.state.denied", nil)
	case "failed":
		return m.translator.T("vivy.tui.state.failed", nil)
	default:
		return state
	}
}

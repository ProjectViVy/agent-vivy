package view

import (
	"strings"

	"agent-vivy/sdk/tui/surface"

	tea "github.com/charmbracelet/bubbletea"
)

// Transcript search (VCP-G3): the "search" action opens a single-line query
// row; matches jump the chat viewport between the containing messages.
// Matching is case-insensitive substring over message content plus tool
// name/preview/result so tool evidence is reachable too.

func (m Model) handleSearchKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		return m, tea.Quit
	case tea.KeyEsc:
		return m.closeSearch(true), nil
	case tea.KeyEnter:
		return m.searchJump(1), nil
	case tea.KeyTab:
		return m.searchJump(1), nil
	case tea.KeyShiftTab:
		return m.searchJump(-1), nil
	case tea.KeyBackspace:
		m.searchQuery = removeLastRune(m.searchQuery)
		m.searchMatches = m.computeSearchMatches()
		if m.searchCursor >= len(m.searchMatches) {
			m.searchCursor = len(m.searchMatches) - 1
		}
		return m, nil
	case tea.KeyRunes:
		m.searchQuery += string(msg.Runes)
		m.searchMatches = m.computeSearchMatches()
		return m, nil
	case tea.KeySpace:
		m.searchQuery += " "
		m.searchMatches = m.computeSearchMatches()
		return m, nil
	}
	return m, nil
}

func (m Model) openSearch() Model {
	if m.searchOpen {
		return m
	}
	m.searchOpen = true
	m.searchQuery = ""
	m.searchMatches = nil
	m.searchCursor = -1
	m.searchSavedScroll = m.chatScroll
	m.searchSavedFollow = m.chatFollow
	m.closeFileCompletion()
	return m
}

// closeSearch exits search mode. restore scrolls the viewport back to where
// it was when search opened (Esc); Enter keeps the landed position.
func (m Model) closeSearch(restore bool) Model {
	m.searchOpen = false
	m.searchQuery = ""
	m.searchMatches = nil
	m.searchCursor = -1
	if restore {
		m.chatAnchorSeg = -1
		m.chatScroll = m.searchSavedScroll
		m.chatFollow = m.searchSavedFollow
		m.clampChatScroll()
		m.captureChatAnchor()
	}
	return m
}

func (m Model) computeSearchMatches() []int {
	query := strings.ToLower(strings.TrimSpace(m.searchQuery))
	if query == "" {
		return nil
	}
	messages := m.driver.ActiveMessages()
	var out []int
	for index, message := range messages {
		if messageMatchesSearch(message, query) {
			out = append(out, index)
		}
	}
	return out
}

func messageMatchesSearch(message surface.Message, query string) bool {
	if strings.Contains(strings.ToLower(message.Content), query) {
		return true
	}
	if tool := message.Tool; tool != nil {
		if strings.Contains(strings.ToLower(tool.ToolName), query) ||
			strings.Contains(strings.ToLower(tool.Preview), query) ||
			strings.Contains(strings.ToLower(tool.Result), query) {
			return true
		}
	}
	return false
}

// searchJump advances the match cursor by delta (wrapping) and scrolls the
// viewport so the matching message's segment tops the visible region.
func (m Model) searchJump(delta int) Model {
	m.searchMatches = m.computeSearchMatches()
	if len(m.searchMatches) == 0 {
		m.searchCursor = -1
		return m
	}
	m.searchCursor = ((m.searchCursor+delta)%len(m.searchMatches) + len(m.searchMatches)) % len(m.searchMatches)
	m.jumpToMessage(m.searchMatches[m.searchCursor])
	return m
}

// jumpToMessage scrolls the transcript so the rendered segment of the given
// message index tops the viewport. Segments align 1:1 with messages whenever
// the history is non-empty (the hero segment only exists below zero).
func (m *Model) jumpToMessage(messageIndex int) {
	l := m.layout()
	width := l.innerW()
	if l.showSidebar {
		width = l.mainW()
	}
	assembly := m.chatSegments(width, m.palette)
	if messageIndex < 0 || messageIndex >= len(assembly.segments) {
		return
	}
	offset := 0
	for _, segment := range assembly.segments[:messageIndex] {
		offset += len(segment)
	}
	m.chatAnchorSeg = -1
	m.chatFollow = false
	m.chatScroll = offset
	m.clampChatScroll()
	m.captureChatAnchor()
}

// jumpToUserMessage moves the viewport to the previous/next user prompt
// relative to the current top line (pi Ctrl+Up/Down).
func (m Model) jumpToUserMessage(delta int) Model {
	messages := m.driver.ActiveMessages()
	if len(messages) == 0 {
		return m
	}
	l := m.layout()
	width := l.innerW()
	if l.showSidebar {
		width = l.mainW()
	}
	assembly := m.chatSegments(width, m.palette)
	starts := make([]int, len(messages))
	offset := 0
	for index := range starts {
		if index < len(assembly.segments) {
			starts[index] = offset
			offset += len(assembly.segments[index])
		}
	}
	current := m.chatScroll
	if m.chatFollow {
		current = assembly.lineCount
	}
	target := -1
	if delta < 0 {
		for index := len(messages) - 1; index >= 0; index-- {
			if messages[index].Role == surface.RoleUser && starts[index] < current {
				target = index
				break
			}
		}
	} else {
		for index := 0; index < len(messages); index++ {
			if messages[index].Role == surface.RoleUser && starts[index] > current {
				target = index
				break
			}
		}
	}
	if target < 0 {
		return m
	}
	m.jumpToMessage(target)
	return m
}

package view

import (
	"encoding/base64"
	"fmt"
	"os"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"agent-vivy/sdk/tui/surface"
)

// treeRow is one rendered row of the /tree navigator: the session node plus
// its layout depth under the fork/clone graph.
type treeRow struct {
	node  surface.TreeNode
	depth int
}

// openTree shows the /tree dialog and kicks the session/tree fetch. The
// kernel owns the graph; this view only orders it for display.
func (m Model) openTree() (Model, tea.Cmd) {
	m.closeFileCompletion()
	m.closeModelPicker()
	m.shortcutsOpen = false
	m.sessionsOpen = false
	m.commandOverlayTitle = ""
	m.commandOverlay = ""
	m.treeOpen = true
	m.treeRows = nil
	m.treeCursor = 0
	m.treeLoading = true
	m.treeError = ""
	return m, m.driver.SessionTree()
}

func (m *Model) applyTreeMsg(msg surface.TreeMsg) {
	if !m.treeOpen {
		return
	}
	m.treeLoading = false
	if msg.Err != nil {
		m.treeError = shortError(msg.Err)
		m.treeRows = nil
		return
	}
	m.treeError = ""
	m.treeRows = flattenTree(msg.Nodes, msg.Edges)
	active := m.driver.Active().ID
	m.treeCursor = 0
	for i, row := range m.treeRows {
		if row.node.SessionID == active {
			m.treeCursor = i
			return
		}
	}
}

func (m Model) handleTreeKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	if msg.Type == tea.KeyCtrlC {
		return m, tea.Quit
	}
	switch msg.Type {
	case tea.KeyEsc:
		m.treeOpen = false
		m.treeError = ""
		return m, nil
	case tea.KeyUp, tea.KeyCtrlP:
		if m.treeCursor > 0 {
			m.treeCursor--
		}
		return m, nil
	case tea.KeyDown, tea.KeyCtrlN:
		if m.treeCursor < len(m.treeRows)-1 {
			m.treeCursor++
		}
		return m, nil
	case tea.KeyEnter:
		if len(m.treeRows) == 0 {
			return m, nil
		}
		id := m.treeRows[m.treeCursor].node.SessionID
		cmd := m.driver.SelectSession(id)
		if cmd == nil {
			m.treeError = m.translator.T("vivy.tui.error.sessionUnavailable", nil)
			return m, nil
		}
		m.treeOpen = false
		m.treeError = ""
		return m, cmd
	}
	return m, nil
}

// flattenTree orders nodes depth-first under their fork/clone parents. Roots
// are nodes whose parent is absent (or outside the bounded snapshot); each
// level sorts by creation time. A visited set bounds pathological cycles.
func flattenTree(nodes []surface.TreeNode, edges []surface.TreeEdge) []treeRow {
	byID := make(map[string]surface.TreeNode, len(nodes))
	for _, node := range nodes {
		byID[node.SessionID] = node
	}
	children := make(map[string][]string, len(nodes))
	for _, edge := range edges {
		if edge.Kind != "fork" {
			continue
		}
		if _, ok := byID[edge.To]; !ok {
			continue
		}
		children[edge.From] = append(children[edge.From], edge.To)
	}
	byCreated := func(ids []string) {
		sort.SliceStable(ids, func(i, j int) bool {
			return byID[ids[i]].CreatedAt < byID[ids[j]].CreatedAt
		})
	}
	var roots []string
	for _, node := range nodes {
		if node.ParentSessionID == "" {
			roots = append(roots, node.SessionID)
			continue
		}
		if _, ok := byID[node.ParentSessionID]; !ok {
			roots = append(roots, node.SessionID)
		}
	}
	byCreated(roots)
	var rows []treeRow
	visited := make(map[string]bool, len(nodes))
	var walk func(id string, depth int)
	walk = func(id string, depth int) {
		if visited[id] {
			return
		}
		visited[id] = true
		node, ok := byID[id]
		if !ok {
			return
		}
		rows = append(rows, treeRow{node: node, depth: depth})
		kids := children[id]
		byCreated(kids)
		for _, kid := range kids {
			walk(kid, depth+1)
		}
	}
	for _, root := range roots {
		walk(root, 0)
	}
	// Nodes unreachable from any root (parent chain into a cycle, or missing
	// root bookkeeping) still render, appended flat in creation order.
	for _, node := range nodes {
		if !visited[node.SessionID] {
			rows = append(rows, treeRow{node: node})
		}
	}
	return rows
}

// copyLastAssistant implements /copy: the newest assistant text is written to
// the terminal clipboard via OSC 52. Terminals that ignore the sequence just
// drop it; the overlay still reports what was (attempted to be) copied.
func (m Model) copyLastAssistant() (Model, tea.Cmd) {
	messages := m.driver.ActiveMessages()
	text := ""
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "assistant" && strings.TrimSpace(messages[i].Content) != "" {
			text = messages[i].Content
			break
		}
	}
	if text == "" {
		return m.showCommandError(fmt.Errorf("%s", m.translator.T("vivy.tui.result.copyEmpty", nil))), nil
	}
	copied := m.translator.T("vivy.tui.result.copied", nil)
	return m, func() tea.Msg {
		seq := "\x1b]52;c;" + base64.StdEncoding.EncodeToString([]byte(text)) + "\a"
		if _, err := fmt.Fprint(os.Stdout, seq); err != nil {
			return surface.CommandResultMsg{Name: "copy", Err: err}
		}
		return surface.CommandResultMsg{Name: "copy", Output: copied}
	}
}

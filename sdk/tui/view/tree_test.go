package view

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"agent-vivy/sdk/tui/command"
	"agent-vivy/sdk/tui/surface"
)

func TestFlattenTreeOrdersDepthFirst(t *testing.T) {
	nodes := []surface.TreeNode{
		{SessionID: "root", Title: "root", CreatedAt: 1},
		{SessionID: "child-b", Title: "b", CreatedAt: 3, ParentSessionID: "root"},
		{SessionID: "child-a", Title: "a", CreatedAt: 2, ParentSessionID: "root"},
		{SessionID: "grand", Title: "g", CreatedAt: 4, ParentSessionID: "child-a"},
		{SessionID: "orphan", Title: "o", CreatedAt: 5, ParentSessionID: "missing"},
	}
	edges := []surface.TreeEdge{
		{From: "root", To: "child-a", Kind: "fork"},
		{From: "root", To: "child-b", Kind: "fork"},
		{From: "child-a", To: "grand", Kind: "fork"},
	}
	rows := flattenTree(nodes, edges)
	var order []string
	for _, row := range rows {
		order = append(order, row.node.SessionID)
	}
	want := []string{"root", "child-a", "grand", "child-b", "orphan"}
	if len(order) != len(want) {
		t.Fatalf("order = %v", order)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("order = %v, want %v", order, want)
		}
	}
	depths := map[string]int{}
	for _, row := range rows {
		depths[row.node.SessionID] = row.depth
	}
	if depths["grand"] != 2 || depths["child-a"] != 1 || depths["orphan"] != 0 {
		t.Fatalf("depths = %v", depths)
	}
}

func TestTreeDialogRendersAndSelects(t *testing.T) {
	driver := &testDriver{
		sessions: []surface.Session{{ID: "root", Title: "Root"}},
		active:   "root",
		sidebar:  surface.Sidebar{Session: surface.Session{ID: "root", Title: "Root"}},
		treeNodes: []surface.TreeNode{
			{SessionID: "root", Title: "Root", CreatedAt: 1},
			{SessionID: "clone-1", Title: "Clone of root", CreatedAt: 2, ParentSessionID: "root"},
		},
		treeEdges: []surface.TreeEdge{{From: "root", To: "clone-1", Kind: "fork"}},
	}
	m := New(driver)
	m.width, m.height = 100, 30

	updated, cmd := m.dispatchCommand(&command.Invocation{Name: "tree"})
	m = updated
	if !m.treeOpen || !m.treeLoading || cmd == nil {
		t.Fatalf("/tree did not open loading dialog: open=%v loading=%v", m.treeOpen, m.treeLoading)
	}
	next, _ := m.Update(cmd())
	m = next.(Model)
	if m.treeLoading || len(m.treeRows) != 2 {
		t.Fatalf("tree did not load: loading=%v rows=%v", m.treeLoading, len(m.treeRows))
	}
	if !strings.Contains(m.View(), "Clone of root") {
		t.Fatalf("child row missing:\n%s", m.View())
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = next.(Model)
	next, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	if m.treeOpen || driver.selected != "clone-1" || cmd == nil {
		t.Fatalf("enter did not switch: open=%v selected=%q", m.treeOpen, driver.selected)
	}
}

func TestTreeDialogEscAndError(t *testing.T) {
	driver := &testDriver{treeErr: errors.New("rpc down")}
	m := New(driver)
	m.width, m.height = 100, 30
	updated, cmd := m.dispatchCommand(&command.Invocation{Name: "tree"})
	m = updated
	next, _ := m.Update(cmd())
	m = next.(Model)
	if m.treeError == "" {
		t.Fatalf("tree error not surfaced: %+v", m.treeRows)
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = next.(Model)
	if m.treeOpen {
		t.Fatal("esc did not close tree dialog")
	}
}

func TestCopyLastAssistantEmptyAndFilled(t *testing.T) {
	driver := &testDriver{
		sessions: []surface.Session{{ID: "s", Title: "S"}},
		active:   "s",
	}
	m := New(driver)
	updated, cmd := m.dispatchCommand(&command.Invocation{Name: "copy"})
	m = updated
	if cmd != nil || !strings.Contains(m.commandOverlay, "no assistant message") {
		t.Fatalf("copy empty: overlay=%q cmd=%v", m.commandOverlay, cmd)
	}

	driver.messages = map[string][]surface.Message{
		"s": {
			{ID: "m1", Role: "user", Content: "hi"},
			{ID: "m2", Role: "assistant", Content: "answer one"},
			{ID: "m3", Role: "tool", Content: "tool out"},
			{ID: "m4", Role: "assistant", Content: "answer two"},
		},
	}
	updated, cmd = m.dispatchCommand(&command.Invocation{Name: "copy"})
	m = updated
	if cmd == nil {
		t.Fatal("copy produced no clipboard command")
	}
}

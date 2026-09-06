package view

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"agent-vivy/sdk/tui/surface"
)

func chatBodyDriver(messages []surface.Message) *testDriver {
	return &testDriver{
		sessions: []surface.Session{{ID: "active", Title: "Current"}},
		active:   "active",
		messages: map[string][]surface.Message{"active": messages},
	}
}

func chatBodyModel(t *testing.T, driver *testDriver) Model {
	t.Helper()
	m := New(driver)
	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	return next.(Model)
}

func TestToolCardCtrlOExpandToggle(t *testing.T) {
	m := chatBodyModel(t, chatBodyDriver([]surface.Message{
		{ID: "t1", Tool: &surface.ToolCard{ToolName: "list_dir", Status: "done", Result: strings.Repeat("entry\n", 20)}},
	}))

	if !strings.Contains(ansi.Strip(m.View()), "more lines") {
		t.Fatalf("default view did not compact the tool result:\n%s", ansi.Strip(m.View()))
	}
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlO})
	m = next.(Model)
	expanded := ansi.Strip(m.View())
	if strings.Contains(expanded, "more lines") {
		t.Fatalf("ctrl+o did not expand the tool result:\n%s", expanded)
	}
	if strings.Count(expanded, "entry") <= compactToolResultLines {
		t.Fatalf("ctrl+o did not reveal lines beyond the %d-line cap: %d", compactToolResultLines, strings.Count(expanded, "entry"))
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlO})
	m = next.(Model)
	if !strings.Contains(ansi.Strip(m.View()), "more lines") {
		t.Fatalf("second ctrl+o did not restore the compact marker:\n%s", ansi.Strip(m.View()))
	}

	optionsModel := New(chatBodyDriver([]surface.Message{
		{ID: "t1", Tool: &surface.ToolCard{ToolName: "list_dir", Status: "done", Result: strings.Repeat("entry\n", 20)}},
	}), Options{DebugToolOutput: true})
	next, _ = optionsModel.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	optionsModel = next.(Model)
	if strings.Contains(ansi.Strip(optionsModel.View()), "more lines") {
		t.Fatalf("DebugToolOutput still compacted the tool result:\n%s", ansi.Strip(optionsModel.View()))
	}
	next, _ = optionsModel.Update(tea.KeyMsg{Type: tea.KeyCtrlO})
	optionsModel = next.(Model)
	if strings.Contains(ansi.Strip(optionsModel.View()), "more lines") {
		t.Fatalf("ctrl+o compacted an already expanded tool result:\n%s", ansi.Strip(optionsModel.View()))
	}
}

func TestReasoningCtrlRCollapse(t *testing.T) {
	m := chatBodyModel(t, chatBodyDriver([]surface.Message{
		{ID: "r1", Role: surface.RoleAssistant, Reasoning: true, Content: "secret-plan-alpha\nsecret-plan-beta\nsecret-plan-gamma"},
	}))

	if !strings.Contains(ansi.Strip(m.View()), "secret-plan-alpha") {
		t.Fatalf("default view did not render the reasoning body:\n%s", ansi.Strip(m.View()))
	}
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlR})
	m = next.(Model)
	collapsed := ansi.Strip(m.View())
	if !strings.Contains(collapsed, "ctrl+r 展开") {
		t.Fatalf("ctrl+r did not collapse reasoning:\n%s", collapsed)
	}
	if strings.Contains(collapsed, "secret-plan-alpha") {
		t.Fatalf("collapsed reasoning leaked body content:\n%s", collapsed)
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlR})
	m = next.(Model)
	if !strings.Contains(ansi.Strip(m.View()), "secret-plan-alpha") {
		t.Fatalf("second ctrl+r did not restore the reasoning body:\n%s", ansi.Strip(m.View()))
	}
}

func TestEmptySessionHero(t *testing.T) {
	m := chatBodyModel(t, chatBodyDriver(nil))
	hero := ansi.Strip(m.View())
	if !strings.Contains(hero, "寻找真心之旅") || !strings.Contains(hero, "ctrl+o 工具输出 · ctrl+r reasoning") {
		t.Fatalf("empty session did not render the hero:\n%s", hero)
	}
	withCwd := chatBodyDriver(nil)
	withCwd.sidebar.CWD = "C:/code/project"
	m = chatBodyModel(t, withCwd)
	if !strings.Contains(ansi.Strip(m.View()), "cwd  C:/code/project") {
		t.Fatalf("hero did not show the working directory:\n%s", ansi.Strip(m.View()))
	}

	m = chatBodyModel(t, chatBodyDriver([]surface.Message{{ID: "m1", Role: surface.RoleAssistant, Content: "hello"}}))
	if strings.Contains(ansi.Strip(m.View()), "ctrl+o 工具输出 · ctrl+r reasoning") {
		t.Fatalf("non-empty session rendered the hero hints:\n%s", ansi.Strip(m.View()))
	}
}

func TestGateBlocksToggles(t *testing.T) {
	driver := chatBodyDriver([]surface.Message{
		{ID: "t1", Tool: &surface.ToolCard{ToolName: "list_dir", Status: "done", Result: strings.Repeat("entry\n", 20)}},
		{ID: "r1", Role: surface.RoleAssistant, Reasoning: true, Content: "secret-plan-alpha"},
	})
	driver.gate = &surface.Gate{Kind: "approval", ID: "approval_1", Title: "write file"}
	m := chatBodyModel(t, driver)

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlO})
	m = next.(Model)
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlR})
	m = next.(Model)
	chat := strings.Join(m.chatLines(80, m.palette), "\n")
	plain := ansi.Strip(chat)
	if !strings.Contains(plain, "more lines") || !strings.Contains(plain, "secret-plan-alpha") {
		t.Fatalf("gate-blocked chat lost compact tool or reasoning body:\n%s", plain)
	}
}

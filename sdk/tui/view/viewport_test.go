package view

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"agent-vivy/sdk/tui/surface"
)

func historyDriver() *testDriver {
	tool := &surface.ToolCard{ToolName: "bash", Status: "done", ToolCallID: "c1", Preview: "bash script", Result: "r1\nr2"}
	messages := []surface.Message{
		{ID: "m1", Role: surface.RoleUser, Content: "first question"},
		{ID: "m2", Role: surface.RoleTool, Tool: tool},
		{ID: "m3", Role: surface.RoleAssistant, Content: "answer one"},
		{ID: "m4", Role: surface.RoleUser, Content: "second question"},
		// Paragraph breaks keep every repeat on its own rendered line: a
		// soft-break fixture would flow-join and change the line arithmetic.
		{ID: "m5", Role: surface.RoleAssistant, Content: strings.Repeat("long answer\n\n", 30)},
	}
	return &testDriver{
		sessions: []surface.Session{{ID: "s1", Title: "one"}},
		active:   "s1",
		messages: map[string][]surface.Message{"s1": messages},
	}
}

func TestPausedViewportAnchorsToMessageWhileHistoryAboveGrows(t *testing.T) {
	d := historyDriver()
	m := New(d)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	m = updated.(Model)
	if !m.chatCanScroll() {
		t.Fatal("history did not overflow the viewport")
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	m = updated.(Model)
	if m.chatFollow || m.chatAnchorSeg < 0 {
		t.Fatalf("page-up did not capture an anchor: follow=%v seg=%d", m.chatFollow, m.chatAnchorSeg)
	}
	paused := m.chatScroll

	// The tool card two messages above the anchor gains two result lines; the
	// numeric offset must move with it instead of drifting the viewport.
	d.messages["s1"][1].Tool.Result = "r1\nr2\nr3\nr4"
	updated, _ = m.Update(surface.RefreshMsg{})
	m = updated.(Model)
	if m.chatFollow {
		t.Fatal("content growth re-enabled follow")
	}
	if grew := m.chatScroll - paused; grew != 2 {
		t.Fatalf("anchored viewport moved %d lines, want 2 (offset=%d paused=%d)", grew, m.chatScroll, paused)
	}

	// The same growth without touching the anchor again stays pinned: a clamp
	// pass with unchanged content must not re-apply the anchor over explicit
	// offsets.
	parked := m.chatScroll
	updated, _ = m.Update(surface.RefreshMsg{})
	m = updated.(Model)
	if m.chatScroll != parked {
		t.Fatalf("clamp pass disturbed the parked offset: %d -> %d", parked, m.chatScroll)
	}
}

func TestChatStampFingerprintsEveryRenderedField(t *testing.T) {
	base := []surface.Message{
		{ID: "m1", Role: surface.RoleUser, Content: "hello"},
		{ID: "m2", Role: surface.RoleAssistant, Content: "hi"},
	}
	baseline := chatStamp(base)
	if chatStamp(append([]surface.Message(nil), base...)) != baseline {
		t.Fatal("unchanged messages changed the stamp")
	}
	for _, tc := range []struct {
		name  string
		mutef func([]surface.Message)
	}{
		{"content", func(ms []surface.Message) { ms[1].Content += "!" }},
		{"id", func(ms []surface.Message) { ms[1].ID = "m2b" }},
		{"role", func(ms []surface.Message) { ms[1].Role = surface.RoleUser }},
		{"streaming", func(ms []surface.Message) { ms[1].Streaming = true }},
		{"reasoning", func(ms []surface.Message) { ms[1].Reasoning = true }},
		{"tool", func(ms []surface.Message) { ms[1].Tool = &surface.ToolCard{ToolName: "bash"} }},
		{"tool result", func(ms []surface.Message) { ms[1].Tool = &surface.ToolCard{ToolName: "bash", Result: "out"} }},
		{"tool status", func(ms []surface.Message) { ms[1].Tool = &surface.ToolCard{ToolName: "bash", Status: "done"} }},
		{"attachment", func(ms []surface.Message) { ms[1].Attachments = []surface.Attachment{{Path: "p.png"}} }},
		{"file context", func(ms []surface.Message) { ms[1].FileContexts = []surface.FileContext{{Path: "a.go"}} }},
		{"message count", func(ms []surface.Message) { ms[1].Content = ms[0].Content; ms[0].ID = "m0" }},
	} {
		mutated := append([]surface.Message(nil), base...)
		tc.mutef(mutated)
		if chatStamp(mutated) == baseline {
			t.Fatalf("%s did not change the stamp", tc.name)
		}
	}
}

func TestChatAssemblyRebuildsOnContentChanges(t *testing.T) {
	d := historyDriver()
	m := New(d)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	m = updated.(Model)

	before := m.chatSegments(100, m.palette)
	beforeCount := before.lineCount
	again := m.chatSegments(100, m.palette)
	if again.lineCount != beforeCount {
		t.Fatal("unchanged history changed its line count")
	}

	// Streaming growth of the last message must become visible: a new
	// paragraph (content + separator) lands in the rebuilt assembly.
	d.messages["s1"][4] = surface.Message{ID: "m5", Role: surface.RoleAssistant, Content: strings.Repeat("long answer\n\n", 30) + "tail\nmore"}
	grown := m.chatSegments(100, m.palette)
	if grown.lineCount <= beforeCount {
		t.Fatalf("streaming growth did not add lines: %d -> %d", beforeCount, grown.lineCount)
	}
	if lines := m.chatLines(100, m.palette); !strings.Contains(strings.Join(lines, "\n"), "tail") {
		t.Fatal("rebuilt assembly lost the new content")
	}

	// A mid-history tool result filling in must become visible too.
	d.messages["s1"][1].Tool.Result = "r1\nr2\nr3"
	filled := m.chatSegments(100, m.palette)
	filledCount := filled.lineCount
	if lines := m.chatLines(100, m.palette); !strings.Contains(strings.Join(lines, "\n"), "r3") {
		t.Fatal("tool result update was not rebuilt into the assembly")
	}

	// Reasoning collapse re-renders multi-line reasoning as one summary row.
	d.messages["s1"][4] = surface.Message{ID: "m5", Role: surface.RoleAssistant, Content: strings.Repeat("deep thought\n\n", 20), Reasoning: true}
	m.reasoningCollapsed = true
	collapsed := m.chatSegments(100, m.palette)
	if collapsed.lineCount >= filledCount {
		t.Fatalf("collapse toggle did not shrink the assembly: %d vs %d", collapsed.lineCount, filledCount)
	}
}

func TestLoadingHistoryNeverPosesAsEmptyConversation(t *testing.T) {
	d := &testDriver{
		sessions: []surface.Session{{ID: "s1", Title: "one"}},
		active:   "s1",
		meta:     surface.Meta{Loading: true},
	}
	m := New(d)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	m = updated.(Model)
	view := ansi.Strip(m.View())
	if !strings.Contains(view, "正在加载会话历史") {
		t.Fatalf("loading history did not show the loading row:\n%s", view)
	}
	if strings.Contains(view, "寻找真心之旅") {
		t.Fatalf("loading history rendered the empty-conversation hero:\n%s", view)
	}

	d.meta = surface.Meta{}
	updated, _ = m.Update(surface.RefreshMsg{})
	m = updated.(Model)
	if !strings.Contains(ansi.Strip(m.View()), "寻找真心之旅") {
		t.Fatalf("empty session lost its hero:\n%s", ansi.Strip(m.View()))
	}
}

package view

import (
	"io"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"agent-vivy/sdk/tui/command"
	"agent-vivy/sdk/tui/surface"
)

type fileCompletionDriver struct {
	*testDriver
	request uint64
	query   string
	files   []surface.FileContext
	trunc   bool
	err     error
}

func (d *fileCompletionDriver) SupportsCapability(name string) bool {
	return name == "project-context.list"
}

func (d *fileCompletionDriver) CompleteProjectFiles(request uint64, query string) tea.Cmd {
	d.request, d.query = request, query
	files := append([]surface.FileContext(nil), d.files...)
	return func() tea.Msg {
		return surface.ProjectFilesMsg{Request: request, Query: query, Files: files, Truncated: d.trunc, Err: d.err}
	}
}

func TestFileCompletionUsesAsyncServerCatalogAndRejectsStaleResults(t *testing.T) {
	d := &fileCompletionDriver{testDriver: &testDriver{}, files: []surface.FileContext{{Path: "README.md"}}}
	m := New(d)
	updated, first := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'@'}})
	m = updated.(Model)
	firstRequest := m.fileCompletionRequest
	if !m.fileCompletionOpen || first == nil || m.fileCompletionQuery != "" {
		t.Fatalf("initial completion open=%v query=%q cmd=%v", m.fileCompletionOpen, m.fileCompletionQuery, first != nil)
	}
	updated, second := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("REA")})
	m = updated.(Model)
	secondRequest := m.fileCompletionRequest
	if second == nil || m.fileCompletionQuery != "REA" {
		t.Fatalf("updated query=%q cmd=%v", m.fileCompletionQuery, second != nil)
	}
	_, _ = m.Update(fileCompletionStartMsg{Request: firstRequest, Query: "", SessionID: m.fileCompletionSessionID})
	if d.request != 0 {
		t.Fatal("stale debounce started a server request")
	}
	_, _ = m.Update(fileCompletionStartMsg{Request: secondRequest, Query: "REA", SessionID: m.fileCompletionSessionID})
	if d.request != secondRequest || d.query != "REA" {
		t.Fatalf("latest debounce request=%d query=%q", d.request, d.query)
	}
	updated, _ = m.Update(surface.ProjectFilesMsg{Request: firstRequest, Query: "", Files: d.files})
	m = updated.(Model)
	if len(m.fileCompletionFiles) != 0 || !m.fileCompletionLoading {
		t.Fatal("stale completion response mutated current popup")
	}
	updated, _ = m.Update(surface.ProjectFilesMsg{Request: secondRequest, Query: "REA", Files: d.files})
	m = updated.(Model)
	if m.fileCompletionLoading || len(m.filteredProjectFiles()) != 1 || !strings.Contains(m.View(), "README.md") {
		t.Fatalf("current response not rendered: %+v\n%s", m.fileCompletionFiles, m.View())
	}
}

func TestFileCompletionSelectsOnlyTrailingTokenAndQuotesRoundTrip(t *testing.T) {
	d := &fileCompletionDriver{testDriver: &testDriver{}, files: []surface.FileContext{{Path: "docs/design notes.md"}}}
	m := New(d)
	m.input = "compare @README.md with @docs/des"
	updated, cmd := m.refreshFileCompletion()
	m = updated
	if cmd == nil {
		t.Fatal("completion debounce was not scheduled")
	}
	updatedTea, _ := m.Update(surface.ProjectFilesMsg{Request: m.fileCompletionRequest, Query: m.fileCompletionQuery, Files: d.files})
	m = updatedTea.(Model)
	updatedTea, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updatedTea.(Model)
	want := `compare @README.md with @"docs/design notes.md" `
	if m.input != want || m.fileCompletionOpen {
		t.Fatalf("selected input=%q open=%v", m.input, m.fileCompletionOpen)
	}
	parsed, err := command.Parse(m.input)
	if err != nil || !reflect.DeepEqual(parsed.ContextPaths, []string{"README.md", "docs/design notes.md"}) {
		t.Fatalf("selected reference round trip = %+v, %v", parsed, err)
	}
}

func TestFileCompletionEscapeLiteralAndSessionFence(t *testing.T) {
	d := &fileCompletionDriver{testDriver: &testDriver{}, files: []surface.FileContext{{Path: "README.md"}}}
	m := New(d)
	m.input = "inspect @REA"
	m, _ = m.refreshFileCompletion()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.fileCompletionOpen || m.input != "inspect @REA" {
		t.Fatalf("escape open=%v input=%q", m.fileCompletionOpen, m.input)
	}
	m.input = "@@literal"
	m, cmd := m.refreshFileCompletion()
	if m.fileCompletionOpen || cmd != nil {
		t.Fatal("literal @@ opened file completion")
	}
	m.input = "inspect @R"
	m, _ = m.refreshFileCompletion()
	d.sessions = []surface.Session{{ID: "new"}}
	d.active = "new"
	updated, _ = m.Update(surface.RefreshMsg{})
	m = updated.(Model)
	if m.fileCompletionOpen {
		t.Fatal("active session change left stale file popup open")
	}
}

func TestFileCompletionGatePriorityAndUnsafeCandidate(t *testing.T) {
	d := &fileCompletionDriver{testDriver: &testDriver{}, files: []surface.FileContext{
		{Path: "bad\x1b[31m.md"}, {Path: "../escape.md"}, {Path: "C:/host.txt"}, {Path: "/absolute.md"}, {Path: "negative.md", Size: -1}, {Path: "safe.md"}, {Path: "safe.md"},
	}, trunc: true}
	m := New(d)
	m.width, m.height = 32, 10
	m.input = "inspect @"
	m, cmd := m.refreshFileCompletion()
	if cmd == nil {
		t.Fatal("completion debounce was not scheduled")
	}
	updated, _ := m.Update(surface.ProjectFilesMsg{Request: m.fileCompletionRequest, Query: m.fileCompletionQuery, Files: d.files, Truncated: d.trunc})
	m = updated.(Model)
	if len(m.filteredProjectFiles()) != 1 || !strings.Contains(m.View(), "partial") || strings.Contains(m.View(), "31m") {
		t.Fatalf("unsafe/truncated popup mismatch:\n%s", m.View())
	}
	d.gate = &surface.Gate{Kind: "approval", ID: "g1"}
	updated, _ = m.Update(surface.RefreshMsg{})
	m = updated.(Model)
	if m.fileCompletionOpen || !strings.Contains(m.View(), "permission") {
		t.Fatalf("gate did not preempt completion:\n%s", m.View())
	}
}

func TestFileCompletionRunsThroughBubbleTeaProgram(t *testing.T) {
	d := &fileCompletionDriver{testDriver: &testDriver{}, files: []surface.FileContext{{Path: "README.md", Size: 7}}}
	program := tea.NewProgram(New(d), tea.WithInput(nil), tea.WithOutput(io.Discard), tea.WithoutRenderer())
	type result struct {
		model tea.Model
		err   error
	}
	done := make(chan result, 1)
	go func() {
		model, err := program.Run()
		done <- result{model: model, err: err}
	}()
	program.Send(tea.WindowSizeMsg{Width: 80, Height: 24})
	program.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'@'}})
	program.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("REA")})
	time.Sleep(300 * time.Millisecond) // includes the 120ms debounce and fake RPC command
	program.Send(tea.KeyMsg{Type: tea.KeyEnter})
	program.Quit()
	select {
	case got := <-done:
		if got.err != nil {
			t.Fatal(got.err)
		}
		model := got.model.(Model)
		if model.input != "@README.md " || model.fileCompletionOpen {
			t.Fatalf("program result input=%q open=%v", model.input, model.fileCompletionOpen)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Bubble Tea completion smoke timed out")
	}
}

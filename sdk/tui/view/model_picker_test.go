package view

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"agent-vivy/sdk/tui/surface"
)

type modelPickerDriver struct {
	*testDriver
	available bool
	catalog   surface.ModelCatalog
	refresh   uint64
	selected  surface.ModelOption
	selectReq uint64
	selectErr error
}

func (d *modelPickerDriver) SupportsModelSelection() bool { return d.available }
func (d *modelPickerDriver) ModelCatalog() surface.ModelCatalog {
	out := d.catalog
	out.Options = append([]surface.ModelOption(nil), out.Options...)
	return out
}
func (d *modelPickerDriver) RefreshModels(request uint64) tea.Cmd {
	d.refresh = request
	return func() tea.Msg { return surface.ModelsMsg{Request: request, Catalog: d.ModelCatalog()} }
}
func (d *modelPickerDriver) SelectModel(request uint64, option surface.ModelOption) tea.Cmd {
	d.selectReq = request
	d.selected = option
	if d.selectErr != nil {
		return func() tea.Msg { return surface.ModelSelectedMsg{Request: request, Option: option, Err: d.selectErr} }
	}
	for i := range d.catalog.Options {
		d.catalog.Options[i].Current = d.catalog.Options[i].Provider == option.Provider && d.catalog.Options[i].Model == option.Model && d.catalog.Options[i].BaseURL == option.BaseURL
	}
	return func() tea.Msg {
		return surface.ModelSelectedMsg{Request: request, Option: option, Catalog: d.ModelCatalog()}
	}
}
func (d *modelPickerDriver) Handle(msg tea.Msg) tea.Cmd {
	if models, ok := msg.(surface.ModelsMsg); ok && models.Err == nil && models.Request == d.refresh {
		d.catalog = models.Catalog
	}
	if selected, ok := msg.(surface.ModelSelectedMsg); ok && selected.Err == nil && selected.Request == d.selectReq {
		d.catalog = selected.Catalog
	}
	return d.testDriver.Handle(msg)
}

func runSingleCmd(t *testing.T, cmd tea.Cmd) tea.Msg {
	t.Helper()
	if cmd == nil {
		t.Fatal("expected command")
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		if len(batch) != 1 {
			t.Fatalf("batch size = %d, want 1", len(batch))
		}
		msg = batch[0]()
	}
	return msg
}

func modelPickerFixture() *modelPickerDriver {
	return &modelPickerDriver{
		testDriver: &testDriver{},
		available:  true,
		catalog: surface.ModelCatalog{Options: []surface.ModelOption{
			{Provider: "openai", Model: "gpt-current", DisplayName: "OpenAI", Current: true},
			{Provider: "compatible", Model: "model-next", BaseURL: "https://secret.invalid/v1", DisplayName: "Custom"},
		}},
	}
}

func TestModelPickerCtrlLOpensFiltersAndCommitsSelection(t *testing.T) {
	d := modelPickerFixture()
	m := New(d)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlL})
	m = updated.(Model)
	if !m.modelPickerOpen || !m.modelPickerLoading || d.refresh == 0 {
		t.Fatalf("picker state = open:%v loading:%v request:%d", m.modelPickerOpen, m.modelPickerLoading, d.refresh)
	}
	updated, _ = m.Update(runSingleCmd(t, cmd))
	m = updated.(Model)
	view := m.View()
	if !strings.Contains(view, "gpt-current") || !strings.Contains(view, "model-next") || strings.Contains(view, "secret.invalid") {
		t.Fatalf("catalog rendering leaked or lost data:\n%s", view)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("next")})
	m = updated.(Model)
	updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if cmd == nil || !m.modelPickerSelecting || d.selected.Model != "model-next" || d.selected.BaseURL == "" {
		t.Fatalf("selection = %+v selecting=%v cmd=%v", d.selected, m.modelPickerSelecting, cmd != nil)
	}
	updated, _ = m.Update(runSingleCmd(t, cmd))
	m = updated.(Model)
	if m.modelPickerOpen || !d.catalog.Options[1].Current {
		t.Fatalf("successful selection did not close/commit: open=%v catalog=%+v", m.modelPickerOpen, d.catalog)
	}
}

func TestModelPickerFailureAndStaleResultsPreserveDialog(t *testing.T) {
	d := modelPickerFixture()
	d.selectErr = fmt.Errorf("server refused model change")
	m := New(d)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlL})
	m = updated.(Model)
	updated, _ = m.Update(runSingleCmd(t, cmd))
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(Model)
	updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	request := m.modelPickerRequest
	updated, _ = m.Update(runSingleCmd(t, cmd))
	m = updated.(Model)
	if !m.modelPickerOpen || !strings.Contains(m.View(), "server refused model change") || d.catalog.Options[0].Current == false {
		t.Fatalf("failed selection lost dialog/current model:\n%s", m.View())
	}
	updated, _ = m.Update(surface.ModelSelectedMsg{Request: request - 1, Catalog: surface.ModelCatalog{}})
	m = updated.(Model)
	if !m.modelPickerOpen {
		t.Fatal("stale result closed the picker")
	}
}

func TestModelPickerSelectionCannotBeEscapedAndLabelsAreTerminalSafe(t *testing.T) {
	d := modelPickerFixture()
	d.catalog.Options[1].DisplayName = "\x1b[31mEvil\nProvider\u202e"
	d.catalog.Options[1].Model = "model\nnext\x1b[0m"
	m := New(d)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlL})
	m = updated.(Model)
	updated, _ = m.Update(runSingleCmd(t, cmd))
	m = updated.(Model)
	view := m.View()
	if strings.Contains(view, "\x1b[31m") || strings.Contains(view, "\u202e") || strings.Contains(view, "model\nnext") {
		t.Fatalf("unsafe model label reached terminal:\n%s", view)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(Model)
	updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if cmd == nil || !m.modelPickerSelecting {
		t.Fatal("selection did not start")
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if !m.modelPickerOpen || !m.modelPickerSelecting {
		t.Fatal("Esc exposed the editor during model selection")
	}
}

func TestModelPickerFailsClosedForBusyQueuedReadOnlyAndUnavailable(t *testing.T) {
	for _, tc := range []struct {
		name      string
		configure func(*modelPickerDriver)
		want      string
	}{
		{name: "busy", configure: func(d *modelPickerDriver) { d.meta.Busy = true }, want: "finish or cancel"},
		{name: "queued", configure: func(d *modelPickerDriver) { d.meta.Queued = 1 }, want: "finish or cancel"},
		{name: "unavailable", configure: func(d *modelPickerDriver) { d.available = false }, want: "unavailable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := modelPickerFixture()
			tc.configure(d)
			m := New(d)
			// Observe an already-busy driver once so the guarded key below is
			// not the first busy observation (which schedules a view-owned
			// spinner tick cmd).
			updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
			m = updated.(Model)
			updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlL})
			m = updated.(Model)
			if cmd != nil || m.modelPickerOpen || !strings.Contains(m.View(), tc.want) {
				t.Fatalf("guard failed cmd=%v open=%v:\n%s", cmd != nil, m.modelPickerOpen, m.View())
			}
		})
	}

	d := modelPickerFixture()
	d.catalog.ReadOnly = true
	m := New(d)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlL})
	m = updated.(Model)
	updated, _ = m.Update(runSingleCmd(t, cmd))
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(Model)
	updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if cmd != nil || !m.modelPickerOpen || !strings.Contains(m.View(), "read-only") || d.selectReq != 0 {
		t.Fatalf("read-only picker selected: request=%d\n%s", d.selectReq, m.View())
	}
}

func TestSlashModelUsesFilterAndGateClosesPicker(t *testing.T) {
	d := modelPickerFixture()
	m := New(d)
	m.input = "/model next"
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if cmd == nil || !m.modelPickerOpen || m.modelPickerFilter != "next" {
		t.Fatalf("slash model failed: open=%v filter=%q", m.modelPickerOpen, m.modelPickerFilter)
	}
	d.gate = &surface.Gate{Kind: "approval", ID: "gate"}
	updated, _ = m.Update(surface.RefreshMsg{})
	m = updated.(Model)
	if m.modelPickerOpen {
		t.Fatal("asynchronous gate did not close model picker")
	}
}

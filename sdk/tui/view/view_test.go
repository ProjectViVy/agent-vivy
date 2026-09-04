package view

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"agent-vivy/sdk/tui/surface"
)

type testDriver struct {
	sessions    []surface.Session
	active      string
	busy        bool
	sidebar     surface.Sidebar
	selected    string
	rename      string
	deleted     string
	gate        *surface.Gate
	sendBlocked bool
	sent        string
	thinking    string
}

func (d *testDriver) Sessions() []surface.Session {
	return append([]surface.Session(nil), d.sessions...)
}
func (d *testDriver) Active() surface.Session {
	for _, session := range d.sessions {
		if session.ID == d.active {
			return session
		}
	}
	return surface.Session{}
}
func (d *testDriver) ActiveMessages() []surface.Message { return nil }
func (d *testDriver) PendingGate() *surface.Gate        { return d.gate }
func (d *testDriver) Meta() surface.Meta                { return surface.Meta{Mode: "test", Busy: d.busy} }
func (d *testDriver) Init() tea.Cmd                     { return nil }
func (d *testDriver) Handle(msg tea.Msg) tea.Cmd {
	if sessions, ok := msg.(surface.SessionsMsg); ok && sessions.Err == nil {
		if sessions.Action == "rename" {
			for i := range d.sessions {
				if d.sessions[i].ID == sessions.ID {
					d.sessions[i] = sessions.Session
				}
			}
		}
		if sessions.Action == "delete" {
			rows := d.sessions[:0]
			for _, session := range d.sessions {
				if session.ID != sessions.ID {
					rows = append(rows, session)
				}
			}
			d.sessions = rows
		}
	}
	return nil
}
func (d *testDriver) MoveSession(int) tea.Cmd   { return nil }
func (d *testDriver) NewSession(string) tea.Cmd { return nil }
func (d *testDriver) Send(text string) tea.Cmd {
	if d.sendBlocked {
		return nil
	}
	d.sent = text
	return func() tea.Msg { return surface.RefreshMsg{} }
}
func (d *testDriver) DecideApproval(string) tea.Cmd { return nil }
func (d *testDriver) AnswerQuestion(string) tea.Cmd { return nil }
func (d *testDriver) SetPermission(string) tea.Cmd  { return nil }
func (d *testDriver) ClearQueue() bool              { return false }
func (d *testDriver) Cancel() tea.Cmd               { return nil }
func (d *testDriver) Sidebar() surface.Sidebar      { return d.sidebar }
func (d *testDriver) ThinkingMode() string {
	if d.thinking == "" {
		return "auto"
	}
	return d.thinking
}
func (d *testDriver) SetThinkingMode(mode string) error {
	if mode == "on" && (!d.sidebar.HasContext || !d.sidebar.Context.ThinkingSupported) {
		return fmt.Errorf("extended thinking is unavailable for the active model")
	}
	d.thinking = mode
	return nil
}
func (d *testDriver) RefreshSessions() tea.Cmd {
	return func() tea.Msg { return surface.SessionsMsg{Action: "list", Sessions: d.Sessions()} }
}
func (d *testDriver) SelectSession(id string) tea.Cmd {
	d.selected = id
	d.active = id
	return func() tea.Msg { return surface.RefreshMsg{} }
}
func (d *testDriver) RenameSession(id, title string) tea.Cmd {
	d.rename = id + ":" + title
	var session surface.Session
	for _, item := range d.sessions {
		if item.ID == id {
			session = item
		}
	}
	session.Title = title
	return func() tea.Msg { return surface.SessionsMsg{Action: "rename", ID: id, Session: session} }
}
func (d *testDriver) DeleteSession(id string) tea.Cmd {
	d.deleted = id
	return func() tea.Msg { return surface.SessionsMsg{Action: "delete", ID: id} }
}

// bareDriver deliberately implements only the required surface.Driver. It
// verifies that the shared view fails closed when a packed/custom driver has
// not opted into session mutations yet.
type bareDriver struct{ inner *testDriver }

func (d *bareDriver) Sessions() []surface.Session       { return d.inner.Sessions() }
func (d *bareDriver) Active() surface.Session           { return d.inner.Active() }
func (d *bareDriver) ActiveMessages() []surface.Message { return nil }
func (d *bareDriver) PendingGate() *surface.Gate        { return nil }
func (d *bareDriver) Meta() surface.Meta                { return d.inner.Meta() }
func (d *bareDriver) Init() tea.Cmd                     { return nil }
func (d *bareDriver) Handle(tea.Msg) tea.Cmd            { return nil }
func (d *bareDriver) MoveSession(int) tea.Cmd           { return nil }
func (d *bareDriver) NewSession(string) tea.Cmd         { return nil }
func (d *bareDriver) Send(string) tea.Cmd               { return nil }
func (d *bareDriver) DecideApproval(string) tea.Cmd     { return nil }
func (d *bareDriver) AnswerQuestion(string) tea.Cmd     { return nil }
func (d *bareDriver) SetPermission(string) tea.Cmd      { return nil }
func (d *bareDriver) ClearQueue() bool                  { return false }
func (d *bareDriver) Cancel() tea.Cmd                   { return nil }

func TestComputeLayoutUsesBothCrushBreakpoints(t *testing.T) {
	for _, tc := range []struct {
		width, height int
		wide          bool
	}{
		{120, 36, true},
		{119, 30, false},
		{120, 29, false},
		{120, 30, true},
	} {
		got := computeLayout(tc.width, tc.height)
		if got.showSidebar != tc.wide {
			t.Fatalf("layout(%d,%d).showSidebar = %v, want %v", tc.width, tc.height, got.showSidebar, tc.wide)
		}
		if tc.wide && got.sidebarW != 32 {
			t.Fatalf("wide sidebar width = %d, want 32", got.sidebarW)
		}
	}
}

func TestSidebarDoesNotRenderSessionCollection(t *testing.T) {
	driver := &testDriver{
		sessions: []surface.Session{{ID: "active", Title: "Current", PermissionPreset: "smart"}, {ID: "other", Title: "Other"}},
		active:   "active",
		sidebar: surface.Sidebar{
			Session:    surface.Session{ID: "active", Title: "Current", PermissionPreset: "smart"},
			HasContext: true,
			Context:    surface.Context{FeedTokens: 1200, ModelLimitTokens: 8000, TotalMessages: 3},
		},
	}
	m := New(driver)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	view := updated.(Model).View()
	if !strings.Contains(view, "Current") || !strings.Contains(view, "1.2k / 8.0k tokens") {
		t.Fatalf("sidebar omitted active/context facts:\n%s", view)
	}
	if strings.Contains(view, "Other") {
		t.Fatalf("inactive session leaked into sidebar:\n%s", view)
	}
}

func TestImageHistoryRenderingUsesMetadataOnlyChips(t *testing.T) {
	lines := renderMessage(surface.Message{
		Role:        surface.RoleUser,
		Content:     "look",
		Attachments: []surface.Attachment{{Name: "photo.png", MimeType: "image/png", Size: 42}},
	}, 80, DefaultPalette())
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "look") || !strings.Contains(joined, "[image: photo.png]") {
		t.Fatalf("attachment chip missing: %s", joined)
	}
	if strings.Contains(joined, "data:") || strings.Contains(joined, "base64") {
		t.Fatalf("history rendered raw attachment payload: %s", joined)
	}
}

func TestFileContextHistoryRenderingUsesMetadataOnlyChips(t *testing.T) {
	lines := renderMessage(surface.Message{
		Role:         surface.RoleUser,
		Content:      "inspect",
		FileContexts: []surface.FileContext{{Path: "README.md", Name: "README.md", Size: 42}},
	}, 80, DefaultPalette())
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "inspect") || !strings.Contains(joined, "[file: README.md]") {
		t.Fatalf("file context chip missing: %s", joined)
	}
	if strings.Contains(joined, "content:") || strings.Contains(joined, "base64") {
		t.Fatalf("file context body leaked into history: %s", joined)
	}
}

func TestSessionsDialogRefreshesFiltersAndSelects(t *testing.T) {
	driver := &testDriver{
		sessions: []surface.Session{{ID: "active", Title: "Current"}, {ID: "other", Title: "Other task"}},
		active:   "active",
		sidebar:  surface.Sidebar{Session: surface.Session{ID: "active", Title: "Current"}},
	}
	m := New(driver)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	m = updated.(Model)
	if !m.sessionsOpen || cmd == nil {
		t.Fatalf("ctrl+s did not open dialog: open=%v cmd=%v", m.sessionsOpen, cmd != nil)
	}
	updated, _ = m.Update(cmd())
	m = updated.(Model)
	if !strings.Contains(m.View(), "▸ Current") {
		t.Fatalf("current session was not preselected:\n%s", m.View())
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("ohr tk")})
	m = updated.(Model)
	if !strings.Contains(m.View(), "Other task") {
		t.Fatalf("title filter hid matching session:\n%s", m.View())
	}
	updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if m.sessionsOpen || cmd == nil || driver.selected != "other" {
		t.Fatalf("filtered selection failed: open=%v cmd=%v selected=%q", m.sessionsOpen, cmd != nil, driver.selected)
	}
}

func TestFuzzyContainsIsOrderedAndUnicodeSafe(t *testing.T) {
	if !fuzzyContains("另一个会话 🚀", "另会🚀") {
		t.Fatal("ordered Unicode subsequence should match")
	}
	if fuzzyContains("other task", "task other") {
		t.Fatal("out-of-order query should not match")
	}
}

func TestPendingGateCannotBeHiddenBySessionsDialog(t *testing.T) {
	driver := &testDriver{
		sessions: []surface.Session{{ID: "active", Title: "Current"}},
		active:   "active",
		gate:     &surface.Gate{Kind: "approval", ID: "approval_1", Title: "write file"},
	}
	m := New(driver)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	m = updated.(Model)
	if m.sessionsOpen || cmd != nil {
		t.Fatalf("ctrl+s hid a pending gate: open=%v cmd=%v", m.sessionsOpen, cmd != nil)
	}
}

func TestGateArrivingWhileSessionsOpenTakesPriority(t *testing.T) {
	driver := &testDriver{
		sessions: []surface.Session{{ID: "active", Title: "Current"}},
		active:   "active",
	}
	m := New(driver)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	m = updated.(Model)
	driver.gate = &surface.Gate{Kind: "approval", ID: "approval_1", Title: "write file", Body: "confirm"}
	if view := m.View(); !strings.Contains(view, "permission") || strings.Contains(view, "filter title") {
		t.Fatalf("gate did not replace sessions overlay:\n%s", view)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	m = updated.(Model)
	if m.sessionsOpen {
		t.Fatal("session dialog remained interactive behind an arriving gate")
	}
}

func TestRejectedSendKeepsDraft(t *testing.T) {
	driver := &testDriver{sendBlocked: true}
	m := New(driver)
	m.input = "keep this draft"
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if cmd != nil || m.input != "keep this draft" {
		t.Fatalf("rejected send lost draft: cmd=%v input=%q", cmd != nil, m.input)
	}
}

func TestSessionsDialogIgnoresOlderMutationResult(t *testing.T) {
	driver := &testDriver{
		sessions: []surface.Session{{ID: "active", Title: "Current"}},
		active:   "active",
	}
	m := New(driver)
	m.sessionsOpen = true
	m.sessionRows = driver.Sessions()
	m.applySessionsMsg(surface.SessionsMsg{Action: "rename", Request: 2, ID: "active", Session: surface.Session{ID: "active", Title: "Newest"}})
	m.applySessionsMsg(surface.SessionsMsg{Action: "list", Request: 1, Sessions: []surface.Session{{ID: "active", Title: "Stale"}}})
	if got := m.sessionRows[0].Title; got != "Newest" {
		t.Fatalf("older session response overwrote newer mutation: %q", got)
	}
}

func TestSessionsDialogRenameAndDeleteUseSingleConfirm(t *testing.T) {
	driver := &testDriver{
		sessions: []surface.Session{{ID: "active", Title: "Current"}, {ID: "other", Title: "Other"}},
		active:   "active",
	}
	m := New(driver)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	m = updated.(Model)
	updated, _ = m.Update(cmd())
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlR})
	m = updated.(Model)
	if !m.sessionRenaming {
		t.Fatal("ctrl+r did not enter rename mode")
	}
	m.sessionRenameInput = "Renamed"
	updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if !m.sessionActionBusy || cmd == nil {
		t.Fatal("rename did not enter submitting state")
	}
	updated, _ = m.Update(cmd())
	m = updated.(Model)
	if m.sessionRenaming || !strings.Contains(m.View(), "Renamed") || driver.rename != "active:Renamed" {
		t.Fatalf("rename result not applied: renaming=%v rename=%q", m.sessionRenaming, driver.rename)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlX})
	m = updated.(Model)
	if m.sessionDeleteID == "" {
		t.Fatal("ctrl+x did not open delete confirmation")
	}
	updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	m = updated.(Model)
	if m.sessionDeleteID != "" || cmd != nil {
		t.Fatal("n should cancel delete without an RPC")
	}
}

func TestSessionsDialogDeleteConfirmAndBusyRefusal(t *testing.T) {
	driver := &testDriver{
		sessions: []surface.Session{{ID: "active", Title: "Current"}, {ID: "other", Title: "Other"}},
		active:   "active",
	}
	m := New(driver)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	m = updated.(Model)
	updated, _ = m.Update(cmd())
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlX})
	m = updated.(Model)
	updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	m = updated.(Model)
	if !m.sessionActionBusy || cmd == nil || driver.deleted != "active" {
		t.Fatalf("delete confirmation did not submit: busy=%v cmd=%v id=%q", m.sessionActionBusy, cmd != nil, driver.deleted)
	}
	updated, _ = m.Update(cmd())
	m = updated.(Model)
	if m.sessionDeleteID != "" || m.sessionActionBusy || strings.Contains(m.View(), "Current") {
		t.Fatalf("delete result not applied: id=%q busy=%v", m.sessionDeleteID, m.sessionActionBusy)
	}

	busyDriver := &testDriver{
		sessions: []surface.Session{{ID: "active", Title: "Current"}, {ID: "other", Title: "Other"}},
		active:   "active",
		busy:     true,
	}
	m = New(busyDriver)
	updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	m = updated.(Model)
	updated, _ = m.Update(cmd())
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlX})
	m = updated.(Model)
	if m.sessionDeleteID != "" || !strings.Contains(m.sessionError, "run is in progress") {
		t.Fatalf("busy active delete was not refused: id=%q err=%q", m.sessionDeleteID, m.sessionError)
	}
}

func TestSessionsDialogCancelAndMissingControllerAreSafe(t *testing.T) {
	driver := &testDriver{
		sessions: []surface.Session{{ID: "active", Title: "Current"}},
		active:   "active",
	}
	m := New(driver)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	m = updated.(Model)
	updated, _ = m.Update(cmd())
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlR})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.sessionRenaming || m.sessionRenameInput != "" {
		t.Fatal("escape did not cancel rename")
	}

	bare := &bareDriver{inner: &testDriver{
		sessions: []surface.Session{{ID: "active", Title: "Current"}},
		active:   "active",
	}}
	m = New(bare)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if !m.sessionsOpen || !strings.Contains(m.sessionError, "unavailable") {
		t.Fatalf("missing controller was not handled safely: open=%v err=%q", m.sessionsOpen, m.sessionError)
	}
}

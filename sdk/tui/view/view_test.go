package view

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"agent-vivy/sdk/tui/surface"
)

func TestWrapTextPreservesStreamingTextAndWrapsCJK(t *testing.T) {
	tests := []struct {
		name  string
		text  string
		width int
		want  []string
	}{
		{name: "continuous Chinese", text: "这是一句话", width: 20, want: []string{"这是一句话"}},
		{name: "CJK cell wrap", text: "这是一句话", width: 8, want: []string{"这是一句", "话"}},
		{name: "exact spaces", text: "a  b c ", width: 20, want: []string{"a  b c "}},
		{name: "safe controls", text: "a\tb\r\x1b[31mc", width: 20, want: []string{"a    bc"}},
		{name: "newlines", text: "甲\n\n乙", width: 8, want: []string{"甲", "", "乙"}},
		{name: "emoji grapheme", text: "AAAAAAA👨‍👩‍👧‍👦B", width: 8, want: []string{"AAAAAAA", "👨‍👩‍👧‍👦B"}},
		{name: "Latin words", text: "hello world", width: 8, want: []string{"hello ", "world"}},
		{name: "boundary space", text: "12345678 x", width: 8, want: []string{"12345678", " x"}},
		{name: "wide whitespace", text: "         ", width: 8, want: []string{"        ", " "}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := wrapText(test.text, test.width)
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("wrapText(%q, %d) = %#v, want %#v", test.text, test.width, got, test.want)
			}
		})
	}
}

func TestRenderMessagePreservesTrailingNewline(t *testing.T) {
	lines := renderMessage(surface.Message{Role: surface.RoleAssistant, Content: "line\n"}, 40, DefaultPalette())
	if len(lines) != 2 {
		t.Fatalf("rendered lines = %#v, want trailing empty line", lines)
	}
}

type testDriver struct {
	sessions    []surface.Session
	active      string
	busy        bool
	meta        surface.Meta
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
func (d *testDriver) Meta() surface.Meta {
	meta := d.meta
	meta.Mode = "test"
	meta.Busy = meta.Busy || d.busy
	return meta
}
func (d *testDriver) Init() tea.Cmd { return nil }
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
			Context: surface.Context{
				FeedTokens: 1200, ModelLimitTokens: 8000, TriggerTokens: 6400,
				TotalMessages: 5, FeedMessages: 3, HasCompactionSummary: true,
			},
		},
	}
	driver.meta = surface.Meta{Busy: true, RunID: "run_active", Queued: 2}
	m := New(driver)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	view := updated.(Model).View()
	for _, want := range []string{"Current", "1.2k / 8.0k tokens", "run · run_active", "queue · 2", "3 / 5 feed messages", "compact at 6.4k", "summary available"} {
		if !strings.Contains(view, want) {
			t.Fatalf("sidebar omitted %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "Other") {
		t.Fatalf("inactive session leaked into sidebar:\n%s", view)
	}
}

func TestSidebarRendersTruthAndScrollsIndependently(t *testing.T) {
	driver := &testDriver{
		sessions: []surface.Session{{ID: "active", Title: "Current"}},
		active:   "active",
		sidebar: surface.Sidebar{
			Session: surface.Session{ID: "active", Title: "Current", UpdatedAt: 1725552000000},
			CWD:     "C:/code/project", Model: "reasoning-model", Provider: "provider-a",
			ReasoningKnown: true, ReasoningSupported: true,
			HasContext: true, Context: surface.Context{FeedTokens: 1200, ModelLimitTokens: 8000},
			HasUsage: true, Usage: surface.SidebarUsage{TotalTokens: 1500, CostKnown: false},
			ModifiedFilesKnown: true,
			MCPKnown:           true, MCP: []surface.MCPServer{{Name: "docs", State: "initialized"}, {Name: "local", State: "configured"}},
			SkillsKnown: true, Skills: []surface.SidebarSkill{{Name: "review"}},
		},
	}
	for i := 0; i < 20; i++ {
		driver.sidebar.ModifiedFiles = append(driver.sidebar.ModifiedFiles, surface.ModifiedFile{
			Path: fmt.Sprintf("pkg/file-%02d.go", i), UpdatedAt: int64(i + 1),
			Diff: surface.SidebarDiff{Additions: 1, Deletions: 1},
		})
	}
	m := New(driver)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m = updated.(Model)
	view := m.View()
	for _, want := range []string{"C:/code/project", "reasoning-model", "provider-a", "reasoning · supported", "cost · unknown", "pkg/file-00.go"} {
		if !strings.Contains(view, want) {
			t.Fatalf("sidebar omitted %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "0.0000") {
		t.Fatalf("unknown cost rendered as a free value:\n%s", view)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlRight})
	m = updated.(Model)
	if !m.sidebarFocused {
		t.Fatal("ctrl+right did not focus a scrollable sidebar")
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnd})
	m = updated.(Model)
	scrolled := m.View()
	if m.sidebarScroll == 0 || !strings.Contains(scrolled, "pkg/file-19.go") {
		t.Fatalf("end did not scroll to the newest modified file: offset=%d\n%s", m.sidebarScroll, m.View())
	}
	for _, want := range []string{"VIVY CODE", "docs · initialized", "local · configured", "Skills · enabled", "review"} {
		if !strings.Contains(scrolled, want) {
			t.Fatalf("scrolled sidebar omitted %q:\n%s", want, scrolled)
		}
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	m = updated.(Model)
	if m.sidebarFocused {
		t.Fatal("left arrow did not exit sidebar focus")
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlRight})
	m = updated.(Model)
	updated, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = updated.(Model)
	if m.sidebarFocused || m.sidebarScroll != 0 {
		t.Fatalf("compact resize retained hidden sidebar state: focused=%v scroll=%d", m.sidebarFocused, m.sidebarScroll)
	}
}

func TestSidebarShortcutDoesNotStealEditorInput(t *testing.T) {
	driver := &testDriver{
		sidebar: surface.Sidebar{
			ModifiedFilesKnown: true,
			ModifiedFiles:      make([]surface.ModifiedFile, 20),
		},
	}
	for i := range driver.sidebar.ModifiedFiles {
		driver.sidebar.ModifiedFiles[i].Path = fmt.Sprintf("pkg/file-%02d.go", i)
	}
	m := New(driver)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m = updated.(Model)

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	m = updated.(Model)
	if m.sidebarFocused || m.input != "l" {
		t.Fatalf("typing l was stolen by sidebar: focused=%v input=%q", m.sidebarFocused, m.input)
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m = updated.(Model)
	if m.sidebarFocused {
		t.Fatal("plain right arrow was stolen by sidebar")
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlRight})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	m = updated.(Model)
	if m.sidebarFocused || m.input != "lx" {
		t.Fatalf("editor input did not resume from sidebar focus: focused=%v input=%q", m.sidebarFocused, m.input)
	}
}

func TestSidebarMouseWheelIsRegionBoundedAndOverlaySafe(t *testing.T) {
	driver := &testDriver{
		sidebar: surface.Sidebar{ModifiedFilesKnown: true, ModifiedFiles: make([]surface.ModifiedFile, 50)},
	}
	for i := range driver.sidebar.ModifiedFiles {
		driver.sidebar.ModifiedFiles[i].Path = fmt.Sprintf("pkg/file-%02d.go", i)
	}
	m := New(driver)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m = updated.(Model)
	l := computeLayout(m.width, m.height)
	sidebarX := l.marginX + l.mainW() + 1

	updated, _ = m.Update(tea.MouseMsg{X: sidebarX, Y: l.marginY + 1, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
	m = updated.(Model)
	if !m.sidebarFocused {
		t.Fatal("sidebar click did not focus scroll owner")
	}
	updated, _ = m.Update(tea.MouseMsg{X: sidebarX, Y: l.marginY + 1, Button: tea.MouseButtonWheelDown})
	m = updated.(Model)
	if m.sidebarScroll != sidebarWheelStep || !m.sidebarFocused {
		t.Fatalf("sidebar wheel = offset %d focused %v, want %d/true", m.sidebarScroll, m.sidebarFocused, sidebarWheelStep)
	}

	before := m.sidebarScroll
	updated, _ = m.Update(tea.MouseMsg{X: l.marginX, Y: l.marginY + 1, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
	m = updated.(Model)
	if m.sidebarFocused {
		t.Fatal("main-area click did not release sidebar focus")
	}
	updated, _ = m.Update(tea.MouseMsg{X: l.marginX, Y: l.marginY + 1, Button: tea.MouseButtonWheelDown})
	m = updated.(Model)
	if m.sidebarScroll != before {
		t.Fatalf("main-area wheel moved sidebar from %d to %d", before, m.sidebarScroll)
	}

	m.sessionsOpen = true
	updated, _ = m.Update(tea.MouseMsg{X: sidebarX, Y: l.marginY + 1, Button: tea.MouseButtonWheelDown})
	m = updated.(Model)
	if m.sidebarScroll != before {
		t.Fatalf("dialog wheel leaked into sidebar: %d -> %d", before, m.sidebarScroll)
	}

	m.sessionsOpen = false
	updated, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = updated.(Model)
	updated, _ = m.Update(tea.MouseMsg{X: 99, Y: 2, Button: tea.MouseButtonWheelDown})
	m = updated.(Model)
	if m.sidebarScroll != 0 || m.sidebarFocused {
		t.Fatalf("compact mouse retained hidden sidebar state: offset=%d focused=%v", m.sidebarScroll, m.sidebarFocused)
	}
}

func TestSidebarRendersKnownEmptyModifiedFiles(t *testing.T) {
	driver := &testDriver{
		sessions: []surface.Session{{ID: "active", Title: "Current"}},
		active:   "active",
		sidebar: surface.Sidebar{
			Session:            surface.Session{ID: "active", Title: "Current"},
			ModifiedFilesKnown: true,
		},
	}
	m := New(driver)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	view := updated.(Model).View()
	if !strings.Contains(view, "Modified Files") || !strings.Contains(view, "None") {
		t.Fatalf("known empty modified-files section missing:\n%s", view)
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

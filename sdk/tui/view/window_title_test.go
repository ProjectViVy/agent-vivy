package view

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"agent-vivy/sdk/tui/surface"
)

func TestWindowTitleForSanitizesAndBoundsSessionTitle(t *testing.T) {
	tests := []struct {
		name         string
		sessionTitle string
		want         string
	}{
		{name: "empty keeps brand", sessionTitle: "", want: "VIVY CODE"},
		{name: "blank keeps brand", sessionTitle: "  \n\t ", want: "VIVY CODE"},
		{name: "control-only keeps brand", sessionTitle: "\x07\x1b", want: "VIVY CODE"},
		{name: "titled session", sessionTitle: "x", want: "VIVY CODE · x"},
		{name: "trimmed", sessionTitle: "  fix the parser  ", want: "VIVY CODE · fix the parser"},
		{name: "control characters collapse", sessionTitle: "a\r\nb\tc", want: "VIVY CODE · a b c"},
		{name: "escapes stripped", sessionTitle: "ok\x1b]2;pwn\ago", want: "VIVY CODE · ok]2;pwngo"},
		{name: "long title capped with ellipsis", sessionTitle: strings.Repeat("字", 80), want: "VIVY CODE · " + strings.Repeat("字", 63) + "…"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := windowTitleFor(test.sessionTitle); got != test.want {
				t.Fatalf("windowTitleFor(%q) = %q, want %q", test.sessionTitle, got, test.want)
			}
		})
	}
}

func TestInitSetsWindowTitle(t *testing.T) {
	if cmd := (Model{}).Init(); cmd == nil {
		t.Fatal("nil-driver Init did not set the window title")
	}
	if cmd := New(&testDriver{}).Init(); cmd == nil {
		t.Fatal("Init did not set the window title")
	}
}

func TestUpdateKeepsTerminalTitleInSyncWithSessionTitle(t *testing.T) {
	driver := &testDriver{
		sessions: []surface.Session{{ID: "s1", Title: "First"}},
		active:   "s1",
		sidebar:  surface.Sidebar{Session: surface.Session{ID: "s1", Title: "First"}},
	}
	m := New(driver)
	if m.windowTitle != windowTitleBrand {
		t.Fatalf("new model title = %q, want %q", m.windowTitle, windowTitleBrand)
	}
	if m.Init() == nil {
		t.Fatal("Init did not set the window title")
	}

	// A titled session updates the terminal title on the first update.
	updated, cmd := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = updated.(Model)
	if cmd == nil || m.windowTitle != "VIVY CODE · First" {
		t.Fatalf("titled session did not set the title: cmd=%v title=%q", cmd != nil, m.windowTitle)
	}

	// An unchanged title must not reissue the terminal escape.
	updated, cmd = m.Update(tea.WindowSizeMsg{Width: 81, Height: 24})
	m = updated.(Model)
	if cmd != nil || m.windowTitle != "VIVY CODE · First" {
		t.Fatalf("unchanged title produced a command: cmd=%v title=%q", cmd != nil, m.windowTitle)
	}

	// A session title change syncs again.
	driver.sidebar.Session.Title = "Second"
	updated, cmd = m.Update(surface.RefreshMsg{})
	m = updated.(Model)
	if cmd == nil || m.windowTitle != "VIVY CODE · Second" {
		t.Fatalf("changed title did not resync: cmd=%v title=%q", cmd != nil, m.windowTitle)
	}

	// Clearing the title falls back to the bare brand.
	driver.sidebar.Session.Title = ""
	updated, cmd = m.Update(surface.RefreshMsg{})
	m = updated.(Model)
	if cmd == nil || m.windowTitle != windowTitleBrand {
		t.Fatalf("empty title did not fall back to the brand: cmd=%v title=%q", cmd != nil, m.windowTitle)
	}
}

func TestUpdateDefersTitleSyncWhileDriverCommandsArePending(t *testing.T) {
	driver := &testDriver{
		sessions: []surface.Session{{ID: "s1", Title: "First"}},
		active:   "s1",
		sidebar:  surface.Sidebar{Session: surface.Session{ID: "s1", Title: "First"}},
	}
	m := New(driver)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	m = updated.(Model)
	if cmd == nil {
		t.Fatal("ctrl+s did not open the sessions dialog")
	}
	// A pending driver command keeps its own message shape so callers that
	// execute the returned command still observe the driver message; the
	// title sync waits for a quiet update.
	msg := cmd()
	if _, batched := msg.(tea.BatchMsg); batched {
		t.Fatalf("driver command was batched with the title sync: %T", msg)
	}
	if _, delivered := msg.(surface.SessionsMsg); !delivered {
		t.Fatalf("driver command message = %T, want surface.SessionsMsg", msg)
	}
	if m.windowTitle != windowTitleBrand {
		t.Fatalf("deferred title applied early: %q", m.windowTitle)
	}

	// The following quiet update applies the deferred title.
	updated, cmd = m.Update(msg)
	m = updated.(Model)
	if cmd == nil || m.windowTitle != "VIVY CODE · First" {
		t.Fatalf("deferred title did not sync: cmd=%v title=%q", cmd != nil, m.windowTitle)
	}
}

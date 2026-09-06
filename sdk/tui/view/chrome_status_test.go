package view

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"agent-vivy/sdk/tui/surface"
)

func TestRenderInputChromeBusySpinnerElapsed(t *testing.T) {
	driver := &testDriver{
		sessions: []surface.Session{{ID: "s1", Title: "overnight notes", PermissionPreset: "smart"}},
		active:   "s1",
		meta: surface.Meta{
			Busy:      true,
			BusySince: time.Now().Add(-90 * time.Second),
		},
	}
	m := New(driver)
	chrome := m.renderInputChrome(80, m.palette)
	if !strings.Contains(chrome, "run 1m30s") {
		t.Fatalf("busy chrome missing spinner + elapsed run label: %q", chrome)
	}
	found := false
	for _, frame := range spinnerFrames {
		if strings.Contains(chrome, frame) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("busy chrome missing braille spinner frame: %q", chrome)
	}
}

func TestRenderInputChromeQueuedCount(t *testing.T) {
	driver := &testDriver{
		sessions: []surface.Session{{ID: "s1", PermissionPreset: "smart"}},
		active:   "s1",
		meta:     surface.Meta{Queued: 2},
	}
	m := New(driver)
	if chrome := m.renderInputChrome(80, m.palette); !strings.Contains(chrome, "queued 2") {
		t.Fatalf("chrome missing queued count: %q", chrome)
	}
	driver.meta.Queued = 0
	if chrome := m.renderInputChrome(80, m.palette); strings.Contains(chrome, "queued") {
		t.Fatalf("chrome shows queued count while the queue is empty: %q", chrome)
	}
}

func TestRenderInputChromeScrollIndicator(t *testing.T) {
	messages := map[string][]surface.Message{"s1": {}}
	for i := 0; i < 60; i++ {
		messages["s1"] = append(messages["s1"], surface.Message{Role: surface.RoleUser, Content: "chat line to overflow the viewport"})
	}
	driver := &testDriver{
		sessions: []surface.Session{{ID: "s1", PermissionPreset: "smart"}},
		active:   "s1",
		messages: messages,
	}
	m := New(driver)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 88, Height: 24})
	m = updated.(Model)
	if !m.chatCanScroll() {
		t.Fatalf("chat should be scrollable with 60 overflow messages")
	}
	m.scrollChat(-10)
	if m.chatFollow {
		t.Fatalf("scrolling should release bottom follow")
	}
	chrome := m.renderInputChrome(m.layout().mainW(), m.palette)
	if !strings.Contains(chrome, "↓ ") || !strings.Contains(chrome, "end 回底") {
		t.Fatalf("scrolled chrome missing scroll indicator: %q", chrome)
	}
}

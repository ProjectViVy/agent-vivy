package view

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"agent-vivy/sdk/tui/surface"
)

func busyChromeModel(t *testing.T) (Model, *testDriver) {
	t.Helper()
	driver := &testDriver{
		sessions: []surface.Session{{ID: "s1", Title: "Current", PermissionPreset: "smart"}},
		active:   "s1",
		busy:     true,
	}
	m := New(driver)
	return m, driver
}

func TestSpinnerTickAdvancesFramesWithWraparound(t *testing.T) {
	m, _ := busyChromeModel(t)
	// The first busy observation starts the chain and the clock.
	next, cmd := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = next.(Model)
	if cmd == nil {
		t.Fatal("busy false→true transition did not schedule the first spinner tick")
	}
	if m.busyStartedAt.IsZero() {
		t.Fatal("busy transition did not start the elapsed clock")
	}
	for i := 0; i < len(spinnerFrames)*2+3; i++ {
		next, _ = m.Update(spinnerTickMsg(time.Now()))
		m = next.(Model)
		if want := (i + 1) % len(spinnerFrames); m.spinnerIndex != want {
			t.Fatalf("tick %d: spinnerIndex = %d, want %d", i, m.spinnerIndex, want)
		}
	}
}

func TestSpinnerResetsWhenRunEnds(t *testing.T) {
	m, driver := busyChromeModel(t)
	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = next.(Model)
	next, _ = m.Update(spinnerTickMsg(time.Now()))
	m = next.(Model)
	if m.spinnerIndex == 0 {
		t.Fatal("spinner did not advance while busy")
	}
	driver.busy = false
	next, cmd := m.Update(spinnerTickMsg(time.Now()))
	m = next.(Model)
	if cmd != nil {
		t.Fatal("spinner tick chain rescheduled after the run ended")
	}
	if m.spinnerIndex != 0 {
		t.Fatalf("spinnerIndex = %d, want 0 after busy→idle", m.spinnerIndex)
	}
	if !m.busyStartedAt.IsZero() {
		t.Fatalf("busyStartedAt not reset after busy→idle: %v", m.busyStartedAt)
	}
}

func TestIdleModelDoesNotScheduleSpinnerTicks(t *testing.T) {
	driver := &testDriver{
		sessions: []surface.Session{{ID: "s1", Title: "Current"}},
		active:   "s1",
	}
	m := New(driver)
	_, cmd := m.Update(spinnerTickMsg(time.Now()))
	if cmd != nil {
		t.Fatal("idle model rescheduled a spinner tick")
	}
	if m.spinnerIndex != 0 {
		t.Fatalf("idle tick advanced spinnerIndex to %d", m.spinnerIndex)
	}
}

func TestSpinnerLabelElapsedFormats(t *testing.T) {
	base := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		delta time.Duration
		want  string
	}{
		{delta: 0, want: "<1s"},
		{delta: 400 * time.Millisecond, want: "<1s"},
		{delta: time.Second, want: "1s"},
		{delta: 12 * time.Second, want: "12s"},
		{delta: 59 * time.Second, want: "59s"},
		{delta: 60 * time.Second, want: "1m00s"},
		{delta: 65 * time.Second, want: "1m05s"},
		{delta: 10*time.Minute + 7*time.Second, want: "10m07s"},
	}
	for _, tc := range cases {
		if got := spinnerElapsed(base, base.Add(tc.delta)); got != tc.want {
			t.Fatalf("spinnerElapsed(+%v) = %q, want %q", tc.delta, got, tc.want)
		}
	}
	if got := spinnerElapsed(base, base.Add(-time.Second)); got != "<1s" {
		t.Fatalf("spinnerElapsed(negative) = %q, want \"<1s\"", got)
	}
	if got := spinnerLabel(spinnerFrames, 3, time.Time{}, base); got != "" {
		t.Fatalf("spinnerLabel with zero startedAt = %q, want empty", got)
	}
	label := spinnerLabel(spinnerFrames, 3, base, base.Add(65*time.Second))
	if want := "⠸ 1m05s"; label != want {
		t.Fatalf("spinnerLabel = %q, want %q", label, want)
	}
	label = spinnerLabel(spinnerFrames, len(spinnerFrames)+2, base, base.Add(12*time.Second))
	if want := "⠹ 12s"; label != want {
		t.Fatalf("spinnerLabel out-of-range index = %q, want %q", label, want)
	}
}

func TestChromeBusyShowsSpinnerAndErrorWins(t *testing.T) {
	m, driver := busyChromeModel(t)
	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = next.(Model)
	m.busyStartedAt = time.Now().Add(-65 * time.Second)
	chrome := ansi.Strip(m.renderInputChrome(80, DefaultPalette()))
	if !strings.Contains(chrome, "1m05s") {
		t.Fatalf("busy chrome missing spinner elapsed label:\n%s", chrome)
	}
	if strings.Contains(chrome, "run…") {
		t.Fatalf("busy chrome still shows the static run marker:\n%s", chrome)
	}
	// A transport error outranks the spinner.
	driver.meta = surface.Meta{Busy: true, Error: "boom"}
	chrome = ansi.Strip(m.renderInputChrome(80, DefaultPalette()))
	if !strings.Contains(chrome, "err · boom") {
		t.Fatalf("error chrome missing err text:\n%s", chrome)
	}
	if strings.Contains(chrome, "1m05s") {
		t.Fatalf("spinner label survived a higher-priority error:\n%s", chrome)
	}
}

func TestChromeIdleKeepsHintsAndNoSpinner(t *testing.T) {
	driver := &testDriver{
		sessions: []surface.Session{{ID: "s1", Title: "Current"}},
		active:   "s1",
	}
	m := New(driver)
	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = next.(Model)
	chrome := ansi.Strip(m.renderInputChrome(80, DefaultPalette()))
	if !strings.Contains(chrome, "shift+tab") || !strings.Contains(chrome, "切换模式") {
		t.Fatalf("idle chrome lost hints:\n%s", chrome)
	}
	for _, frame := range spinnerFrames {
		if strings.Contains(chrome, frame) {
			t.Fatalf("idle chrome shows a spinner frame %q:\n%s", frame, chrome)
		}
	}
	if strings.Contains(chrome, "run…") {
		t.Fatalf("idle chrome shows the run marker:\n%s", chrome)
	}
}

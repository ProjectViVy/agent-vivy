package view

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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

func TestJoinChromeRowPadsToFullWidth(t *testing.T) {
	row := joinChromeRow("abc", "xyz", 10)
	if got := lipgloss.Width(row); got != 10 {
		t.Fatalf("joined row width = %d, want 10: %q", got, row)
	}
	if !strings.HasPrefix(row, "abc") || !strings.HasSuffix(row, "xyz") {
		t.Fatalf("joined row = %q, want left-anchored right-aligned", row)
	}
	cjk := joinChromeRow("中文", "ab", 10)
	if got := lipgloss.Width(cjk); got != 10 {
		t.Fatalf("CJK joined row width = %d, want 10: %q", got, cjk)
	}
	if got := joinChromeRow("hello world", "", 6); lipgloss.Width(got) != 6 || !strings.HasPrefix(got, "hello") {
		t.Fatalf("empty right segment = %q, want truncated left", got)
	}
	// An oversized left segment is truncated and the right segment is dropped.
	got := joinChromeRow("0123456789abcdefghij", "xyz", 8)
	if lipgloss.Width(got) != 8 || strings.Contains(got, "xyz") {
		t.Fatalf("overflow row = %q, want truncated left without right", got)
	}
	if got := joinChromeRow("abc", "xyz", 0); got != "" {
		t.Fatalf("zero width row = %q, want empty", got)
	}
}

const chromeTestTitle = "an extremely long session title that must be trimmed"

func chromeMetaModel(t *testing.T) (Model, *testDriver) {
	t.Helper()
	driver := &testDriver{
		sessions: []surface.Session{{ID: "s1", Title: chromeTestTitle}},
		active:   "s1",
		meta:     surface.Meta{Host: "127.0.0.1:8787", Queued: 2},
	}
	return New(driver), driver
}

// chromeMetaFixtureWidths measures the styled candidates so the degradation
// stages below can pick row widths without assuming glyph widths.
func chromeMetaFixtureWidths(p Palette) (chip, host, fixed int) {
	chip = lipgloss.Width(p.PromptWarn.Render("⏸ 2 queued"))
	host = lipgloss.Width("127.0.0.1:8787")
	return chip, host, chip + chromeGap + host
}

func TestChromeMetaShowsQueuedHostAndTitleInOrder(t *testing.T) {
	m, _ := chromeMetaModel(t)
	p := DefaultPalette()
	meta := m.chromeMeta("", 100, p)
	plain := ansi.Strip(meta)
	if !strings.Contains(plain, "⏸ 2 queued") {
		t.Fatalf("chrome meta missing queued chip:\n%s", plain)
	}
	if !strings.Contains(plain, "127.0.0.1:8787") {
		t.Fatalf("chrome meta missing host:\n%s", plain)
	}
	if !strings.Contains(plain, chromeTestTitle) {
		t.Fatalf("chrome meta missing full title:\n%s", plain)
	}
	if queued, host, title := strings.Index(plain, "⏸"), strings.Index(plain, "127.0.0.1"), strings.Index(plain, "an extremely"); queued > host || host > title {
		t.Fatalf("chrome meta order wrong (queued=%d host=%d title=%d):\n%s", queued, host, title, plain)
	}
	// With queued=0 the chip disappears but host and title remain.
	m2, _ := chromeMetaModel(t)
	m2.driver.(*testDriver).meta.Queued = 0
	plain2 := ansi.Strip(m2.chromeMeta("", 100, p))
	if strings.Contains(plain2, "queued") {
		t.Fatalf("queued chip rendered with an empty queue:\n%s", plain2)
	}
	if !strings.Contains(plain2, "127.0.0.1:8787") {
		t.Fatalf("host lost with queued=0:\n%s", plain2)
	}
}

func TestChromeMetaTrimsTitleBeforeDropping(t *testing.T) {
	m, _ := chromeMetaModel(t)
	p := DefaultPalette()
	_, _, fixedW := chromeMetaFixtureWidths(p)
	// Room exactly 10 cells for the title: it survives tail-truncated.
	width := fixedW + chromeGap + 10 + chromeGap
	meta := ansi.Strip(m.chromeMeta("", width, p))
	if !strings.HasSuffix(meta, "an extrem…") {
		t.Fatalf("over-long title was not tail-trimmed:\n%s", meta)
	}
	if strings.Contains(meta, chromeTestTitle) {
		t.Fatalf("full title survived a narrow row:\n%s", meta)
	}
}

func TestChromeMetaDropsTitleThenHostThenQueued(t *testing.T) {
	p := DefaultPalette()
	chipW, _, fixedW := chromeMetaFixtureWidths(p)
	// Stage 1: the title has no room; queued + host stay.
	m, _ := chromeMetaModel(t)
	meta := ansi.Strip(m.chromeMeta("", fixedW+chromeGap+minChromeTitleWidth-1+chromeGap, p))
	if strings.Contains(meta, "an extrem") || strings.Contains(meta, "…") {
		t.Fatalf("title survived a row that cannot seat it:\n%s", meta)
	}
	if !strings.Contains(meta, "127.0.0.1:8787") || !strings.Contains(meta, "⏸ 2 queued") {
		t.Fatalf("host or queued dropped too early:\n%s", meta)
	}
	// Stage 2: host dropped, queued kept last.
	m, _ = chromeMetaModel(t)
	meta = ansi.Strip(m.chromeMeta("", fixedW+1, p))
	if strings.Contains(meta, "127.0.0.1:8787") {
		t.Fatalf("host survived a row that cannot seat it:\n%s", meta)
	}
	if !strings.Contains(meta, "⏸ 2 queued") {
		t.Fatalf("queued chip dropped before the host:\n%s", meta)
	}
	// Stage 3: everything dropped.
	m, _ = chromeMetaModel(t)
	if got := m.chromeMeta("", chipW+1, p); got != "" {
		t.Fatalf("chrome meta = %q, want empty on a tiny row", got)
	}
}

func TestChromeMetaEmptyWithoutCandidates(t *testing.T) {
	driver := &testDriver{
		sessions: []surface.Session{{ID: "s1"}},
		active:   "s1",
	}
	m := New(driver)
	if got := m.chromeMeta("", 40, DefaultPalette()); got != "" {
		t.Fatalf("chrome meta with no candidates = %q, want empty", got)
	}
}

func TestChromeRowKeepsLeftHintsAndRightMetaAligned(t *testing.T) {
	m, _ := chromeMetaModel(t)
	next, _ := m.Update(tea.WindowSizeMsg{Width: 160, Height: 30})
	m = next.(Model)
	p := DefaultPalette()
	chrome := m.renderInputChrome(140, p)
	plain := ansi.Strip(chrome)
	if got := lipgloss.Width(plain); got != 140 {
		t.Fatalf("chrome row width = %d, want 140:\n%s", got, plain)
	}
	if !strings.HasPrefix(plain, "shift+tab") {
		t.Fatalf("chrome row no longer starts with the hint keys:\n%s", plain)
	}
	if !strings.Contains(plain, "⏸ 2 queued") || !strings.Contains(plain, chromeTestTitle) {
		t.Fatalf("chrome row lost the right meta segment:\n%s", plain)
	}
}

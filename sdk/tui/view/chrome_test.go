package view

import (
	"fmt"
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

// TestSpinFrameAdvancesOnEveryMessage pins the heartbeat-driven animation: the
// frame index moves on each Update, and busyStatus wraps it over the frame set.
func TestSpinFrameAdvancesOnEveryMessage(t *testing.T) {
	m, _ := busyChromeModel(t)
	before := m.spinFrame
	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = next.(Model)
	if m.spinFrame != before+1 {
		t.Fatalf("spinFrame = %d, want %d after one message", m.spinFrame, before+1)
	}
	if got := spinnerFrames[m.spinFrame%len(spinnerFrames)]; got == "" {
		t.Fatal("spinner frame set must not contain empty frames")
	}
}

func TestChromeBusyShowsSpinnerAndErrorWins(t *testing.T) {
	m, driver := busyChromeModel(t)
	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = next.(Model)
	driver.meta = surface.Meta{Busy: true, BusySince: time.Now().Add(-65 * time.Second)}
	chrome := ansi.Strip(m.renderInputChrome(80, DefaultPalette()))
	if !strings.Contains(chrome, "run 1m05s") {
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
	if strings.Contains(chrome, "run 1m05s") {
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
	if !strings.Contains(chrome, "shift+tab") || !strings.Contains(chrome, "Switch mode") {
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

// scrollHintModel builds a chat long enough to scroll several viewports.
func scrollHintModel(t *testing.T) (Model, *testDriver) {
	t.Helper()
	rows := make([]surface.Message, 0, 60)
	for i := 0; i < 60; i++ {
		rows = append(rows, surface.Message{ID: fmt.Sprintf("m%d", i), Role: surface.RoleUser, Content: fmt.Sprintf("history line %d", i)})
	}
	driver := &testDriver{
		sessions: []surface.Session{{ID: "s1", Title: "Current"}},
		active:   "s1",
		messages: map[string][]surface.Message{"s1": rows},
	}
	return New(driver), driver
}

func TestChromeScrollHintVariants(t *testing.T) {
	p := DefaultPalette()
	if got := (Model{}).chromeScrollHint(p, chatScrollInfo{follow: true, offset: 0, maxScroll: 50, viewport: 10}); got != "" {
		t.Fatalf("follow-mode hint = %q, want empty", ansi.Strip(got))
	}
	if got := (Model{}).chromeScrollHint(p, chatScrollInfo{follow: false, offset: 0, maxScroll: 0, viewport: 10}); got != "" {
		t.Fatalf("maxScroll=0 hint = %q, want empty", ansi.Strip(got))
	}
	if got := (Model{}).chromeScrollHint(p, chatScrollInfo{follow: false, offset: 9, maxScroll: 12, viewport: 10}); got != "" {
		t.Fatalf("near-bottom hint = %q, want empty", ansi.Strip(got))
	}
	got := ansi.Strip((Model{}).chromeScrollHint(p, chatScrollInfo{follow: false, offset: 2, maxScroll: 12, viewport: 10}))
	if !strings.Contains(got, "↓ end") || !strings.Contains(got, "Back to bottom") {
		t.Fatalf("near-bottom hint = %q, want the end jump", got)
	}
	got = ansi.Strip((Model{}).chromeScrollHint(p, chatScrollInfo{follow: false, offset: 0, maxScroll: 79, viewport: 10}))
	if !strings.Contains(got, "↑ History") || !strings.Contains(got, "79 lines below") {
		t.Fatalf("deep-park hint = %q, want remaining lines", got)
	}
}

func TestGKeyJumpsToBottomAndDraftKeepsG(t *testing.T) {
	m, driver := scrollHintModel(t)
	next, _ := m.Update(tea.WindowSizeMsg{Width: 88, Height: 24})
	m = next.(Model)
	if !m.chatFollow {
		t.Fatal("fresh model should follow the chat bottom")
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	m = next.(Model)
	if m.chatFollow {
		t.Fatal("page-up did not clear chatFollow")
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	m = next.(Model)
	if !m.chatFollow || m.chatScroll != m.chatMaxScroll() {
		t.Fatalf("G did not jump to the bottom: follow=%v scroll=%d max=%d", m.chatFollow, m.chatScroll, m.chatMaxScroll())
	}
	if m.input != "" {
		t.Fatalf("G leaked into the draft: %q", m.input)
	}
	// With a draft, G is plain input.
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	m = next.(Model)
	m.input = "draft"
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	m = next.(Model)
	if m.input != "draftG" || m.chatFollow {
		t.Fatalf("G stole an in-progress draft: input=%q follow=%v", m.input, m.chatFollow)
	}
	// While busy, G is plain input too (warm up the busy observation first).
	driver.busy = true
	next, _ = m.Update(tea.WindowSizeMsg{Width: 88, Height: 24})
	m = next.(Model)
	m.input = ""
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	m = next.(Model)
	if m.input != "G" || m.chatFollow {
		t.Fatalf("busy G jumped to the bottom: input=%q follow=%v", m.input, m.chatFollow)
	}
}

func TestChromeRowShowsScrollHintWhileParked(t *testing.T) {
	m, _ := scrollHintModel(t)
	next, _ := m.Update(tea.WindowSizeMsg{Width: 88, Height: 24})
	m = next.(Model)
	if strings.Contains(ansi.Strip(m.View()), "Back to bottom") {
		t.Fatal("following viewport shows the scroll hint")
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	m = next.(Model)
	if view := ansi.Strip(m.View()); !strings.Contains(view, "↓ end") || !strings.Contains(view, "Back to bottom") {
		t.Fatalf("parked viewport missing the jump-to-bottom hint:\n%s", view)
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	m = next.(Model)
	if view := ansi.Strip(m.View()); !strings.Contains(view, "↑ History") || !strings.Contains(view, "lines below") {
		t.Fatalf("deep-parked viewport missing the remaining-lines hint:\n%s", view)
	}
	// Returning to the bottom with `end` hides the hint again.
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnd})
	m = next.(Model)
	view := ansi.Strip(m.View())
	if strings.Contains(view, "Back to bottom") || strings.Contains(view, "↑ History") {
		t.Fatalf("bottom viewport still shows the scroll hint:\n%s", view)
	}
}

func TestChromeScrollHintHiddenNearBottom(t *testing.T) {
	m, _ := scrollHintModel(t)
	next, _ := m.Update(tea.WindowSizeMsg{Width: 88, Height: 24})
	m = next.(Model)
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	m = next.(Model)
	m.chatFollow = false
	m.chatScroll = m.chatMaxScroll() - 2
	next, _ = m.Update(surface.RefreshMsg{})
	m = next.(Model)
	if strings.Contains(ansi.Strip(m.View()), "Back to bottom") {
		t.Fatal("scroll hint shown within two lines of the bottom")
	}
	if m.chatFollow {
		t.Fatal("parking above the bottom re-enabled follow")
	}
}

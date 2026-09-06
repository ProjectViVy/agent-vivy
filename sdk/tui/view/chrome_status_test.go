package view

import (
	"strings"
	"testing"
	"time"

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
	if chrome := m.renderInputChrome(80, m.palette); !strings.Contains(chrome, "⏸ 2 queued") {
		t.Fatalf("chrome missing queued count: %q", chrome)
	}
	driver.meta.Queued = 0
	if chrome := m.renderInputChrome(80, m.palette); strings.Contains(chrome, "queued") {
		t.Fatalf("chrome shows queued count while the queue is empty: %q", chrome)
	}
}

// TestRenderInputChromeScrollIndicator was superseded during the
// feat/tui-detail-polish merge: the scroll-position fragment moved out of the
// right-aligned meta into the dedicated chromeScrollHint on the left segment,
// which carries exact remaining-line counts. The live-frame hint is covered
// end to end by TestChromeRowShowsScrollHintWhileParked in chrome_test.go.

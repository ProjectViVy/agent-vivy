# 2026-09-07 — VIVY CODE status line: spinner / elapsed time / queued right segment and scroll indicator

## What changed

B status-line trio (bubbletea fullscreen shell, `sdk/tui`), delivered in commit `e2ad7f2`:

- **F1 braille spinner + elapsed timer**: the static `run…` on the left side
  of the busy status line becomes `⠙ run 1m15s`. `surface.Meta` adds
  `BusySince` (zero value = idle); all busy transitions on the `live` side
  converge on `setBusyLocked` (7 call sites), and `Meta()` exposes the
  timestamp. The view's `Model.spinFrame` increments with each message in
  `Update` (the driver's existing 40ms `liveTickMsg` heartbeat keeps
  redrawing; it continues on its own whether busy or not, so no new timer is
  needed). Ten braille frames are hand-written, with no `bubbles` dependency.
- **F12 chrome row left/right segments + queue count**: `renderInputChrome`
  changes from a left-aligned single row to left/right segments—the left is
  `err · …` / spinner+elapsed time / shortcut hints (priority unchanged), and
  the right is `queued N` (shown only when `Queued > 0`). The right side is
  padded and aligned to the full row width; `lipgloss.Width()` measures width,
  and the single-line height remains unchanged (`chromeHeight=1`, no layout
  jitter).
- **F6 scroll indicator + end to bottom**: when scrolled up
  (`!chatFollow` and scrollable), the right segment shows
  `↓ <pct>% · end to bottom`; `pct` is `chatScroll/chatMaxScroll` clamped to 0–100.
  The End-to-bottom behavior already existed (`model.go` KeyEnd →
  `chatFollow=true`); this change completes the visible indicator and its
  discoverability without changing keybindings.

## Files

- `sdk/tui/surface/surface.go` — `Meta.BusySince` (additive domain field)
- `sdk/tui/live/controller.go` — `busySince` + `setBusyLocked`, consolidating 7 busy-transition call sites
- `sdk/tui/view/model.go` — `spinFrame` field + one increment in `Update`
- `sdk/tui/view/render.go` — `spinnerFrames`, `busyStatus`, `chromeRightStatus`, and right-aligned `renderInputChrome` segments
- `sdk/tui/view/chrome_status_test.go` — 3 deterministic unit tests (new)
- `sdk/tui/view/zpreview_test.go` — manual preview tool extended with busy+queued / two scrolled frames (still gated by `TUI_PREVIEW=1`)

## Explicitly not done

- The compact header (`renderCompactHeader`) is untouched, and its left/middle/
  right segmentation remains as before; the sidebar's `queue · N` row is also
  unchanged.
- No `bubbles`/new dependency, scroll-key changes, or second timer was added.
- The existing viewport debt recorded at TODO line 64 (paused offset uses a
  render-line-number anchor, and clamp fully re-renders history) is outside
  this batch.

## Lane note

This batch and the parallel window-title / sidebar-color lane both wrote
`model.go`/`render.go` in the root tree. That lane independently landed three
commits, `2768390`, `83d242e`, and `fc63529`; this commit contains only the
status-line deliverable paths, and the two lanes' contents are fully orthogonal
in the committed state.

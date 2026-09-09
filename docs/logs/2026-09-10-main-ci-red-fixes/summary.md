# 2026-09-10 — main `just ci` red fixes

## What changed

After syncing to `origin/main` (`bcc7468`), `just ci` failed with three
independent findings, all introduced by the 2026-09-09 i18n lane whose
`TUI-CMD-I18N` board entry already recorded that Go/just were unavailable
when it shipped. This iteration repairs those findings on the Go/TUI side
that could not be verified before the merge:

1. **fmt-check (5 files not gofmt-clean)** — ran `gofmt -w` on
   `internal/app/settings/settings_test.go`, `internal/rpc/control.go`,
   `internal/rpc/control_test.go`, `sdk/tui/live/controller.go`,
   `sdk/tui/view/render.go`. Formatting only; no semantic edits.
2. **`internal/codeface.TestCodeLaunchSettingsLocaleUsesSharedPath`** — the
   new test drives `app.RunFaceWithAppOptions`, whose checkpoint bridge
   fail-closes on an unknown eino engine version. `go test` binaries lack the
   embedded module metadata, so the test now pins
   `runtime.SetEngineVersionOverride(pinnedEinoVersion)` (go.mod pin
   `v0.9.13`) with a `t.Cleanup` restore — the same pattern every
   `internal/app` suite already uses.
3. **`sdk/tui/view.TestLocaleDialogAndStateNarrowAndWide`** — at width 60 the
   sessions dialog footer (`↑/↓ move · enter/tab select · ^r rename · ^x
   delete`, 52 cells) exceeded the 48-cell inner text budget and lipgloss
   hard-wrapped it mid-word ("del"/"ete"), so the contiguous-`delete`
   assertion failed. Added `wrapWords` in `render.go` (greedy fold on spaces)
   and applied it to the sessions footer with the dialog's real text budget
   (`w - Dialog.GetHorizontalPadding()`); the dialog width expression is now
   computed once and reused.

## What was explicitly not done

- The same latent hard-wrap hazard exists for the rename/delete dialog
  footers at terminal widths below the tested minimum; they are short today
  and no test covers them, so they were left as-is rather than widening scope.
- The i18n lane's other open conformance limitation (cross-face catalog-shape
  parity) tracked under `TUI-CMD-I18N` is untouched.
- No changes to the checkpoint fail-close contract itself; the fix is
  test-side pinning, consistent with the existing design.

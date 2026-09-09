# Verification record

Date: 2026-09-07

## Commands and results

- `go test ./sdk/tui/view` — ok (including 4 new tests)
- `go test ./sdk/tui/...` — all ok (command / face / live / stream / view)
- `gofmt -l sdk/tui` — no output (clean formatting)
- `go vet ./sdk/tui/...` — no warnings
- `just ci` — ran in the background, exit code 0 (see the task notification; in Auto Mode the task output file is not reread)

## New tests (`sdk/tui/view/viewport_test.go`)

- `TestPausedViewportAnchorsToMessageWhileHistoryAboveGrows` — after a tool
  result expands by 2 lines above the anchor, the numeric offset moves with the
  content by 2 lines; a second Refresh with the same content does not move it
  again (fingerprint latch).
- `TestChatStampFingerprintsEveryRenderedField` — any change to content/ID/role/
  streaming/reasoning/all Tool fields/attachments/file context/message count
  changes the fingerprint.
- `TestChatAssemblyRebuildsOnContentChanges` — unchanged history line count is
  stable; streaming growth and mid-turn tool-result filling both trigger a
  rebuild; toggling reasoning collapse reduces the line count.
- `TestLoadingHistoryNeverPosesAsEmptyConversation` — while `Meta.Loading` is
  set, renders “Loading session history…” and not the empty-conversation hero; after
  Loading is cleared, the hero returns.

## Process notes

- The early fixture used `strings.Repeat("long answer\n", 30)`; markdown soft
  wrapping flow-joined the repeated content into ~4 lines, so history never
  overflowed the viewport. It was changed to blank lines between paragraphs
  (`"long answer\n\n"`) so each paragraph occupies one line and the anchor test
  has a middle landing point.
- Paragraphs render as “content line + blank-line separator”; the assertion was
  relaxed from “exactly +1 line” to “growing and visible” to avoid coupling to
  glamour paragraph spacing.
- `chatSegments` returns a shared assembly pointer (rebuilt in place), so tests
  must not compare fields from two returned values across changes—copy
  `lineCount` to an int first, then assert.

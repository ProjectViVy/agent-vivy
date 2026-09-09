# Human acceptance guide

## How to check (real-device smoke test, pending manual execution)

```
just run                 # Terminal 1: control plane 127.0.0.1:8787
vivy tui --live          # Terminal 2: connect, then work through the checklist below
```

## Checklist (the 7 features delivered by this lane)

| # | Feature | Human-observable result |
|---|---|---|
| 1 | F2 Multi-line input | In the composer, press ctrl+j to insert line breaks and enter multiple lines → all lines remain visible; after more than 6 lines, the top lines collapse into `…`; each added line increases the composer's height (capped at 6 lines), and the bottom bar is not pushed off |
| 2 | F3 Placeholder | Clear the input → after `::: `, a dim `Ask something…  / command · @file · !shell` appears; typing any character makes it disappear; it does not appear while an approval dialog is pending |
| 3 | F4 Large paste | Paste text of >2000 characters (or >40 lines) → a yellow `⚠ Large paste · N lines / M characters · …` chip appears in the attachment-row position; it disappears automatically when deletion brings the input below the thresholds |
| 4 | F11 Busy dimming | While a message is running, the composer's border dims gray; when the run ends, the mode color returns |
| 5 | F1 Spinner | During busy, the lower-left shows a braille spinner + elapsed time (e.g. `⠸ 12s`, 120ms/frame); error text takes priority; it resets on completion |
| 6 | F12 Status line | When a turn is queued, the right side shows `⏸ N queued`; when the window is wide enough, the right side also shows the host and session title; as the window narrows, the title is dropped first, then the host, and finally queued |
| 7 | F6 Scroll hint | PgUp into history → the lower-left shows `↓ end to bottom` (when far from the bottom: `↑ history · N more lines below`); pressing `end` or `G` returns to the bottom and removes the hint; pressing `G` with empty input works, while with a draft `G` behaves normally as draft entry |

## Feature-to-commit mapping for all 12 features (full-proposal terminology)

- This lane (this branch): F2 `73aa834`, F3 `7f60072`, F4 `b93af3e`, F11 `02781d8`, F1 `8dbf8ff`, F12 `15c7950`, F6 `a345a51`
- Main lane (the other lane, already on main): F1/F12 `e2ad7f2`, F5/F9/F13 `9052ca5` (merge `b108f0c`), F10 `2768390`, F7 `83d242e` (fix `bb5e294`)
- F8: not done; recorded in `docs/TODO.md` §0.1

## Known observations (non-blocking)

- The compact (narrow-screen) mode's chrome right segment duplicates header information (`TUI-DETAIL-CHROME-DUP`).
- F1/F12 have parallel implementations on main; before merging, use the main version (see `summary.md`).

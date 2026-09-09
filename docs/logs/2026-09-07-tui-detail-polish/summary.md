# TUI detail polish (batches A + B) — lane delivery summary

- Branch: `feat/tui-detail-polish` (worktree `agent-vivy-tui-polish`, based on main@c64b63a + proposal `92c33ae`)
- Proposal: `docs/plans/2026-09-06-tui-detail-polish.md` (12 features, 4 batches)
- Date: 2026-09-07

## This lane's delivery (8 commits, one per feature)

| Feature | Topic | Commit |
|---|---|---|
| F2 | composer multi-line input fully visible + adaptive height (`editorInputLines` pure function, `maxEditorLines=6` tail window + `…` top-truncation marker, `editorReserve` extended) | `73aa834` |
| F3 | dim placeholder for empty input (`composerPlaceholder`, hidden when gate/draft/sidebar focused) | `7f60072` |
| F4 | large-paste warning chip (`pasteGuardChip`, 2000-character / 40-line thresholds, grows and shrinks in real time with the draft) | `b93af3e` |
| F11 | dim composer border in busy state (`p.Dim`, busy takes precedence over work-mode color) | `02781d8` |
| F1 | braille spinner + locally observed elapsed time (`spinnerTickMsg`/`tea.Tick(120ms)` requeue, `spinnerLabel` with injected now, `meta.Error` priority) | `8dbf8ff` |
| F12 | chrome row right-side environment metadata (`joinChromeRow` full-width assembly; queued/host/title fallback order title→host→queued) | `15c7950` |
| F6 | scroll-hover hint + `G` return to bottom (`chatScrollInfo` passed during rendering, hidden within ≤3 lines of bottom) | `a345a51` |
| docs | proposal hash backfill + delivery status + this log + TODO entry | (this commit) |

## External changes during execution (important)

Another lane delivered the remaining parts of the same proposal in parallel on
main: F1/F12 (`e2ad7f2`), F5/F9/F13 (`9052ca5`, merge `b108f0c`), F10
(`2768390`), and F7 (`83d242e` + audit fix `bb5e294`). Therefore:

- **Batches C/D were not dispatched again in this lane** (to avoid duplicate parallel implementations).
- This lane's F1/F12 are **parallel implementations** to main's and remain on this branch (the proposal permits non-merging); if merged later, the recommended approach is to take the main version and port only this branch's unique F6 and batch A (main has no corresponding implementation).
- The root worktree was not touched by this lane at any point during execution.

## Explicitly not done

- F8 (per-message token/cost): requires a new surface protocol, outside the "pure display details" boundary; recorded in `docs/TODO.md` §0.1 (`TUI-DETAIL-F8`).
- F12's compact-mode chrome right segment duplicates the header title/host information (a visual tradeoff), recorded as `TUI-DETAIL-CHROME-DUP`.
- No surface-protocol changes, new dependencies, mouse support, or bubbles were introduced.

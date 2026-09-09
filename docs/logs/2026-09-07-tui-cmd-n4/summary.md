# TUI-CMD-N4 — Dynamic-command asynchronous-state follow-up closeout

## What changed

Closes the three deferred async-state gaps in the dynamic command path that the
2026-09-05 read-only review found (board row `TUI-CMD-N4`):

1. **Stale boot cannot overwrite a newer catalog** (`sdk/tui/live/controller.go`)
   - New `dynamicCommandEpoch` on `Live`, bumped by every
     `RefreshDynamicCommands` call.
   - `bootCmd` captures the epoch under `l.mu` before fetching
     `commands/list` and carries it in `liveBootMsg.CommandEpoch`.
   - `applyBoot` applies `msg.Commands` (and `msg.CommandErr`) only when
     `msg.CommandEpoch == l.dynamicCommandEpoch`. A late boot snapshot from an
     older epoch is dropped whole, so a refresh that started after the boot
     fetch still owns the catalog.
2. **Send refusal restores the operator's draft** (`sdk/tui/view/model.go`)
   - In `surface.DynamicCommandExpandedMsg` handling, when the expansion
     succeeded but `Driver.Send` returned nil (e.g. refused across a load
     transition), the original slash draft is put back into the editor and a
     "could not start a turn" diagnostic is shown, instead of silently
     dropping the input.
3. **Pending expansion locks the editor and secondary surfaces**
   (`sdk/tui/view/model.go`)
   - While `dynamicCommandPending` is set, `handleKey` swallows every key
     except Esc (explicit cancel, restores the saved draft) and Ctrl+C
     (quit). Typing cannot be clobbered when the expansion restores its
     draft, and secondary surfaces (sessions picker, shortcuts, model picker,
     palette re-entry, confirmations) cannot be opened mid-expansion.
   - Mouse input cannot reach dispatch or submit while pending
     (`handleMouse` only routes focus/wheel and returns early), so the
     keyboard lock plus the retained `submitInput` guard (now
     defense-in-depth) cover the state.

## Tests

- `sdk/tui/live/controller_test.go`
  `TestDynamicCommandBootSnapshotCannotOverwriteNewerRefresh` — a boot
  snapshot fetched before a refresh but applied after it cannot resurrect its
  older catalog; the same rule drops a stale boot `CommandErr` (catalog and
  `Meta().Error` stay owned by the completed refresh).
- `sdk/tui/view/command_test.go`
  `TestDynamicCommandSendRefusalRestoresDraft` — send-refused expansion keeps
  the slash draft, clears pending, and surfaces the diagnostic.
  `TestDynamicCommandPendingLocksEditorAndSurfaces` — Ctrl+C still quits,
  typing/Enter/Ctrl+T are swallowed, Esc cancels and restores the draft.
- `TestDynamicCommandSerializesAndRestoresDraftAcrossCancellation` updated:
  the second submission is now dropped by the pending lock before
  `submitInput` (no diagnostic render), asserting swallowed keys instead of
  the unreachable guard text.

## Explicitly not done

- TUI-CMD-N5 scope (plain REPL catalog refresh, RPC cancellation on Esc,
  lastErr cleanup on successful refresh, overlay priority, escaped
  `<loaded_skill>` envelope) remains open on the board.
- No `git push` was performed.

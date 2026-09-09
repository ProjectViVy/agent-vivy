# TUI-CMD-N5 — Dynamic-command P2 parity and resource cleanup

## What changed

The P2 closeout for board row `TUI-CMD-N5` (a carryover from the 2026-09-05
review) was implemented item by item under the current single fullscreen face
architecture (`internal/tui/repl.go` has been absorbed by refactoring; the
plain REPL directory gap that existed only during startup loading disappeared
with it; the palette refreshes the catalog on every open,
`openCommandPalette` → `RefreshDynamicCommands`):

- **True cancellation of the expansion RPC (surface + live + view)**
  - `sdk/tui/surface/surface.go`: added the `DynamicCommandCanceller` capability
    interface (`CancelDynamicCommand(request uint64)`) and added it to `Driver`.
  - `sdk/tui/live/controller.go`: `Live` holds
    `dynamicCommandCancels map[uint64]context.CancelFunc` (`request` is a
    view-private counter; one Live serves one view, so IDs are unique);
    `ExecuteDynamicCommand` registers the cancel function and unregisters it
    when the RPC ends; `CancelDynamicCommand` is idempotent, and unknown or
    completed requests are no-ops.
  - `sdk/tui/view/model.go`: when Esc cancels a pending expansion, it cancels
    the RPC first (the editor unlocks immediately instead of waiting through
    the full 15s timeout), then bumps the request to discard the late result;
    when the active session changes, a pending expansion belonging to the old
    session is cancelled as well, and its late message follows the existing
    discard path (restoring the draft + the "discarded after the active
    session changed" diagnostic).
- **Session mismatch and overlay priority fixed**
  - The argument form records `dynamicArgumentSession` (the active session when
    the palette was opened); when the active session changes, a form belonging
    to the old session closes immediately (consistent with the existing file
    completion session guard).
  - Review confirmed the existing key-routing priority: gate > pending
    expansion lock > sessions > shortcuts > model picker > command confirm >
    command overlay > palette > argument form > file completion > editor.
    Palette and argument-form mutual exclusion is guaranteed by routing order
    (while the form owns the keyboard, the palette cannot re-enter), so no new
    code is needed.
- **Successful refresh clears the stale error**
  - The successful path of `applyDynamicCommandsMsg` clears a lingering
    `lastErr` with the `dynamic commands: …` prefix (following the existing
    pattern that clears the `session sidebar:` prefix), so the previous failed
    refresh's error banner no longer remains after the catalog recovers.
- **No change after evaluation**: the escaped `<loaded_skill>` envelope. Vivy's
  skill dynamic-command expansion into an equivalent plain-text instruction is
  an intentional equivalence accepted in the 2026-09-05 review; switching to
  Crush's XML envelope would only change prompt shape without behavioral
  benefit, so plain text is retained.

## Tests

- `sdk/tui/live/controller_test.go`
  - `TestDynamicCommandRefreshSuccessClearsStaleCatalogError`: a failed
    refresh sets the error → a successful refresh clears it and installs the
    catalog;
  - `TestCancelDynamicCommandAbortsInFlightExpansion`: fakeEnv records the
    context of its most recent call (`callContext`), and a blocked
    `commands/expand` returns an error immediately after
    `CancelDynamicCommand`; repeated cancellation and unknown requests are
    no-ops.
- `sdk/tui/view/command_test.go`
  - `TestDynamicCommandEscapeCancelsInFlightExpansion`: Esc records the
    cancellation, restores the draft, and silently drops the late message
    after cancellation;
  - `TestDynamicCommandSessionChangeCancelsAndClosesArgumentForm`:
    switching sessions cancels the old session's pending expansion and closes
    its argument form; when the cancellation message arrives late, it follows
    the discard path (draft restored + diagnostic).

## Explicitly not done

- REPL list/expand / real mixed MCP server end-to-end test expansion: the
  current test surface covers typed catalog, expansion, missing capabilities,
  boot/refresh races, and cancellation; real MCP server integration tests
  belong to the `MCP-TRANSPORT-1` wave (upstream mcp-go component integration).
- No `git push`.

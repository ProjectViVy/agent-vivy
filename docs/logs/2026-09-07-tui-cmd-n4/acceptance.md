# Acceptance — TUI-CMD-N4

How a human can tell the fix landed (VIVY CODE fullscreen TUI, dynamic
commands require a backend that advertises `commands.list` + `commands.expand`
with at least one skill/MCP-prompt command):

1. **Catalog is not resurrected by a stale boot**
   - Open the TUI with a slow backend: fire a palette refresh (open the
     command palette, which triggers `RefreshDynamicCommands`) while the boot
     fetch is still in flight.
   - Before: the late boot snapshot could replace the refreshed catalog.
   - After: whichever fetch completed last logically wins by epoch — a boot
     snapshot that started before a refresh never replaces that refresh's
     catalog, and a failed boot snapshot does not raise a stale
     "dynamic commands" error banner over a successful refresh.
2. **Failed turn start keeps the operator's input**
   - Type `/review something` (a dynamic skill command) and let the expansion
     succeed while the session is in a state where the turn start is refused
     (e.g. a load transition).
   - Before: the editor was left empty — the drafted slash line was gone.
   - After: `/review something` reappears in the editor with a
     "dynamic command could not start a turn" diagnostic; nothing is sent.
3. **Expansion is modal**
   - Dispatch a dynamic command; while the expansion is in flight, type text
     or press Enter — nothing appears in the editor and nothing is sent.
     Ctrl+C still quits; Esc cancels the pending expansion and restores the
     drafted slash line.
   - The command palette / shortcuts / sessions / model picker cannot be
     opened mid-expansion.

Unit-level proof for each bullet is in `summary.md` (three tests in
`sdk/tui/live/controller_test.go` and `sdk/tui/view/command_test.go`).

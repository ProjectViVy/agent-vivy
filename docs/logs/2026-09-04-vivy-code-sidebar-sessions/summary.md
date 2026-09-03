# Vivy Code sidebar and Sessions surface

## Changed

- Moved the shared fullscreen TUI model, layout, palette, renderer, keyboard
  handling, overlay placement, and nil-driver fallback into `sdk/tui/view`.
- Reduced `internal/tui/view` and `faces/tui/view` to compatibility wrappers
  over the one shared implementation, keeping built-in and packed faces in
  lockstep.
- Matched Crush's compact geometry: a terminal is compact when its width is
  below 120 columns or its height is below 30 rows; wide mode reserves a
  fixed 32-column right rail.
- Removed the session collection from the right rail. The rail now shows the
  active session and only server-owned context facts when `session/context`
  is available. Unknown cwd, model, cost, files, LSP, MCP, skills, and
  current-thinking fields are not fabricated.
- Added the independent Ctrl+S Sessions dialog with fresh `session/list`,
  current-session preselection, Unicode-safe fuzzy title filtering,
  Enter/Tab/Ctrl+Y selection,
  rename confirmation, delete confirmation, and active-busy deletion refusal.
- Wired both live faces (and the deterministic demo driver) to the shared
  session-controller seam and real `session/context`, `session/rename`, and
  `session/delete` RPCs. Failed mutations leave the authoritative state
  unchanged and preserve dialog state.
- Added monotonic request fences so late session loads/list responses cannot
  replace the operator's newer selection or mutation, and kept a pending
  approval/question above the Sessions shortcut.
- A gate that arrives while Sessions is already open now replaces that
  secondary overlay. Session loads fence `Send`; a rejected send keeps the
  editor draft rather than targeting the previously active session.

## Explicitly not done

- The control plane does not expose an independent `updated_at` timestamp,
  current cwd/model/provider/cost, aggregated modified files, LSP health, or
  session-mounted MCP/skills state. Those unavailable sections remain hidden
  rather than being inferred. These are tracked for a later contract slice.
- Slash-command breadth, shell/@/MCP input routing, and full chat history
  cursor behavior belong to the subsequent command/input wave.

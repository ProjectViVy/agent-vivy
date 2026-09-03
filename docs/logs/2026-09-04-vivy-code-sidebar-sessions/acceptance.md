# Acceptance

1. Launch either the built-in or packed fullscreen TUI at 120×30 or larger.
   The right rail is 32 columns, shows the active session/context facts, and
   does not list inactive sessions.
2. Resize to 119 columns or to 29 rows. The rail disappears and the compact
   header appears; no clipped wide layout remains.
3. Press Ctrl+S. The Sessions dialog loads the real session list, highlights
   the current session, filters by title while typing, and switches with
   Enter, Tab, or Ctrl+Y.
4. In that dialog, Ctrl+R then Enter renames the selected session; Ctrl+R then
   Esc cancels. Ctrl+X then y deletes after one confirmation; n/Esc cancels.
   Attempting to delete the active session during a run is refused in-place.
5. Make rename/delete RPCs fail. The dialog remains open with the entered
   title/selection and the old session state intact, so the action can be
   retried.

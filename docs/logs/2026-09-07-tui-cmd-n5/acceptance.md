# Acceptance — TUI-CMD-N5

How a human can tell it worked (run `vivy-code.exe` fullscreen TUI against a
server exposing dynamic commands):

1. Open the palette (`/`), pick a dynamic skill command, submit. While the
   expansion spinner state is pending, press Esc — the editor returns your
   original slash draft immediately, even if the backend is slow (previously
   the editor could stay locked for up to the 15s RPC timeout).
2. Start a dynamic command expansion, then switch the active session from
   another client. The pending expansion cancels by itself; you get the
   "discarded after the active session changed" diagnostic instead of a
   locked editor.
3. Pick a dynamic command with required arguments, type partial arguments,
   then switch the active session — the argument form closes instead of
   dispatching the command into the wrong session.
4. Make the dynamic-command catalog fail once (e.g. stop the backend while
   refreshing), see the error banner, restore the backend and reopen the
   palette — the stale catalog error clears once the refresh succeeds.

Rollback: revert the single commit for this slice; Esc/session-switch then
merely discard the future result without cancelling the RPC.

# Acceptance — how a human can tell it worked

Run a live `vivy-code` session (or `just vivy-code` then launch the binary
inside a project).

## F10 — sidebar Modified Files

1. Let the session edit a few files (or reopen a session that already did).
2. Look at the right rail's `Modified Files` section:
   - the `+N` count is green and the `-N` count is red (path/time stay dim);
   - a long path shows `…` in the middle with the actual filename still
     readable at the end (e.g. `pkg/dee…long/engine.go  +12 -3`);
   - short paths keep their ` · time` suffix; very long paths may drop the
     timestamp so the filename survives.
3. Nothing overflows the rail; no row wraps or spills past the border.

## F7 — terminal window title

1. The terminal tab/window title shows `VIVY CODE` as soon as the TUI starts.
2. After the first session title exists (auto-derived from the opening prompt
   or set via `/rename <title>`), the title becomes `VIVY CODE · <title>`.
3. Renaming the session or switching sessions in the sessions dialog updates
   the title; a session without a title falls back to the bare brand.

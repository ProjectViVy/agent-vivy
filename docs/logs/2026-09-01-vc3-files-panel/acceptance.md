# Acceptance — VC-3g files panel

## How a human verifies it works

1. Run `just dev` (or the split pair) and open `http://127.0.0.1:3015`.
2. A folder-icon button labeled Files appears at the top right, alongside Todos
   and Review Center.
3. Start a run that writes a file (for example, ask vivy to write `notes.md`),
   then click Files while the run is active or after it finishes:
   - the panel lists files in that run's workspace (relative path + size, sorted
     by path);
   - clicking a text file shows a syntax-highlighted preview on the right/below
     (dark GitHub theme);
   - oversized files show a truncation marker, and binary files show a "Binary
     file" placeholder rather than mojibake;
   - when there are more than 2000 files, the list shows a truncation marker
     (normal runs do not reach this).
4. With no run, the panel is honest: the empty state says "Start a run to view
   its workspace files."
5. The Chinese interface shows its localized Files copy; the English interface
   shows Files and English copy.

## Boundaries (must not happen)

- Attempts to read `../`, absolute paths, or drive-letter paths are all rejected
  (verified by the real-server smoke test, without leaking internal error
  details).
- There are no write/delete/download entry points—the panel is a read-only
  preview; restore/rollback belongs to file-version history (awaiting the O1..O6
  ruling).
- When `WorkspaceFiles` is not assembled (for example, workspace is disabled),
  `workspace/*` RPCs return MethodNotFound and the UI does not pretend the list is
  empty.

## Acceptance on a machine without a provider

This machine has no OPENAI/ANTHROPIC key, so browser acceptance covers only the
panel shell (empty state + toggle). Running the full "run produces a file → panel
highlighted preview" flow once on a machine with a key completes acceptance; the
kernel list/read path is covered by the real-server WebSocket smoke test (see the
table in verification.md).

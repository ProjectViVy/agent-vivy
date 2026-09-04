# Vivy Code project-file context

## Delivered

- Extended the shared `sdk/tui/command` language with an independent `Shell`
  kind (`!script`) and `File` kind (`@path`). Shell scripts preserve every
  byte after the marker; `!!` and `@@` escape one marker. File markers are
  recognized only at line start or after Unicode whitespace, and are removed
  from the model prompt while their untrusted path tokens are retained.
- Added shared view seams for project-context sending and a client-only shell
  request. The view never executes a process or reads a file; drivers without
  the required seam fail closed.
- Wired the built-in fullscreen TUI, packed face TUI, and legacy REPL through
  the same client contracts. `@file` first calls `project-context/resolve` for
  bounded metadata and then sends the original `context_paths` again in
  `turn/start`, allowing the server to re-resolve at the run boundary.
- Added the server-owned `project-context/resolve`/`list` boundary with an
  explicit ProjectRoot, `os.OpenRoot`, canonical containment, sensitive-file
  and symlink-target denial, UTF-8/control validation, and 1 MiB-per-file,
  8-file, 4 MiB-total limits. Absolute, traversal, UNC/drive, Windows ADS,
  binary, oversized, and escaping inputs fail with stable path-free errors.
- Added a separate durable `FileContext` snapshot model and SQLite/Postgres
  migrations. Append, edit, fork, delete, session replay, context accounting,
  compaction accounting, and model-request digests include file snapshots;
  history RPC/TUI rendering projects metadata only. Session compaction keeps
  snapshot-owning messages verbatim instead of folding their bodies into a
  lossy summary, including equal-millisecond boundaries; Eino's in-run
  summarizer also preserves the exact bounded project-file messages.
- Queue snapshots preserve context paths. If resolution or `turn/start`
  revalidation fails, the shared editor restores a retryable `@file` draft in
  both built-in and packed fullscreen faces.
- Added the `shell/start` client seam with exactly `session_id` and `script`.
  Shell runs enter the existing `run/subscribe`/event/approval/cancel
  lifecycle after acceptance; no TUI or packed face has a local process or
  broker execution fallback.

## Explicitly not delivered here

`!shell` server execution is not implemented. This cut includes only parser
classification and a client `shell/start` seam so neither terminal face can
fall back to local execution or forward the script to the model. The server
returns method-not-found and help labels the feature unavailable. A later
delivery must implement `runtime.Service.RunShell` through the complete bash
governance path: validation/classification, policy, hooks, proposal/approval,
durable Journal/tool lifecycle, resume/cancel/terminal events, redaction, and
bounded output.

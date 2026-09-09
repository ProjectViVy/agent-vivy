# CMP-1 — reduction clear offload Backend (file-level recovery)

## What changed

The clear phase of eino reduction previously ran only in clear-only mode
(`Backend: nil`) in Vivy: old tool results being cleared became in-memory
placeholders and their content was discarded completely. This slice connects
the offload side:

- `runtime.EngineConfig` adds `OffloadBackend *EinoFilesystemBackend`
  (`internal/runtime/engine.go`). A concrete type is used instead of the eino
  interface: boxing a typed-nil pointer in a non-nil interface would make eino
  misidentify offload mode and then fail every write on a nil receiver. A nil
  check is performed in `buildCompactionHandlers` before conversion, structurally
  excluding typed-nil values.
- `buildCompactionHandlers` receives a new `offload` argument and writes it to
  eino reduction's `Config.Backend`; `ReadFileToolName: tools.ReadFileName`
  names Vivy's `read_file` in the placeholder text; and
  `GenClearOffloadFilePath: genClearOffloadPath` is wired in.
- `genClearOffloadPath` (`internal/runtime/compaction_middleware.go`) produces
  the workspace-relative, forward-slash path `compaction/clear/<call-id>`. The
  eino default is `filepath.Join(RootDir, "clear", callID)` (`RootDir` defaults
  to `/tmp`, and Windows backslashes enter the placeholder text); with the
  custom path, the placeholder text can be opened directly by `read_file` on
  every platform.
- `safeOffloadCallID`: the provider-supplied call ID becomes the filename. The
  eino default implementation does not sanitize it—malicious or abnormal IDs
  (`..\x`, `../../x`) would try to escape the run workspace (although the
  downstream `safeWorkspacePath` fails closed, it would make the entire clear
  operation error). The whitelist is `[a-zA-Z0-9_-]` with a maximum length of
  128; an out-of-range or empty value falls back to `uuid.NewString()` (matching
  eino's default semantics). `google/uuid` in go.mod was changed from indirect
  to direct.
- App wiring (`internal/app/compaction.go` `buildEngineConfig` plus two call
  sites in app.go) reuses the existing `fileBackend` (the same run-workspace
  backend as AgentsMDBackend); the startup and settings-save reload paths are
  consistent.

## Behavior

- With a workspace (`runtime.workspace_root` configured), a clear trigger writes
  the old tool-result content to `<run-workspace>/compaction/clear/<call-id>`.
  The placeholder text is `<persisted-output>Tool result saved to:
  compaction/clear/<call-id> … Use read_file to view.`, and the model can
  retrieve it with `read_file` within the same run.
- Without a workspace (`fileBackend` is nil, as in a direct runtime test
  harness), behavior remains byte-for-byte identical to the old version
  (in-memory placeholder with no offload).

## What was explicitly not done

- Cross-run lifecycle and cleanup for offload files: the run workspace already
  provides isolation and cleanup semantics, and offload is meaningful only
  within a run, so no additional TTL or cleanup hook was added.
- Offloading summarization results (eino summarization has no such seam).
- The compaction-settings UI overlay (a CMP-2 leftover covered by a separate
  slice).

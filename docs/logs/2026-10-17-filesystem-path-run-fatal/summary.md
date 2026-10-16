# Filesystem path mistakes no longer fail the whole run

Date: 2026-10-17
Scope: Vivy kernel — `internal/runtime` filesystem backend only.
Related incident: `run_8d376d0f457fa5e5` (dev home), reported as

```text
engine event error: [NodeRunError] failed to stream tool call chatcmpl-tool-b98efc6e734be6ff: filesystem: invalid path
node path: [node_1, ToolNode]
```

## What happened

The session ran under the read-only sandbox. `bash` was refused (correctly), so
the model probed the run workspace with `list_dir`:

| call | arguments | outcome before this change |
| --- | --- | --- |
| `chatcmpl-tool-a3d4240d1153e449` | `{"depth":"2","max_entries":"200","path":"."}` | ok (empty workspace) |
| `chatcmpl-tool-b8b01547253d3727` | `{"depth":"1","path":"/"}` | silently reinterpreted as the workspace root |
| `chatcmpl-tool-b98efc6e734be6ff` | `{"path":"./"}` | `filesystem: invalid path` → **whole run failed** |

Two defects combined:

1. `safeWorkspacePath` (`internal/runtime/filesystem_backend.go`) rejected any
   value that cleaned to `"."` but was not literally `"."`. `"./"`, `".//"` and
   `"./."` are ordinary workspace-relative spellings and were rejected while
   `"."` was accepted. The same clause silently accepted `"/"` and `"\foo"` on
   Windows, because `filepath.IsAbs` reports `false` for volume-less Windows
   roots, so the model's rooted path quietly listed the workspace root instead
   of being refused (on Linux the same call is refused).
2. Eino's `compose.ToolsNode.Stream` fails the node when a tool returns a
   non-interrupt error (`compose/tool_node.go:1309`
   "failed to stream tool call %s: %w"). Vivy's soft conversion (NUDGE-DESIGN
   §5, `internal/runtime/tool_failure.go`) only recovers an allowlisted subset —
   `*tools.ArgError`, `fs.ErrNotExist` from a readonly tool, and typed MCP tool
   errors. The filesystem backend raised plain `fmt.Errorf` values for its own
   caller-path defects, so the model's argument mistake was classified fatal and
   killed the run.

## What changed

- `internal/runtime/filesystem_backend.go` — `safeWorkspacePath` now resolves
  `"."`-cleaning spellings (`""`, `"."`, `"./"`, `".//"`, `"./."`) to the
  workspace root instead of erroring; added `rootedPath`, which refuses a
  leading `/` or `\` on every platform, so Windows no longer reinterprets a
  rooted path as workspace-relative; caller-path defects now return
  `*tools.ArgError{Field: "path"}` through the new `pathArgumentError` helper
  (workspace-relative, inside-the-workspace, component-not-a-directory), and
  `ListDir`'s "target is not a directory" check does the same. These are
  argument-value failures of the model's own call, so §5 soft conversion turns
  them into an unsuccessful tool result with a bounded diagnostic: the run
  continues and the model can correct the next call.
- `internal/runtime/filesystem_backend_test.go` — two regression tests
  (root spellings accepted; path defects are typed `ArgError` and classify as
  recoverable).
- `internal/runtime/tool_failure_test.go` — `TestFilesystemPathMistakeRecoversInsideRun`,
  a service-level replay of the incident through the real tool/backend.
- `sdk/internal/assembly/conformance_results.json` — the five `internal`-rooted
  `sourceSha256` rows re-pinned to the new `internal/sourcehash.Tree` identity
  (`1acae684…` → `982fc2a3…`), as every `internal/` edit requires. Record:
  [`ci.md`](ci.md).
- `docs/TODO.md` — `TOOL-REFUSAL-RESIDUAL` updated with the partial fix and the
  still-fatal sibling errors.
- Deliberate boundary denials (`protected path denied`, `symlink path denied`)
  and environment errors stay plain errors and stay fatal **in this cut**; the
  follow-up cut converts the boundary denials to typed refusals — see
  [`../2026-10-17-tool-argument-run-fatal/summary.md`](../2026-10-17-tool-argument-run-fatal/summary.md).
  See the TODO note above.

## Explicitly not done

- No change to the §5 allowlist, the tool adapter, or Eino usage. The fix uses
  the existing `*tools.ArgError` row of the classification table.
- The remaining run-fatal argument-value errors in the same filesystem tools are
  not converted in this cut: `patch`/`multiedit` "old_string was not found" /
  "matched N times", `read_file` `end_line < start_line` and the image over-cap
  message, `search_files` "query must not be empty", `glob`/`grep` pattern
  errors, and `write_file` "content exceeds N bytes". They are recorded on the
  existing `TOOL-REFUSAL-RESIDUAL` board row.
- No UI, RPC, storage, schema, or product-contract change. No migration.

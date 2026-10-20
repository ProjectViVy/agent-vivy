# Filesystem path mistakes no longer fail the whole run (2026-10-20)

## What was wrong

A normal chat turn in the dev scratch home (`.workspace/dev/home-clean`) ended
after about twelve seconds with no answer. The run was
`run_83e4d830e5a6af53` in session `sess_edcf4ebb2275dca4`; the user prompt was
“看看工作区有啥”. The model called:

```json
{"depth":"2","path":"./","recursive":"true"}
```

`list_dir` returned `filesystem: invalid path`. The tool adapter did not
classify that plain error as recoverable, so Eino's ToolsNode wrapped it as
`failed to stream tool call ...` and raised a `NodeRunError`. The run failed
with `cause_category: internal_error` and the generic user-facing message
“The model run could not be completed. Please try again.” The process was not
hung; the browser received `run.failed` instead of a final answer.

Root cause had three parts:

1. `safeWorkspacePath` used `filepath.Clean(filepath.FromSlash(value))` and
   rejected any cleaned-to-root value except the literal `"."`
   (`clean == "." && value != "."`). `filepath.Clean("./") == "."`, so a valid
   workspace-root alias was treated as invalid.
2. The validation error was a plain `error`, outside the
   `classifyToolFailure` allowlist (`*tools.ArgError`, readonly
   `fs.ErrNotExist`, `*mcphost.ToolExecutionError`). It therefore kept its
   fatal path through the adapter and Eino.
3. Empty directory listings serialized as `"entries":null` instead of `[]`,
   which is an adjacent contract defect the model would have seen next.

## What changed

- `internal/runtime/filesystem_backend.go`
  - Removed the `clean == "." && value != "."` rejection; `"./"`, `"./."`,
    `"foo/.."` and similar aliases now resolve to the workspace root.
  - `safeWorkspacePath` now returns `*tools.ArgError{Field: "path"}` for
    workspace-relative, containment, protected-path, symlink and
    not-a-directory component errors. These are already classified as
    recoverable `invalid_arguments`, so the model sees the problem and the run
    continues.
  - `safeWorkspacePath` still returns the `os.ErrNotExist` chain for missing
    paths, preserving `agentsmd`'s absent-file skip and readonly `not_found`
    classification. Runtime inspection failures (permissions/IO) remain
    ordinary fatal errors.
  - `ListDir`'s “target is not a directory” error is also an argument error,
    and its result starts with a non-nil `Entries` slice.
- `internal/runtime/filesystem_path_test.go` (new)
  - Root aliases are accepted.
  - Bad path shapes become `*tools.ArgError`.
  - Empty listings encode `"entries":[]`.
  - Real backend errors classify as recoverable `invalid_arguments` through
    `classifyToolFailure`.
  - Missing read targets keep the `os.ErrNotExist` chain.
- `sdk/internal/assembly/conformance_results.json`
  - Re-pinned the five internal-rooted Provider `sourceSha256` values from
    `9ed7deb7…` to `58e65c93…`. The provider-conformance identity covers every
    file under `internal/`, so this source edit necessarily moves it; the
    checked-in artifact must carry the new value or
    `TestCheckedInProviderConformanceMatchesExecutedSuites` fails before any
    suite runs.

No classifier allowlist, schema, migration, RPC, event payload or UI change.
The security checks still run before any access; only the error type changed.

## Scope

Two source files, one new test file, the conformance evidence artifact, this
log, and a board update. The board row `TOOL-REFUSAL-RESIDUAL` remains the
deferred umbrella for the other fatal tool paths; its note now records this
sub-case as fixed. The stale duplicate row in `docs/TODO.md` section 0.1 was
removed because `docs/DEFER.MD` is the single deferred record.

Local `just ci` could not complete on this host: `vite build` fails while
removing its esbuild temp file (`Access is denied`), and the identical failure
reproduces on a clean `main` checkout, so it is environmental and independent
of this change. The finding is recorded as `CI-VITE-ESBUILD-TEMP` in
`docs/TODO.md` §0.1. UI typecheck, all 643 Vitest tests, `vet`, `fmt-check`,
`headless-compile`, `plugin-ci`, `internal/runtime`, and `internal/tools`
passed; details are in `verification.md`.

## Explicitly not done

- Other backend I/O failures stay fatal: for example `read_file` on a
  directory, permission-denied reads, or an internal storage failure. They are
  not path-shape validation and must not be presented to the model as if the
  call were merely refused.
- Unknown tool names (`UnknownToolsHandler`), pre-tool hook blocks and the
  “tool is not selected” guard are untouched; they remain under
  `TOOL-REFUSAL-RESIDUAL`.
- No global allowlist widening, no changes to write/missing-path fatal
  semantics, and no attempt to retry automatically. The model decides how to
  correct the call.

## Related evidence

- `docs/logs/2026-09-19-approval-resume-and-timeout/` recorded the prior
  same-class incident (`run_d32ebd41a4203750`), which fixed governance
  denials but deliberately left genuine tool-implementation errors fatal.
- `docs/plans/2026-10-19-complete-deletion-plan.md` and the Issue #91 Story
  package preserve this path validation and the fatal-classification
  boundary; they do not fix this incident by themselves.

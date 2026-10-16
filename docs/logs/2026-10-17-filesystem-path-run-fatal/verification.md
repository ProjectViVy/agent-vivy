# Verification — filesystem path run-fatal fix

Date: 2026-10-17
Tree: `feat/issue-58-unified-context` working tree (root; one write lane active).

## Incident evidence (read-only, before the change)

The failing run was read back from the dev home Journal
(`.workspace/dev-home/vivy.db`, `run_events`/`tool_operations`/`messages` for
`run_8d376d0f457fa5e5`, session `sess_a67760830555585e`,
`sandbox_mode = read_only`):

- `chatcmpl-tool-b98efc6e734be6ff`, tool `list_dir`, effective arguments
  `{"path":"./"}`, `state=completed`, `failure = filesystem: invalid path`.
- `chatcmpl-tool-b8b01547253d3727`, tool `list_dir`, `{"depth":"1","path":"/"}`
  returned the workspace root listing (`.`) with no error.

Path semantics were confirmed with a throwaway probe
(`.workspace/tmp/pathprobe`, deleted after use):

```text
"./"    fromSlash=".\\"  isAbs=false  clean="."   join="C:\\root"
"/"     fromSlash="\\"   isAbs=false  clean="\\"  join="C:\\root"
"/foo"  fromSlash="\\foo" isAbs=false clean="\\foo" join="C:\\root\\foo"
"../escape" clean="..\\escape" join="C:\\escape"
"C:foo" vol="C:"
```

## Commands and results

| Command | Result |
| --- | --- |
| `gofmt -l internal/runtime/{filesystem_backend,filesystem_backend_test,tool_failure_test}.go` | no output (formatted) |
| `go vet ./internal/runtime/` | clean |
| `go test ./internal/runtime/ -run 'TestEinoFilesystemBackend\|TestFilesystemPathMistakeRecoversInsideRun\|TestToolFailure' -count=1 -v` | PASS, 10.0s — all pre-existing filesystem/tool-failure tests plus the three new ones |
| Pre-fix replay: `git stash push -- internal/runtime/filesystem_backend.go` then the same focused run | **FAIL** — `TestEinoFilesystemBackendAcceptsWorkspaceRootSpellings`: `list_dir "./" = filesystem: invalid path`; `TestEinoFilesystemBackendPathDefectsAreArgumentErrors`: every case returned a plain error (and `path "/"` returned no error at all); `TestFilesystemPathMistakeRecoversInsideRun`: `run run_686befda23524504 never reached status completed (last status failed)` — the incident reproduced end to end. Stash popped afterwards; the working tree was then restored to the fix. |
| `just ci` | `fmt-check`, `ui-ci` (692 vitest + typecheck + vite build) and `vet` PASS; `test` FAILS on pre-existing environment limits (Go build-cache access denied, missing Windows symlink privilege). Full breakdown and pre-change comparison: [`ci.md`](ci.md). |

## New tests

`internal/runtime/filesystem_backend_test.go`

- `TestEinoFilesystemBackendAcceptsWorkspaceRootSpellings` — `""`, `"."`,
  `"./"`, `".//"`, `"./."` all list the workspace root; `read_file "./root.txt"`
  reads normally.
- `TestEinoFilesystemBackendPathDefectsAreArgumentErrors` — `/etc`, `/`,
  `\Windows`, `C:\Windows` (Windows only), `../outside`, `a\x00b`,
  `root.txt/nested`, a file as the list target, and `search_files` with a rooted
  path all return `*tools.ArgError{Field:"path"}` and classify as
  recoverable/`invalid_arguments`.

`internal/runtime/tool_failure_test.go`

- `TestFilesystemPathMistakeRecoversInsideRun` — service-level arc through the
  real `Service` + Eino `ChatModelAgent`/`ToolsNode` + real `list_dir` tool +
  real `EinoFilesystemBackend` under a read-only sandbox: the model calls
  `list_dir "./"` (clean root listing) and `list_dir "/"` in one turn; the
  second call's `tool.finished` row is `recoverable`/`invalid_arguments` with a
  model-visible `workspace-relative` diagnostic, and the run reaches
  `completed`. This is the real-path smoke for this change: it exercises the
  same engine/Journal path the incident ran on, with the tool implementation
  under test rather than a stub.

## Unverified / limitations

- No live interactive browser reproduction was run: the incident's provider
  calls are not replayable without credentials, and the kernel change has no
  UI surface. The service-level test above is the functional equivalent of the
  failing run (same Engine, ToolNode, adapter, Journal, real tool).
- `just ci` cannot go fully green on this machine: the `test` recipe fails on
  two pre-existing environment limits (Go build cache access denied for the
  test process; Windows symlink privilege / Developer Mode off), both present in
  the 2026-10-16 pre-change CI log. `diva-recall-test`, `headless-compile` and
  `plugin-ci` were not reached for that reason; none of them covers this change.
- PostgreSQL conformance was not re-run: no storage, schema or migration change.
- The re-pinned conformance digest was verified through the release gate's own
  identity comparison and the `sdk/internal/assembly` package, not by a fully
  green `just release-conformance` (blocked by the same symlink privilege).

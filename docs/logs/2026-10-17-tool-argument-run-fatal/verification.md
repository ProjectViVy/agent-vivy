# Verification — 2026-10-17 tool argument run-fatal cut

All commands ran from the repository root on the
`feat/issue-58-unified-context` working tree, 2026-10-17.

## Focused gates

| Command | Result |
| --- | --- |
| `gofmt -l internal/runtime internal/tools` | clean |
| `go vet ./internal/runtime/ ./internal/tools/` | clean |
| `go test ./internal/runtime ./internal/tools -count=1 -run 'TestToolFailure\|TestFilesystemPathMistake\|TestEinoFilesystemBackend\|TestApplyStringPatch\|TestBackendPatch\|TestBackendMultiPatch\|TestBoundaryDenial\|TestBackendGrep\|TestPatchArgument\|TestProtectedPath\|TestToolArgumentDecode\|TestJobsTool'` | `ok agent-vivy/internal/runtime`, `ok agent-vivy/internal/tools` |
| `go test ./internal/tools -count=1` | ok |
| `go test ./internal/runtime -count=1` (full package, earlier in the cut) | one existing text-asserting test failed and was updated — see below |

The full-package run surfaced exactly one affected existing test,
`TestBackendGrepRejectsInvalidPattern`, which asserted the old bare message
("invalid grep pattern"). It now asserts the typed `*tools.ArgError{pattern}`
instead of the wording; every other test in the package passed.

## Pre-fix replay (the fix is what makes the tests pass)

Two surgical replays. Each reverts only the source file(s) that carry one seam,
keeps every new test in place, and requires the tree to keep compiling.

### Replay 1 — approval pre-flight and boundary refusals reverted

```
git stash push -m replay-s2-s3-adapter -- internal/runtime/tooladapter.go
go test ./internal/runtime -count=1 -run 'TestPatchArgumentMistakeRecoversWithoutApprovalUnderAsk|TestPatchArgumentMistakeRecoversWhenAutoApproved|TestProtectedPathReadIsRefusedInsideRun'
# --- FAIL: TestPatchArgumentMistakeRecoversWithoutApprovalUnderAsk (30.77s)
# --- PASS: TestPatchArgumentMistakeRecoversWhenAutoApproved (0.89s)
# --- FAIL: TestProtectedPathReadIsRefusedInsideRun (30.60s)
# FAIL agent-vivy/internal/runtime 70.759s
git stash pop
```

The `ask` case was replayed against a harness with the checkpoint store wired
(`checkpoints: true`, the production shape of that interrupt). Pre-fix it failed
in `handleInterrupt` with the tool-argument error surfacing from the proposal
build, and the run ended `failed`:

```
WARN interrupt handling failed run=run_f65c3f5c8972a5d0 err="tool argument \"old_string\": was not found in the file; re-read it and copy the exact text"
tool_failure_test.go:536: run run_f65c3f5c8972a5d0 never reached status completed (last status failed)
```

The protected-path case fails with the reported node error:

```
WARN run failed run=run_1deef48c979e690e err="engine event error: [NodeRunError] failed to stream tool call call-read-env: runtime: protected path denied: .env is a protected path"
node path: [node_1, ToolNode]
```

(`TestPatchArgumentMistakeRecoversWhenAutoApproved` still passes because it
covers the invocation seam, which this stash does not touch — that is the
expected split between the two seams.)

### Replay 2 — argument typing reverted

```
git stash push -m replay-s1-typing -- internal/runtime/filesystem_backend.go internal/runtime/patch_engine.go internal/runtime/search_backend.go
go test ./internal/runtime -count=1 -run 'TestEinoFilesystemBackendArgumentDefectsAreTypedArgumentErrors|TestApplyStringPatchDefectsAreTypedArgumentErrors|TestPatchArgumentMistakeRecoversWhenAutoApproved|TestPatchArgumentMistakeRecoversWithoutApprovalUnderAsk'
# --- FAIL: all four (0.05s / 0.00s / 30.56s / 30.60s), FAIL agent-vivy/internal/runtime 67.904s
git stash pop
```

Pre-fix service-level signature:

```
WARN run failed run=run_f83b378ea5973ac1 err="engine event error: [NodeRunError] failed to stream tool call call-patch-auto: filesystem: old_string was not found"
node path: [node_1, ToolNode]
tool_failure_test.go:577: run run_f83b378ea5973ac1 never reached status completed (last status failed)
```

After `git stash pop` the focused set above is green again.

## Real-path smoke (split pair, real backend, real UI, disposable home)

No provider key and no committed fixture: a disposable backend on
`127.0.0.1:8899` with `VIVY_USER_HOME=.workspace/tmp/smoke-home-prefix`, the
frozen environment-variable provider (`VIVY_PROVIDER=deepseek
VIVY_API_BASE=http://127.0.0.1:9931/v1 VIVY_MODEL=mock-fast
DEEPSEEK_API_KEY=anything`), a local OpenAI-compatible mock that emits one
`patch` call with `{"path":"notes.txt","old_string":"NOPE","new_string":"x"}`,
and Vite on `127.0.0.1:3017` with `VIVY_BACKEND_ADDR=http://127.0.0.1:8899`.
The split pair already running on :3015/:8787 was not touched, and nothing was
written under `data/`.

The same prompt was sent through the browser twice: once with the three typing
files stashed (pre-fix), once with the fix restored (post-fix). Chromium was
driven headless through the repository's Playwright install.

| | Pre-fix | Post-fix |
| --- | --- | --- |
| Transcript | `Stopped` / `Edit` / `notes.txt` / **"This conversation did not finish"** / **"The run could not be paused for approval. Please try again."** | `Failed` / `Edit` / `tool argument "path": does not exist; create it with write_file first`, then the assistant's next message |
| Run banner | `failed` | `completed` |
| Approval prompt | none (the run died before it) | none |
| Backend log | `WARN interrupt handling failed run=run_0ff9abd7bd318b83 err="filesystem: path does not exist: file does not exist"` (`service.go:4065`) | no warning |

Ledger truth from the disposable home (`vivy.db`, opened read-only):

```
runs:            [('run_0ff9abd7bd318b83','failed'), ('run_ea3c5c36a3c36773','completed')]
approvals rows:  0
event types:     run.failed 1 (pre-fix run), run.completed 1, tool.finished 1, tool.approval_required 0
run.failed:      {"cause_category":"internal_error","message":"The run could not be paused for approval. Please try again.","outcome":"failed",...}
tool.finished:   {"tool_name":"patch","outcome":"recoverable","reason":"invalid_arguments",
                  "error":"tool argument \"path\": does not exist; create it with write_file first",...}
run.completed:   {"outcome":"completed","summary":"已收到工具反馈，继续。",...}
```

Screenshot of the post-fix transcript: `.workspace/tmp/smoke-ui.png` (scratch,
gitignored). The smoke script, mock and journal queries are likewise under
`.workspace/tmp/` and are not part of the deliverable.

## Source digest and conformance

```
# run from .workspace/wt-pin, a detached worktree standing at this commit
go run ./sdk/internal/cmd/source-hash internal 04f63283687fbada68ccf6b13a2d1a3b8090a22c1a86ae9da44b299fea9896a9
# 04f63283687fbada68ccf6b13a2d1a3b8090a22c1a86ae9da44b299fea9896a9   (5 rows re-pinned)
go run ./sdk/internal/cmd/source-hash internal 04f63283…
# 04f63283… (idempotent: the declared digest is zeroed before hashing, so the
# tree's own value is the fixed point)
```

The pin must be computed where the working tree equals the commit. An earlier
attempt computed it from the shared root tree after a second lane's uncommitted
`internal/` edits had landed; that produced `dd773820…`, which the release gate
rejected (`release source digest … = 04f63283…, checked-in evidence covers
dd773820…`). The delivered value is the one the fixed-point check and the
release gate agree on.

| Command | Result |
| --- | --- |
| `go test ./sdk/internal/assembly -count=1` (worktree at this commit) | passes except `TestHashSourceTreeRejectsSymbolicLinks` (host symlink privilege) |
| `VIVY_RELEASE_CONFORMANCE=1 go test ./sdk/internal/conformance -run '^TestCheckedInProviderConformanceMatchesExecutedSuites$'` (worktree at this commit) | digest check passes; the only failure is the `full-ui` fixture that shells out to `TestPackAndInspectSealUIAssemblyIdentity` and hits the host symlink privilege |

## `just ci`

See [`ci.md`](ci.md) for the recipe-by-recipe record. Summary: `fmt-check`,
`ui-ci` (i18n + typecheck + 693 vitest tests + `vite build`) and `vet` pass; the
`test` recipe's only failures are the three suites that need the Windows symlink
privilege this host does not hold (`sdk/internal`, `sdk/internal/assembly`,
`sdk/internal/conformance`), and the packages this change touches
(`internal/runtime` 605.9s, `internal/tools`, plus `internal/rpc`,
`internal/provider`, `agent-vivy/ui`, `sdk/conformance`) pass. The record was
taken in a detached worktree at this commit because a second lane was writing
`internal/` and `ui/` in the shared root tree; one root-tree run is kept as an
advisory log and shows that lane's mid-write errors.

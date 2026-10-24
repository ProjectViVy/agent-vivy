# Sub-agent fault isolation — verification

## Environment

- Host: macOS arm64, worktree `.worktrees/agent-tool-fault-isolation`, branch
  `fix/agent-tool-fault-isolation` (base `stable` at `0f1f7720`).
- Host Go is 1.27.1; the module pins Go 1.26.4 and `google.golang.org/grpc v1.64.1` does not build
  under 1.27 (`undefined: http2.TrailerPrefix`). Every Go command below ran with
  `GOTOOLCHAIN=go1.26.4 GOPROXY=https://goproxy.cn,direct`.
- A fresh worktree has no `ui/dist`; the ignored build output was copied from the root checkout so
  `ui/embed.go` (`//go:embed all:dist`) could compile. Nothing under `data/` was read or written.

## Commands and results

| Command | Result |
| --- | --- |
| `gofmt -l <touched go files>` | no output |
| `go vet ./internal/tools ./internal/storage ./internal/runtime ./internal/app` | clean |
| `go build ./...` | clean |
| `go test ./internal/runtime ./internal/app ./internal/tools ./internal/storage/... -count=1` | all packages ok: `internal/runtime 72.288s`, `internal/app 20.238s`, `internal/tools 2.780s`, `internal/storage 0.313s`, `internal/storage/migrations 4.855s`, `internal/storage/postgres 0.356s`, `internal/storage/sqlite 14.333s` |
| `git diff --check` | clean |
| `go test ./internal/app -run TestAgentChildFailureKeepsParentRunAlive -count=1 -v` | `PASS (0.27s)` |

The focused E2E re-run emitted the new attribution line:

```text
WARN child run failed run=child_6ca83b213adc1732 err="engine event error: [NodeRunError] error, status code: 500, ... body: child provider exploded"
```

That line is the record the incident could not produce: the child's own provider failure is now
visible and named, while the parent run continues.

## What the tests pin

- `internal/runtime/tool_failure_test.go`
  - `*tools.ChildTaskError` classifies as `recoverable / child_failed / effects unknown` with a
    bounded diagnostic.
  - The same classification is skipped when the cause is a cancellation.
  - `TestToolInvocationFailureIsNeverAProviderFailure`: a `toolInvocationError` whose text contains
    `API key missing` and `status code: 500` is not a provider failure and maps to
    `causeInternalError`; a bare `API key missing` error still is one, so the exclusion is typed, not
    a widened text match.
- `internal/tools/agent_test.go`
  - Missing, empty, non-object and oversized tasks all return a structured `*tools.ArgError`, which
    is what routes a model mistake to the existing `invalid_arguments` recovery.
- `internal/runtime/service_test.go`
  - The one-shot child's admitted prompt instruction equals `childStaticInstruction` exactly, the
    admitted persona source is `runtime/task-child` and its body is the same text, and the
    restricted engine's override matches.
- `internal/app/agent_e2e_test.go` (`TestAgentChildFailureKeepsParentRunAlive`, new)
  - Real HTTP OpenAI-compatible server: the child request (detected by its system instruction)
    returns HTTP 500, the parent delegates then answers.
  - Asserts: parent status `completed`; no parent `run.failed`; the tool result carries
    `sub-agent failed`; the durable operation outcome is `recoverable / child_failed / unknown`;
    at least two parent provider calls and at least one child call.

## Deviations from the approved plan

- The plan proposed a new in-package `internal/runtime` test wiring `runtime.NewScriptedModel` to the
  real `StartOneShotChild`. The app E2E above exercises the same path with the real provider adapter,
  child admission and prompt machinery, so it replaces that test instead of duplicating it.
- The plan's E2E assertion "`cause_category != provider_error` anywhere" was narrowed to "no parent
  `run.failed`". The child's own HTTP 500 is a genuine provider failure and is correctly classified
  as such on the child run; the regression being pinned is that it never becomes the parent's
  terminal.

## Unrun lanes

- `just wiring-ci` + `just kernel-ci` require an independent Linux-filesystem checkout; not run on
  this macOS host. Must run at the candidate before merge, after the expected rebase onto
  `fix/run-failure-repair`.
- Split-pair browser smoke (`just run` + `cd ui && pnpm dev`, `http://127.0.0.1:3015`) is owner-run
  because it needs a live provider key. The deterministic app E2E is the automated evidence for the
  same behavior.
- Windows native acceptance: not applicable (no Windows-specific path touched).

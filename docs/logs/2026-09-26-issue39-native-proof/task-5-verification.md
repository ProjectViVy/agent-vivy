# Task 5 verification

## Baseline and preserved state

- Starting HEAD: `b46998a67b9d17d9ab7cc1c9f27ad4c22506fe23`.
- Initial status contained exactly the three inherited untracked files named
  below. They were not edited, deleted, or staged.
- SHA-256 values before documentation edits:
  - `.omo/evidence/task4-executor-verification-b46998a6.md`:
    `e0007b5d41253ab67753a0ec8528cb0ad157f14b80028975094becc2e15e1813`
  - `.omo/evidence/task4-g0-red-proof-code-review.md`:
    `f528c9f59fb3e88c7eaca820213984d78bac733ca4ddd8a713cf86ddc91fcb4a`
  - `docs/logs/2026-09-26-issue39-native-proof/task-4-verification.md`:
    `fe0d3e4578a811d941d00e40cc0544136e093232953d9a2f224fcb081e96a92a`
- `git diff --exit-code b77090618c175c8c1dec3aeafcb3576d5d5e0b38..HEAD -- internal/runtime/service.go internal/runtime/approval_test.go`
  exited `0` with no output.

## Exact runtime observations

### Inherited real Service/Eino RED

```text
'/mnt/c/Program Files/Go/bin/go.exe' test ./internal/runtime -run '^TestOrchestrationNative$' -count=1 -v
```

Exit `1`. `TestOrchestrationNative` executed, Eino reported node path `[b]`,
and the real Service boundary returned
`runtime: native orchestration unimplemented`. This is the required unchanged
RED, not `[no tests to run]` and not a compile failure.

### Existing unknown-effect contract

```text
'/mnt/c/Program Files/Go/bin/go.exe' test ./internal/runtime -run '^TestToolFailureUnknownEffectsCountOnce$' -count=1 -v
```

Exit `0`. The named test executed and passed. Its production-adjacent contract
states that an invocation which may have crossed the effect boundary is
recorded as `effects=unknown`; the test only establishes one call in a live
process and does not claim restart-safe exactly-once execution.

### Failing-first broker replay reproducer

```text
'/mnt/c/Program Files/Go/bin/go.exe' test ./internal/runtime -run '^TestGraphConformanceBrokerReplayRisk$' -count=1 -v
```

Exit `1`. The named test executed the production `ExecuteBrokerTool` twice
with the same run identity and arguments, representing restart replay after
loss between the broker effect and durable completion. Observable:
`same broker operation executed 2 times across replay, want exactly one`.
No broker operation/idempotency key exists to return the first result.

### Required G0 surface

```text
'/mnt/c/Program Files/Go/bin/go.exe' test ./internal/runtime -run 'TestOrchestrationNative|GraphConformance' -count=1 -v
```

Exit `1`; exact output is retained in
`task-5-issue39-native-orchestration.log`. Both
`TestOrchestrationNative` and `TestGraphConformanceBrokerReplayRisk` ran. The
first failed at the unchanged typed sentinel; the second observed two effects
for the replayed broker operation. Therefore the six-condition G0 surface did
not pass, and no GO is claimed.

## UltraQA probes

- `cancel_resume`: FAIL for G0 as a whole. Pinned Eino supports targeted
  resume, but no Service-owned Workflow path exists and the named surface
  remains RED.
- `repeated_interruptions`: FAIL for G0 as a whole for the same reason; no
  Task 5 production resume boundary was added.
- `stale_state`: PASS as a fail-closed capability already present.
  `VersionedCheckpointStore` binds engine version and prompt identity, but
  those checks do not close the external-effect crash window.
- `dirty_worktree`: PASS. The three exact inherited untracked files retained
  their hashes and were excluded from explicit staging.
- `hung_or_long_commands`: PASS. Go invocations used `timeout 120s`; no
  command exceeded the bound.
- `flaky_tests`: not claimed as a pass for the absent implementation. Race
  repetition of the replay probe ran three times with `-race -count=3`; all
  three named executions observed two effects and no race diagnostic.
- `misleading_success_output`: PASS. Evidence requires named test lines and
  exit codes. The required surface exits non-zero; the separate unknown-effect
  test is not counted as G0 success.
- `malformed_input`: N/A; no runtime/input schema changed.
- `prompt_injection`: N/A; no new untrusted text reaches a child or authority
  boundary.

## Product gates

```text
'/mnt/c/Program Files/Go/bin/go.exe' vet ./internal/runtime
```

Exit `0`, no diagnostics.

```text
'/mnt/c/Program Files/Go/bin/go.exe' test -race ./internal/runtime -run '^TestGraphConformanceBrokerReplayRisk$' -count=3 -v
```

Exit `1` as required by the reproducer. The named test ran three times; each
reported `executed 2 times across replay, want exactly one`. No race diagnostic
appeared.

`just ci` was invoked through the repository's Windows `just.exe`. The first
attempt failed before a recipe because the WSL worktree `.git` indirection was
not a Windows path. The corrected invocation exported translated `GIT_DIR` and
`GIT_WORK_TREE` and placed Windows Go/Git/PowerShell on `PATH`. It passed the
format recipe, then stopped at the repository vet recipe with exit `1`:

```text
ui\embed.go:12:12: pattern all:dist: no matching files found
error: Recipe `vet` failed on line 28 with exit code 1
```

The preceding UI recipe could not run `pnpm` in that Windows environment, so
it did not create `ui/dist`. A direct local `pnpm install --frozen-lockfile`
attempt then failed fetching `https://registry.npmjs.org/pnpm/latest` with
`UND_ERR_CONNECT_TIMEOUT`. This environment failure occurred before Go tests;
it is not represented as either a G0 pass or as the expected intentional RED.
The focused runtime compiler/vet command above did pass.

`git diff --check` exited `0`. This is a stopped NO-GO iteration, not a
partially verified implementation.

## Cleanup receipt

No server, browser, container, database service, or background Go process was
started. All Go tests were foreground processes bounded by `timeout 120s`.
The completion scan found no `go.exe test` or `runtime.test.exe` process.
Temporary captures under `/tmp/task5-*.log` are outside the repository and are
not staged. Cleanup of ignored `ui/node_modules` is **blocked by provenance**:
its contents may include cache state that predated the failed package-manager
attempt, so it was preserved rather than destructively removed. A scoped
removal attempt was stopped once provenance could not be proven; the final
process scan found no removal, Go test, just, pnpm, Vite, server, browser, or
container process. No production journal under `data/` was read or written.

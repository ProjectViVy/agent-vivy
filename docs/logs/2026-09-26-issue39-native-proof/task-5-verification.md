# Task 5 verification

## 1. Goal and files

At exact baseline `9f26f091f851bfdbe3b878d2cf8480320192ea53`, correct only
the Task 5 test wording and evidence claims. The changed files are
`internal/runtime/toolbroker_test.go` and this Task 5 acceptance, JSON, log,
summary, and verification evidence. No production orchestration, fallback,
scheduler, second runtime, or Tasks 6–14 work was added.

## 2. Baseline / RED

Before the wording correction, the required focused command was run:

```text
'/mnt/c/Program Files/Go/bin/go.exe' test ./internal/runtime -run '^TestGraphConformanceBrokerReplayRisk$' -count=1 -v
```

It exited `1` and reported two calls. Source inspection confirms that the test
directly calls `ExecuteBrokerTool` twice with the same run and input in one
process and increments an in-memory fixture counter. That is an observation,
not Service/Engine/store recreation, injected crash/restart evidence, or a
real external side effect.

## 3. Constraints

G0 safety is unchanged and remains terminal NO-GO. The probe neither proves
real external-effect exactly-once behavior nor supplies a stable operation ID.
The required Service/Eino/checkpoint/fresh-store crash/recovery test is
unproven. No user-visible delivery is claimed; Tasks 6–14 remain **NOT RUN**.

## 4. Automated verification

Post-change commands and their exact outputs are recorded in
`task-5-issue39-native-orchestration.log` and this document. The focused and
race commands intentionally exit `1`: that non-zero result is the named
NO-GO probe, not product success. The unknown-effect control must exit `0`;
`git diff --check` must exit `0`.

- Focused command: exit `1`; the captured artifact reports
  `same-process direct broker calls re-invoked in-memory fixture 2 times;
  crash/restart safety remains unproven`.
- `'/mnt/c/Program Files/Go/bin/go.exe' test -race ./internal/runtime -run
  '^TestGraphConformanceBrokerReplayRisk$' -count=3 -v`: exit `1`; all three
  executions reported that same two-call fixture observation and no race
  diagnostic appeared.
- `'/mnt/c/Program Files/Go/bin/go.exe' test ./internal/runtime -run
  '^TestToolFailureUnknownEffectsCountOnce$' -count=1 -v`: exit `0`; the named
  control passed.
- `git diff --check`: exit `0`.

## 5. Manual-QA channel

Run:

```text
'/mnt/c/Program Files/Go/bin/go.exe' test ./internal/runtime -run '^TestGraphConformanceBrokerReplayRisk$' -count=1 -v
```

PASS for this evidence correction means the named test runs and reports that
the in-memory fixture was called twice, while the test comment and Task 5 docs
explicitly state it is same-process only and G0 crash/restart remains
unproven. Its non-zero exit is not product success.

## 6. UltraQA

- `stale_state`: baseline SHA is pinned before the correction; final status
  retains the same `9f26f091f851bfdbe3b878d2cf8480320192ea53` checkout until
  the focused commit. The pre-existing untracked hashes are retained:
  `e0007b5d41253ab67753a0ec8528cb0ad157f14b80028975094becc2e15e1813`,
  `f528c9f59fb3e88c7eaca820213984d78bac733ca4ddd8a713cf86ddc91fcb4a`,
  `30e02a72a9ae07f14c0f7869f565ef87caceca7fcebfc5c745d81f072227b8d1`,
  `24cb77537a7d0b09ece24635167bbf141aa1b613f976975b7e30f60867006882`,
  and `fe0d3e4578a811d941d00e40cc0544136e093232953d9a2f224fcb081e96a92a`.
- `dirty_worktree`: preserve all pre-existing untracked evidence and stage
  explicit tracked paths only.
- `misleading_success_output`: label the expected non-zero focused and race
  outputs as NO-GO, never PASS.
- `flaky_tests`: run the race probe three times; each must show two fixture
  calls and no race diagnostic.
- `malformed_input`: N/A; no parser changed.
- `prompt_injection`: N/A; no prompt boundary changed.
- `cancel_resume`: N/A; no resumable runtime was added.
- `hung_or_long_commands`: foreground tests are bounded by `timeout 120s`.
- `repeated_interruptions`: N/A; no long-running flow was added.

## 7. Artifact and cleanup

The named focused output is captured in
`task-5-issue39-native-orchestration.log`. Before commit, inspect
branch/upstream/status, use only the verified existing human Git identity, and
stage explicit tracked paths. Do not push or create a PR. No server, browser,
container, journal, or generated QA resource is used; `ui/node_modules` stays
absent. The commands are foreground and bounded by `timeout 120s`; the final
scan finds no Go test, runtime test, pnpm, Vite, just, or Vivy process. The
focused and control commands use `-count=1`, the failing race probe cannot be
cached as a passing test result, and no task-specific cache or QA resource was
created. Preserve the listed untracked state.

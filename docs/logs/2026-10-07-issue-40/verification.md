# Issue #40 verification — 2026-10-07

Evidence below is from the final working tree, not historical ND-D1 records.

## Turn-policy follow-up — PASS

The owner-approved follow-up removed the parent and child eight-turn policy
rather than raising it to 10,000. Fresh parent and `ChildView` regressions each
completed 24 tool calls, exceeding both the retired eight-turn policy and
Eino's implicit default of twenty. Separate regressions prove cancellation at
the twenty-fifth call and shared model-budget termination after 22 completed
calls, with one durable terminal in both cases.

Pinned Eino exposes no supported resume option for rewriting an old opaque
checkpoint's iteration counter. The versioned checkpoint envelope now carries
runtime compatibility version 2; unversioned/older checkpoints fail closed
before Eino can resume their retained eight- or twenty-turn state. New
checkpoints still resume normally. This is an explicit new-Run requirement for
pending pre-follow-up work, not automatic effect replay or counter mutation.

The isolated split-pair product probe used no `runtime.max_tool_turns` setting.
Two Runs issued 24 model-directed calls each: all successful writes settled in
the first Run; 24 ordinary missing-file failures settled in the second; both
completed. The success Run emitted no Nudge and the failure Run emitted exactly
repeat counts 3 and 5. Authorized synthetic task text remained unchanged in
requested arguments and the durable final response. Shared production data was
not read or written.

```json
{"scenario":"poll","status":"completed","finished":24,"nudges":0,"approvals":24,"faithful_final":true,"core_turn_setting":"absent"}
{"scenario":"error","status":"completed","finished":24,"nudges":2,"approvals":0,"faithful_final":true,"core_turn_setting":"absent"}
```

Fresh verification included focused runtime race tests, config/app/runtime,
standalone EXP vet/tests, SDK conformance, static production-symbol/dependency
audits and `GIT_CONFIG_GLOBAL=/dev/null just ci`. The canonical gate passed;
the final compatibility-envelope edit was followed by the same focused and
canonical gates recorded below.

Final post-envelope evidence:

- `go test ./internal/config ./internal/app ./internal/runtime ./sdk/internal/conformance -count=1 -timeout 15m`: PASS (runtime 58.835s; conformance 54.463s).
- `go test -race ./internal/runtime -run 'TestServiceWithoutTurnPolicyExceedsFormerLimits|TestServiceLongRunRemainsCancellable|TestServiceModelBudgetStillStopsLongRuns|TestNudgeState|TestVersionedCheckpointStore|TestServiceRecoverResumableApproval' -count=1 -timeout 5m`: PASS (16.706s).
- `go test ./... -count=1 && go vet ./...` in `plugins/exp/turn-limit`: PASS.
- `GIT_CONFIG_GLOBAL=/dev/null just ci`: PASS, exit 0, including UI lint/type/build, core vet/tests, headless compile smoke and all nested plugin vet/tests.
- The split pair and the 24-call probe were restarted/repeated after the final checkpoint-envelope edit; both JSON outcomes above were reproduced, exit 0.
- Final review closed the checkpoint-compatibility Important on the described fixed-epoch delta. This was logical review, not an independent rerun of the final implementation; the parent-owned gates above supply execution evidence.

## Environment

Linux amd64; Go 1.26.8, Node 24.9.0, pnpm 11.19.0, just 1.43.1, PowerShell 7.5.4. The VM initially lacked these tools despite its configured blueprint; restored the pinned tools without changing product code.

Laputa is at lock commit `ff3936f44ff8cf08c12af2cf698c194cfe474fd3`. The stored origin is canonical GitHub, but the VM's global `url.insteadOf` makes `git remote get-url` report the Git-manager proxy and the initial `just ci` fail before tests. Running with `GIT_CONFIG_GLOBAL=/dev/null` disables that environment-only URL rewrite for one process tree. No Git config, source pin, bootstrap validator, recipe, security policy, or merge gate was changed or skipped. This is the same Linux-VM workaround recorded in the preceding H1 delivery.

## Fresh focused verification — PASS

```sh
go test ./internal/logging ./internal/runtime ./internal/observerhost \
  ./internal/contexthost ./internal/skillhost ./internal/actionhost \
  ./internal/statushost ./internal/modulehost ./internal/storage/... \
  ./internal/rpc ./internal/tools -count=1 -timeout 15m
```

Executed with a disposable local PostgreSQL database on port 55440 through `VIVY_POSTGRES_TEST_DSN`; Postgres tests ran rather than silently skipping. Runtime passed in 71.082s, Action Host in 2.050s, Postgres in 15.484s, SQLite in 15.041s, and RPC in 23.956s; remaining listed packages passed.

Regressions include synthetic email/password/key-looking strings and literal `[REDACTED]`, normal/enhanced tool/model/replay paths, repeated failures/refusals, Generate/Stream reminder fit/skip/retry behavior, and Journal-failure retry safety. The last Action audit regression was observed red (`authorized diagnostic changed`) before the precise actual-Secret containment fix, then green.

The full existing runtime tests continue to cover durable ordering, mismatched/duplicate call IDs, cancellation, interrupted/resumed legs, policy/refusal, schema validation, approval argument binding, workspace/symlink containment, and output/context budgets.

## EXP Modules — PASS

Each nested Go module ran `go test ./... -count=1` and `go vet ./...` from its own directory; the root module intentionally does not own these packages.

```text
ok agent-vivy/plugins/exp/redaction       0.003s
ok agent-vivy/plugins/exp/argument-guard  0.010s
ok agent-vivy/plugins/exp/loop-detection  0.005s
```

```sh
go run ./sdk verify plugins/exp/redaction
go run ./sdk verify plugins/exp/argument-guard
```

```text
ok vivy/exp-redaction
ok vivy/exp-argument-guard
```

## Executed conformance — PASS

```sh
go test ./sdk/internal/conformance -count=1 -timeout 15m
```

Result: `ok agent-vivy/sdk/internal/conformance 67.586s`.
The checked-in result index was refreshed to the final canonical internal-tree digest `241de3e4a3fb21b7bb2a3bd6e9f35f26cc29fe4036c3a93113c6f8b393b23477` and verified against executed suites. Required actual-Secret/grant checks remain; a conformance check named `redaction` is not a default string redactor.

## Full canonical gate — PASS

```sh
GIT_CONFIG_GLOBAL=/dev/null just ci
```

The final command exited zero after the last SDK staging fix. It includes the real `ensure-laputa`, bootstrap tests, Go format/vet/tests, UI install/typecheck/tests/build, i18n checks, headless compile, and recursive plugin CI. UI reported 77 files / 598 tests passed; root `sdk/internal` and executed conformance passed.

`TestPackAndInspectEveryShippedRecipe` packs default, minimal, headless, vivy-code, SCX, lite, DIVA, masks-selected, masks-omitted, and masks-backend-only. It checks each sealed Manifest, generated Assembly, and compiled Go symbol table for EXP exclusion and removed core redactor/guard/loop APIs. The matrix passed in the full gate.

## Product entry — PASS (RPC smoke, not owner UI E2E)

Started the real `just run` backend and `cd ui && pnpm dev` frontend; opened `http://127.0.0.1:3015` in Chrome. Used `VIVY_USER_HOME=/home/ubuntu/issue40-smoke`, a minimal private `VIVY_CONFIG`, a local OpenAI-compatible SSE provider on port 9914, and a synthetic non-secret API key. No production `data/vivy.db`, `data/demo/`, or `data/workspaces/` was read or written.

The fixture set `runtime.max_tool_turns: 16` to accommodate nine deliberately repeated calls plus a final model response. The shipped default of 8 was not changed: the initial fixture reached that genuine independent iteration limit, not a repetition stop. Another initial fixture correctly failed for an unselected `echo_info` tool; the final probe uses selected built-in tools, preserving ToolHost authority.

Through Vite's authenticated `/rpc` WebSocket proxy the probe initialized an isolated Persona, started real sessions/turns, approved synthetic file-write proposals through the real `approval/respond` API, inspected durable `run/log` events, and read `session/messages`:

```json
{"scenario":"poll","status":"completed","finished":9,"nudges":0,"faithful_final":true}
{"scenario":"error","status":"completed","finished":9,"nudges":2,"faithful_final":true}
```

The first scenario performs eight identical successful writes and a ninth changed write. Requested arguments and the durable final answer retain the synthetic email/password/key-like text, literal marker, traversal-like text, punctuation, `powershell`, and `cmd.exe`. The second deliberately reads a missing file nine times, verifies durable failure results, and asserts reminder repeat counts exactly `[3,5]`. Both complete beyond the former sixth-repeat limit. The probe makes no automatic tool retry; each call is a new distinct model-issued ID. The final working tree repeated both scenarios with the same results.

A separately packed explicit-EXP Generation also reached the real ToolHost: policy allowed `read_file`, `experimental_argument_guard` denied the opted-in traversal argument, and the Run failed with zero tool finishes. That expected failure proves explicit legacy guard selection, not default behavior.

## Static audit — PASS

- `git diff --check` passed.
- `gofmt -l` over the canonical tracked plus non-ignored untracked Go source set returned no files.
- `rg 'ValidateArgsSafety|RedactSensitive|newRedactingHandler|errLoopDetected' internal` returned no matches.
- `go list -deps ./cmd/vivy ./cmd/vivy-code ./cmd/vivy-shared` contained no EXP package imports.
- Real policy/Secret/tool constraints remain; no new full-payload or resolved-credential logging was added.

## Independent review — PASS

Independent read-only review: https://app.devin.ai/sessions/1906351751e24979b443624efc3e891d . Review closed four Important findings after reproduced fixes: Journal number precision, generated pre-tool runtime wiring, command-local bash literals, and SDK self-staging source hashes. Final disposition: no substantiated Critical/Important findings remain.

## Separate baseline finding — not fixed here

An untouched baseline repeated `TestPlanGoalIntegratedRoundLimitBlocksDurably` ten times and failed three times on restart with `memory.sqlite3: no such table: schema_meta`. The verified condition is an existing incomplete first store; cancellation interrupting initial schema creation is the current source-supported hypothesis, not yet fault-injection proof. This patch does not modify BML/Memory. The follow-up is recorded in `docs/TODO.md`.

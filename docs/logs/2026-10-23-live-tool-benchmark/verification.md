# Verification

## Target and method

- Native Windows executable source: `397b6f17`.
- Checkout/documentation head before this delivery: `681c781a`.
- Recipe: `recipes/minimal-chat.vivy.yml`, eleven Modules.
- Generation: `1c6fd4a5bf0a9ebfb3f4a0be06465186`.
- Process `43788` was checked through `Get-Process`; its executable is the
  isolated `.workspace/d14-manual-397b6f17/minimal-chat/vivy.exe`.
- Existing disposable D14 backend/home on `127.0.0.1:8787`; healthz returned
  HTTP 200 and `status=ok`. No restart, production Journal access, or provider
  credential read-back was performed.
- Node `v24.14.1`; `node --check benchmark.mjs` and RPC-only `--preflight` pass.
  The preflight reports only `list_dir` and `read_file` active.
- Two sequential rounds, six tasks per round, ten fresh titled sessions and
  two follow-up turns. No retry of a failed task or warm-up sample is excluded.

The external client uses `/rpc/bootstrap`, `initialize`, `session/create`,
`turn/start`, `run/subscribe`, `run/log`, `session/messages`, and unsubscribe.
The bootstrap token remains in memory. Only numeric metrics, route and tool
names, run/session identities, and synthetic answers/expected values are
saved. Raw event payloads, model requests, settings, and credentials are not
written to the result artifact.

Eino capability check: this adds no model loop or runtime capability. The
existing `Service.Run` path uses pinned Eino v0.9.21
`adk.NewTypedChatModelAgent` and `adk.NewTypedRunner`; custom-provider model
construction remains behind EinoExt `agenticopenai` v0.2.4. The client only
observes that path through public RPC.

## Measurement definitions

End-to-end time uses the client's monotonic clock from immediately before
`turn/start` to receipt of the terminal event. First-text time uses the first
non-empty `model.delta`, including all preceding tool/model work; it is not
provider TTFT. Completion-token counts include all model calls in the run,
not just final-answer text. Usage is summed once from each distinct
`model.call.finished` closure; absent usage stays unknown. Upstream gateway
usage is accepted as provider-reported evidence, not independently tokenized.

The scorer checks exact synthetic answers, required successful `read_file`
calls, and one completed terminal event. The bug-expression check tolerates
whitespace, a `return` prefix, and a trailing semicolon. Live notifications
and replay are deduplicated by run/sequence identity. These are twelve small
samples over a local gateway: network/cache/load conditions are uncontrolled,
and no population-level latency or model-quality claim is made.

Observed result: 12/12 pass; 39 model requests and 39 completed closures;
29 requested and finished tools; twelve single completed terminal events.
There are two expected missing-file tool errors, zero provider retry events,
zero approval requests and zero timeouts. Aggregate usage is 103,309 input,
861 output and 104,170 total tokens. Median elapsed is 10.957 seconds.

The initial executed client mislabeled the latest capture timestamp as
`started_at`. The saved projection renames that value `captured_at` and does
not claim a wall-clock start time. The checked-in client now records a real
start plus update timestamp, uses the existing `run/cancel` method for its
timeout guard, and tolerates cleanup after a failed connection. Those branches
did not affect this completed, zero-timeout run. Final syntax/preflight and
offline result consistency checks cover the recorded client/artifact without
spending another set of live calls.

## Commands and outcomes

| Command/check | Outcome |
| --- | --- |
| `node docs/logs/2026-10-23-live-tool-benchmark/benchmark.mjs` with `BENCH_OUTPUT=.workspace/benchmarks/2026-10-23-live-tools` | Exit 0; twelve live samples, all pass. |
| `node --check docs/logs/2026-10-23-live-tool-benchmark/benchmark.mjs` | Pass, including the final reporting/cleanup corrections. |
| `node docs/logs/2026-10-23-live-tool-benchmark/benchmark.mjs --preflight` | Pass before and after the run; same two active readonly tools. No model call. |
| Offline Node assertions over `results.json` | Pass: twelve unique runs, ten sessions, independent JSON-answer comparison, closure/usage/tool counts, terminal counts, expected error counts and aggregate totals. |
| Credential-shaped-literal scan of this directory | No matches for key, bearer-token or query-token value patterns. |
| `git diff --cached --check` | Pass. |
| Guarded scratch cleanup | Removed only the resolved `.workspace/benchmarks/2026-10-23-live-tools` directory after verifying the durable projection. |
| Final `GET /healthz` | HTTP 200, `status=ok`. |

The readonly real calls use the existing local gateway at
`http://127.0.0.1:8045/v1`. No network request is made to discover models or
refresh provider settings. All observed call closures identify the same
configured raw model ID, `gemini-3.8-flash-tiered`.

## Historical search and unrun gates

Read-only searches covered the current and former Windows repositories,
ignored workspace/worktree filenames, relevant script contents, committed
filename/commit history, and known WSL `/home/mastwet/work` and `/tmp` file
names. No original real-model/task benchmark harness or numeric results were
found. The older `docs/dev/real-provider-smoke.md` is an acceptance checklist,
not a historical performance baseline. This delivery establishes a new
reproducible sample and makes no before/after performance claim.

No kernel, UI, migration, dependency, or runtime source changes are made.
Linux kernel/race, PostgreSQL, Docker, full-product CI and browser acceptance
are not rerun for this external benchmark/report delivery. The existing
runtime remains available for the owner's manual use. Stable-root integration,
push, release, and D14 closure are outside this delivery.

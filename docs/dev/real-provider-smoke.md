# Real Provider Smoke Report (M4 acceptance)

Date: 2026-08-07 · Commits: `c50a27f`, `2ecf879`, `200357b`, this doc.

## Gateway under test

An OpenAI-compatible gateway was used via the `VIVY_API_BASE` override
(`c50a27f`): base URL on `api.stepfun.com`, model `step-3.7-flash`,
credential injected at process start from an archived keys file into the
`OPENAI_API_KEY` environment variable only. Per D-010 and the E3 audit,
the key value never appears in config files, the journal, logs, commits,
or this report; `TestSecretsNeverReachStorage` and the source-level
guards remain green on every gate run below.

## Automated env-gated suite (`2ecf879`)

`internal/app/realsmoke_test.go` runs only when `VIVY_REAL_SMOKE=1` and
`OPENAI_API_KEY` are set; otherwise it skips, keeping CI and the default
gate offline. With the real gateway it PASSED (`-race -count=1`):

- AS-1: session → turn → JSON-RPC notifications consumed to `run.completed` with
  gapless seq, then the messages endpoint returned the full user +
  assistant round trip (28 events, ~38s wall time).
- AS-7: reconnect with `after_seq=1` replayed exactly the tail,
  frame-by-frame seq/type parity with the live stream.
- AS-5: cancel issued right after the first `model.delta` of a long
  stream → exactly one terminal event, `run.cancelled`.

## Manual walkthrough (real provider, JSON-RPC + browser)

| AS | Scenario | Result |
|---|---|---|
| AS-1 | Streaming chat completes over the real model | PASS (API and browser UI: reply streamed to completion, `run completed` badge) |
| AS-2 | Induce `echo_info` ("call echo_info with text …") | PASS after fix: `tool.requested` carried `{"text":"hello-real-smoke"}`, `tool.started/finished` fired without approval, run completed |
| AS-3 | Induce `write_note`, deny the approval | PASS: `tool.approval_required` surfaced with the exact args, deny → tool result recorded the denial wording, run completed without executing the effect |
| AS-4 | Induce `write_note`, approve | PASS: approve → `tool.finished` with `note saved (1 total)`, run completed |
| AS-5 | Cancel mid-stream | PASS (automated suite; see above) |
| AS-6 | Hard-kill the process mid-run, restart | PASS: the interrupted run settled to `run.failed` with `cause_category: internal_error` and the restart-recovery wording; sessions and messages survived |
| AS-7 | Refresh the UI mid/post-run | PASS (browser walkthrough: history intact after reload; automated reconnect parity above) |
| AS-8 | Mock exact-sequence determinism | Covered by the existing engine tests (mock provider unchanged) |
| AS-9 | Secrets never reach storage | Covered by the E3 guards (`9fa4d15`), re-run green on every gate of this batch |

UI walkthrough notes: page rendered with session list and chat pane,
message sent → streamed reply → refresh kept full history; no console
errors. The approval dialog was exercised through the JSON-RPC decision
method; the same notification path is covered by the scripted-model
integration tests.

## Defect found and fixed: tool parameter schemas (`200357b`)

The first AS-2 walkthrough attempts failed: the gateway model emitted
hallucinated argument names (`example_parameter_1`) or wrong-shaped
arguments. Root cause was Vivy-side, not model-side: `toolAdapter.Info`
published only `Name`/`Desc`, so real gateways received no JSON schema
and guessed. Fix: `domain.ToolSpec` now declares `Params`
(`ToolParam{Desc, Required}`), both builtin tools declare their single
required string argument, and the adapter converts them via
`schema.NewParamsOneOfByParams` into the tool info. Adapter tests assert
the schema reaches the JSON Schema form (type `string`, required). After
the fix the gateway produced correct `text`/`content` arguments on every
attempt.

## Conclusion

M4's core acceptance (real provider end to end) holds: streaming,
persistence, reconnect replay, cancellation, approval gate in both
directions, crash recovery, and the UI all behave per spec against a live
OpenAI-compatible gateway. One real defect was found and fixed during the
walkthrough. Remaining board items are unchanged: E4 (graceful-shutdown
hardening), D4 (Playwright/import-lint), B5 (conformance harness).

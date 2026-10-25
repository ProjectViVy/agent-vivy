# Verification

## Environment

- Host: macOS arm64.
- Ambient Go 1.27.1 cannot build the pinned dependency set (`google.golang.org/grpc` fails with `undefined: http2.TrailerPrefix`); every command below ran with `GOTOOLCHAIN=go1.26.4`, matching the repository pin.
- No live provider call. All provider-facing tests use `httptest`.

## Required suites

```
GOTOOLCHAIN=go1.26.4 go test ./internal/runtime/... ./internal/provider/... ./internal/app/... -count=1
```

Result: `ok` for `internal/runtime` (69.9s), `internal/runtime/compaction`
(0.1s), `internal/provider` (0.5s), `internal/app` (19.3s),
`internal/app/settings` (0.8s).

```
GOTOOLCHAIN=go1.26.4 go vet ./internal/runtime/... ./internal/provider/...
```

Result: no output (clean).

## Focused tests added or updated

- `internal/runtime/request_shape_test.go`: every tool id in a multi-call /
  multi-result turn, reasoning bytes, adjacent system rows and preamble bytes,
  tool declarations, the 256-block cap with complete counts, identifier
  clamping.
- `internal/runtime/request_invariants_test.go`: each refusal rule, legitimate
  multi-call/multi-result and id-reuse turns, structural fingerprint identity,
  `hasToolTurns`, and `Begin` refusal (request + finished pair, v5 version,
  `ModelRequestInvalid`, zero provider calls, zero model-call charge).
- `internal/runtime/tool_result_order_test.go`: bare `(2013)` does not retry;
  a semantic phrase plus a tool turn retries once with an identical
  fingerprint; missing tool turns and invalid inputs decline; the retry and
  overflow paths share the budget.
- `internal/provider/wire_golden_test.go`: exact HTTP JSON for system, user,
  assistant text, tool calls/results, declarations and streaming; the summary
  matches the captured request and never contains a nonce.
- `internal/app/assembly_audit_test.go`: request rows require v5 with no
  `sha256` and no nonce; a failing provider produces one attempt, a failed
  finish with a `wire` summary (status 400, roles, content forms, body bytes)
  and no nonce anywhere in the journal payload.
- `internal/runtime/payload_schema_test.go` and
  `internal/runtime/service_test.go`: v5 version expectations, v5 fixtures, and
  invalid wire/block/count shapes.

## Failures found and fixed during verification

The first full run failed three existing tests because the initial invariant
rejected duplicate call ids: `TestModelWorkIdentityUsesEinoCallIDForDistinctCallsAndRetries`
(two mutations instead of three), `TestServiceBashToolBackgroundEndToEnd`
(refused at message 8), and `TestAgentChildTurnLimitBecomesToolResult` (the
child diagnostic replaced by the refusal). The validator now allows an id to
be reused only after its earlier occurrence is answered, while a still-pending
duplicate, an orphaned result, a separated result, and a doubly answered result
remain refused. All three pass in the final run.

## Not run

- `just wiring-ci`, `just kernel-ci`, `just d02-ci`: Linux/WSL-only gates and
  not runnable on this macOS host. They remain required on the candidate.
- Browser/split-pair smoke: no UI behavior changed.
- Owner live check at `http://127.0.0.1:3015` with a real MiniMax endpoint:
  costs tokens and is intentionally manual. When run, read only the dev home
  `.workspace/dev/home/vivy.db`; never `data/vivy.db`, `data/demo/`, or
  `data/workspaces/`.

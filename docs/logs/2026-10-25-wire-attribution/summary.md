# Model-call wire attribution and honest retry

## What changed

The failed turn `run_d006394b34bdddf0` (MiniMax-M3.1-Flash, HTTP 400
`invalid params, 400 (2013)`) exposed a handoff with no owner: execution,
history, audit and the HTTP body each projected the same call differently, and
a generic provider code was treated as a retryable tool-order error. This
delivery makes the handoff verifiable without merging the pipelines.

- **v5 `model.request`** (`internal/runtime/request_shape.go`,
  `payloads.go`, `mapper.go`, `schemas/events/payloads/model.request.json`).
  One traversal publishes the wire-facing role per message, the ordered block
  kinds with byte counts, every tool call/result id, complete counts, bound
  tool declarations (names, ordered parameter names, scalar type, required),
  and a 256-block shape cap. Content-free; no digest. v1–v4 rows still decode.
- **Pre-send invariants** (`internal/runtime/request_invariants.go`). Only
  provably unsendable shapes are refused: empty user/assistant turns, tool
  blocks on system turns, results on assistant turns, calls without id/name,
  orphan or doubly answered results, results mixed with non-result user blocks,
  and results separated from their call by a user message. Duplicate call ids
  are allowed: the engine reuses ids across retries, and every call is tracked
  as its own pending occurrence. A multi-message fingerprint
  (`role:blocks:kinds{call:id}{result:id}`) proves replay identity without
  content.
- **Begin refusal** (`internal/runtime/service.go`,
  `runModelCallObserver.Begin`). A violation persists the v5 request and a
  paired `model.call.finished{status:"failed", error.name:"ModelRequestInvalid"}`,
  charges no model call, and never invokes the provider. The OBS-02 one
  request ↔ one finished lifecycle holds for refusals too.
- **Honest retry**. `isToolResultOrderFailure` now requires HTTP 400 plus a
  semantic order phrase; a bare `invalid params, 400 (2013)` never retries.
  A grant requires a real tool turn, an input that passes the invariants, and a
  content-free fingerprint; at most one retry per run. The reason records the
  fingerprint, and a second rejection logs `identical=true|false` before
  declining.
- **Provider wire recorder** (`internal/provider/wire.go`, `openai.go`). A
  `RoundTripper` under `agenticopenai.ChatConfig.HTTPClient` buffers and
  replays the request body byte-for-byte, and on a non-2xx response reports a
  bounded, content-free `WireSummary` through a per-call context sink. The
  runtime journals it as the optional `wire` object in `model.call.finished`
  only for failed calls. Capture cannot fail or alter a request; bodies above
  8 MiB are sent in full but summarized only from the inspected prefix.
- **Golden wire tests** (`internal/provider/wire_golden_test.go`) pin the ACL
  mapping through the real provider constructor and an `httptest` server:
  system string, user/assistant content arrays, tool-call-only assistant
  without `content`, `role:"tool"` with `tool_call_id`, function declarations,
  streaming options, and a nonce-free summary.

## Scope

Runtime, provider, event schemas, and the tests that own them. No UI, RPC,
storage-schema, or configuration change. No live provider call is made by any
test.

## Deliberately not done

The shared turn-record builder (execution/history/audit), reasoning-block
projection into the feed, and an operator-gated full-body dump remain deferred
in `docs/TODO.md` §0.1. The assistant `content` array normalization is
conditional on captured wire evidence; the first turn with two system messages
succeeded on this endpoint, so system coalescing is not proposed.

## Plan deviation

The approved plan rejected any duplicate call id. The full suites proved that
existing engine retry and child paths legitimately reuse an id across turns
(`TestModelWorkIdentityUsesEinoCallIDForDistinctCallsAndRetries`,
`TestServiceBashToolBackgroundEndToEnd`,
`TestAgentChildTurnLimitBecomesToolResult`). Keeping that rule would turn a
sendable feed into a run failure, which the plan's own risk section forbids.
The validator now allows an id to be reused only after its earlier occurrence
has been answered; an id that is still pending, or a result that is orphaned,
separated or doubly answered, is still refused.

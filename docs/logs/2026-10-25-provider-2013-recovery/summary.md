# Recover from a bare provider `2013` and normalize the outbound wire

## What changed

A MiniMax-M3.1-Flash child run failed on its **first** model call with a bare
`invalid params, 400 (2013)`. The body was structurally canonical (3 messages
`[system, user, user]`, 17 tool declarations, 12,463 bytes, no assistant or
tool row), our pre-send invariants passed, and a sibling child in the same
minute with the same tools and contract rows succeeded; 7 of 76 child first
calls failed this way over two days. `d4753fa3` had deliberately narrowed the
retry classifier so that a bare `2013` is never retried, so the run failed
instead of costing one extra request.

This delivery does two things: it retries that one case when the input proves
no history-shape cause, and it makes every replayed turn canonical on the wire
in the shapes MiniMax's own harness sends.

- **Outbound payload normalizer** (`internal/provider/outbound.go`, new).
  `newOutboundNormalizedModel` wraps the constructed `agenticopenai` model and
  appends `acl/openai.WithRequestPayloadModifier(normalizeOutboundPayload)` last
  on every `Generate`/`Stream`, so the modifier wins over caller options
  (`model.GetImplSpecificOptions` applies options in order). Inside the body:
  - text-only `content` arrays on `user` and `assistant` rows become one plain
    string (joined with `"\n"`); arrays carrying `image_url`, `input_audio`,
    `video_url` or an unknown part are untouched;
  - `tool_calls[].function.arguments` is compacted when it is valid JSON
    (key order and numeric lexemes preserved; never round-tripped through
    `any`), replaced with `{}` when empty, and replaced with `{}` plus one
    content-free `slog.Warn` (call id and byte length only) when invalid —
    the call and its result still pair;
  - every other top-level field and message field survives verbatim
    (`stream_options`, vendor extras, `reasoning_content`, `tool_call_id`);
  - any decode/encode failure returns the original body unchanged. The
    modifier runs above `wireHTTPClient`, so `WireSummary.ContentForms`
    describes the bytes actually sent (`internal/provider/wire.go` header
    updated).
- **Bare `2013` first-request retry** (`internal/runtime/service.go`).
  `isBareProvider2013` accepts only HTTP 400 whose text carries `2013` as a
  token and none of the deterministic phrases (tool-order, invalid function
  arguments JSON, context-window family). `isUnambiguousFirstRequest` requires
  every message to be a system or user row, at least one user row, no tool
  call or tool result, and a passing `validateModelInput`.
  `provider2013RetryDecision` grants at most one retry per run with
  `RejectReason: "provider_2013_first_request"`; the identical bytes are
  replayed because a 400 produced no output.
- **One retry budget** (`internal/runtime/service.go`). `toolOrderRetryState` /
  `toolOrderRetries` became `modelRetryState` / `modelRetries`, shared by
  `grantModelRetry`, because overflow, tool-order and `2013` retries consume the
  same `modelMaxRetries` budget and the same per-run cleanup entry.
- **Regression coverage**. New `internal/provider/outbound_test.go` and
  `internal/runtime/provider_2013_retry_test.go`; `wire_golden_test.go` now
  expects string content for user and assistant rows, pins that invalid UTF-8
  reaches the wire as U+FFFD, and `context_test.go` pins that an unanswered
  assistant tool call never reaches the projected context.

## Scope

`internal/provider`, `internal/runtime`, `go.mod` (one dependency moved from
indirect to direct, version and `go.sum` unchanged), and the records that own
them. No UI, RPC, storage-schema or configuration change. No test reaches a
live provider.

## Deliberately not done

- **No synthesized tool results.** `pairToolTurns` still drops an unanswered
  call and an orphan result; fabricating a result would put model-visible text
  that never happened into history. Covered by
  `TestBuildRunContextNeverSendsDanglingToolCall` instead.
- **Two system rows stay.** Input is `[system(instruction), system(preamble),
  …]` by design, the endpoint accepts it, and first turns with two system rows
  succeeded. Merging would erase the static/per-run split.
- **No surrogate sanitizer.** Request bodies are produced by `encoding/json`
  from Go strings, which replaces invalid UTF-8 with U+FFFD and cannot emit a
  lone surrogate; `TestWireGoldenInvalidUTF8IsReplaced` characterizes that.
- **`WIRE-FULL-BODY-DUMP` stays open**, so the content-level bisect for the
  remaining `2013` cases is still blocked. This change narrows the blast
  radius of that failure; it does not explain it.

## Plan deviation

None in behavior: the approved scope table is implemented item by item (1–3
implemented, 4–6 no change), with the retry-state generalization it asked for.
One file outside the plan's list changed: `internal/app/assembly_audit_test.go`
simulated the exact case now retried (`invalid params, 400 (2013)` on the first
turn) and asserted one captured request, so the package failed once the replay
was live. The test now requires two byte-identical requests and keeps its
content-free-wire assertions; it is stronger than before, because the identical
replay is the property the journal fingerprint relies on. See
`verification.md`.

# VCP A2 — `--mode print` and `--mode json`

Date: 2026-10-06. Story: [A2](../../superpowers/plans/vivy-code-parity/A2-print-json-modes.md). Spec: VCP-D1 §5.1 / RQ-JSON.

## What landed

- **`sdk/facerun`** (new package): the shared non-interactive run engine.
  `facerun.Run(ctx, host, opts, sink)` resolves the session
  (`--session-id` > `-c/--continue` newest > fresh `session/create`),
  drives `initialize` → `turn/start` → `run/subscribe` (journal replay via
  `after_seq` semantics), verifies every assistant stream against the v2
  `model.completed` digest (content_sha256 + byte_len), cancels loudly when
  a run blocks on approval or a user question, and maps terminal events to
  `completed`/`failed`/`cancelled`.
- **Sink contract**: semantic `Sink` (deltas, tool lifecycle, blockers,
  terminal), optional `RawSink` (verbatim journal events) and
  `LifecycleSink` (session/run ids + terminal status) so JSONL mode can
  emit records the journal does not carry (`session`, `turn_start`,
  `turn_end`, `agent_end`, `agent_settled`).
- **`TextSink`** = pi-style print output (assistant text on Out, noise on
  Err). **`JSONLSink`** = pi-compatible JSONL records, `"v":1` schema,
  `vivy` field carries the raw event type for losslessness; unmapped
  journal types pass through as `journal_event`.
- **`sdk/tui/face`**: `Options.Mode` dispatch — `print`/`json` route into
  facerun; `rpc` still returns `ModeUnavailableError` (A3).
- **`faces/headless`**: rewritten as a thin delegate onto facerun — one
  engine for every headless face (single source of truth).
- **Loud guards** in `facerun.Options.unsupported()`: `--export`,
  `--no-session`, `-r/--resume` (picker), `--session`, `--fork`,
  `--session-dir` fail with named errors instead of silently running the
  wrong session. `--no-session` is rejected on principle: every VIVY run
  is journaled (write fence); the pi semantic cannot be honored honestly.

## JSONL record mapping

session/turn_start (lifecycle) → agent_start (`run.started`) →
message_start (`model.request`) → message_update (`model.delta`,
`model.reasoning_delta`) → message_usage (`model.usage`) →
message_end (`model.completed`) → tool_execution_start/update/end
(`tool.started`/`tool.operation`,`tool.requested`,`tool.approval_decided`/
`tool.finished`) → compaction_end (`context.compacted`) →
auto_retry_start/update (`provider.retry`/`provider.stall`) →
session_update (`session.truncated`/`session.forked`) → error
(`run.failed`/`run.cancelled`/blocks) → turn_end → agent_end →
agent_settled (terminal tail emitted by RunSettled so ordering holds for
every outcome). `compaction_start` has no journal counterpart yet — it
lands with D1 (compaction instructions) if a start event is added.

## Deviations / notes

- `-r/--resume` is a TTY picker in pi; headless modes must name the
  session (`-c` or `--session-id`) — loud error, documented in `unsupported()`.
- `go test ./internal/app` continue-test uses a second `Storage.DataDir`
  for the second invocation (shared journal, fresh memory home) — same
  isolation real vivy-code instances get via `codeface.Prepare`.
- `just ci` full sweep deferred to story end per repo convention; focused
  suites all green (see verification.md).

## Digests re-pinned

- `faces/headless` module descriptor + `vivy-module.yaml`: → `d9b48817…`
- Conformance `vivy/headless` (plain tree hash): → `24ba4a2b…` in
  `conformance_results.json` **and** `reproduction_test.go` suite table
  (two pins — easy to miss the second).
- `internal/` shared digest (new test file): → `60521960…` in the 5
  internal suites.

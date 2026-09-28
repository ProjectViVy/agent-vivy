# ACP-04 HITL, Cancellation and Cleanup Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Story / Epic:** ACP-04 / E1. **Goal:** Route permission and Ask User through Vivy's existing reviews, and terminate active Runs safely on cancellation or connection loss.

**Architecture:** Extend ACP-03's bounded connection-local dispatch. Reverse requests are bound to an owned Run and a durable Vivy gate ID; only Control RPC decides a review. A separate read loop remains responsive to client `session/cancel` while requests await user action. **Tech Stack:** Go 1.26.4, ACP-01 pinned stable SDK, Vivy Control RPC, committed Journal events. **Spec:** [reviewed design](../../specs/2026-09-28-issue1-acp-face-design.md), baseline `3c4ed66`. **State / dependencies:** [index](index.md); requires ACP-03 accepted owned state, writer and event dispatch.

## Global Constraints

- One active prompt per session; one client connection per process; all review IDs bound to `(session_id,run_id,review_id,tool_call_id)` as applicable. Effective grant stays `rpc.client`.
- Never grant permission or fabricate Ask User on timeout, unsupported form capability, disconnect, malformed or late response; fail closed through existing `approval/respond`, `question/respond`, `review/respond` or `run/cancel`.
- `session/cancel` is an ACP notification with no response; prompt returns `cancelled` only after the owning Vivy Run commits cancellation. No promise to undo an already authorized tool effect.
- Errors and limits are frozen by ACP-01. No new Journal, Policy, OS sandbox, or alternate review store.

## Review Focus

1. Client sends `session/cancel` while a permission or form reverse request is pending: the read loop still processes it and never grants afterward.
2. A late approval from a cancelled or expired gate is ignored even if the ACP request had once been valid.
3. A guessed gate ID from a second session cannot pass through this adapter to `approval/respond` or `question/respond`.
4. EOF or broken pipe cancels every *owned active* Run with bounded cleanup; a failed cleanup is logged to stderr without reporting completion.
5. Unsupported form client, decline/cancel, reverse request timeout, and saturated outgoing queue fail closed rather than leave a live prompt falsely successful.

---

### Task 1: Permission and form review routing

**Files:** Create (proposed) `plugins/acp/{reviews.go,reviews_test.go}`; modify proposed `plugins/acp/{events.go,connection.go,prompt.go}` only where dispatch/reverse calls must be wired.

**Interfaces:** Consume ACP-03's owned state, writer, and `tool.approval_required` / `user.question_required` committed event dispatch. Produce bounded pending-review records keyed by real Vivy IDs. Map ACP `session/request_permission` allow-once/deny to Control `approval/respond` only after an option matches a pending owned gate; map form-only `elicitation/create` accepted `answer` to `question/respond`. On form decline/cancel use `review/respond` cancellation or cancel owning Run per the frozen G0 contract. Never advertise URL mode.

- [ ] Write failing tests for approve and deny, mismatched/foreign IDs, concurrent independent reviews, late response after expiry, invalid option, unsupported form client, accepted bounded answer, form decline/cancel and timeout. Assert zero authorizing Host calls on invalid cases.
- [ ] Run `go test ./...` in `plugins/acp -run 'Test(Permission|Question|Review)' -count=1`; expect failures before wiring.
- [ ] Implement per-review claim/settle state with a single authoritative transition; send reverse requests asynchronously via the bounded writer so inbound cancel remains live. On failure, attempt `run/cancel` with a bounded cleanup context; do not synthesize approval/answer.
- [ ] Rerun focused tests; assert only Control RPC mutates gate state, real gate expiry wins races, and no sensitive review payload appears in ACP output or logs.

### Task 2: Cancel, disconnect and outcome races

**Files:** Create (proposed) `plugins/acp/{lifecycle.go,lifecycle_test.go}`; modify proposed `plugins/acp/{connection.go,session.go,prompt.go}` as needed.

**Interfaces:** Consume ACP-03 session/run ownership and committed terminal dispatcher. For owned `session/cancel`, call `run/cancel` with active `run_id` and finish only on committed terminal state. EOF, signal/context cancellation, backpressure/write failure stop accepting prompts, cancel all owned active Runs under one bounded cleanup context, stop subscriptions/reverse calls, then return from `Instance.Run` so existing FaceHost closes App. A failed Control cancellation is a reported cleanup failure, not a completed Run.

- [ ] Write failing tests for cancel during permission/form, same-session prompt overlap, two-session cancellation isolation, cancel-versus-completion orderings, already-completed prompt, EOF, broken writer and cleanup cancellation failure. Capture stderr/stdout to ensure only ACP frames reach stdout.
- [ ] Run `go test ./...` in `plugins/acp -run 'Test(Cancel|Disconnect|Backpressure|Cleanup)' -count=1`; expect failures before lifecycle change.
- [ ] Implement once-only prompt completion and bounded cleanup; serialize ACP output without serializing the whole read loop. Use Vivy's persisted terminal event to settle cancel/complete races, returning sanitized failure after `run.failed`.
- [ ] Rerun focused and full adapter tests, run `just ci` in a working environment and add real outcome traces to the delivery log. Report any Run that could not be durably cancelled; do not mark Story Done on an incomplete cleanup gate.

**Handoff:** ACP-05 receives a stable Module source tree, complete transcript fixtures and review/cancel evidence. Any public capability, error, or source-code change after source pinning must regenerate ACP-05's hash and rerun Pack/Inspect.

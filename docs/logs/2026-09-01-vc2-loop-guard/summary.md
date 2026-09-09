# VC-2 Loop detection (tool loop guard)

Date: 2026-09-01  Branch: `feat/vc1a-bash-tool` (worktree `agent-vivy-vc0`)  Task: VC-2 item 2 (research §5 VC-2.2)

## Scope

Align with Crush's `StopWhen` loop guard: **terminate the run when the same
"call + result" signature repeats more than 5 times among the last 10 completed
tool calls**. The signature is tool name + normalized-parameter JSON + result
text + error text (64-bit FNV hash; the result is not retained in memory).

- **Implementation point**: `eventMapper` adds `loopWindow` (one per run; the
  window resets after approval recovery—the recovery is a human decision point,
  so restarting the count is appropriate and documented). When a tool result
  completes, the signature is recorded in `toolResultEventsParts`; over the limit
  returns the `errLoopDetected` sentinel.
- **Parameter normalization**: at request time, re-marshal the decoded
  `map[string]any` (Go sorts by key) into `openToolCall.argsJSON`; calls with the
  same parameters but reordered JSON keys still count as repeats.
- **Termination path**: `terminalEvent` adds the `errLoopDetected` category and
  the new cause category `loop_detected`; the user-visible message is bounded
  (FR-11: no signature or engine internals leak). Behavior matches
  `ErrExceedMaxIterations` (MA-4) and the budget circuit breaker.
- **Not done**: no configuration item (window/limit are compile-time constants,
  matching Crush 10/>5); no `engine.go` change (preserve the GATEWAY document
  constraint); no UI change (`failure.ts` directly displays server messages for
  non-`provider_error` categories, so `loop_detected` works automatically).

## Legal and alignment

Crush is FSL-1.1-MIT: this slice only aligns behavior (10-step window, stop after
>5 repeats), with zero code copied; hashing/window/normalization are entirely
  our own implementations.

## Verification

See `verification.md`.

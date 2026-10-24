# D2 — Context-overflow recovery (compact-and-retry)

**Goal:** provider overflow / `stopReason==length` triggers one auto-compact + retry inside the run; second overflow fails honestly.
**Epic:** D. **Requirements:** RQ-CMP. **Predecessor:** D1 (CompactSession signature, summarizer seam).
**Spec:** VCP-D1 §5.5.

## Scope

**Files:** `internal/runtime` consume/error path (classify provider errors; Anthropic `context_length_exceeded`, OpenAI `context_length`/rate-limit-token errors, `stopReason:length`), `internal/runtime/compaction_service.go` (in-run compact entry point), Journal event `context.compacted{mode:"overflow-recovery"}`, `auto_retry_*` projection alignment.

## Tasks

- [ ] Eino check: confirm how the pinned adapter surfaces overflow (typed error vs message string) — record inspected types.
- [ ] Error classifier → `ErrContextOverflow` (internal typed error; NOT user-facing panic).
- [ ] On overflow mid-run: journal attempt, run `CompactSession` (no instructions), continue the run once; emit `auto_retry_start/end` for face display.
- [ ] Repeated overflow in same run → fail with clear cause; `stopReason:length` alone (no error) → same path.
- [ ] Tests: fake provider returning overflow → run continues post-compact; second overflow → run failure with cause; no compaction loop.
- [ ] `go test ./internal/runtime -run 'Overflow|Compact'`; `just ci`.
- [ ] Commit `feat(runtime): overflow compact-and-retry recovery`.

## Boundary

One retry per run per overflow class; never silently truncates messages to squeeze under the window (compaction is the honest mechanism).

## Acceptance

A scripted overflow during a live run completes the run successfully after auto-compaction.

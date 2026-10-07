# B2 — TUI steering keys + queue UI

**Goal:** `Enter`=steer, `Alt+Enter`=follow-up, `Alt+Up`=dequeue; queue display shows both tracks.
**Epic:** B. **Requirements:** RQ-STEER. **Predecessor:** B1 (kernel queue truth + RPC).
**Spec:** [VCP-D1](../../specs/2026-10-06-vivy-code-parity-design.md) §5.2 + O8.

## Scope

**Files:** `sdk/tui/live/controller.go` (replace local `queuedTurn` FIFO with kernel queue calls), `sdk/tui/view` (composer hints + queue lane rendering), `sdk/tui/command` (`/queue` shows both tracks; `/queue clear` → `queue/clear`).

## Tasks

- [ ] Busy composer: Enter → `turn/steer`; Alt+Enter (ctrl+q fallback where terminals lack alt) → `turn/follow_up`; Alt+Up → dequeue head back into editor text (via `turn.dequeued` payload).
- [ ] Queue lane renders steer/follow_up tracks distinctly (count + preview); mode indicator when `one-at-a-time`.
- [ ] Abort: kernel flushes queue to dequeue events → restore newest to editor (pi behavior).
- [ ] Idle Enter unchanged (direct `turn/start`).
- [ ] Tests: controller unit tests for key→RPC mapping; queue lane render; dequeue restore.
- [ ] `go test ./sdk/tui/...`; TUI smoke on real session (steer visibly lands at a boundary).
- [ ] Commit `feat(tui): steering and follow-up key tracks`.

## Boundary

Keys become configurable under G2 later — define them as named actions now so G2 just adds the loader.

## Acceptance

During a long run, Enter-steer visibly steers the next turn; Alt+Enter lands after settle; Alt+Up restores text.

# B1 — Kernel dual-track queue + turn-boundary steering

**Commit:** `feat(runtime): dual-track message queue with turn-boundary steering`
**Depends on:** A3 (rpc mode), face.Options (A1)
**Spec:** `docs/superpowers/specs/2026-10-06-vivy-code-parity-design.md` §5.2
**Plan:** `docs/superpowers/plans/vivy-code-parity/B1-kernel-steering.md`

## Eino capability check (mandatory first step)

Eino v0.9.13 adk supports the mechanism out of the box — **M2 chosen**, no
internal loop surgery:

- `adk.WithCancel() (AgentRunOption, AgentCancelFunc)` — register a cancel
  handle per run.
- `cancelFn(adk.WithAgentCancelMode(adk.CancelAfterChatModel))` —
  **boundary cancel**: the run checkpoints at the next turn boundary
  (model call edge), never mid-tool-batch.
- `ResumeWithParams(ctx, checkPointID, &ResumeParams{Targets})` with
  `ChatModelAgentResumeData{HistoryModifier}` — resume the checkpointed
  run while rewriting history to inject the steer message.

Verified semantics (runtime debug + `adk/interrupt_test.go`):

- A boundary cancel surfaces in consume as `errRunCancelled` whose
  `*adk.CancelError` carries `InterruptContexts` — the checkpoint info.
  The mapper now stashes `InterruptInfo` into `interruptDetails.Raw`.
- Resume `Targets` are keyed by real `InterruptCtx.ID`s (UUIDs), not
  `"agent:"+name`; `resumeSteeredRun` walks `InterruptContexts` for
  contexts whose last address segment is `AddressSegmentAgent`.
- A boundary checkpoint pins the next node to ToolNode with a trailing
  `assistant(tool_calls)` message; a bare appended `user` fails
  `"expected message role is Assistant"`. The HistoryModifier therefore
  inserts the steer `user` message **before** that tail.
- The resume leg must NOT run inside the cancelled run's own consume
  frame (checkpoint serialization deadlock). `resumeSteeredAsync`
  spawns it on a goroutine, same pattern as `resumeSuspended`.

## What landed

### Domain + journal

- `internal/domain/turn_queue.go`: `QueuedTurn`, tracks `steer` /
  `follow_up`, modes `all` / `one-at-a-time`.
- Journal vocabulary 57 → 60: `turn.queued`, `turn.dequeued`
  (`{queue_id, track, reason, next_run_id?}`), `turn.steered`.

### Kernel (`internal/runtime`)

- `sessionQueue` per session: two lanes + per-lane modes +
  `admittedBy`/`lastAdmitted` admission records.
- `Service.Steer`: demotes to the follow-up lane when the run is
  suspended on a pending approval/question or has no cancel seam;
  otherwise journals `turn.queued` and issues the boundary cancel.
  `CancelFunc` returning `!ok` (already settled) demotes the item.
- On `errRunInterrupted`/`errRunCancelled` while steer-armed: build
  resume targets + HistoryModifier and resume asynchronously; on resume
  failure demote the items and settle `run.cancelled`.
- `emitTerminal` tail: Completed/Failed → `drainFollowUps` (lane mode:
  `all` drains whole lane, `one-at-a-time` pops head only); Cancelled →
  `flushQueue` (both lanes dequeue with `aborted`).
- Durability: `turn.queued`/`turn.dequeued` journal on the newest run's
  journal (admission re-journals the tail); restart rebuild replays only
  the session's latest run journal.

### Face discovery — no wire hint (bus constraint)

`streamRun` drops live events `Seq <= last`, closes subscriber channels
on the terminal publish, and journal replay stops at the terminal entry —
a post-terminal wire-only hint can never reach faces. Instead the kernel
records `admittedBy[settlingRun] = newRun` on the session queue;
`queue/state{session_id, after_run_id}` returns `admitted_run_id`, and
`session/get` exposes `last_admitted_run_id`. Faces poll `queue/state`
briefly after a terminal (`admitSettledFollowUp`, ~2 s) and subscribe
the admitted run.

### RPC (`internal/rpc/control.go`)

`turn/steer`, `turn/follow_up`, `queue/state`, `queue/clear`,
`queue/mode`; `queue/clear` returns the dequeued texts (editor restore).
Busy rule per plan: `turn/steer`/`turn/follow_up` on an idle session
fall back to a fresh `turn/start` (pi: queued turn while idle = prompt).

### rpc-mode face (`sdk/tui/face/rpc.go`)

`steer`, `follow_up`, `prompt{streamingBehavior}`, `clear_queue`,
`set_steering_mode`, `set_follow_up_mode` now hit the kernel verbs;
on settle the face resolves `queue/state` for kernel-admitted runs and
projects `turn_start{admitted:true}` + subscribes the new run.

### codeclient

`Steer` returns `Disposition`; the stale "fails until B1" assertion now
asserts the start fallback.

## Tests

`internal/runtime/queue_test.go` — all passing:

- `TestSteerInjectsAtTurnBoundary` — steer visible in resumed history,
  never mid-tool-batch.
- `TestSteerOnIdleSessionIsUnavailable` — kernel 409; RPC falls back to
  a fresh run.
- `TestFollowUpAdmitsAfterSettle` — auto-admission after terminal.
- `TestFollowUpOneAtATimeKeepsTail` — 3-run cascade, one dequeue marker
  per admitted run journal.
- `TestQueueRebuildsFromNewestRunJournal` — restart rebuild.
- `TestSteerDuringSuspendedRunDemotesToFollowUp` — suspended-run demote.

Plus `TestCodeFaceRPCModeQueuesFollowUpWhileRunning` (two settles over
the wire) and the updated golden transcript.

## Pitfalls for future kernel work

- **Never journal inside `s.mu`** — append blocks on storage.
- Boundary-cancel is a `CancelError`, not an interrupt — handle both
  branches.
- Resume targets = real `InterruptCtx.ID`s; HistoryModifier must respect
  a trailing `assistant(tool_calls)` tail.
- Post-terminal wire hints are unreachable by bus design — use
  state-pull (`admittedBy`) instead.

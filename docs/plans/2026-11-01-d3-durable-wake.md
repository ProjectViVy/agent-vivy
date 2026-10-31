# D3 durable waiting and wake delivery

Status: implemented and independently reviewed. Base: `c0bad0d` (D2).

## Goal and boundaries

Close execute -> persist waiting -> wake at a UTC deadline -> agent chooses
continue, wait again or end, using the existing coding face and Rig execution.
Keep the same task identity and monotonic operation numbering across waits.
New D3 sessions only; preserve the D2 executable and its sessions. No migration,
daemon, cron, idle task creation, inbox, Minecraft or AutoDream implementation.
No new database, external runtime, autonomy budget or approval mechanism.

## Implementation sequence

1. Select a compile-time `wake` feature in the persistent recipe. Extend the
   existing durable checkpoint with Waiting, schedule identity/generation,
   task generation, UTC deadline, reason and delivery state. Require a new
   saved schema. One live wake per task, multiple waiting tasks per agent.
2. Extend task_control with wait and cancel_wake; freeze relative deadlines
   when the control is proposed. Apply after the control and sibling results
   commit. Add read-only wake_status, Rust schedule/cancel APIs, and local
   /wake list, cancel and reschedule commands with generation checks.
   Uncertain mutations use status inspection, never blind replay.
3. Reuse Rig WakeAt/Due and one earliest host timer. Recheck wall clock and
   generations before firing. Save fired state and queued input together;
   release input only after its exact receipt. Reconstruct timers and overdue
   inputs on reopen. Native input delivery observes the single-tool boundary;
   the model decides whether to switch tasks. Cancellation invalidates only
   the targeted schedule and withdraws undelivered notices.
4. Project internal wake notices with explicit plugin authorship and stable
   source IDs. Preserve persona + committed Recap + current task context,
   including the task's previous operations. Keep lifecycle source bytes
   immutable for Recap. Foreground work preempts unfinished Recap. Live hosts
   wait for deadlines; print mode saves Waiting and exits with a notice.
5. Validate offline state transitions, receipt failures, stale generations,
   clock changes, tool-boundary races, restart boundaries, context continuity
   and cancellation. Run the Windows executable against a scripted gateway,
   plus appropriate D1/D2/coding regressions and absent-feature checks. Record
   bounded latency/model-call/log-size evidence, update design/handoff, and
   obtain one fresh whole-branch review before the focused commit.

## Acceptance

- A task waits and resumes repeatedly without creating unrelated user tasks.
- A wake never dispatches before its schedule/fire/input checkpoint receipts.
- Restart neither loses a committed wake nor replays uncertain side effects.
- Cancelling/replacing a wake fences old generations without stopping another
  active task; unstarted siblings yield at an individual tool boundary.
- Waiting makes no foreground model calls until input or a due wake arrives;
  existing interactive Recap remains independent maintenance.
- Old and capability-incompatible logs are refused without modification.
- Tests distinguish scripted gateway behavior from real-provider acceptance.

The initial implementation retains cumulative checkpoint logs. Bounded smoke
measurements do not establish economical multi-day operation. Real-provider
acceptance remains deferred by the owner.

## Delivery evidence

47 offline checkpoint/runtime/context/wake tests and 34 root tests passed.
The D2-only feature set passed its 35 tests. Native Windows D3, D1, D2 with
optional native memory, and coding smoke passed as recorded in HANDOFF.
The production executable passed wait/wake/silent completion with exit zero and
no diagnostic hooks. A fresh read-only whole-branch review found no substantiated
Critical, Important or Minor issue requiring correction; no second review was
used. Recorded scope exclusions remain exclusions, not successful verification.

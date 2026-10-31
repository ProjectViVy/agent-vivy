# D2 assistant context implementation plan

**Goal:** connect native Laputa persona and asynchronous Recap-only context to
the D1 persistent agent, with separately enabled native-memory acceptance.
**Baseline:** Vivy `2eb70d6`, local Rig `810d7dfd`; shared Rig execution only.
**Spec:** DESIGN, sections "Requirements audit" and "Persona, Recap and memory
integration", plus the owner's accepted D2 plan and scheduling clarification.

## Accepted boundaries

- New D2 sessions only. Preserve D1 data and executable; incompatible opens fail
  before changing data. No automatic migration or production pin change.
- Persona/instructions + committed Recap + current input/activity. No raw old
  chat fallback or automatic recall. Active tool pairs must remain complete.
- Background Recap yields to foreground input; foreground never waits for model
  synthesis. Retain pending sources and reject late cancelled candidates.
- No timers, idle autonomy, AutoDream, persistent delegation, coding session
  management, owner classification, second runtime/database or new product face.
- Compile-time capability selection; memory stays optional and off by default.
- Ordinary process-crash guarantee; no OS/power-loss claim. Windows first.

## Tasks and interfaces

- [x] **1. Native persona.** `vivy-laputa`: versioned workspace persona, read,
  CAS update and rollback with update-ID reconciliation, OS lock, atomic current
  replacement and immutable revisions. Empty revision zero when absent. Commands
  show/update/rollback. Root persona survives base-role replacement; new operations
  see updates and retries keep frozen requests. Tests: conflict, repeated update,
  concurrent writer, interrupted publication, rollback, actual composed request.
- [x] **2. Request projection.** Optional `AssistantContextPlugin` in durable;
  stable task/message references and per-agent required saved context state in
  the same checkpoint. New tasks omit old raw messages; continuing tasks retain
  their own tools; switches/recovery preserve identities and pairing. Freeze
  persona/Recap revisions with the actual request. Fit using Rig model limits;
  preserve and suspend oversized required input, require missing window facts.
- [x] **3. Async maintenance.** Persist `RecapJob` independently of user tasks;
  reuse Rig model calls/summary validation/retry/cancellation and exact receipts.
  Coalesce closed sources only, one job per agent, no empty calls. Track actual
  partial source coverage; never mark silently truncated sources complete.
  Publish complete candidates against expected revision and source window only
  after receipt. Foreground cancels maintenance, preserves pending material and
  fences late replies. Commands `/recap status` and `/recap refresh`, excluded
  from user tasks/sources. Interactive idle maintenance; headless completion does
  not wait for synthesis. Summary uses concise event memory, commitments,
  corrections and truthful task status; source time, no invented provenance.
- [x] **4. Optional memory.** Typed remember/get/search over existing Python
  workbench and native MemPalace, same worker, structured arguments. Stable
  workspace/source and task/event identity frozen before dispatch. Missing IDs
  fail instead of `Entity`/`unsaved`. Explicit get reconciles uncertain writes;
  conflict/incomplete stays unresolved, no blind replay or arbitrary Python
  inspection permission. Disabled memory cannot affect Recap acceptance.
- [x] **5. Joint acceptance and review.** Offline tests and real executable
  scripted gateway requests; kill at summary intent/result/publication; persona
  revision/retry, tool-context continuity, foreground cancellation and recovery;
  separate actual MemPalace acceptance and existing configured provider check.
  Retain coding regressions. Record request bytes/calls, preemption latency, log
  growth and reopen time. Update DESIGN/HANDOFF. One whole-branch fresh review,
  related commits only, no merge/push.

## Implementation organization

Persona storage/composition stays in the small Laputa plugin. Durable context
projection and maintenance live in focused modules beside its runtime and share
its existing checkpoint ownership. Memory adapters live beside the Python worker;
they do not introduce another worker or memory store. The persistent recipe
selects persona and context explicitly. No Rig extension is assumed.

## Review focus

Cancelled maintenance must never publish late or cancel a foreground task.
Input/task switches must not leak another task's raw context or orphan a tool ID.
Source coverage must describe the bytes actually summarized, including long input.
Persona retry and update races must not change a frozen model operation.
Read-only memory reconciliation must not authorize arbitrary Python replay.

## Execution record

Implementation is in `feat/d2-assistant-context`, reusing the isolated worktree.
The ignored `.workspace/d2/progress.md` contains evidence and implementation
rulings. D1 executable retained under `.workspace/d2/baseline/`.


## Verification and final review

Completed in the isolated branch. Root tests: 34; local Rig consumer D0/D1/D2:
35; Laputa: 5 (also passed against the unchanged production Rig pin); typed
Rust workbench: 22; prepared Python/native suite: 45. D2 executable acceptance
passes no-Python dependency selection, actual request projection/preemption,
transient retry, four Recap process-kill boundaries, typed native memory,
unknown-write reconciliation and D1 incompatibility with unchanged logs. Coding
smoke and the nine-boundary D1 matrix plus Shell/Python stop regression pass.
Formatting and whitespace checks pass. Measurements and commands are in HANDOFF.

A fresh independent whole-branch read-only review found no actionable Critical,
Important or Minor findings. It inspected the recorded execution evidence and
independently checked the diff; it did not rerun tests. No corrective pass was
needed. Its declined-to-judge items are accepted scope limits: real-provider
compatibility and semantic summary quality, other platforms/OS or power failure,
long-run log economy, and sandboxing against arbitrary earlier Python code.
These are not certified by deterministic fixtures or this review.

The existing owner ruling defers actual-provider verification; no complete
replacement credential/model pair was available. Native test reruns used the
prepared interpreter and its writable isolated temporary directory; system
MemPalace packages were incompatible. Neither required a product workaround.
The branch and evidence are retained; no merge, push or production-data migration.

# A2A-02 — Atomic native admission for external messages

Executed the three tasks of
[A2A-02](../../superpowers/plans/2026-10-07-a2a-server/A2A-02.md) on branch
`A2A`. One transaction carries the durable receipt decision, candidate
session, ownership row, message, run, prompt snapshot and journal events;
retry resolves through the receipt inside the transaction before every
busy gate (RF1/RF2).

- **A2A-02.1** — `channel_task_*` schema (migration `036`, sqlite +
  postgres twins), `domain.ChannelTask*` types, `ChannelTaskStore`
  interface + both backends, `channel.task_admitted` vocabulary event +
  payload schema. Fault-matrix suite (8 subtests + two-handle) green on
  both engines.
- **A2A-02.2** — `Service.SubmitChannelTask` /
  `RunOptions.ChannelTask` admission path: canonical
  `channel-task/v1` input hash, receipt-before-busy ordering (cheap
  `projectionMu` pre-check + in-transaction recheck), candidate session
  minted via `newPrefixedID("sess_")` discipline, workspace loser
  cleanup through `DiscardNewPrivateAdmission`, optional
  `DiscardFrozenSession` seam (laputa pin bump pending), runtime
  startup sweep `SweepChannelTaskOrphans` for provisional workspaces,
  wired in `App.Run`/`StartEmbeddedServices`. `-race` clean.
- **A2A-02.3** — ownership reads (`GetChannelTaskOwner`,
  `ListChannelTaskRuns` keyset cursor) + retained deletion tombstones:
  `DeleteSession` marks `channel_task_contexts`/`channel_task_receipts`
  `deleted_at` inside the same transaction before row removal;
  tombstoned dedup keys revive with fresh session identity; live
  receipts pointing at missing runs report corruption, never
  resubmission.

Semantic map honored end to end: foreign identity → `ErrNotFound`,
same key + different hash → `ErrConflict`, busy context + distinct
message → `ErrWorkRunConflict`, identical retry → original receipt.

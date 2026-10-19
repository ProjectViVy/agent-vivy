# HITL Review Center

Status: implemented and release-verified P0, 2026-08-12

## Contract

`domain.ReviewItem` is the read model for human work. It projects approvals
and user questions without exposing the runtime-owned proposal payload. The
projection contains the owning session/run, tool and source, actor and
timestamps, expiry, target, precondition, bounded preview, risk findings,
redacted arguments, question prompt, and the terminal reason.

SQLite remains the source of truth. Approval and question rows retain their
original stores and compatibility methods; `ReviewStore` adds `review/list`
and `review/get` for a cross-session queue and detail view. `review/respond`
routes approval approve/deny and question answer/cancel to the existing
runtime service, so authorization and first-writer-wins behavior stay on the
server.

## Lifecycle

```text
pending -> approved | denied | cancelled | expired | stale
pending -> approved                            (smart preset, timeout auto-approval)
pending -> answered | cancelled | expired       (question)
approved -> stale                              (precondition mismatch)
```

Every response is conditional on `pending`, and the first durable writer wins.
The interaction sweeper performs the server-side timeout transition. For a
pending approval it applies one of two transitions at the single deadline
recorded on the row (`expires_at`, the configured review window clamped by the
hard `tools.approval.expiration`):

- Under the **smart preset only** (sandbox `workspace_write` + policy `ask`),
  and only while
  `runtime.sandbox.approval.timeout_seconds` (settings → 通用 → 审批超时) is
  greater than zero, the sweeper **approves the call on the user's behalf**,
  records the decision with the `system` actor and an
  `auto-approved after Ns …` reason, and resumes the run. A background
  conversation therefore continues instead of dying at the deadline.
- Under every other preset, or with timed auto-approval disabled, the
  approval **expires** and closes the owning run with the structured
  `human_timeout` cause (unchanged).

The eligibility check reads the sandbox mode and approval policy recorded on
the row when the human was asked, never the current settings, so changing the
preset cannot retroactively authorize a call. The enabled flag and duration
come from the live setting. Restart recovery uses the same transition.
Decision, expiry, cancellation, and stale outcomes are journal events and can
be replayed by the run inspector.

## Restart across an engine version (pending-run policy)

A paused run resumes from its Eino checkpoint, and a checkpoint is readable
only by the exact Eino version that wrote it (`VersionedCheckpointStore`,
`internal/runtime/checkpoint.go`). A Vivy upgrade or rollback that changes
the Eino version therefore cannot resume a run that was waiting on a person.
Supported boundary for the Eino v0.9.21 upgrade:

- Upgrade from the historical v0.9.13 binary to a binary containing the durable
  cleanup fix without draining first. Recovery fails an incompatible paused run
  with `run.failed`, cause `internal_error`, and the message *"Vivy was updated
  while this run was waiting, and a paused run cannot continue on a different
  engine version. Its pending request was cancelled. Please start the task again."*
- Approval and Question cancellation events precede the terminal; Plan review
  cancellation follows it. Before these writes, recovery saves a durable cleanup
  intent in the existing BlobStore. A failed write aborts recovery/startup and
  preserves the cause. Repeated Recover or a restart retries the intent, even
  when its Run is already terminal. The intent is removed only after all writes
  succeed. Journal replay, lifecycle CAS and Work request identity prevent
  duplicate cancellation and terminal events. Never delete review evidence.
- No checkpoint is converted and no version check is bypassed. A missing or
  unreadable same-version checkpoint follows the same cleanup protocol with the
  generic restart message. Prompt-snapshot incompatibilities remain pending.
- **An unpatched historical binary cannot open the database after migration
  44.** The actual Eino v0.9.13 build from `e960929` fails closed with
  `unknown migration version 44`, both with pending requests and after the
  current binary has cancelled them. Cleanup alone therefore does not make
  this rollback safe or runnable. Do not claim bidirectional safety or start
  this binary against the upgraded database.
- The supported Eino rollback target is a compatibility build from the same
  source, with this durable cleanup fix and migration 44 retained and Eino
  pinned to v0.9.13. Build it with
  `node scripts/build-eino0913-rollback.mjs`; verify its embedded Eino version
  before use. Its exact-version guard still rejects v0.9.21 checkpoints. It
  records one failed Run and completes Approval/Question/Plan cancellation
  before it accepts traffic; it does not resume that interrupted work. The
  isolated actual-binary matrix covers direct compatibility rollback and the
  pre-cleaned case. If this compatibility build is unavailable, rollback to
  the unpatched historical binary is unsupported after migration 44.
- Before stopping the current binary, stop admitting new work. A compatibility
  build can clean active/paused requests during startup; verify zero active
  Runs, pending Approvals, pending Questions, pending Plan reviews, and cleanup
  intents afterward. Check that each affected Run has exactly one terminal
  and its cancellation evidence, and repeat the check after restart. A cleanup
  error blocks rollback; keep the current binary and retry. Never convert a
  checkpoint or bypass its version check.
- To preserve a paused task's result, complete its human review before upgrading.

Evidence: `internal/runtime/recovery_engine_version_test.go`,
`internal/runtime/recovery_cleanup_test.go`, and the final-source actual-binary
matrix in `docs/logs/2026-10-19-pr83-closeout/verification.md`. The earlier
review baseline and initial matrix remain in
`docs/logs/2026-10-18-pr80-recovery/verification.md`. These cover the current
ChatModel generation only. AgenticModel/message-generation migration remains
subject to separate acceptance, including S4 B5/B6/B7.

## UI rule

Review Center is a full main-area surface for cross-session work. The run
inspector uses the same `renderReviewCard` renderer for inline decisions. This
keeps the queue and the active run consistent while preserving the current
session and composer state. Approval and question controls are intentionally
separate: an answer is data, never authorization for an effectful tool.

Arguments are redacted by key class before entering the ReviewItem API. File
and Skills previews are diff-first; other proposal types use the same target,
risk, trust, scope, and precondition fields and bounded preview. Browser Use
is excluded, and GraphTool remains a test-only conformance surface.

## Release verification

The real-process acceptance record is in
`docs/logs/2026-08-12-hitl-release-closure/`. It includes deterministic
approval/question Playwright coverage, race/restart/expiry/stale evidence, and
the release gate results. The mandatory gate does not require live provider
credentials or external network access.

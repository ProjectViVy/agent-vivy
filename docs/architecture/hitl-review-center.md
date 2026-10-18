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
Policy, in both directions:

- **No drain is required before upgrading, and no checkpoint is converted.**
  Version checks stay fail-closed.
- On startup, recovery closes each such run with `run.failed`, cause
  `internal_error`, and the message *"Vivy was updated while this run was
  waiting, and a paused run cannot continue on a different engine version.
  Its pending request was cancelled. Please start the task again."*
- Its pending interaction is settled, so no actionable card outlives the run:
  an approval or question is **cancelled** (journaled before the terminal),
  and a pending Plan review is **cancelled** after it. Prompt-snapshot
  incompatibilities are unaffected and still keep the review pending.
- A lost checkpoint at the same version is settled the same way, with the
  generic restart message.
- Operators who want a paused task to finish should decide its pending review
  before upgrading.

Evidence: `internal/runtime/recovery_engine_version_test.go`.

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

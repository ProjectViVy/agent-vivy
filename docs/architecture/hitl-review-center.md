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
pending -> answered | cancelled | expired       (question)
approved -> stale                              (precondition mismatch)
```

Every response is conditional on `pending`. The interaction sweeper performs
the server-side timeout transition and closes the owning run with the
structured `human_timeout` cause. Restart recovery uses the same transition.
Decision, expiry, cancellation, and stale outcomes are journal events and can
be replayed by the run inspector.

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

# Core fixes batch 1

The operator recovery RPC now calls `RecoverBackground` and reports an active
or suspended Run as a conflict. It no longer invokes startup recovery on live
process-owned work.

Start and queue RPCs share input preparation. Relative attachment and context
paths resolve against the target Session's selected workspace, with the existing
project default used only when the Session has no selection. Existing containment,
symlink, image, and text limits remain enforced.

SQLite receives an escaped absolute file URI, preserving literal `#`, `?`, `%`,
spaces, and Windows drive letters. No schema or migration changes are required.

Queued requests retain mode, policy profile, face, thinking, collaboration,
provenance, attachments, resolved context snapshots, and continuity inputs.
Private request bytes live in the existing protected BlobStore; Journal/Face
projections carry references. Enqueue and removal acknowledge only durable
success, preserve error causes, and leave memory unchanged on failure.

Each follow-up starts its own Run. The remaining FIFO queue is part of that
Run's atomic `run.started` admission, before inference. Restart recovery resumes
idle Session queues after resolving old active Runs. Rich steer requests become
follow-ups; text steering retains the Eino boundary cancellation/resume path.
In-flight steering remains recoverable until a successful terminal; a failed
continuation returns it to follow-up. Interrupted delivery therefore favors
retry over losing an accepted request.

Recovery selects a primary carrier, validates snapshots, and surfaces replay
errors. Sealed-carrier removals use Session-owned synthetic Journal tombstones,
following the existing synthetic-control-event pattern; they do not create a
second queue or database. Both storage backends delete these tombstones with
the Session. Consumed, removed, and cancelled private snapshots are reclaimed
after the corresponding durable transition.

Eino capability check: inspected the pinned Eino v0.9.21 `adk/runner.go`,
`TypedRunner`, `CheckPointStore`, and the existing `WithCancel`,
`CancelAfterChatModel`, `ChatModelAgentResumeData`, and `ResumeWithParams` use.
They own the in-run continuation mechanism. Vivy retains only its Session input
and Journal admission seam; no parallel agent engine was introduced.

Scope excludes UI source changes, Studio, model/provider catalogs, schema
migrations, deployment, and live provider calls. No release artifact was
published, so a separate release record is unnecessary.

Unreachable private-blob maintenance after a pre-marker process crash or failed
cleanup delete remains tracked as `QUEUE-REQUEST-ORPHAN-GC` in `docs/TODO.md`.
Normal request consumption and removal reclaim their snapshots.

# Acceptance

After previewing a file change, change the target to different bytes of the same
length and restore its timestamp. Approving the old preview must refuse, retaining
the new external bytes. Missing and empty targets are distinct. Re-read/re-present
the target and retry with a new intended operation to proceed. A later allowed
file call in a resumed run must use its own authorization, not the prior review.

An existing read tracked only by historical timestamp requires a new read; it
cannot authorize overwrite. File history remains available after schema upgrade,
without content digests. The same behavior applies to ToolWorld Host writes,
including no-op and revoked write access. Download conflicts do not fetch when
already stale, do not overwrite during in-flight changes, and leave no scratch.

Web/TUI review retains action,target,preview,risks and decisions; the digest row
is removed. The owner deferred split3015 smoke, so these user-facing steps remain
pending real-path verification. Independent behavior review and required backend
checks are recorded in verification.md; this slice does not accept all of D09.

Roll back source with the focused commit. Data rollback requires a quiescent
backup and deliberate cutover; never run an old executable against converted
metadata. D14 owns final finite artifact packaging and release acceptance.

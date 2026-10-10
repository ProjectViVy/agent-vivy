# Acceptance

- Recall and edited resend retain complete captured turn options and content on
  idle start, busy steer, and busy follow-up paths.
- Image/file bodies remain private in the TUI driver; metadata renders in the
  editor and snapshots are reused without filesystem resolution.
- The GUI exposes restored mode/thinking, image thumbnails, selected continuity
  references/scope, and captured file contexts; image/context removal is explicit.
- Failed sends retain the current text, preference edits, added images, original
  request ID, and captured context for retry.
- Abort/clear recovery retains separate full DTOs and provides explicit recall;
  recovery never merges different turn options or automatically starts a run.
- Nonempty user drafts and drafts typed while recall is pending are preserved.
- Unsupported options remain pending. Conditional dequeue mismatches restore no
  turn and consume no unvalidated work.
- Focused regression and existing impacted tests pass; integrated product and
  browser gates belong to the parent delivery record.

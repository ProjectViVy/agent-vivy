# Acceptance

- An injected audit-event failure leaves no rewind marker or fork child.
- Forked message attachments are committed in the same transaction as the child session.
- A truncation-store read failure returns an error rather than exposing unfiltered history.
- Chat edit, rewind, and fork controls remain open with the original draft/object when a request fails.
- `session/edit` returns a run only after the edit marker, replacement message, active run row, and `run.started` event commit together.

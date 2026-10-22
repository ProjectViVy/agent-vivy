# Scoped human-response acceptance

- Approve, deny, or answer immediately after the corresponding durable request
  is published. If native suspension registration is still landing, settlement
  waits for it. The same Run then resumes its actual Eino checkpoint and closes
  once. Approve creates one note; deny and question handling create none.
- Repeating the accepted response reports the existing already-decided or
  already-answered result and cannot resume or perform the effect again.
- If registration never arrives within the existing bound, the call returns an
  error and leaves the durable slot pending. After a valid registration lands,
  the same slot remains usable.
- Cancel while registration is landing. The Run cancels once, no response
  decision/answer is recorded, no second model call occurs, and no tool effect
  executes.
- Start a human response before expiry but let readiness cross its deadline.
  Settlement returns the existing expired error and keeps the slot pending.
  The established smart-mode system timeout path can still auto-approve.
- Child approval/recovery, root recovery, shell approve/deny, native
  orchestration resume, and synchronous cancellation during approval publication
  retain their targeted native test behavior.

These checks use disposable SQLite stores. The boundary fixture admits the
immutable prompt and seeds the same process maps as `Service.Run`, then invokes
the actual `Service.drive`/typed Eino runner. It blocks the owner's
post-publication `Context.Err` check and verifies `projectionMu` is free; it does
not pause publication, fabricate checkpoint bytes, or add a production hook.
Continuation assertions count actual model/tool lifecycle events and effects.

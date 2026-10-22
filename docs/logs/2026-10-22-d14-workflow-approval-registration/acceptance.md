# Native workflow response acceptance

- Approve or deny the actual workflow request immediately after its durable
  publication, while private suspension registration is still blocked. The
  response waits; it cannot consume the decision slot early.
- After registration, the suspend owner reports its successful approval handoff
  and the same native workflow resumes. The approved node completes once, or
  records a refused outcome once when denied. Its already completed sibling
  retains exactly its original operation transitions.
- A repeated response returns the existing already-decided result. The workflow
  produces one terminal, without repeated node effects.
- Cancellation during the gap closes the workflow once, cancels the approval,
  leaves its gated node unclaimed, and reports no successful pending handoff.
- Missing approval-store configuration fails the actual workflow handoff,
  produces one failed terminal, and leaves no private pending entry.

The fixture runs the existing native graph and checkpoint owner. Its test-only
context stops the owner's post-publication `Err` check and verifies
`projectionMu` is free, so the responding client can execute normally. No
checkpoint payload is fabricated and no production hook is introduced. The
approve/deny cases check both the original workflow invocation result and its
actual resumed node outcomes.

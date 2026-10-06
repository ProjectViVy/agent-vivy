# Human review of the architecture

Open the [design](../../superpowers/specs/2026-10-07-a2a-server-design.md).

1. Verify that the A2A Module owns protocol adaptation, while VIVY owns
   task/session identity, execution, durable history and approvals.
2. Check the chosen TaskHost operations, authenticated context ownership and
   durable deduplication, including retries without a context ID.
3. Confirm that ordinary remote answers require native atomic acceptance
   and can never become local approval decisions.
4. Decide section 8: approve standard snapshot recovery with future ordered
   updates, or retain exact event replay and require a separately approved
   extension before G0 can close.
5. Confirm PENS remains independent, the server is explicitly selected by
   Recipe, and implementation is not scheduled by accepting a draft.

Acceptance here is review of a concrete architecture proposal. It is not a
claim that an A2A endpoint is available or that issue #2 is implemented.

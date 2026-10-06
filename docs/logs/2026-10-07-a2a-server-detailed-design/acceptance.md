# Detailed design review

The [single design document](../../superpowers/specs/2026-10-07-a2a-server-design.md)
is the deliverable. Reviewers can now trace:

1. A wire message through authenticated TaskHost values into native admission.
2. Missing-context retries and ordinary answers through exact durable receipts
   and crash/rollback outcomes.
3. Committed Journal records into safe task state, history, artifacts and SSE.
4. Configuration into dedicated Host lifecycle, authentication and bounded I/O.
5. Each boundary into a concrete change slice and named acceptance fixture.

G0 remains open for the reconnect guarantee, the actual SDK compatibility
probe and native provisional-session resource lifecycle evidence. Approving
or discussing this document does not itself schedule implementation or close
issue #2.

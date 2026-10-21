# D11 child-session and Work exact-input closure

Child admission and activation now retain normalized actual task/tool inputs in the existing private durable binding and activation receipts. Canonical authority values, existing operation keys, parent owner/run lineage and selected tool ceilings decide reuse; no child request or authority digest is produced or checked. Same-operation changes fail before creating another child activation. Unknown converted inputs are unavailable for reuse, continuation, direct start and checkpoint recovery.

Work requests now retain normalized intended input in a private column of the existing session work event owner. New public event payloads omit it, and ordinary native Work publication clears it explicitly. Work request IDs and owner CAS remain; model identities derive directly from the existing Run/operation/Eino tool-call identity. Goal, submission and plan-pause identifiers derive from those operation identities without SHA computation. Replays can rebuild against a current model cursor while comparing original intent and durable behavior.

No second execution path, runner, receipt store or generic identity framework was introduced. One-shot/continuable distinction, child mailbox sequence/receipt ownership, policy selection, child tool subsets, cancel/drain behavior and Plan review lifecycle stay on their existing owners. Dedicated tests cover same-length task/tool/policy changes, owner and scope conflicts, privacy, restart replay, unavailable input and mailbox/history retention. Related existing fixtures were mechanically adapted to the new fields.

## Eino capability check

The dependency checkpoint pins Eino v0.9.21. Inspected local upstream `adk/runner.go` (`NewRunner`, `CheckPointStore`), `compose/checkpoint.go` (`CheckPointStore`, `WithCheckPointID`) and `compose/tool_node.go` (`GetToolCallID`). Existing Eino ADK execution and compose checkpoint ownership remain unchanged; Work identity continues to consume `compose.GetToolCallID`. These APIs provide execution, interruption and call identity, but do not own Vivy session request receipts or canonical policy ceilings. The minimal product-owned binding stays in its existing storage boundary; no substitute orchestration was added.

## Delivery boundary

This branch owns paired migration 046. Migrations 042/043 belong to the memory/notes lane and 044/045 to the workflow lane; their absence here is intentional and blocks a complete local manifest. The integration owner also changes the shared GoalRoundAdmission field/forwarding and scrubs its direct Goal publication. Full integrated kernel/Face/browser and real PostgreSQL acceptance remain with that owner. No dependency pins, production state, Studio files or UI flows were changed. No release artifact was built in this implementation lane.

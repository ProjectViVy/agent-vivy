# A2A Server detailed design

Expanded the existing [issue #2 design](../../superpowers/specs/2026-10-07-a2a-server-design.md)
in place after the owner requested a detailed design. This preserves one
design authority and leaves implementation unscheduled.

The revision specifies SDK values and wrapper identity, three native
authorization/receipt tables, atomic admission and ordinary answers,
candidate-session preparation, Journal evidence and bounded projection,
JSON-RPC examples, HTTP configuration/security/lifecycle, error mapping,
seven dependency slices and 15 acceptance fixtures.

Detailed source inspection found that new-session Mask/workspace preparation
requires an existing Session, while FrozenCore preparation can write into a
separate native store. The candidate-session path therefore needs an explicit
native provisional-resource lifecycle gate. Core SQL atomicity is not claimed
to cover persona storage or the filesystem. The earlier continuity prompt
snapshot gap is not silently treated as fixed.

The standard snapshot-recovery versus exact event-replay decision is still
open. The dependency slices describe design sequencing, not an executable
implementation plan: issue #2 requires G0 review first.

No runtime source, schema migration, dependency, Recipe or deployed endpoint
was changed. This is a design delivery, so no release/rollout record applies.

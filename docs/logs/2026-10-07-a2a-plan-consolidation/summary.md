# A2A plan consolidation

The owner requested absorbing useful details from the earlier planning branch
before deleting it. The maintained successor is remote branch `A2A`, whose
pre-consolidation head is `3348095f400db4b729a5f7354e3f038296d49484`.
The source is `docs/issue2-a2a-server-plan` at
`99f9b7b2f3a7cab3a87b04c1989a7d5824dc9d10` (ten planning documents).

The existing 2026-10-07 design and package now include a complete old-to-new
Story map, pending scope/default-Generation decisions, the retained optional
exact-event replay contract, and its conditional A2A-R1 implementation plan.
The seven base Stories keep their 18 tasks; A2A-R1 contributes two conditional
tasks. Standard-server milestone acceptance is separate from B acceptance so
the conditional dependency has no cycle or premature G2 completion.

Preserved old-specific details include no-snapshot replay, terminal replay,
sequence/ordinal cursors, same-connection catch-up/tail, durable client
apply-and-cursor persistence, unavailable/deleted history, cursor denial,
revocation, standard-client regression and artifact re-verification. The old
local-answer/single-principal/loopback choices remain explicit alternatives;
consolidation does not silently select a broader first release.

The consolidation commit retains both input commits as parents, while its
file tree contains only documentation changes over the newer head. This keeps
the original source retrievable after branch deletion without maintaining two
active plan trees or merging stale runtime code. The old branch is left for
the owner to delete. No issue mutation, implementation, release or deployment
is included. A standalone release note is unnecessary for planning documents.

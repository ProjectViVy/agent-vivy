# A2A Server architecture draft

The owner requested architecture design for issue #2 after reviewing A2A/PENS
research. The [design](../../superpowers/specs/2026-10-07-a2a-server-design.md)
uses official SDK transport with a custom RequestHandler, a protocol-neutral
optional TaskHost and the existing native Service/Journal authority.

The draft defines module composition, authenticated ownership, atomic
message admission, ordinary input continuation, task projection, HTTP
lifecycle, cancellation, limits, failure behavior and evidence gates.
PENS remains an independent application environment and potential client.

Source inspection exposed three material gaps: generic chat ingress parses
approval commands; ordinary question answers need actor-aware atomic
acceptance before remote exposure; standard A2A reconnect does not provide
the issue's exact durable-cursor replay guarantee. The draft recommends
snapshot convergence plus future ordered events and leaves that acceptance
change explicitly subject to owner review.

The historical CH-C9 note now points to the draft. TODO tracks design review;
DEFER retains unscheduled implementation and NeuroLink. This delivery adds no
runtime code, public API, migration, dependency or Recipe. It is not a release,
so there is no release record or product rollout.

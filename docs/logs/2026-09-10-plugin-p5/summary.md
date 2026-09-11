# PLG-P5 Summary

PLG-P5 was manually scheduled by the repository owner on 2026-09-10. This
iteration makes `std/provider-profile@v1` declarative, routes supported model
execution through one internal ModelHost, registers the verified first-party
profiles in the default Generation, and projects supported and deferred
capability truth without exposing Secrets.

The only executable adapter families in this phase are the repository-pinned
EinoExt OpenAI-compatible and Claude components. Provider OAuth, Azure OpenAI,
Anthropic Bedrock, Anthropic Vertex, and unpinned native vendor protocols remain
`DEFERRED-INDEFINITE`; this iteration adds no substitute implementation.

P3, P4, P6, P7, P8, and P9 are outside this delivery.

The delivered runtime has one ModelHost, declarative first-party `openai` and
`anthropic` Profiles in the default Generation, exact raw model-ID forwarding,
and a Secret-free capability projection shared by RPC and UI. Profiles move
through `COMPILED`, `UNCONFIGURED`, `READY`, or `UNAVAILABLE`; missing pinned
families remain `DEFERRED-INDEFINITE` and cannot be selected by either client
or server paths.

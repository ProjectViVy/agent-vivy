# PLG-P5 Acceptance

A reviewer can accept this phase because:

1. Generation inspection lists the compiled Provider Profiles and source
   descriptors; runtime status lists adapter families and supported/deferred
   state without Secret values.
2. OpenAI-compatible and Anthropic configurations retain their existing forms
   and raw model IDs.
3. A missing credential is reported as unconfigured; it does not trigger a
   network probe or silent fallback.
4. A deferred family is visible but cannot be selected as executable and does
   not synthesize an endpoint.
5. Focused Provider/Profile/ModelHost tests, default and minimal Generation
   pack/inspect smokes, the full repository gate, and authorized fake/local
   provider smoke all pass. `just ci` itself is unavailable on this Linux
   runner, so its constituent checks were run directly and passed.

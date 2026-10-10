# R2 acceptance

- Generate accepts period + current/completed window + stable operation key + optional target; never submits prompt/graph/scope/tools/body. Wired: `generate` input pinned by client test; backend schema `additionalProperties:false`.
- Controls render only while `vivy.reports.*` answer; absent generation hides the panel (e2e test 3, unit test).
- Progress by Run ID re-read of `vivy.reports.get`; reconnect issues the same bounded read — no failure inferred from an interrupted socket (e2e test 2).
- Generated vs edited distinction; `View generated version`/`Use this version` adopt through N1 expected-version CAS; stale adoption hits the existing conflict flow.
- Feedback labeled `included` only when anchored to a generated revision id — never inferred from body text.
- No source-validation badge on edited prose; no scheduling claims; EN/ZH copy.

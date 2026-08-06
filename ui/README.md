# ui

The browser-based UI shell (Vite), built against the Vivy HTTP API and SSE
event stream only (FR-9, D-013).

Ground rules:

- Calls ONLY the Vivy HTTP API. Never imports Go types, Eino types, or
  reference-project types (D-007).
- Consumes real session/run/tool/approval/recovery states. No mock domain
  records in any path exercised by users (PRD §6.2, RK-5).
- Supports session list, message input, streaming display, approval
  interactions, run status, error display, and refresh-without-state-loss
  (AS-7).

Skeleton stage: empty. Scaffolded in task D3, smoke-tested in D4
(Playwright against the real Go process).

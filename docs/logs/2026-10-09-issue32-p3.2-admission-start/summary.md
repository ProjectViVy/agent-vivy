# P3.2 backend slice: workflow admission and start-session contract

The server now rejects duplicate child tool names at schema and host admission, refuses to publish those definitions, keeps malformed historical publications readable but blocks their start, and checks that a prepared start request's captured `session_id` matches the server-bound session.

The workflow UI bridge and editor retry-intent work remains open. The repository routes Web UI Module implementation through `oil-frontend`; `.agents/skills/oil-frontend/` is empty in this checkout, and the available `testing-vivy-ui` skill covers browser execution rather than implementation guidance. No Module source was edited.

The Module identity, Provider/Consumer, Port, authority owner, grants, Recipe and lifecycle record is in [P3.1 summary](../2026-10-09-issue32-p3.1-workflow-draft-cas/summary.md). Backend interfaces remain the existing INOFY admission and durable operation-dedup path.

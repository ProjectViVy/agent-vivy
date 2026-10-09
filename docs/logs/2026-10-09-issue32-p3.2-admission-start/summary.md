# P3.2 backend slice: workflow admission and start-session contract

The server now rejects duplicate child tool names at schema and host admission, refuses to publish those definitions, keeps malformed historical publications readable but blocks their start, and checks that a prepared start request's captured `session_id` matches the server-bound session.

## UI continuation

`oil-frontend` is still absent. Under the user's explicit instruction to continue, the planned UI behavior was implemented with existing Module conventions and `testing-vivy-ui`, without visual redesign.

The bridge now prepares a deeply snapshotted, frozen start request with captured session, parent Run, source, input and UUID operation ID; retries forward it unchanged and validate required fields/source selector/128-byte operation-ID bound. Editor actions confirm each successful save before validate, publish or start. Lost start replies retain the request for retry without a second save. Editing the source clears it so the next start saves and creates a new operation. Published revision rows retain pending requests by workflow and revision.

The Module source hash is `26a119336563334d0ae75a478734730caa614085794a242fc2442453e05fbd18`. Verification and commit details are in `verification.md`; candidate browser acceptance remains part of P7.

The Module identity, Provider/Consumer, Port, authority owner, grants, Recipe and lifecycle record is in [P3.1 summary](../2026-10-09-issue32-p3.1-workflow-draft-cas/summary.md). Backend interfaces remain the existing INOFY admission and durable operation-dedup path.

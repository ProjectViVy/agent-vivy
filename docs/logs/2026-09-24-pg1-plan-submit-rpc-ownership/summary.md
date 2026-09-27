# PG-1 Plan Submission RPC Ownership

Public JSON-RPC clients can no longer create `plan.submitted` events by supplying caller-chosen origin IDs. The `plan/submit` route and its corresponding UI method declarations were removed; model submissions remain owned by the runtime tool path, which derives origin data from the active Eino tool context. The RPC mutation builder also rejects `PlanSubmitted` as a model-only operation.

Added a handler-level regression test with a same-session active primary run and forged ToolCall ID. It verifies the request is rejected and both the durable work state and replay remain unchanged. The test harness now gives its runtime service the SQLite WorkStore required to exercise this path.

Not changed: model-tool submission behavior, human Plan review transitions, Eino versions, or user-visible UI. Browser smoke and live PostgreSQL conformance were not available in this environment; see `verification.md`.

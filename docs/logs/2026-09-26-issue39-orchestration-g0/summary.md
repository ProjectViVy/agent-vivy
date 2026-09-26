# Issue 39 ORCH-01 recovery and native proof — 2026-09-26

## Result

**G0: BLOCKED.** The implementation and local verification are present in the worktree, but the repository `just ci` recipe could not run because `just` and PowerShell are not installed. PostgreSQL DSN-backed conformance is also unverified because `VIVY_POSTGRES_TEST_DSN` is unset. These are missing gates, not evidence of Eino incompatibility.

## Implemented

- Added a run-scoped durable `ToolOperationStore` with paired SQLite/PostgreSQL migration 026. Admission, claim, completion/failure and the D15 `tool.operation` Journal lifecycle event share a transaction. The SQL row retains invocation bytes for recovery; the `tool.operation` event contains digests and lifecycle metadata, not raw invocation arguments. Existing `tool.requested` and approval-required event payloads retain their pre-existing contracts and redaction/projection behavior.
- Routed ordinary and approval-governed tool calls and proof-node operations through this boundary. Recovery rechecks current pre-tool middleware against the original middleware input and blocks on changed effective arguments. Unknown claimed outcomes cannot be replayed automatically.
- Preserved policy authority across approval: a current `PolicyDeny` rejects both initial approved execution and replay of a previously completed operation; approval satisfies `PolicyPrompt` only.
- Added a bounded Service-owned Eino Workflow proof with parallel A/B work, an explicit join/output mapping, approval pause while a sibling continues, cancellation, same-checkpoint resume and prompt/engine identity checks.
- Added a separate-process crash matrix with an external durable effect fixture across four crash boundaries. Completed results are reused; uncertain claimed outcomes remain blocked without another effect.

## Verification

The full repository Go suite passed before the final broker-policy fix. Post-fix focused runtime/app/storage tests, provider conformance, `go vet` and UI checks pass. A post-fix full-suite rerun was blocked by automatic review when a test attempted an unapproved request to `api.deepseek.com`; it was not retried. The exact commands and crash evidence are in [verification](verification.md).

No public child or DAG product surface has shipped. ORCH-02 through ORCH-08 remain gated, PostgreSQL schema acceptance remains unverified, and no external exactly-once guarantee is claimed. No commit was created; repository `AGENTS.md` prohibits AI-authored commits.

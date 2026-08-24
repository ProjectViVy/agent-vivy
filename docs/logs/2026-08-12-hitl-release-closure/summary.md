# HITL Release Closure Summary

Date: 2026-08-12
Status: complete

## Outcome

The P0 HITL Review Center is now exercised through the real Go process and
embedded UI with deterministic local fixtures. The mandatory path covers
approval, question, narrow-screen review, keyboard focus, durable response,
and run completion without a live provider or external network dependency.

## Delivered

- Added `runtime.mock_scenario`, a test-only deterministic Eino tool-calling
  provider mode. The `hitl` scenario drives approval and question calls from
  the same app/runtime/RPC path used in production.
- Added real Playwright coverage for Review Center discovery, approval,
  question answering, responsive layout, focus traversal, reload parity, and
  session management.
- Kept Review Center and inline inspector on the shared review-card renderer.
  Opening Review Center now makes it the active work surface so a narrow
  inspector cannot intercept review actions.
- Answer input now enables its submit action immediately while preserving the
  draft across rerenders.
- Preserved Browser Use exclusion and GraphTool's test-only boundary.

## Explicitly deferred

Proposal editing, scoped remember/allow policies, structured MCP elicitation,
reviewer assignment, history/search, external notifications, and bulk approval
remain P1 work.

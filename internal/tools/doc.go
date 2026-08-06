// Package tools owns the Vivy ToolSpec registry and approval policy.
//
// V0 registers exactly two tools (D-012): one read-only tool that executes
// automatically (still emitting tool.started/tool.finished), and one
// effectful tool that emits tool.approval_required and must not execute
// until a server-side Approval record exists for the exact run_id +
// tool_call_id (FR-6, FR-7, D-009). Registration is Vivy-owned, not Eino's.
//
// Skeleton stage: empty. Implemented in tasks C5/C6/D2.
package tools

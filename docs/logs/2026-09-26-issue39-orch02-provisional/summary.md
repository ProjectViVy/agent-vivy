# Issue 39 provisional implementation summary — 2026-09-27

ORCH-02–07 implementation is present in the worktree. The changes add durable explicit child modes and mailbox state, native Service/Eino child activation, bounded immutable workflow revisions, native Eino Workflow execution, model-facing `workflow` and `reply_parent` tools, child/workflow RPC methods, and localized RunInspector projections.

Direct parent-child messages are addressed by stable ChildSession, use recipient-scoped order and durable receipts, and are consumed at safe points with at-least-once delivery. Child replies enter the next active parent Run's bounded model input and advance the parent inbox cursor on turn completion; `child_inbox` offers an explicit read/acknowledge path. Parent reauthorization within the origin Session preserves mailbox access, while historical child inspection remains scoped to that origin Session after Run/ChildSession completion. Each recipient can admit at most 128 messages per ChildSession and each body is limited to 32 KiB. Workflow nodes are one-shot/read-only and bounded by descriptor and output limits; explicit one-shot Service calls retain the existing parent-authorized approval path.

The prior independent review's eight Important findings have received one regression-tested fix pass. Focused Go tests, `go vet`, command builds, UI typecheck and Vitest pass. SQLite storage conformance passes. See [verification.md](verification.md) for exact commands and [acceptance.md](acceptance.md) for requirement-by-requirement blockers.

Acceptance and release remain blocked on PostgreSQL DSN-backed tests, repository `just ci`, the integrated R1–R14/G0–G4 matrix, descendant resource-accounting evidence, and real host/browser E2E. No product surface has been released.

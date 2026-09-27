# ND-0: pin nudge Eino integration contracts

2026-09-23. Issue #58; Story ND-0 of docs/plans/nudge; baseline branch
docs/issue58-nudge-design @ 5b093c7f.

Delivered `internal/runtime/nudge_contract_test.go`: a test-only proof that
the pinned Eino v0.9.13 surfaces support the NUDGE-DESIGN §4/§6 ordering and
metadata contract without a second runtime. Ten subtests cover both adapter
flavors, Generate and Stream paths, durability ordering, failure-metadata
correlation, journal-failure Abort, cancellation, provider retry parity,
resume with a fresh state, and compaction-vs-wrapper ordering.

Key result: a WrapModel-installed request barrier can hold the inner model
out until every tool result in the input-tail batch is durably journaled,
while Eino still yields finished tool results to the consumer — no
deadlock. The baseline (no barrier) shows the engine entering the inner
model before durability, confirming the barrier is needed, not redundant.

One upstream finding: Eino v0.9.13 `compose/tool_node.go:1253` writes a
function-scope `err` inside the enhanced tool-result converter, so any
parallel enhanced tool batch races under `-race`. Orthogonal to the nudge
design; recorded in docs/TODO.md and verification.md.

No production code changed. ND-0 passing is not product acceptance; it
releases ND-2's design assumptions.

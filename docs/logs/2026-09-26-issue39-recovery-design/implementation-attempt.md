# Issue 39 implementation attempt — 2026-09-26

This attempt implements the first D15 recovery slice after the architecture revision. ORCH-01 Tasks 1–4 are recorded complete in the worktree ledger. G0 remains BLOCKED; the previous NO-GO artifact is unchanged.

## Implemented

- Added a run-scoped `ToolOperationStore` contract and paired SQLite/PostgreSQL migrations. Admission, atomic claim, completion result/failure and the corresponding Journal event are committed together. Reusing a key with different arguments conflicts; completed operations return the persisted resolution; claimed unknown outcomes are not reclaimed.
- Routed ordinary Service tools, approved worker broker calls, and the fixed proof-node identity through the durable operation boundary. Worker completion-event persistence errors now propagate.
- Added a bounded Service-owned Eino Workflow proof with independent A/B nodes, an explicit join, an order-only dependency and a stable workflow Run/checkpoint/prompt scope. Typed checkpoint values use registered schema names. The proof interrupts after both node outputs are durable and resumes under the same Run and prompt.
- Updated event schemas, migration/conformance expectations and approval/search tests for the new operation events.

The node body in this proof returns its task string. This verifies Eino scheduling, Service operation admission and checkpoint binding; it is not yet a model-backed child activation, an agent-authored arbitrary DAG, or the integrated approval path.

## Verification

- `go test ./internal/storage/sqlite ./internal/storage/postgres ./internal/storage/migrations ./internal/domain -count=1 -timeout=120s`: PASS. `VIVY_POSTGRES_TEST_DSN` is unset, so this is not PostgreSQL conformance evidence.
- `go test ./internal/runtime -count=1 -timeout=180s`: PASS, using Go 1.26.4 selected through `/usr/lib/go-1.24/bin/go` with that directory on `PATH`.
- `go test ./internal/app -run '^$' -count=1 -timeout=120s`: compile-only PASS with a temporary `ui/dist/index.html` embed fixture; the fixture was removed. No full app test suite was run.
- `git diff --check`: PASS.
- `just ci`: NOT RUN; `just` and PowerShell are unavailable in this Linux workspace.

## Remaining G0 evidence

The integrated Service approval interrupt while a sibling continues, cancellation and orphan-activation checks, invalid engine/prompt rejection, and the separate-process crash matrix (before claim, after claim, after effect, and after completion before graph checkpoint) remain open. PostgreSQL DSN-backed conformance also remains open. No exactly-once external-effect or automatic-continuation claim is made.

No commit was created. Repository `AGENTS.md` prohibits AI-authored commits; the implementation remains reviewable in the current worktree.

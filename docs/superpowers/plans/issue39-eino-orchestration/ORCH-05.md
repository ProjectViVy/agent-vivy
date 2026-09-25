# ORCH-05 — finite graph contract and immutable revision

> **For implementers:** REQUIRED SUB-SKILL: `superpowers:executing-plans` or explicitly assigned `superpowers:subagent-driven-development`.

**Goal:** Validate a bounded agent-authored DAG and persist an immutable revision before any node runs.
**Architecture:** A host-owned pure descriptor/validator and SQL admission transaction; execution stays in ORCH-06's Eino Workflow.
**Tech Stack:** Go, paired SQLite/Postgres migrations and conformance tests.
**Spec:** [architecture](../../specs/2026-09-23-issue39-eino-orchestration-design.md), R4/R6; [index](index.md) predecessor ORCH-02.
**Review focus:** bounds and authority cannot be bypassed by text, duplicate node keys, topological order or retry; description does not become a second execution state.

## Files and interface

NEW `internal/orchestration/{descriptor,validation}.go` and tests (pure domain, **no Eino imports**). Add `internal/storage/{sqlite,postgres}/workflow_revisions.go` and paired `internal/storage/migrations/{sqlite,postgres}/NNN_workflow_revisions.sql`; modify `internal/storage/contracts.go`, conformance suite, `internal/runtime/service.go` only for admission wiring. `WorkflowRevision{ID,Digest,ParentRunID,Nodes,Edges}` and `TaskNode{Key,Task,MaskHint}`, `Dependency{From,To,OutputKey}` from design are sketches. Persist canonical schema version, immutable JSON, digest, parent authority digest, parent/run lineage, admission operation key. Reuse existing SnapshotStore/Journal where possible; introduce a table only for facts not representable safely with existing stores. If ORCH-02 chose migration 026, next is 027; decide at rebased tip, not a hard-coded filename.

## Tasks

- [ ] Write table tests: empty graph, duplicate/unknown keys, self-edge, cycle, unreachable node, unconsumed node/output, oversized task/output, finite node/depth/width limits, conflicting operation ID and canonical digest. Accepted nodes must be reachable from start and consumed by workflow output/end. Document actual Service caps.
- [ ] Implement deterministic key ordering, topology/output-consumption validation and authority check. Parent model only; tools may narrow immutable ceiling. Outputs are explicit bounded mappings. Reject before Run. No retries/loops/dynamic mutation.
- [ ] Implement paired migrations and read/append-only revision storage. Make `(parent_run_id, operation_id)` unique with payload digest conflict handling and immutable descriptor bytes. Create workflow Run (`RunKindChild` with graph descriptor binding, unless a separate kind is justified in code review), preserving parent/root lineage. Journal proposal accepted/start only after descriptor and Run persist.
- [ ] Test two concurrent identical submissions across backend instances; one revision/Run and no nodes. Test parent deletion racing admission. Run `go test ./internal/orchestration ./internal/storage/... -run 'Workflow|Graph|DeleteSession' -count=1` plus `just ci` with actual Postgres conformance.

## Failure and handoff

Return structured validation errors with node/edge paths; reject mutated same-ID revisions. If storage commit fails, no active graph Run is exposed. Existing graph reads must remain possible after future schema evolution. Evidence is canonical digest vectors, backend uniqueness/deletion tests and Run/Journal sequence. ORCH-06 consumes a validated descriptor, never raw model text.

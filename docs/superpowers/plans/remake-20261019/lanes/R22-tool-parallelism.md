# R22: Same-batch tool parallelism intake plan

> For execution: read the accepted design and this plan, then use `superpowers:executing-plans` in one isolated write lane. Delegation is not selected by this package.

**Goal:** Restore the retained observable behavior for `tool-parallelism` with the smallest compatible delta.
**Architecture:** Consume the accepted core and its existing typed boundaries. Use donor behavior and tests as evidence; rewrite only incompatible glue. No whole old-main merge.
**Tech stack:** Go 1.26.4, repository-selected Eino/EinoExt and Laputa/INOFY; source UI uses the repository's React/TypeScript toolchain where selected.
**Spec:** [Remake design](../../../specs/2026-10-19-remake-design.md).
**Status:** Blocked for product implementation. These are bounded intake sub-batches, not falsely Ready final code tasks.
**Branch:** `remake/tool-parallelism`. **Worktree:** `<lanes-root>/tool-parallelism/agent-vivy-remake-tool-parallelism` with its own sibling `laputa/`.
**Immediate predecessors:** R02, R03. The index is the sole state/dependency authority.

## Inputs and outputs

Consumes: accepted core SHA B, exact dependency pins, approved retained-hunk map, R03's applicable schema/revision/approval/Assembly contracts and any explicitly selected feature predecessor.
Produces: one feature delta, explicit selected/omitted behavior if optional, source-first generated projections, exact-candidate evidence and an integration receipt. Public target signatures are intentionally not invented before R02; freeze them in the lane's refined implementation plan before coding.

## Governing issue requirements (synchronized 2026-10-20)

- [#91](https://github.com/ProjectViVy/agent-vivy/issues/91) owns the stable loop and one execution owner. Check pinned native Eino dispatch before custom scheduling; preserve call/result, approval, ownership and crash-safety contracts.
- [#93](https://github.com/ProjectViVy/agent-vivy/issues/93) owns EXP extraction. Nudge's hard stop moves with its package; preserve ordinary MaxToolTurns/Eino caps and never run core plus package implementations together.
- [#83](https://github.com/ProjectViVy/agent-vivy/issues/83) coordinates re-intake after the core seam. Performance claims need comparable measurement on the exact candidate; this donor is no speedup evidence.

remake/tool-parallelism is raw donor source, not a second runner or Eino repin authorization.


## Verified donor entry points

- `internal/runtime/parallel_tools_test.go` — parallel-tool donor.
- `internal/runtime/tool_batch_order.go` — parallel-tool donor.
- `internal/runtime/tool_batch_order_test.go` — parallel-tool donor.
- `internal/runtime/work_control_tools.go` — main donor.
- `internal/runtime/plan_review.go` — main donor.

Donor anchors: `559f21a5`, `2a9ea09b`. Resolve full SHAs from the source inventory or frozen repository before execution. A donor anchor may be a mixed merge; it is not a cherry-pick instruction.

## Ordered sub-batches

- T1: Recheck native Eino v0.9.21 parallel dispatch and accepted ownership limits before custom code.
- T2: Admit only necessary work-tool ordering and exclusion barriers at the existing run boundary.
- T3: Resume same-model-turn batches and repeated sibling approvals without nudge-owned completion barriers.

## Execution checklist

- [ ] Read R01's hunk dispositions and compare each retained behavior against B; classify already-covered behavior without restoring its old code.
- [ ] Resolve this lane's exact target blocker: Source paths exist only on feat/parallel-tool-dispatch where noted. Core causal loop/approval ownership must be accepted; custom ordering is justified only by a demonstrated native API gap.
- [ ] Replace this blocked card with executable per-sub-batch steps naming the selected files, actual target API signatures, exact tests, prepared commands, failure paths and expected outputs. Reuse the linked original plans for intent; no copied Done/Pass status.
- [ ] Reuse a meaningful existing regression; add only missing behavior/failure coverage and establish the intended failing assertion on the candidate.
- [ ] Port the smallest retained code and adapt to B. Keep locks/pins/migrations/generated files under the integration ownership in R03.
- [ ] Run the focused checks and applicable selected/omitted real path in acceptance.md; then required combined-candidate gates. Record skips as pending.
- [ ] Independently review the changed boundary and integrate the exact reviewed delta through `remake/integration`; invalidate affected evidence if the candidate changes.

## Acceptance cases owned by this lane

- An isolated deterministic barrier probe shows ordinary same-batch tools overlap.
- A work-tool commit drains earlier siblings and fences later effects in request order.
- Consecutive approvals/resume do not deadlock, duplicate effects or invoke a second runner.
- Record comparable latency and allocations on the exact candidate; no unmeasured performance claim.

## Source test discovery and verification

In the pinned donor, use `rg '^func Test'` on the verified test paths or package paths above; in B confirm which tests still apply before selecting them. Then run `go vet <affected packages>` and `go test -count=1 -timeout 35m <affected packages> -run '<accepted test selection>'` after the target's documented asset/dependency preparation. UI changes use the target's existing typecheck/test/build and real-backend smoke entry. Placeholders in this paragraph are a refinement gate, not commands to execute unchanged. Precise commands for the first low-risk pilot are in R04.

## Exclusions and rollback

Do not transplant the obsolete nudge barrier, forced Eino repin or a new concurrency service.

Rollback is to the preceding accepted binary/source candidate with the declared compatible data format. A source revert is not a safe data downgrade. Preserve failed/unknown side-effect outcomes and never blindly replay tools. Newer or unsupported data must be rejected before writes. No live user store is used for validation.

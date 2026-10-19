# R15: Durable BML memory and native operations intake plan

> For execution: read the accepted design and this plan, then use `superpowers:executing-plans` in one isolated write lane. Delegation is not selected by this package.

**Goal:** Restore the retained observable behavior for `bml` with the smallest compatible delta.
**Architecture:** Consume the accepted core and its existing typed boundaries. Use donor behavior and tests as evidence; rewrite only incompatible glue. No whole old-main merge.
**Tech stack:** Go 1.26.4, repository-selected Eino/EinoExt and Laputa/INOFY; source UI uses the repository's React/TypeScript toolchain where selected.
**Spec:** [Remake design](../../../specs/2026-10-19-remake-design.md).
**Status:** Blocked for product implementation. These are bounded intake sub-batches, not falsely Ready final code tasks.
**Branch:** `remake/bml`. **Worktree:** `<lanes-root>/bml/agent-vivy-remake-bml` with its own sibling `laputa/`.
**Immediate predecessors:** R02, R03. The index is the sole state/dependency authority.

## Inputs and outputs

Consumes: accepted core SHA B, exact dependency pins, approved retained-hunk map, R03's applicable schema/revision/approval/Assembly contracts and any explicitly selected feature predecessor.
Produces: one feature delta, explicit selected/omitted behavior if optional, source-first generated projections, exact-candidate evidence and an integration receipt. Public target signatures are intentionally not invented before R02; freeze them in the lane's refined implementation plan before coding.

## Governing issue requirements (synchronized 2026-10-20)

- [#4](https://github.com/ProjectViVy/agent-vivy/issues/4) remains open and unscheduled. VIVY owns authorized scope/admission/budgets; providers own storage/indexing/ranking. Validate BML first and Mentle through DIVA. No implicit backend switch, dual write, store migration or cascading deletion; keep one subject-level ACTMEM and explicit targeted forgetting.
- [#60](https://github.com/ProjectViVy/agent-vivy/issues/60) is a separate optional offload/recall experiment and may not be implemented unless selected. Reuse existing long-result handling where enough; do not replace compression or create another continuity engine.
- [#83](https://github.com/ProjectViVy/agent-vivy/issues/83) coordinates re-intake against #91 contracts. BML2 is the target format; old bytes do not authorize restoring old schemas/hash behavior.

remake/bml is raw donor source, not acceptance of #4 or #60.


## Verified donor entry points

- `bml/bound.go` — main donor.
- `bml/bound_read.go` — main donor.
- `bml/store.go` — main donor.
- `bml/home.go` — main donor.
- `internal/modules/memory/binding.go` — main donor.
- `plugins/vivy-memory/ui/vivy-memory/src/page.tsx` — main donor.

Donor anchors: `5770f07f`, `d0f88f3e`, `74f8788a`, `3f547b58`, `a44b0f69`, `cf8f9bc6`, `25915b32`, `64cd48bf`, `88200c8b`. Resolve full SHAs from the source inventory or frozen repository before execution. A donor anchor may be a mixed merge; it is not a cherry-pick instruction.

## Ordered sub-batches

- B1: Compare accepted #91 BML format and Home ownership against native-operation donors.
- B2: Port only missing scoped operation/replay/tombstone behavior.
- B3: Adapt #88 disk-backed lifecycle/model-consumption and management UI acceptance; joint Recap sink checks only where selected.

## Execution checklist

- [ ] Read R01's hunk dispositions and compare each retained behavior against B; classify already-covered behavior without restoring its old code.
- [ ] Resolve this lane's exact target blocker: Do not replace #91 BML2 with the old format. Native-operation adapter compatibility and the selected Laputa Home contract must be named in R03.
- [ ] Replace this blocked card with executable per-sub-batch steps naming the selected files, actual target API signatures, exact tests, prepared commands, failure paths and expected outputs. Reuse the linked original plans for intent; no copied Done/Pass status.
- [ ] Reuse a meaningful existing regression; add only missing behavior/failure coverage and establish the intended failing assertion on the candidate.
- [ ] Port the smallest retained code and adapt to B. Keep locks/pins/migrations/generated files under the integration ownership in R03.
- [ ] Run the focused checks and applicable selected/omitted real path in acceptance.md; then required combined-candidate gates. Record skips as pending.
- [ ] Independently review the changed boundary and integrate the exact reviewed delta through `remake/integration`; invalidate affected evidence if the candidate changes.

## Acceptance cases owned by this lane

- Write -> close/reopen -> get/search returns durable data.
- Correction/delete and supported same-record replay do not duplicate or resurrect records.
- The subsequent captured model request consumes the expected memory.
- Management UI agrees with the actual backend; foreign scope and negative budgets are refused.

## Source test discovery and verification

In the pinned donor, use `rg '^func Test'` on the verified test paths or package paths above; in B confirm which tests still apply before selecting them. Then run `go vet <affected packages>` and `go test -count=1 -timeout 35m <affected packages> -run '<accepted test selection>'` after the target's documented asset/dependency preparation. UI changes use the target's existing typecheck/test/build and real-backend smoke entry. Placeholders in this paragraph are a refinement gate, not commands to execute unchanged. Precise commands for the first low-risk pilot are in R04.

## Exclusions and rollback

No invented exactly-once memory_add semantics, broad memory-provider matrix or DIVA-installed acceptance prerequisite.

Rollback is to the preceding accepted binary/source candidate with the declared compatible data format. A source revert is not a safe data downgrade. Preserve failed/unknown side-effect outcomes and never blindly replay tools. Newer or unsupported data must be rejected before writes. No live user store is used for validation.

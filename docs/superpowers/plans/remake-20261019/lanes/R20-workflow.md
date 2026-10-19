# R20: Existing INOFY workflow correctness intake plan

> For execution: read the accepted design and this plan, then use `superpowers:executing-plans` in one isolated write lane. Delegation is not selected by this package.

**Goal:** Restore the retained observable behavior for `workflow` with the smallest compatible delta.
**Architecture:** Consume the accepted core and its existing typed boundaries. Use donor behavior and tests as evidence; rewrite only incompatible glue. No whole old-main merge.
**Tech stack:** Go 1.26.4, repository-selected Eino/EinoExt and Laputa/INOFY; source UI uses the repository's React/TypeScript toolchain where selected.
**Spec:** [Remake design](../../../specs/2026-10-19-remake-design.md).
**Status:** Blocked for product implementation. These are bounded intake sub-batches, not falsely Ready final code tasks.
**Branch:** `remake/workflow`. **Worktree:** `<lanes-root>/workflow/agent-vivy-remake-workflow` with its own sibling `laputa/`.
**Immediate predecessors:** R02, R03. The index is the sole state/dependency authority.

## Inputs and outputs

Consumes: accepted core SHA B, exact dependency pins, approved retained-hunk map, R03's applicable schema/revision/approval/Assembly contracts and any explicitly selected feature predecessor.
Produces: one feature delta, explicit selected/omitted behavior if optional, source-first generated projections, exact-candidate evidence and an integration receipt. Public target signatures are intentionally not invented before R02; freeze them in the lane's refined implementation plan before coding.

## Verified donor entry points

- `internal/runtime/workflow_product.go` — main donor.
- `internal/runtime/inofy_admission.go` — main donor.
- `internal/rpc/inofy_product.go` — main donor.
- `internal/storage/workflow_definitions.go` — main donor.
- `plugins/vivy-workflow/ui/vivy-workflow/src/editor/NodePanel.tsx` — main donor.
- `plugins/vivy-workflow/ui/vivy-workflow/src/launch-context.ts` — main donor.

Donor anchors: `64a3196c`, `0de913ac`, `8b08b1ac`, `3ffec313`, `84bfa8fd`, `dcb7a1b2`, `e083a141`. Resolve full SHAs from the source inventory or frozen repository before execution. A donor anchor may be a mixed merge; it is not a cherry-pick instruction.

## Ordered sub-batches

- W1: Draft CAS, paging/status and canonical node.inputs authoring.
- W2: Author -> save -> publish -> run consumes the same inputs.
- W3: Active-parent launch, retry intent and truthful idle/history availability.

## Execution checklist

- [ ] Read R01's hunk dispositions and compare each retained behavior against B; classify already-covered behavior without restoring its old code.
- [ ] Resolve this lane's exact target blocker: Use accepted hash-free INOFY contract/pin from #91 D11. #32 and #90 are overlapping donors, not two writers.
- [ ] Replace this blocked card with executable per-sub-batch steps naming the selected files, actual target API signatures, exact tests, prepared commands, failure paths and expected outputs. Reuse the linked original plans for intent; no copied Done/Pass status.
- [ ] Reuse a meaningful existing regression; add only missing behavior/failure coverage and establish the intended failing assertion on the candidate.
- [ ] Port the smallest retained code and adapt to B. Keep locks/pins/migrations/generated files under the integration ownership in R03.
- [ ] Run the focused checks and applicable selected/omitted real path in acceptance.md; then required combined-candidate gates. Record skips as pending.
- [ ] Independently review the changed boundary and integrate the exact reviewed delta through `remake/integration`; invalidate affected evidence if the candidate changes.

## Acceptance cases owned by this lane

- Published inputs reach the actual workflow node.
- A stale draft cannot overwrite a newer revision and keeps the user draft.
- Active context launch uses valid parent ownership; idle/history state is clearly unavailable where the contract forbids independent launch.
- Pagination and engine lifecycle agree with real backend state.

## Source test discovery and verification

In the pinned donor, use `rg '^func Test'` on the verified test paths or package paths above; in B confirm which tests still apply before selecting them. Then run `go vet <affected packages>` and `go test -count=1 -timeout 35m <affected packages> -run '<accepted test selection>'` after the target's documented asset/dependency preparation. UI changes use the target's existing typecheck/test/build and real-backend smoke entry. Placeholders in this paragraph are a refinement gate, not commands to execute unchanged. Precise commands for the first low-risk pilot are in R04.

## Exclusions and rollback

No fabricated parent run, permission bypass, new workflow engine, new graph types, triggers or wait/resume expansion.

Rollback is to the preceding accepted binary/source candidate with the declared compatible data format. A source revert is not a safe data downgrade. Preserve failed/unknown side-effect outcomes and never blindly replay tools. Newer or unsupported data must be rejected before writes. No live user store is used for validation.

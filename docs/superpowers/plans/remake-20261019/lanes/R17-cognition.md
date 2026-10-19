# R17: Cognitive capture, Mission and restart correctness intake plan

> For execution: read the accepted design and this plan, then use `superpowers:executing-plans` in one isolated write lane. Delegation is not selected by this package.

**Goal:** Restore the retained observable behavior for `cognition` with the smallest compatible delta.
**Architecture:** Consume the accepted core and its existing typed boundaries. Use donor behavior and tests as evidence; rewrite only incompatible glue. No whole old-main merge.
**Tech stack:** Go 1.26.4, repository-selected Eino/EinoExt and Laputa/INOFY; source UI uses the repository's React/TypeScript toolchain where selected.
**Spec:** [Remake design](../../../specs/2026-10-19-remake-design.md).
**Status:** Blocked for product implementation. These are bounded intake sub-batches, not falsely Ready final code tasks.
**Branch:** `remake/cognition`. **Worktree:** `<lanes-root>/cognition/agent-vivy-remake-cognition` with its own sibling `laputa/`.
**Immediate predecessors:** R02, R03. The index is the sole state/dependency authority.

## Inputs and outputs

Consumes: accepted core SHA B, exact dependency pins, approved retained-hunk map, R03's applicable schema/revision/approval/Assembly contracts and any explicitly selected feature predecessor.
Produces: one feature delta, explicit selected/omitted behavior if optional, source-first generated projections, exact-candidate evidence and an integration receipt. Public target signatures are intentionally not invented before R02; freeze them in the lane's refined implementation plan before coding.

## Governing issue requirements (synchronized 2026-10-20)

- [#32](https://github.com/ProjectViVy/agent-vivy/issues/32) closed as superseded, not fixed. [#83](https://github.com/ProjectViVy/agent-vivy/issues/83) retains C1-C7 for cognition/Recap/core intake: preserve admitted input, Mission/run authority, CAS, restart identity, scope isolation and truthful recovery.
- [#72](https://github.com/ProjectViVy/agent-vivy/issues/72) sets DIVA local Skill as the default evolution receiver; EvoMap remains unfinished/default-off. Missing receivers cannot be advertised or return false success; reject stale revisions and avoid blind replay.
- [#7](https://github.com/ProjectViVy/agent-vivy/issues/7) strategy composition remains separate/unscheduled. [#6](https://github.com/ProjectViVy/agent-vivy/issues/6) AutoDream is candidate-only; reflection itself cannot mutate memory, Skill, ACTMEM or persona.
- Keep nudge/warming/steering in [#93](https://github.com/ProjectViVy/agent-vivy/issues/93).

remake/cognition is raw donor source; select only gaps after #91 target contracts are accepted.


## Verified donor entry points

- `internal/runtime/cognitive_service.go` — main donor.
- `internal/runtime/cognitive_binding.go` — main donor.
- `internal/modules/diva-cognitive/factory.go` — main donor.
- `internal/app/memory_loop_fixture_test.go` — main donor.
- `internal/app/memory_loop_recall_test.go` — main donor.

Donor anchors: `f26e8d44`, `b69a4b83`, `93ac1800`, `35e559f6`, `e787f7b7`, `192b924b`, `19e6aced`, `d0e01ff9`, `504d62c2`, `a4b2dbd0`. Resolve full SHAs from the source inventory or frozen repository before execution. A donor anchor may be a mixed merge; it is not a cherry-pick instruction.

## Ordered sub-batches

- C1: CAS/admission-window/retry identity and failed-composition ownership cleanup.
- C2: Exact admitted user capture, immutable per-run Mission and settled reflection gates.
- C3: Archive/recall/restart evidence, scope isolation and intended automatic/manual trigger semantics.

## Execution checklist

- [ ] Read R01's hunk dispositions and compare each retained behavior against B; classify already-covered behavior without restoring its old code.
- [ ] Resolve this lane's exact target blocker: #32 branch-only deltas must be compared with #46 and final-main integration; selected dependency contract is verified at its exact pin.
- [ ] Replace this blocked card with executable per-sub-batch steps naming the selected files, actual target API signatures, exact tests, prepared commands, failure paths and expected outputs. Reuse the linked original plans for intent; no copied Done/Pass status.
- [ ] Reuse a meaningful existing regression; add only missing behavior/failure coverage and establish the intended failing assertion on the candidate.
- [ ] Port the smallest retained code and adapt to B. Keep locks/pins/migrations/generated files under the integration ownership in R03.
- [ ] Run the focused checks and applicable selected/omitted real path in acceptance.md; then required combined-candidate gates. Record skips as pending.
- [ ] Independently review the changed boundary and integrate the exact reviewed delta through `remake/integration`; invalidate affected evidence if the candidate changes.

## Acceptance cases owned by this lane

- Original admitted input, not assistant summary, is the captured source.
- Restart restores the same admitted window and Mission without repeating unknown effects.
- Source correction/delete/replay and scope boundaries retain their asserted behavior.
- Manual and automatic trigger cases remain separate; fixture shutdown can be observed repeatedly.

## Source test discovery and verification

In the pinned donor, use `rg '^func Test'` on the verified test paths or package paths above; in B confirm which tests still apply before selecting them. Then run `go vet <affected packages>` and `go test -count=1 -timeout 35m <affected packages> -run '<accepted test selection>'` after the target's documented asset/dependency preparation. UI changes use the target's existing typecheck/test/build and real-backend smoke entry. Placeholders in this paragraph are a refinement gate, not commands to execute unchanged. Precise commands for the first low-risk pilot are in R04.

## Exclusions and rollback

Do not port retired orchestration proof, redundant mandatory observer settlement or dead Embedded Host fixtures to keep old suites green.

Rollback is to the preceding accepted binary/source candidate with the declared compatible data format. A source revert is not a safe data downgrade. Preserve failed/unknown side-effect outcomes and never blindly replay tools. Newer or unsupported data must be rejected before writes. No live user store is used for validation.

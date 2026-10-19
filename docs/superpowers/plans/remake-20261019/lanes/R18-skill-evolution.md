# R18: Reviewed local Skill evolution intake plan

> For execution: read the accepted design and this plan, then use `superpowers:executing-plans` in one isolated write lane. Delegation is not selected by this package.

**Goal:** Restore the retained observable behavior for `skill-evolution` with the smallest compatible delta.
**Architecture:** Consume the accepted core and its existing typed boundaries. Use donor behavior and tests as evidence; rewrite only incompatible glue. No whole old-main merge.
**Tech stack:** Go 1.26.4, repository-selected Eino/EinoExt and Laputa/INOFY; source UI uses the repository's React/TypeScript toolchain where selected.
**Spec:** [Remake design](../../../specs/2026-10-19-remake-design.md).
**Status:** Blocked for product implementation. These are bounded intake sub-batches, not falsely Ready final code tasks.
**Branch:** `remake/skill-evolution`. **Worktree:** `<lanes-root>/skill-evolution/agent-vivy-remake-skill-evolution` with its own sibling `laputa/`.
**Immediate predecessors:** R02, R03. The index is the sole state/dependency authority.

## Inputs and outputs

Consumes: accepted core SHA B, exact dependency pins, approved retained-hunk map, R03's applicable schema/revision/approval/Assembly contracts and any explicitly selected feature predecessor.
Produces: one feature delta, explicit selected/omitted behavior if optional, source-first generated projections, exact-candidate evidence and an integration receipt. Public target signatures are intentionally not invented before R02; freeze them in the lane's refined implementation plan before coding.

## Governing issue requirements (synchronized 2026-10-20)

- [#64](https://github.com/ProjectViVy/agent-vivy/issues/64) owns local DIVA proposals: update the same Skill identity where appropriate, require human review, keep durable provenance/revisions, reject stale bases, and recover apply/restart states. No effective change before approval; EvoMap is outside this delivery.
- [#72](https://github.com/ProjectViVy/agent-vivy/issues/72) requires default local-Skill-only evolution, EvoMap experimental/default-off, truthful missing-receiver behavior and idempotent same-operation replay; unknown outcomes are not blindly retried.
- [#7](https://github.com/ProjectViVy/agent-vivy/issues/7) generic strategy composition is separate and unscheduled. [#83](https://github.com/ProjectViVy/agent-vivy/issues/83) coordinates re-intake on #91 contracts; donor code does not complete #64 UI/product acceptance.

remake/skill-evolution is raw donor source, not permission for EvoMap or auto-approval.


## Verified donor entry points

- `internal/runtime/cognitive_skill_proposal.go` — main donor.
- `internal/runtime/cognitive_skill_proposal_test.go` — main donor.
- `internal/runtime/skills_backend.go` — main donor.
- `internal/domain/skill.go` — main donor.
- `internal/storage/sqlite/approvals.go` — main donor.
- `internal/storage/postgres/approvals.go` — main donor.

Donor anchors: `0b482974`, `f2be7f61`, `421e9270`, `9de2a24e`. Resolve full SHAs from the source inventory or frozen repository before execution. A donor anchor may be a mixed merge; it is not a cherry-pick instruction.

## Ordered sub-batches

- K1: Select enabled user-owned local Skills and admit typed reflection proposals with source provenance.
- K2: Bind proposal to the current semantic revision under the accepted no-hash contract and require human review.
- K3: Apply/reject/expire/stale/restart outcomes through the existing revision store and Review Center.

## Execution checklist

- [ ] Read R01's hunk dispositions and compare each retained behavior against B; classify already-covered behavior without restoring its old code.
- [ ] Resolve this lane's exact target blocker: Compatible published Laputa skill_proposal and selected target revision interface; baseline cognitive interfaces must actually exist or be declared as a dependency after R02.
- [ ] Replace this blocked card with executable per-sub-batch steps naming the selected files, actual target API signatures, exact tests, prepared commands, failure paths and expected outputs. Reuse the linked original plans for intent; no copied Done/Pass status.
- [ ] Reuse a meaningful existing regression; add only missing behavior/failure coverage and establish the intended failing assertion on the candidate.
- [ ] Port the smallest retained code and adapt to B. Keep locks/pins/migrations/generated files under the integration ownership in R03.
- [ ] Run the focused checks and applicable selected/omitted real path in acceptance.md; then required combined-candidate gates. Record skips as pending.
- [ ] Independently review the changed boundary and integrate the exact reviewed delta through `remake/integration`; invalidate affected evidence if the candidate changes.

## Acceptance cases owned by this lane

- Real proposal -> human approval -> same Skill content changes -> later Skill read uses the changed revision.
- Denial, expiry and stale revision never apply.
- Restart reconciles pending/approved-but-unapplied revisions and the file-write/status crash window.
- Foreign/disabled/non-user-owned Skills cannot be selected.

## Source test discovery and verification

In the pinned donor, use `rg '^func Test'` on the verified test paths or package paths above; in B confirm which tests still apply before selecting them. Then run `go vet <affected packages>` and `go test -count=1 -timeout 35m <affected packages> -run '<accepted test selection>'` after the target's documented asset/dependency preparation. UI changes use the target's existing typecheck/test/build and real-backend smoke entry. Placeholders in this paragraph are a refinement gate, not commands to execute unchanged. Precise commands for the first low-risk pilot are in R04.

## Exclusions and rollback

Do not replay the temporary service.go deletion/restoration pair as feature code. No EvoMap expansion, auto-approval timeout or parallel revision authority.

Rollback is to the preceding accepted binary/source candidate with the declared compatible data format. A source revert is not a safe data downgrade. Preserve failed/unknown side-effect outcomes and never blindly replay tools. Newer or unsupported data must be rejected before writes. No live user store is used for validation.

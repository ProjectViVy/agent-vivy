# R13: Constrained reports and first-use entry intake plan

> For execution: read the accepted design and this plan, then use `superpowers:executing-plans` in one isolated write lane. Delegation is not selected by this package.

**Goal:** Restore the retained observable behavior for `reports` with the smallest compatible delta.
**Architecture:** Consume the accepted core and its existing typed boundaries. Use donor behavior and tests as evidence; rewrite only incompatible glue. No whole old-main merge.
**Tech stack:** Go 1.26.4, repository-selected Eino/EinoExt and Laputa/INOFY; source UI uses the repository's React/TypeScript toolchain where selected.
**Spec:** [Remake design](../../../specs/2026-10-19-remake-design.md).
**Status:** Blocked for product implementation. These are bounded intake sub-batches, not falsely Ready final code tasks.
**Branch:** `remake/reports`. **Worktree:** `<lanes-root>/reports/agent-vivy-remake-reports` with its own sibling `laputa/`.
**Immediate predecessors:** R12. The index is the sole state/dependency authority.

## Inputs and outputs

Consumes: accepted core SHA B, exact dependency pins, approved retained-hunk map, R03's applicable schema/revision/approval/Assembly contracts and any explicitly selected feature predecessor.
Produces: one feature delta, explicit selected/omitted behavior if optional, source-first generated projections, exact-candidate evidence and an integration receipt. Public target signatures are intentionally not invented before R02; freeze them in the lane's refined implementation plan before coding.

## Governing issue requirements (synchronized 2026-10-20)

- [#5](https://github.com/ProjectViVy/agent-vivy/issues/5) remains open and unscheduled. Preserve daily/weekly/monthly report-only scope, provenance and workspace boundaries; manual and scheduled runs share one bounded operation. Do not mutate memory/persona, propose evolution or invoke arbitrary tools; generated activity is excluded from source collection by default. Model failures need truthful deterministic fallback.
- [#6](https://github.com/ProjectViVy/agent-vivy/issues/6) AutoDream is separate and candidate-only; do not make it a report dependency or let reflection apply memory, Skill, ACTMEM or persona mutations.
- [#83](https://github.com/ProjectViVy/agent-vivy/issues/83) coordinates later intake. Use a second scheduler, queue or runner only if an accepted concrete contract requires it; current plan excludes them.

remake/reports is raw donor source; it does not schedule #5.


## Verified donor entry points

- `internal/reportcontract/ports.go` — main donor.
- `internal/modules/reports/module.go` — main donor.
- `internal/modules/reports/actions.go` — main donor.
- `plugins/vivy-notebook/ui/vivy-notebook/src/reports.tsx` — main donor.
- `recipes/no-reports.vivy.yml` — main donor.
- `recipes/reports-backend.vivy.yml` — main donor.

Donor anchors: `b2e38b9e`, `0091936d`, `12613e75`, `98fa815b`, `92d54247`. Resolve full SHAs from the source inventory or frozen repository before execution. A donor anchor may be a mixed merge; it is not a cherry-pick instruction.

## Ordered sub-batches

- R1: Trusted report admission and bounded manual generation, publish into the accepted notebook.
- R2: Empty-state first-report entry, generation/cancel/settings UI and provenance.
- R3: Schedule and recovery only after manual generation is accepted.

## Execution checklist

- [ ] Read R01's hunk dispositions and compare each retained behavior against B; classify already-covered behavior without restoring its old code.
- [ ] Resolve this lane's exact target blocker: Accepted notebook from R12 plus baseline workflow admission and scheduling contracts; a missing workflow interface adds a real prerequisite only after R02 inspection.
- [ ] Replace this blocked card with executable per-sub-batch steps naming the selected files, actual target API signatures, exact tests, prepared commands, failure paths and expected outputs. Reuse the linked original plans for intent; no copied Done/Pass status.
- [ ] Reuse a meaningful existing regression; add only missing behavior/failure coverage and establish the intended failing assertion on the candidate.
- [ ] Port the smallest retained code and adapt to B. Keep locks/pins/migrations/generated files under the integration ownership in R03.
- [ ] Run the focused checks and applicable selected/omitted real path in acceptance.md; then required combined-candidate gates. Record skips as pending.
- [ ] Independently review the changed boundary and integrate the exact reviewed delta through `remake/integration`; invalidate affected evidence if the candidate changes.

## Acceptance cases owned by this lane

- An empty section can generate its first report through the real UI, without pre-seeding by RPC.
- Generation uses the constrained approved model-call path and no arbitrary tools.
- Concurrent edit/move/delete does not overwrite a newer head; generated and edited revisions remain distinguishable.
- Restart after publication does not repeat the model call or duplicate the publication; schedule retry/catch-up is bounded.

## Source test discovery and verification

In the pinned donor, use `rg '^func Test'` on the verified test paths or package paths above; in B confirm which tests still apply before selecting them. Then run `go vet <affected packages>` and `go test -count=1 -timeout 35m <affected packages> -run '<accepted test selection>'` after the target's documented asset/dependency preparation. UI changes use the target's existing typecheck/test/build and real-backend smoke entry. Placeholders in this paragraph are a refinement gate, not commands to execute unchanged. Precise commands for the first low-risk pilot are in R04.

## Exclusions and rollback

Do not bring #86 Toolkit gate files into this lane. Do not create a second timer, queue or agent runner.

Rollback is to the preceding accepted binary/source candidate with the declared compatible data format. A source revert is not a safe data downgrade. Preserve failed/unknown side-effect outcomes and never blindly replay tools. Newer or unsupported data must be rejected before writes. No live user store is used for validation.

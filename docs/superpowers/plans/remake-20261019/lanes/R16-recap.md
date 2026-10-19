# R16: Subject continuity, exact evidence and governed Recap intake plan

> For execution: read the accepted design and this plan, then use `superpowers:executing-plans` in one isolated write lane. Delegation is not selected by this package.

**Goal:** Restore the retained observable behavior for `recap` with the smallest compatible delta.
**Architecture:** Consume the accepted core and its existing typed boundaries. Use donor behavior and tests as evidence; rewrite only incompatible glue. No whole old-main merge.
**Tech stack:** Go 1.26.4, repository-selected Eino/EinoExt and Laputa/INOFY; source UI uses the repository's React/TypeScript toolchain where selected.
**Spec:** [Remake design](../../../specs/2026-10-19-remake-design.md).
**Status:** Blocked for product implementation. These are bounded intake sub-batches, not falsely Ready final code tasks.
**Branch:** `remake/recap`. **Worktree:** `<lanes-root>/recap/agent-vivy-remake-recap` with its own sibling `laputa/`.
**Immediate predecessors:** R02, R03. The index is the sole state/dependency authority.

## Inputs and outputs

Consumes: accepted core SHA B, exact dependency pins, approved retained-hunk map, R03's applicable schema/revision/approval/Assembly contracts and any explicitly selected feature predecessor.
Produces: one feature delta, explicit selected/omitted behavior if optional, source-first generated projections, exact-candidate evidence and an integration receipt. Public target signatures are intentionally not invented before R02; freeze them in the lane's refined implementation plan before coding.

## Governing issue requirements (synchronized 2026-10-20)

- [#58](https://github.com/ProjectViVy/agent-vivy/issues/58) records the completed unified-context direction, not stable-target acceptance. Use one free-form, subject-level ACTMEM Recap across VIVY sessions, visible/editable in its UI; LIFE first, CODE excluded from phase one.
- Persist source evidence and replacement Recap/references before unloading raw context; preserve provenance/recovery. Forgetting is explicit and targeted. Workers do not inherit persona/Recap without trusted scope.
- [#57](https://github.com/ProjectViVy/agent-vivy/issues/57) owns subject/worker lifecycle; [#66](https://github.com/ProjectViVy/agent-vivy/issues/66) is editable prompt draft only, with no runtime activation, new permissions or separate persona/store. [#4](https://github.com/ProjectViVy/agent-vivy/issues/4) owns provider/scope rules.
- [#83](https://github.com/ProjectViVy/agent-vivy/issues/83) coordinates bounded re-intake; verify target/data contracts.

remake/recap is raw donor source, not a second continuity authority or acceptance.


## Verified donor entry points

- `internal/storage/subject_history.go` — main donor.
- `internal/storage/subjectsql/subjectsql.go` — main donor.
- `internal/runtime/recap_context.go` — main donor.
- `internal/runtime/recap_compaction.go` — main donor.
- `internal/app/subject_binding_test.go` — main donor.
- `plugins/vivy-memory/ui/vivy-memory/src/recap.tsx` — main donor.

Donor anchors: `dc548d43`, `7edab6ad`, `66fabfd2`, `f27dbe16`, `ee053ead`, `69096de7`, `0a762f25`, `71130731`, `473f3b5e`, `83748fba`. Resolve full SHAs from the source inventory or frozen repository before execution. A donor anchor may be a mixed merge; it is not a cherry-pick instruction.

## Ordered sub-batches

- S1: Subject identity and bounded exact source evidence, including storage-owned excluded terminal interactions.
- S2: Input preparation, explicit summary/compaction lifecycle, restart and one subject sink.
- S3: Governed Recap actions/tools and real management UI; preserve truthful unavailable states.
- S4: Reconcile #70/#71/#79/#82/#85 unique deltas once; consume producer/replay fixes, not each full branch.

## Execution checklist

- [ ] Read R01's hunk dispositions and compare each retained behavior against B; classify already-covered behavior without restoring its old code.
- [ ] Resolve this lane's exact target blocker: R03 must align 044-046 history data, admitted input and qualified sink contracts with accepted #91; selected Laputa closure must support both Recap and any selected Skill bridge.
- [ ] Replace this blocked card with executable per-sub-batch steps naming the selected files, actual target API signatures, exact tests, prepared commands, failure paths and expected outputs. Reuse the linked original plans for intent; no copied Done/Pass status.
- [ ] Reuse a meaningful existing regression; add only missing behavior/failure coverage and establish the intended failing assertion on the candidate.
- [ ] Port the smallest retained code and adapt to B. Keep locks/pins/migrations/generated files under the integration ownership in R03.
- [ ] Run the focused checks and applicable selected/omitted real path in acceptance.md; then required combined-candidate gates. Record skips as pending.
- [ ] Independently review the changed boundary and integrate the exact reviewed delta through `remake/integration`; invalidate affected evidence if the candidate changes.

## Acceptance cases owned by this lane

- Bound subject owner mismatch is explicit; no silent second recap stream or path migration.
- Refused/malformed/resumed tool input and result provenance remains exact within the selected contract.
- Unresolvable historical sources are explicitly excluded rather than blocking the subject forever.
- Recap edit/read/compact/restart is visible in the next model request; management availability follows backend capability.

## Source test discovery and verification

In the pinned donor, use `rg '^func Test'` on the verified test paths or package paths above; in B confirm which tests still apply before selecting them. Then run `go vet <affected packages>` and `go test -count=1 -timeout 35m <affected packages> -run '<accepted test selection>'` after the target's documented asset/dependency preparation. UI changes use the target's existing typecheck/test/build and real-backend smoke entry. Placeholders in this paragraph are a refinement gate, not commands to execute unchanged. Precise commands for the first low-risk pilot are in R04.

## Exclusions and rollback

No second context/summary owner, old evidence-hash authority or automatic LIFE activation beyond retained product scope. Sidebar/subject-entry work absent in donors remains a backlog gap.

Rollback is to the preceding accepted binary/source candidate with the declared compatible data format. A source revert is not a safe data downgrade. Preserve failed/unknown side-effect outcomes and never blindly replay tools. Newer or unsupported data must be rejected before writes. No live user store is used for validation.

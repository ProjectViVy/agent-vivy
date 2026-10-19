# R12: Optional durable notebook intake plan

> For execution: read the accepted design and this plan, then use `superpowers:executing-plans` in one isolated write lane. Delegation is not selected by this package.

**Goal:** Restore the retained observable behavior for `notebook` with the smallest compatible delta.
**Architecture:** Consume the accepted core and its existing typed boundaries. Use donor behavior and tests as evidence; rewrite only incompatible glue. No whole old-main merge.
**Tech stack:** Go 1.26.4, repository-selected Eino/EinoExt and Laputa/INOFY; source UI uses the repository's React/TypeScript toolchain where selected.
**Spec:** [Remake design](../../../specs/2026-10-19-remake-design.md).
**Status:** Blocked for product implementation. These are bounded intake sub-batches, not falsely Ready final code tasks.
**Branch:** `remake/notebook`. **Worktree:** `<lanes-root>/notebook/agent-vivy-remake-notebook` with its own sibling `laputa/`.
**Immediate predecessors:** R02, R03. The index is the sole state/dependency authority.

## Inputs and outputs

Consumes: accepted core SHA B, exact dependency pins, approved retained-hunk map, R03's applicable schema/revision/approval/Assembly contracts and any explicitly selected feature predecessor.
Produces: one feature delta, explicit selected/omitted behavior if optional, source-first generated projections, exact-candidate evidence and an integration receipt. Public target signatures are intentionally not invented before R02; freeze them in the lane's refined implementation plan before coding.

## Governing issue requirements (synchronized 2026-10-20)

- [#39](https://github.com/ProjectViVy/agent-vivy/issues/39) was owner-closed completed on 2026-10-18: Markdown editing/preview, revisions, comments, scoped management, delete/restore, provenance export, explicit tools and optional/headless boundaries. Automatic prompt injection is excluded.
- Re-intake on the #91 stable baseline is coordinated by [#83](https://github.com/ProjectViVy/agent-vivy/issues/83); this does not reopen #39. Any new Notebook expectations need their own issue.
- Keep reports in [#5](https://github.com/ProjectViVy/agent-vivy/issues/5) and the reports lane.

remake/notebook is raw donor source; verify what is absent before proposing a bounded re-intake.


## Verified donor entry points

- `internal/notebookcontract/ports.go` — main donor.
- `internal/modules/notebook/module.go` — main donor.
- `internal/modules/notebook/actions.go` — main donor.
- `internal/modules/notebook/tools.go` — main donor.
- `internal/app/assembly_notebook_test.go` — main donor.
- `plugins/vivy-notebook/ui/vivy-notebook/src/editor.tsx` — main donor.

Donor anchors: `47490e74`, `49585b99`, `fa0a9cbf`, `8ffe3f81`, `4e6c9510`, `ff6731e9`. Resolve full SHAs from the source inventory or frozen repository before execution. A donor anchor may be a mixed merge; it is not a cherry-pick instruction.

## Ordered sub-batches

- N1: Notebook storage/revision and source provenance; keep ordinary prompts free of automatic note preamble.
- N2: Typed scoped actions/tools and explicit selected/omitted composition.
- N3: Real editor, comments, revision history and export; keep reports absent.

## Execution checklist

- [ ] Read R01's hunk dispositions and compare each retained behavior against B; classify already-covered behavior without restoring its old code.
- [ ] Resolve this lane's exact target blocker: R03 must settle target notebook schema mapping and revised no-hash revision/concurrency representation before storage code is portable.
- [ ] Replace this blocked card with executable per-sub-batch steps naming the selected files, actual target API signatures, exact tests, prepared commands, failure paths and expected outputs. Reuse the linked original plans for intent; no copied Done/Pass status.
- [ ] Reuse a meaningful existing regression; add only missing behavior/failure coverage and establish the intended failing assertion on the candidate.
- [ ] Port the smallest retained code and adapt to B. Keep locks/pins/migrations/generated files under the integration ownership in R03.
- [ ] Run the focused checks and applicable selected/omitted real path in acceptance.md; then required combined-candidate gates. Record skips as pending.
- [ ] Independently review the changed boundary and integrate the exact reviewed delta through `remake/integration`; invalidate affected evidence if the candidate changes.

## Acceptance cases owned by this lane

- Create/edit/read/export survives close/reopen.
- Concurrent edit conflicts preserve the user draft; same-operation lost-ACK retry does not create another mutation.
- Foreign scope and forged human origin are denied.
- An omitted notebook Recipe has no notebook implementation entry; ordinary chat does not automatically learn notebook-derived text.

## Source test discovery and verification

In the pinned donor, use `rg '^func Test'` on the verified test paths or package paths above; in B confirm which tests still apply before selecting them. Then run `go vet <affected packages>` and `go test -count=1 -timeout 35m <affected packages> -run '<accepted test selection>'` after the target's documented asset/dependency preparation. UI changes use the target's existing typecheck/test/build and real-backend smoke entry. Placeholders in this paragraph are a refinement gate, not commands to execute unchanged. Precise commands for the first low-risk pilot are in R04.

## Exclusions and rollback

No report generation, report scheduler, Obsidian integration or new storage plugin framework.

Rollback is to the preceding accepted binary/source candidate with the declared compatible data format. A source revert is not a safe data downgrade. Preserve failed/unknown side-effect outcomes and never blindly replay tools. Newer or unsupported data must be rejected before writes. No live user store is used for validation.

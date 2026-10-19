# R19: Laputa persona authority and onboarding intake plan

> For execution: read the accepted design and this plan, then use `superpowers:executing-plans` in one isolated write lane. Delegation is not selected by this package.

**Goal:** Restore the retained observable behavior for `persona` with the smallest compatible delta.
**Architecture:** Consume the accepted core and its existing typed boundaries. Use donor behavior and tests as evidence; rewrite only incompatible glue. No whole old-main merge.
**Tech stack:** Go 1.26.4, repository-selected Eino/EinoExt and Laputa/INOFY; source UI uses the repository's React/TypeScript toolchain where selected.
**Spec:** [Remake design](../../../specs/2026-10-19-remake-design.md).
**Status:** Blocked for product implementation. These are bounded intake sub-batches, not falsely Ready final code tasks.
**Branch:** `remake/persona`. **Worktree:** `<lanes-root>/persona/agent-vivy-remake-persona` with its own sibling `laputa/`.
**Immediate predecessors:** R02, R03. The index is the sole state/dependency authority.

## Inputs and outputs

Consumes: accepted core SHA B, exact dependency pins, approved retained-hunk map, R03's applicable schema/revision/approval/Assembly contracts and any explicitly selected feature predecessor.
Produces: one feature delta, explicit selected/omitted behavior if optional, source-first generated projections, exact-candidate evidence and an integration receipt. Public target signatures are intentionally not invented before R02; freeze them in the lane's refined implementation plan before coding.

## Governing issue requirements (synchronized 2026-10-20)

- [#57](https://github.com/ProjectViVy/agent-vivy/issues/57) keeps one subject across sessions: configured avatar/name above sessions, no Coding/Life toggle or separate personality per session. Navigation does not cancel work or grant authority; workers do not inherit persona/Recap automatically.
- [#66](https://github.com/ProjectViVy/agent-vivy/issues/66) is editable guidance only. Reuse Laputa persona/content mechanisms; do not invent a roster, Settings fields or bundle types. Prompt text cannot enforce permission/isolation.
- [#58](https://github.com/ProjectViVy/agent-vivy/issues/58) owns Recap/context UI; keep that work in the Recap lane.

remake/persona is raw donor source; #57/#66 remain scope owners.


## Verified donor entry points

- `internal/runtime/prompt_state.go` — main donor.
- `internal/modules/diva-cognitive/dispatch.go` — main donor.
- `plugins/vivy-persona/ui/vivy-persona/src/onboarding.tsx` — main donor.
- `plugins/vivy-persona/ui/vivy-persona/src/gate.tsx` — main donor.
- `plugins/vivy-persona/ui/vivy-persona/src/view.tsx` — main donor.

Donor anchors: `8088f5bc`, `83ee6246`. Resolve full SHAs from the source inventory or frozen repository before execution. A donor anchor may be a mixed merge; it is not a cherry-pick instruction.

## Ordered sub-batches

- P1: Preserve one Laputa companion persona authority and explicit uninitialized behavior.
- P2: First-run guidance, persona display/avatar and dismissal/lifecycle behavior.

## Execution checklist

- [ ] Read R01's hunk dispositions and compare each retained behavior against B; classify already-covered behavior without restoring its old code.
- [ ] Resolve this lane's exact target blocker: Inspect baseline persona/Mask bridge and actual Laputa pin; do not import #68 wholesale.
- [ ] Replace this blocked card with executable per-sub-batch steps naming the selected files, actual target API signatures, exact tests, prepared commands, failure paths and expected outputs. Reuse the linked original plans for intent; no copied Done/Pass status.
- [ ] Reuse a meaningful existing regression; add only missing behavior/failure coverage and establish the intended failing assertion on the candidate.
- [ ] Port the smallest retained code and adapt to B. Keep locks/pins/migrations/generated files under the integration ownership in R03.
- [ ] Run the focused checks and applicable selected/omitted real path in acceptance.md; then required combined-candidate gates. Record skips as pending.
- [ ] Independently review the changed boundary and integrate the exact reviewed delta through `remake/integration`; invalidate affected evidence if the candidate changes.

## Acceptance cases owned by this lane

- Fresh profile can complete onboarding through the real UI.
- Companion requests use the persona authority; coding/worker faces retain their supported boundary.
- Reload and error paths show truthful setup state and preserve user input.

## Source test discovery and verification

In the pinned donor, use `rg '^func Test'` on the verified test paths or package paths above; in B confirm which tests still apply before selecting them. Then run `go vet <affected packages>` and `go test -count=1 -timeout 35m <affected packages> -run '<accepted test selection>'` after the target's documented asset/dependency preparation. UI changes use the target's existing typecheck/test/build and real-backend smoke entry. Placeholders in this paragraph are a refinement gate, not commands to execute unchanged. Precise commands for the first low-risk pilot are in R04.

## Exclusions and rollback

No notebook auto-injection, separate personality store or restored conversation-mode selector.

Rollback is to the preceding accepted binary/source candidate with the declared compatible data format. A source revert is not a safe data downgrade. Preserve failed/unknown side-effect outcomes and never blindly replay tools. Newer or unsupported data must be rejected before writes. No live user store is used for validation.

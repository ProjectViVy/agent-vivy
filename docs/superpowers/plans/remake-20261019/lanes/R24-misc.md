# R24: Miscellaneous maintenance and developer experience intake plan

> For execution: read the accepted design and this plan, then use `superpowers:executing-plans` in one isolated write lane. Delegation is not selected by this package.

**Goal:** Restore the retained observable behavior for `misc` with the smallest compatible delta.
**Architecture:** Consume the accepted core and its existing typed boundaries. Use donor behavior and tests as evidence; rewrite only incompatible glue. No whole old-main merge.
**Tech stack:** Go 1.26.4, repository-selected Eino/EinoExt and Laputa/INOFY; source UI uses the repository's React/TypeScript toolchain where selected.
**Spec:** [Remake design](../../../specs/2026-10-19-remake-design.md).
**Status:** Blocked for product implementation. These are bounded intake sub-batches, not falsely Ready final code tasks.
**Branch:** `remake/misc`. **Worktree:** `<lanes-root>/misc/agent-vivy-remake-misc` with its own sibling `laputa/`.
**Immediate predecessors:** R02, R03. The index is the sole state/dependency authority.

## Inputs and outputs

Consumes: accepted core SHA B, exact dependency pins, approved retained-hunk map, R03's applicable schema/revision/approval/Assembly contracts and any explicitly selected feature predecessor.
Produces: one feature delta, explicit selected/omitted behavior if optional, source-first generated projections, exact-candidate evidence and an integration receipt. Public target signatures are intentionally not invented before R02; freeze them in the lane's refined implementation plan before coding.

## Governing issue requirements (synchronized 2026-10-20)

- Keep this branch to general maintenance, dev environment, CI/fixtures, Studio gitlink/branding and truly cross-cutting docs. Route identifiable feature changes to their owning line; no unreviewed mixed dump.
- [#14](https://github.com/ProjectViVy/agent-vivy/issues/14) retains older-image offload and SSH gaps; [#15](https://github.com/ProjectViVy/agent-vivy/issues/15) defines SSH as a Code Bundle extension outside Minimal Face. [#2](https://github.com/ProjectViVy/agent-vivy/issues/2) keeps A2A separate; [#3](https://github.com/ProjectViVy/agent-vivy/issues/3) leaves image generation to PENS. Do not implement/close these through miscellaneous cleanup.
- [#83](https://github.com/ProjectViVy/agent-vivy/issues/83) coordinates selected re-intake. #91 owns core deletions; keep retired Embedded/C-ABI/hash removal covered/retired.

remake/misc is raw donor source; include only truly general changes without a tighter feature owner.


## Verified donor entry points

- `justfile` — main donor.
- `scripts/dev.sh` — main donor.
- `scripts/lane-new.mjs` — main donor.
- `scripts/ensure-laputa.mjs` — main donor.
- `Dockerfile` — main donor.
- `.github/workflows/ci.yml` — main donor.
- `AGENTS.md` — main donor.

Donor anchors: `393c84e1`, `9693b38e`, `60730fb4`, `7b42b292`, `e9609296`, `512e51bd`. Resolve full SHAs from the source inventory or frozen repository before execution. A donor anchor may be a mixed merge; it is not a cherry-pick instruction.

## Ordered sub-batches

- M1: General developer environment/CI/portable fixture fixes. Keep feature-only checks with the feature.
- M2: Confirm obsolete Embedded Host/C ABI remains removed under #91; record covered/retired rather than restoring it.
- M3: Studio gitlink/branding and generic instruction/tooling changes after compatibility review.
- M4: Keep SSH/A2A/Agentic planning-only records as backlog evidence; classify historical source-hash maintenance as retired, not a patch to restore.

## Execution checklist

- [ ] Read R01's hunk dispositions and compare each retained behavior against B; classify already-covered behavior without restoring its old code.
- [ ] Resolve this lane's exact target blocker: Donor lane-new.mjs always creates feat/<slug>; do not use it unchanged for remake branches. Use native worktree commands with per-lane sibling Laputa unless a separately scoped helper change is chosen.
- [ ] Replace this blocked card with executable per-sub-batch steps naming the selected files, actual target API signatures, exact tests, prepared commands, failure paths and expected outputs. Reuse the linked original plans for intent; no copied Done/Pass status.
- [ ] Reuse a meaningful existing regression; add only missing behavior/failure coverage and establish the intended failing assertion on the candidate.
- [ ] Port the smallest retained code and adapt to B. Keep locks/pins/migrations/generated files under the integration ownership in R03.
- [ ] Run the focused checks and applicable selected/omitted real path in acceptance.md; then required combined-candidate gates. Record skips as pending.
- [ ] Independently review the changed boundary and integrate the exact reviewed delta through `remake/integration`; invalidate affected evidence if the candidate changes.

## Acceptance cases owned by this lane

- Supported fresh development startup and repository commands work on the named platform.
- Packing/container paths include only required dependencies and respect the accepted artifact contract.
- No obsolete Embedded/C ABI entry or custom source-hash gate returns as collateral maintenance.
- Misc ledger has no unreviewed user-facing runtime feature hidden in it.

## Source test discovery and verification

In the pinned donor, use `rg '^func Test'` on the verified test paths or package paths above; in B confirm which tests still apply before selecting them. Then run `go vet <affected packages>` and `go test -count=1 -timeout 35m <affected packages> -run '<accepted test selection>'` after the target's documented asset/dependency preparation. UI changes use the target's existing typecheck/test/build and real-backend smoke entry. Placeholders in this paragraph are a refinement gate, not commands to execute unchanged. Precise commands for the first low-risk pilot are in R04.

## Exclusions and rollback

One misc branch as requested; no broad cleanup or automatic AGENTS rewrite. Protocol/CI requirement changes need their actual authorization.

Rollback is to the preceding accepted binary/source candidate with the declared compatible data format. A source revert is not a safe data downgrade. Preserve failed/unknown side-effect outcomes and never blindly replay tools. Newer or unsupported data must be rejected before writes. No live user store is used for validation.

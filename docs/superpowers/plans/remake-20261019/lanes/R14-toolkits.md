# R14: Native content package lifecycle intake plan

> For execution: read the accepted design and this plan, then use `superpowers:executing-plans` in one isolated write lane. Delegation is not selected by this package.

**Goal:** Restore the retained observable behavior for `toolkits` with the smallest compatible delta.
**Architecture:** Consume the accepted core and its existing typed boundaries. Use donor behavior and tests as evidence; rewrite only incompatible glue. No whole old-main merge.
**Tech stack:** Go 1.26.4, repository-selected Eino/EinoExt and Laputa/INOFY; source UI uses the repository's React/TypeScript toolchain where selected.
**Spec:** [Remake design](../../../specs/2026-10-19-remake-design.md).
**Status:** Blocked for product implementation. These are bounded intake sub-batches, not falsely Ready final code tasks.
**Branch:** `remake/toolkits`. **Worktree:** `<lanes-root>/toolkits/agent-vivy-remake-toolkits` with its own sibling `laputa/`.
**Immediate predecessors:** R02, R03. The index is the sole state/dependency authority.

## Inputs and outputs

Consumes: accepted core SHA B, exact dependency pins, approved retained-hunk map, R03's applicable schema/revision/approval/Assembly contracts and any explicitly selected feature predecessor.
Produces: one feature delta, explicit selected/omitted behavior if optional, source-first generated projections, exact-candidate evidence and an integration receipt. Public target signatures are intentionally not invented before R02; freeze them in the lane's refined implementation plan before coding.

## Verified donor entry points

- `internal/bundlecontract/manifest.go` — main donor.
- `internal/bundles/manager.go` — main donor.
- `internal/modules/toolkits/module.go` — main donor.
- `plugins/vivy-toolkits-ui/ui/vivy-toolkits/src/ToolkitPage.tsx` — main donor.
- `scripts/toolkits-e2e.mjs` — main donor.
- `ui/e2e/toolkits.spec.ts` — main donor.

Donor anchors: `60fbb8e6`, `1aa36240`, `54f16437`, `f7c90c11`, `1ceeacbc`, `66ff7f94`, `2ba1312a`, `07e8b131`, `908cb039`, `5cc24902`. Resolve full SHAs from the source inventory or frozen repository before execution. A donor anchor may be a mixed merge; it is not a cherry-pick instruction.

## Ordered sub-batches

- T1: Resolve D07 owned release identity, fresh executable consent and target persistence before import.
- T2: Acquire/install/read native content and lifecycle/recovery with one catalog owner.
- T3: Update/rollback/rebind/revocation and required executable consent for MCP/Hook/Skill/Mask declarations.
- T4: Library/details/staged-change UI, then reconcile #86/#87 browser launcher exactly once and retain TK-13 cases.

## Execution checklist

- [ ] Read R01's hunk dispositions and compare each retained behavior against B; classify already-covered behavior without restoring its old code.
- [ ] Resolve this lane's exact target blocker: D07 design, release/consent/data contracts and actual baseline module/Host APIs must be accepted for this batch.
- [ ] Replace this blocked card with executable per-sub-batch steps naming the selected files, actual target API signatures, exact tests, prepared commands, failure paths and expected outputs. Reuse the linked original plans for intent; no copied Done/Pass status.
- [ ] Reuse a meaningful existing regression; add only missing behavior/failure coverage and establish the intended failing assertion on the candidate.
- [ ] Port the smallest retained code and adapt to B. Keep locks/pins/migrations/generated files under the integration ownership in R03.
- [ ] Run the focused checks and applicable selected/omitted real path in acceptance.md; then required combined-candidate gates. Record skips as pending.
- [ ] Independently review the changed boundary and integrate the exact reviewed delta through `remake/integration`; invalidate affected evidence if the candidate changes.

## Acceptance cases owned by this lane

- ArtifactID/release sequence/config revision identifies the owned release without restoring custom digest gates.
- Install/update/rollback/crash-reopen retain consistent active declarations and visible failed states.
- New executable material requires scoped fresh consent; revocation takes effect.
- Real Toolkit browser suites run from the repository entry with isolated backend/UI ports and homes.

## Source test discovery and verification

In the pinned donor, use `rg '^func Test'` on the verified test paths or package paths above; in B confirm which tests still apply before selecting them. Then run `go vet <affected packages>` and `go test -count=1 -timeout 35m <affected packages> -run '<accepted test selection>'` after the target's documented asset/dependency preparation. UI changes use the target's existing typecheck/test/build and real-backend smoke entry. Placeholders in this paragraph are a refinement gate, not commands to execute unchanged. Precise commands for the first low-risk pilot are in R04.

## Exclusions and rollback

Not the build-time SDK plugin platform. No runtime loading of Go/UI source or new content marketplace. Native dependency/TLS/Git integrity is not retired by application hash removal.

Rollback is to the preceding accepted binary/source candidate with the declared compatible data format. A source revert is not a safe data downgrade. Preserve failed/unknown side-effect outcomes and never blindly replay tools. Newer or unsupported data must be rejected before writes. No live user store is used for validation.

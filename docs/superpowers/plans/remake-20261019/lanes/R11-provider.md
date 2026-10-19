# R11: Provider routing and model capability fixes intake plan

> For execution: read the accepted design and this plan, then use `superpowers:executing-plans` in one isolated write lane. Delegation is not selected by this package.

**Goal:** Restore the retained observable behavior for `provider` with the smallest compatible delta.
**Architecture:** Consume the accepted core and its existing typed boundaries. Use donor behavior and tests as evidence; rewrite only incompatible glue. No whole old-main merge.
**Tech stack:** Go 1.26.4, repository-selected Eino/EinoExt and Laputa/INOFY; source UI uses the repository's React/TypeScript toolchain where selected.
**Spec:** [Remake design](../../../specs/2026-10-19-remake-design.md).
**Status:** Blocked for product implementation. These are bounded intake sub-batches, not falsely Ready final code tasks.
**Branch:** `remake/provider`. **Worktree:** `<lanes-root>/provider/agent-vivy-remake-provider` with its own sibling `laputa/`.
**Immediate predecessors:** R02, R03. The index is the sole state/dependency authority.

## Inputs and outputs

Consumes: accepted core SHA B, exact dependency pins, approved retained-hunk map, R03's applicable schema/revision/approval/Assembly contracts and any explicitly selected feature predecessor.
Produces: one feature delta, explicit selected/omitted behavior if optional, source-first generated projections, exact-candidate evidence and an integration receipt. Public target signatures are intentionally not invented before R02; freeze them in the lane's refined implementation plan before coding.

## Verified donor entry points

- `internal/app/provider_selection.go` — main donor.
- `internal/provider/resolving.go` — main donor.
- `internal/provider/discover.go` — main donor.
- `internal/app/settings/settings.go` — main donor.
- `internal/rpc/control_thinking_test.go` — main donor.
- `ui/src/components/chat/ChatInput.tsx` — main donor.

Donor anchors: `f5322c92`, `2f66363c`, `968ae236`, `8d77c0c5`, `1bdfdca4`, `773dc741`, `652ac58e`, `8a37e059`. Resolve full SHAs from the source inventory or frozen repository before execution. A donor anchor may be a mixed merge; it is not a cherry-pick instruction.

## Ordered sub-batches

- P1: Registered provider endpoint/key and deterministic environment selection.
- P2: Model list/default consistency, bounded discovery and trusted output cap.
- P3: Thinking metadata projection, empty-list Settings behavior and composer semantics. Coordinate source-first ChatInput edits with chat-ui.

## Execution checklist

- [ ] Read R01's hunk dispositions and compare each retained behavior against B; classify already-covered behavior without restoring its old code.
- [ ] Resolve this lane's exact target blocker: Compare c091ccf4 and later accepted core against each donor; recovery cleanup e7255a7f/0c083f12 may contain extra behavior absent from the preamble and needs a separate cut.
- [ ] Replace this blocked card with executable per-sub-batch steps naming the selected files, actual target API signatures, exact tests, prepared commands, failure paths and expected outputs. Reuse the linked original plans for intent; no copied Done/Pass status.
- [ ] Reuse a meaningful existing regression; add only missing behavior/failure coverage and establish the intended failing assertion on the candidate.
- [ ] Port the smallest retained code and adapt to B. Keep locks/pins/migrations/generated files under the integration ownership in R03.
- [ ] Run the focused checks and applicable selected/omitted real path in acceptance.md; then required combined-candidate gates. Record skips as pending.
- [ ] Independently review the changed boundary and integrate the exact reviewed delta through `remake/integration`; invalidate affected evidence if the candidate changes.

## Acceptance cases owned by this lane

- Configured endpoint and raw provider model ID appear in the outbound request.
- Ambiguous environment selection abstains or follows the explicit selection contract; keys are not leaked.
- Unsupported thinking is disabled; an unselected thinking level omits the override and uses the persisted default.
- Trusted per-call output cap remains effective after thinking options.

## Source test discovery and verification

In the pinned donor, use `rg '^func Test'` on the verified test paths or package paths above; in B confirm which tests still apply before selecting them. Then run `go vet <affected packages>` and `go test -count=1 -timeout 35m <affected packages> -run '<accepted test selection>'` after the target's documented asset/dependency preparation. UI changes use the target's existing typecheck/test/build and real-backend smoke entry. Placeholders in this paragraph are a refinement gate, not commands to execute unchanged. Precise commands for the first low-risk pilot are in R04.

## Exclusions and rollback

AgenticModel/P2-P4 and executable openai-responses are not delivered capabilities to restore. Eino/Claude preamble is already owned by #91 at c091ccf4; it must not be duplicated.

Rollback is to the preceding accepted binary/source candidate with the declared compatible data format. A source revert is not a safe data downgrade. Preserve failed/unknown side-effect outcomes and never blindly replay tools. Newer or unsupported data must be rejected before writes. No live user store is used for validation.

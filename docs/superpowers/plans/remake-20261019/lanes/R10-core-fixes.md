# R10: Core correctness and durable interaction fixes intake plan

> For execution: read the accepted design and this plan, then use `superpowers:executing-plans` in one isolated write lane. Delegation is not selected by this package.

**Goal:** Restore the retained observable behavior for `core-fixes` with the smallest compatible delta.
**Architecture:** Consume the accepted core and its existing typed boundaries. Use donor behavior and tests as evidence; rewrite only incompatible glue. No whole old-main merge.
**Tech stack:** Go 1.26.4, repository-selected Eino/EinoExt and Laputa/INOFY; source UI uses the repository's React/TypeScript toolchain where selected.
**Spec:** [Remake design](../../../specs/2026-10-19-remake-design.md).
**Status:** Blocked for product implementation. These are bounded intake sub-batches, not falsely Ready final code tasks.
**Branch:** `remake/core-fixes`. **Worktree:** `<lanes-root>/core-fixes/agent-vivy-remake-core-fixes` with its own sibling `laputa/`.
**Immediate predecessors:** R02, R03. The index is the sole state/dependency authority.

## Inputs and outputs

Consumes: accepted core SHA B, exact dependency pins, approved retained-hunk map, R03's applicable schema/revision/approval/Assembly contracts and any explicitly selected feature predecessor.
Produces: one feature delta, explicit selected/omitted behavior if optional, source-first generated projections, exact-candidate evidence and an integration receipt. Public target signatures are intentionally not invented before R02; freeze them in the lane's refined implementation plan before coding.

## Governing issue requirements (synchronized 2026-10-20)

- [#91](https://github.com/ProjectViVy/agent-vivy/issues/91) owns the stable core: preserve real tool-result/call identity in the next model request, one execution owner, permissions and crash safety; do not restore retired hashes or a second runner.
- [#32](https://github.com/ProjectViVy/agent-vivy/issues/32) closed as superseded coordination, not fixed remediation. [#83](https://github.com/ProjectViVy/agent-vivy/issues/83) retains C1-C7, H4 and R1-R4 for bounded core/cognition/Recap/provider re-intake; recheck and disposition each on target.
- Same-Run steering and nudge/warming belong to [#93](https://github.com/ProjectViVy/agent-vivy/issues/93), not ordinary queue fixes. A donor branch does not enable EXP policy.

remake/core-fixes is raw donor source; #83 does not make it ready or accepted.


## Verified donor entry points

- `internal/rpc/protocol.go` — main donor.
- `internal/rpc/protocol_test.go` — main donor.
- `internal/tools/bashclass.go` — main donor.
- `internal/runtime/queue.go` — main donor.
- `internal/runtime/tooladapter.go` — main donor.
- `internal/logging/diagnostics.go` — main donor.

Donor anchors: `da86f916`, `12cef819`, `d8c2b839`, `242091b8`, `09fabb05`, `01ef1180`, `f9034c04`. Resolve full SHAs from the source inventory or frozen repository before execution. A donor anchor may be a mixed merge; it is not a cherry-pick instruction.

## Ordered sub-batches

- C1: RPC notification ordering and nonblocking calls; bash redirect classification; model-visible recoverable tool errors.
- C2: Faithful authorized task data and EXP policy deltas; preserve actual permission checks.
- C3: Durable turn DTO admission, restart, cancellation, GUI/TUI recall and bounded ACK. Separate ordinary queue fixes from same-Run steering owned by #93.
- C4: Append-safe diagnostics and required model settlement errors; port the retained behavior without recreating retired observation ownership.

## Execution checklist

- [ ] Read R01's hunk dispositions and compare each retained behavior against B; classify already-covered behavior without restoring its old code.
- [ ] Resolve this lane's exact target blocker: Select one smallest still-missing core fix after #91 acceptance; classify already-covered fixes instead of reapplying them.
- [ ] Replace this blocked card with executable per-sub-batch steps naming the selected files, actual target API signatures, exact tests, prepared commands, failure paths and expected outputs. Reuse the linked original plans for intent; no copied Done/Pass status.
- [ ] Reuse a meaningful existing regression; add only missing behavior/failure coverage and establish the intended failing assertion on the candidate.
- [ ] Port the smallest retained code and adapt to B. Keep locks/pins/migrations/generated files under the integration ownership in R03.
- [ ] Run the focused checks and applicable selected/omitted real path in acceptance.md; then required combined-candidate gates. Record skips as pending.
- [ ] Independently review the changed boundary and integrate the exact reviewed delta through `remake/integration`; invalidate affected evidence if the candidate changes.

## Acceptance cases owned by this lane

- Notification handlers retain wire order while handlers can call RPC.
- A malformed or recoverable tool request reaches the model as the supported tool outcome; unauthorized execution remains refused.
- Queued thinking, attachments and captured context survive recall/restart; an oversized admission is rejected before ACK.
- Diagnostics allow append growth and detect truncation/replacement under the accepted cursor contract.

## Source test discovery and verification

In the pinned donor, use `rg '^func Test'` on the verified test paths or package paths above; in B confirm which tests still apply before selecting them. Then run `go vet <affected packages>` and `go test -count=1 -timeout 35m <affected packages> -run '<accepted test selection>'` after the target's documented asset/dependency preparation. UI changes use the target's existing typecheck/test/build and real-backend smoke entry. Placeholders in this paragraph are a refinement gate, not commands to execute unchanged. Precise commands for the first low-risk pilot are in R04.

## Exclusions and rollback

No blanket restoration of custom hashes, handoff barriers, redaction or turn-limit default policies. EXP policy presence is not permission to enable it.

Rollback is to the preceding accepted binary/source candidate with the declared compatible data format. A source revert is not a safe data downgrade. Preserve failed/unknown side-effect outcomes and never blindly replay tools. Newer or unsupported data must be rejected before writes. No live user store is used for validation.

# Stable loop first, then gated capability re-intake

Date: 2026-10-19

Authority: [shared deletion design](2026-10-19-complete-deletion-plan.md) and
[#91 planning index](../superpowers/plans/issue-91/index.md). This supplement
clarifies sequencing and handoff; it does not authorize implementation, change
product scope, replace technical contracts or certify any behavior.

## Intent and baseline

The rollback selects a stable place to refine the agent loop, not abandonment of
post-baseline capabilities. Baseline A is
`202aac3885cb1479998127f0fb9004bea54aca57` (2026-10-07), selected by the owner
as the stable benchmarked pi-parity closeout. The reviewed planning branch head
was `5be9843d184cadb840c510210cf6c092d74c9e18`; no product changes are implied
by its documentation commits.

The original comparison from baseline A through `e9609296` counted 168 commits
and 30 first-parent merges. It is a historical donor inventory, not a current
main-distance claim or a list to merge wholesale. Main was later observed at
`c57dfd322a50daf5ac2365369e84f81f1733a8b2` after PR #84 merged on
2026-10-19. A merge into that line does not establish acceptance on baseline A.
Preserve the stable branch; do not merge/rebase current main merely to recover
capabilities.

## Phase A: Stabilize the production loop on baseline A

Use [#91](https://github.com/ProjectViVy/agent-vivy/issues/91) for the base cut:
hash deletion, contraction of redundant continuation ownership and an inert
loop-exp seam. First establish the causal milestone on the production path:
model -> real isolated nonce read -> exact result/call ID in model request two
-> final text -> one terminal -> idle. Preserve the single Service.Run/Eino/
Journal/policy path, approval/claim/outcome fences, required accounting, hard
budgets, cancel and recovery.

D04/D05 retain one existing core implementation of coaching, warming and
same-Run steering until each owning epic cuts over. No dual implementation or
silent feature retirement is allowed. D13/D14 base acceptance does not require
the not-yet-delivered EXP packages or claim their final physical omission.
Missing future re-intake coordination, absent bundles/reports or an uninspected
upstream finding must not block independent loop proof.

The causal milestone is not full #91 closure. Applicable baseline source/data,
consumer, independent-review and required product/real-path gates remain.
A confirmed incompatible upstream API blocks its affected cut; do not bypass
the gate or call all hash deletion complete. Laputa/Studio current-pin API checks
remain pending under shared §11.10; the earlier audits used different pins.

Finite source support is SQL prefixes 001..035 with the enumerated legacy
normalization, baseline approvals/checkpoints and BML v1, converted on disposable
copies through the separately delivered finite artifact. Source heads 036..043
and other unlisted newer stores fail closed before writes; future support needs
its own reviewed decision and fixtures. No production data access, resets or
released-migration rewrites are authorized.

## Phase B: Extract retained experiments separately

[E-NUDGE](../superpowers/plans/epic-nudge/index.md),
[E-WARMING](../superpowers/plans/epic-warming/index.md) and
[E-STEERING](../superpowers/plans/epic-steering/index.md) own package parity,
settings/UI/events/recovery, core-body removal and final C11 physical omission.
Their acceptance includes selected behavior, none/each/all composition and
independent integrated artifact inspection, not merely an inert seam.

Preserve the owner's selection decisions: nudge and warming are not installed
in the shipped default-family/lite/masks Recipes; do not infer steering is
default-disabled. Nudge's count-6 repeated-tuple hard stop moves with nudge;
core MaxToolTurns/Eino MaxIterations remain when nudge is absent. Package
cutover leaves one implementation, never core-plus-package fallback.

## Phase C: Re-introduce capabilities in reviewed batches

[#83](https://github.com/ProjectViVy/agent-vivy/issues/83) coordinates staged
re-intake and PR integration.
[#81](https://github.com/ProjectViVy/agent-vivy/issues/81) retains the capability
backlog. Their live issue inventory owns donor PR state and overlap; historical
PRs and the backup branch are evidence/donor diffs, not blanket merge orders.

Each proposed batch records a small handoff in the existing issue/PR:
- Capability and owner, exact donor/source and target baseline candidate.
- Dependencies and overlap with already accepted work; take only the intended
  behavior, not stale commits or parallel implementations.
- Affected contracts, pinned APIs, schema/data source support and consent.
- Adaptation to the accepted loop/Assembly/authority boundaries, including
  complete removal of retired hashes and redundant handoffs.
- Focused tests plus applicable mandatory CI, real-path and independent
  acceptance evidence at the exact resulting candidate; missing evidence stays
  pending, not inherited from an old merge.
- Failure/rollback boundary and explicit next batch or blocker.

[D07](../superpowers/plans/issue-91/D07.md) is the explanatory release/consent
checkpoint for the later bundle/toolkit batch. It has no baseline implementation,
converter fragment, DAG edge or wave slot. That batch must resolve owned
release identity, fresh executable consent, conversion and recovery before it
ships. Report/new-notebook/toolkit paths absent at baseline are not created just
to satisfy the older plan. Existing baseline Work/workflow/notes, skill/memory
and Studio-ledger functionality remains owned by its actual cuts.

Original plan packages, backup history and dated logs remain intact as evidence.
Current linked contracts and phase applicability govern new work.

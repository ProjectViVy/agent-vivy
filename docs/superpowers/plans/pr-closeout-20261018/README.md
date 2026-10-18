# Existing PR closeout preparation

Authoritative tracker: [#83](https://github.com/ProjectViVy/agent-vivy/issues/83). Capability gaps and acceptance: [#81](https://github.com/ProjectViVy/agent-vivy/issues/81).

Prepared 2026-10-18. Branch `integration/pr-closeout-20261018` starts from current main `e96092966cfbfa443648374948e7e7e16dd01683`. This commit adds this document only; it integrates no product code. Future status changes belong in #83; this document is the immutable preparation receipt, not a second active checklist.

## Responsibility split

- #83 owns existing PR integration: E5 Recap/peer repairs, E7 Provider #80 closeout, E6 existing #69 integration, dependency pins, supersession decisions, conflicts and final combined-candidate readiness.
- #81 keeps E1/E2/E3/E4, E6 missing Skill lifecycle coverage and real same-Skill loop acceptance, E8 ACP, and E9 targeted remediation. E3 alone owns W1/W2 plus E9-04 shared workflow work.
- Historical E6-01 task 1 recovery/reconciliation routes to #83; missing authority/lifecycle cases and corrections remain #81 E6. E6-02 consumes the integrated candidate and stays #81. Do not create a second implementation of #69.
- One future authorized integrator serializes Go/Laputa pins, runtime/App assembly and generated outputs. Feature lanes hand off bounded diffs and exact evidence; no competing repins.

## Pinned open-PR ledger

Snapshot checked 2026-10-18 17:09 UTC; recheck before execution.

| PR | Head | Relationship |
| --- | --- | --- |
| [#80](https://github.com/ProjectViVy/agent-vivy/pull/80) | `001c563740a4ff32e1376f6bbfb2c6caaa7b22b3` | Existing Provider priority; CI disposition pending. |
| [#69](https://github.com/ProjectViVy/agent-vivy/pull/69) | `f2be7f614035927547571127bea73c25b516d614` | Skill bridge; Laputa #9 contract. |
| [#70](https://github.com/ProjectViVy/agent-vivy/pull/70) | `a44b0f69d6cf0954bd1ada1667c2a20715a9e02b` | Reconcile predecessor/unique changes against #79. |
| [#71](https://github.com/ProjectViVy/agent-vivy/pull/71) | `c2e6ac0656ed0e833bb794072f551dde9f40e189` | Reconcile predecessor/unique changes against #79. |
| [#79](https://github.com/ProjectViVy/agent-vivy/pull/79) | `b4c03754d2798404234ab7cd7f49bbb26e2bec63` | Recap candidate; current-main conflicts reported. |
| [#82](https://github.com/ProjectViVy/agent-vivy/pull/82) | `c158f2eaea29dc88ad6c6f50901c882b12a6ebe4` | Peer repair PR targets #79 backup branch, not main; reconcile once. |

All six are open/unmerged. Laputa [#9](https://github.com/ProjectViVy/laputa/pull/9) remains open at `b3a18e16a93c0b593e4a7f296b4e3ef24b040637`; [#10](https://github.com/ProjectViVy/laputa/pull/10) remains draft/open at `27da7b9e2d991857650b9e373627b65119a73eec`. Select one compatible published closure rather than overwriting one contract with another. The reported peer Laputa patch still needs exact published diff/SHA evidence before repinning.

## Later execution gates, not actions performed

Preserve #80 priority; reconcile #70/#71/#79/#82 supersession before assembly; integrate #69 with the selected compatible dependency closure. Preserve main's Nierland Studio gitlink update and current Toolkit/work-control fixes. Preserve Provider recovery intent, peer provenance/exclusion/readiness behavior and explicit Skill approval. Refresh generated evidence only on the final source.

Fresh workflow reads: #80 run [39099594755](https://github.com/ProjectViVy/agent-vivy/actions/runs/39099594755) and #79 run [39089954647](https://github.com/ProjectViVy/agent-vivy/actions/runs/39089954647) are completed/failure; #82 run [39118116632](https://github.com/ProjectViVy/agent-vivy/actions/runs/39118116632) is completed/action_required. None is a final combined-candidate pass.

Final readiness requires exact-SHA review and required CI, affected race/recovery/migration/reopen suites, actual native Windows Garden and both Recap browser bodies, and truthful disclosure of unavailable PostgreSQL/platform evidence. Keep rollback cleanup/event restrictions and normal-cancel limits. Do not combine historical branch-local PASS counts or silently waive red gates. Only genuinely affected work waits for dependency gates; DIVA installed acceptance, #66 activation and the full #58 rollout do not block unrelated BML/product work.

Peer fork CI at the same `c158f2eaea29dc88ad6c6f50901c882b12a6ebe4` also exists: [39116060177](https://github.com/songbai-vv/agent-vivy/actions/runs/39116060177), freshly verified completed/failure (updated 2026-10-18 10:20:20 UTC). The prior independent log audit reported backend success but aggregate failure with three actionable blockers: native Windows Garden `hnsw@v0.6.1/encode.go:304: undefined: renameio.TempFile` (BML 162 pass; ACTMEM 121 pass/1 skip); PNPM launcher missing `D:\a\agent-vivy\pnpm\bin\pnpm.mjs` before either Recap test body; and a new unclassified Web i18n key `approvals.quick.otherSession`. These detail findings are attributed to that prior inspected-log audit, not a new log execution/re-audit in this preparation. Retain the i18n classification blocker alongside Garden and launcher gates. Fork failure and upstream action_required are distinct receipts; neither is a passing final candidate.

## Preserved plans

- [BML](https://github.com/ProjectViVy/agent-vivy/blob/080186ae099ca96f8863c0bd8b35d6fb7bfa228f/docs/superpowers/plans/issue-81/bml/index.md)
- [Product gaps](https://github.com/ProjectViVy/agent-vivy/blob/a3b85c66ab718240d0209a6bd4a5c1661f891696/docs/superpowers/plans/issue-81/product-gaps/index.md)
- [Integration package](https://github.com/ProjectViVy/agent-vivy/blob/7815cd60326d120ec560b3217eedd974709b5ff8/docs/superpowers/plans/issue-81/integration/package.md) and [dependency ledger](https://github.com/ProjectViVy/agent-vivy/blob/7815cd60326d120ec560b3217eedd974709b5ff8/docs/superpowers/plans/issue-81/integration/dependencies.md)

Those packages remain unchanged at their original 7b42b292 baseline. #83 and #81's ownership overlay supersede coordination routing only; historical audit remains available as superseded evidence.

No implementation environment was launched. No product code, tests, dependency pins, existing PR branches or plan packages were changed; no merge/rebase, CI approval/rerun, paid API, production activation or issue closure was performed or authorized. Execution and merging require later authorization.

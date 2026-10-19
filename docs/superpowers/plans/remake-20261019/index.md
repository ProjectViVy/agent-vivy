# VIVY remake overall plan

Date: 2026-10-19. This is the sole status/dependency index of this package. [Design](../../specs/2026-10-19-remake-design.md) · [Acceptance](acceptance.md) · [Integration](integration.md).

## Execution update: donor branches

[Donor extraction receipt](extraction.md) records the authorized source separation and publication. The 15 planned lines plus a warming-history line are donor branches, not build/product-accepted features. Dedicated files are split; mixed files stay explicit shared references. Target-specific hunk/API adaptation remains blocked by accepted core B.

## Current facts and readiness

Donor main `9cd2801f8090723a652d3e835f72d83715b82121`; original A `202aac3885cb1479998127f0fb9004bea54aca57`; observed active core `c091ccf4351600737550b07611753dfaec846eef` (not accepted B). The frozen window has **394 reachable commits (314 main / 80 branch-only), 343 non-merges and 51 merges**. [Ledger](commit-ledger.csv) has full SHAs, both dates, candidate primary/secondary owners, merge state, shared files and duplicate hints. [Paths](commit-paths.json) supplies every commit's first-parent changed paths. [Snapshot](source-snapshot.json) supplies all frozen branch heads and source coverage limits. [Older prerequisites](outside-window-prerequisites.json) names seven pre-window candidates.

The source scan and candidate routing are complete. Final per-hunk ownership is **not** complete; no feature is implementation-Ready. R01 can continue as planning work without waiting for the other computer. R02/R03 and the feature implementation plans wait for actual accepted contracts. Do not create all worktrees from old main just to make the directory list look finished.

## Preparatory Stories

| ID | Outcome | Immediate inputs | Status | Plan |
|---|---|---|---|---|
| R00 | Immutable donor/ref inventory and coverage | — | Done: this package's read-only evidence | [R00](R00.md) |
| R01 | Review mixed commits, supersession and retained hunks | R00 | Ready for planning-only continuation | [R01](R01.md) |
| R02 | Accepted #91 core handoff and actual API inventory | R00 | Blocked: B is not yet supplied/accepted | [R02](R02.md) |
| R03 | Shared pins/schema/revision/Assembly intake contracts | R01, R02 | Blocked on concrete inputs | [R03](R03.md) |
| R04 | First smallest missing core-fix candidate integration | R10 | Blocked; recommended pilot | [R04](R04.md) |

## Feature lines

CSV primary_owner values are owner slugs; future branch refs always use remake/<slug>, except the existing external #91 owner. Candidate commit counts below are routing counts, not feature size, implementation readiness or an assertion that all hunks should survive. Merge commits have an integration-evidence owner, not another feature implementation.

| ID | Branch | Retained capability | Immediate predecessor | Candidate commits | Status / plan |
|---|---|---|---|---:|---|
| R10 | `remake/core-fixes` | Core correctness and durable interaction fixes | R03 | 29 | Blocked — [R10](lanes/R10-core-fixes.md) |
| R11 | `remake/provider` | Provider routing and model capability fixes | R03 | 27 | Blocked — [R11](lanes/R11-provider.md) |
| R12 | `remake/notebook` | Optional durable notebook | R03 | 10 | Blocked — [R12](lanes/R12-notebook.md) |
| R13 | `remake/reports` | Constrained reports and first-use entry | R12 | 5 | Blocked — [R13](lanes/R13-reports.md) |
| R14 | `remake/toolkits` | Native content package lifecycle | R03 | 22 | Blocked — [R14](lanes/R14-toolkits.md) |
| R15 | `remake/bml` | Durable BML memory and native operations | R03 | 14 | Blocked — [R15](lanes/R15-bml.md) |
| R16 | `remake/recap` | Subject continuity, exact evidence and governed Recap | R03 | 76 | Blocked — [R16](lanes/R16-recap.md) |
| R17 | `remake/cognition` | Cognitive capture, Mission and restart correctness | R03 | 61 | Blocked — [R17](lanes/R17-cognition.md) |
| R18 | `remake/skill-evolution` | Reviewed local Skill evolution | R03 | 3 | Blocked — [R18](lanes/R18-skill-evolution.md) |
| R19 | `remake/persona` | Laputa persona authority and onboarding | R03 | 3 | Blocked — [R19](lanes/R19-persona.md) |
| R20 | `remake/workflow` | Existing INOFY workflow correctness | R03 | 16 | Blocked — [R20](lanes/R20-workflow.md) |
| R21 | `remake/acp` | Restricted ACP stdio Face | R03 | 4 | Blocked — [R21](lanes/R21-acp.md) |
| R22 | `remake/tool-parallelism` | Same-batch tool parallelism | R03 | 2 | Blocked — [R22](lanes/R22-tool-parallelism.md) |
| R23 | `remake/chat-ui` | Chat usability and quick approval | R03 | 3 | Blocked — [R23](lanes/R23-chat-ui.md) |
| R24 | `remake/misc` | Miscellaneous maintenance and developer experience | R03 | 35 | Blocked — [R24](lanes/R24-misc.md) |

## Integration and retained EXP ownership

`remake/integration` is the one assembled-candidate/shared-contract lane, not a source to copy unrelated features from. Three optional branches may be instantiated when the corresponding #93 Epic is released: `remake/nudge`, `remake/warming`, `remake/steering`. Their authority remains the existing `epic-nudge`, `epic-warming`, `epic-steering` plans and #93. No new Epic/issue or duplicate implementation plan is created here. None is a blanket prerequisite for unrelated feature intake. Nudge/warming remain unselected in shipped default-family/lite/masks Recipes; do not infer a new steering default decision. The four warming-history commits are routed to that existing Epic; no in-window commit is required for an Epic to have retained scope.

`#91` owns the 29 active-core planning/implementation entries, so they are not replayed as remake capability work. The 51 merge rows require integration-resolution review. Misc contains general maintenance and multi-feature coordination; feature-specific docs/tests are routed with their feature wherever identifiable.

## Dependency waves and practical priority

Machine-readable immediate edges are in [dag.json](dag.json). This is dependency eligibility, not an order to run every eligible lane simultaneously.

1. R00
2. R01, R02
3. R03
4. R10, R11, R12, R14, R15, R16, R17, R18, R19, R20, R21, R22, R23, R24
5. R04, R13

Practical recommendation after core acceptance: run R01/R02/R03, then one smallest missing core-fix pilot. Next restore self-contained Provider fixes and Notebook; add Reports only after Notebook. Workflow/Persona/ACP/chat UI may proceed when their exact consumed contracts exist. Memory/Recap/cognition/Skill and Toolkit have higher shared ownership/data review cost and remain separately selectable batches. Tool parallelism comes after stable loop/approval behavior and a fresh native Eino capability check. Misc is incremental, never one unreviewed final dump. This ranking is a recommendation, not an invented dependency graph or cancellation of lower-priority work.

## Definition of package acceptance

Every in-scope reachable SHA remains accounted for; candidate and accepted hunk routing are distinguished. Each selected batch names retained behavior, actual target APIs, exact donor/base/head, data support, failure handling, executable steps and meaningful commands before it becomes Ready. Evidence is attached to the final combined candidate at the stage where it was actually run. No original feature issue is closed by a historical merge or by this planning artifact. The end state is independently resumable feature branches and accepted product behavior, not a bulk restoration of old main.

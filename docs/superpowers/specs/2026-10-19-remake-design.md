# VIVY remake: staged feature re-intake design

Date: 2026-10-19. Status: overall planning package for review; no feature implementation, branch publication, worktree creation or merge authorized by this document.

## Intent and requirements

The owner is refining the rolled-back core on another computer. Preserve every in-window reachable commit as inventory, split retained behavior into feature-owned `remake/*` branches/worktrees, and re-introduce small verified batches. General miscellaneous work has one `remake/misc` lane. Do not disturb the active #91 writer.

| ID | Required result |
|---|---|
| RM1 | Complete frozen reachable-commit coverage for the window; explicit outside-window prerequisites and evidence limitations. |
| RM2 | One primary candidate owner per commit, with many-to-many hunk/contract ownership after review; no dropped or twice-applied behavior. |
| RM3 | Feature-owned remake branches and isolated worktrees; miscellaneous work unified, with feature tests/docs retained with their features. |
| RM4 | Re-intake only onto an accepted stable-core candidate, preserving hash-free ownership, one production execution path, permissions and durability. |
| RM5 | Bounded focus-first checks and applicable exact-candidate integration/data/real-path evidence; missing gates stay pending. |
| RM6 | No hidden feature retirement, new default activation, unsupported data downgrade or second memory/runner/queue authority. |
| RM7 | Resume from this package on another machine using repository identities and refs, not this scratch filesystem or old chat. |

## Chosen architecture and alternatives

Use exact donor commits as historical evidence and port only the smallest retained behavior onto accepted core B; keep one feature owner, one integration owner for shared locks/schema/generated bindings, and native Git worktrees for isolation. Whole-PR replay is cheaper initially but carries mixed features and retired contracts; rebuilding every feature from scratch loses working code/tests and costs more. Behavior-level intake preserves useful implementation without forcing the old architecture back. This package is the Phase-C intake plan under #83/#81; it does not replace #91 or duplicate #93.

## Source authority and immutable observations

- Window: 2026-10-08T00:00:00+08:00 through 2026-10-19T23:42:20+08:00, inclusive, committer time. Author time is separately retained.
- Donor main: `9cd2801f8090723a652d3e835f72d83715b82121`. Original baseline A: `202aac3885cb1479998127f0fb9004bea54aca57`.
- Active core observed: `feat/issue-91-base@c091ccf4351600737550b07611753dfaec846eef`. Design observed: `feat/agent-loop-deletion@3fe696685bf14367ffd83945fe4221597552d83c`.
- The frozen reachable set contains 394 commits: 314 in main, 80 branch-only; 51 are merges.
- Seven older commits after A are listed separately, including #36/#37/#38; not silently included in the time-window count.
- Main contains 321 post-A commits. The historical 168 count belonged to an older main snapshot.
- A timestamp window is not evidence of complete historical branch visibility. Deleted/unreachable branches, reflogs on the other computer, local-only/unpushed changes, fork-only PR tips not reachable from these heads, and changes after the cutoff are outside this scan. Fetch additional known refs only if a ledger gap points to them; append an explicitly versioned delta.
- Full refs and metadata: source-snapshot.json. Every counted SHA has one CSV row and a path record. Candidate ownership is provisional; hunk acceptance remains R01. Stable patch IDs provide nine duplicate groups as historical review hints only, never product/runtime identity.

## Ownership and authority contracts

#91 owns stable loop deletion, data format changes and core boundary acceptance. #93 owns nudge/warming/steering extraction; these plans are referenced, not rewritten. #83 owns intake sequencing and integration; #81/original issues retain capability acceptance. `remake/integration` owns the assembled candidate, shared dependency pins, target migrations and generation producers. A feature lane supplies one minimal reviewed delta and relevant tests. One active writer per checkout; shared-file edits are sequenced through the integrator. Do not invent logical dependencies solely because two files overlap.

Each lane handoff names exact donor/base/head, retained/omitted/covered/retired hunks and rationale, actual APIs/data support, private dependency closure, test commands/outcomes and pending evidence. Receipt state is proposed -> mapped -> target-ready -> implemented -> focused-verified -> integrated -> accepted. Source presence, merge status and functional acceptance are different states.

Commit dispositions after R01: retained_delta; already_in_core; superseded_or_duplicate; retired_by_approved_scope; historical_docs_or_tests; blocked_dependency; mixed_split_required. A commit can contribute multiple hunks to different lanes; every hunk has one integrating owner. Merge commits require first-parent and resolution review, not automatic cherry-pick -m. Failed attempts followed by repair are compared against the final source (notably the Skill service.go deletion/restoration pair).

## Feature boundaries and dependencies

The index lists 15 feature/maintenance lanes and independent #93 experiment references. Reports requires accepted Notebook. Recap/BML share a reviewed sink boundary but do not require each other's entire implementation by default. Skill evolution consumes whichever accepted cognitive contract actually exists; R02 creates a dependency on cognition only if a required missing interface is demonstrated. ACP uses accepted FaceHost/Assembly; Toolkit consumes D07. Core-fixes contains separately reviewable correctness batches, not a mandatory combined rewrite. Provider thinking metadata is owned by Provider; chat-ui consumes it and keeps independent quick-approval/compact changes.

Build-time SDK/Recipe/Module composition is core/shared infrastructure, whereas user-installed Toolkit/Bundle content belongs to `remake/toolkits`. Worktree separation alone is not runtime modularity. An optional capability must be explicitly selected, obey existing typed ports and preserve physical omission where the accepted product contract requires it.

## Shared code, persistence and external dependencies

The integrator serializes go.mod/go.sum, Laputa source lock, shared Service/App bindings, paired migrations, Recipe composition and generated Assembly/UI. Feature authors propose minimal edits or a defined integration patch; no competing repins and no hand-editing generated projections. Each worktree has its own adjacent Laputa directory so ../laputa does not point to another active lane's checkout. Studio submodule state is per checkout; record exact gitlinks.

Historical schemas: 036-037 Notebook, 038-040 Reports, 041-043 Bundle/Toolkit, 044-046 subject evidence/disposition. This is provenance, not target migration allocation. #91's target schema/BML format supersedes historical assumptions. Do not copy unused migrations to reach a number, rewrite released SQL, renumber a published migration, reset stores or install obsolete readers. R03 names each supported source lineage and target conversion path, reserves new paired IDs serially, and requires copy-based fresh/upgrade/reopen/failure rollback/parity proof for the affected data. An unknown/newer store fails before writes. A later feature may need its own converter fragment even if #91 acceptance passed.

Use one compatible published Laputa closure for selected contracts; do not blindly take each donor pin. Eino/Claude alignment is already being handled by #91 c091ccf4; compare its exact behavior before classifying old donors as covered. Extra recovery cleanup and Docker packaging differences remain review candidates. AgenticModel/openai-responses migration is research/planning evidence, not shipped behavior to restore.

## Eino capability check

Primary source inspected at v0.9.21 on this planning turn:
- [adk/runner.go](https://github.com/cloudwego/eino/blob/v0.9.21/adk/runner.go): NewRunner, TypedRunner.Run and ResumeWithParams.
- [adk/chatmodel.go](https://github.com/cloudwego/eino/blob/v0.9.21/adk/chatmodel.go): NewChatModelAgent and MaxIterations.
- [compose/tool_node.go](https://github.com/cloudwego/eino/blob/v0.9.21/compose/tool_node.go): ToolsNodeConfig.UnknownToolsHandler and ExecuteSequentially (native sequential/parallel selection).
These APIs justify reusing the existing native runner/tool boundary, not adding one. They do not by themselves prove VIVY approval/work-tool ordering or recovery parity; the parallelism lane must demonstrate the remaining gap at the actual accepted pin before porting a custom barrier. Eino imports remain confined to runtime/provider. No library upgrade is selected by this package.

## Lifecycle, failure and rollback

Prepare one accepted core B and disposable home; admit through the existing Service/RPC/policy/approval path; persist ownership/outcome before exposing success; allow native Eino to continue; settle exactly one terminal and idle cleanup. Feature failure remains local and visible without restoring observer handoff ownership. Approval mismatch, stale edit/revision, omitted capability and unknown effect outcome keep their existing typed failure behavior. Never normalize away approved original inputs or automatically retry an unknown side effect.

Rollback source/artifact and data compatibility separately. A source revert cannot safely reopen a newer schema without an explicit supported path. Backups and conversions use disposable copies in verification; no live store is accessed by this task.

## Verification and cost policy

Follow acceptance.md. Do inexpensive scope/link/ledger checks while planning; during implementation run the smallest affected regression cycle plus required shared-contract gates, then integrate once on an exact candidate. Do not rerun unchanged expensive suites without a new risk or changed candidate. No invented latency/LOC target. Capture comparable tool-loop latency, model/tool call counts and optional-package activity before/after each relevant batch; a regression is reviewed, never hidden in old receipts.

Owner validation tiers observed on 56c58daa apply explicitly to the #91 base cut: L0 docs; L1 focused package checks; L2 shared-contract suite; one final just ci; PR-stage evidence remains pending. They are not an automatic waiver for every remake feature. For remake, apply current target repository rules and the batch's affected real-path/data gates; any future reduction needs its actual authorization. Distinguish engineering-ready from release/product-accepted.

## Scope of this delivery

Planning artifacts and read-only inventories only. No feature code, dependency install, worktree/branch creation, tracker mutation, paid API, merge, deployment or original issue closure. No execution mode is selected for feature implementation. Default recommendation is one integrating writer and selective independent review; not one agent per lane. Supermanagement's three referenced appendices were unavailable, so this package follows its readable main skill and repository evidence rather than claiming those appendices were inspected.

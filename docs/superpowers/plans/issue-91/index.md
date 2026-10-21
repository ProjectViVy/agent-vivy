# Issue #91: Complete deletion planning package

Issue: [#91](https://github.com/ProjectViVy/agent-vivy/issues/91) · Current baseline: `stable@fde3c1f96f086507a35e52689e58fc6e4db981b4` · Earlier planning branches: `feat/agent-loop-deletion`, `codex/agent-loop-deletion`.

**Current outcome (2026-10-21):** D01, D02, D03, D06 and D08 are accepted through the owner-authorized scoped WSL route; the Story table below owns their evidence and remaining boundaries. #91 keeps deletion/contraction work; #93 retains nudge, warming and steering pending first-party plugin extraction. C11 supersedes the earlier EXP placement/default-exclusion plan. Final upgrade publication and #91 closure remain open.

Phase/handoff: [stable loop and gated feature re-intake](../../../plans/2026-10-19-stable-loop-reintake-handoff.md). This supplement sets phase applicability, not a second technical specification.

## Authority and baseline

- Shared design: [complete deletion plan](../../../plans/2026-10-19-complete-deletion-plan.md), especially §§10–11 C1–C11 and detailed architecture. Reuse this authority; do not create a competing spec.
- Source evidence: [core deletion insight](../../../research/2026-10-19-agent-loop-core-deletion-insight.md) and [slimming review](../../../research/2026-10-19-agent-loop-slimming-review.md). These are source findings and proposals, not product acceptance.
- Plan authored against `e96092966cfbfa443648374948e7e7e16dd01683` (2026-10-18). **Implementation baseline A:** `202aac38` (2026-10-07, last stable and benchmarked pi-parity closeout); the original plan's docs-only commits were replayed onto that baseline. File references from the 10-18 tree are re-anchored in the [baseline-A audit](../../../research/2026-10-19-baseline-a-reanchor.md); do not cite a 10-18-only path as present.
- Root/UI AGENTS and relevant plugin/kernel/Eino skills were read. Two independent read-only domain reviews checked runtime/approval and assembly/persistence. No production state was accessed; the historical external audit used the pins recorded in §11.10. INOFY's pin is unchanged; current baseline-A Laputa/Studio pins differ and their API findings remain unverified until inspected. No inspection of the current pins is claimed by this supplement.
- Planning docs/commit messages are English. The original 2026-10-19 delivery was planning-only. The current owner requests prompt kickoff of #91 and explicitly retains the #93 implementation pending later pluginization; applicable predecessors, source revalidation and product evidence remain required.

## Current scope and near-term kickoff (2026-10-20)

The owner wants the other #91 work to start promptly and keeps the three #93
components on the current branch, with pluginization explicitly required.
[C11](../../../plans/2026-10-19-complete-deletion-plan.md#101a-c11-retained-behavior-and-required-first-party-plugin-extraction)
owns the revised scope: preserve current behavior, use ordinary first-party
plugins, and discard the earlier EXP placement/default-exclusion instructions.
The owner-confirmed sequence is to retain the implementation now and carry out the
actual extraction later under #93. Do not label existing core code a completed
plugin.

| Order | Work | Concrete completion boundary |
| --- | --- | --- |
| 1 | D00 re-anchor and causal regression baseline | D01 starts at `stable@ce548925` with scoped source/pin checks and real App causal regression. Other cuts retain their own D00 prerequisites. Human smoke is useful baseline evidence, not full #91 acceptance. |
| 2 | D01, then D02 | Remove default digest audit, then align completion/request producers and Face consumers without custom hashes. Verify each bounded cut; retain mandatory Journal behavior and framing. |
| 3 | D03 and D06, sequenced through shared owners | Preserve exact tool/approval intent while removing digest binding; simplify source/Assembly/UI generation. Keep current feature selection. |
| 4 | D04, then D05 | Remove the extra result handoff/FNV with coaching intact; remove only demonstrated warming/steering coupling. Re-anchor their proposed factory bindings to C11 before execution. |
| 5 | Remaining durable owners, minimal and final acceptance | D08–D12 follow their existing dependencies and finite conversion requirements; D13/D14 integrate the exact candidate. D12 may start earlier after its own D00 gate when capacity permits. |

This is a priority order, not a replacement dependency DAG or permission to
skip migration/acceptance gates. No waiting for #93 package extraction before
D01/D02. D07 remains outside the baseline executable DAG. Use one write lane;
do not merge old main/remake donor branches to reconstruct this baseline.

Eino capability check: stable already pins v0.9.21 and uses
`adk.NewTypedChatModelAgent[*schema.AgenticMessage]`, `adk.NewTypedRunner` and
`adk.TypedBaseChatModelAgentMiddleware[*schema.AgenticMessage]`. Preserve these
native owners; revalidate any older text-generation signature at the affected
cut. The provider/engine migration is already present and is not repeated.

Latest record: [scope and kickoff verification](../../../logs/2026-10-20-issue91-stable-kickoff/verification.md).
Earlier selection and retained-feature logs remain historical evidence.

## Baseline A and retained-package epics (2026-10-19)

The owner chose `202aac38` (2026-10-07) as the implementation baseline because it is the last stable pi-parity state that passed benchmark runs; the original comparison through `e9609296` counted 168 commits and 30 first-parent merges, a historical inventory rather than current-main distance. Main was observed at `c57dfd322a50daf5ac2365369e84f81f1733a8b2` after PR #84 merged on 2026-10-19. Later work remains capability backlog/donor evidence to re-review and re-introduce in gated batches, not assumed present or discarded. The plan's docs-only commits were replayed onto this baseline on `feat/agent-loop-deletion`; the original `codex/agent-loop-deletion` branch remains history.

Delivery split: #91 remains the base cut (hash deletion, core contraction, seam).
#93 owns actual first-party plugin extraction. The following component tracks
are retained, but their experimental extraction drafts require re-planning:

| Epic | Scope | Current disposition | Stories |
| --- | --- | --- | --- |
| [E-NUDGE](../epic-nudge/index.md) | Coaching plugin, parity and one implementation after cutover | Retain stable behavior; ordinary plugin extraction later | N00–N03 require re-planning |
| [E-WARMING](../epic-warming/index.md) | Warming plugin, accounting and recovery | Retain current availability/configuration; ordinary plugin extraction later | W00–W03 require re-planning |
| [E-STEERING](../epic-steering/index.md) | Steering plugin and same-Run queue/recovery | Retain stable behavior; ordinary plugin extraction later | S00–S03 require re-planning |

Baseline evidence: [baseline-A re-anchor audit](../../../research/2026-10-19-baseline-a-reanchor.md) and [epic-split delivery record](../../../logs/2026-10-19-baseline-a-epic-split/summary.md). Re-introduction of post-baseline capabilities is the separate gated track in [#83](https://github.com/ProjectViVy/agent-vivy/issues/83), with [#81](https://github.com/ProjectViVy/agent-vivy/issues/81) retained as capability backlog. Phase A stabilizes the loop first; missing future batch coordination is not its blocker. Old PRs are evidence/donor diffs, never blanket merge instructions.

## Requirements and coverage

| Requirement | Observable outcome | Owning Stories |
| --- | --- | --- |
| R1 | Completely remove Vivy-owned hash implementations, state, protocols, fields, generators, dead code and active requirements; optional final artifact label is not required. | D01–D12 |
| R2 | One production model -> real tool -> exact result/call ID -> next model request -> final terminal/idle path, using existing Eino. | D02–D05, D08, D13–D14 |
| R3 | Preserve approved intent, server permission/scope, durable ownership/outcomes, conflicts and no blind effect replay. | D03–D04, D08–D12; D07 only on later re-intake |
| R4 | Finite supported data transition, fresh/upgrade/reopen/failure rollback and SQLite/Postgres/BML parity; no data reset. | D00, D03, D08–D12, D14; D07 only on later re-intake |
| R5 | Delete selected mechanisms through their complete closure. Retain current nudge/warming/steering behavior and require ordinary first-party plugin extraction under C11; no duplicate execution path. | D01–D12, D14; #93 owns actual extraction |
| R6 | Regeneration and supported official Recipe/build variants preserve simplified contracts and current capability availability. | D06, D13–D14 base path; #93 owns eventual plugin composition/omission |
| R7 | Usable minimal, tiny real-token chat Face and complete runnable minimal-plus-feature examples after core contraction. | D13–D14 |
| R8 | Independent behavior and residue reviewers, human real-token E2E and required exact-candidate CI before closure. | D14 |

## Architecture decisions and delivery prerequisites

Hash retirement and the three retained capabilities remain settled. C11 revises
their plugin placement and retention policy; the earlier §11 experimental
bindings require re-anchoring before the affected implementation. Missing
delivery/test evidence is explicit and is not supplied by architecture text.

| Gate | Recommendation and exact decision | Blocks |
| --- | --- | --- |
| G1 Product and durability | Retain the typed T1 factory/Host design and direct producer outcome -> native ToolMessage -> WrapModel ownership. Re-anchor proposed bindings to C11 and stable's typed agentic APIs. Required effect/schedule/final/terminal/accounting commits stay mandatory; remove proven duplicate authority only. | D04/D05 accepted predecessors and affected binding revalidation; #93 package extraction is later |
| G2 Supported data and finite upgrade | Settled matrix and target schema (§11.7–11.9): supported old normalizer -> 035 -> copied target T, BML2, explicit NULL/unbindable-input refusal and old-pause terminal mapping, separate Studio ledger. D14 assembles finite artifact. | Disposable fixture proof, all converter fragments and upgrade artifact delivery before final release |
| G3 Release consent | Later D07 re-intake contract: opaque ArtifactID + release sequence/config revision; scoped fresh consent, preserved history/unknown effects (§11.7). No bundle/toolkit subsystem or converter at baseline A. | The batch that re-introduces bundle/toolkit, not phase-A loop work |
| G4 Exact content and cursor trust | Bounded before/selection/presentation bytes in existing owners; ContinuityStore atomic presentation receipt, session/truncation pruning, separate transfer TTL. Stateless untrusted cursor reauthorized per page (§11.6). | D09 implementation/privacy/concurrency proof |
| G5 External/native boundary | Native dependency/TLS/Git/bundler boundary stays. INOFY audit pin is unchanged; Laputa/Studio audit findings are historical and unverified at actual baseline-A pins (§11.10). No opaque-hash exception. | Inspect current consumers first; confirmed incompatible seams need reviewed upstream delivery in D10/D11/D06. Unrelated loop proof is not blocked. D07 is later re-intake only. |

No rewrite of AGENTS, current product requirements or mandatory gates is performed by this planning delivery. Hash-related contradictory clauses are changed with the authorized implementation cut. Workflow/gate reductions need their own explicit authorization; they cannot be inferred from a request for speed.

## Epics

| Epic | Coherent outcome | Stories |
| --- | --- | --- |
| E0 | Scope, contracts and finite support boundary settled | D00 |
| E1 | Remove audit/completion/request evidence authority | D01–D02 |
| E2 | Preserve exact ownership and supported data without hashes | D03, D08–D09, D12 |
| E3 | Simplify core continuation and settle the loop-exp seam; package delivery split into E-NUDGE/E-WARMING/E-STEERING | D04–D05 |
| E4 | Hash-free explicit source/Assembly composition; release checkpoint for later re-intake | D06; D07 explanatory only |
| E5 | Preserve selected feature behavior with owner revisions | D10–D11 |
| E6 | Usable minimal and complete real-path examples | D13 |
| E7 | Independently accept the integrated candidate | D14 |

## Story index (sole status/dependency record)

Planned = concrete plan exists, awaiting implementation handoff or accepted prerequisites. Blocked = exact decision/data/dependency missing. Ready requires settled shared decisions, accepted immediate outputs and implementation authorization. Accepted requires observed evidence within the owner-selected scope; it does not imply overall #91 or full-product release acceptance.

| Story | Epic / requirement | Outcome | Immediate predecessors | Plan | Status | Evidence / blocker |
| --- | --- | --- | --- | --- | --- | --- |
| D00 | E0 / R1–R8 | Freeze scope and finite upgrade contracts | — | [D00](D00.md) | D01/D06/D12 scoped closure revalidated; other cuts pending | D06 source ownership/typed Ports/Generation identity, affected persistence and actual pinned Studio consumers inspected; typed Eino owners retained. [Evidence](../../../logs/2026-10-21-issue91-d06/review.md). D12 supported metadata/BML formats and independent finite fragments are also revalidated; this does not accept unrelated data/external gates. |
| D01 | E1 / R1,R5 | Delete digest audit and unused experimental hash code | D00 | [D01](D01.md) | Accepted (scoped) | Exact integrated cc74e7d8 passes 17 causal/hooks/nudge/recovery cases, 13 historical owner cases, strict 23-Provider reproduction and required WSL wiring/kernel/affected Face/Web gates. All 55 historical failures mapped. [Final evidence and limits](../../../logs/2026-10-21-issue91-d06/verification.md). Historical full-product CI failure remains recorded; release/#91 closure separate. |
| D02 | E1 / R1,R2,R5 | Remove model completion and request hash protocols across Faces | D00 | [D02](D02.md) | Accepted | Stable integration, byte-only live/replay contract and prerequisite repairs pass the owner-authorized WSL wiring/kernel/affected Face/Web route, real PostgreSQL, strict conformance and race. Independent findings have RED/GREEN corrections. [Final evidence and limits](../../../logs/2026-10-21-wsl-kernel-ci/verification.md); historical failures retained, no full-product release or #91 closure claimed. |
| D03 | E2 / R1,R2,R3,R4 | Bind tool operations and approval to exact private inputs | D00 | [D03](D03.md) | Accepted | Exact private inputs, typed atomic outcomes, v2 approvals, native cached sibling recovery and paired 036 conversion fixtures pass SQLite/real Postgres, hard-kill/race, scoped WSL kernel and real split-browser checks. Independent review clear after corrections. [Evidence](../../../logs/2026-10-21-issue91-d03/verification.md). D08/D12/D14 retain final upgrade publication. |
| D04 | E3 / R1,R2,R3,R5,R6 | Settle the typed seam and remove nudge FNV/handoff with coaching retained | D02, D03, D06 | [D04](D04.md) | Blocked | Accepted D02/D03/D06 outputs; re-anchor factory bindings to revised C11 and typed agentic APIs |
| D05 | E3 / R1,R2,R3,R5,R6 | Settle the warming/steering seam and remove confirmed hash/duplicate coupling | D04 | [D05](D05.md) | Blocked | Accepted D04 output; reconcile proposed bindings while preserving stable behavior |
| D06 | E4 / R1,R5,R6 | Delete source, Assembly and UI hash evidence generation end to end | D00 | [D06](D06.md) | Accepted (scoped) | Exact integrated cc74e7d8 passes full SDK (1,079 cases), all ten Recipes and external fixtures, 77-file double generation, scoped WSL/Face/Web gates and real split-browser checks. Actual Studio consumers fixed, separate ledger/core037 copied conversion and native Studio build verified; independent reviews clear. [Evidence and identities](../../../logs/2026-10-21-issue91-d06/verification.md). Compatible Studio gitlink is local-only; final upgrade/release publication remains separate. |
| D07 | E4 / R1,R3,R4,R5 | Replace bundle and toolkit content addressing with owned releases | — | [D07](D07.md) | Retained — explanatory | Owner decision 2026-10-19: baseline A has no bundle/toolkit subsystem, so no deletion here; D07 is the re-intake checkpoint (resolve content-addressed identity/consent before a batch from `backup/main-pre-rollback-20261019` merges) |
| D08 | E2 / R1,R2,R3,R4 | Delete Host and admitted-snapshot hashes, bind checkpoints directly | D03 | [D08](D08.md) | Accepted | Host direct values, private immutable prompt/policy, v2 Run/snapshot checkpoint binding and its original paired037 copied conversion pass WSL wiring/minimal kernel, real PostgreSQL (123 cases, zero skips), race and split-browser approve/deny/hard-restart checks. Independent review clear after closing the stale continuity TODO. [Evidence and limits](../../../logs/2026-10-21-issue91-d08/verification.md). The data lane consumes that unchanged business SQL as paired041 after D06/D09 allocations; exact integrated3cc284dd passes scoped Linux/kernel/race,142 real-PG cases,Face/Web/SDK and actual Provider checks. D14 retains final upgrade publication. |
| D09 | E2 / R1,R3,R4,R5 | Remove file, history, reference and deliverable hash authority | D03 | [D09](D09.md) | Implemented — acceptance open | Accepted D03 and scoped D06 integrated; file/approval exact-byte slice with paired038 passes scoped Linux/real-PG/Face/Web checks. Stateless cursor v2 passes scoped Linux/real-PG/UI/race checks. Shared continuity exact-input receipts with paired039 pass scoped Linux/real-PG/Face/Web/race checks. Exact-preview references pass scoped Linux/real-PG/Face/Web/SDK/Provider/race checks. Retained presentation/immutable transfer/export closure with paired040 passes exact a48b5495 Linux/real-PG/Face/Web/SDK/Provider/race checks; human acceptance remains open. D12 standard3015 check is owner-deferred. |
| D10 | E5 / R1,R3,R4,R5 | Preserve skill, memory and cognitive revision CAS without hashes | D08, D12 | [D10](D10.md) | Blocked | D08 accepted; D12 acceptance open. Actual ff3936f4 Capture/effect/memory/persona/StrategyDigest APIs remain incompatible; reviewed upstream delivery required. [Current-pin evidence](../../../logs/2026-10-21-data-lane-d08-integration/notes.md) |
| D11 | E5 / R1,R3,R4,R5 | Remove baseline Work, workflow and notes business digests | D03, D08 | [D11](D11.md) | Blocked | D03/D08 accepted. Actual INOFY 71e2c9bb requires Program/input/answer/checkpoint digests; reviewed hash-free delivery and integration evidence missing. [Current-pin evidence](../../../logs/2026-10-21-data-lane-d08-integration/notes.md) |
| D12 | E2 / R1,R3,R4,R5 | Remove migration checksums and convert the independent BML format | D00 | [D12](D12.md) | Implemented — acceptance open | Accepted D03 integrated; BML2/direct MEMRULES, final metadata and separately reviewed finite fragments pass scoped WSL/kernel/real-PG/bridge checks. Required3015 smoke deferred by owner on 2026-10-21; D06 has stopped its task servers; D14 final artifact/release remains separate. [Record](../../../logs/2026-10-21-issue91-d12-bml/summary.md). |
| D13 | E6 / R2,R6,R7 | Ship usable minimal, a tiny real-token Face and runnable examples | D05, D06, D08 | [D13](D13.md) | Blocked | Core contraction accepted; repository gate changes require explicit approval |
| D14 | E7 / R1–R8 | Accept one final candidate through independent behavior and residue audits | D01, D09, D10, D11, D13 | [D14](D14.md) | Blocked | All predecessors accepted; real-token environment; fresh independent auditors |

## Immediate dependency contracts

D00 supplies C1–C11/§11/G1–G5, not code; required upstream version/pin deliveries are external prerequisites in §11.10. D02 supplies the aligned live/replay completion contract. D03 supplies exact invocation/approval ownership. D04 supplies the inert loop-exp seam, the bounded exact-tuple nudge window and removal of the consumer-owned handoff; D05 supplies the warming/steering seam and hash-only coupling removal. Both record the interim core-owned behavior; package extraction and C11 selection behavior are supplied by the three epics, not by #91. Other scope follows the existing plan. D06 supplies simplified source/generation producers. D07 records later immutable release/consent obligations and supplies no baseline output. D08 supplies admitted values/checkpoint binding. D12 supplies BML v2 and final metadata ownership. D09/D10/D11 supply accepted feature behavior and migration evidence. D13 supplies runnable minimal/Face/examples. D14 consumes all integrated outcomes, not author completion claims.

Edges are prerequisite inputs, not a priority ranking. Wave two starts after the common D00 scope freeze. D12 is an independent format/metadata cut after that freeze; feature-specific SQL is owned by its individual Story, not a late central column-drop task.

D06 -> D04 supplies simplified source/Assembly and the inert factory seam.
Reconcile its bindings with C11; #93 delivers actual plugin selection and
physical omission later.

D03 -> D08 supplies private invocation/approval bindings with no D07 dependency. Two transitive-looking edges carry separate real inputs and are intentional: D03 -> D11 supplies claim/replay and intended-request semantics independently of D08's admitted snapshot; D06 -> D13 supplies pack/generator APIs independently of D08's runtime admission contract. All other redundant transitive edges are omitted.

## Topological waves

1. D00
2. D01, D02, D03, D06, D12
3. D04, D08, D09
4. D05, D10, D11
5. D13
6. D14

D07 is outside the executable baseline DAG/waves: it has no baseline predecessors, output or acceptance dependency. The 14 executable Stories form 23 edges and six waves; D07 remains the fifteenth document as a later checkpoint.

Waves describe dependencies, not permission for simultaneous edits. No elapsed-time or speedup estimates are claimed. Default execution is one write lane on the specified branch; fresh read-only reviewers can be concurrent where useful. The package does not mandate one implementation agent per Story.

## Shared-file scheduling

- D02/D03/D04/D05/D08/D09/D11 touch `service.go`, mapper/payloads/domain DTOs or RPC/control: serialize or split shared edits through one integrating owner; do not add fictional logical edges solely for file conflicts.
- Baseline D03/D08–D12 own their applicable feature migrations; D07 reserves no baseline SQL or converter fragment; allocate paired numeric IDs serially above the current maximum (035 at planning; the 10-18 tree reached 043, not present at baseline A). D12 alone owns migration runner/metadata. The data lane D12 -> D09 -> D10 -> D11 coordinates shared-file integration and serial allocation with the other schema owners; D03's in-progress 036 is not copied or reassigned. Do not rewrite or renumber a released migration.
- D06 owns descriptor/manifest/generator contract and current hash-related instruction edits. Baseline D08–D13 consume it; a later D07 batch also consumes its accepted contract; later feature fields may require another generated diff, always through the same producers.
- D07 is retained as the re-intake checkpoint for bundle release/consent and masks' release reference (no baseline-A deletion action). D10 owns skill/memory/mask content revision behavior. D08 owns admitted snapshots/checkpoint binding; do not create duplicate private snapshot services.
- D09 owns reference/download DTO and `HistoryPicker`; D02 owns live completion state in the shared UI store. Source Module UI is edited before staging, never generated copies.
- A second concurrent implementation lane requires repository worktree isolation and its own pinned Laputa checkout. One active write lane per checkout; no shared mutable sibling dependency.

## Acceptance matrix

| Risk / outcome | Required evidence | Owner |
| --- | --- | --- |
| Causal tool continuation | Real isolated nonce read; captured second model request contains exact result + call ID; final text, one terminal, idle | D04, D13, independent D14 |
| Intent and crash safety | Changed original input conflicts; canonical approval binding; sibling resume; admitted/claimed/effect/completed crash matrix; no blind replay | D03, D08–D11 |
| Phase-A retained behavior/seam | Single existing core coaching/warming/same-Run steering remains real behind the interim seam, no dual implementation; causal loop, cancellation/durability/budgets remain authoritative. No C11 omission claim. | D04–D05, D13–D14 baseline candidate |
| Later first-party plugin integration | Preserve shipped behavior through extraction; selected plugins work, all three compose, deliberate omissions omit implementation/control. Current core retention is not plugin completion. | #93 and the re-planned component tracks; not an extra #91 predecessor |
| Content/privacy | Same-length stale edits conflict, authorized selection exact, foreign cursor cannot gain scope, immutable chunks, no private arguments in public events | D03, D09 |
| Supported data | Approved source matrix -> target on disposable SQLite/Postgres/BML, reopen/failure rollback; unknown effects/old pauses explicit; no old runtime reader | D03, D08–D12; D07 only on later re-intake |
| Generation/residue | Source-first removal, double regeneration idempotent with same ID, official Recipe/artifact inspection, exact remaining-match dispositions | D06, D14 |
| Retained baseline capabilities | Real reviewed skill/memory and Work/workflow/notes behavior under simplified contracts; actual upstream seams retain their gates | D10–D11 |
| Later capability re-intake | Bundle/toolkit release consent/rollback, report/new-notebook and other post-baseline capabilities get complete reviewed authority/data/acceptance closure in their batch | #83 coordination, #81 backlog; D07 where applicable |
| Usable baseline minimal | Runnable minimal + feature examples, actual existing Module omission contract, production tiny Face with human real tokens; omission of the three retained components belongs to later #93 acceptance | D13–D14 |
| Final closure | Two fresh non-author subagents accept behavior and residue at exact final candidate; just ci + real Postgres + BML + affected real-path checks | D14 |

## Detailed architecture update

The [shared design §11](../../../plans/2026-10-19-complete-deletion-plan.md#11-detailed-target-architecture) records module/import direction, exact factories/Hosts, normal/retry/cancel/resume lifecycle, target schemas, finite source matrix, copied-target cutover and historical/current pin provenance with affected consumer checks. [Architecture verification](../../../logs/2026-10-19-issue-91-detailed-architecture/verification.md) records this doc-only update. No upstream source/pin, runtime, schema or generated product file changed.

## Planning verification and change handling

Planning checks: exact baseline comparison, local referenced-path validation, relative-link validation, requirement/Story coverage, unique IDs/known DAG endpoints/no self-edge/no cycle/no redundant transitive edge, derived waves and `git diff --check`. Results live in [the planning verification record](../../../logs/2026-10-19-issue-91-planning/verification.md) and the [baseline-A epic-split record](../../../logs/2026-10-19-baseline-a-epic-split/verification.md).

This supplement's focused documentation verification is recorded in [the stable-loop handoff record](../../../logs/2026-10-19-stable-loop-handoff/verification.md). Earlier dated logs are historical evidence and do not override corrected phase applicability.

Product checks were not run: no implementation changed. No `just ci`, generation, Postgres, live-provider, installed-data or post-deletion-audit result is inferred from document checks. Future execution must prepare the repository-supported environment and record missing prerequisites honestly.

A shared-contract change invalidates downstream readiness and affected evidence. Revise this design/package, not a parallel report. Retained and retired scopes, support versions and public acceptance changes return to owner review. Local implementation details within the accepted contract can be corrected autonomously. The final implementation commit must identify exactly what was verified; a planning commit does not close #91.

## Shared migration coordination at tested3cc284dd

This data lane owns the integrated catalog; Story owners retain business schema
and finite conversion semantics. Existing001–040 remain unchanged. No number
after041 is reserved and041 is not a final converter target declaration.

| Owner | Integrated pair | Provenance |
| --- | --- | --- |
| D03 | 036_tool_operation_private_binding | Accepted D03 fragment. |
| D06 | 037_generation_identity | Accepted scoped D06 fragment. |
| D09 | 038_file_approval_before_state | Exact private file approval/read state. |
| D09 | 039_continuity_intended_input | Exact private normalized receipt input. |
| D09 | 040_deliverable_presentations | Private presentation bytes; no guessed historical backfill. |
| D08 | 041_admitted_checkpoint_binding | Exact unchanged SQL from accepted dd5610c3 original provisional037. |

Pending integration coordination: a final read-only root check found another
active D06 merge and a provisional038_generation_identity draft. That conflicts
with this tested catalog and is not accepted by this lane. Root reconciliation
must consume one coordinated catalog rather than copy competing numbered DDL.
The root checkout remains owned by its active write lane and was not edited
here. [Integrated verification](../../../logs/2026-10-21-data-lane-d08-integration/verification.md)
records the exact candidate; D14 retains final finite assembly/acceptance.

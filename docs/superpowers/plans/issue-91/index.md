# Issue #91: Complete deletion planning package

Issue: [#91](https://github.com/ProjectViVy/agent-vivy/issues/91) · Plan branch: `feat/agent-loop-deletion` (baseline A) · Original plan branch: `codex/agent-loop-deletion`.

**Current outcome:** design amendment, 14 executable baseline Story plans plus explanatory D07 (D00–D14), immediate dependency DAG, migration gates and independent acceptance plan, re-anchored to the 2026-10-07 stable baseline (`202aac38`) with the three retained experimental packages split into independent epics (E-NUDGE, E-WARMING, E-STEERING). This is a planning deliverable; no runtime deletion, migration, regeneration, product CI, post-deletion audit or incident-root-cause proof is claimed.

Phase/handoff: [stable loop and gated feature re-intake](../../../plans/2026-10-19-stable-loop-reintake-handoff.md). This supplement sets phase applicability, not a second technical specification.

## Authority and baseline

- Shared design: [complete deletion plan](../../../plans/2026-10-19-complete-deletion-plan.md), especially §§10–11 C1–C11 and detailed architecture. Reuse this authority; do not create a competing spec.
- Source evidence: [core deletion insight](../../../research/2026-10-19-agent-loop-core-deletion-insight.md) and [slimming review](../../../research/2026-10-19-agent-loop-slimming-review.md). These are source findings and proposals, not product acceptance.
- Plan authored against `e96092966cfbfa443648374948e7e7e16dd01683` (2026-10-18). **Implementation baseline A:** `202aac38` (2026-10-07, last stable and benchmarked pi-parity closeout); the original plan's docs-only commits were replayed onto that baseline. File references from the 10-18 tree are re-anchored in the [baseline-A audit](../../../research/2026-10-19-baseline-a-reanchor.md); do not cite a 10-18-only path as present.
- Root/UI AGENTS and relevant plugin/kernel/Eino skills were read. Two independent read-only domain reviews checked runtime/approval and assembly/persistence. No production state was accessed; the historical external audit used the pins recorded in §11.10. INOFY's pin is unchanged; current baseline-A Laputa/Studio pins differ and their API findings remain unverified until inspected. No inspection of the current pins is claimed by this supplement.
- Planning docs/commit messages are English. Current user asked for a plan package, so implementation is not started. All Stories have concrete contracts. Architecture choices are recorded; upstream delivery, accepted implementation predecessors and product evidence remain separate prerequisites.

## Owner-confirmed scope update (2026-10-19)

The owner requires **nudge, warming and steering to remain as explicit species-selected experimental packages**. Their former whole-feature-retirement recommendation is superseded. D04/D05 settle the inert loop-exp seam and remove hash/handoff coupling only; the full package extraction is owned by the independent epics E-NUDGE, E-WARMING and E-STEERING. These packages are absent from minimal and every Recipe that does not select them; selected Recipes retain their full settings/UI/events/behavior. Selection is through Recipe/generated Assembly, not a hidden always-compiled switch. All other deletion/simplification cuts, data/security constraints and unresolved technical evidence continue under the existing plan; do not extend experimental retention to other mechanisms. No runtime work has started.

Latest selection decision: [experimental-package verification](../../../logs/2026-10-19-issue-91-experimental-selection/verification.md).

Scope-correction evidence: [retained-feature verification](../../../logs/2026-10-19-issue-91-retained-features/verification.md). Previous planning logs remain historical records of the superseded draft.

## Baseline A and retained-package epics (2026-10-19)

The owner chose `202aac38` (2026-10-07) as the implementation baseline because it is the last stable pi-parity state that passed benchmark runs; the original comparison through `e9609296` counted 168 commits and 30 first-parent merges, a historical inventory rather than current-main distance. Main was observed at `c57dfd322a50daf5ac2365369e84f81f1733a8b2` after PR #84 merged on 2026-10-19. Later work remains capability backlog/donor evidence to re-review and re-introduce in gated batches, not assumed present or discarded. The plan's docs-only commits were replayed onto this baseline on `feat/agent-loop-deletion`; the original `codex/agent-loop-deletion` branch remains history.

Delivery split: #91 remains the base cut (hash deletion, core contraction, seam) and does not claim C11 physical omission on its own. The three retained experimental packages are delivered as independent epics:

| Epic | Scope | Package | Stories |
| --- | --- | --- | --- |
| [E-NUDGE](../epic-nudge/index.md) | Full nudge coaching extraction, parity, settings/UI/events/recovery, evidence and core-body deletion | `plugins/exp/nudge/` | N00–N03 |
| [E-WARMING](../epic-warming/index.md) | Full warming extraction, parity, settings/UI/events/accounting/recovery, evidence and core-body deletion | `plugins/exp/warming/` | W00–W03 |
| [E-STEERING](../epic-steering/index.md) | Full steering extraction, parity, settings/UI/events/queue recovery, evidence and core-body deletion | `plugins/exp/steering/` | S00–S03 |

Baseline evidence: [baseline-A re-anchor audit](../../../research/2026-10-19-baseline-a-reanchor.md) and [epic-split delivery record](../../../logs/2026-10-19-baseline-a-epic-split/summary.md). Re-introduction of post-baseline capabilities is the separate gated track in [#83](https://github.com/ProjectViVy/agent-vivy/issues/83), with [#81](https://github.com/ProjectViVy/agent-vivy/issues/81) retained as capability backlog. Phase A stabilizes the loop first; missing future batch coordination is not its blocker. Old PRs are evidence/donor diffs, never blanket merge instructions.

## Requirements and coverage

| Requirement | Observable outcome | Owning Stories |
| --- | --- | --- |
| R1 | Completely remove Vivy-owned hash implementations, state, protocols, fields, generators, dead code and active requirements; optional final artifact label is not required. | D01–D12 |
| R2 | One production model -> real tool -> exact result/call ID -> next model request -> final terminal/idle path, using existing Eino. | D02–D05, D08, D13–D14 |
| R3 | Preserve approved intent, server permission/scope, durable ownership/outcomes, conflicts and no blind effect replay. | D03–D04, D08–D12; D07 only on later re-intake |
| R4 | Finite supported data transition, fresh/upgrade/reopen/failure rollback and SQLite/Postgres/BML parity; no data reset. | D00, D03, D08–D12, D14; D07 only on later re-intake |
| R5 | Delete selected mechanisms through their complete closure. Preserve nudge/warming/steering as explicit species-selected EXP packages, physically absent when unselected and functional when selected; no wrappers or unconditional wiring. | D01–D12, D14; E-NUDGE, E-WARMING, E-STEERING |
| R6 | Regeneration and supported official Recipe/build variants preserve simplified contracts and physical omission. | D06, D13–D14 base path; EXP epics own C11 omission |
| R7 | Usable minimal, tiny real-token chat Face and complete runnable minimal-plus-feature examples after core contraction. | D13–D14 |
| R8 | Independent behavior and residue reviewers, human real-token E2E and required exact-candidate CI before closure. | D14 |

## Architecture decisions and delivery prerequisites

Hash retirement, the three retained EXP capabilities and all other agreed cuts remain settled. Section 11 now fixes the technical contracts below; none is a repeated product-scope question. Missing delivery/test evidence is explicit and is not supplied by architecture text.

| Gate | Recommendation and exact decision | Blocks |
| --- | --- | --- |
| G1 Product and durability | Settled: three closed T1-only factories, pure SDK policies and selected runtime adapters (§11.1–11.5). Direct producer outcome -> native ToolMessage -> WrapModel; required effect/schedule/final/terminal/accounting commits stay mandatory. Keep current discovery semantics; remove proven duplicate authority only. | D04/D05 accepted implementation predecessors; no missing Host design or renewed scope question |
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

Planned = concrete plan exists, awaiting implementation handoff or accepted prerequisites. Blocked = exact decision/data/dependency missing. Ready requires settled shared decisions, accepted immediate outputs and implementation authorization. Review/Done require observed evidence; no Story is Done in this delivery.

| Story | Epic / requirement | Outcome | Immediate predecessors | Plan | Status | Evidence / blocker |
| --- | --- | --- | --- | --- | --- | --- |
| D00 | E0 / R1–R8 | Freeze scope and finite upgrade contracts | — | [D00](D00.md) | Planned | §11 architecture/source inspection recorded; implementation handoff and future product evidence pending |
| D01 | E1 / R1,R5 | Delete digest audit and unused experimental hash code | D00 | [D01](D01.md) | Planned | Design review and implementation authorization |
| D02 | E1 / R1,R2,R5 | Remove model completion and request hash protocols across Faces | D00 | [D02](D02.md) | Planned | §11.6 completion/historical path fixed; implementation authorization |
| D03 | E2 / R1,R2,R3,R4 | Bind tool operations and approval to exact private inputs | D00 | [D03](D03.md) | Planned | §11.7 legacy binding/pending mapping fixed; accepted D00 and implementation handoff |
| D04 | E3 / R1,R2,R3,R5,R6 | Settle the loop-exp seam and remove nudge FNV/handoff (nudge package is E-NUDGE) | D02, D03, D06 | [D04](D04.md) | Blocked | §11.1–11.4 seam settled; accepted D02/D03/D06 implementation outputs |
| D05 | E3 / R1,R2,R3,R5,R6 | Settle the warming/steering seam and remove hash-only coupling (packages are E-WARMING/E-STEERING) | D04 | [D05](D05.md) | Blocked | §11.3/§11.5 lifecycle settled; accepted D04 implementation output |
| D06 | E4 / R1,R5,R6 | Delete source, Assembly and UI evidence generation end to end | D00 | [D06](D06.md) | Blocked | Accepted D00/implementation handoff; current Studio consumer inspection pending for affected closure only, not independent core generation/seam proof |
| D07 | E4 / R1,R3,R4,R5 | Replace bundle and toolkit content addressing with owned releases | — | [D07](D07.md) | Retained — explanatory | Owner decision 2026-10-19: baseline A has no bundle/toolkit subsystem, so no deletion here; D07 is the re-intake checkpoint (resolve content-addressed identity/consent before a batch from `backup/main-pre-rollback-20261019` merges) |
| D08 | E2 / R1,R2,R3,R4 | Delete Host and admitted-snapshot hashes, bind checkpoints directly | D03 | [D08](D08.md) | Blocked | §11.7 checkpoint/Run transition fixed; accepted D03 output, no D07 baseline dependency |
| D09 | E2 / R1,R3,R4,R5 | Remove file, history, reference and deliverable hash authority | D03 | [D09](D09.md) | Blocked | §11.6 storage/cursor contract fixed; accepted D03 output |
| D10 | E5 / R1,R3,R4,R5 | Preserve skill, memory and cognitive revision CAS without hashes | D08, D12 | [D10](D10.md) | Blocked | Accepted D08/D12; inspect actual baseline-A Laputa APIs, then require upstream delivery only for a confirmed incompatible seam |
| D11 | E5 / R1,R3,R4,R5 | Remove baseline Work, workflow and notes business digests | D03, D08 | [D11](D11.md) | Blocked | Coordinated hash-free INOFY version/pin plus accepted D03/D08 |
| D12 | E2 / R1,R3,R4,R5 | Remove migration checksums and convert the independent BML format | D00 | [D12](D12.md) | Planned | §11.7–11.9 format/metadata/BML design fixed; accepted D00 and implementation handoff |
| D13 | E6 / R2,R6,R7 | Ship usable minimal, a tiny real-token Face and runnable examples | D05, D06, D08 | [D13](D13.md) | Blocked | Core contraction accepted; repository gate changes require explicit approval |
| D14 | E7 / R1–R8 | Accept one final candidate through independent behavior and residue audits | D01, D09, D10, D11, D13 | [D14](D14.md) | Blocked | All predecessors accepted; real-token environment; fresh independent auditors |

## Immediate dependency contracts

D00 supplies C1–C11/§11/G1–G5, not code; required upstream version/pin deliveries are external prerequisites in §11.10. D02 supplies the aligned live/replay completion contract. D03 supplies exact invocation/approval ownership. D04 supplies the inert loop-exp seam, the bounded exact-tuple nudge window and removal of the consumer-owned handoff; D05 supplies the warming/steering seam and hash-only coupling removal. Both record the interim core-owned behavior; package extraction and C11 selection behavior are supplied by the three epics, not by #91. Other scope follows the existing plan. D06 supplies simplified source/generation producers. D07 records later immutable release/consent obligations and supplies no baseline output. D08 supplies admitted values/checkpoint binding. D12 supplies BML v2 and final metadata ownership. D09/D10/D11 supply accepted feature behavior and migration evidence. D13 supplies runnable minimal/Face/examples. D14 consumes all integrated outcomes, not author completion claims.

Edges are prerequisite inputs, not a priority ranking. Wave two starts after the common D00 scope freeze. D12 is an independent format/metadata cut after that freeze; feature-specific SQL is owned by its individual Story, not a late central column-drop task.

D06 -> D04 supplies simplified source/Assembly and the inert factory seam. Physical optional-package selection is delivered and accepted by the EXP epics.

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
- Baseline D03/D08–D12 own their applicable feature migrations; D07 reserves no baseline SQL or converter fragment; allocate paired numeric IDs serially above the current maximum (035 at planning; the 10-18 tree reached 043, not present at baseline A). D12 alone owns migration runner/metadata. Do not rewrite or renumber a released migration.
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
| Later EXP integration and physical omission | Each selected package works, all three compose, and no unselected implementation/constructor/state read/registration/call/control ships. Preserve full parity, owner Recipe decisions and core behavior without packages. Not accepted until those packages exist. | E-NUDGE/E-WARMING/E-STEERING, including their integrated independent acceptance; not an extra #91 predecessor |
| Content/privacy | Same-length stale edits conflict, authorized selection exact, foreign cursor cannot gain scope, immutable chunks, no private arguments in public events | D03, D09 |
| Supported data | Approved source matrix -> target on disposable SQLite/Postgres/BML, reopen/failure rollback; unknown effects/old pauses explicit; no old runtime reader | D03, D08–D12; D07 only on later re-intake |
| Generation/residue | Source-first removal, double regeneration idempotent with same ID, official Recipe/artifact inspection, exact remaining-match dispositions | D06, D14 |
| Retained baseline capabilities | Real reviewed skill/memory and Work/workflow/notes behavior under simplified contracts; actual upstream seams retain their gates | D10–D11 |
| Later capability re-intake | Bundle/toolkit release consent/rollback, report/new-notebook and other post-baseline capabilities get complete reviewed authority/data/acceptance closure in their batch | #83 coordination, #81 backlog; D07 where applicable |
| Usable baseline minimal | Runnable minimal + feature examples, actual existing Module omission contract, production tiny Face with human real tokens; EXP physical omission remains with epics | D13–D14 |
| Final closure | Two fresh non-author subagents accept behavior and residue at exact final candidate; just ci + real Postgres + BML + affected real-path checks | D14 |

## Detailed architecture update

The [shared design §11](../../../plans/2026-10-19-complete-deletion-plan.md#11-detailed-target-architecture) records module/import direction, exact factories/Hosts, normal/retry/cancel/resume lifecycle, target schemas, finite source matrix, copied-target cutover and historical/current pin provenance with affected consumer checks. [Architecture verification](../../../logs/2026-10-19-issue-91-detailed-architecture/verification.md) records this doc-only update. No upstream source/pin, runtime, schema or generated product file changed.

## Planning verification and change handling

Planning checks: exact baseline comparison, local referenced-path validation, relative-link validation, requirement/Story coverage, unique IDs/known DAG endpoints/no self-edge/no cycle/no redundant transitive edge, derived waves and `git diff --check`. Results live in [the planning verification record](../../../logs/2026-10-19-issue-91-planning/verification.md) and the [baseline-A epic-split record](../../../logs/2026-10-19-baseline-a-epic-split/verification.md).

This supplement's focused documentation verification is recorded in [the stable-loop handoff record](../../../logs/2026-10-19-stable-loop-handoff/verification.md). Earlier dated logs are historical evidence and do not override corrected phase applicability.

Product checks were not run: no implementation changed. No `just ci`, generation, Postgres, live-provider, installed-data or post-deletion-audit result is inferred from document checks. Future execution must prepare the repository-supported environment and record missing prerequisites honestly.

A shared-contract change invalidates downstream readiness and affected evidence. Revise this design/package, not a parallel report. Retained and retired scopes, support versions and public acceptance changes return to owner review. Local implementation details within the accepted contract can be corrected autonomously. The final implementation commit must identify exactly what was verified; a planning commit does not close #91.

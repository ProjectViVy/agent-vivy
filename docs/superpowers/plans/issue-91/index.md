# Issue #91: Complete deletion planning package

Issue: [#91](https://github.com/ProjectViVy/agent-vivy/issues/91) · Branch: `codex/agent-loop-deletion`.

**Current outcome:** design amendment, 15 Story plans (D00–D14), immediate dependency DAG, migration gates and independent acceptance plan. This is a planning deliverable; no runtime deletion, migration, regeneration, product CI, post-deletion audit or incident-root-cause proof is claimed.

## Authority and baseline

- Shared design: [complete deletion plan](../../../plans/2026-10-19-complete-deletion-plan.md), especially §10 C1–C10. Reuse this authority; do not create a competing spec.
- Source evidence: [core deletion insight](../../../research/2026-10-19-agent-loop-core-deletion-insight.md) and [slimming review](../../../research/2026-10-19-agent-loop-slimming-review.md). These are source findings and proposals, not product acceptance.
- Planning branch read at `9c5da97e3292591fa960b8cd14d921c17fea5812`; fresh main read at `e96092966cfbfa443648374948e7e7e16dd01683`. Product-code delta between original audit and fetched main: none. Planning commit records its own revision in Git, not a self-referential hash field.
- Root/UI AGENTS and relevant plugin/kernel/Eino skills were read. Two independent read-only domain reviews checked runtime/approval and assembly/persistence. No production state was accessed; external Studio/Laputa/INOFY contract closure is not yet proven.
- Planning docs/commit messages are English. Current user asked for a plan package, so implementation is not started. D01/D02/D06 have concrete draft cuts but stay Planned until review/authorization. Other Stories identify explicit blockers rather than pretending readiness.

## Owner-confirmed scope update (2026-10-19)

The owner requires **nudge, warming and steering to remain as explicit species-selected experimental packages**. Their former whole-feature-retirement recommendation is superseded. D04 extracts coaching; D05 extracts actual warming and same-Run steering. These packages are absent from minimal and every Recipe that does not select them; selected Recipes retain their full settings/UI/events/behavior. Selection is through Recipe/generated Assembly, not a hidden always-compiled switch. All other deletion/simplification cuts, data/security constraints and unresolved technical evidence continue under the existing plan; do not extend experimental retention to other mechanisms. No runtime work has started.

Latest selection decision: [experimental-package verification](../../../logs/2026-10-19-issue-91-experimental-selection/verification.md).

Scope-correction evidence: [retained-feature verification](../../../logs/2026-10-19-issue-91-retained-features/verification.md). Previous planning logs remain historical records of the superseded draft.

## Requirements and coverage

| Requirement | Observable outcome | Owning Stories |
| --- | --- | --- |
| R1 | Completely remove Vivy-owned hash implementations, state, protocols, fields, generators, dead code and active requirements; optional final artifact label is not required. | D01–D12 |
| R2 | One production model -> real tool -> exact result/call ID -> next model request -> final terminal/idle path, using existing Eino. | D02–D05, D08, D13–D14 |
| R3 | Preserve approved intent, server permission/scope, durable ownership/outcomes, conflicts and no blind effect replay. | D03–D04, D07–D12 |
| R4 | Finite supported data transition, fresh/upgrade/reopen/failure rollback and SQLite/Postgres/BML parity; no data reset. | D00, D03, D07–D12, D14 |
| R5 | Delete selected mechanisms through their complete closure. Preserve nudge/warming/steering as explicit species-selected EXP packages, physically absent when unselected and functional when selected; no wrappers or unconditional wiring. | D01–D12, D14 |
| R6 | Regeneration and supported official Recipe/build variants preserve simplified contracts and physical omission. | D06, D13–D14 |
| R7 | Usable minimal, tiny real-token chat Face and complete runnable minimal-plus-feature examples after core contraction. | D13–D14 |
| R8 | Independent behavior and residue reviewers, human real-token E2E and required exact-candidate CI before closure. | D14 |

## Review decisions before implementation

Hash retirement and retention of nudge/warming/steering are confirmed. Remaining recommendations are proposals for review. D00 records only unresolved decisions; it must not ask again whether those three capabilities should remain.

| Gate | Recommendation and exact decision | Blocks |
| --- | --- | --- |
| G1 Product and durability | Confirmed: nudge, warming and same-Run steering are retained only through explicitly selected experimental Modules/Recipes; remove unconditional core wiring. All other cuts and their existing gates follow the prior plan. C11 requires a typed selectable host seam: current observer/pre-tool Ports are insufficient; D00 must settle that concrete technical contract before D04/D05 become Ready. | D04–D05: typed seam, accepted predecessors and existing data/durability gates; no renewed feature-retention question |
| G2 Supported data and finite upgrade | Separately delivered old-release normalization -> exact baseline SQL 043/BML v1 -> final hash-free target, with backup and helper removed from final runtime. Preserve all currently supported legacy shapes through the finite artifact. Expire unbindable approvals; old paused checkpoint recovery is explicit, not seamless; retain unknown/completed effect fences. Approve source matrix, release boundary and immutable historical SQL disposition. | All durable cuts |
| G3 Release consent | Use opaque acquired artifact plus existing install/release sequence and catalog revisions. Exact declaration/config consent; new release/config requires reapproval unless explicitly preserving a proven reusable consent. Review revocation semantics. | D07 and admitted bundle consumers |
| G4 Exact content and cursor trust | Use bounded actual before/selection/presentation bytes in existing owners, one immutable transfer snapshot; stateless untrusted cursor with server reauthorization and allowed cut validation. Accept loss of signed cursor-issuance authenticity; preserve scope, selection and honest snapshot behavior. Approve storage/cleanup semantics. | D09 |
| G5 External/native boundary | Keep Go/pnpm/TLS/authentication and native bundler internals; remove every Vivy-owned evidence check and explicit INOFY digest authority. Omit optional final label by default. Verify actual pinned external APIs and Studio consumers; coordinated change if required. | D06, D10–D12, global residue closure |

No rewrite of AGENTS, current product requirements or mandatory gates is performed by this planning delivery. Hash-related contradictory clauses are changed with the authorized implementation cut. Workflow/gate reductions need their own explicit authorization; they cannot be inferred from a request for speed.

## Epics

| Epic | Coherent outcome | Stories |
| --- | --- | --- |
| E0 | Scope, contracts and finite support boundary settled | D00 |
| E1 | Remove audit/completion/request evidence authority | D01–D02 |
| E2 | Preserve exact ownership and supported data without hashes | D03, D08–D09, D12 |
| E3 | Simplify core continuation and extract three species-selected experimental capabilities | D04–D05 |
| E4 | Hash-free explicit source/Assembly/release composition | D06–D07 |
| E5 | Preserve selected feature behavior with owner revisions | D10–D11 |
| E6 | Usable minimal and complete real-path examples | D13 |
| E7 | Independently accept the integrated candidate | D14 |

## Story index (sole status/dependency record)

Planned = concrete plan exists, awaiting review/authorization or accepted prerequisites. Blocked = exact decision/data/dependency missing. Ready requires settled shared decisions, accepted immediate outputs and implementation authorization. Review/Done require observed evidence; no Story is Done in this delivery.

| Story | Epic / requirement | Outcome | Immediate predecessors | Plan | Status | Evidence / blocker |
| --- | --- | --- | --- | --- | --- | --- |
| D00 | E0 / R1–R8 | Freeze scope and finite upgrade contracts | — | [D00](D00.md) | Blocked | G1–G5 owner review; external consumer/source-format facts |
| D01 | E1 / R1,R5 | Delete digest audit and unused experimental hash code | D00 | [D01](D01.md) | Planned | Design review and implementation authorization |
| D02 | E1 / R1,R2,R5 | Remove model completion and request hash protocols across Faces | D00 | [D02](D02.md) | Planned | C1 review and implementation authorization |
| D03 | E2 / R1,R2,R3,R4 | Bind tool operations and approval to exact private inputs | D00 | [D03](D03.md) | Blocked | G2 supported pending-state policy; C2/C3 review |
| D04 | E3 / R1,R2,R3,R5,R6 | Extract species-selected experimental nudge and simplify continuation | D02, D03, D06 | [D04](D04.md) | Blocked | EXP scope confirmed; typed selectable seam and accepted D02/D03/D06; existing durability gates |
| D05 | E3 / R1,R2,R3,R5,R6 | Extract species-selected experimental warming and steering | D04 | [D05](D05.md) | Blocked | EXP scope confirmed; typed selectable seam, D04 and existing G2 queued-state contracts |
| D06 | E4 / R1,R5,R6 | Delete source, Assembly and UI evidence generation end to end | D00 | [D06](D06.md) | Planned | C4 and G5 approval; external source selection preserved |
| D07 | E4 / R1,R3,R4,R5 | Replace bundle and toolkit content addressing with owned releases | D03, D06 | [D07](D07.md) | Blocked | G2 artifact upgrade boundary; G3 consent/revocation policy |
| D08 | E2 / R1,R2,R3,R4 | Delete Host and admitted-snapshot hashes, bind checkpoints directly | D03, D07 | [D08](D08.md) | Blocked | G2 checkpoint policy and admitted-format transition |
| D09 | E2 / R1,R3,R4,R5 | Remove file, history, reference and deliverable hash authority | D03 | [D09](D09.md) | Blocked | G2 historical receipts; G4 cursor/snapshot contract |
| D10 | E5 / R1,R3,R4,R5 | Preserve skill, memory and cognitive revision CAS without hashes | D08, D12 | [D10](D10.md) | Blocked | G2/G5 memory dependency support; C9/C10 review |
| D11 | E5 / R1,R3,R4,R5 | Remove Work, workflow, notebook and report business digests | D03, D08 | [D11](D11.md) | Blocked | G2 paused work formats; G5 pinned INOFY API disposition |
| D12 | E2 / R1,R3,R4,R5 | Remove migration checksums and convert the independent BML format | D00 | [D12](D12.md) | Blocked | G2 finite converter/source matrix; C10 and G5 review |
| D13 | E6 / R2,R6,R7 | Ship usable minimal, a tiny real-token Face and runnable examples | D05, D06, D08 | [D13](D13.md) | Blocked | Core contraction accepted; repository gate changes require explicit approval |
| D14 | E7 / R1–R8 | Accept one final candidate through independent behavior and residue audits | D01, D09, D10, D11, D13 | [D14](D14.md) | Blocked | All predecessors accepted; real-token environment; fresh independent auditors |

## Immediate dependency contracts

D00 supplies accepted C1–C10/G1–G5, not code. D02 supplies the aligned live/replay completion contract. D03 supplies exact invocation/approval ownership. D04 supplies direct durable outcome-to-Eino continuation and explicit EXP nudge selection. D05 supplies optional EXP warming/steering; both follow C11 selected/unselected behavior. Other scope follows the existing plan. D06 supplies simplified source/generation producers. D07 supplies immutable release/consent references. D08 supplies admitted values/checkpoint binding. D12 supplies BML v2 and final metadata ownership. D09/D10/D11 supply accepted feature behavior and migration evidence. D13 supplies runnable minimal/Face/examples. D14 consumes all integrated outcomes, not author completion claims.

Edges are prerequisite inputs, not a priority ranking. Wave two starts after the common D00 scope freeze. D12 is an independent format/metadata cut after that freeze; feature-specific SQL is owned by its individual Story, not a late central column-drop task.

D06 -> D04 supplies simplified source/Assembly generation for physical optional-package selection.

Three transitive-looking edges carry separate real inputs and are intentional: D03 -> D08 supplies private invocation/approval bindings independently of D07's release references; D03 -> D11 supplies claim/replay and intended-request semantics independently of D08's admitted snapshot; D06 -> D13 supplies pack/generator APIs independently of D08's runtime admission contract. All other redundant transitive edges are omitted.

## Topological waves

1. D00
2. D01, D02, D03, D06, D12
3. D04, D07, D09
4. D05, D08
5. D10, D11, D13
6. D14

Waves describe dependencies, not permission for simultaneous edits. No elapsed-time or speedup estimates are claimed. Default execution is one write lane on the specified branch; fresh read-only reviewers can be concurrent where useful. The package does not mandate one implementation agent per Story.

## Shared-file scheduling

- D02/D03/D04/D05/D08/D09/D11 touch `service.go`, mapper/payloads/domain DTOs or RPC/control: serialize or split shared edits through one integrating owner; do not add fictional logical edges solely for file conflicts.
- D03/D07–D12 own their feature migrations; allocate paired numeric IDs serially above the current maximum (043 at planning). D12 alone owns migration runner/metadata. Do not rewrite or renumber a released migration.
- D06 owns descriptor/manifest/generator contract and current hash-related instruction edits. D07–D13 consume it; later feature fields may require another generated diff, always through the same producers.
- D07 owns bundle release/consent and masks' release reference. D10 owns skill/memory/mask content revision behavior. D08 owns admitted snapshots/checkpoint binding; do not create duplicate private snapshot services.
- D09 owns reference/download DTO and `HistoryPicker`; D02 owns live completion state in the shared UI store. Source Module UI is edited before staging, never generated copies.
- A second concurrent implementation lane requires repository worktree isolation and its own pinned Laputa checkout. One active write lane per checkout; no shared mutable sibling dependency.

## Acceptance matrix

| Risk / outcome | Required evidence | Owner |
| --- | --- | --- |
| Causal tool continuation | Real isolated nonce read; captured second model request contains exact result + call ID; final text, one terminal, idle | D04, D13, independent D14 |
| Intent and crash safety | Changed original input conflicts; canonical approval binding; sibling resume; admitted/claimed/effect/completed crash matrix; no blind replay | D03, D08–D11 |
| Selected EXP enhancements and omission | Each selected package works, all three compose, and none is constructed/read/registered/called/shown when unselected. Nudge coaching, accounted warming and same-Run steering remain real; core cancellation/durability/budgets work without them. | D04–D05, D13–D14 |
| Content/privacy | Same-length stale edits conflict, authorized selection exact, foreign cursor cannot gain scope, immutable chunks, no private arguments in public events | D03, D09 |
| Supported data | Approved source matrix -> target on disposable SQLite/Postgres/BML, reopen/failure rollback; unknown effects/old pauses explicit; no old runtime reader | D03, D07–D12 |
| Generation/residue | Source-first removal, double regeneration idempotent with same ID, official Recipe/artifact inspection, exact remaining-match dispositions | D06, D14 |
| Retained optional features | Real reviewed skill/memory, release consent/rollback and Work/workflow/notebook/report behavior under simplified contracts | D07, D10–D11 |
| Usable minimal | Complete runnable minimal + feature examples, physical omission, production tiny Face with human real tokens | D13–D14 |
| Final closure | Two fresh non-author subagents accept behavior and residue at exact final candidate; just ci + real Postgres + BML + affected real-path checks | D14 |

## Planning verification and change handling

Planning checks: exact baseline comparison, local referenced-path validation, relative-link validation, requirement/Story coverage, unique IDs/known DAG endpoints/no self-edge/no cycle/no redundant transitive edge, derived waves and `git diff --check`. Results live in [the planning verification record](../../../logs/2026-10-19-issue-91-planning/verification.md).

Product checks were not run: no implementation changed. No `just ci`, generation, Postgres, live-provider, installed-data or post-deletion-audit result is inferred from document checks. This environment has no located Go/just executable or installed dependencies; future execution must prepare the repository-supported environment and record missing prerequisites honestly.

A shared-contract change invalidates downstream readiness and affected evidence. Revise this design/package, not a parallel report. Retained and retired scopes, support versions and public acceptance changes return to owner review. Local implementation details within the accepted contract can be corrected autonomously. The final implementation commit must identify exactly what was verified; a planning commit does not close #91.

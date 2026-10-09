# Issue #32 remediation implementation package

Status: **DESIGNED; product implementation not started.** Owner request:
2026-10-09, work-package design with a detailed document for every phase.
Design authority: [written design](../../specs/2026-10-09-issue32-remediation-design.md).
Issue authority: [VIVY #32](https://github.com/ProjectViVy/agent-vivy/issues/32).
This index is the sole phase/finding status board; plans own steps and interfaces.

## Read order

1. Repository AGENTS.md and applicable scoped instructions in both repositories.
2. The written design, especially source identities, global constraints and
   compatibility/failure contracts.
3. The selected phase plan and its prerequisite task Interfaces blocks.
4. Existing authoritative iteration/native acceptance records for evidence.

VIVY reviewed main: `017ec8cc37970b291e04c619990aed00d5403116`.
DIVA reviewed main: `518a33ef09858ee1bb190579dd7529aceaa15dd6`.
Recheck both before execution. File paths in plans are repository-relative;
explicit `DIVA:` paths belong to agent-diva, all others to agent-vivy.

## Phases and task ownership

| Phase | Independently reviewable tasks | Prerequisite | Plan | State |
| --- | --- | --- | --- | --- |
| P0 | baseline, coverage, evidence/lane setup | owner documentation request | [P0](P0-baseline.md) | DESIGNED |
| P1 | frozen frontend input; H1 window auth; H2 active-host CI; H3 strict candidate gate | P0 | [P1](P1-desktop-gates.md) | DESIGNED |
| P2 | C1 state CAS; C2 intent/recovery; C3/C4 pins; C5/C6 control | P0; P2.1 before remaining tasks | [P2](P2-cognitive.md) | DESIGNED |
| P3 | W3/W7 draft CAS; W1/W2 publication/start; W4/W8 cursors; W5/W6 UI/status | P0; internal task dependencies | [P3](P3-workflow.md) | DESIGNED |
| P4 | H4 logger; C7 cleanup; H5/H7 lifecycle; H6 listener; C8 capability UI | P0; coordinate P2 App changes | [P4](P4-host-lifecycle.md) | DESIGNED |
| P5 | R1/R3 bounded continuation; R4 mandatory settlement | P0; coordinate P2 Service changes | [P5](P5-diagnostics-settlement.md) | DESIGNED |
| P6 | W9 proof retirement and legacy fail-closed path | P0; coordinate Service approval changes | [P6](P6-legacy-proof.md) | DESIGNED |
| P7 | integration; SDK/consumer pins; final candidate; conditional cutover/closure | P1-P6 engineering gates | [P7](P7-final-integration.md) | DESIGNED |

First blocking tranche: P1.0-P1.3 and P2.1-P2.2 (five P1 findings plus the frontend-input prerequisite). Remaining tasks may be prepared
in isolated lanes, but do not delay those five P1 repairs with unrelated work.
Implementation method is unselected. Recommended: task-level implementation
and independent review; state/Service/App changes stay sequential in each lane.
This recommendation is not an instruction to spawn concurrent product editors.

## Finding coverage

| Finding | Severity | Owning task | Current disposition |
| --- | --- | --- | --- |
| H1 | P1 | P1.1 | PLANNED |
| H2 | P1 | P1.2 | PLANNED |
| H3 | P1 | P1.3 | PLANNED |
| C1 | P1 | P2.1 | PLANNED; defect probe reproduced |
| C2 | P1 | P2.2 | PLANNED; call-chain confirmed |
| H4 | P2 | P4.1a | PLANNED |
| H5 | P2 | P4.2 | PLANNED |
| H6 | P2 | P4.3 | PLANNED |
| H7 | P2 | P4.2 | PLANNED |
| C3 | P2 | P2.3 | PLANNED |
| C4 | P2 | P2.3 | PLANNED |
| C5 | P2 | P2.4 | PLANNED |
| C6 | P2 | P2.4 | PLANNED |
| C7 | P2 | P4.1b | PLANNED |
| C8 | P2 | P4.4 | PLANNED |
| W1 | P2 | P3.2 | PLANNED |
| W2 | P2 | P3.2 | PLANNED |
| W3 | P2 | P3.1 | PLANNED |
| W4 | P2 | P3.3 | PLANNED |
| W5 | P2 | P3.4 | PLANNED |
| W6 | P2 | P3.4 | PLANNED |
| W7 | P2 | P3.1 | PLANNED; narrowed to remaining same-author collision |
| W8 | P3 | P3.3 | PLANNED |
| W9 | P3 | P6.1 | PLANNED |
| R1 | P2 | P5.1 | PLANNED; defect probe reproduced |
| R2 | P2 | P0.2 | SUPERSEDED by owner #40 decision and merged PR #42 |
| R3 | P2 | P5.1 | PLANNED; defect probe reproduced |
| R4 | P2 | P5.2 | PLANNED; defect probe reproduced |

Totals: 28 original findings, 27 planned repairs, 1 explicit supersession.
No finding is marked fixed by this design package. Migration milestone W5/W6
and finding IDs W5/W6 are different scopes; do not use one as the other's proof.

## Evidence and closure

When a task is implemented, add its branch/commit, red regression observation,
green command outcome, real-path observation and remaining unexercised scenarios
to the phase's iteration record. Then advance its index row from PLANNED to
IMPLEMENTED and ENGINEERING-VERIFIED as evidence permits. Product acceptance
and RELEASE-ACCEPTED remain separate transitions.

Use `docs/logs/YYYY-MM-DD-issue32-pN/` in the owning repository, with
summary.md, verification.md and acceptance.md. A review records actual results,
not just commands copied from a plan. Record skipped PostgreSQL/Windows/audio
as pending. Final evidence additionally identifies final full commits, trees,
locks, Generation, frontend, toolchain, platform and artifact checksums.

This documentation delivery is recorded in
[summary](../../../logs/2026-10-09-issue32-design/summary.md),
[verification](../../../logs/2026-10-09-issue32-design/verification.md) and
[acceptance](../../../logs/2026-10-09-issue32-design/acceptance.md).

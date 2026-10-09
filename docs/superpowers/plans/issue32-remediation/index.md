# Issue #32 remediation implementation package

Status: **P1 native acceptance and P7 native/product gates remain open; P2 implementation and P3–P6 engineering tasks are complete.** P0 baseline and traceability are verified. P7.0 producer integration is committed; DIVA source repin, consumer and platform acceptance continue under P7.1–P7.3.
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
| P0 | baseline, coverage, evidence/lane setup | owner documentation request | [P0](P0-baseline.md) | BASELINE VERIFIED |
| P1 | H1 window auth; H2 active-host CI; H3 strict candidate gate | P0 | [P1](P1-desktop-gates.md) | IN PROGRESS |
| P2 | C1 state CAS; C2 intent/recovery; C3/C4 pins; C5/C6 control | P0; P2.1 before remaining tasks | [P2](P2-cognitive.md) | IMPLEMENTATION COMPLETE; aggregate `just ci` and SDK/Port conformance pending P7 |
| P3 | W3/W7 draft CAS; W1/W2 publication/start; W4/W8 cursors; W5/W6 UI/status | P0; internal task dependencies | [P3](P3-workflow.md) | ENGINEERING-VERIFIED LOCALLY; P3.1-P3.4 implementation and focused/full suite checks complete under the recorded unavailable-skill ruling; real-browser candidate acceptance remains P7 |
| P4 | H4 logger; C7 cleanup; H5/H7 lifecycle; H6 listener; C8 capability UI | P0; coordinate P2 App changes | [P4](P4-host-lifecycle.md) | IMPLEMENTATION ENGINEERING-VERIFIED LOCALLY; P4.2 native lifecycle and P4.4 candidate observations remain P7 gates |
| P5 | R1/R3 bounded continuation; R4 mandatory settlement | P0; coordinate P2 Service changes | [P5](P5-diagnostics-settlement.md) | IMPLEMENTED AND ENGINEERING-VERIFIED LOCALLY; aggregate `just ci`, SDK conformance, and native Windows execution pending P7 |
| P6 | W9 proof retirement and legacy fail-closed path | P0; coordinate Service approval changes | [P6](P6-legacy-proof.md) | ENGINEERING VERIFIED; `just ci` pending P7 |
| P7 | integration; SDK/consumer pins; final candidate; conditional cutover/closure | P1-P6 engineering gates | [P7](P7-final-integration.md) | VIVY P7.0 producer integration committed; DIVA final source pin and consumer engineering gate in progress; native/product owner gates remain separate |

Current execution lane: P3.1-P3.4, P4.1a/b, P4.2, P4.3 and P4.4, P5.1/P5.2,
and P6.1 implementation work is engineering-verified locally. P7.0 producer
integration, conformance, split-UI start/inspect smoke, and Linux-executable
aggregate recipe components pass. DIVA repin/consumer verification and native
candidate/owner gates remain open. P3.4
followed the documented `oil-frontend` availability ruling; its interaction/
data contracts are covered by UI tests. P1 native acceptance remains independent.
The reviewed DIVA frontend lock is already tracked and matches its canonical
hash; P1.2 verifies frozen use without a separate lock-restoration task.
Implementation method is unselected. Recommended: task-level implementation
and independent review; state/Service/App changes stay sequential in each lane.
This recommendation is not an instruction to spawn concurrent product editors.

## Finding coverage

| Finding | Severity | Owning task | Current disposition |
| --- | --- | --- | --- |
| H1 | P1 | P1.1 | IMPLEMENTED; local headless tests pass; canonical Linux host race pending |
| H2 | P1 | P1.2 | IMPLEMENTED in DIVA `8918b6f2`, with source-tree pin/aggregate-test follow-up in `607fc54f`; 18 local contracts pass; Linux/Windows native CI pending (DIVA logs: `v0.0.2-h2-ci-and-lock-gates`, `v0.0.3-h3-candidate-publication-gates`) |
| H3 | P1 | P1.3 | IMPLEMENTED in DIVA `607fc54f`; strict v2 checker passes a synthetic two-platform/17-row candidate and rejects incomplete/tampered cases; native builds, real W5 acceptance, archive tags, and approved W6 state pending |
| C1 | P1 | P2.1 | IMPLEMENTED; focused regressions and cognitive race suite pass; aggregate `just ci` unavailable in this environment ([P2.1 verification](../../../logs/2026-10-09-issue32-p2.1-state-cas/verification.md)) |
| C2 | P1 | P2.2 | IMPLEMENTED; fault matrix, cognitive race tests, full runtime package and SQLite version conformance pass; PostgreSQL DSN and aggregate `just ci` pending |
| H4 | P2 | P4.1a | IMPLEMENTED AND ENGINEERING-VERIFIED locally; aggregate `just ci` and native Windows profile acceptance pending P7 ([P4.1a verification](../../../logs/2026-10-09-issue32-p4.1a-logger-ownership/verification.md)) |
| H5 | P2 | P4.2 | IMPLEMENTED AND ENGINEERING-VERIFIED LOCALLY; Linux/Windows process acceptance pending P7 |
| H6 | P2 | P4.3 | IMPLEMENTED AND ENGINEERING-VERIFIED LOCALLY; native shell observation pending P7 |
| H7 | P2 | P4.2 | IMPLEMENTED AND ENGINEERING-VERIFIED LOCALLY; Linux/Windows process acceptance pending P7 |
| C3 | P2 | P2.3 | IMPLEMENTED; current Mission pin checked at admission, collection and atomic effect application ([P2.3 verification](../../../logs/2026-10-09-issue32-p2.3-current-policy-pin/verification.md)) |
| C4 | P2 | P2.3 | IMPLEMENTED; resolver hashes the durable admission policy snapshot; commit `35e559f6` |
| C5 | P2 | P2.4 | IMPLEMENTED; coherent control fields derive from one snapshot ([P2.4 verification](../../../logs/2026-10-09-issue32-p2.4-control-cancel/verification.md)) |
| C6 | P2 | P2.4 | IMPLEMENTED; current strategy cancellation rechecks under the terminal commit gate and rejects stale/foreign IDs ([P2.4 verification](../../../logs/2026-10-09-issue32-p2.4-control-cancel/verification.md)) |
| C7 | P2 | P4.1b | IMPLEMENTED AND ENGINEERING-VERIFIED locally; aggregate `just ci` pending P7 ([P4.1b verification](../../../logs/2026-10-09-issue32-p4.1b-cognitive-cleanup/verification.md)) |
| C8 | P2 | P4.4 | IMPLEMENTED AND ENGINEERING-VERIFIED LOCALLY; real-candidate provider observation pending P7 |
| W1 | P2 | P3.2 | ENGINEERING-VERIFIED: duplicate-tool admission/publication/start guards and prepared UI session/source requests; candidate browser acceptance pending P7 |
| W2 | P2 | P3.2 | ENGINEERING-VERIFIED: immutable start requests, immediate save confirmation, and retained retries in editor/published rows; candidate browser acceptance pending P7 |
| W3 | P2 | P3.1 | ENGINEERING-VERIFIED: backend CAS and Module explicit create/edit/current-token revision editing; candidate browser acceptance pending P7 |
| W4 | P2 | P3.3 | IMPLEMENTED AND ENGINEERING-VERIFIED in SQLite/PostgreSQL |
| W5 | P2 | P3.4 | ENGINEERING-VERIFIED LOCALLY: cursor pagination, refresh/session stale-response fencing, continuation retry and row retention; real-browser candidate acceptance remains P7 |
| W6 | P2 | P3.4 | ENGINEERING-VERIFIED LOCALLY: native/engine status separation, completed cancellation guard and recovery-event detail refresh; real-browser candidate acceptance remains P7 |
| W7 | P2 | P3.1 | ENGINEERING-VERIFIED: same-author create collision and opaque ETags covered through backend and Module source tests; candidate browser acceptance pending P7 |
| W8 | P3 | P3.3 | IMPLEMENTED AND ENGINEERING-VERIFIED; malformed cursors map to refreshable invalid input |
| W9 | P3 | P6.1 | IMPLEMENTED AND ENGINEERING-VERIFIED; aggregate `just ci` remains pending P7 |
| R1 | P2 | P5.1 | IMPLEMENTED AND ENGINEERING-VERIFIED locally; aggregate `just ci` and native Windows run pending |
| R2 | P2 | P0.2 | SUPERSEDED by owner #40 decision and merged PR #42 |
| R3 | P2 | P5.1 | IMPLEMENTED AND ENGINEERING-VERIFIED locally; aggregate `just ci` and native Windows run pending |
| R4 | P2 | P5.2 | IMPLEMENTED AND ENGINEERING-VERIFIED locally; aggregate `just ci` and SDK/consumer conformance pending P7 ([P5.2 verification](../../../logs/2026-10-09-issue32-p5.2-model-settlement/verification.md)) |

Totals: 28 original findings, 27 planned repairs, 1 explicit supersession.
H1 implementation is not yet ENGINEERING-VERIFIED because the canonical Linux
host race run is pending. Migration milestone W5/W6 and finding IDs W5/W6 are
different scopes; do not use one as the other's proof.

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

# INOFY Cutover and Reusable Workflow — Work Package Index

Date: 2026-09-29. State: **Implementation in progress; no core cutover acceptance**. Overall decision and constraints are in [issue #23](https://github.com/ProjectViVy/agent-vivy/issues/23) and the [detailed architecture](../../specs/2026-09-29-inofy-cutover-design.md). This index owns the cross-repository S11 package status; [INOFY S11](https://github.com/ProjectViVy/INOFY/blob/docs/s11-vivy-cutover-plan/docs/superpowers/plans/inofy/S11.md) points here and owns its schema API task.

## Outcome and dependency DAG

Full S11 in this delivery cycle includes the core task graph cutover **and** reusable definitions with a VIVY-hosted editor. The first production switch can be accepted at S11-E; full S11 requires S11-G. Garden S12 has a separate owner and gate. Direct child delegation and existing sessions remain VIVY native. No old DAG conversion, compatibility mode, optional engine, INOFY App database/service or standalone VIVY Studio shell.

| Story | Repository / outcome | Immediate predecessor | State | Gate |
| --- | --- | --- | --- | --- |
| [S11-A](01-inofy-schema.md) | INOFY: export canonical Definition schema | S01–S10 library/editor baseline | Published at `6acfcc6b1a51921c45316a15eb0846bc0e775a9e`; VIVY pinned | G1 |
| [S11-B](02-admission.md) | VIVY: tool/RPC Definition admission and authority | A | In progress; execution guarded until C/D | G1 |
| [S11-C](03-storage.md) | VIVY: atomic Core Storage RunStore, both drivers | B | Planned | G4, G5, G6 |
| [S11-D](04-executor.md) | VIVY: native child NodeExecutor and recovery | C | Planned | G2, G5, G6 |
| [S11-E](05-cutover.md) | VIVY: sole production INOFY path, remove old DAG | D | Planned | G1–G8 core |
| [S11-F](06-definitions.md) | VIVY: reusable definitions and host actions | E | Planned | G9 |
| [S11-G](07-ui.md) | VIVY: selected UI Module with INOFY editor | F | Planned | G7, G10; full S11 |

Topological waves: `{A} → {B} → {C} → {D} → {E} → {F} → {G}`. The strict sequence keeps changes to workflow admission/service and UI contracts reviewable without conflicting concurrent edits. Current statuses are planning states only; a Story becomes Done only with its specified tests, real-path evidence, reviewed commit and iteration-log entry. A later plan revision may parallelize independent files after the interfaces are settled.

## Cross-repository handoffs

- INOFY S11-A exports schema backed by its canonical decoder. Pin the resulting commit/version in VIVY only after its test evidence exists. S10 already supplies a reusable editor/host transport seam, but its VIVY adapter is not proof of a live host.
- S11-A's INOFY schema change passed `go test ./... -count=1` on 2026-09-29 and was published after explicit user approval as `6acfcc6b1a51921c45316a15eb0846bc0e775a9e`. VIVY pins `v0.0.0-20260929145515-6acfcc6b1a51`.
- VIVY S11-B freezes normalized Definition, catalog, ProgramMeta and authority at admission. S11-C binds the initial INOFY admission transition to that native Run with one atomic transaction. S11-D executes child activations; S11-E owns terminal cutover.
- S11-F implements native repository/CAS and host actions, then S11-G selects a product UI Module in `recipes/default.vivy.yml` and proves the browser flow. Product UI may be disabled without disabling core task graphs.
- INOFY S12 Garden and S13 release have their own plans. S13 cannot claim integrated G7/G10 evidence until S11-G passes; this plan does not silently mark S12 or S13 complete.

## Execution and evidence discipline

Before coding, refresh repository heads and scoped AGENTS instructions, confirm migration number and file paths, and resolve any contract drift in this index. Follow the per-Story failing test → minimal implementation → passing test/real-path gate → commit steps. VIVY requires `just ci`, both database drivers for storage gates, split-UI real-path smoke for product changes, and iteration-log entries; a skipped dependency is a blocked gate. This plan does not authorize skipping a required check. No tests have been run for these plan documents.

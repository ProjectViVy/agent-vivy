# Issue 39 implementation package: Eino-native orchestration

Epic: native governed children and bounded DAG; P1; [Issue #39](https://github.com/ProjectViVy/agent-vivy/issues/39). Baseline: agent-vivy `f6fb11bc71be2d06946ff33b0462aa56f9ff51ef`; Eino v0.9.13. [Architecture](../../specs/2026-09-23-issue39-eino-orchestration-design.md) owns architectural rules and the decision register. This index alone owns Story state and dependency edges. Each Story owns implementation steps and evidence. Proposed signatures and paths in the design/plans are not existing functionality.

## Read order, scope and release authority

Read repository `AGENTS.md`, the architecture, this index, then the Story; read `ui/AGENTS.md` for UI work. The user requested a detailed **planning package**, not implementation authorization. All implementation Stories remain **Planned · review gate**; the native feasibility Story must pass before the rest can move to Ready. An authorized implementer must rebase/review the current source and confirm interfaces before work. This package does not authorize a custom scheduler, external agent backend, #40 product, shared context, named mask inheritance, independent personality or swarm messaging. If G0 fails, stop and return evidence to #39 instead of implementing a fallback architecture.

## Requirements and acceptance identifiers

| ID | Observable outcome | Gate |
| --- | --- | --- |
| R1 | Governed child agent uses Service/Eino model, tool, approval and budget path; parity with limits and cancellation | G0, G1 |
| R2 | Child task Session persists across separate activation Runs, isolated from parent and identity/personality | G1 |
| R3 | Child list/status/wait/follow-up/interrupt share host/UI authority; old `child/*` calls remain compatible | G1, G3 |
| R4 | Agent-authored finite DAG validates before execution and is pinned to an immutable revision | G2 |
| R5 | Eino Workflow executes dependency fanout/join with stable Run mapping, restart, cancel, no repeated side effect | G0, G2 |
| R6 | Journal and host projection explain child/workflow lifecycle; opaque Eino checkpoint is execution-only | G0–G3 |
| R7 | Full SQL, runtime, RPC, UI and release verification recorded | G4 |
| R8 | Stable Agent Session routing supports durable parent-child mailbox messages; active-run delivery, acknowledgement/retry and restart semantics are proven before exposing message controls | D8, ORCH-02/04 |
| R9 | Interrupt preserves a continuable ChildSession; Run cancellation, optional Session close and descendant outcomes have distinct documented behavior | D9, ORCH-04 |
| R10 | Clean spawn is the default; any context fork is an explicit bounded snapshot of completed turns with no authority/personality inheritance | D10, ORCH-03/08 |
| R11 | Read-only child tools remain the baseline; any later write mode defines workspace ownership, isolation, conflicts and integration | D11 |
| R12 | Model and tool selection stay within the parent capability and budget envelope | D12, ORCH-03/08 |
| R13 | Admission, descendant budget/usage roll-up, cost projection and restart/backpressure behavior are truthful and non-duplicating | D13, ORCH-01/08 |
| R14 | One-shot task runs and continuable mailbox-addressable children have explicit, compatible lifecycle modes | D14, ORCH-02/04 |

## Epic / Story graph

| Story | Epic | Requirements | Outcome and immediate predecessor output | Plan | State | Evidence / next gate |
| --- | --- | --- | --- | --- | --- | --- |
| ORCH-01 | Native proof | R1,R5,R6 | Standalone integrated Service/Eino proof; none | [01](ORCH-01.md) | Planned · review gate | G0 unverified; Go toolchain absent in this planning workspace. |
| ORCH-02 | Child foundation | R1,R2,R6,R8,R14 | Durable binding for continuable child Sessions and admission conformance; explicit one-shot/continuable scope; 01: verified runtime semantics | [02](ORCH-02.md) | Planned · blocked by 01 and D14 schema scope | G1 schema/lifecycle review. |
| ORCH-03 | Child execution | R1,R2,R6 | Production native agent activation through Service; 02: durable binding, idempotent admission | [03](ORCH-03.md) | Planned · blocked by 02 | G1 parity and rollback. |
| ORCH-04 | Child controls | R2,R3,R6,R8,R9 | Host RPC/UI continuation and interrupt with durable wait; 03: native child runs; mailbox send/receive is gated by D8 | [04](ORCH-04.md) | Planned · blocked by 03; mailbox API blocked by D8 | G1/G3; do not treat later-Run follow-up as in-flight message delivery. |
| ORCH-05 | Graph foundation | R4,R6 | Validated immutable descriptor and durable revision admission; 02: SQL child binding conventions | [05](ORCH-05.md) | Planned · blocked by 02 | G2 validation/idempotency. |
| ORCH-06 | Graph execution | R1,R4,R5,R6 | Eino Workflow graph Run with governed node children; 03: native child invoker, 05: revision/validator | [06](ORCH-06.md) | Planned · blocked by 03,05 | G0 finding must still hold in production path. |
| ORCH-07 | Graph product surface | R3,R4,R6 | Host proposal/start/read/cancel and UI inspection; 04: child controls, 06: graph execution | [07](ORCH-07.md) | Planned · blocked by 04,06 | G3 real controls/locales. |
| ORCH-08 | Integrated acceptance | R1–R14 | Recovery/side-effect/SQL/UI integration and release evidence; 07: completed surface | [08](ORCH-08.md) | Planned · blocked by 07 | G1–G4 and all applicable D8–D14 evidence required before closure. |

The table is the **only** maintained dependency DAG. Derived waves: `{01}`, `{02}`, `{03,05}`, `{04,06}`, `{07}`, `{08}`. `03` and `05` can have independent owners after 02; `04` and `06` both touch App/Service and must coordinate shared edits or execute serially. No date/velocity forecast is asserted. The technical path G0 → G1 → G2 → G3 → G4 is a delivery gate, not a claim that every wave has passed. ORCH-01 is proof work only; no user-visible feature is released from the first wave. Mailbox address compatibility is required for continuable ChildSessions; D8 still gates live message delivery and receipt behavior. Peer/swarm messaging remains separately deferred under D5. Review D9–D14 in the architecture before accepting control, context, workspace, resource, or child-mode behavior.

## Coordination and source ownership

| Boundary | Primary Story(s) | Coordination rule |
| --- | --- | --- |
| `internal/runtime` Eino and Service | 01 proof, 03 child, 06 graph, 08 acceptance | Keep Eino imports inside `internal/runtime`/`internal/provider`; 03 owns native child invocation contract before 06 consumes it. |
| `internal/storage` domain/migrations | 02 child, 05 descriptor | Migration pairs SQLite/Postgres, append-only; 05 chooses next migration number on its rebased branch. Shared snapshot/journal semantics come from existing stores. |
| `internal/app` broker/wiring | 03,04,06,07 | Integrate sequentially if same files conflict. No second handwritten worker loop retained as a parallel product path. |
| `internal/rpc` and UI | 04 child, 07 graph | Extend existing types and store, then RunInspector. Follow Face contract and both locale catalogs; do not edit `ui/src/generated/**`. |

No Story changes another Story's status in a plan file. An owner updates this index once predecessor evidence is reviewed. If a contract changes, update architecture and every affected plan in one reviewable change, invalidate dependent readiness, then re-evaluate topology. Stop any lane whose input has changed. The human maintainer remains accountable for architecture and integrated acceptance; independent coding/review agents may examine disjoint proof or test areas once contracts are fixed. Parallel file edits require isolated worktrees and non-overlapping ownership.

## Review, evidence, handoff

For each implementation Story, record the baseline commit, failing and passing focused checks, relevant Journal/Run observations, DB backend results, risk/rollback decision and changed paths in its review or `docs/logs/YYYY-MM-DD-slug/`. Gate G4 requires `just ci`, the real development UI at `http://127.0.0.1:3015`, and a release log (`summary.md`, `verification.md`, `acceptance.md`). A missing tool or external dependency is a blocked gate, never a pass. In this planning workspace `go` is unavailable, so no runtime verification has been performed.

Before execution: reconcile against current `origin/main`, Issue #39, masks changes, worker and Service changes, Eino version, and active TODO rows `APR-TIMEOUT-UX`/`TOOL-REFUSAL-RESIDUAL`. Avoid claiming all child approval policies behave like parent until verified. Do not close #39 or publish broad capability based on unit fixtures alone. The initial run is intentionally read-only; write-capable children, worktree reconciliation, graph peer communication and context-sharing require separate approval/evidence.

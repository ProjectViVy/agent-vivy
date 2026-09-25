# Issue 39 implementation package: Eino-native orchestration

Epic: native governed children and bounded DAG; P1; [Issue #39](https://github.com/ProjectViVy/agent-vivy/issues/39). Contract-freeze baseline: agent-vivy `a836c4088953dd6f8997cfe0fba4c479bc4908fb`; Eino v0.9.13. [Architecture](../../specs/2026-09-23-issue39-eino-orchestration-design.md) owns frozen architectural rules and decision register. This index alone owns Story state and dependency edges. Each Story owns implementation steps and evidence. Proposed signatures and paths are not existing functionality.

## Read order, scope and release authority

Read repository `AGENTS.md`, architecture, this index, then the Story; read `ui/AGENTS.md` for UI work. ORCH-01 alone is execution-ready as technical proof. No production implementation or product interface is released. This contract freeze sets D4/D8/D10/D12/D14; it does not bypass G0/G1 feasibility gates or approve downstream Story readiness. No custom scheduler, external agent backend, #40 product, shared context, named mask inheritance, independent personality or peer/swarm messaging. If G0 fails, stop and return evidence to #39, no fallback architecture.

## Start preflight (2026-09-24)

| Check | Observed state | Start implication |
| --- | --- | --- |
| Source | Remote `main` remains `f6fb11bc71be2d06946ff33b0462aa56f9ff51ef`; plan branch is based on that commit, with no upstream diff. `go.mod` pins Go 1.26.4 and Eino v0.9.13. | ORCH-01 source references are current at this check. Recheck before starting if `main` changes. |
| Decision and scope | At the 2026-09-24 preflight, G0/D7 were unverified and child/mailbox decisions were pending. Task 3 later froze D4/D8/D10/D12/D14; that resolves these contracts but does not waive G0/G1 or release ORCH-02 implementation. | ORCH-01 remains the only execution-ready Story. No production implementation or user-facing control is approved by this documentation freeze. |
| Verification environment | This Linux planning checkout has no `go`, `just` or PowerShell (`powershell.exe`/`powershell`/`pwsh`); `VIVY_POSTGRES_TEST_DSN` is unset. The `justfile` uses PowerShell and `.github/workflows/ci.yml` provisions Go/just on `windows-latest`. | ORCH-01 is **plan-ready**, but cannot be executed or verified in this checkout. Use a Windows development/CI environment with Go from `go.mod`, `just`, PowerShell, pnpm and available SQL test backends; record any Postgres skip. GitHub CI on a PR or manual workflow dispatch supplies the established Windows gates, but has not run for this proof. |
| First handoff | [ORCH-01](ORCH-01.md) names the existing runtime/Service, checkpoint and graph fixtures, the test-only change boundary, failing then passing proof, and GO/NO-GO evidence. | Owner starts with API/identity call path, then a focused failing integration test. Stop downstream Stories if the production Service seam cannot satisfy G0. |

**Readiness:** ORCH-01 is ready to be picked up in a conforming Windows execution environment; its G0 result is **open**. ORCH-02–08 remain blocked by predecessors and their named decisions. A new implementation branch/worktree should be based on current `main`; this documentation branch remains the reviewable plan reference. Mark a later Story Ready here only after predecessor traces, schema/product decisions and toolchain results are recorded.

## Requirements and acceptance identifiers

## Frozen contract values

| Decision | Approved contract |
| --- | --- |
| D4 authority and lineage | Preserve immutable origin parent Run and Session. A new active authorizer Run in that same parent Session may continue after the origin parent Run terminates. Record current authorizer Run separately from each activation Run. Effective authority is current authority intersected with the original immutable ceiling. Parent Session deletion fences continuation. |
| D8 direct mailbox | Required, direct parent-child only, addressed to stable ChildSession. Durable stable message IDs, sender-scoped idempotency keys, recipient ordering, explicit lifecycle and durable receipt/cursor. Ack means admitted, not consumed. Consume at a verified safe point. Retry/restart is at-least-once; no exactly-once effect claim. |
| D10 context | Clean context only: explicit task, admitted direct messages and approved dependency outputs. No hidden transcript, personality or history. |
| D12 capability | Parent model only. Child tool selection may narrow, never widen, original immutable tool ceiling. |
| D14 modes | One-shot and continuable coexist. Legacy synchronous agent and DAG nodes default one-shot and non-addressable. Only explicit continuable mode has stable ChildSession and follow-up/mail. |

Every Story consumes these identities and rules: origin parent Run, current authorizer Run, activation Run, and ChildSession. The architecture owns the full failure and lifecycle contract; the index is the sole Story-state/dependency DAG.

| ID | Observable outcome | Gate |
| --- | --- | --- |
| R1 | Governed child agent uses Service/Eino model, tool, approval and budget path; parity with limits and cancellation | G0, G1 |
| R2 | Child task Session persists across separate activation Runs, isolated from parent and identity/personality | G1 |
| R3 | Child list/status/wait/follow-up/interrupt share host/UI authority; old `child/*` calls remain compatible | G1, G3 |
| R4 | Agent-authored finite DAG validates before execution and is pinned to an immutable revision | G2 |
| R5 | Eino Workflow executes dependency fanout/join with stable Run mapping, restart, cancel, no repeated side effect | G0, G2 |
| R6 | Journal and host projection explain child/workflow lifecycle; opaque Eino checkpoint is execution-only | G0–G3 |
| R7 | Full SQL, runtime, RPC, UI and release verification recorded | G4 |
| R8 | Required durable direct parent-child mailbox targets stable ChildSession, with stable IDs/idempotent admission/order/lifecycle, safe-point consumption, restart-safe receipt and at-least-once semantics; no exactly-once effect claims | D8, ORCH-02/04 |
| R9 | Interrupt preserves a continuable ChildSession; Run cancellation, optional Session close and descendant outcomes have distinct documented behavior | D9, ORCH-04 |
| R10 | Clean context only: explicit task, admitted direct messages and approved dependency outputs; no hidden transcript/personality/history | D10, ORCH-03/08 |
| R11 | Read-only child tools remain the baseline; any later write mode defines workspace ownership, isolation, conflicts and integration | D11 |
| R12 | Parent model only; child tool selection may narrow but never widen immutable authority ceiling | D12, ORCH-03/08 |
| R13 | Admission, descendant budget/usage roll-up, cost projection and restart/backpressure behavior are truthful and non-duplicating | D13, ORCH-01/08 |
| R14 | One-shot task runs and continuable mailbox-addressable children have explicit, compatible lifecycle modes | D14, ORCH-02/04 |

## Epic / Story graph

| Story | Epic | Requirements | Outcome and immediate predecessor output | Plan | State | Evidence / next gate |
| --- | --- | --- | --- | --- | --- | --- |
| ORCH-01 | Native proof | R1,R5,R6 | Standalone integrated Service/Eino proof; none | [01](ORCH-01.md) | Ready for Windows execution | G0 unverified. G0/G1 are not waived by this contract freeze. |
| ORCH-02 | Child foundation | R1,R2,R6,R8,R14 | Durable binding and direct mailbox conformance; explicit one-shot/continuable mode; 01: verified runtime semantics | [02](ORCH-02.md) | Planned · blocked by 01 | G1 schema/lifecycle review. PostgreSQL test is owner-deferred, unverified, and remains a later acceptance gate. |
| ORCH-03 | Child execution | R1,R2,R6 | Production native agent activation through Service; 02: durable binding, idempotent admission | [03](ORCH-03.md) | Planned · blocked by 02 | G1 parity and rollback. |
| ORCH-04 | Child controls | R2,R3,R6,R8,R9 | Host RPC/UI continuation, durable mailbox, interrupt and wait; 03: native child runs | [04](ORCH-04.md) | Planned · blocked by 03 | G1/G3; follow-up is not in-flight message delivery. |
| ORCH-05 | Graph foundation | R4,R6 | Validated immutable descriptor and durable revision admission; 02: SQL child binding conventions | [05](ORCH-05.md) | Planned · blocked by 02 | G2 validation/idempotency. |
| ORCH-06 | Graph execution | R1,R4,R5,R6 | Eino Workflow graph Run with governed node children; 03: native child invoker, 05: revision/validator | [06](ORCH-06.md) | Planned · blocked by 03,05 | G0 finding must still hold in production path. |
| ORCH-07 | Graph product surface | R3,R4,R6 | Host proposal/start/read/cancel and UI inspection; 04: child controls, 06: graph execution | [07](ORCH-07.md) | Planned · blocked by 04,06 | G3 real controls/locales. |
| ORCH-08 | Integrated acceptance | R1–R14 | Recovery/side-effect/SQL/UI integration and release evidence; 07: completed surface | [08](ORCH-08.md) | Planned · blocked by 07 | R1-R14 and G0-G4 evidence required. PostgreSQL conformance remains a required later gate; owner deferral is not a pass. |

The table is the **only** maintained Story-state dependency DAG. Derived waves: `{01}`, `{02}`, `{03,05}`, `{04,06}`, `{07}`, `{08}`. `03` and `05` can have independent owners after 02; `04` and `06` coordinate shared App/Service edits. G0→G4 are delivery gates, not passed status. ORCH-01 is proof only. PostgreSQL conformance is owner-deferred to a later dedicated iteration, remains unverified and is not a pass. Direct mailbox is required for continuable ChildSessions; peer/swarm remains deferred under D5. See architecture for frozen D4/D8/D10/D12/D14 contracts.

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

# Issue 39 implementation package: Eino-native orchestration

Epic: native governed children and bounded DAG; P1; [Issue #39](https://github.com/ProjectViVy/agent-vivy/issues/39). Contract-freeze baseline: agent-vivy `a836c4088953dd6f8997cfe0fba4c479bc4908fb`; Eino v0.9.13. [Architecture](../../specs/2026-09-23-issue39-eino-orchestration-design.md) owns frozen architectural rules and decision register. This index alone owns Story state and dependency edges. Each Story owns implementation steps and evidence. Proposed signatures and paths are not existing functionality.

## Read order, scope and release authority

Read repository `AGENTS.md`, architecture, this index, then the Story; read `ui/AGENTS.md` for UI work. ORCH-01 owns the D15 minimum recovery foundation followed by the integrated G0 proof. Implementation began on 2026-09-26: the durable tool-operation boundary and a bounded Service/Eino Workflow proof are present in the current worktree. G0 has not passed and no product interface is released. The user has authorized provisional branch implementation through ORCH-07 before their later end-to-end test; this work does not bypass G0/G1, authorize merge/release, or convert missing evidence into a pass. No custom scheduler, external agent backend, #40 product, shared context, named mask inheritance, independent personality or peer/swarm messaging. If evidence demonstrates a native incompatibility or unsafe recovery, stop that dependent path and return evidence to #39; no fallback architecture.

## Historical start preflight (2026-09-24)

This table records the earlier environment/scope inspection; revised readiness below and architecture D15 govern the next attempt.

| Check | Observed state | Start implication |
| --- | --- | --- |
| Source | Remote `main` remains `f6fb11bc71be2d06946ff33b0462aa56f9ff51ef`; plan branch is based on that commit, with no upstream diff. `go.mod` pins Go 1.26.4 and Eino v0.9.13. | ORCH-01 source references are current at this check. Recheck before starting if `main` changes. |
| Decision and scope | At the 2026-09-24 preflight, G0/D7 were unverified and child/mailbox decisions were pending. Task 3 later froze D4/D8/D10/D12/D14; that resolves these contracts but does not waive G0/G1 or release ORCH-02 implementation. | ORCH-01 remains the only execution-ready Story. No production implementation or user-facing control is approved by this documentation freeze. |
| Verification environment | This Linux planning checkout has no `go`, `just` or PowerShell (`powershell.exe`/`powershell`/`pwsh`); `VIVY_POSTGRES_TEST_DSN` is unset. The `justfile` uses PowerShell and `.github/workflows/ci.yml` provisions Go/just on `windows-latest`. | ORCH-01 is **plan-ready**, but cannot be executed or verified in this checkout. Use a Windows development/CI environment with Go from `go.mod`, `just`, PowerShell, pnpm and available SQL test backends; record any Postgres skip. GitHub CI on a PR or manual workflow dispatch supplies the established Windows gates, but has not run for this proof. |
| First handoff | [ORCH-01](ORCH-01.md) names the minimum D15 recovery foundation, real Service/Eino proof, crash matrix, and evidence needed to judge G0. | Owner starts with API/identity call path, then a focused failing integration test. Stop downstream Stories if the production Service seam cannot satisfy G0. |

**Readiness (development attempt 2026-09-26):** G0 remains **BLOCKED**, not a demonstrated Eino incompatibility. ORCH-01 implements D15 admission/claim/result recovery, a bounded Service-owned Eino A/B join, Service approval pause/resume while a sibling runs, cancellation, and a four-boundary separate-process crash matrix. A full `go test ./...` passed before the final broker-policy fix. After that fix, focused runtime/app tests, the updated 23-suite provider conformance, `go vet ./...`, storage and UI checks pass. The final full-suite rerun triggered automatic review because a test attempted to contact `api.deepseek.com` with unapproved test data; it was not retried. `just ci` also cannot run because `just` and PowerShell are absent; `VIVY_POSTGRES_TEST_DSN` is unset, so PostgreSQL DSN-backed conformance remains unverified. These gates are not passes and keep G0 blocked. See the [current acceptance record](../../../logs/2026-09-26-issue39-orchestration-g0/acceptance.md). The user's later owner directive permits provisional implementation through ORCH-07; G0/G1 and release remain blocked.

**Owner-directed provisional continuation (2026-09-26):** Following the user's instruction to continue through all planned implementation before their later end-to-end test, ORCH-02–07 may proceed as provisional branch work while G0 is blocked. This is not G0/G1 acceptance, does not authorize merge or release, and keeps all public surfaces gated. ORCH-08 may prepare and run available integration checks, but its unavailable E2E/release gates must remain blocked until real evidence exists. If ORCH-01 later fails a required gate or demonstrates an incompatibility, stop dependent work and return to the gate evidence before release.

**Gate retest (2026-09-28):** On `devin/1790577560-pg-dsn-conformance`, PostgreSQL DSN-backed conformance (`go test ./internal/storage/postgres` with `VIVY_POSTGRES_TEST_DSN`), full `just ci`, and the real host/browser E2E at `http://127.0.0.1:3015` all ran and pass — see the [retest acceptance record](../../../logs/2026-09-28-issue8-issue12-gate-retest/acceptance.md). The retest surfaced and fixed two PostgreSQL conformance defects (binary-safe `BYTEA` content params; exclusive-lease-aware continuity reopen) and a Smart-preset `submit_plan` approval-resume crash (`internal/runtime/plan_review.go`). G0/G1/G3 now have recorded evidence; G4 release remains the owner's decision and is not claimed here.

## Current development attempt (2026-09-26)

| Area | Evidence | Status |
| --- | --- | --- |
| D15 operation boundary | Run-scoped operation IDs bind request digest, stored middleware input and effective arguments; admission/claim/completion update the private SQL row and D15 `tool.operation` lifecycle event atomically. That event contains digests, not raw invocation arguments; existing `tool.requested` and approval-required payloads retain their prior contracts. Claimed unknown outcomes are fenced. | SQLite tests pass. PostgreSQL schema/tests are present; DSN-backed conformance is unverified. |
| Native Workflow | Service-owned Eino `Workflow` runs independent A/B nodes, joins explicit outputs, applies an order-only dependency, pauses for approval while a sibling continues, cancels safely, and resumes with the same checkpoint and immutable prompt. | Focused and full `internal/runtime` and `internal/app` suites pass. |
| D15 crash matrix | A fresh process recovers four boundaries using the same Run/checkpoint/operation IDs and an external durable effect fixture. | Focused separate-process test passes; completed results are reused and claimed unknown outcomes do not replay. |
| App / UI | Focused Go package tests, UI typecheck and the complete 402-test Vitest suite pass in the current attempt. | `just ci` cannot run because `just` and PowerShell are unavailable. A full `go test ./...` was not rerun after an earlier automatic-review block caused by an existing unapproved external test request. |
| Child modes and mailbox | Explicit one-shot/continuable lifecycle, Service/Eino activation, safe-point message receipts, idempotent `reply_parent`, and recipient/body caps are implemented. | SQLite and focused runtime/app/RPC/tool checks pass; PostgreSQL database behavior and integrated reauthorization/restart accounting remain unverified. |
| Immutable DAG and product surface | Bounded descriptor/revision, Eino Workflow, model-facing `workflow` tool, workflow RPC and localized RunInspector projection are implemented. | Focused Go and UI checks pass; integrated G0/G2, real host/browser E2E and G3 remain blocked. |
| G0 gate | No product interface has shipped. | BLOCKED until full repository CI and PostgreSQL DSN-backed conformance are verified. |

## Requirements and acceptance identifiers

## Frozen contract values

| Decision | Approved contract |
| --- | --- |
| D4 authority and lineage | Preserve immutable origin parent Run and Session. A new active authorizer Run in that same parent Session may continue after the origin parent Run terminates. Record current authorizer Run separately from each activation Run. Effective authority is current authority intersected with the original immutable ceiling. Parent Session deletion fences continuation. |
| D8 direct mailbox | Required, direct parent-child only, addressed to stable ChildSession. Durable stable message IDs, sender-scoped idempotency keys, recipient ordering, explicit lifecycle and durable receipt/cursor. Each recipient is capped at 128 admitted messages per ChildSession and bodies at 32 KiB. Continuable children reply through `reply_parent`. Ack means admitted, not consumed. Consume at the Service/Eino safe point. Retry/restart is at-least-once; no exactly-once effect claim. |
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
| ORCH-01 | Recovery foundation and native proof | R1,R5,R6 | D15 durable operation boundary, then integrated Service/Eino proof; no ORCH-02 prerequisite | [01](ORCH-01.md) | In progress · D15 operation boundary; G0 BLOCKED | Implement and verify D15, then process-restart/parallel/approval/cancel matrix. Historical NO-GO is not proof of Eino incompatibility. ORCH-02–08 stay gated. |
| ORCH-02 | Child foundation | R1,R2,R6,R8,R14 | Durable binding and direct mailbox conformance; explicit one-shot/continuable mode; 01: verified runtime semantics | [02](ORCH-02.md) | Provisional implementation present · G0 BLOCKED | [Provisional verification log](../../../logs/2026-09-26-issue39-orch02-provisional/verification.md). SQLite conformance and child/message focused checks pass; PostgreSQL DSN-backed execution, G0/G1, and release remain blocked. |
| ORCH-03 | Child execution | R1,R2,R6 | Production native agent activation through Service; 02: durable binding, idempotent admission | [03](ORCH-03.md) | Provisional implementation present · G0/G1 BLOCKED | Child activation and recovery use Service/Eino; full parity, integrated restart, and G1 acceptance remain blocked. |
| ORCH-04 | Child controls | R2,R3,R6,R8,R9 | Host RPC/UI continuation, durable mailbox, interrupt and wait; 03: native child runs | [04](ORCH-04.md) | Provisional implementation present · G0/G1/G3 BLOCKED | Child RPC/UI and direct messaging are wired; local tests pass. D9 close semantics remain open; real browser/E2E is pending. |
| ORCH-05 | Graph foundation | R4,R6 | Validated immutable descriptor and durable revision admission; 02: SQL child binding conventions | [05](ORCH-05.md) | Provisional implementation present · G0/G2 BLOCKED | Validator and paired revision storage are present; SQLite conformance passes, PostgreSQL DSN-backed behavior is unverified. |
| ORCH-06 | Graph execution | R1,R4,R5,R6 | Eino Workflow graph Run with governed node children; 03: native child invoker, 05: revision/validator | [06](ORCH-06.md) | Provisional implementation present · G0/G2 BLOCKED | Native Eino workflow, one-shot child nodes and bounded output projection are present; integrated G0/G2 recovery acceptance remains blocked. |
| ORCH-07 | Graph product surface | R3,R4,R6 | Host proposal/start/read/cancel and UI inspection; 04: child controls, 06: graph execution | [07](ORCH-07.md) | Provisional implementation present · G0/G1/G3 BLOCKED | Workflow RPC, model tool and localized RunInspector are present; local UI checks pass, real host/browser E2E remains pending. |
| ORCH-08 | Integrated acceptance | R1–R14 | Recovery/side-effect/SQL/UI integration and release evidence; 07: completed surface | [08](ORCH-08.md) | In progress · G0/G1/G3 evidence recorded · G4 BLOCKED | 2026-09-28 retest: PostgreSQL DSN conformance, `just ci`, and the real host/browser E2E all pass — see the [retest log](../../../logs/2026-09-28-issue8-issue12-gate-retest/acceptance.md). Release (G4) remains the owner's decision and is not claimed. |

The table is the **only** maintained Story-state dependency DAG. Derived waves: `{01}`, `{02}`, `{03,05}`, `{04,06}`, `{07}`, `{08}`. `03` and `05` can have independent owners after 02; `04` and `06` coordinate shared App/Service edits. G0→G4 are delivery gates, not passed status. The owner has authorized provisional implementation through ORCH-07 while those gates remain blocked; this affects work permission only, never acceptance/release status. PostgreSQL DSN conformance was run on 2026-09-28 and passes (retest log above). Direct mailbox is required for continuable ChildSessions; peer/swarm remains deferred under D5. See architecture for frozen D4/D8/D10/D12/D14 contracts.

## Coordination and source ownership

| Boundary | Primary Story(s) | Coordination rule |
| --- | --- | --- |
| `internal/runtime` Eino and Service | 01 recovery foundation/proof, 03 child, 06 graph, 08 acceptance | Keep Eino imports inside `internal/runtime`/`internal/provider`; 03 owns native child invocation contract before 06 consumes it. |
| `internal/storage` domain/migrations | 01 minimum recovery foundation, 02 child, 05 descriptor | Migration pairs SQLite/Postgres, append-only; 05 chooses next migration number on its rebased branch. Shared snapshot/journal semantics come from existing stores. |
| `internal/app` broker/wiring | 01 recovery boundary, 03,04,06,07 | Integrate sequentially if same files conflict. No second handwritten worker loop retained as a parallel product path. |
| `internal/rpc` and UI | 04 child, 07 graph | Extend existing types and store, then RunInspector. Follow Face contract and both locale catalogs; do not edit `ui/src/generated/**`. |

No Story changes another Story's status in a plan file. An owner updates this index once predecessor evidence is reviewed. If a contract changes, update architecture and every affected plan in one reviewable change, invalidate dependent readiness, then re-evaluate topology. Stop any lane whose input has changed. The human maintainer remains accountable for architecture and integrated acceptance; independent coding/review agents may examine disjoint proof or test areas once contracts are fixed. Parallel file edits require isolated worktrees and non-overlapping ownership.

## Review, evidence, handoff

For each implementation Story, record the baseline commit, failing and passing focused checks, relevant Journal/Run observations, DB backend results, risk/rollback decision and changed paths in its review or `docs/logs/YYYY-MM-DD-slug/`. Gate G4 requires `just ci`, the real development UI at `http://127.0.0.1:3015`, and a release log (`summary.md`, `verification.md`, `acceptance.md`). A missing tool or external dependency is a blocked gate, never a pass. The initial planning checkout lacked Go; the current provisional worktree has Go 1.26.4 and focused local checks are being rerun. This does not substitute for repository CI, PostgreSQL conformance, or browser/E2E evidence.

Before execution: reconcile against current `origin/main`, Issue #39, masks changes, worker and Service changes, Eino version, and active TODO rows `APR-TIMEOUT-UX`/`TOOL-REFUSAL-RESIDUAL`. Avoid claiming all child approval policies behave like parent until verified. Do not close #39 or publish broad capability based on unit fixtures alone. Continuable children and workflow nodes use read-only tools; the existing one-shot Service path may use an explicitly selected parent-authorized effectful tool through normal approval. General writable children, worktree reconciliation, graph peer communication and context-sharing require separate approval/evidence.

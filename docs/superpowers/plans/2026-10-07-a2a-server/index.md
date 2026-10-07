# A2A Server Implementation Plan Package

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans or, when explicitly selected, superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. Read this index, the assigned Story and the linked design before work.

**Goal:** Deliver issue #2's removable, authenticated northbound A2A server through native VIVY task authority.

**Architecture:** A T2 Channel Module translates the official SDK into a narrow TaskHost; Core owns admission, ownership, Journal projection, cancellation and continuation. Keep native Run/Eino execution and local approval authority. The SDK default handler/TaskStore alternative is rejected because it introduces execution and mutable task authority outside VIVY.

**Tech Stack:** repository Go 1.26.4, SQLite/PostgreSQL, Eino v0.9.13, official `a2a-go/v2 v2.6.0`, A2A 1.0 JSON-RPC/SSE, generated Module Assembly.

**Spec:** [Single architecture and detailed design](../../specs/2026-10-07-a2a-server-design.md), detailed baseline `58d11432f559607e6b0ee6e48e3217ade93a642a`; native code baseline `dd78fcf142f384d47ce5cfefb43738fdb9a7346d`.

**Authorization:** On 2026-10-07 the owner explicitly requested this package before G0 closure. That authorizes preparing all Story plans now; it does not approve the proposed reconnect acceptance change, schedule functional work, or mark G0/G1/G2 passed. [Issue #2](https://github.com/ProjectViVy/agent-vivy/issues/2) was re-read and remains open/unscheduled. Its earlier plan-after-G0 ordering is superseded only for plan preparation by this request. Issue text and labels are unchanged.

## Consolidation and source disposition

This package on remote branch `A2A` is the sole maintained successor to
`docs/issue2-a2a-server-plan` at `99f9b7b2f3a7cab3a87b04c1989a7d5824dc9d10`.
The source commit is retained in the consolidation history; reading or
executing this package does not require the old branch. The owner authorized
absorbing details before deleting it. Contract choices remain in
[design section 1.1](../../specs/2026-10-07-a2a-server-design.md#11-consolidated-scope-and-decisions-still-owned-by-g0).

| Earlier artifact/Story | Current authoritative home |
|---|---|
| 2026-09-29 design and old index | Current design, especially sections 1.1 and 8.2; this index |
| Old A2A-00: contract/probe | A2A-00, including issue amendment and default-Generation conflict |
| Old A2A-01: task/grant seam | A2A-01; conditional replay addition belongs to A2A-R1 |
| Old A2A-02: admission/ownership | A2A-02; newer prompt/candidate/locking evidence retained |
| Old A2A-03: projection/replay | A2A-04 for standard/native projection; A2A-R1 for wire replay |
| Old A2A-04: listener/auth | A2A-05; earlier minimum deployment remains a G0 alternative |
| Old A2A-05: official adapter | A2A-06.1–2 |
| Old A2A-06: exact replay | Conditional A2A-R1, design section 8.2 |
| Old A2A-07: integration/omission | A2A-06.3 for standard path; A2A-R1.2 refreshes B acceptance |

Seven base Stories retain 18 tasks. One conditional Story adds two tasks,
which remain blocked unless G0 selects B. Retiring the source branch does
not approve remote question answers, a broader deployment envelope or B.

## Global Constraints

- `taskId = run_id`; `contextId = session_id`.
- Module `projectvivy/a2a-server`, source `plugins/a2a-server/`, provider/instance `a2a`, `std/channel@v1`, `core/channel-host@v1`, grant `channel.a2a`; keep Channel cardinality `0..n`.
- No second Runner, TaskStore, Journal, Policy path or agent loop. No A2A SDK/Eino types in the public TaskHost seam; no Eino import outside existing runtime/provider quarantine.
- Do not add task methods to base `channel.Host`. Preserve existing Channel behavior and typed-nil capability discovery.
- Do not route A2A content through `PublishInbound`. Ordinary answers never settle ApprovalStore; remote approval and outbound A2A are excluded.
- PENS remains an independent application environment and optional external client. No PENS, QQ, NeuroLink, image backend, Studio or UI feature is added here.
- Core owns paired immutable SQLite/PostgreSQL migrations. Receipts/ownership are indexes, not lifecycle/output stores. No plugin database or DDL.
- Official SDK custom `RequestHandler` plus `NewJSONRPCHandler`; no SDK `NewHandler`, `AgentExecutor` or `TaskStore` construction. First-cut completed text segments, not token streaming.
- Host owns listener/authentication/limits. Option A has only `POST /a2a` and `GET /.well-known/agent-card.json`; B adds only the exact section 8.2 replay path after adoption. No management `/rpc`, global mux or hard-coded `:8787`.
- Design sections 5–11 own exact limits, states and errors. Plans specify code/test locations, not alternate values. Protective limits are proposed settings, not measured performance.
- All functional Stories use `.agents/skills/vivy-plugin` and `vivy-kernel-ci`; future runtime work additionally uses `vivy-eino`. Product acceptance requires `just ci` and real-path evidence. Never count skipped PostgreSQL tests as passes.
- Only human contribution identity may author/commit work. Never touch tenant `data/vivy.db`, `data/demo/` or `data/workspaces/`.

## Scope and requirement coverage

| Requirement | Observable outcome | Design | Owning Stories |
|---|---|---|---|
| R1 Composition | Optional task seam, correct Module grants, old Channels unaffected | 3–5 | A2A-01, A2A-06 |
| R2 Admission | Missing-context retries return one native Session/Run; immutable prompt and durable receipt | 6 | A2A-02 |
| R3 Continuation | Ordinary remote answer is atomic, actor-aware and resumes the same Run or fails durably on recovery | 6, 9 | A2A-03 |
| R4 Projection | Journal-authoritative, bounded state/history/artifacts and ordered subscriptions | 7–8 | A2A-04 |
| R5 Isolation | Principal ownership, safe not-found, deletion tombstones, credential rotation and revocation | 5–6, 10 | A2A-02, A2A-04, A2A-05 |
| R6 Transport | Host-owned authenticated endpoint, discovery, lifecycle, limits and safe errors | 10–11 | A2A-05, A2A-06 |
| R7 Interoperability | Official client drives the approved A2A subset and reconnect contract | 7–8 | A2A-00, A2A-06 |
| R8 Human control | Local approval only; cancellation affects exactly the mapped Run | 9 | A2A-03, A2A-04, A2A-06 |
| R9 Artifact delivery | Explicit Recipe, truthful Inspect/conformance, selected/omitted builds and rollback | 4, 12–13 | A2A-06; A2A-R1 refreshes B artifacts |
| R10 Conditional exact replay | If B is adopted, retained public events recover across disconnect/terminal/restart with durable client deduplication | 8.2 | A2A-R1 |

## Epics and Story authority

E0 establishes evidence and adoption. E1 supplies governed native task operations. E2 exposes and validates the removable protocol surface. This table is the sole mutable source of Story status and dependencies; Story files do not maintain independent status checklists.

| Story | Epic / requirements | Outcome | Immediate predecessors / required accepted output | Plan | Status | Evidence / blocker |
|---|---|---|---|---|---|---|
| A2A-00 | E0 / R7 | SDK probe, preparation lifecycle decision, G0 adoption | None | [A2A-00](A2A-00.md) | Planned | Execution not requested; Go/just absent here; sections 1.1/8 choices, default-Generation resolution and native preparation evidence pending |
| A2A-01 | E1 / R1 | Public TaskHost values and grant-preserving Assembly wiring | A2A-00: frozen contract and recorded G0 approval | [A2A-01](A2A-01.md) | Blocked | G0 not accepted; functional work unscheduled |
| A2A-02 | E1 / R2, R5 | Atomic native admission, scoped receipts and deletion tombstones | A2A-01: SDK values and stable Module/provider/instance identity | [A2A-02](A2A-02.md) | Blocked | Native provisional-resource contract from G0 and predecessor acceptance required |
| A2A-03 | E1 / R3, R8 | Atomic ordinary answers and honest crash recovery | A2A-02: core receipt/ownership transactions and launch discipline | [A2A-03](A2A-03.md) | Blocked | Native admission acceptance required |
| A2A-04 | E1 / R4, R5, R8 | Bounded TaskHost reads, cancellation and replay/live projection | A2A-03: complete accepted-message/answer event contract and native transitions | [A2A-04](A2A-04.md) | Blocked | Native task/answer evidence required |
| A2A-05 | E2 / R5, R6 | Dedicated Host HTTP lifecycle, authentication and discovery view | A2A-04: real TaskHost and private request-binding contract | [A2A-05](A2A-05.md) | Blocked | G1 must be accepted and G2 explicitly scheduled |
| A2A-06 | E2 / R1, R6–R9 | Official SDK Module, Recipe and end-to-end artifact acceptance | A2A-05: authenticated listener, limits and live discovery projection | [A2A-06](A2A-06.md) | Blocked | Host lifecycle acceptance and G2 authorization required |
| A2A-R1 | E2 / R10, R5, R9 | Optional exact-event replay and refreshed enhanced-client/artifact acceptance | A2A-06: accepted standard adapter, native Host and artifact driver | [A2A-R1](A2A-R1.md) | Blocked | B not selected; wire/error contract unresolved; G0 adoption, upstream acceptance and G2 scheduling required |

### Dependency order and shared files

Immediate DAG: `A2A-00 -> A2A-01 -> A2A-02 -> A2A-03 -> A2A-04 -> A2A-05 -> A2A-06`.
Topological waves: `{A2A-00}`, `{A2A-01}`, `{A2A-02}`, `{A2A-03}`, `{A2A-04}`, `{A2A-05}`, `{A2A-06}`. This intentionally serializes integration: the reducer consumes the final answer schema, and listener activation consumes a real Host. No time-based critical path or delivery date is claimed.

File conflicts reinforce this sequence: A2A-01/04/05 share app/Host wiring; A2A-02/03 share native admission/storage; A2A-03/04 share Journal/event semantics; A2A-01/06 share Assembly/conformance. Use one write lane. If parallel execution is later useful, first split ownership and use isolated worktrees; logical readiness alone never permits concurrent edits to these files.

If B is selected, append `A2A-06 -> A2A-R1`; its sole direct predecessor supplies all upstream contracts transitively. A2A-06 then accepts the standard-server milestone only; G2/issue acceptance also requires A2A-R1. If A is selected, A2A-R1 is unselected and cannot block standard acceptance. Neither path permits a separate task/event authority.

The original D0–D6 design slices map one-to-one to A2A-00–06. A2A-04 now waits for A2A-03 acceptance instead of integrating an unaccepted answer schema later. This is execution sequencing, not a changed product contract.

## Review Focus

These five failure classes are assigned once here and concretely tested in their owning tasks.

| Focus | User expectation | Test owner |
|---|---|---|
| RF1 Retry equivalence | Response options may change on retry; execution selectors/text may not. Answer normalization is stable. | A2A-02.2, A2A-03.1 |
| RF2 Deleted or foreign identity | Guessing IDs, rotating a token or retrying after deletion cannot transfer ownership or recreate work. | A2A-02.3, A2A-04.2 |
| RF3 Race at stream boundaries | Terminal commit, missed publish or interrupted state never leaves a stream hanging or invents success. | A2A-04.3 |
| RF4 Proxy and lifecycle inputs | Forged headers, invalid TLS/config, credential revocation and shutdown cannot expose management authority. | A2A-05.1–2, A2A-06.1 |
| RF5 Reply budget after acceptance | An oversized artifact/list yields an honest bounded error without re-running or rewriting the native result. | A2A-04.2, A2A-06.1 |

## Contract handoff and Eino capability check

- A2A-01 owns public declarations from design section 5. A2A-02 owns internal admission/store declarations. A2A-03 owns answer event version/transition semantics. A2A-04 owns Host-private binding and projection declarations. A2A-05 owns typed HTTP config. A2A-06 consumes these without redefining them.
- Existing `adk.NewRunner`, `Runner.Run` and `Runner.ResumeWithParams` in pinned Eino v0.9.13 already own execution/resumption through `internal/runtime/engine.go`. Upstream source at `c5e6aef927cca02bea934541f8dff2ea711b2ca7` was inspected. No new loop, checkpoint manager or Eino wrapper is needed.
- Eino does not replace VIVY's SQL ownership/receipt transaction or a public A2A transport. The small TaskHost adapter addresses that concrete boundary; no historical EinoExt server is adopted. If upstream later supplies a transport suitable for this seam, only the Module adapter is replaceable; native authority remains.
- PLG-P9 is recorded complete in the existing platform index (2026-09-14, run #147). A2A-00 verifies that evidence is still present at the execution base. A past platform pass is not an A2A acceptance pass.

## Acceptance fixture ownership

All names below are proposed tests, not existing passing evidence. Story-local subtests expand the design's 15 fixtures; they do not create a second acceptance definition.

| Design fixture | Owning task |
|---|---|
| `TestA2ACustomHandlerOfficialClient` | A2A-00.1, reused/migrated in A2A-06.1 |
| `TestChannelTaskHostGrantWrapper` | A2A-01.2 |
| `TestChannelTaskMissingContextConcurrentRetry` | A2A-02.2 |
| `TestChannelTaskReplayBeforeBusyAndQuota` | A2A-02.2 |
| `TestChannelTaskAdmissionFaultMatrix` | A2A-02.1–2 |
| `TestChannelTaskNewSessionPreparation` | A2A-02.2, contract supplied by A2A-00.2 |
| `TestChannelTaskOwnershipAndTombstone` | A2A-02.3, A2A-04.2 |
| `TestChannelTaskAnswerAtomicRace` | A2A-03.1 |
| `TestChannelTaskAnswerCrashRecovery` | A2A-03.2 |
| `TestTaskTextProjectionEquivalence` | A2A-04.1 |
| `TestTaskStreamSnapshotTailRaces` | A2A-04.3 |
| `TestTaskListBoundsAndTokens` | A2A-04.2 |
| `TestA2AHTTPIsolationAndLifecycle` | A2A-05.1–2 |
| `TestA2AApprovalIsLocalOnly` | A2A-03.2, A2A-06.2 |
| `TestA2ASelectedAndOmittedArtifacts` | A2A-06.3; A2A-R1.2 repeats after B changes |
| Conditional `TestTaskReplayRetainedHistory` / `TestA2AReplayWire` | A2A-R1.1 |
| Conditional `TestA2AReplayDurableClient` / `TestA2AReplayDenials` | A2A-R1.2 |

## Execution and evidence rules

1. Only planning is currently authorized. A2A-00 is the first work package when execution is requested. Its probe/investigation can proceed before the owner's reconnect decision; its adoption task cannot.
2. At kickoff check branch/base drift, Go 1.26.4, `just`, repository sibling-module setup from README/CI, and a disposable PostgreSQL server. Resolve missing tooling through supported setup; do not edit replace directives merely to make a probe pass. Do not merge a newer main blindly into this design branch.
3. Proposed test blocks are assertion sketches, not precompiled code. Use Go's existing testing conventions; no assertion framework or new test runner is required. Red means the named behavioral assertion or missing proposed symbol fails, not unrelated setup failure.
4. Each task is a reviewable commit. Stage only its listed paths. Each Story's final commit includes `docs/logs/<actual-date>-a2a-<story-number>/{summary,verification,acceptance}.md` and this index's evidence/status update; reuse that directory within the Story. Human review gates remain open until checked.
5. Run focused tests during each task. At each functional Story boundary run `just ci`; storage changes additionally require actual SQLite and PostgreSQL parity, migration/reopen/failure evidence. Preserve commands, revision, exit code and skips in the Story log.
6. Before A2A-05, independently confirm G1's combined SDK, admission, answer and projection acceptance. Only the owner schedules G2. After A2A-06, verify the entire selected/omitted product, not just mocked Host or SDK transport tests.
7. If G0 selects B, adopt section 8.2 and A2A-R1, freeze its wire/error contract and record the one-extension issue exception before making functional Stories Ready. Selecting A leaves that conditional Story unselected. Resolve section 1.1 continuation/deployment/default-Generation choices and propagate any narrowing to the existing Stories before execution; the current package does not implement an unapproved extension. If provisional persona cleanup needs a native API, freeze its exact symbols and failure/recovery tests in the design and A2A-02 before execution of that task.
8. Rollback disables the endpoint or omits the Module. Keep native history and receipt tombstones. Binary downgrade across migrations needs an explicitly tested compatibility window; do not invent down-migrations.

## Current delivery evidence

The [consolidation record](../../../logs/2026-10-07-a2a-plan-consolidation/verification.md) records the source-to-successor coverage, including the conditional replay plan. The package itself is documentation only. [Planning iteration](../../../logs/2026-10-07-a2a-plan-package/verification.md) records link/path/DAG/coverage checks and self-review. No Story test, SDK probe, CI run, listener or migration is claimed as implemented. There is no Ready implementation Story at this handoff.

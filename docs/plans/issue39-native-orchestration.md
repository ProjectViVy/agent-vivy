# issue39-native-orchestration - Work Plan

## TL;DR (For humans)
**What you'll get:** Vivy will have governed disposable task agents, explicitly continuable child agents with durable direct parent-child messaging, and small validated dependency workflows whose execution and evidence stay inside Vivy's existing runtime.

**Why this approach:** The first delivery proves the pinned Eino engine can satisfy Vivy's approval, cancellation and recovery contracts. Only that proven native path may advance; identity, authority, budget and durable evidence remain owned by Vivy.

**What it will NOT do:** It will not add an unrestricted swarm, saved workflow product, external-agent delegation, writable parallel workers, shared personality/history, a second runtime, or a fallback scheduler.

**Effort:** XL
**Risk:** High - approval-safe recovery and durable messaging cross runtime, persistence, host and UI boundaries.
**Decisions to sanity-check:** One-shot and continuable modes coexist; DAG tasks remain one-shot by default; same-origin Session Runs may reauthorize a child; clean context and parent model are fixed.

Current state (2026-09-26): provisional implementation for ORCH-02–07 is present, including child lifecycle and direct mailbox, native Service/Eino activation, finite immutable workflow revisions, native Workflow execution, the model-facing `workflow` and `reply_parent` tools, RPC, and UI projections. Focused local Go/UI checks pass and are recorded. G0/G1 and release remain BLOCKED: PostgreSQL DSN-backed conformance, repository `just ci`, integrated R1-R14 evidence and real host/browser E2E are not complete. The owner will run E2E after implementation is ready. See [current G0 evidence](../logs/2026-09-26-issue39-orchestration-g0/acceptance.md) and [provisional verification](../logs/2026-09-26-issue39-orch02-provisional/verification.md). Missing evidence is BLOCKED; a demonstrated incompatibility or unsafe replay is NO-GO for the tested path.

---

> TL;DR (machine): XL/high-risk gated delivery of Eino-native governed children, durable direct mailbox and bounded Workflow DAG, with eight Story contracts and full recovery/UI acceptance.

## Scope
### Must have
- Keep `docs/superpowers/plans/issue39-eino-orchestration/index.md` as the only Story-state/dependency authority and execute waves `{01}`, `{02}`, `{03,05}`, `{04,06}`, `{07}`, `{08}`.
- G0 must prove the real Service, policy, approval, budget, broker, Eino Workflow and checkpoint path before child or graph surfaces ship. The owner has authorized provisional branch implementation before that gate; implementation status does not change the G0 release gate.
- Preserve the legacy synchronous one-shot agent tool; make DAG nodes one-shot by default; add a distinct explicit continuable mode with stable ChildSession identity.
- Permit post-origin-Run continuation only from a new active Run in the same origin parent Session, with effective authority equal to current authority intersected with the immutable original ceiling.
- Deliver durable direct parent-child messages with stable IDs, operation idempotency, recipient order, admitted/deliverable/consumed/failed-or-expired states, restart preservation and a proven Eino safe point.
- Use Eino Workflow for dependency execution. Host validation owns finite node/depth/width/input/output limits; Service owns concurrency, budget, backpressure and cancellation.
- Preserve Journal/Run as public truth and Eino checkpoint as opaque continuation state. Persist and project graph, node, activation and message identities explicitly.
- Verify SQLite and PostgreSQL, actual restart/crash boundaries, public RPC, English/Chinese UI, accessibility and real `:3015` browser behavior.
### Must NOT have (guardrails, anti-slop, scope boundaries)
- No `WithMaxRunSteps` on Eino Workflow, `AddEnd` fiction, custom scheduler, second worker/model loop, second Journal, or Eino types outside the import quarantine.
- No unrestricted peer/sibling swarm, task claiming, optional #40 workflow product, external-agent provider, cross-Session takeover, context fork, model picker or persistent child personality.
- No mailbox completion claim from `child/followup`, stable addressing alone, mocked UI, standalone lambdas, skipped PostgreSQL tests, grep output or process-memory projections.
- No migration edits after release, unsafe overwrite of unrelated worktree changes, silent replay of unknown effects, or terminal Run append.

### Authoritative product decisions
These values were explicitly selected by the repository owner during the Issue #39 planning approval in this session and are durably recorded in `.omo/drafts/issue39-native-orchestration.md`. Executors must apply them as requirements, not reopen them or substitute defaults.

| Decision | Authoritative value |
| --- | --- |
| D4 parent-terminal continuation | A new active Run in the same original parent Session may reauthorize a continuable child after the origin parent Run terminates. Preserve immutable origin lineage; effective authority is current authority intersected with the original delegation ceiling. Parent Session deletion permanently rejects continuation. |
| D8 direct mailbox | Complete durable direct parent-child messaging is required in this delivery. Stable addressing and follow-up are insufficient. Admission uses stable IDs/idempotency/order; consumption, retry, interruption and restart states must be proven before exposing the API. |
| D10 context | Clean context only. No completed-turn fork, shared transcript, in-flight copy, personality data or hidden prompt inheritance. Explicit task, admitted direct messages and approved dependency outputs are the only cross-boundary context. |
| D12 model/tools | Child uses the parent model; no picker or override. Tool selection may only narrow the currently authorized parent set and may never widen the immutable original ceiling. |
| D14 child modes | One-shot and continuable modes coexist. Existing synchronous agent tool and DAG nodes default to one-shot/non-addressable. Only explicit continuable creation owns a stable ChildSession and accepts follow-up/mail. |
| Test strategy | TDD plus real Service/Eino integration, SQLite and PostgreSQL, and real browser acceptance at `http://127.0.0.1:3015`. |

### Owner execution directives (2026-09-26)

- Recovery revision: the owner approved architecture D15 after reviewing the historical NO-GO. Task 5 now includes the minimum durable logical operation, atomic claim, completion/error persistence and proof-node admission before the integrated G0 test. Reuse existing Journal/storage transactions; a necessary narrow paired migration is allowed here, without enabling general Task 6 child/mailbox scope. Unknown effects must stop automatic replay; universal external exactly-once execution is not an acceptance requirement. This supersedes the prior terminal disposition and proof-only restriction, not the safety gate. Historical evidence remains unchanged.
- Provisional continuation: the owner authorized implementation before providing credentials and will run end-to-end tests after code is complete. The owner has since authorized provisional, unshipped implementation through ORCH-07 while G0 is BLOCKED. This does not change Task 5 acceptance, convert missing evidence into a pass, or authorize merge/release. ORCH-08 may run available checks, but unavailable PostgreSQL, `just ci`, E2E, G0/G1 and release evidence remain blocked. Stop the affected path before release if G0 fails or a dependent safety invariant cannot be proven.

- Plan-vs-need: actual user-visible requirements take precedence over this plan's technical route. Executors may select or adjust implementation details without further confirmation when user-visible behavior and contracts remain satisfied and repository architecture/safety constraints are preserved. Record the decision, evidence, alternatives, and resulting delta. Ask only for missing user-visible acceptance or a safety boundary, not an invisible implementation preference.
- G0 proof-order revision: the owner approved a minimal, compileable, behaviorless Service-owned lifecycle entrypoint returning typed `ErrNativeOrchestrationUnimplemented`, followed by a real Eino Workflow integration test that reaches this Service boundary and fails. Task 5 implements the minimum shared Service-owned Run/admission/checkpoint/child-activation path. This exception enables the RED proof only; G0 acceptance is unchanged, behavioral production work remains test-first, and no standalone-lambda success or custom scheduler is allowed.

## Verification strategy
> Zero human intervention - all verification is agent-executed.
- Test decision: TDD. Go `testing` plus existing storage conformance, Vitest, Playwright/manual browser automation, and repository `just ci`.
- Every implementation todo begins with a focused failing test whose failure names the missing contract, then the minimum change, then focused pass and broader gate.
- PostgreSQL is PASS only when `VIVY_POSTGRES_TEST_DSN` is set and the named tests report executed cases; skipped output is UNVERIFIED and blocks release.
- Evidence: `<attemptDir>/task-<N>-issue39-native-orchestration.{md,json,log,png}` where `attemptDir` is the current ulw-loop attempt directory; outside ulw-loop use `.omo/evidence/issue39-native-orchestration/`.
- Each evidence record includes source commit, exact command, test names actually run, Run/Session/message/revision/checkpoint IDs, Journal sequences, effect counts, backend and explicit PASS/BLOCKED/NO-GO.

## Execution strategy
### Parallel execution waves
> Target 5-8 todos per wave. Fewer than 3 (except the final) means you under-split.
- Wave 0, preparation: todos 1-3. Todo 1 and 2 may run in parallel in separate read-only lanes; todo 3 consumes both.
- Wave 1, G0: todos 4-5 sequentially. Task 5 first establishes the D15 recovery foundation, then runs G0. BLOCKED or NO-GO keeps later waves pending; it never marks unexecuted work complete.
- Wave 2, durable child foundation: todos 6-7 sequentially.
- Wave 3, independent foundations: todos 8 and 10 may run in parallel after todo 7 in isolated worktrees; coordinate migration numbers and shared Service contracts before merging.
- Wave 4, controls and native graph: todos 9 and 11 may run in parallel only if file ownership is disjoint; otherwise sequence them because both touch App/Service.
- Wave 5, product surface: todo 12.
- Wave 6, integrated acceptance: todos 13-14 sequentially, then F1-F4 in parallel.

### Dependency matrix
| Todo | Depends on | Blocks | Can parallelize with |
| --- | --- | --- | --- |
| 1 | none | 3,4 | 2 |
| 2 | none | 3,4 | 1 |
| 3 | 1,2 | 4 | none |
| 4 | 3 | 5 | none |
| 5 | 4 | 6-14 | none |
| 6 | 5 GO | 7 | none |
| 7 | 6 | 8,10 | none |
| 8 | 7 | 9,11 | 10 |
| 9 | 8 | 12 | 11 when files do not overlap |
| 10 | 7 | 11 | 8 |
| 11 | 8,10 | 12 | 9 when files do not overlap |
| 12 | 9,11 | 13 | none |
| 13 | 12 | 14 | none |
| 14 | 13 | F1-F4 | none |

## Todos
> Implementation + Test = ONE todo. Never separate.
<!-- APPEND TASK BATCHES BELOW THIS LINE WITH edit/apply_patch - never rewrite the headers above. -->
- [x] 1. Reconcile the execution baseline and isolate the write lane
  What to do / Must NOT do: Use `git-master` read-only inspection to record branch, HEAD, `origin/main`, worktree status, Eino pin, migrations, active Issue #39 comments and overlapping mask/worker/Service changes. Create or select a task-owned clean worktree for execution because the current root contains unrelated `.omo/.omc` operational state. Never reset, clean, stage or overwrite unrelated changes.
  Parallelization: Wave 0 | Blocked by: none | Blocks: 3,4
  References: `AGENTS.md` Parallel lanes and Delivery; `docs/superpowers/plans/issue39-eino-orchestration/index.md:5-18,63-69`; `go.mod:3,13`; GitHub Issue #39 comments `5754140743`, `5754206830`, `5798636327`.
  Acceptance criteria: evidence records `git status --short --branch`, `git rev-parse HEAD origin/main`, `git diff --check`, current migration tails, and confirms the chosen worktree has no unrelated tracked edits; all baseline drift is reconciled into todo 3 before code.
  QA scenarios: happy - exact baseline and isolated worktree recorded; failure - changed Eino pin, migration collision or overlapping active lane blocks execution with paths and owner. Use PowerShell/git; Evidence `<attemptDir>/task-1-issue39-native-orchestration.md`.
  Commit: N | preparation evidence only

- [x] 2. Establish a non-skipping Windows verification environment
  Deferred by repository owner on 2026-09-25: PostgreSQL conformance execution is assigned to a later dedicated iteration. This is not a PostgreSQL PASS; the recorded exit-0 SKIP remains BLOCKED evidence. Do not claim PostgreSQL parity or complete schema acceptance until that follow-up runs the named tests against a real disposable backend.
  What to do / Must NOT do: Verify Go 1.26.4, Node/pnpm, `just`, PowerShell, local Eino v0.9.13 source and a disposable PostgreSQL DSN. Put `C:\Program Files\Go\bin` on the task process PATH when needed; do not modify global environment or expose DSNs. Run a named SQLite and PostgreSQL conformance test to prove both execute rather than skip.
  Parallelization: Wave 0 | Blocked by: none | Blocks: 3,4
  References: `README.md` Requirements/Quick start; `justfile`; `.github/workflows/ci.yml`; `internal/storage/postgres/conformance_test.go:20-22`; `docs/logs/2026-09-24-issue39-start-preflight/verification.md`.
  Acceptance criteria: `go version`, `just --version`, `node --version`, `pnpm --version` and PowerShell version are captured; `go env GOMODCACHE` contains Eino v0.9.13; focused SQLite test shows named executed cases and PASS; PostgreSQL execution is explicitly deferred by owner and remains BLOCKED (never count a skip as pass). Any later database/schema delivery that requires backend parity remains gated on real PostgreSQL conformance.
  QA scenarios: happy - SQLite conformance executes; PostgreSQL is explicitly owner-deferred with no pass claim; failure - missing tool/SQLite failure blocks and any skipped PostgreSQL result remains unverified. Use PowerShell/Go; Evidence `<attemptDir>/task-2-issue39-native-orchestration.log` with secrets redacted.
  Commit: N | environment evidence only

- [x] 3. Correct and freeze Issue #39 contracts before production code
  Resume note (2026-09-26): contract freeze commit b77090618c175c8c1dec3aeafcb3576d5d5e0b38 is independently confirmed, contains the 12 intended documentation paths, and is the base of the G0 worktree. Independent link traversal found 29 targets and 0 failures. At that point G0 remained open; Task 5 below retains the later historical NO-GO and records the approved recovery revision.
  What to do / Must NOT do: Update the architecture, index and ORCH-01–08 plans together. Close D4/D8/D10/D12/D14 using the exact values in this plan's Authoritative product decisions table, whose approval source is `.omo/drafts/issue39-native-orchestration.md`; specify origin parent Run, current authorizer Run, activation Run and ChildSession identities; define mailbox states and at-least-once/idempotent semantics; make direct messaging required; replace `WithMaxRunSteps` and `AddEnd` sketches with host finite limits and actual `End()` usage; change ORCH-08 acceptance to R1-R14; resolve disconnected-node validation as stated in this plan. Keep index as sole Story status DAG and ORCH-01 as the only execution-ready Story until G0 passes. Do not reinterpret or reopen the approved product decisions during execution.
  Parallelization: Wave 0 | Blocked by: 1,2 | Blocks: 4
  References: `docs/superpowers/specs/2026-09-23-issue39-eino-orchestration-design.md:39-46,74-112,139-156`; `index.md:20-69`; all `ORCH-*.md`; local Eino `compose/graph.go:679-695,880-884`, `compose/graph_run.go:362-379`, `compose/workflow.go:164-209`; this plan Scope.
  Acceptance criteria: `rg` finds no active Issue39 instruction to use `WithMaxRunSteps`, `AddEnd`, optional mailbox delivery, open context/model choice, or R1-R7-only acceptance; Markdown links resolve; docs state planning approval does not bypass gates; `git diff --check` passes.
  QA scenarios: happy - decision table and all Story contracts agree; failure - any contradictory lifecycle, Eino API or acceptance statement fails a scripted doc assertion. Use `rg`, Node link checker and `git diff --check`; Evidence `<attemptDir>/task-3-issue39-native-orchestration.md`.
  Commit: Y | `docs(issue39): freeze native orchestration contracts`

- [x] 4. Build the failing G0 Service/Eino integration proof
  Owner-approved revision (2026-09-26): the original test-only route is not truthful because Vivy lacks the Service-owned entrypoint. Following the owner's instruction to prioritize real user-visible requirements over an invisible technical route, Task 4 may add only a compileable, behaviorless Service-owned lifecycle stub returning typed `ErrNativeOrchestrationUnimplemented`; a real Eino Workflow test must invoke the real Service fixture and reach that sentinel. This is the intentional RED proof, not G0 evidence. Eino v0.9.13 has Workflow checkpoint/interrupt/resume; the missing capability is Vivy's Service-owned graph Run, prompt/checkpoint identity binding, and child activation. Do not build a fake-success lambda or infer GO/NO-GO.
  What to do / Must NOT do: PIN the existing primary-Service approval behavior on the unchanged code. Add the minimal typed Service boundary in a new `internal/runtime` file and a focused integration test constructing a real `compose.Workflow`; a node must call the real Service entrypoint. Invoke the Workflow so the named test fails at runtime with `ErrNativeOrchestrationUnimplemented`. The compileable stub is the only owner-approved pre-RED production scaffold; implement no graph lifecycle, child activation, recovery, scheduler, or schema here. Update ORCH-01 to reflect this approved proof order while keeping its G0/G1 product acceptance unchanged.
  Parallelization: Wave 1 | Blocked by: 3 | Blocks: 5
  References: `ORCH-01.md:15-30`; `internal/runtime/graph_conformance_test.go`; `internal/runtime/{service,engine,checkpoint,checkpointadapter}.go`; `internal/app/worker.go:89-229`; `internal/storage/contracts.go`; local Eino Workflow/checkpoint/interrupt APIs cited in draft findings.
  Acceptance criteria: PIN command `'/mnt/c/Program Files/Go/bin/go.exe' test ./internal/runtime -run '^TestServiceApprovalApproveFlow$' -count=1 -v` passes unchanged. Then `TestOrchestrationNative` is discovered and executed through real Eino Workflow and real Service fixture, and fails at runtime with the typed unimplemented sentinel (not a compile error and not `[no tests to run]`). Record that graph/child/effect IDs do not exist at this RED stage; Task 5 must create and record them. Never use `WithMaxRunSteps` for Workflow.
  QA scenarios: Go runtime surface invocation: `'/mnt/c/Program Files/Go/bin/go.exe' test ./internal/runtime -run '^TestOrchestrationNative$' -count=1 -v`. PASS for this proof task means output names `TestOrchestrationNative`, shows the expected typed sentinel from the invoked Service, and exits non-zero; FAIL means no named test ran, compilation failed, a standalone lambda bypassed Service, or the failure cause differs. Evidence `docs/logs/2026-09-26-issue39-native-proof/task-4-approved-order-red.md`.
  Commit: Y | `test(runtime): define Service-owned orchestration proof seam`

- [ ] 5. Establish minimum recovery foundation, then resolve integrated G0
  Current disposition: reopened under architecture D15; foundation and integration evidence are missing (BLOCKED). The historical attempt at `9f26f091f851bfdbe3b878d2cf8480320192ea53` called `ExecuteBrokerTool` twice in one process and observed a fixture counter of 2. It did not exercise stable operation identity, crash/restart, fresh stores or a real external effect. Its logs and later wording corrections remain unchanged; they do not prove Eino incompatibility. No product capability has been delivered.
  What to do / Must NOT do: Follow revised ORCH-01 in order: identity-aware RED tests, minimum Service-owned operation admission/atomic claim/result resolution, then minimum graph Run/prompt/checkpoint binding and governed idempotent one-shot node admission. Reuse Journal/storage transactions; justify a narrow storage seam and paired migrations only where required. This foundation does not wait for Task 6. Persist the effective invocation, preserve operation IDs across attempts, propagate completion-write errors and block unknown-effect replay. Eino v0.9.13 owns Workflow scheduling and opaque checkpoints; Service retains authority, budget, approval and cancellation. No general transaction platform, custom scheduler, second loop, public API or writable-child permission expansion.
  Parallelization: Wave 1 | Blocked by: 4 | Blocks: 6-14 until G0 proof accepted
  References: revised architecture D15; revised `ORCH-01.md`; `internal/runtime/{service,toolbroker,orchestration,checkpoint,checkpointadapter}.go`; `internal/app/worker.go`; existing storage transactions and pinned Eino Workflow/checkpoint APIs.
  Acceptance criteria: real Service/Eino parallel overlap/join, approval pause with sibling progress, parent cancellation, invalid engine/prompt rejection and the D15 process-crash matrix pass with named tests. Stable operation and node identities prevent duplicate admission; completed results are reused; claimed unresolved effects visibly block without replay. Distinct same-argument operation IDs both execute. Completion persistence failures propagate. Both SQL backends remain required for schema acceptance; deferred PostgreSQL is explicitly unverified. Run focused runtime/storage tests and required `just ci` gates.
  QA scenarios: use an observable durable effect outside the Journal transaction; terminate the execution process before claim, after claim/before invocation, after effect/before completion, and after completion/before graph checkpoint. Reopen stores in a new process with fresh Service/Engine instances. Assert the architecture D15 outcomes and capture IDs, sequences, effect counts and parent-visible state. Same-process double invocation is not recovery evidence. Write a new dated attempt log; preserve historical artifacts. Missing implementation/evidence is BLOCKED; demonstrated incompatibility/unsafe replay is NO-GO for that path. Neither releases Task 6; no fallback executor.
  Commit: Y | `feat(runtime): establish governed recovery boundary` then `test(runtime): prove native Eino orchestration` when evidence supports it

- [ ] 6. Persist explicit child modes, lineage and same-Session reauthorization — provisional implementation present; G0 BLOCKED
  Current disposition: paired SQLite/PostgreSQL migrations, binding/activation records, authority-ceiling persistence, Service admission, same-Session reauthorization and deletion fencing are implemented locally. SQLite conformance passes; PostgreSQL DSN-backed behavior, integrated budget/recovery acceptance, G0/G1 and release remain unverified.
  What to do / Must NOT do: TDD paired SQLite/PostgreSQL migrations and stores for explicit one-shot versus continuable mode, stable ChildSession binding, origin parent Session/Run, current authorizer Run, activation Run, immutable authority ceiling/digest, admission operation ID and payload digest. Existing agent tool and DAG defaults remain one-shot/non-addressable. Continuation requires a new active Run in the same origin parent Session and effective authority `current ∩ original ceiling`; parent Session deletion permanently fences activations. Preserve historical same-Session child rows as legacy reads only.
  Parallelization: Wave 2 | Blocked by: 5 GO | Blocks: 7
  References: revised architecture D4/D13/D14; `ORCH-02.md`; `internal/domain/{run,session}.go` after confirming actual paths; `internal/storage/contracts.go`; paired migration rules in `AGENTS.md`; existing Run `ParentID/RootID/Depth`; `Service.DeleteSession`.
  Acceptance criteria: backend conformance proves identical concurrent admission returns one binding/Run, changed payload conflicts before effects, continuation records separate origin/authorizer/activation IDs, authority widening fails, one-shot targeting fails, and deletion race leaves no binding/Session/Run/checkpoint. Fresh install, upgrade, reopen and both SQL backends pass.
  QA scenarios: happy - continuable child reauthorized by a later same-Session parent Run; failure - cross-Session, deleted Session, broader authority, reused operation ID with different payload, or one-shot follow-up is rejected durably. Use Go storage/runtime tests; Evidence `<attemptDir>/task-6-issue39-native-orchestration.{log,json}`.
  Commit: Y | `feat(storage): persist governed child sessions`

- [ ] 7. Move child activation onto Service and Eino — provisional implementation present; G1 BLOCKED
  Current disposition: native one-shot and continuable child activations use the Service/Eino runner, authority-bounded tools, approval path, cancellation, isolated workspace, and durable lifecycle events. The handwritten worker loop is no longer a selectable production path. Focused runtime/app checks pass; G1 parity, integrated restart/budget evidence and release remain blocked.
  What to do / Must NOT do: Add the minimal Service activation seam established by G0, route one-shot and continuable activations through the same Eino agent runner, policy, approval, checkpoint and broker path as parent Runs, and make worker manager thin lifecycle glue. Preserve read-only child tools, depth/concurrency/turn/budget behavior and old child RPC reads. Delete the handwritten `internal/worker` model/tool loop only after all callers, events and tests migrate; never leave selectable dual engines.
  Parallelization: Wave 2 | Blocked by: 6 | Blocks: 8,10
  References: `ORCH-03.md`; `internal/app/{worker,agenttool}.go`; `internal/worker/`; `internal/runtime/{service,engine,checkpoint}.go`; G0 adapter/trace; `internal/app/worker_test.go`.
  Acceptance criteria: red/green tests prove actual Eino path, clean context, parent model, immutable task mask hint, read-only tools, approval resume, parent/session cancellation, restart-safe wait and no personality access; old synchronous agent-tool result/error behavior remains compatible; `go test ./internal/app ./internal/runtime -run 'Child|Worker|Prompt|Approval|Checkpoint' -count=1` passes and no production caller reaches the old loop.
  QA scenarios: happy - one-shot and continuable activations complete through Eino; failure - denied tool, budget exhaustion, restart or parent cancellation produces one terminal durable outcome and no orphan. Use Go tests plus call-path trace; Evidence `<attemptDir>/task-7-issue39-native-orchestration.{log,json}`.
  Commit: Y | `feat(runtime): run child activations through Eino`

- [ ] 8. Implement durable direct parent-child mailbox and safe consumption — provisional implementation present; G0/G1 BLOCKED
  Current disposition: durable direct mailbox, recipient-scoped order/cursors/receipts, Service authorization, native activation safe-point consumption, `reply_parent`, and admission caps of 128 messages per recipient and 32 KiB per body are implemented. SQLite conformance and focused Go checks pass; PostgreSQL DSN-backed behavior and integrated restart/release acceptance remain unverified.
  What to do / Must NOT do: TDD host-owned mailbox persistence addressed to continuable ChildSession. Store stable message ID, sender principal derived by Host, sender-scoped operation ID/payload digest, recipient sequence, body/reference, state and receipt cursor. Initial authorization permits only origin-parent relation in both directions; peer/sibling messages are rejected. Define states `admitted`, `deliverable`, `consumed`, `failed`/`expired`; sending returns admission only. Integrate consumption at the G0-proven Service/Eino interrupt/resume or polling boundary; never mutate live input/history. Recheck authority at delivery and retain pending mail across interrupt/restart.
  Parallelization: Wave 3 | Blocked by: 7 | Blocks: 9,11
  References: revised architecture D8/D9; `ORCH-04.md:11-27`; durable storage/event seams; Eino checkpoint/interrupt APIs proven in todo 5; DSH comparator links in Issue #39 are behavioral only.
  Acceptance criteria: both backends prove stable FIFO per recipient sequence under concurrent senders, same operation/payload returns same ID, changed payload conflicts, unauthorized/cross-Session/one-shot target rejects, restart retains pending identity/order, active and idle continuable child consume at the proven boundary, cancellation after admission has defined state, and replies are separate messages. No exactly-once effect claim.
  QA scenarios: happy - parent sends while child active and after origin Run completion via same-Session reauthorization, child consumes once and replies; failure - crash before/after receipt, duplicate request, revoked authority, closed/deleted Session and backpressure all produce truthful durable states. Use Go storage/runtime fault-injection tests; Evidence `<attemptDir>/task-8-issue39-native-orchestration.{log,json}`.
  Commit: Y | `feat(runtime): add durable child messaging`

- [ ] 9. Deliver child follow-up, message and interruption through RPC and UI — provisional implementation present; G1/G3 BLOCKED
  Current disposition: child follow-up, interrupt, history, mailbox RPC and localized RunInspector controls are wired to host state; focused RPC/UI tests and typecheck pass. Real host/browser E2E and the remaining D9 close semantics are not complete.
  What to do / Must NOT do: Extend existing `child/start,get,list,wait,cancel` compatibly with explicit continuable creation, `child/followup`, `child/interrupt`, message send/list/receipt and scoped history using host-authoritative state. Keep follow-up as a new activation Run and messaging as a separate operation. Distinguish ChildSession ID, activation Run ID and current authorizer in RPC/UI; legacy cancel is not close. Add close only with the revised D9 outcome for pending mail and descendants. Hide child Sessions from the ordinary sidebar.
  Parallelization: Wave 4 | Blocked by: 8 | Blocks: 12
  References: revised `ORCH-04.md`; `internal/rpc/control.go:318-324`; `internal/app/worker.go:267-347`; `ui/src/lib/{api,store}.ts`; `ui/src/components/chat/RunInspector.tsx`; `ui/src/i18n/{en,zh}.ts`; `docs/architecture/VIVY-FACE-PACK.md`; `ui/AGENTS.md`.
  Acceptance criteria: Go RPC and Vitest tests cover legacy compatibility, operation replay/conflict, origin and same-Session authorization, durable wait after restart, message state, interrupt/cancel/close distinction, locale parity and no secret/checkpoint leakage; Playwright at `:3015` exercises real create/follow-up/send/interrupt/reload/restart flows with accessible controls and real backend responses.
  QA scenarios: happy - explicit continuable child survives activation completion and page/process restart; failure - unrelated parent, one-shot recipient, closed child, stale authorizer, denied approval and backpressure show typed localized outcomes. Use Go test, pnpm test and Playwright; Evidence `<attemptDir>/task-9-issue39-native-orchestration.{log,png,json}`.
  Commit: Y | `feat(child): add continuable controls and messaging`

- [ ] 10. Persist finite immutable workflow revisions — provisional implementation present; G2 BLOCKED
  Current disposition: pure descriptor validation and immutable revision storage with paired migrations are implemented. Current bounds are recorded in ORCH-05. SQLite conformance passes; PostgreSQL DSN-backed schema/data behavior remains unverified, so schema acceptance is blocked.
  What to do / Must NOT do: TDD pure Vivy descriptor/validator with no Eino imports and paired backend persistence. Validate schema version, stable unique keys, references, cycles, reachable/consumed nodes, finite node/depth/width/task/input/output limits, parent-authorized tool subset, explicit data mappings, canonical digest and operation replay/conflict before creating a workflow Run. Persist immutable descriptor bytes, origin/authority digest and graph Run lineage. Do not add loops, expression DSL, retries or dynamic mutation.
  Parallelization: Wave 3 | Blocked by: 7 | Blocks: 11
  References: revised `ORCH-05.md`; `internal/orchestration/` new pure package; `internal/storage/contracts.go`; paired migration rules; todo 6 admission conventions; Eino `WorkflowNode.AddInput` semantics.
  Acceptance criteria: table tests cover empty, duplicate/unknown/self/cycle, unreachable/unconsumed, exact limit boundaries, authority escalation, canonical field order, same/conflicting operation IDs, concurrent submission and deletion race; both backends fresh/upgrade/reopen pass and invalid proposals create no Run/revision/node.
  QA scenarios: happy - independent A/B and dependent join canonicalize to one digest; failure - cycle, oversize, unauthorized tool, mutated same ID, storage failure or deletion race returns structured path errors with zero execution. Use Go unit/conformance tests; Evidence `<attemptDir>/task-10-issue39-native-orchestration.{log,json}`.
  Commit: Y | `feat(orchestration): persist validated workflow revisions`

- [ ] 11. Execute governed workflows with native Eino dependencies — provisional implementation present; G0/G2 BLOCKED
  Current disposition: validated immutable revisions compile into native Eino Workflow; nodes use stable one-shot child Runs and explicit dependency output mappings. Focused runtime/app workflow tests pass. Integrated G0/G2 recovery, cross-backend and full resource-accounting acceptance remain blocked.
  What to do / Must NOT do: Compile only validated immutable revisions into Eino Workflow. Each node lambda idempotently admits a one-shot child activation through Service keyed by graph Run/revision digest/node key, waits durably, projects bounded output and returns it to Eino. Use order dependencies or explicit `AddInput` mappings and `End()` as verified; rely on Host finite validation and Service slots/budget/backpressure, never `WithMaxRunSteps`. Graph Run owns checkpoint and graph events; child Runs remain inspectable.
  Parallelization: Wave 4 | Blocked by: 8,10 | Blocks: 12
  References: revised `ORCH-06.md`; G0 implementation/trace; `internal/runtime/{service,checkpoint}.go`; new `workflow.go`; Eino v0.9.13 Workflow/graph/checkpoint APIs; todo 8 mailbox is independent of node output mapping.
  Acceptance criteria: real Service/Eino diamond starts independent nodes concurrently and join receives only mapped outputs; tests prove failed dependency, occupied slots/backpressure, approval pause with sibling progress, parent cancel, corrupt checkpoint, restart before/after node mapping and graph completion; node admission is idempotent; completed tool results are reused and unknown effects block automatic replay under D15; one terminal graph event and no post-terminal append.
  QA scenarios: happy - fanout/join recovers from process restart; failure - dependency error prevents descendant, capacity timeout is explicit, corrupt checkpoint fails closed, or injected crash reconciles the same child rather than creating another. Use Go runtime/app integration tests and event timeline assertions; Evidence `<attemptDir>/task-11-issue39-native-orchestration.{log,json}`.
  Commit: Y | `feat(runtime): execute governed Eino workflows`

- [ ] 12. Expose workflow proposal, execution and inspection through the real host and UI — provisional implementation present; G0/G1/G3 BLOCKED
  Current disposition: workflow proposal/start/get/list/cancel RPC, model-facing `workflow` tool, host-derived workflow/node/output projection, and localized RunInspector controls are wired. Local RPC/UI tests and typecheck pass; real host/browser E2E and product gates remain outstanding.
  What to do / Must NOT do: Add typed `workflow/propose,start,get,list,cancel` operations. Propose is pure and non-persisting; start revalidates inline descriptor/digest and authority, atomically persists revision/Run, and uses todo 11. Project deterministic node key, mapped child Run/status/result and Journal sequence. Extend existing store/RunInspector without a browser state machine; show only functional actions and link child detail. Do not present the optional #40 saved workflow product.
  Parallelization: Wave 5 | Blocked by: 9,11 | Blocks: 13
  References: revised `ORCH-07.md`; `internal/rpc/{control,control_test}.go`; App composition; `ui/src/lib/{api,store}.ts`; RunInspector; locales; Face contract and `ui/AGENTS.md`.
  Acceptance criteria: host tests prove invalid graph never persists, same start replays same Run, changed payload conflicts, stale/unauthorized parent rejects, restart status derives from durable facts and cancellation works during approval; UI tests cover loading/error/empty/terminal states and locale/accessibility; Playwright at `:3015` proves real propose, parallel join, reload, cancel and invalid graph.
  QA scenarios: happy - graph and node state survive browser/process reload; failure - cycle, stale digest, unauthorized parent, failed dependency and cancellation display exact host truth without mock/local reconstruction. Use Go test, pnpm test and Playwright; Evidence `<attemptDir>/task-12-issue39-native-orchestration.{log,png,json}`.
  Commit: Y | `feat(workflow): add governed host and UI surface`

- [ ] 13. Prove cross-boundary recovery, deletion and resource accounting — partial provisional evidence; acceptance BLOCKED
  Current disposition: D15 has a separate-process crash matrix; child/workflow lifecycle, deletion, mailbox receipts and SQLite storage conformance have focused coverage. The integrated R1-R14 matrix, PostgreSQL conformance, descendant budget/usage reconciliation across reauthorization/restart and full deletion-orphan audit are not complete.
  What to do / Must NOT do: Add integrated fault-injection/conformance coverage spanning both SQL backends, Service/Eino, mailbox, workflow and public host. Cover one-shot/continuable modes, same-Session reauthorization, full descendant budget reservation/actual usage/release, parent/child/graph cancellation, message pending/receipt, checkpoint/Journal races, external effect deduplication and parent Session deletion. Do not infer success from process memory or eventual output only.
  Parallelization: Wave 6 | Blocked by: 12 | Blocks: 14
  References: revised `ORCH-08.md`; architecture R1-R14/G0-G4; storage conformance suite; runtime/service tests; todo 5 trace identities; todos 6/8/10 schemas.
  Acceptance criteria: matrix records every R1-R14 row on SQLite and PostgreSQL with test name and actual execution; injected failures at checkpoint-before-Journal, Journal-before-projection and effect-before-terminal produce no duplicate admission/message receipt/effect; deletion leaves no child binding, Session, Run, revision, node map, checkpoint or mailbox orphan; budget totals do not reset or double count across activations/restart.
  QA scenarios: happy - full scenario recovers and totals reconcile; failure - any duplicate, orphan, unauthorized continuation, skipped backend, mismatched total or misleading pass blocks release. Use Go fault-injection and SQL inspection; Evidence `<attemptDir>/task-13-issue39-native-orchestration.{log,json}`.
  Commit: Y | `test(orchestration): prove durable recovery and accounting`

- [ ] 14. Run product gates and publish auditable delivery evidence — in progress; G0/G1/G4 BLOCKED
  Current disposition: focused local Go/UI evidence and provisional logs are being finalized. `just ci`, PostgreSQL DSN-backed conformance and real host/browser E2E are still required; the owner will run E2E after implementation. No acceptance or product release claim is made.
  What to do / Must NOT do: Run focused Go/UI suites, `just ci`, and real `just dev` browser acceptance at `http://127.0.0.1:3015`. Create `docs/logs/YYYY-MM-DD-issue39-eino-orchestration/{summary,verification,acceptance}.md`; update architecture/index/TODO/COMPLETE only from evidence. Acceptance maps R1-R14 and G0-G4, records exact environment and blocked/skipped checks, and includes rollback: disable public admission while retaining readable records and append-only migrations. Do not close Issue #39 or mark a Story complete from self-report.
  Parallelization: Wave 6 | Blocked by: 13 | Blocks: F1-F4
  References: revised `ORCH-08.md`; `AGENTS.md` Validation/Iteration logs/Backlog/Delivery; `justfile`; docs log examples; all prior evidence.
  Acceptance criteria: focused packages and `just ci` exit 0 with no skipped required backend; Playwright/manual automation proves delegation, follow-up, direct messaging, interrupt, reload/restart, graph proposal, join, cancel, two locales and accessibility against real host; delivery docs cite commands/results/IDs and unresolved items are placed in TODO or DEFER with restart criteria.
  QA scenarios: happy - all R1-R14/G0-G4 evidence and rollback review are complete; failure - any skipped gate, stale docs, browser mock, secret leakage, TODO placeholder or unsupported done claim blocks completion. Use `just ci`, `just dev`, Playwright, `rg` placeholder scan and `git diff --check`; Evidence `<attemptDir>/task-14-issue39-native-orchestration.{log,png,md}`.
  Commit: Y | `docs(issue39): record orchestration acceptance`

## Final verification wave

The earlier terminal-attempt reviews do not establish acceptance of the revised work. All checks below are pending; record fresh evidence after the implementation tasks and required gates. Independent reviews may use isolated read-only lanes.

- [ ] F1. Plan compliance: verify D15 foundation precedes integrated G0, downstream dependencies remain gated and all R1-R14 claims map to executed evidence. Missing code or tests must not be reported as complete.
- [ ] F2. Code quality and recovery safety: inspect the real Service/broker path, operation identity, atomic claim and completion failure handling. Require no unresolved high/medium findings. The direct-double-call probe is historical evidence, not a valid substitute for restart coverage.
- [ ] F3. Real-path acceptance: run the D15 process-crash matrix, native parallel/approval/cancellation scenarios and required product UI checks at `:3015` once those surfaces exist. Include both SQL backends before schema/release acceptance. Record actual commands and outcomes; skips are not passes.
- [ ] F4. Scope fidelity: preserve Service/Journal authority, Eino quarantine, read-only children and all explicit deferrals. No custom executor, unrestricted messaging, external delegation or general transaction platform. Separate architecture changes, implemented behavior and verified delivery.

## Commit strategy
- One focused commit per todo that changes tracked files; tests and implementation stay together. Task 5 may contain focused foundation and integrated-proof commits; record the gate verdict from actual evidence.
- Stage explicit paths only. Never stage `.omo/`, `.omc/`, runtime data, DSNs, credentials or unrelated changes.
- Migration commits include both dialect files with the same logical ID and all conformance tests; never amend a released migration.
- Do not commit, push or create a PR unless the executing user authorization includes it. Recommended handoff is `$start-work issue39-native-orchestration --make-pr`, which creates a task-owned worktree.

## Success criteria
- G0 records an evidence-backed GO and all subsequent Stories complete in the canonical index order; otherwise BLOCKED/NO-GO keeps dependent tasks pending with no fallback implementation.
- Existing synchronous one-shot agent behavior remains compatible; explicit continuable children retain durable Session identity and support same-origin-Session reauthorization without authority or budget expansion.
- Direct parent-child messages survive interruption/restart with stable IDs/order/states and proven safe consumption; follow-up remains a separate activation operation.
- Valid finite DAGs execute through Eino Workflow with bounded host admission, parallel dependencies, explicit joins, cancellation, checkpoint recovery, idempotent child admission, completed tool-result reuse and explicit blocking of unknown effects under D15.
- Journal/Run/storage are the only product truth; UI and RPC reconstruct status after restart and expose no process-memory fiction, raw checkpoint, secret or hidden authority.
- SQLite and PostgreSQL, focused tests, `just ci`, real `:3015` browser acceptance, English/Chinese accessibility and R1-R14/G0-G4 release evidence all pass with no required skip.
- No unrestricted swarm, workflow product, external delegation, context/model option, writable parallel child, custom scheduler, second runtime or second source of truth appears in the final diff.

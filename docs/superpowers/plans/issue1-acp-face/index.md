# Issue #1 Restricted ACP v1 Pilot Delivery Plan

**Design:** [Reconciled detailed design](../../specs/2026-10-07-acp-stdio-face-design.md).
**Executable baseline:** main at dd78fcf142f384d47ce5cfefb43738fdb9a7346d.
**Planning authorization:** The owner selected the restricted pilot and requested consolidation on ACP before retiring the old documentation branch. This is not G0 execution, SDK acceptance or G1 scheduling.
**Authority:** This index alone owns Story status and dependencies. The design owns proposed behavior. Historical specs/indexes redirect here; no parallel delivery board remains.

## Provenance and scope reconciliation

The original design and ACP-01 through ACP-05 came from docs/issue1-acp-face-design at [abed12f](https://github.com/ProjectViVy/agent-vivy/commit/abed12f7749cf3ec0258ec3dc911639f821fa55b), based on main 3c4ed66. The October draft at [45c466c](https://github.com/ProjectViVy/agent-vivy/commit/45c466c6aea1622a4465a835f6a06540870a13ef) independently added eight Stories on current main. This reconciliation preserves the former Story identity and the latter's source-grounded detail. Its merge commit retains both planning heads; it does not restore the older product tree.

| Stable Story | Superseded October task groups | Preserved outcome |
|---|---|---|
| ACP-01 | G0-01 + G0-02 | SDK evidence and reviewed contract |
| ACP-02 | G1-01 + G1-02 | Host seams, private launcher and build isolation |
| ACP-03 | G1-03 + G1-04 | Module, sessions, prompt and committed projection |
| ACP-04 | G1-05 plus original ACP-04 cleanup | Permission, questions and failure closure |
| ACP-05 | G1-06 | Hash-bound artifacts, omission and real-client acceptance |

The older single-launch-directory restriction is corrected to honor accepted session cwd; the former Options.ProjectRoot proposal is replaced by canonical roots returned by Core's session creation and session-aware context resolution. Preserve the one-runtime authority, isolated launch storage, effective rpc.client, default-product regressions and original A1-A5 outcomes. Do not reintroduce a root-provided ACP SDK dependency merely for the probe: its isolated nested module keeps the product closure unchanged.

The remaining review decisions are concrete, not hidden implementation discretion:

| Decision | Current draft / smallest next evidence | Execution boundary |
|---|---|---|
| SDK pin and public controls | Evaluate v0.0.4 against pinned schema, cancellation admission, real pipes and safe errors | A rejection report does not accept the dependency; no automatic fork/upgrade |
| Privacy presentation | Proposed optional TextPresentationHost, whole-line sanitization and original-byte digest checks | Owner reviews API and possible no-newline latency before ACP-02/03 |
| Packaging | Prefer existing selected build overlay; prove Web/TUI implementation omission and report assets separately | New Recipe.entrypoint, separate target and asset stripping require a distinct G0 choice, not implicit approval |
| Bounds and real client | Design limits are proposed, not measured; select a client/version supporting positive permission and form-answer paths | Freeze exact values/client with evidence; revise downstream signatures/tests before Ready |

## Global Constraints

- Module projectvivy/acp; T2; std/face@v1 through core/face-host@v1; effective rpc.client; 0..1 selected Face. Per-run metadata stays face=code.
- Local stdio, one connection/process, multiple owned sessions, one active prompt/session. Require mcpServers=[] and reject nonempty values before effects; no full ACP v1 conformance claim.
- No new runtime, Host/Port, Journal, policy store, listener, client filesystem/terminal delegation, session restoration, extra roots or MCP manager.
- Public plugin imports neither agent-vivy/internal nor cloudwego/eino. Reuse accepted ACP root types, conn and transport/stdio; never copy its codec or import SDK internals.
- Preserve default gateway, vivy tui, vivy run and codeface local-world behavior. All protocol-mode stdout is ACP NDJSON. Generic face helpers take streams and use the compiled Provider, not runtime discovery.
- Core owns canonical workspaces, file containment, review decisions and durable cancellation. Non-file ResourceLink URI/name is bounded user content with no adapter fetch. Safe local file references alone become context_paths.
- Allowlisted output, recognized-secret/private-root protection, ordered committed events and fail-closed cleanup are required. Exact new APIs and proposed bounds remain subject to ACP-01.
- Product tests use isolated state; never access data/vivy.db, data/demo or data/workspaces. Generate assembly/source-bound evidence through existing tools; all commits are human-attributed.

## Review focus and requirement traceability

| Risk | Required behavior | Owner |
|---|---|---|
| RF-1: URI encodings, native drives and symlinks; process A/session B | No cross-root read; no remote reference fetch; use durable session root | ACP-02, ACP-03 |
| RF-2: simultaneous session reservations, duplicate initialize, idle cancel | Bounded ownership; no cancellation of a future prompt | ACP-03 |
| RF-3: early subscription replay, terminal/cancel race, final partial line | Ordered output and one final response; verify original bytes | ACP-03 |
| RF-4: late/unknown review reply or ambiguous decision RPC | No stale authorization/answer; reconcile existing review state | ACP-04 |
| RF-5: blocked pipe, SDK panic, Control lost during teardown | Bounded shutdown, safe errors and honest incomplete-cleanup result | ACP-01, ACP-02, ACP-04, ACP-05 |

| Original requirement | Current coverage |
|---|---|
| A1: real client prompt | R1 protocol and R5 lifecycle; ACP-03/05 |
| A2: real IDs and one Runtime/Journal/HITL | R2 authority/workspace and R4 interactions; ACP-02/03/04 |
| A3: committed updates and clean stdout | R3 projection/privacy and R5 lifecycle; ACP-02/03/05 |
| A4: source/hash/grant and physical omission | R6 assembly; ACP-02/05 |
| A5: bounded fail-closed behavior | R1-R5; ACP-01/03/04/05 |

Epics remain E0 contract freeze (ACP-01), E1 governed local Face (ACP-02/03/04), and E2 generation acceptance (ACP-05). Completed plans are not implemented Epics.

## Story status and dependencies

| Story | Deliverable | Immediate predecessor and accepted output | Plan | Status | Current blocker |
|---|---|---|---|---|---|
| ACP-01 | Executable SDK verdict and accepted G0 contract | None | [ACP-01](ACP-01.md) | Planned | Execution not started; compatibility, design decisions and owner contract review pending |
| ACP-02 | Host seams, selected launcher and build isolation | ACP-01: exact SDK/API/limits, privacy and one packaging contract | [ACP-02](ACP-02.md) | Blocked | G0 not accepted; G1 not scheduled |
| ACP-03 | Owned sessions, prompt/cancel and ordered projection | ACP-02: Host interfaces and private selected-Face path | [ACP-03](ACP-03.md) | Blocked | Predecessor evidence absent |
| ACP-04 | Permission, questions and all-owned-run cleanup | ACP-03: prompt generations, event reducer and bounded connection | [ACP-04](ACP-04.md) | Blocked | Predecessor evidence absent |
| ACP-05 | Pinned artifacts and product acceptance | ACP-04: complete adapter, with ACP-02 launch/build contract | [ACP-05](ACP-05.md) | Blocked | Predecessor evidence, real client and required product gates absent |

Immediate edges: ACP-01 -> ACP-02 -> ACP-03 -> ACP-04 -> ACP-05.
Waves: {ACP-01}, {ACP-02}, {ACP-03}, {ACP-04}, {ACP-05}.
Serial execution preserves shared Host/lifecycle decisions and seals the source only after the adapter is complete. This is not a duration estimate or a mandate to delegate.

G0 completion and G1 scheduling are independent. ACP-01 is Done only after accepted compatibility evidence, owner contract review and required checks. A rejected SDK may finish its probe task while ACP-01 remains Blocked. Even with ACP-01 Done, ACP-02 stays Blocked until the owner explicitly schedules G1. Do not carry an earlier session's missing-toolchain observation as a permanent project fact; verify prerequisites when execution starts.

## File ownership and execution prerequisites

| Story | Owned change surface |
|---|---|
| ACP-01 | Isolated sdk/internal/testdata/acp-compat; docs/research/acp-sdk-compatibility.md; draft/canonical ACP contracts and conflicting normative sections after review |
| ACP-02 | sdk/port/face; internal/app/facehost; internal/rpc error/context seams; internal/faceprocess; codeface extraction; only the G0-selected CLI/pack route |
| ACP-03 | plugins/acp module, transport, control, input, session/prompt and projection |
| ACP-04 | plugins/acp interactions and shared lifecycle failure paths |
| ACP-05 | Recipe/source pins, conformance reproduction, artifact tests and real-client evidence |

Eino capability check: reuse the existing Eino v0.9.13 ADK/Runner path behind internal/runtime; ACP adds no model/tool loop. Candidate github.com/eino-contrib/acp supplies protocol/connection/stdio only. Design section 11 records inspected APIs; ACP-01 executes their compatibility check before any product dependency adoption.

The inspected justfile selects powershell.exe and Go 1.26.4; full gates also need Node, pnpm and just. Bootstrap sibling sources with just ensure-laputa and UI with just ui-build where required. Run just ci for kernel/public-contract/product changes. Root go test ./... does not cover nested plugin/probe modules; run their commands in their directories. Use supported Go/CGO for race checks.

Source changes can invalidate attestations before ACP-05. In the owning Story use go run ./sdk/internal/cmd/source-hash DIRECTORY CURRENT_DECLARED_SHA256, update only actual declared references, regenerate with go generate ./internal/generated/assembly when needed, and execute TestCheckedInProviderConformanceMatchesExecutedSuites. Never guess hashes/pass flags. ACP-05 adds final ACP-specific evidence.

Each Story records actual commands/results in docs/logs/<actual-delivery-date>-acp-<story-id>/{summary,verification,acceptance}.md. Commit explicit changed paths. This consolidation leaves canonical adoption and required implementation gates pending; no product CI pass is claimed.

## Acceptance coverage

Scenario IDs use AC; ACP IDs name Stories only.

| Scenario | Required evidence owner |
|---|---|
| AC-01 real-client prompt | ACP-03; final ACP-05 |
| AC-02 rejected input and no-fetch resource references | ACP-03 |
| AC-03 process A/session B tools, context and instructions | ACP-02/03; final ACP-05 |
| AC-04 ordered replay and integrity | ACP-03 |
| AC-05 split text sanitization and original digest | ACP-02/03 |
| AC-06 isolated cancel before run ID | ACP-03 |
| AC-07 SDK prompt/cancel admission | ACP-01/03 |
| AC-08 permission approve/deny/invalid/late | ACP-04; final ACP-05 |
| AC-09 successful form answer plus cancel/unavailable/expiry | ACP-04; real-client positive proof ACP-05 |
| AC-10 EOF, broken/blocked pipe, signal and flood | ACP-01/02/04; final ACP-05 |
| AC-11 safe wire errors and diagnostics | ACP-01/02/03; final ACP-05 |
| AC-12 selected/omitted implementation and honest asset inventory | ACP-02/05 |

## Handoff and branch retirement

The next executable candidate is ACP-01 only when its execution is requested. Use native sequential execution unless later authorized work benefits from delegation. If G0 changes an API, SDK pin, scope or build route, edit the one design and all affected downstream plans before Ready.

The old documentation branch may be retired after the published reconciliation commit is verified to retain abed12f in its ancestry and the old branch has not advanced. Original text remains in Git history; current paths resolve to this package/design. Removing the branch does not close Issue #1, accept G0 or schedule G1.

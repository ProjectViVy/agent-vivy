# Issue 81 product gaps: E2/E3/E4 closeout plan package

**Revision:** PG-1, 2026-10-18. **Scope owner:** [#81](https://github.com/ProjectViVy/agent-vivy/issues/81), E2/E3/E4 and their A4 corrections.
**Baseline:** latest remote `main` observed at planning time: `7b42b2920f72de6f37de8b9e4ff4bf6a110a762f`.
**Branch:** `plan/issue81-product-gaps`. All changes in this package stay in this directory.
**Authorization:** planning and a branch/backup only. No product implementation, paid API, merge, deployment, issue mutation or closure.

This is a delta to existing report, INOFY and Toolkit plans, not a new product design. Read [source evidence](source-evidence.md), [architecture and decisions](boundaries.md), then the selected Story. Source presence, historical tests, current tests and acceptance are distinct. No product test was executed to produce these documents.

## Requirements and Epic outcomes

| Requirement | Existing owner | Outcome | Plans |
| --- | --- | --- | --- |
| PG-R | E2 / #5; A1/C12 | Generate and configure the first report naturally from empty UI, preserving existing provenance | R1 |
| PG-WI | E3 / #9/#21; A6/C08 | Canonical node.inputs survive author/save/publish/run | W1 |
| PG-WL | E3 / #9/#21; C08 | Active, idle and history launch obey actual parent admission; unavailable launch is understandable | W2 |
| PG-TG | E4 / #20; A2/C09 | One reproducible native browser entry prepares the binary and isolates ports | T0 |
| PG-TM | E4 / #20; TK-13 | Reconcile B1–B14/A01–A28, implement only missing verification, record unaccepted gates | T1–T5 |
| PG-D | A4 in this scope | Prepare evidence-backed status corrections for one centralized documentation landing | D1 |

Phase one remains BML in the separate E1 lane. DIVA acceptance is separate. #79/Laputa #10 and #80 are read-only external dependencies: do not repair, reassign or reschedule them. #6/#7/#60–63, #67 publication, new workflow nodes/triggers/discovery/wait-resume, Notebook #39 reopening, full AgenticModel migration and #57/#66 runtime activation are excluded.

## Authoritative Story status and immediate dependency table

This table alone owns PG status and dependency data. Story files reference it; the canonical Toolkit acceptance table still owns A/B requirement semantics. Planned is a complete plan, not implementation authorization or a test pass. An executor may mark Ready only after explicit implementation authorization, refreshed source/AGENTS, required venue/skills and accepted predecessor evidence.

| Story | Epic | Immediate predecessors / supplied output | State | Plan | Acceptance owner |
| --- | --- | --- | --- | --- | --- |
| R1 | E2 | None; consumes existing report operation | Planned | [R1](R1.md) | Natural first-use and omission UI |
| W1 | E3 | None; consumes current INOFY schema/executor | Planned | [W1](W1.md) | Binding roundtrip plus consumed values |
| W2 | E3 | None; consumes existing active-parent admission | Planned | [W2](W2.md) | Safe launch in each navigation state |
| T0 | E4 small plumbing | None; consumes existing browser specs | Planned | [T0](T0.md) | Native prepared isolated browser gate |
| T1 | E4 matrix | None; consumes existing TK-13 tests and matrix | Planned | [T1](T1.md) | Source/publication/restart/storage faults |
| T2 | E4 matrix | None; consumes existing admission/fence tests | Planned | [T2](T2.md) | Admission/concurrency/revocation |
| T3 | E4 matrix | None; consumes current Hook/MCP adapters | Planned | [T3](T3.md) | Exact consent and failure behavior |
| T4 | E4 matrix | T0: reproducible binary and browser entry | Planned | [T4](T4.md) | UI failure/recovery and omission |
| T5 | E4 integration | T1,T2,T3,T4: reviewed matrix evidence | Planned | [T5](T5.md) | Complete matrix disposition and final gates |
| D1 | A4 | R1,W1,W2,T5: accepted results or exact blocked/residual records | Planned | [D1](D1.md) | In-scope status correction handoff |

~~~mermaid
flowchart TD
  T0 --> T4
  T1 --> T5
  T2 --> T5
  T3 --> T5
  T4 --> T5
  R1 --> D1
  W1 --> D1
  W2 --> D1
  T5 --> D1
~~~

Derived waves: {R1,W1,W2,T0,T1,T2,T3} → {T4} → {T5} → {D1}.
Waves show dependency eligibility, not parallel-session authorization. This session creates no children. Recommended future execution is sequential native execution, with independent review at each Story exit and final combined review; any future reviewer/session needs its own explicit assignment.

## Shared files are sequencing constraints, not fabricated dependencies

- W1/W2 share workflow unit-test discovery, browser fixture/config and locale catalog: land W1 then W2 in one authorized lane, or integrate sequentially from isolated lanes.
- T1/T2/T3 share manager/runtime/App tests; T1 owns publication faults, T2 admission/fences, T3 Hook/MCP behavior. Serialize overlapping fixture and App changes.
- T0 owns `justfile` and Toolkit Playwright config; T4 consumes that interface without another launcher.
- R1 notebook and T0/T4 Toolkit suites all default to Vite :3015. Execute separately or use supported isolated overrides, never attach to an unrelated server.
- Public status files, historical plan indices, `docs/TODO.md`, `docs/DEFER.MD` and GitHub trackers have one future centralized writer. D1 supplies a patch specification only; this branch edits none of them.

## Execution protocol inherited by each Story

Read repository and scoped instructions again; use the authorized isolated lane and pinned private Laputa sibling per AGENTS. Do not install unknown software or access production data. Existing tests must be mapped before adding coverage. Proposed test names are labeled in each plan; setup/discovery failure is not RED. For test-only missing coverage, demonstrate sensitivity by a disposable negative control or failing injected scenario, then restore the source before GREEN. Never leave a weakened assertion or mutated production source in the deliverable.

Use deterministic scripted inference/local MCP fixtures. Clear external credentials in disposable subprocess environments; do not dump environment values. Browser evidence uses a real backend at split Vite origin. Unit mocks prove contracts, not product wiring. Model quality is a separate bounded, credential-authorized acceptance row and is not run by this task.

For future product changes: focused checks → required `just ci` → relevant real-path/omission smoke → independent review → human-attributed focused commit and iteration log. Live PostgreSQL must use an authorized disposable DSN; a skip is blocked parity, never PASS. Failures owned by #80 remain on that lane; record exact candidate/control evidence without upgrades, skipped tests or automatic waivers.

## Decisions and exact blockers

The selected W2 scope is clear unavailability outside a valid active parent. A new independent idle launch path is outside this package; its absence does not block W1/R1/T0. If the owner later requires idle execution, only that new Story blocks on a reviewed admission/authority contract.

Required environment/skills and new authorization block implementation readiness, not delivery of this plan. Independent review is planned, not performed by a second agent. The [planning verification record](verification.md) states actual checks and infrastructure limitations. No Story is marked Done, no original issue is considered closeable solely because this package exists.

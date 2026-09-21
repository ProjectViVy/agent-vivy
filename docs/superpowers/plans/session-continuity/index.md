# Issue #51 — Session Continuity Story Index

Revision SC-P4, 2026-09-21. Planning package complete; product implementation has not started. Execution and method selection still require user approval.

## Authority and scope

- [SC-D3 product specification](../../specs/2026-09-21-session-continuity-design.md) is unchanged, including its full frontend design.
- [Shared contracts and constraints](../2026-09-21-session-continuity.md) own DTOs, transaction rules, bounds and review focus.
- This index alone owns Story status and dependency data. Individual plans contain executable checklists, not competing status tables.
- Inspected branch: `docs/issue51-architecture`, revision `e2bfc333f857ddf36f828e3bb17e06e7c6a3e8cf`; underlying code `a2d2b5912e8dd93668aae787b198441d8100a969`.
- This turn authorizes planning only. No product edits, merge, push, issue updates or deployment. No individual Story is marked Ready from a draft interface alone.

## Requirements and Epics

| ID | Observable requirement | Epic / evidence owner |
| --- | --- | --- |
| R0 | One runtime/Journal/workspace authority; bounded typed contracts and verified integration prerequisites | Core-0: T0–T1 |
| R1 | Model search/read/trace and human inspection with redaction, provenance and stable paging | Core-1: T2–T3; GUI T7–T8 |
| R2 | Explicit untrusted copied references; excerpt attachment does not grant further reading | Core-2: T4–T8 |
| R3 | Additive deliverable sets, shell-created files, safe verified preview/download | Core-3: T9–T11 |
| R4 | Retry, cancellation, deletion, replay, compaction and concurrent mutation remain honest | T2,T4,T6,T9,T10; integrated T12 |
| R5 | Complete retained GUI, draft/queue, accessible states, en/zh and responsive interaction | T7,T8,T11,T12 |
| R6 | Real query/reference/code/present loop and physical Lite artifact omission | Core-4: T12 |

## Story dependency and status ledger

| Story / plan | Epic | Requirements | Immediate predecessors and required output | Outcome | Status | Evidence / blocker |
| --- | --- | --- | --- | --- | --- | --- |
| [T0](T0.md) | Core-0 | R0 | None | Reconcile baseline and validation prerequisites | Planned | Read-only audit can be selected after plan review; no implementation baseline gate passed. |
| [T1](T1.md) | Core-0 | R0,R1,R2,R3 | T0: baseline revision, migration owner and environment evidence | Domain, JSON schemas, limits and task authority | Planned | No accepted implementation evidence yet; predecessor acceptance and execution authorization required. |
| [T2](T2.md) | Core-1 | R1,R4 | T1: domain DTOs, schemas, effective limits and authority helper | Stable, bounded history storage on both backends | Blocked | Migration owner from #45 is not in this baseline; T0 must resolve its integration and allocation before DDL. |
| [T3](T3.md) | Core-1 | R1 | T2: stable HistoryQueryStore cuts and backend parity | Governed history projection, tools and inspection RPC | Planned | No accepted implementation evidence yet; predecessor acceptance and execution authorization required. |
| [T4](T4.md) | Core-2 | R2,R4 | T2: stable HistoryQueryStore cuts and backend parity | Atomic task admission and operation receipts | Blocked | Migration owner from #45 is not in this baseline; T0 must resolve its integration and allocation before DDL. |
| [T5](T5.md) | Core-2 | R2 | T3: sanitized HistoryService, model tools and inspection RPC; T4: atomic ContinuityStore admission and idempotent receipts | Explicit reference preview and attachment | Planned | No accepted implementation evidence yet; predecessor acceptance and execution authorization required. |
| [T6](T6.md) | Core-2 | R2,R4 | T5: reference preview/digest and attach integration | Feed projection and historical lifecycle | Planned | No accepted implementation evidence yet; predecessor acceptance and execution authorization required. |
| [T7](T7.md) | Core-2 | R1,R2,R5 | T5: reference preview/digest and attach integration | History picker, composer and queue | Planned | No accepted implementation evidence yet; predecessor acceptance and execution authorization required. |
| [T8](T8.md) | Core-2 | R1,R2,R4,R5 | T6: reference feed and lifecycle projection; T7: typed submission, picker and queue integration | Reference details and model history timeline | Planned | No accepted implementation evidence yet; predecessor acceptance and execution authorization required. |
| [T9](T9.md) | Core-3 | R3,R4 | T4: atomic ContinuityStore admission and idempotent receipts | Safe file presentation and verified transfers | Planned | No accepted implementation evidence yet; predecessor acceptance and execution authorization required. |
| [T10](T10.md) | Core-3 | R1,R3,R4 | T9: secure presentation and connection-bound verified bytes; T3: sanitized HistoryService, model tools and inspection RPC | Presentation tool, RPC and durable discovery | Planned | No accepted implementation evidence yet; predecessor acceptance and execution authorization required. |
| [T11](T11.md) | Core-3 | R3,R5 | T10: present tool/RPC, durable list and artifact history filter; T8: reference/get adapter, timeline row and detail surface | Additive delivery groups and GUI actions | Planned | No accepted implementation evidence yet; predecessor acceptance and execution authorization required. |
| [T12](T12.md) | Core-4 | R1,R2,R3,R4,R5,R6 | T11: delivery cards, summary and verified download | End-to-end coding acceptance and Lite | Planned | No accepted implementation evidence yet; predecessor acceptance and execution authorization required. |

## Topological waves

1. T0
2. T1
3. T2
4. T3, T4
5. T5, T9
6. T6, T7, T10
7. T8
8. T11
9. T12

Waves are logical ordering, not permission for parallel writes. Recommended execution is native sequential T0 through T12 because transaction, registry, RPC and frontend owners overlap. The DAG allows T9 after T4; T10 additionally needs T3's actual history implementation to enable artifact filtering. T11 waits for T8's common timeline integration as well as T10's delivery API. T12 requires T11; T6/T8 are already transitive ancestors whose evidence must still be checked.

## Shared-file scheduling

| Shared boundary | Story owners | Scheduling rule |
| --- | --- | --- |
| Migration manifest and message/session/journal writers | T2 then T4 | One storage lane; T0 allocates from the actual migration owner. No inline DDL or competing migration numbers. |
| Tool registry / app composition / RPC dispatch | T3,T5,T8,T10 | Sequence shared-file edits; each service's adapters remain cohesive. |
| Service startup, context and run metadata | T4,T5,T6 | Transaction output is accepted before feed/lifecycle integration. |
| api/store and SDK Face declarations | T7,T8,T11 | One frontend lane; update host and SDK shapes together and run compatibility tests. |
| run-rows, ChatView and locales | T8 then T11 | T8 owns reference row merge; T11 extends it for immutable delivery sets. |
| Recipes / generation evidence | T12 | Reuse generated assembly and existing module IDs; do not change default selection to simulate Lite. |

## Detailed-design corrections from code inspection

1. T7's typed submission affects existing SDK Face declarations: `sdk/ui/src/module.ts` declares positional startTurn/enqueueMessage, and `ui/src/lib/ui-sdk-face-compat.test.ts` enforces exact parity. Include these consumers and fixtures in T7/T11.
2. T10 enables the delivery artifact filter in HistoryService, so it consumes T3 in addition to T9. Record the dependency instead of relying on sequential luck.
3. T8 explicitly owns reference/get RPC wiring; T6 owns its runtime lifecycle projection.
4. T3 owns one canonical selection digest encoder reused by T5; a second encoder would make preview/admission disagree.
5. Remove redundant T1 -> T9 and T6/T8 -> T12 scheduling edges; their outputs are already prerequisites through the graph. All acceptance obligations remain.
6. T0 cannot integrate an unmerged storage PR without authorization. Until centralized migrations are available, downstream DDL remains blocked.

No product semantics, schema implementation, dependency or existing project rule changes are made by these corrections.

## Acceptance and handoff

Read the spec, shared ledger, this index and the selected Story together. Give the executor accepted predecessor revisions/results, permitted files and stop conditions. No worker should infer authority from the presence of a link or a checked planning box.

Planning checks: 13 unique Story IDs and plan files; complete original checklist preservation; known endpoints; no self-edges/cycles; reduced dependency graph; relative links; balanced fences; requirement coverage and shared-file ownership. See [planning verification](../../../logs/2026-09-21-session-continuity-story-plans/verification.md).

Implementation evidence remains unverified: SQLite/PostgreSQL conformance, model-call traces, real split-GUI actions/download bytes, Lite inspect output and `just ci`. Complete the final T12 gate before claiming the Epic is delivered. Postgres or browser skips are not passes. Record implementation evidence once in the existing planned implementation log; update this index from observed results, not worker claims.

On contract change, invalidate affected Story readiness and review downstream consumers before continuing. Push/PR and #46/#49/#51 updates require separate authorization.

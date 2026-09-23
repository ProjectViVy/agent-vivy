# Issue #51 — Session Continuity Story Index

Revision SC-P9, 2026-09-22. Planning package baseline refreshed; T0, T1 and T2 are accepted at source/review level, with Go/PostgreSQL/just verification still pending. T3 implementation is present on the isolated branch but remains pending acceptance; external push/PR/issue updates remain separately unauthorized.

## Authority and scope

- [SC-D4 product specification](../../specs/2026-09-21-session-continuity-design.md) is unchanged in product semantics, including its full frontend design.
- [Shared contracts and constraints](../2026-09-21-session-continuity.md) own DTOs, transaction rules, bounds and review focus.
- This index alone owns Story status and dependency data. Individual plans contain executable checklists, not competing status tables.
- Current code baseline: `a8d361b0244a1c40be513622bbdaebb5c9d40014` (main). PR #45 is merged through `680ef78`; its centralization implementation is `760ac1c`. `internal/storage/migrations` owns paired embedded SQLite/PostgreSQL migrations via `manifest.go`/`runner.go`; highest is `023_workspace_path.sql` in both dialects and `024` is the next available logical number, not a reservation.
- T0/T1 execution is authorized in this isolated branch. No external push, PR, issue update or deployment is authorized. No Story is accepted from a draft interface alone; acceptance requires reviewed implementation evidence.

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
| [T0](T0.md) | Core-0 | R0 | None | Reconcile baseline and validation prerequisites | Accepted | SC-P5/SC-D4 documentation baseline refresh and its evidence log were reviewed and accepted; `just ci` remains unverified (exit 127: `just` not found). |
| [T1](T1.md) | Core-0 | R0,R1,R2,R3 | T0: baseline revision, migration owner and environment evidence | Domain, JSON schemas, limits and task authority | Accepted | Commits `a594912` + `9a1c5c1` passed task review and scoped fix re-review; focused Go tests remain unverified because `go` is unavailable. |
| [T2](T2.md) | Core-1 | R1,R4 | T1: domain DTOs, schemas, effective limits and authority helper | Stable, bounded history storage on both backends | Accepted | Commits `76cdf60`, `145d5d9`, `0ed1a3b` and `cf6aebc` passed implementation/fix/scoped acceptance review; R1–R6 storage findings are closed at source level. Go/Postgres/just/CI remain unverified because the toolchain/services are unavailable. |
| [T3](T3.md) | Core-1 | R1 | T2: stable HistoryQueryStore cuts and backend parity | Governed history projection, tools and inspection RPC | Accepted | Implementation `3519c73`, `bcc6aaa`, `60fb0a4`, `a4d8996a`; review fixes `2fa25e3b`; user-directed v2-only `model.completed` contract `50dc5ed3` + `a320f55f`; blocker fixes `d3f4878b`; stale-fixture burn-down `29908309`; conformance digest refresh `41e67904`. Focused Go tests PASS (runtime/rpc/app/tools/storage/sdk/faces-headless; Postgres SKIP — unavailable, not a pass); UI vitest 14/14 PASS. Independent review artifact with two scoped re-review rounds, final verdict "acceptable for T3 acceptance": `docs/logs/2026-09-21-session-continuity-implementation/t3-review.md`. `just ci` PASS on `41e67904` — all stages green (fmt-check, ui-ci vitest 392/392, vet, `go test -timeout 20m ./...`, headless-compile, plugin-ci); recorded in `verification.md`. |
| [T4](T4.md) | Core-2 | R2,R4 | T2: stable HistoryQueryStore cuts and backend parity | Atomic task admission and operation receipts | Accepted | Implementation `e337ac5d`: `storage.ContinuityStore` on both backends + migration `025_continuity_receipts`, sorted-ID session locking, revision-pinned source expectations, receipt-first dedup replaying original `RunID`/events on identical retry and `ErrConflict` on changed payload, `ErrRunClosed`/`ErrCommitUncertain`/`ErrSourceChanged` contracts, runtime `RunOptions.Continuity` admission with `AdmissionWorkspaceAllocator` rollback and `run.started` accepted-scope payload. Focused `TestContinuityAtomic`/`TestContinuityRetry` PASS (sqlite+runtime; Postgres SKIP — unavailable, not a pass); full `internal/storage`+`internal/runtime` suites PASS; `just ci` PASS on `e337ac5d`, recorded in `verification.md`. |
| [T5](T5.md) | Core-2 | R2 | T3: sanitized HistoryService, model tools and inspection RPC; T4: atomic ContinuityStore admission and idempotent receipts | Explicit reference preview and attachment | Accepted | Implementation `5019b4d6`: `runtime.ReferenceService` (Preview/Attach/AttachForAdmission/Lookup), `reference_context` tool + `tools.WithToolCallID`, `reference/preview` operator RPC, `turn/start` continuity decode, admission snapshots committed as `context.reference_attached` startup events via `CommitContinuityRun`, model attaches via `CommitContinuityOperation` keyed on stable tool_call id, digest-conflict `ErrSourceChanged`, reference-id destination-owned reads. Focused `TestReference*` PASS (runtime/tools/rpc; Postgres SKIP — unavailable, not a pass); `just ci` PASS on `5019b4d6`, recorded in `verification.md`. |
| [T6](T6.md) | Core-2 | R2,R4 | T5: reference preview/digest and attach integration | Feed projection and historical lifecycle | Accepted | Implementation `92f6b8f9`: `contexthost.ResolvedSnapshotSource` (memory-only) + `ProjectReferenceContext` emit explicit `[imported reference: …]` user parts with `included`/`elided_budget`/`unavailable` markers inside the shared feed budget; startup-region scan injects committed snapshots onto their owning turn while same-run model attaches ride the tool result; `reference/get` returns `ReferenceView` (copy + live source_status + feed_status); compaction keeps a `Folded references:` manifest + `ReferenceIDs` in the `cmp_` event; fork copies prefix startup refs under `frok_<session>` inside `CommitSessionFork`; rewind hides owning turns; prompt rule marks imported blocks as quoted data. Focused `TestReferenceContext|TestReferenceLifecycle` PASS; `just ci` PASS on `92f6b8f9`, recorded in `verification.md`. |
| [T7](T7.md) | Core-2 | R1,R2,R5 | T5: reference preview/digest and attach integration | History picker, composer and queue | Accepted | Implementation `f7c4f212`: typed `FaceTurnSubmission`/`FaceTurnContinuity` owned by `@vivy/ui-sdk` and threaded atomically through `startTurn`/`startRun`/`enqueueMessage` + compat test; host flattens continuity into wire `request_id`/`references`/`history_scope`; `HistoryPicker` (Dialog ≥768px / Sheet <768px, metadata-only session list, submit-only search, stale-generation discard, preview-gated Attach, opt-in further-reading) + `ContextReferenceChip`; queue deep-copies submissions, retains a stale head, resumes on removal; draft state is session-bound and clears only after successful send. Focused vitest 47/47, sdk/ui vitest 28/28, `just ci` PASS on `f7c4f212` (404/404 ui tests), real-browser smoke at :3015 with journal-verified `context.reference_attached` + `continuity_receipts`, recorded in `verification.md`. |
| [T8](T8.md) | Core-2 | R1,R2,R4,R5 | T6: reference feed and lifecycle projection; T7: typed submission, picker and queue integration | Reference details and model history timeline | Accepted | Implementation `15096e78`: `foldContinuityRows` in `run-rows.ts` projects committed `context.reference_attached` events into `context_reference` rows deduped by `run_id:seq` + reference id (replay/live union, no second card from tool results); `ReferenceDetail` renders the saved snapshot (excerpt, origin badge, collapsed ids/digest details) separately from live `reference/get` `source_status`/`feed_status`, with `history/read` source jump that never switches the active session and Escape-collapse focus return; `reference/get` RPC dispatch hardened with session-existence check (`CodeNotFound`); history tools humanized under a `History` presentation variant; RunInspector run tab lists attached references; SDK Face contract gained `FaceContextReference`/`FaceReferenceView`/`FaceHistoryReadRequest` + `referenceGet`/`historyRead` + `referenceViews`/`loadReferenceView` (compat test mirrored atomically). Focused vitest 64/64, full ui 416/416, sdk/ui vitest + tsc PASS, `go test ./internal/rpc` PASS, `just ci` PASS on `15096e78`, real-browser smoke at :3015 verified collapsed card → expand → live badges + View source without session switch (3 items cut early marked untested), recorded in `verification.md`. |
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
| Migration manifest and message/session/journal writers | T2 then T4 | One storage lane under `internal/storage/migrations`; T0 records highest `023_workspace_path.sql` and next available logical number `024` without reserving it. No inline DDL or competing migration numbers. |
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
6. PR #45 is merged through `680ef78` (centralization implementation `760ac1c`), so T2 and T4 are not blocked by migration ownership. They remain non-Ready until their actual predecessor implementation and verification evidence is accepted.

No product semantics, schema implementation, dependency or existing project rule changes are made by these corrections.

## Acceptance and handoff

Read the spec, shared ledger, this index and the selected Story together. Give the executor accepted predecessor revisions/results, permitted files and stop conditions. No worker should infer authority from the presence of a link or a checked planning box.

Planning checks: 13 unique Story IDs and plan files; complete original checklist preservation; known endpoints; no self-edges/cycles; reduced dependency graph; relative links; balanced fences; requirement coverage and shared-file ownership. See [planning verification](../../../logs/2026-09-21-session-continuity-story-plans/verification.md).

Implementation evidence remains unverified: SQLite/PostgreSQL conformance, model-call traces, real split-GUI actions/download bytes, Lite inspect output and `just ci`. Complete the final T12 gate before claiming the Epic is delivered. Postgres or browser skips are not passes. Record implementation evidence once in the existing planned implementation log; update this index from observed results, not worker claims.

On contract change, invalidate affected Story readiness and review downstream consumers before continuing. Push/PR and #46/#49/#51 updates require separate authorization.

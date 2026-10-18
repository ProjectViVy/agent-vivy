# E1-BML-03: Durable replay boundaries Implementation Plan

> **For agentic workers:** After separate execution authorization, use `superpowers:executing-plans` task by task. No child sessions are authorized here. Steps use checkbox syntax.

**Goal:** Prove stable-event dedupe and durable same-record tombstone suppression without advertising unsupported public mutation guarantees.
**Architecture:** Extend existing provider and library tests with close/reopen joins. Keep EventID delivery, record-level CAS/replay and public Add as separate contracts; use the existing BML store, not a receipt database.
**Tech Stack:** Go, existing observer Port, BML SQLite/FTS5.
**Spec:** [Architecture](architecture.md), A1/A4–A6/A8–A9; BML-R4/R6. Dependencies/status: [index](index.md).

## Global constraints

- Exact stable event replay means same `RunID`, sequence, type, timestamp and payload. Do not claim payload-conflict validation that main does not implement.
- `memory_add` lacks an operation ID; identical calls create distinct records by existing design.
- Public long-term removal and library history tombstones are not interchangeable capabilities. No new history-delete tool/UI is planned.
- No schema upgrade, bound-operation implementation or cherry-pick from #79. Its tests are reference-only unless a later candidate containing them is selected.
- Module fixtures are serial; library fixtures use independent stores and close every handle.

## Files and interfaces

- Create (proposed): `internal/modules/memory/replay_persistence_test.go`, package `memory_test`, provider delivery across reopen.
- Create (proposed): `bml/replay_persistence_test.go`, package `bml`, low-level suppression across reopen.
- Reuse existing helpers `completedEvent`, `testConfig`, `storeRecord`, `tombstoneRecord` and existing Store test patterns. Do not modify shared helper files without demonstrated need.
- Read only: `internal/modules/memory/{provider,service}.go`, `bml/{home,store,search}.go`, existing replay/tombstone tests.

Consumes existing `memory.NewProvider() *memory.Provider`,
`ObserveRunWithReceipt(context.Context, observer.RunEvent) (observer.DeliveryReceipt, error)`,
`Query(context.Context, contextsource.Request) (contextsource.Page, error)`,
`bml.Open(context.Context, string, string) (*bml.Store, error)`,
`Store.Put`, `PutTombstone`, `Metadata`, `Get`, `SearchVisible`, `Close`.
Use the already inspected signatures and original tests for revision parameters.
Produces `TestBMLCloseoutStableEventReopenReplay` and
`TestBMLCloseoutTombstoneReplayAcrossReopen`; no new production interface.

## Review focus

1. A test changes EventID accidentally: replay the original immutable event object and compare receipts exactly.
2. Fresh root or in-memory cache stands in for durability: close owner/store, reopen same path, and perform no reseeding.
3. Raw target still exists but test calls that resurrection: assert public visibility/search suppression separately from raw retention.
4. Target rewrite re-adds an FTS row: query after rewrite and another reopen; verify no duplicate target row.
5. Candidate bound-operation receipts leak into main claims: current commands must compile against main APIs only.

## Ordered tasks

- [ ] **1. Run and reuse the existing guarantees.**

  ```sh
  go test ./internal/modules/memory -run '^TestProviderObserveCompletedIngestsHistoryOnce$' -count=1 -v
  (cd bml && go test ./... -run '^(TestTombstoneSuppressionSurvivesTargetRewrite|TestImportRecords|TestHomeAddsIdenticalContentAtSameTime)$' -count=1 -v)
  ```

  Expected: required tests execute and pass. The last test is positive evidence that repeated Add is distinct creation, not a defect to repair. If new equivalent durable coverage already landed, reuse it rather than duplicate it.

- [ ] **2. Add the provider durable dedupe test.** Open one disposable cfg, construct one `completedEvent(t,"bml-closeout-run",7,"eventprobe synthetic completion")`, and call `ObserveRunWithReceipt`. Require nil error, `DeliveryCompleted`, matching EventID and nonempty receipt ID. Query `"eventprobe"`, require exactly one history candidate; retain its native ID, version, content and evidence metadata. Public List must not present it as a long-term entry. Explicitly `memory.Close`, reopen the same cfg, create a new Provider and replay the exact original event. Require receipt equality and exactly the same single candidate ID/version/content. Close/reopen once more and repeat the read; there is no second seed call or new EventID.

- [ ] **3. Add the library durable tombstone test.** Start with `bml.Open(ctx,dir,"workspace-1")` and existing `storeRecord`/`tombstoneRecord` helpers. Put one deterministic long-term target using current metadata's store revision and insert-only expected record revision. Persist a tombstone with `PutTombstone(ctx,tomb,storeRevision,targetID,targetRevision)`. Close and reopen the same store before replay attempts. Retain the tombstone and target IDs and use current `Metadata` revisions for each intentional operation.

- [ ] **4. Exercise both supported same-record paths.** First re-submit the original target through insert-only `Put` (nil expected record revision): require `StoreError.Code==ErrRecordRevisionConflict` and unchanged metadata. Then deliberately rewrite that existing raw target using its actual record revision and current store revision, following `TestTombstoneSuppressionSurvivesTargetRewrite`. This low-level API can accept a target rewrite; the durable superseding tombstone must still suppress it. Close/reopen again. Require `SearchVisible` excludes the target, the tombstone still names it, and there is exactly one target row plus the tombstone in raw Store listing. Raw `Get` may still return the retained target and is not a public visible-record API. No test may claim physical erasure from that result.

- [ ] **5. Record the exact guarantee matrix.**

  | Surface | Accepted guarantee | Limit to preserve |
  |---|---|---|
  | Same committed EventID via observer | Same receipt and one history candidate across reopen | No new payload-equality or all-event exactly-once claim |
  | Same existing record insert-only replay | Explicit revision conflict without another row | Not a durable operation lookup/receipt |
  | Explicit low-level target rewrite after tombstone | Tombstone keeps target out of visible FTS after reopen | Raw retained target may change; public history deletion not implied |
  | Public `memory_add` | Existing unique creation behavior | No idempotency key; uncertain outcome needs inspection, not blind retry |
  | #79 bound mutation tests | Optional future evidence after pin selection/revalidation | Absent on this main; not executed or imported here |

- [ ] **6. Run focused and module checks, obtain independent review, and commit.**

  ```sh
  go test ./internal/modules/memory -run '^TestBMLCloseoutStableEventReopenReplay$' -count=1 -v
  (cd bml && go test ./... -run '^TestBMLCloseoutTombstoneReplayAcrossReopen$' -count=1 -v)
  go test -race ./internal/modules/memory -run '^TestBMLCloseoutStableEventReopenReplay$' -count=1
  (cd bml && go test -race ./... -run '^TestBMLCloseoutTombstoneReplayAcrossReopen$' -count=1)
  (cd bml && go test ./... -count=1)
  ```

  Expected: each required named test executes and passes; no races or skips. Reviewer inspects both evidence surfaces independently and confirms no API overclaim. Stage only the two new tests and owned execution evidence; focused human-attributed commit, no AI trailer. Final integrated `just ci` remains required by the index.

## Stop conditions, acceptance and rollback

Stop if replay produces another candidate, changes receipt/version, loses the
tombstone across reopen, permits visible resurrection, requires a missing bound
API, or tests touch another root. Block 03; retain exact input, returned outcome
and candidate SHA, without changing production code or scheduling #79 repair.
An unavailable race toolchain blocks that verification result explicitly.

Accept only the matrix actually demonstrated by the named checks and independent
review. A public history-delete capability remains unsupported; it is not a
hidden blocker for long-term BML E1. Rollback closes only owned handles and removes
the disposable stores after failure evidence is preserved.

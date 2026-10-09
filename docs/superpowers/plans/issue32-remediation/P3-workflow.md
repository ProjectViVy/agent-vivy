# P3 Workflow Product Contract Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Resolve W1–W8 so workflow drafts use safe CAS, start retries remain idempotent, pagination is complete, and the UI displays backend lifecycle truth.

**Architecture:** Keep the existing `definitions.Repository` adapter and `startINOFYWorkflow` admission path. Put token/cursor contracts in Core Storage, capture immutable start requests in the existing face bridge, and render native and engine status separately in the source UI Module.

**Tech Stack:** Go; SQLite and PostgreSQL via existing storage drivers; INOFY `v0.0.0-20260930141905-71e2c9bbe47d`; Eino `v0.9.13`; existing `github.com/google/uuid v1.6.0`; React, TypeScript, Vitest, and the existing UI SDK.

**Spec:** [P3 workflow product contract](../../specs/2026-10-09-issue32-remediation-design.md#p3-workflow-product-contract).

## Global Constraints

- Preserve the single `Service.Run` / Journal / Policy path and existing authority boundaries.
- Add no orchestration engine, credential store, dependency, or schema migration.
- Draft creation sends `create: true`; editing sends a nonempty ETag. Empty ETags never authorize overwrite.
- Every successful draft write receives a fresh opaque UUID ETag; timestamps remain real timestamps.
- Preserve historical publications and legacy draft ETags until the next successful edit.
- Capture operation ID, session, parent Run, workflow, revision or draft ETag, and input once per start intent.
- Never resave or mint a new operation ID while retrying an unchanged prepared start request.
- Run order is `(created_at DESC, workflow_run_id ASC)`; reject timestamp-only legacy Run cursors with a refresh error.
- Keep `status` and `engine_status` separate; `completed` is terminal and `supports_resume: false` stays false.
- Edit `plugins/vivy-workflow/ui/vivy-workflow/src/**`; never hand-edit `ui/src/generated/**`.
- Use disposable test databases; do not access `data/vivy.db`, `data/demo/`, or `data/workspaces/`.

## Review Focus

- Two same-author browser creators see absence, then save sequentially: one succeeds and the other conflicts without replacing its artifact (P3.1).
- Legacy tokens and backwards clocks: the first repaired edit invalidates the old token without fabricating timestamps (P3.1).
- A start reply is lost while the current parent changes: retry retains the original complete request and does not save again (P3.2).
- Equal timestamps, colon-containing workflow IDs, and malformed cursor tails: continuation is complete and invalid cursors produce a refreshable parameter error (P3.3).
- Recovery events and failed Load more requests: recovery remains visible, resume stays unsupported, and successfully loaded rows remain available (P3.4).

## Boundaries and preflight

Read the linked spec and the root/UI `AGENTS.md` before execution. Tasks are sequential because CAS and start lifecycle share the editor, and cursor/status work shares list queries. Use a clean isolated implementation worktree according to the root lane rule; this document authorizes no implementation during the design turn.

The existing Module is `vivy/workflow-ui`, T2, source `repo:plugins/vivy-workflow`; Provider `vivy.workflow-ui.sidebar` uses supported `std/ui-extension@v1` (`0..n`, ordered), with `PresentationHost` as sole Host Consumer. It requests no additional Grant. Its generation lifecycle, route `/workflows`, and default Recipe selection remain unchanged. Browser requests still use backend session, policy, draft ownership, and admission checks.

**Eino capability check:** Inspect pinned Eino `compose.Graph.Compile` and INOFY `Compile`, `Program.Meta`, and `definitions.Service.SaveDraft` / `Publish` before code. Existing `validateINOFYDefinition` already uses INOFY compilation, and `launchINOFYWorkflow` uses its durable RunStore adapter. These APIs own graph compilation/execution; they do not own Vivy's author-scoped SQL CAS, operation request retention, or list presentation. Reuse them unchanged. Custom work is limited to those host seams; if upstream supplies equivalent token/cursor handling later, replace the host helper without changing storage ownership or published history.

**Source evidence:** Rehash the workflow Module after each task touching it with `go run ./sdk/internal/cmd/source-hash plugins/vivy-workflow <current-declared-sha256>`. Set the emitted value in both `plugins/vivy-workflow/vivy-module.yaml` and `plugins/vivy-workflow/module.go`; confirm a second hash run emits the same value. Keep the source-bound conformance bundle update in the program's final integration/evidence task, after all selected-source edits.

---

### Task P3.1: Safe draft creation and opaque CAS tokens — W3, W7

**Files:**
- Modify: `internal/storage/workflow_definitions.go`; `internal/storage/sqlite/workflow_definitions.go`; `internal/storage/postgres/workflow_definitions.go`.
- Modify: `internal/runtime/inofy_definitions.go`; `internal/runtime/workflow_product.go`; `internal/rpc/inofy_product.go`.
- Modify: `plugins/vivy-workflow/ui/vivy-workflow/src/client.ts`; `plugins/vivy-workflow/ui/vivy-workflow/src/editor/EditorPane.tsx`; `plugins/vivy-workflow/i18n/catalog.json`; `ui/vitest.config.ts`.
- Test: `internal/storage/conformance/workflow_definitions.go`; `internal/storage/sqlite/workflow_definitions_test.go`; `internal/storage/postgres/workflow_definitions_test.go`.
- Test: `internal/runtime/workflow_product_test.go`; `internal/rpc/inofy_product_test.go`; `plugins/vivy-workflow/ui/vivy-workflow/src/face-bridge.test.ts`; `plugins/vivy-workflow/ui/vivy-workflow/src/workflow-page.test.tsx`.
- Modify source evidence: `plugins/vivy-workflow/vivy-module.yaml`; `plugins/vivy-workflow/module.go`.

**Interfaces:**
- Keep `UpdateWorkflowDraftCAS(ctx context.Context, sessionID string, in storage.WorkflowDraftUpdate) (storage.WorkflowDraft, error)` and `WorkflowDefinitionETagAbsent = "\x00absent"`.
- Replace the internal timestamp-derived helper with `storage.NewWorkflowDefinitionETag() string`, returning `uuid.NewString()`; remove unused `WorkflowDefinitionETag(workflowID string, seq int64)` and `itoaBase10`.
- Keep `INOFYSaveDraft(ctx context.Context, sessionID domain.SessionID, workflowID, expectedETag string, artifactJSON json.RawMessage) (definitions.Draft, error)`; accept only the absent sentinel or a nonempty token.
- RPC `inofy.saveDraft`: `{workflow, artifact, create?: boolean, etag?: string | null, session_id?}`; exactly one valid create/edit mode.
- Keep `WorkflowClient.saveDraft(id: string, artifact: Artifact, etag: string | null): Promise<DraftView>`; `null` serializes as `{create: true}`, a nonempty string as `{etag}`.

- [x] **P3.1.1 Write the token regression cases in shared conformance.**

Add `ETagRotatesWithRepeatedAndBackwardsTime` under `AssertWorkflowDefinitionContract`. For a new unique workflow, create at `1000`, then save three different artifacts at `1000`, `1000`, and `999`; use the immediately preceding ETag each time. Assert every ETag is nonempty and distinct, `CreatedAt == 1000`, and final `UpdatedAt == 999`. Reusing any prior token must satisfy `errors.Is(err, storage.ErrWorkflowDefinitionConflict)` and must leave the final artifact unchanged.

Pin these assertions after constructing the four writes in the test:

```go
if d0.ETag == d1.ETag || d1.ETag == d2.ETag || d2.ETag == d3.ETag { t.Fatal("etag repeated") }
if d3.CreatedAt != 1000 || d3.UpdatedAt != 999 { t.Fatalf("timestamps changed: %+v", d3) }
if !errors.Is(staleErr, storage.ErrWorkflowDefinitionConflict) { t.Fatalf("stale write: %v", staleErr) }
```

- [x] **P3.1.2 Write legacy-token and PostgreSQL insert-race tests.**

Add `TestWorkflowDraftLegacyETagRotates` in each driver's existing definitions test file. Seed one valid draft with `etag = "wf-legacy:0123456789abcdef:1001"`, `created_at = 1000`, `updated_at = 1000`; edit with that token at `1000`, reopen, and assert the new token differs and the legacy token conflicts without changing bytes. Use existing test-only database access; add no production test hook.

Add PostgreSQL `TestWorkflowDraftConcurrentAbsentInsertConflict`: create the author, hold `LOCK TABLE workflow_definition_drafts IN SHARE MODE` in a blocking test transaction, start two absent-token saves, wait with a bounded test deadline until both INSERT transactions have ungranted `RowExclusiveLock` requests for that table in `pg_locks`, then release the lock. Assert exactly one success, one `ErrWorkflowDefinitionConflict`, and one persisted winning artifact. If the barrier deadline expires, fail the test and roll back the blocker.

- [x] **P3.1.3 Write product/RPC create-intent regressions.**

Add `TestWorkflowProductSaveRequiresExplicitCAS`: empty expected token fails; absent creates; duplicate absent conflicts; current token edits; foreign author cannot overwrite. Add `TestINOFYSaveDraftExplicitCreateOrETag`: omitted/null/empty edit ETags and `create: true` with a nonempty ETag return `InvalidParams`; `{create: true}` creates once and the same author's second create returns `CodeConflict` with `data.code == "revision_conflict"`. Keep existing foreign-author assertions. Update `TestINOFYProductRPCSurface` to create explicitly.

- [ ] **P3.1.4 Write browser tests and select authoritative Module tests.**

UI portion pending: the repository's `vivy-plugin` skill requires `oil-frontend`,
which is unavailable in this checkout; the user was asked whether to provide it
or authorize `testing-vivy-ui` as the substitute. Do not edit generated staging.

In `ui/vitest.config.ts`, include both workflow source test globs and exclude `src/generated/ui/vivy-workflow/src/**/*.test.*`. Add `saveDraft sends explicit create intent for a null etag` and `saveDraft rejects an empty edit etag` in `face-bridge.test.ts`; inspect the actual RPC request. Add `two missing-draft editors do not overwrite the first creator` to `workflow-page.test.tsx`: both load misses, first create succeeds, second conflicts, server artifact remains first creator's value. Add `opening a revision loads the current draft etag before editing`: revision content remains the edit source, next save uses the loaded draft token. A missing own draft uses explicit creation; a foreign existing draft conflicts.

- [ ] **P3.1.5 Run the new tests and confirm behavioral failures.** Backend regressions were run red; browser tests remain pending with P3.1.4.

```bash
go test ./internal/storage/sqlite -run 'TestWorkflow(DraftLegacyETagRotates|DefinitionContract/ETagRotatesWithRepeatedAndBackwardsTime)' -count=1
go test ./internal/runtime -run TestWorkflowProductSaveRequiresExplicitCAS -count=1
go test ./internal/rpc -run TestINOFYSaveDraftExplicitCreateOrETag -count=1
pnpm -C ui exec vitest run ../plugins/vivy-workflow/ui/vivy-workflow/src/face-bridge.test.ts ../plugins/vivy-workflow/ui/vivy-workflow/src/workflow-page.test.tsx
go test ./internal/storage/postgres -run 'TestWorkflowDraft(LegacyETagRotates|ConcurrentAbsentInsertConflict)' -count=1 -v
```

Expected: token-repeat, unconditional same-author overwrite, revision editor token, and primary-key race assertions fail. PostgreSQL requires an existing disposable `VIVY_POSTGRES_TEST_DSN`; `SKIP` does not count as red or green evidence. These are future execution commands, not checks run during this design turn.

- [x] **P3.1.6 Implement fresh ETags in both existing storage writes.**

Have both `UpdateWorkflowDraftCAS` implementations call `storage.NewWorkflowDefinitionETag()` only after validating the CAS mode. Preserve existing `CreatedAt`, assign `UpdatedAt = in.Now`, and use the old stored ETag in the UPDATE predicate. Tighten `ValidateWorkflowDraftUpdate(in WorkflowDraftUpdate) error` to reject empty expected tokens. Translate the PostgreSQL INSERT's typed `*pgconn.PgError` code `23505` for the draft primary key to `ErrWorkflowDefinitionConflict`; preserve all other errors and their cause chains.

- [x] **P3.1.7a Implement explicit create/edit mode across runtime and RPC.**

Runtime rejects an empty expected ETag; the storage adapter accepts only the
absent sentinel or a concrete edit token. RPC decoding maps `{create:true}` to
create-only, requires a nonempty ETag for edit, and rejects conflicting modes.

- [ ] **P3.1.7b Implement current-token revision editing and explicit browser create intent.**

`WorkflowClient.saveDraft` serializes `null` as explicit creation and accepts only nonempty edit ETags; it rejects `""` locally. In `openRevision(id: string, revision: number)`, load the immutable revision for edit content and the current own draft for its ETag; use `null` only when the own-draft lookup returns the existing missing-draft error. Preserve revision content and dirty state. Keep `editor.forkNote` and set English to `Editing published revision {revision}; save updates your current draft.` and Chinese to `正在编辑已发布版本 {revision}；保存将更新当前草稿。`.

- [ ] **P3.1.8 Verify, rehash the Module, and commit the deliverable.**

Run P3.1.5 again, then `go test ./internal/storage/sqlite ./internal/storage/postgres ./internal/runtime ./internal/rpc -count=1`. Rehash the Module as specified above **before** `pnpm -C ui typecheck` invokes automatic staging. Expected: all targeted regressions pass, PostgreSQL tests actually execute, and typecheck passes. Stage only P3.1 files.

```bash
git commit -m "fix: require explicit workflow draft CAS and rotate opaque etags"
```

**Execution ruling:** Backend steps P3.1.6 and P3.1.7a proceeded before the
browser tests because they are independently verifiable and the required
`oil-frontend` skill is unavailable. The risk if this sequencing is wrong is a
later editor contract adjustment; no UI source changes were made.

---

### Task P3.2: Admission parity and stable start intent — W1, W2

**Files:**
- Modify: `internal/runtime/inofy_admission.go`; `internal/rpc/inofy_product.go`.
- Modify: `plugins/vivy-workflow/ui/vivy-workflow/src/face-bridge.ts`; `plugins/vivy-workflow/ui/vivy-workflow/src/client.ts`; `plugins/vivy-workflow/ui/vivy-workflow/src/studio/transport.ts`.
- Modify: `plugins/vivy-workflow/ui/vivy-workflow/src/editor/EditorPane.tsx`; `plugins/vivy-workflow/ui/vivy-workflow/src/panes/WorkflowsPane.tsx`; `plugins/vivy-workflow/i18n/catalog.json`.
- Test: `internal/runtime/inofy_admission_test.go`; `internal/runtime/workflow_product_test.go`; `internal/rpc/inofy_product_test.go`.
- Test: `plugins/vivy-workflow/ui/vivy-workflow/src/face-bridge.test.ts`; create `plugins/vivy-workflow/ui/vivy-workflow/src/editor/editor-start.test.tsx`; create `plugins/vivy-workflow/ui/vivy-workflow/src/panes/workflow-start.test.tsx`.
- Modify source evidence: `plugins/vivy-workflow/vivy-module.yaml`; `plugins/vivy-workflow/module.go`.

**Interfaces:**
- Consume P3.1 `WorkflowClient.saveDraft` and its returned `DraftView.etag`.
- Reuse `domain.CanonicalToolNames(names []string) ([]string, error)` without modifying it.
- Create exported `WorkflowRunIntent = {workflow: string; revision?: number; draft_etag?: string; input?: JsonValue}` in `studio/transport.ts`.
- Create `WorkflowStartRequest = Readonly<WorkflowRunIntent & {session_id: string; parent_run_id: string; operation_id: string}>` in that file.
- Add `HostBridge.prepareStartRun(intent: WorkflowRunIntent): WorkflowStartRequest`; implement it in `FaceBridge`.
- Add `WorkflowClient.prepareStartRun(intent: WorkflowRunIntent): WorkflowStartRequest`; change `startRun(request: WorkflowStartRequest): Promise<{run_id: string}>` and the matching `StudioTransport` signature.
- Existing server `INOFYStartRun(ctx context.Context, sessionID domain.SessionID, params INOFYStartRunParams) (WorkflowStartResult, error)` and its durable operation dedup remain unchanged.

- [x] **P3.2.1 Write admission and publication regressions.**

Add `TestINOFYAdmissionRejectsDuplicateToolNames`: `taskDefinition()` with `tool_names = ["read_file", "read_file"]` fails `validateINOFYDefinition`; repeat with `["read_file", " read_file "]` and with `[""]`. Extend `TestINOFYToolSchemaExposesOnlyHostChildTasks` so literal duplicates fail JSON Schema validation. Add `TestWorkflowProductPublishRejectsDuplicateTools`: save a syntactically valid draft, validate returns a host-admission diagnostic, publish fails, and no revision is allocated. Add `TestWorkflowProductHistoricalDuplicateToolsRejectStart`: seed one immutable malformed publication using existing storage APIs; getRevision remains readable; starting it returns `ErrINOFYInvalidDefinition` and persists no workflow admission or child effects.

- [ ] **P3.2.2 Write bridge full-request retention tests.**

Replace the per-call-minting bridge test with `prepareStartRun captures one immutable request`. Assert operation ID is a UUID, `session_id == "sess-1"`, `parent_run_id == "run-parent"`, and exact workflow/revision/input are captured. Change the store's current parent to `"run-other"` and mutate the original input object after preparation; sending the same request twice must make identical RPC calls with the original parent and input. Add `startRun rejects a request with missing or oversized operation id`: no truncation, minting, or RPC call. Preparing another intent yields a distinct UUID. Keep ordinary session-bound non-start calls unchanged.

- [ ] **P3.2.3 Write save-then-start and published-start UI fault tests.**

Use happy-dom and existing host/test patterns; assert observable requests and rendered actions. In `editor-start.test.tsx`, add `lost start reply retries the saved snapshot without resaving`: save responds with ETag `"e2"`, first start rejects a simulated transport-loss error, retry returns `"run-child"`; save count is one, both start bodies are deeply equal, and the dirty badge is cleared immediately after save. Add `save confirmation survives validate and publish failure`: each action records `"e2"` before the follow-up fails, and the next explicit save sends `"e2"`. Add `editing after a failed start creates a new intent`: altered artifact is saved once with the confirmed token and its start has a different operation ID. In `workflow-start.test.tsx`, add the equivalent lost-reply test for published revision `2`, preserving exact input and parent.

For each lost-reply case, pin the captured wire calls:

```ts
expect(saveCalls).toHaveLength(1);
expect(startCalls).toHaveLength(2);
expect(startCalls[1]).toEqual(startCalls[0]);
expect(startCalls[1]).toMatchObject({ session_id: 'sess-1', parent_run_id: 'run-parent' });
expect(container.textContent).not.toContain('plugin.vivy/workflow-ui.editor.dirty');
```

- [ ] **P3.2.4 Write safe stale-source and session-binding tests.**

Add `TestWorkflowProductDraftRetryAfterSourceChangeConflicts`: admit draft ETag `e1`, edit it to `e2`, retry the exact first operation with `e1`, and assert `ErrWorkflowDefinitionConflict` plus one admission total. Add UI `stale-source retry retains ambiguous intent`: repeated retry sends the original body, receives `revision_conflict`, performs no save, and never prepares a replacement operation. Add `TestINOFYStartRunRejectsMismatchedCapturedSession`: a live peer bound to session A receives a prepared request claiming B; return `InvalidParams` before admission.

```go
if !errors.Is(retryErr, storage.ErrWorkflowDefinitionConflict) { t.Fatalf("retry: %v", retryErr) }
if len(revisions) != 1 || revisions[0].RunID != first.Run.ID { t.Fatalf("duplicate admission: %+v", revisions) }
```

- [ ] **P3.2.5 Run new tests to establish red evidence.**

```bash
go test ./internal/runtime -run 'Test(INOFYAdmissionRejectsDuplicateToolNames|INOFYToolSchemaExposesOnlyHostChildTasks|WorkflowProduct(PublishRejectsDuplicateTools|HistoricalDuplicateToolsRejectStart|DraftRetryAfterSourceChangeConflicts))' -count=1
go test ./internal/rpc -run TestINOFYStartRunRejectsMismatchedCapturedSession -count=1
pnpm -C ui exec vitest run ../plugins/vivy-workflow/ui/vivy-workflow/src/face-bridge.test.ts ../plugins/vivy-workflow/ui/vivy-workflow/src/editor/editor-start.test.tsx ../plugins/vivy-workflow/ui/vivy-workflow/src/panes/workflow-start.test.tsx
```

Expected: duplicate authoring, fresh per-call operation keys, delayed save confirmation, and retry/resave assertions fail. The stale-source runtime case may already pass; retain it as a compatibility guard.

- [x] **P3.2.6 Implement host tool-name parity.**

Add `uniqueItems: true` to `childConfigSchema.tool_names`. In `validateINOFYDefinition(ctx context.Context, raw json.RawMessage, allowedTools []string) (inofyAdmission, error)`, call `domain.CanonicalToolNames(config.ToolNames)` and reject errors with the node ID and preserved cause. Validate the existing membership ceiling without rewriting the definition, list, or digest. Existing validation, publication, proposal, and start paths already share this function; do not add a second validator.

- [ ] **P3.2.7 Implement prepared start requests at the existing bridge seam.**

Server-side captured-session validation is implemented and covered by
`TestINOFYStartRunRejectsMismatchedCapturedSession`; bridge preparation and
verbatim forwarding remain pending the UI implementation gate.

`FaceBridge.prepareStartRun` validates a live session and parent, copies the complete intent including a snapshot of input, and creates one UUID. `FaceBridge.call` forwards `inofy.startRun` prepared bodies verbatim after validating nonempty session, parent, operation ID of at most `128` bytes, and one source selector; it never overrides fields from current store state. Keep session injection for other calls. In `inofyStartRun`, require the captured `session_id` to match the server-resolved session; use the existing `InvalidParams` envelope for mismatch. Update the existing HostBridge test doubles and client/transport types together.

- [ ] **P3.2.8 Implement immediate save confirmation and retained UI intent.**

Apply `saved.etag` and `saved.artifact` immediately after each successful save, before validate/publish/start. Store a `WorkflowStartRequest | null` pending intent separately from draft state and keep it on transport error or source conflict. The Retry start action submits that request directly. Successful start clears it; a user change to workflow/source/input/artifact clears it so the next Start prepares a fresh intent. Changes to the host's current parent alone do not mutate an existing intent. Apply the same pending-intent lifecycle to published-start rows in `WorkflowsPane`; pending state is keyed by workflow and revision. Add `plugin.vivy/workflow-ui.editor.retryStart` (`Retry start` / `重试启动`) and `plugin.vivy/workflow-ui.editor.startSourceConflict` (`The saved draft changed. Check Runs before starting a new operation.` / `已保存草稿发生变化。请先检查运行列表，再启动新操作。`) using the existing catalog format.

- [ ] **P3.2.9 Verify, rehash the Module, and commit.**

Run P3.2.5 again; run `go test ./internal/runtime ./internal/rpc -count=1`. Rehash as specified in preflight before `pnpm -C ui typecheck`. Expected: all regressions pass, one server Run for unchanged retry, and no capability/digest regression. Stage only P3.2 files.

```bash
git commit -m "fix: align workflow admission and preserve start retry intent"
```

**Execution ruling:** `INOFYValidateDraft` reports duplicate tool names as
`schema_mismatch` after `uniqueItems` is added to the trusted node config
schema, before the host-specific canonical-name check runs. The regression
asserts that the diagnostic identifies `tool_names` and the duplicate items;
the error still fails closed at validation and publication. Requiring a
`host_admission` code here would contradict the earlier schema gate. Cost if
wrong: clients may depend on a different validation diagnostic code.

---

### Task P3.3: Driver-parity keyset cursors — W4, W8

**Files:**
- Create: `internal/storage/workflow_cursors.go`; `internal/storage/workflow_cursors_test.go`.
- Modify: `internal/storage/workflow_definitions.go`; `internal/storage/sqlite/workflow_definitions.go`; `internal/storage/postgres/workflow_definitions.go`; `internal/storage/conformance/workflow_definitions.go`.
- Modify: `internal/runtime/inofy_definitions.go`; `internal/rpc/inofy_product.go`.
- Test: existing `internal/storage/{sqlite,postgres}/workflow_definitions_test.go`; `internal/rpc/inofy_product_test.go`.

**Interfaces:**
- Add `storage.ErrWorkflowCursorInvalid` with a wrapped error message that tells the caller to refresh the list.
- Add `type WorkflowRunCursor struct { CreatedAt int64; RunID string }`.
- Add `EncodeWorkflowRunCursor(createdAt int64, runID string) string` and `DecodeWorkflowRunCursor(cursor string) (WorkflowRunCursor, error)`.
- Add `ParseWorkflowDefinitionCursor(cursor string) (workflowID string, revision uint64, err error)`.
- Keep existing `ListWorkflowDefinitionRuns` and `ListWorkflowDefinitions` signatures and page shapes; they consume these shared helpers.
- Add test-only `admitDefinitionRunFixture(t *testing.T, slot Slot, sessionID domain.SessionID, parentID, runID domain.RunID, createdAt int64) storage.WorkflowStepStore` in conformance. Use `WorkflowStepFixture` as the existing schema-2 admission model; bind source `"wf-pages"`, revision `1`, and preserve matching Run/revision/event timestamps.

- [x] **P3.3.1 Write cursor codec tests with exact accepted/rejected forms.**

Add `TestWorkflowRunCursorRoundTrip`: encode `(1000, "run:b")`, decode it, and assert exactly those values. Add `TestWorkflowRunCursorRejectsInvalid`: reject legacy `"1000"`, wrong version `"v2.e30"`, invalid base64, invalid JSON, missing/empty ID, nonpositive timestamp, unknown JSON fields, and tokens over `2048` bytes; all errors match `ErrWorkflowCursorInvalid`. Define the format as `"v1." + base64.RawURLEncoding(JSON {"created_at":1000,"run_id":"run:b"})`.

Add `TestWorkflowDefinitionCursorFinalColon`: `"team:flow:2"` yields `("team:flow", 2)`, and empty cursor yields `("", 0)` for initial paging. Reject `":2"`, `"flow:"`, `"flow:0"`, `"flow:-1"`, `"flow:+1"`, nondecimal tails, and revisions greater than signed SQL `int64`; preserve existing IDs of up to `256` bytes, including colons.

```go
if decoded.CreatedAt != 1000 || decoded.RunID != "run:b" { t.Fatalf("cursor: %+v", decoded) }
if workflowID != "team:flow" || revision != 2 { t.Fatalf("definition cursor: %q %d", workflowID, revision) }
if !errors.Is(invalidErr, storage.ErrWorkflowCursorInvalid) { t.Fatalf("invalid cursor: %v", invalidErr) }
```

- [x] **P3.3.2 Write shared storage continuation and session-isolation tests.**

Add independent cases `RunPagingWithTimestampTies` and `RunPagingFromMissingBoundary` to conformance. Admit `run-a`, `run-b`, and `run-c` for one session at `1000`, plus `run-d` at `999`; page size `1` must enumerate `[run-a, run-b, run-c, run-d]` exactly once, finish with `NextCursor == ""`, and exclude another session's equal-time Run. For the missing-boundary case, use an independent session containing only `run-b`, `run-c`, and `run-d`, and start from encoded cursor `(1000, "run-a")`: continuation still returns `run-b` even though no boundary row exists. Add `DefinitionPagingWithColonIDs`: publish `team:flow` revisions `1` and `2`; size `1` resumes from `team:flow:1` and returns revision `2`. Keep bounded default `50`, maximum `200`.

```go
if !reflect.DeepEqual(ids, []string{"run-a", "run-b", "run-c", "run-d"}) { t.Fatalf("paged ids: %v", ids) }
if cursor != "" { t.Fatalf("terminal cursor: %q", cursor) }
if second.Revisions[0].WorkflowID != "team:flow" || second.Revisions[0].Revision != 2 { t.Fatalf("colon page: %+v", second) }
```

- [x] **P3.3.3 Write RPC invalid-cursor classification tests.**

Add `TestINOFYListRejectsMalformedCursor`: `inofy.listRuns` with cursor `"1000"` and `inofy.listWorkflows` with `"team:flow:bad"` return `InvalidParams`, `data.code == "invalid_input"`, and a message containing `refresh`. Neither case returns an internal storage error or a successful empty page.

- [x] **P3.3.4 Run the new regressions and confirm red evidence.**

```bash
go test ./internal/storage -run 'TestWorkflow(RunCursor|DefinitionCursor)' -count=1
go test ./internal/storage/sqlite -run 'TestWorkflowDefinitionContract/(RunPagingWithTimestampTies|RunPagingFromMissingBoundary|DefinitionPagingWithColonIDs)' -count=1
go test ./internal/storage/postgres -run 'TestWorkflowDefinitionContractPostgres/(RunPagingWithTimestampTies|RunPagingFromMissingBoundary|DefinitionPagingWithColonIDs)' -count=1 -v
go test ./internal/rpc -run TestINOFYListRejectsMalformedCursor -count=1
```

Expected: missing codecs initially fail to compile; after their declarations exist, baseline queries skip timestamp ties and reject valid colon IDs. PostgreSQL execution requires the configured disposable DSN.

- [x] **P3.3.5 Implement shared codecs and strict driver queries.**

Implement the declared helpers in `workflow_cursors.go`: bounded versioned JSON/base64 Run tokens and final-colon definition splitting with positive decimal revision. Initial Run cursor is only `""`; all nonempty legacy numeric cursors fail. Both drivers use this continuation predicate after decoding: `created_at < cursor.created_at OR (created_at = cursor.created_at AND workflow_run_id > cursor.run_id)`. Keep `ORDER BY created_at DESC, workflow_run_id ASC` and `limit + 1`; build the next token from the last emitted row. Definition queries keep their existing `(workflow_id ASC, revision ASC)` ordering and emit their existing colon representation.

- [x] **P3.3.6 Preserve the typed invalid-cursor cause through runtime and RPC.**

`inofyDefinitionRepository.List(ctx context.Context, cursor string, limit int) (definitions.Page, error)` must return/wrap `ErrWorkflowCursorInvalid` without converting it into `inofy.ErrStorageFailed`. `INOFYListRuns` already preserves storage errors. Add an `errors.Is(err, storage.ErrWorkflowCursorInvalid)` case to `inofyRPCError(err error) *Error`, returning `InvalidParams` with `inofyErrorData("invalid_input")` and the refresh message. Preserve all other mappings.

- [x] **P3.3.7 Verify driver parity and commit (`64a3196c`).**

**Execution ruling:** The repository requires the unavailable `oil-frontend`
sub-skill before UI Module edits. At the user's direction to continue through
all work items, the independent backend portions of P3.2 and P3.3 proceeded;
UI work remains gated and no Module source changed. The cost if this sequencing
is wrong is a browser-discovered contract adjustment after the backend work.

**Commit ruling:** P3.1, P3.2 and P3.3 backend changes share the storage/runtime/RPC
files, and their combined affected-package verification has already passed.
Land these backend slices in one atomic commit and keep their separate iteration
logs; Module UI changes will remain separate. The cost if wrong is coarser
task-level git attribution and less focused cherry-picks.

Run P3.3.4 again, then `go test ./internal/storage ./internal/storage/sqlite ./internal/storage/postgres ./internal/rpc -count=1`. Expected: no missed/repeated equal-timestamp rows, colon IDs continue correctly, and malformed/legacy tokens fail explicitly. Stage the verified P3.1-P3.3 backend slices together; do not stage Module UI files.

```bash
git commit -m "fix: harden workflow backend draft and paging contracts"
```

---

### Task P3.4: Complete list navigation and truthful lifecycle — W5, W6

**Files:**
- Modify: `internal/storage/workflow_definitions.go`; `internal/storage/sqlite/workflow_definitions.go`; `internal/storage/postgres/workflow_definitions.go`; `internal/storage/conformance/workflow_definitions.go`; `internal/rpc/inofy_product.go`.
- Modify: `plugins/vivy-workflow/ui/vivy-workflow/src/client.ts`; `plugins/vivy-workflow/ui/vivy-workflow/src/studio/schema.ts`; `plugins/vivy-workflow/ui/vivy-workflow/src/panes/WorkflowsPane.tsx`; `plugins/vivy-workflow/ui/vivy-workflow/src/panes/RunsPane.tsx`; `plugins/vivy-workflow/i18n/catalog.json`.
- Test: `internal/runtime/workflow_product_test.go`; `internal/rpc/inofy_product_test.go`; `plugins/vivy-workflow/ui/vivy-workflow/src/face-bridge.test.ts`; create `plugins/vivy-workflow/ui/vivy-workflow/src/panes/pagination-status.test.tsx`.
- Modify source evidence: `plugins/vivy-workflow/vivy-module.yaml`; `plugins/vivy-workflow/module.go`.

**Interfaces:**
- Consume P3.3 opaque `NextCursor` and existing `WorkflowClient.listWorkflows(cursor?: string)` / `listRuns(cursor?: string)` promises.
- Extend `storage.WorkflowRunSummary` with `EngineStatus string`; RPC list rows include `engine_status` alongside unchanged native `status`.
- Extend UI `RunSummary` with `engine_status?: string`; `RunDetail` inherits it. Keep native status values, including `completed`.
- Add local `isTerminalStatus(status: string | undefined): boolean` behavior for native `completed`, `failed`, `cancelled`; `engine_status == "recovery_required"` is an explicit recovery presentation, not a native terminal.
- Keep list method response `{items, next_cursor: string | null}`; normalize terminal wire `""` to `null` in `WorkflowClient`.

- [x] **P3.4.1 Write joined status projection and RPC assertions.**

Add conformance `RunSummaryEngineProjection` using P3.3 fixtures and existing `NewStepCommit`, `StepAdmitEvent`, and `CommitWorkflowStep`. Test unexecuted admitted fallback `"admitted"`, success `(status="completed", EngineStatus="succeeded")`, and interrupted `(status="active", EngineStatus="recovery_required")`. A fixture with no projection and a native terminal Run falls back to that native terminal value, matching `GetWorkflow`. Extend `TestWorkflowProductRunBindsRevision` and `TestWorkflowProductCancelRun` to assert list and detail agree on the same two fields. Add `TestINOFYListRunsSeparatesNativeAndEngineStatus`; inspect both JSON properties and retain `supports_resume == false`.

```go
if completed.Status != "completed" || completed.EngineStatus != "succeeded" { t.Fatalf("completed summary: %+v", completed) }
if recovery.Status != "active" || recovery.EngineStatus != "recovery_required" { t.Fatalf("recovery summary: %+v", recovery) }
if caps["supports_resume"] != false { t.Fatalf("resume overpromised: %+v", caps) }
```

- [ ] **P3.4.2 Write terminal-cursor and list paging UI tests.**

Add `list clients normalize empty terminal cursors to null` to `face-bridge.test.ts`. In `pagination-status.test.tsx`, add `WorkflowsPane and RunsPane append all pages`: first page returns one row and opaque cursor, Load more sends that exact cursor, second page appends a distinct row, and terminal `""` removes/disables further loading. Add `load more failure preserves rows and retries the same cursor`: first attempt errors, rows remain, retry uses identical cursor. Add `refresh resets the list and cursor`: replacement first page does not append old rows. Assert busy loading disables repeated Load more. Add `late page response cannot replace a refreshed query`: defer the old continuation, refresh or change the selected session, resolve the fresh first page, then resolve the old continuation; only the fresh rows/cursor remain. Give workflows row keys workflow/revision and Run keys Run ID.

- [ ] **P3.4.3 Write lifecycle and recovery presentation tests.**

Add `completed native runs disable cancel`: getRun returns native `"completed"` and engine `"succeeded"`; both values are displayed and Cancel is disabled. Add `recovery required is distinct in list and detail`: native `"active"` plus engine `"recovery_required"` gets a visible recovery label and guidance, not an ordinary-running badge; capabilities remain resume-false and no Resume action is rendered. Deliver `run_recovery_required` through the subscription and assert getRun refreshes and the recovery state becomes visible without waiting for a terminal event.

```ts
expect(cancelButton.disabled).toBe(true);
expect(container.textContent).toContain('completed');
expect(container.textContent).toContain('succeeded');
expect(container.textContent).toContain('recovery_required');
expect(resumeButton).toBeUndefined();
```

- [ ] **P3.4.4 Run new regressions and establish red evidence.**

```bash
go test ./internal/storage/sqlite -run 'TestWorkflowDefinitionContract/RunSummaryEngineProjection' -count=1
go test ./internal/storage/postgres -run 'TestWorkflowDefinitionContractPostgres/RunSummaryEngineProjection' -count=1 -v
go test ./internal/runtime -run 'TestWorkflowProduct(RunBindsRevision|CancelRun|HonestCapabilities)' -count=1
go test ./internal/rpc -run TestINOFYListRunsSeparatesNativeAndEngineStatus -count=1
pnpm -C ui exec vitest run ../plugins/vivy-workflow/ui/vivy-workflow/src/face-bridge.test.ts ../plugins/vivy-workflow/ui/vivy-workflow/src/panes/pagination-status.test.tsx
```

Expected: missing summary engine status, no Load more control, retained empty cursor, and enabled completed-Run cancellation produce failures.

- [x] **P3.4.5 Implement efficient summary status projection.**

Extend each existing `ListWorkflowDefinitionRuns` query with one `LEFT JOIN workflow_executions e ON e.workflow_run_id = r.workflow_run_id`. Scan `COALESCE(e.status, CASE WHEN ru.status IN ('completed','failed','cancelled') THEN ru.status ELSE 'admitted' END)` into `EngineStatus`. Preserve native `ru.status`, session filtering, ordering, and P3.3 predicates. Include `engine_status` in `inofyListRuns`; do not call `GetWorkflow`, load checkpoints, or inspect per-row Journal history.

Backend projection, Runtime list/detail parity and RPC serialization regressions
pass in both drivers. P3.4.2/.3/.6 UI pagination and lifecycle presentation remain
pending the required UI implementation skill.

- [ ] **P3.4.6 Implement bounded list navigation and lifecycle display.**

Normalize list terminal cursors in `WorkflowClient.listWorkflows` / `listRuns`. In each pane retain `nextCursor: string | null` and loading state; first page replaces rows, Load more appends while keeping existing rows on failure, Refresh discards the previous cursor and replaces from the first page. Deduplicate by existing row identities; use a per-query request epoch so late responses from an older refresh/session cannot update rows, cursor or loading state. In `RunsPane`, include native `completed` in terminal/cancel handling, show `engine_status` separately, prioritize a recovery badge/guidance when engine status requires it, and refresh detail on `run_recovery_required` as well as terminal events. Preserve cancellation as the existing governed action and add no Resume action. Add Module keys `workflows.loadMore` and `runs.loadMore` (`Load more` / `加载更多`), `runs.engineStatus` (`Engine: {status}` / `引擎：{status}`), and `runs.recoveryRequired` (`Recovery required; automatic resume is unavailable.` / `需要恢复；自动续运行当前不可用。`), each under `plugin.vivy/workflow-ui.`.

- [ ] **P3.4.7 Verify focused tests, rehash, and run required product acceptance.**

Run P3.4.4 again; rehash the Module before `pnpm -C ui typecheck`. Then execute the program's final source-evidence update and required `just ci`, real-DSN PostgreSQL suites, SDK verification/Recipe packing/Inspect, and split-browser smoke at `http://127.0.0.1:3015`. In the smoke create/edit/publish/start, simulate one lost start reply and retry, load tied-time Runs across pages, inspect a completed Run and recovery-required Run, and verify refresh after legacy cursor rejection. Record commands/outcomes in `docs/logs/YYYY-MM-DD-issue32-remediation/{summary,verification,acceptance}.md`. A skipped PostgreSQL suite or unexercised browser flow is an explicit acceptance gap.

- [ ] **P3.4.8 Commit the independently reviewable deliverable.**

Stage only P3.4 files and its verification record after passing focused checks. Leave the program's cross-package final evidence/release commit to its owning task.

```bash
git commit -m "fix: expose workflow pagination and truthful run lifecycle"
```

## Coverage and handoff

| Finding | Task | Completion evidence |
|---|---|---|
| W1 duplicate tool names | P3.2 | Schema, host validation, publish rejection, readable historical row with safe start rejection |
| W2 lost reply / stale editor state | P3.2 | Identical full retry request, one save, immediate confirmation, no replacement operation on ambiguous conflict |
| W3 repeated ETags | P3.1 | Same/backward-time writes, legacy-token invalidation, reopen |
| W4 timestamp tie loss | P3.3 | Four ordered Runs across one-row pages in both drivers |
| W5 truncated UI lists | P3.4 | Successful continuation, terminal cursor, failed continuation, refresh |
| W6 native/engine status mismatch | P3.4 | Joined summary/detail parity, completed cancel disabled, recovery event refresh, resume false |
| W7 unsafe create intent | P3.1 | Same-author duplicate create conflict and PostgreSQL primary-key race mapping |
| W8 colon cursor IDs | P3.3 | Final-colon parsing, valid existing IDs, explicit malformed-tail errors |

Implementation begins only in a later authorized execution turn. Review these four tasks with the linked spec; the program integration task owns final whole-branch review and durable evidence reconciliation.

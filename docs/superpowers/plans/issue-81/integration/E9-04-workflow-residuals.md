# E9-04 Applicable Workflow Repairs Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans after implementation authorization. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Recover still-applicable #32 W1-W8 corrections through the single E3 workflow lane, with one author/save/publish/start/inspect acceptance path.

**Architecture:** Keep INOFY and current Service admission as sole execution path. Reuse old P3.1-P3.4 implementation: explicit create/edit CAS, opaque fresh tokens, immutable start request, compound cursors and separate native/engine status.

**Tech Stack:** Existing Go storage/runtime/RPC, SQLite/PostgreSQL, React workflow Module and Vitest/Playwright.

**Spec:** #9/#21, #32 W1-W8; old `P3-workflow.md` at `6045c0a69a74c04a1a6221d93c0d7bb32ae7a6e1`. Parent E3 owns overlapping node.inputs/launch edits. Status: package.md; conflict table: dependencies.md.

## Global Constraints

One E3 writer, no parallel E9 workflow patch. No new engine, node types, triggers, wait/resume, fabricated parent or permission bypass. Do not restore old branch generated files or replace current unrelated UI. No SQL DDL is necessary for these repairs. Read current UI/plugin/browser skills before later source edits.

## Review Focus

Repeated/backward timestamps; lost start reply with changed parent/input; equal-time runs across pages; overlapping refresh/session responses; native completed versus engine recovery_required. Tests below exercise each through the real owning boundary.

## Files and interfaces

Existing backend: `internal/storage/{workflow_definitions.go,conformance/workflow_definitions.go,sqlite/workflow_definitions.go,postgres/workflow_definitions.go}` and driver tests; `internal/runtime/{inofy_admission,inofy_definitions,workflow_product}.go` and tests; `internal/rpc/inofy_product.go` and tests. Branch-only helper to recover: `internal/storage/workflow_cursors.go` and tests.

Existing UI root `plugins/vivy-workflow/ui/vivy-workflow/src/`: `client.ts`, `face-bridge.ts`, `studio/{transport,schema}.ts`, `editor/EditorPane.tsx`, `panes/{WorkflowsPane,RunsPane}.tsx` and source tests. Recover branch-only `editor/editor-start.test.tsx`, `panes/{workflow-start,pagination-status}.test.tsx`. Read/adjust `ui/vitest.config.ts` so actual Module source tests run and generated copies do not double-run. E3 owns existing `editor/NodePanel.tsx` input authoring; its source path was verified on main.

Preserve storage public methods and `INOFYStartRun` dedupe. Recover old helpers `NewWorkflowDefinitionETag() string`, `EncodeWorkflowRunCursor(createdAt int64, runID string) string`, `DecodeWorkflowRunCursor(cursor string) (WorkflowRunCursor, error)`, `ParseWorkflowDefinitionCursor(cursor string) (workflowID string, revision uint64, err error)`. Run cursor `v1.` raw-URL JSON contains `{created_at,run_id}` with 2048-byte cap; malformed/legacy unsafe cursors ask for refresh, never silently drop same-time rows.

Reuse typed UI `WorkflowRunIntent` and `WorkflowStartRequest = Readonly<WorkflowRunIntent & {session_id:string;parent_run_id:string;operation_id:string}>`; `HostBridge`/`WorkflowClient.prepareStartRun` captures/freeze-clones once and `startRun` sends that immutable request. Native status is `status`; engine status is optional `engine_status`; never overload one vocabulary.

## Tasks

### 1. Explicit draft CAS — old P3.1, W3/W7

- [ ] Recover `ETagRotatesWithRepeatedAndBackwardsTime`, `TestWorkflowDraftLegacyETagRotates`, PostgreSQL `TestWorkflowDraftConcurrentAbsentInsertConflict`, runtime `TestWorkflowProductSaveRequiresExplicitCAS`, RPC `TestINOFYSaveDraftExplicitCreateOrETag`, and browser same-author dual-create/revision-edit cases.
- [ ] Four edits at equal/backward `Now` must produce different ETags while retaining real CreatedAt/UpdatedAt. A legacy token rotates on successful update and later conflicts. Two absent creates yield one winner/one conflict, no overwrite. Missing/null/empty edit tokens fail; explicit `{create:true}` creates once. Editing a published revision uses its content plus current own draft token, not an unconditional save.
- [ ] Run recovered focused tests red; transplant old `NewWorkflowDefinitionETag` UUID implementation using the existing dependency, both driver write predicates and PostgreSQL typed 23505 draft-primary-key conflict translation. Storage/runtime/RPC reject empty expected tokens; RPC translates create intent to `WorkflowDefinitionETagAbsent`. Other error causes survive.
- [ ] Run `go test ./internal/storage/sqlite ./internal/storage/postgres ./internal/runtime ./internal/rpc -run 'Workflow.*(Draft|CAS)|INOFYSaveDraft' -count=1 -v` and source UI cases. PostgreSQL requires disposable DSN; skips are pending, not PASS.

### 2. Admission and immutable start — old P3.2, W1/W2 plus E3

- [ ] Recover `TestINOFYAdmissionRejectsDuplicateToolNames`, schema `uniqueItems` assertions, publication rejection and historical malformed-publication start refusal. Duplicate `read_file`, whitespace-equivalent names and empty names fail before effect. Use existing `domain.CanonicalToolNames` rather than a second normalizer.
- [ ] Recover immutable-request bridge/editor/published-list tests: a successful save followed by lost start reply retains confirmed ETag and exact session/parent/operation/input across retry; does not resave; changed user intent gets a new operation. Stale-source/conflict retry cannot silently create a replacement. Reject missing or >128-byte operation IDs rather than truncating them. Runtime dedupe/admission remains authoritative.
- [ ] Run tests red, port old P3.2 methods and UI state, run green. Fold E3 schema-level `node.inputs` and active/idle/history launch behavior into the same editor/bridge review. If there is no valid parent, show the existing honest launch restriction or obtain the specific admission decision; never manufacture one. One shared acceptance test covers author -> save -> publish -> start consuming actual inputs.

### 3. Cursor parity and complete UI — old P3.3/P3.4, W4-W6/W8

- [ ] Recover storage cursor conformance: three equal-time runs with limit=1 enumerate exactly once in stable `(created_at DESC,run_id DESC)` order; SQLite/PostgreSQL return the same traversal. Colon-containing workflow IDs round-trip by last separator; malformed/oversized revision cursor returns refreshable invalid-input. Preserve authorization/source scope on every page.
- [ ] Run regressions red, port shared cursor helper and both driver keyset predicates. Keep default list API, expose engine_status distinctly through existing RPC/type mapping. No schema migration or second query store.
- [ ] Recover source UI continuation tests: load further pages, continuation failure retains rows and permits retry, session switch/refresh invalidates stale responses, duplicate rows do not multiply, completed cannot cancel, recovery_required remains visible in list/detail and committed recovery events trigger refresh. Port smallest existing old P3.4 implementation; no visual redesign.
- [ ] Run `go test ./internal/storage/... ./internal/runtime ./internal/rpc -count=1` with real disposable PostgreSQL, then `pnpm -C ui exec vitest run ../plugins/vivy-workflow/ui/vivy-workflow/src` and `pnpm -C ui typecheck`. Report actual source-test inclusion and skips.

### 4. Shared acceptance and evidence

- [ ] Recompute workflow Module source evidence with existing helper, regenerate Assembly/UI staging using supported commands, and run `just ci`. Never hand-edit `ui/src/generated/assembly.ts` or old conformance flags.
- [ ] Use real split Vite/Go browser with local scripted inference: two draft editors conflict correctly; publish/start reads node.inputs; lost-reply retry produces one Run; list continuation reaches equal-time runs; completed/recovery states match backend. Preserve E3 idle/history ownership gates. Record actual request IDs, driver/ports/test bodies and exact artifact SHA in the E3 lane evidence, referenced once from E9.
- [ ] Independent reviewer checks combined E3/E9 patch for CAS/data loss, admission identity, cursor correctness and truthful status. Commit each coherent subtask with its tests under the human contributor; parent records one owner/evidence link per original finding.

**Acceptance:** W1-W8 are accounted for on the current candidate without duplicate E3 work. Drafts cannot silently overwrite; ambiguous start retries cannot multiply effects; lists and status remain complete and honest across both drivers and actual UI.

**Stop conditions:** E3 ownership unresolved; no supported valid parent for requested launch; driver parity cannot run; public CAS/cursor compatibility conflict. Block the affected subtask with exact evidence, never expand workflow scope or mark a skipped gate complete.

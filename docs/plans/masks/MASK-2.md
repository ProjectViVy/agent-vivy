# MASK-2 durable catalog and session control Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans for native execution, or superpowers:subagent-driven-development only when that method is selected. Steps use checkbox syntax for tracking.

**Goal:** Provide durable, revisioned custom masks and authorized session selection.
**Architecture:** Core Storage owns paired migrations and atomic CAS; a T1 service merges built-ins with custom rows; ActionHost exposes a scoped private facade.
**Tech Stack:** Go, SQLite/PostgreSQL, existing ActionHost/RPC; no additional database or public Storage Port.
**Spec:** [design](../../specs/2026-09-21-mask-subsystem-design.md), baseline `5253f77`/`a0f892c`, [MASK-C1](contracts.md).
**Epic / requirements:** MASK-43 / M3, M4. State/predecessor evidence: [index](index.md).

## Global Constraints

- Catalog is organism-wide; selection is per server-loaded session.
- CRUD expected revisions, server-created IDs, immutable original create digest.
- Delete-in-use is rejected; old run snapshots survive catalog edits/deletion.
- All DDL is paired embedded Core Storage SQL, append-only after release.
- No new auto-approval, permissions, background scanner or model-visible tool.

## Review Focus

- Two tabs updating the same revision cannot silently overwrite (Task 1).
- Delete and a concurrent selection/fork cannot create a dangling reference (Task 1).
- Retry after edit uses original create digest, not current content (Task 1).
- Peer binding switches cannot grant arbitrary JSON session access (Task 2).
- Audit failure after write is ambiguous, not a claimed rollback (Task 2).

## Task 1: SQL and transaction semantics

**Files:** NEW `internal/storage/migrations/sqlite/024_masks_and_prompt_snapshots.sql`
and matching postgres path; renumber as one pair if occupied on execution branch.
NEW `internal/storage/sqlite/masks.go`, `masks_test.go`, postgres equivalents;
NEW `internal/storage/conformance/masks.go`; modify both backends'
`history_mutations.go`, `sessions.go`, conformance suites and migration tests.
MASK-1 declarations in `internal/storage/masks.go` remain authoritative.

**Consumes:** storage.MaskStore and mask request/result values.
**Produces:** both real MaskStore implementations, schema for snapshots, atomic
fork selection copy; snapshot admission methods are delivered by MASK-3.

- [ ] Add shared conformance scenarios for fresh/upgrade/reopen, request retry,
  CRUD CAS, selection revision0, session deletion, built-in references and fork.
  Critical concurrency fixture algorithm:

```text
create custom C at revision1; sessions A and B exist
barrier-release two SetMaskSelection(A, C, expected=0)
assert exactly one success and one revision_conflict; stored revision=1
race DeleteCustomMask(C, expected=1) with SetMaskSelection(B,C,expected=0)
assert either deletion wins and selection fails, or selection wins and deletion
returns mask_in_use; never a successful selection referencing missing C
```

- [ ] Run `go test ./internal/storage/sqlite -run Mask -count=1` to capture red.
- [ ] Add three tables exactly per architecture; include create_operation_id unique
  and create_request_digest. Foreign-key session/run retention and mask_id index
  are mandatory; no FK from historical snapshot to mutable catalog.
- [ ] Implement bounded custom list query (`id > cursor ORDER BY id LIMIT limit+1`),
  canonical CRUD and revision predicates. Create retry within the same transaction
  looks up operation UUID first, compares immutable digest, returns existing ID.
  After deletion its bounded dedup guarantee ends; do not add a tombstone ledger.
- [ ] Implement row lock order and existence checks:

```text
selection transaction:
    lock parent session; read selection or virtual rev0; compare expected
    for custom target lock definition and verify existence
    insert/update selection with revision+1; commit
custom delete transaction:
    lock definition; compare expected; count selection references
    if count>0 fail mask_in_use; otherwise delete; commit
fork transaction:
    lock source parent session; read selection
    lock selected custom definition if any
    existing fork writes + independent child selection revision1; commit
```

- [ ] Keep one backend transaction owner. SQLite uses existing serialized writes;
  Postgres uses actual row locks with dedicated-connection race tests. Check rollback
  when any fork write fails. Never use Service.projectionMu as cross-backend proof.
- [ ] Run SQLite/conformance/migration tests and Postgres tests with a disposable
  `VIVY_POSTGRES_TEST_DSN`. Record skipped environment explicitly, not PASS.
  Commands: `go test ./internal/storage/migrations ./internal/storage/sqlite -count=1`
  and `go test ./internal/storage/postgres -run 'Mask|BackendConformance' -count=1`.
- [ ] Commit schema/store change as `feat: persist revisioned mask catalog and selection`.

## Task 2: Service and authenticated actions

**Files:** NEW `internal/modules/masks/service.go`, `actions.go`, `service_test.go`,
`actions_test.go`; NEW `internal/actionhost/masks.go`, `masks_test.go`;
modify `internal/actionhost/host.go`, `internal/app/app.go`,
`internal/rpc/control.go`, `internal/rpc/module_action_test.go`.
NEW `internal/app/masks.go`, `masks_test.go` keeps named private facade wiring focused.

**Consumes:** MASK-1 factory; Task1 MaskStore; existing peer Identity/Policy/audit.
**Produces:** masks.Open; real Service/PromptAssets; seven namespaced actions and
allowlisted error projection; server-authorized selection facade.

- [ ] Add service tests merging built-in/custom metadata, cursor boundaries, missing
  built-in, reserved IDs, body limit, capture as owned copy and separate sessions.
  Add ActionHost fixture asserting a T2 provider cannot obtain mask.ActionHost.
- [ ] Run `go test ./internal/modules/masks ./internal/actionhost -run Mask -count=1`
  and capture red behavior.
- [ ] Build Service from narrow store and immutable assets. Start checks store
  availability; App requires RunAdmissionStore too before enabling production masks
  (MASK-3 implements it). Unit fixtures may provide real isolated test stores.
- [ ] Add compiler-bound owner-aware provider facade wrapping existing providerHost:

```text
on action invocation, after existing authenticate/authorize/input/audit gates:
    if sealed provider owner is T1 vivy/masks:
        pass internal mask facade with named operations
    else pass unchanged public Host
facade Set/GetSelection:
    load authenticated identity from request context
    require request session == peer-bound session and session exists
    call narrowly injected manager method; no raw Store returned
```

- [ ] Define exact JSON schemas and byte limits from MASK-C1; map domain result DTOs.
  List omits bodies; no client owner/tenant accepted. Reject unsafe integer revisions.
  Keep write effects and normal policy decision. Do not auto-grant on UI origin.
- [ ] Safely map only internal trusted mask errors through ActionHost/RPC, preserve
  failure audit outcome, keep unknown provider causes redacted. Test malformed JSON,
  extra fields, timeout, cancellation, denied profile and peer/session mismatch.
- [ ] Simulate completion-audit failure after successful create/update: state remains
  committed, client gets failure, reread and operation UUID/CAS make outcome observable.
  Create retry after a later edit still returns original ID; different original
  content conflicts. Do not assert durability rollback after completion audit failure.
- [ ] Run `go test ./internal/modules/masks ./internal/actionhost ./internal/app ./internal/rpc -run Mask -count=1`,
  relevant existing action gateway suites, then `just ci`. Commit explicit paths as
  `feat: expose scoped mask control actions`.

## Acceptance and handoff

Return AC-02/AC-03 evidence, SQL IDs/checksums, exact API revision and commands.
Unconfigured Postgres prevents parity acceptance, but does not justify dropping it.
Do not activate default Recipe or claim model injection. No database down migration;
rollback data only with the architecture's compatible-artifact/backup policy.

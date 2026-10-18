# E1-BML-01: Durable public BML lifecycle Implementation Plan

> **For agentic workers:** After separate execution authorization, use `superpowers:executing-plans` task by task. This planning task authorizes no implementation or additional sessions. Steps use checkbox syntax.

**Goal:** Prove the existing public long-term BML operations survive real disk close/reopen, including CAS correction and targeted removal.
**Architecture:** Exercise existing action providers and their one composition-owned Service/Home. Close the global owner before reopening the same disposable data directory; never replace SQLite with a fake or recreate expected records after reopen.
**Tech Stack:** Go 1.26.4, existing BML SQLite/FTS5, module action providers.
**Spec:** [Acceptance architecture](architecture.md), A1–A6 and A8–A9; BML-R1/R3/R6 in [index](index.md).

## Global constraints

- Follow root AGENTS.md; fixtures are serial because `memory.Active()` is process-global.
- No new schema, CRUD API, product code, provider dependency or production data access.
- Preserve existing tests rather than repeat their whole operation matrix.
- Every applied/listed/failed status and stable reason must be asserted, not inferred from lack of Go error.
- Existing `memory_add` has no operation ID. Never claim repeated identical Add is a replay or automatically retry an uncertain Add.

## Files and interfaces

- Create (proposed): `internal/modules/memory/persistence_acceptance_test.go`, package `memory_test`, owning only the lifecycle join.
- Reuse: `actions_test.go` helpers `invokeOutcome`, `wantStatus`, `wantFailure`; `module_test.go` helper `testConfig`.
- Read only: `internal/modules/memory/{module,service,actions,tools}.go`, `bml/{home,store,search}.go`, existing tests listed in architecture.
- Future execution evidence: the delivery log defined in the index, owned by the integrator.

Consumes existing `memory.Open(context.Context, config.Config) (*memory.Service, error)`,
`memory.Close() error`, `memory.Active() *memory.Service`,
`(*Service).Home() *bml.Home`, `(*Home).DatabasePath() string` and action constants
`ActionAdd/Get/Search/Update/Remove/List`.
Action inputs are `{kind:"long_term",content}`, `{id}`, `{query}`,
`{id,content,base_revision}` and `{id,reason,base_revision}`.
Outputs are `bml.MemoryCrudOutcome`; get returns `listed` with one entry, not `entry`.
Produces test `TestBMLCloseoutPublicDiskLifecycle` and evidence of stable IDs,
revisions, content and visibility across ownership restarts. No new runtime API.

## Review focus

1. Reopen accidentally creates a new root or reuses a live Home: assert same path, nil Active after Close, and stale Service unavailability.
2. Stale CAS silently wins: compare persisted winner after another reopen.
3. FTS retains corrected/deleted content: query both old and current tokens after reopen.
4. Tombstone confused with physical erasure: public get is not-found; low-level record retention is expected and separately covered in 03.
5. Outcome parsing treats a failed provider payload as success: assert status/reason and exact selected record ID at every mutation/read.

## Ordered tasks

- [ ] **1. Reconcile existing coverage.** Read `TestHomeLongTermCRUDIsRevisionedAndTombstoned`, `TestUpdateAndRemoveEnforceRevisionCAS`, `TestMemoryUpdateToolHonorsCASAndPreservesContent`, `TestMemoryRemoveToolTombstonesUnderCAS`, `TestClosedServiceReportsUnavailableOutcome`. Run existing suites before adding coverage:

  ```sh
  (cd bml && go test ./... -count=1)
  go test ./internal/modules/memory -count=1
  ```

  Expected: all required tests pass without skip. If equivalent durable action coverage has landed since the pin, reuse it and report the exact test instead of adding a duplicate.

- [ ] **2. Add the disk lifecycle test.** Use one `root := t.TempDir()` and `cfg := testConfig(root)`. Fail if a service is already active; register final `memory.Close` cleanup. Open, add `"bmlcloseout oldvalue synthetic record"` through `ActionAdd`, require `applied`, nonempty ID and revision 1. Stat `first.Home().DatabasePath()` and record it. Close explicitly; require `Active()==nil` and that `first.Get` reports `failed/bml_unavailable`. Reopen using the identical cfg, assert a new Service pointer and identical database path. Require Get and Search(`"oldvalue"`) each return exactly that ID, revision 1 and exact content. No writes between reopen and reads.

- [ ] **3. Extend the same test through correction.** Update that ID to `"bmlcloseout newvalue synthetic record"` with base revision 1, require `applied` and revision 2. Submit stale update and stale remove at base revision 1, require `failed/memory_revision_conflict` for both. Close/reopen again. Get returns only the revision-2 content; Search(`"oldvalue"`) excludes the ID and Search(`"newvalue"`) includes it once. This verifies the winner and index, not only a returned mutation DTO.

- [ ] **4. Extend through targeted deletion.** Remove the same ID at base revision 2 with nonempty reason. Require `applied`; the returned entry is the deposed revision-2 record, not a made-up tombstone revision. Close/reopen; Get is `failed/memory_not_found`; List and Search(`"bmlcloseout"`), Search(`"newvalue"`) exclude that ID. Do not demand that all history or raw evidence vanished. Do not resubmit Add to “replay” the removed record.

- [ ] **5. Verify the added join.** Run:

  ```sh
  go test ./internal/modules/memory -run '^TestBMLCloseoutPublicDiskLifecycle$' -count=1 -v
  go test ./internal/modules/memory -count=1
  go test -race ./internal/modules/memory -run '^TestBMLCloseoutPublicDiskLifecycle$' -count=1
  ```

  Expected: the named test executes (not “no tests to run”), all assertions pass, no race finding. A new acceptance test may pass immediately against delivered code; do not manufacture a production defect or change behavior to obtain a red phase. A failure is diagnostic evidence and invokes the stop conditions below.

- [ ] **6. Review and commit the test-only increment.** Have an independent reviewer inspect the exact diff and persisted-state assertions under later review authorization. Record commands/results and limitations. Stage only this test and its owned execution evidence, `git diff --cached --check`, commit with the configured responsible human identity and no AI trailers. Do not merge or close a tracker.

## Stop conditions, acceptance and rollback

Stop on an unexpected active service, config resolving outside the disposable
root, failed close, required-test skip, revision/index mismatch or a product fix
being necessary. Preserve the failure; block 01 and dependent 02/04 only. Do not
import or repair #79. Missing Go blocks execution checks, not unrelated planning.

Accept when one continuous public-action chain proves R1/R3 using disk-backed
state and real ownership restarts, with the named checks and independent review.
Final `just ci` belongs to the exact integrated candidate gate in the index.
Rollback closes owned handles and discards only the test-created root after
capturing evidence; no tenant deletion or shared-tree cleanup.

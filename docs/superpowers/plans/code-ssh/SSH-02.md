# SSH-02 worker files and receipts implementation plan

> **For agentic workers:** Use superpowers:executing-plans for native execution. Steps use checkboxes; follow the index's readiness gates before coding.

**Goal:** Mutate only the bound remote project and preserve truthful execution evidence across retries/crashes.
**Architecture:** Worker-owned Linux root handle and private durable receipt store; local Vivy Journal retains approval/admission authority.
**Tech Stack:** Go 1.26.4; standard library persistence; pinned x/sys Linux primitives only where secure no-follow resolution requires them.
**Spec:** [SSH-D1](../../specs/2026-10-11-code-ssh-design.md) §6; [SSH-C1](contracts.md); baseline `ce33bc8776b9f52ab13bed5f2cdaf8ebad01f666`.
**Epic / requirements:** E1 / R2, R4-R5. State and predecessors: [index](index.md).

## Global constraints and review focus

No database, synchronization cache or local fallback. Private state is outside
the project, 0700/0600, inaccessible to file methods. No automatic receipt
eviction. Unknown outcome never becomes completed because current bytes match.
Review fsync ordering, concurrent duplicate admission, symlink races, stale
preconditions and partial multiedit. Path containment is not a process sandbox.

## Prerequisites and contracts

Consume SSH-01's exact wire/digest/typed payloads and A02. Produce SSH-C1
receipts.Store and workspace.Workspace signatures, closed fs.* handlers and
operations.get. Store alone owns admission/deduplication; handlers cannot bypass
it. Dispatch later reused by SSH-03/06. Linux test environment is required.

## Task 1: Durable receipt identity and crash recovery

**Files:** NEW proposed `extensions/ssh-worker/receipts/{store,atomic,recovery}.go`,
`receipts/{store,recovery}_test.go`, `worker/{dispatch,dispatch_test}.go`.

- [ ] Write `TestConcurrentDuplicateAdmitsOnce`, `TestConflictDigestDenied`,
  `TestReceiptCrashWindowsRemainUnknown`, `TestStateFailurePreventsSideEffect`,
  `TestMissingReceiptNeverAllowsReplay`. Inject failure before/after intent fsync,
  side effect, terminal rename and directory fsync; count actual effects.
- [ ] Run `go test ./receipts ./worker -count=1` in the worker module for red.
  Implement exclusive operation admission with one process owner per state
  directory plus a per-identity critical section. Persist intent before dispatch;
  Settle requires durable result/dir sync before terminal reply. Unknown/corrupt
  records stop replay. Reject unsafe state directory ownership/permissions.
  Validate bounded identity strings and use digest-derived receipt filenames;
  caller IDs can never become filesystem paths. Same-uid job access to worker
  state requires the approved execution/isolation profile, not only mode 0700.
- [ ] Recovery marks interrupted started work unknown; retained terminal result
  is returned for a duplicate without invocation. Per-operation JSON may retain
  compact identity tombstones after explicit maintenance; V1 does no eviction.
  operations.get is a read-only binding-scoped lookup, not admission.
- [ ] Run `go test -race ./receipts ./worker -count=1` and `go vet ./receipts ./worker`.
  Assert concurrent effects equal one and failed state writes equal zero.
  Commit as `feat: persist SSH worker operation receipts`.

## Task 2: Root-confined reads, edits and partial multiedit

**Files:** NEW proposed `workspace/{root_linux,files,edits,search}.go`,
`workspace/{root_linux,files,edits}_test.go`; UPDATE worker dispatch and module
dependency pin only if a Linux primitive is necessary.

- [ ] Add `TestRootRejectsEscapesAndSymlinkRace`, `TestPrivateStateInaccessible`,
  `TestWriteRequiresExpectedDigest`, `TestPatchRejectsAmbiguousMatch`,
  `TestMultieditPreflightAndPartialReceipt`, `TestSearchAndReadBounds`.
  Fixtures include ../, absolute/drive/backslash paths, NUL, symlink ancestors,
  FIFO/device files, stale content and failure on the second per-file commit.
- [ ] Run `go test ./workspace ./worker -count=1` for red. Open a trusted root
  handle; resolve/open descendants with beneath/no-symlink/no-magic-link semantics
  on the final descriptor, not Lstat followed by a racy absolute open. Prefer
  Go root APIs if they satisfy all tests; otherwise use existing pin
  `golang.org/x/sys v0.47.0` with Linux openat2 flags. Unsupported primitives
  return capability unavailable; no insecure path fallback. Record exact minimum
  remote kernel capability in hello/install docs if openat2 is used.
- [ ] Enforce approved sensitive-path policy before I/O. Atomic write uses same
  directory temp file → content sync → rename → directory sync. Hash/precondition
  validation and commit run under the worker's mutation serialization; external
  writers cannot be claimed to participate in that lock. Retain evidence of
  detected changes; do not claim a universal cross-process compare-and-swap.
- [ ] Multiedit groups by file, validates every original digest and unique match
  before any write, commits deterministically per file and persists progress.
  Partial crash/failure names completed/unresolved paths; never returns all-or-
  nothing success. Bound list/search recursion, results and byte reads.
- [ ] Run `go test -race ./workspace ./receipts ./worker -count=1`, crash helper
  subprocess tests and `go vet ./...`. Check A03 and update exact capability docs.
  Commit as `feat: add confined SSH workspace file operations`.

## Acceptance and handoff

Return A03, effect counters and crash-window receipts, approved root/state
configuration, exact unsupported-platform behavior and dependency rationale.
SSH-03 consumes these implementations, not alternate in-memory fakes. Stop if
secure resolution or durable storage cannot be established. Rollback closes
owned handles and deletes test roots only; operator receipts remain retained.

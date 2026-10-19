# Issue 58 S2 Task 3: complete disposition capture

Date: 2026-10-15. Implementation model: GPT-6.1 Sol High.
Base: `8d7c23cd81afa85c7fba84b600aa5e5a2668dadf`.
Source commit: `3787f6252a69205dd0c65ab23603041930a00da9`.

## Delivered contract

The same `storage.SubjectSourceStore` now exposes:

```go
CaptureSubjectContext(context.Context, SubjectHistoryBinding) (SubjectContextSnapshot, error)
ListSubjectPendingRuns(context.Context, SubjectHistoryBinding) ([]SubjectPendingRun, error)
```

The authorized Service producer is
`(*runtime.Service).CaptureSubjectContext(ctx, readerSessionID)`.
The original four C2 method signatures, `CaptureSubjectHistory`, immutable source
digest, actual terminal outcome and durable terminal-interaction sequence are unchanged.
Existing per-session `RepairSubjectHistory` remains compatible.

`SubjectContextSnapshot` contains `Through`, `Dispositions`, `DispositionDigest`
and the architect-approved `Sources []SubjectSourceBinding` extension. Sources is
the complete distinct persisted fixed-domain membership from the SAME SQL cut,
including archived sources with no markers. It is current authorization input,
not an acknowledgement/cursor. Parent architecture ruling explicitly required this
extension after the no-marker archived-source grant counterexample; parent owns
the corresponding authoritative design/plan edits, excluded from this commit.

Disposition and target fields follow revision 3: marker view SessionID and SQL
MarkerID; all exact marker fields; target InteractionID, Sequence, canonical
SourceSessionID, RunID, SourceDigest and same-length exact canonical MessageIDs /
ViewMessageIDs pairs. Marker ID is identity only. Copy rewinds describe only that
copied view; no parent disposition, original digest, outcome, or index sequence is
rewritten. Fork/forked-from provenance markers do not supersede.

## Storage snapshot and digest

`subjectsql.Store.contextTx` opens one read-only transaction. SQLite pins its
snapshot at the first identity SELECT and retains it through commit; the existing
SQLite backend uses one connection, and the native concurrent admission/seal/marker
fixture exercises serialized transaction cuts. PostgreSQL explicitly requests
`sql.LevelRepeatableRead` plus `ReadOnly:true`, avoiding multiple independent READ
COMMITTED observations. Real PostgreSQL execution is still unverified below.

Within that one cut capture validates identity, checks the terminal-unsealed set,
reads the existing subject head, captures complete persisted source membership,
reads eligible rewind/edit markers, resolves durable closed message-position
ranges and exact copy links, validates participating source/run pins, then hashes.
All raw SQL transaction queries use `s.query` or `s.one` (which calls `s.query`),
so PostgreSQL receives native `$n` placeholders. Backend wrappers also preserve
the existing PostgreSQL lease guard. No new schema/migrations, SQL bodies,
checkpoint authority, marker cursor, recap, acknowledgement or cache were added.

Anchors resolve by exact IDs and durable `messages.position`, never timestamps or
content equality. Missing cutoff/tail, reversed ranges, missing/cyclic/inconsistent
copy mappings fail closed. Exact-copy traversal is bounded to 128 links; deeper
chains fail with incomplete evidence rather than guessing. Unindexed/private /
unpinned / NoContext / active rows are not targets and cannot retract an original.
An admission edit affects old indexed originals before its replacement seals.

The SHA-256 disposition digest serializes a versioned struct containing binding
and the complete sorted disposition/target metadata. View rows retain their exact
canonical pairs in stable durable-position order; targets sort by interaction
identity and markers by view SessionID/SQL identity. These are serialization keys,
not a cross-session chronological cursor. Through and unrelated source membership
are not hashed; a new unrelated interaction/source leaves disposition digest stable
unless an actual affected target set changes. Overlapping markers retain exact
scoped ranges for union semantics, including equal/backward CreatedAt fixtures.

## Authorization and domain repair

Service authorizes the current whole-domain reader first. Under the existing
`projectionMu`, it selects a finite metadata-only pending set over all persisted
subject-domain sources, including archived A and pinned main B. It compares every
selected source/run pin to current trusted Host decisions, repairs ALL selected
terminal projections in phase one, then reauthorizes and idempotently seals in
phase two. Existing Journal/message projector, retained tool receipt/artifact
expansion, complete-evidence prefix guard and durable seal allocator remain owners.
The query is proportional to pending metadata; it does not enumerate every
historical run pin or scan all source bodies on the capture path. Existing projector
body reads occur only for selected pending repair work.

After atomic capture, Service current-resolves and compares every captured source
binding, including archived/already-covered sources with no markers. It separately
checks marker/target run pins and exact intermediate copied-view source grants.
A target emerging after preflight is checked before return. Current reader grant
is checked again before return. Any denial/error returns a ZERO entire snapshot,
never a partial set. Captured source bindings are immutable storage admission pins;
they are not cached current authorization and must be rechecked by S3 at provider /
resume boundaries. No permission denial is handled by deleting/reclassifying old
facts or by reconciling recap text.

The metadata pending query detects both terminal status missing terminal Journal
and terminal Journal with stale active status. Genuine active/inflight/no-terminal
and unpinned NoContext runs remain excluded. Inconsistent status is not invented or
repaired by this producer: the existing core recovery owner must make it consistent
before a later capture can succeed. A terminal committing after the runtime's
pending selection but before SQL capture is detected by the SAME-cut pending
query and returns `ErrSubjectEvidenceIncomplete` with zero metadata; a fresh call
repairs it. No second recovery scheduler or automatic retry engine was created.

## RED and GREEN evidence

Raw logs and SHA-256 manifests are stored beside this report.

| Evidence | Meaningful observed failure before fix |
| --- | --- |
| `task-3-red.log` | Backend and Service lacked complete context capture capability. |
| `task-3-red-domain-grant.log` | Already-covered archived A with no marker and revoked grant returned successful Through=2. |
| `task-3-red-pending.log` | Terminal-status A missing terminal Journal event silently disappeared; capture advanced to Through=1. |
| `task-3-red-status.log` | Sole terminal Journal with stale active status returned success/empty ceiling. |
| `task-3-red-exclusion.log` | Pre-binding unpinned row was treated as missing canonical evidence and invalidated an eligible original. |
| `task-3-red-interphase.log` | Real terminal admitted between pending selection and capture was silently omitted at Through=0. |

Final commands used `PATH=/workspace/scratch/243d1cbb859f/toolchains/bin:$PATH`,
Go `go1.26.4 linux/amd64`, ordinary `sum.golang.org` checksums:

```sh
go test ./internal/storage/sqlite ./internal/runtime -run '^TestIssue58Subject(Context|DomainPendingRepair)' -count=1 -v
go test -race ./internal/storage/sqlite ./internal/runtime -run '^TestIssue58Subject(Context|DomainPendingRepair)' -count=1 -v
go test ./internal/storage/postgres -run '^TestIssue58SubjectContext' -count=1 -v
go test ./internal/storage/... ./internal/runtime -run '^TestIssue58' -count=1 -json
git diff --check
```

| Final result | Actual output |
| --- | --- |
| Focused | 11 top-level cases + 4 failure-path subcases PASS; SQLite 0.325s, runtime 0.478s. |
| Focused race | Same 11 top-level + 4 subcases PASS; SQLite 8.711s, runtime 12.149s; no race report. |
| Affected Issue58 | 84 passed test events, 13 skipped test events, 0 failed; 5 tested packages pass and 3 packages have no test files. |
| PostgreSQL Task3 | 0 executed database cases; 5 explicitly skipped because `VIVY_POSTGRES_TEST_DSN` is unset. **Pending S6 real-server evidence, not a pass.** |
| Diff whitespace | Clean for the staged producer increment. |

Final fixtures cover archived A / pinned-main B reopen and idempotent repair,
projection-all-before-seals (prefix guard would fail otherwise), projection failure,
missing terminal event, stale active status, revoked pending grant, true active and
actual NoContext shell exclusion, exact restoration of a no-marker covered source
grant, grant-version mismatch, ordinary-reader denial, nested copied-view mappings,
intermediate source denial, missing mapping/anchor fail-closed, private-view
exclusion, immutable parent digest, unchanged ceiling on rewind, admission edits,
concurrent capture/seal/edit/rewind with explicit post-seal marker labels and
stable digest under an unrelated new interaction.

Source manifests printed by the final native tests:

| Fixture | Value |
| --- | --- |
| Archived original | `context-archived-A-run:2` / `8552ad1f6f8123f3bde02100c0f3c9bd52a056d7dab7d6ef2096b6e2e6b19219` |
| Archived/edit disposition | `35a86d868fd68338173080c03a3b33cbec477166271bd97503e6f26f82a33789` (Through=1; 2 markers) |
| Nested parent original | `context-copy-parent-run:2` / `6415b6134edd4f83910b46a5cc6a5b6a872c825054e239c2101a047e77e4b70c` |
| Nested view disposition | `447908b13b9a2e5f8eb15d1d09ec6302a429b812163f586bc232f4bb68a49e06` (Through=3; 2 view markers) |

`task-3-verification.json` records exact source file hashes, raw log hashes/bytes,
all affected skip names and counts. The PostgreSQL affected package's green process
status includes 13 skipped database cases and is not PostgreSQL behavioral proof.

## Self-review, scope and handoff

Reviewed SQL dialect rebinding, read-only transaction boundaries, row iterator
closure before dependent queries, zero-return failures, unchanged original APIs,
current grant comparisons, exact view/canonical identity, finite copy traversal,
pending phase ordering, prefix-guard preservation and interphase terminal arrival.
No unresolved source defect was found in this self-review. Independent Task3 review
is a separate parent-owned gate after this stable committed handoff.

This is metadata producer/recovery work. No Eino/model/provider orchestration,
UI/#66/pin, Recap consumer, S5 observer, deployment, push or merge was changed.
Official Eino v0.9.20 and its import quarantine are untouched. Existing
`projectRunMessagesLocked`, Journal and retained ToolOperationStore/BlobStore
recovery remain the evidence owners; no new upstream API is needed for metadata
SQL capture. S3 actual HTTP/provider integration is still future work, and this
report does not claim a scripted snapshot as provider acceptance. No full CI,
full runtime package rerun, tenant data or PostgreSQL installation workaround was
attempted. No DDL changed, so migration parity remains the existing paired tables
rather than a new migration claim. Parent-held design/S2/S3/index and frozen S1
review files are excluded from producer/report commits.

Rollback disables the new capture consumer; retained originals, existing markers,
immutable index and original APIs remain. Do not fall back to reader-session-only
history. Real PostgreSQL execution and S3 provider/resume grant rechecks remain
explicit downstream gates.

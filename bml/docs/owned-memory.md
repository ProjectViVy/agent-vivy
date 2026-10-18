# Native owned-memory contract (schema 2)

BML remains one Home/Store, one canonical SQLite database and the existing FTS5
ranker. It does not own Recap, core originals, C2 grants, a transcript archive,
an observer cursor or a scheduler. The trusted host validates source eligibility
and supplies the complete ownership binding; the library never infers it from
ProfileID or existing machine records. Garden/Vivy imports are absent.

## Ownership and native APIs

`OwnershipBinding` is the exact tuple SubjectID, DomainID, ScopeKind, logical
WorkspaceID and DestinationID. Personal scope requires empty logical workspace;
workspace scope requires its explicit workspace. Strings are not trimmed or
normalized into another identity. Existing `Record.Scope` retains its independent
native tenant/physical database workspace/session semantics. A binding is trusted
host input, not an authentication token or a substitute for current core grants.

| API | Result and boundary |
| --- | --- |
| `Store.MutateBound(ctx, binding, BoundMutation)` | Native create/update/tombstone in the existing putInner transaction; returns immutable MutationOutcome. |
| `Store.LookupBoundMutation(ctx, binding, operationID, payloadDigest)` | Exact retained outcome, nil when absent, explicit conflict/denial/capability failure otherwise. Never reconstructs an old result from the current row. |
| `Store.GetBound(ctx, binding, recordID, expectedRevision)` | Exact visible native row; stale/unknown-owner/tombstoned/superseded material fails explicitly. |
| `Store.SearchBound(ctx, binding, SearchQuery)` | Existing native FTS escaping, rank and visibility with ownership predicate before ranking/LIMIT. |
| `Store.BoundCapabilities()` | Opened-handle schema/read-only capabilities; does not advertise a future adapter. |
| `Home.MutateBound/LookupBoundMutation/SearchBound` | Same owned native operations using the existing lazy store/lifecycle. Home refreshes legacy L1 after mutation; refresh errors retain the committed outcome for retry. |
| `Home.ExportBoundSource(ctx, binding, id, expectedRevision)` | Exact native historical/long-term source with stored ownership and visibility checks; no core-original reconstruction. |
| `Home.ExportSource(ctx, id, expectedRevision)` | Legacy/unbound historical/long-term source only; the unscoped method never grants access to bound material. |

BoundMutation carries an explicit operation, operation ID, caller payload digest,
complete canonical Record and optional store/record/target CAS. Nil store CAS means
use the actual revision within the transaction; explicit zero is a distinct request.
Nil record CAS means absent for create. Update requires a positive record revision.
Tombstone requires an absent new native marker and positive target revision.
Its Supersedes list must contain exactly that one guarded TargetRecordID; an
extra target is rejected before persistence. Legacy unbound multi-target
tombstones retain their existing behavior.
Record/evidence/provenance fields are preserved. All raw Go strings are checked for
valid UTF-8 before fingerprint marshaling, including nested optional source fields.
Direct Go enum values that would decode differently are rejected.

Operation IDs occupy one native database namespace. The retained result binds the
full owner/destination and caller digest plus SHA256 of the canonical complete
request, including Record/source fields, operation and optional CAS shape. Changed
requests cannot replay using an unchanged caller digest. Validation precedes lookup;
successful replay precedes current row/store CAS and capacity conditions. Replays
after update, tombstone or reopen return the original outcome without a new write.

MutationOutcome retains the actual native record ID/revision, store revision,
operation, binding, digest, target ID and original canonical/index disposition.
Tombstone returns the new native marker's ID/revision and the target ID; it does
not claim that the deposed original's row revision advanced. Completed/ready
describe that original SQLite/FTS commit, not the current row's later visibility.
No record body is copied into the receipt table.

## One canonical transaction and isolation

`memory_ownership` and `memory_operations` are written before putInner's existing
Commit, alongside record CAS, tombstone/supersedes, FTS and counters. There is no
CRUD-then-receipt dual write and no use of the retired memory_apply_journal seam.
Writable handles use the pinned driver's immediate transaction mode and existing
busy timeout, preserving SQLite serialization across Store handles. Insert and
actual deferred-constraint commit failures roll back the complete operation.

The same transaction checks existing record ownership, supersedes/tombstone
targets and foreign suppression. A deleted target cannot be updated/deleted again
under its old CAS, and unknown legacy records cannot gain a new subject owner.
Cross-binding/destination operation lookups fail without disclosing the result.

Legacy Store Get/List/Search/SearchVisible, imports and writes exclude or reject
owned material. Ownership filters precede LIMIT/ranking; legacy Home CRUD, recall
and startup L1 inherit that exclusion. Session GC also excludes owned records and
their FTS/relations. Bound reads exclude unknown/unbound old rows. Existing legacy
CRUD, FTS and identity migration remain for their own records. These library
guarantees do not prove actual App ContextHost, worker or C2 projection wiring.

## Schema, read-only and recovery

Fresh stores create schema 2. The existing schema transaction upgrades supported
schema 1 by adding the two native tables and updating schema_meta atomically;
record JSON, IDs, record/store revisions, counters and FTS data are unchanged.
No old ownership or outcome is backfilled. Real Home.existingStore mutable opens
perform the same bounded upgrade. Missing Home reads stay lazy/absent.

A schema-1 file containing the new reserved table names is not a supported legacy
shape: it is rejected rather than adopting unqualified ownership/history. Schema
2 validates required native columns; a missing outcome table fails rather than
being silently recreated. Unsupported versions and foreign database identities
fail closed. No core migration or actual tenant-data migration is performed here.

OpenExisting is read-only: schema 1 can serve its existing legacy reads but all
new bound capabilities are unavailable. Read-only schema 2 can search/export/look
up bound outcomes, but cannot mutate or restore over the database. No read-only
open upgrades schema or creates native data. Read-only snapshots remain unchanged.
A schema-1 handle checks the actual schema and reads legacy data in the same
SQLite read transaction. If a supported writer has already upgraded that
snapshot to schema 2, Get/List/Search/SearchVisible/SupersededTargetIDs return
`schema_changed`; callers must close and reopen the handle. An in-flight schema-1
snapshot can finish reading its original legacy data. Cached capabilities stay
conservative until reopen; they never grant access to newly owned rows.
OpenExistingCanonical retains its documented mutable identity-upgrade behavior;
it is distinct from OpenExisting.

Backup remains a SQLite VACUUM INTO consistent snapshot of records, ownership and
outcomes. Backup validation during Restore/identity migration/rollback uses the
read-only path. Restore upgrades a disposable copied database first and replaces
the closed live file only after successful upgrade/close; upgrade failure preserves
the original live database and backup bytes. The receiver still closes on every
Restore path. Callers must own the database maintenance lifecycle exclusively;
restore is an explicit snapshot rollback, not a concurrent merge or new receipt
history beyond the chosen snapshot.

Canonical workspace identity migration/rollback is rejected before metadata,
record, backup or manifest writes if ownership OR historical operation state is
present, including after records disappear. Rewriting physical Record.Scope would
invalidate immutable request fingerprints. The existing unbound legacy identity
migration/rollback remains supported and tested. Home's fixed machine identity
does not use that migration. This restriction is intentional; no automatic
conversion, re-keying or bound-state migration is supplied.

Native SQLite 1.56.0 and root MVS 1.59.0 are qualified separately. Actual Windows,
App/adapter activation, whole S5/S6, UI, live-model and tenant acceptance remain
root-owned downstream gates.

## Operation-only status consumer

`Store/Home.LookupBoundOperation(ctx, binding, operationID)` serves a trusted
consumer whose status contract has no caller digest. It validates the full stored
owner/destination and returns the original retained digest/outcome, never the
current record. A missing operation stays absent; legacy schema/read-only
capabilities stay explicit. The existing `LookupBoundMutation` still requires a
nonempty matching digest. Neither API changes mutation replay's complete native
request fingerprint or CAS. This adds no schema, cache or receipt store.

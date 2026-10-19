# Issue58 S5 Task1a: native adapter and App ownership

This checkpoint prepares one native BML backend for the selected App's trusted
Recap subject. It does not activate LIFE projection. Historical C2 source
navigation (Task1b), projection activation, resident tool/recall qualification,
and loaded-revision reporting (Task2) remain unimplemented.

## Native backend

`internal/modules/memory.BoundBackend` implements the existing Garden
`memory.Backend` contract using the exact App-owned BML Home. Native BML owns
CRUD, FTS rank/visibility, exact bound export, CAS, immutable operation results,
and replay. The adapter adds no records, receipt cache, transcript store,
scheduler, ranking system, or raw `CreateMemory` ingest port. Borrowed `Close`
does not close Home. Native mutation errors remain native errors.

Garden mutations have no source clocks. The deterministic translation uses
explicitly unspecified zero timestamps, historical kind, session-sync provenance,
and the supplied observed/inferred classification; none denotes an observed
capture time or a user assertion. The full canonical Garden request hash is
included in native provenance, and typed source references are preserved in
versioned base64 JSON evidence URIs. All raw request strings are validated as
UTF-8 before JSON/base64 encoding. Caller payload digests never replace native
complete-record fingerprints. Tombstones retain their actual native marker ID
and revision; replay and status return retained original outcomes after later
updates, deletion, and reopen.

Search cards omit source bodies. Expansion uses exact native record/revision
export. This native material is not an export of core C2 originals and does not
establish source eligibility. Pagination only slices native FTS's bounded ordered
100-hit result; encrypted continuations belong to the same adapter and request.

The narrow producer prerequisite adds read-only Store/Home
`LookupBoundOperation(binding, operationID)` to serve Garden's digest-free
`MutationStatus`. It shares native query/decoding with strict digest lookup,
validates complete ownership and stored immutable outcome, and changes no schema
or mutation/replay semantics. `LookupBoundMutation` still requires the digest.

## Ownership and composition

One existing Active registry and one lifecycle mutex serialize acquisition and
release. The normalized Home root and actual core `OrganismLeaseOwner` must both
match before reuse. Foreign-root and same-Home/different-core-lease acquisition
fail without disclosing or closing the first App's service. Exact Service release
cannot detach a later replacement; no-memory App close does not close ambient
memory. Constructor failure cleans only the service it actually acquired.

App receives typed copies of selected firstparty memory tools, actions, recall,
and observer providers, bound to its exact service. Caller Assembly reuse does
not rebind earlier App handles. Closed bound providers cannot follow ambient
Active. Standalone legacy unbound callers retain their existing registry behavior.

The existing trusted Recap binding callback constructs the selected adapter after
its native owner resolves C1 identity and before its workers start. It borrows
App's Home through private firstparty FactoryInput. No second owner or generic
plugin Port is added. ProfileID remains `diva`; legacy Garden/raw/Mentle routing,
Coding tools and existing subscriptions remain. No backend map entry or profile
alias is installed. A selected no-BML generation has no prepared BML sink; this
checkpoint does not yet deny LIFE admission for that absence.

Run's ordinary cancellation path releases its exact Home after consumers drain
and before the core lease closes. As before, early listener returns require the
reusable in-process caller to invoke full App Close; the existing face/headless
entrypoints defer that close. The CLI fatal-error path exits the process instead;
this checkpoint does not claim it drains all consumers on an early listener
error. This change does not independently close Home while its consumers remain
live.

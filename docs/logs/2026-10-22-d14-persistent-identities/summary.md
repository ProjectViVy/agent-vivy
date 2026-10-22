# D14 persistent identity closure

Channel sessions now resolve the exact platform `(channel, chat_id, topic_id)`
tuple in the existing session owner. Paired migration 047 adds nullable tuple
columns and one partial unique index; session creation binds the tuple in the
same insert. Converted sessions keep their original IDs, titles and historical
references. New session IDs encode the complete tuple reversibly. Concurrent
creation resolves the unique winning row.

Observer progress keys encode the complete provider/Run tuple. Read-only startup
admission refuses old observer namespaces and unbound channel history before
DDL or progress writes. Runtime lookup also refuses to create a conversation
beside unproven channel history. No old digest computation or fallback reader
remains in the three identity producers.

Memory history IDs and receipt IDs encode the complete original Run ID without
sanitization or truncation. The existing BML record owner resolves the exact
original `provenance.source_id` first, preserving historical record IDs,
revisions, links and hidden records on replay. Ambiguous source associations
and ID reuse for another source fail closed. Capturing the existing store CAS
before source lookup prevents concurrent allocations from duplicating a source.
The production SourceID is the immutable committed Journal EventID. A newer
projection of that event reuses its original stored representation and cannot
rewrite historical content or revision. The provider remains the sole
production caller through the existing Service facade.

## Ownership and design

The smallest existing-owner design is a tuple binding on sessions, existing
snapshot keys for observer progress, and existing BML provenance for memory
replay. Rekeying old sessions would change references embedded in immutable
history; a separate binding registry would duplicate ownership. Migration files
001 through 046 remain unchanged.

The sync module remains T1 `vivy/memory-bml-sync`, source `file:internal`,
provider `vivy.memory.bml`, using `std/context-source@v1` and
`std/observer/run@v1`. No Port, Grant, constructor, lifecycle, delivery timeout
or Recipe changes were added. Eino capability check: this changes persistence
identity, adds no orchestration capability and leaves the pinned Eino v0.9.21
runner/import quarantine intact. Eino types do not enter storage or BML.

## Delivery boundary

The finite upgrade artifact owns copied-target binding of old channel tuples,
observer key conversion and restoration of exact memory source identities.
Only source-owned full tuples or explicit private original tuples verified
against the old identity may authorize a binding. Display titles and
channel/message rows omit the topic and cannot establish it. Missing or
ambiguous evidence blocks conversion and leaves source history intact.

No production Journal was accessed. This lane does not deliver or certify the
finite artifact, a release, PostgreSQL acceptance, browser acceptance, full
product CI, or the independent deletion audits. The lead owns integrated
acceptance and artifact coordination.

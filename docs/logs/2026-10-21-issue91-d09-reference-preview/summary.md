# D09 exact reference previews

Reference attachment now binds selected positions to the bounded actual records
shown in preview/history_read. ExpectedItems is untrusted, validated input;
the existing HistoryService re-reads authorized records and ReferenceService
compares positions, text, authors, provenance and redaction/truncation flags
exactly before copying. Changed or missing previews fail without a destination
copy. Same request/input still replays the existing destination-owned result.

Remove SelectionDigest, reference ExpectedDigest, preview/snapshot digest fields,
context inclusion/marker digest fields, and the matching tool/Face/UI/schema
producers together. The picker submits actual preview records; queue snapshots
keep them immutable. Retired digest wire inputs are rejected, with no fallback.
Historical Journal JSON remains unchanged; current typed snapshots ignore extra
retired fields and preserve IDs/captured text, copies and lifecycle behavior.

Source authority is checked before private visibility metadata is read. Existing
ContinuityMutation now also carries the same source expectations already owned
by admission. Its transaction locks sessions before the owner run, rechecks
source existence/truncation revision and refuses deletion/rewind races. Saved
receipt replay does not require the old source. No new store or runner is added.

A real PostgreSQL lock regression exposed parent-first subtree deletion versus
canonical reference session locks. Deletion now discovers, canonically locks and
rechecks membership before writes; a changed tree conflicts without partial
deletion. Reverse-order cleanup is preserved. SQLite keeps its serialized owner.

Pinned Eino v0.9.21 ContentBlock.UserInputText and existing native projection
owners were inspected. Imported content remains user data through ContextHost;
the existing immutable ReferenceID supplies that source's version identity.
No orchestration, provider or prompt pipeline is introduced. No SQL is allocated
for this cut;039 remains the integrated lane head, not D14's finalT.

Affected SDK checks also exposed stale Face test mocks; only those fixtures are
updated. Their missing continuous SDK typecheck gate remains in the existing
TODO record. Durable presentation/transfer closure is the remaining D09 work.
D10/D11 prerequisites, owner-deferred3015 smoke and D14 acceptance stay separate.

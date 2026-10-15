# Issue58 S2 Task1: trusted subject identity and interaction metadata

Implemented the core identity, singleton consumer and metadata-only ordering
slice on top of the existing organism Storage and Service.Run. The subject UUID,
authority UUID and canonical existing ACTMEM path are registered once under
`organism/subject/v1`; owner/path or malformed identity mismatches fail closed.
The existing FactoryInput.ResolveRecapBinding callback carries SubjectID at the
actual owner Root, independently of legacy ProfileID.

Trusted composition supplies MainConversationResolver and optional ordinary
SubjectSourceResolver. The same admission transaction pins immutable Session and
Run schema/policy/actual grant snapshots and protects originals before terminal
seal. Main segments share a stable conversation and are excluded from ordinary
Session listings. Ordinary source contribution grants no resident LIFE/read
rights. Retained deletion is rejected before runtime cancellation/tombstoning and
at the low-level storage boundary; authorized archival preserves source reads.

The C2 index stores only identities, sequence, digest and actual terminal outcome.
Sequence allocation and insertion share a subject-serialized transaction;
replay verifies original metadata and evidence. CaptureSubjectHistory returns the
single immutable pagination ceiling; through=0 means the empty prefix. Exact
text-only Journal/projection/user/attachment evidence can seal, including the real
Service.Run fixture. Incomplete, shortened, excluded or mismatched evidence cannot
consume sequence. Original bodies remain messages/Journal/artifacts.

Real concurrent first registration exposed SQLite's old read-then-write CAS.
Snapshot.Put now uses atomic SQL CAS. Connection initialization sets a short 25ms
busy wait; one bounded context-aware helper retries only typed BUSY/LOCKED for
atomic Snapshot CAS and the new subject-enabled transactions. Ordinary nil-Subject
admission bypasses retry. An initially proposed 5000ms driver timeout demonstrably
ignored a 50ms deadline for 5.0196s; the final cancellation test returned in 56.7ms.
The small shared subjectsql package keeps the two SQL backends on one transaction
and evidence-validation implementation, with paired embedded migration 044.

Not delivered: #66 producer/sidebar, resident Recap injection or summarization,
S2 Task2 lossless tool/steering/partial-delta repair or automatic seal observer,
second Store/database/transcript, legacy record migration or public LIFE toggles.
Eligible direct shell fails explicitly before allocation/execution until Task2
provides atomic subject admission and exact evidence; ordinary unshared shell and
NoContext semantics are retained. This is not a final LIFE tool restriction.

Readiness: this is an implemented Task1 increment, not complete Issue58 or
installed LIFE integration. Real PostgreSQL execution and centralized artifact
CI remain pending; skipped server tests are not PostgreSQL acceptance evidence.

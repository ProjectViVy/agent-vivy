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


## Task2: exact originals, sealing and retained archive

This increment replaces Task1's temporary shell rejection and text-only evidence boundary. Authorized ordinary sources and resident LIFE use the existing ToolAdapter / operation coordinator / Journal and backend BlobStore. Exact model-supplied arguments, actual raw result and error are persisted before compaction or soft-error conversion can reach the model. Versioned receipt references commit artifact integrity, original result/error hashes and lengths, ownership and the replay projection. Missing/corrupt artifacts or save failure remain explicit errors/uncertainty and cannot execute a tool twice or consume an index sequence. Ordinary unshared tool results remain opaque and retain their existing strategy.

Governed shell-first and subsequent main/ordinary shell admissions atomically pin the same subject/schema/policy/grant and retention protection without a synthetic user row. NoContext runs remain unpinned/unindexed. A currently trusted singleton can register after only prior governed NoContext shell activity, never after ordinary chat or normal shell activity. Exact pending-shell arguments survive approval cancellation/state cleanup.

Canonical readers repair deterministic Journal projections, verify completion hashes, expand owned original tool artifacts, retain steering/attachments/file snapshots, and label partial terminal output. New seals cannot advance over known terminal unsealed evidence gaps; repair resumes the same contiguous metadata index. Current Host grants are rechecked per source/read. Rewind disposition derives from existing markers separately from immutable original digests. Authorized archive preserves addressability, artifacts and deletion protection.

Paired append-only migration 045 makes shell's admitted user-message FK nullable and adds metadata-only exact copy-to-parent links in the existing atomic fork/clone transaction. New equal-content originals keep distinct IDs across nested copies/restart/archive. Historical copies without a link are explicitly unavailable; no content matching/backfill or new body authority exists. Parent coordinates have no deletion-restricting FK, preserving ordinary unprotected deletion.

Not delivered: S3 head/compaction/summarization, UI/#66 producer, new LIFE modes, a second Store/database/transcript/engine, dependency changes, push/merge/deploy, or tenant-data access. Required non-race gates pass. The required race gate remains blocked by the confirmed pinned Eino v0.9.13 enhanced ToolsNode converter race, also reproduced by the existing unretained Plan scenario. PostgreSQL server acceptance remains unavailable. This is a reviewable Task2 implementation, not S3/release acceptance.

## Official Eino native-fix remediation

The dependency increment after Task2 pins official Eino v0.9.20 with normal module
sums. It removes the enhanced stream converter race natively and preserves upstream
canceled-stream checkpoint handling. EinoExt, Laputa 4c7 pins, local replacements,
all original tests and concurrent tool capabilities remain unchanged. Two test-only
current-pin constants now reflect go.mod. The existing persona authority fixture
now supplies the actual isolated SQLite SnapshotStore required by S2's factory
identity dependency; every original assertion remains. No production adapter or
S2 behavior changed. Source candidate: `d0f1e487baa3ba74d4b0ec7471a7547592b2809c`.
See [the full remediation report](eino-upgrade-report.md) for native APIs, exact
RED/GREEN evidence and qualified remaining product acceptance.


## Task2 fix round1 — accepted I1/I2

Corrected PostgreSQL's two new exact-copy statements to native positional parameters inside its existing raw SQL transaction. Retained native Question answers now use the existing coordinator's exact request/result receipt before enhanced projection, while Question remains the decision authority and Eino remains the interrupt/resume engine. No ask_user invocation, new state, weakened evidence validator or ordinary-session retention was introduced. The new actual suspend/reopen/recover/resume regression proves a 38,043-byte answer (including a literal untrusted-output header) expands exactly, survives model shortening with its reference, seals once and permits the next domain interaction; ordinary unshared control remains excluded.

All required fix-round1 gates pass at the already upgraded official Eino v0.9.20. Full details and limitations are appended to task-2-report.md; exact source/output hashes and skips are in task-2-fix-1-verification.json. PostgreSQL server acceptance and centralized S6 product CI remain pending.

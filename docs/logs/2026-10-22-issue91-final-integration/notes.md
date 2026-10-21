# Integration ledger

Base: stable cd0d4d3f. Data donor: 50d63a32. Owner authorized sequential D09/D12, D10/D11 and D14 on 2026-10-22.

Pre-flight: D09/D12 exact private file/continuity/presentation inputs and final metadata/BML2 consume D03 authority; stable D08 admitted snapshots/checkpoints and D06 Generation identity share the migration owner. Integrate these contracts in one write lane.

Ruling: preserve stable SQL 001-038 byte-for-byte. Donor D09 becomes 039 file approval before-state, 040 continuity intended-input, 041 deliverable presentations. Git recognized the donor D06/D08 rename; restore stable 037 and retain 038 after canonical SQL equality. Donor scratch stores are not supported formats. Cost if wrong: intermediate stores must be refused, never renumbered in place.

Ruling: retain D12 final metadata/shape owner without checksum readers. Stable old-metadata 036-038 need finite copied conversion; other same-number lineages fail name/shape checks. Cost if wrong: source classification; no-write tests gate it.

Browser integration exposed an existing D09 ownership error: unbound sessions
use a separate private workspace per Run, but a read marker identified only
the session and relative path. The existing private file_reads owner now keys
the actual resolved root plus relative path; file_versions and user history
retain relative paths. No schema or second store is introduced. Old rootless
markers cannot prove ownership and require a fresh present/absent read.
Confirmed missing reads record absence only after the normal confined path
resolver verifies it. Different Runs sharing one selected root still share
the exact-byte precondition. Regression tests use the real SQLite recorder.

Independent correction review identified and cleared two further issues:
storage errors must omit the private scoped key while preserving their cause,
and confined missing reads must resolve the nearest existing ancestor when
several parent directories are absent. Dangling links, non-directory and
permission failures remain errors; reconstructed paths still undergo root
confinement and read-only checks. Both findings reproduced RED before fixes.

The continuity browser harness now uses the authenticated product RPC
singleton, initializes its disposable Persona, waits for actual session
selection and submits through the enabled button. Its second Run intentionally
has a separate private root; the assertion checks the first root stays intact.
No live provider or production database participates in these tests.

Upstream integration identity, verified through official commit metadata:
Laputa merge9acc999301d5 (2026-10-21T09:00:26Z) has the exact reviewed PR13
head tree; INOFY merge6f1397e417fc (2026-10-21T08:46:05Z) has the exact
reviewed PR17 head tree. D10/D11 will consume those immutable merge commits.

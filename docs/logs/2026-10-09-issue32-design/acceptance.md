# Reviewing the Issue #32 design package

This record describes document acceptance, not installed-product acceptance.
The plans are reviewable drafts; the owner's design acceptance is not inferred
from their creation or from a passing document check.

1. Open the [index](../../superpowers/plans/issue32-remediation/index.md).
   Confirm eight phase links, all 28 finding dispositions, and that none is
   marked fixed. R2 should link the explicit superseding owner decision.
2. Read the [design](../../superpowers/specs/2026-10-09-issue32-remediation-design.md).
   Confirm source identities, architecture/Eino boundaries, persistence and
   compatibility contracts, dependencies, and engineering versus product gates.
3. Open any phase. Its tasks should identify files, concrete interfaces,
   prerequisite work, meaningful failure/regression scenarios, verification
   commands, an independently reviewable commit and the phase exit condition.
   All implementation checkboxes remain unchecked.
4. Follow C1/C2 into P2: original-version CAS, persisted admission identity,
   restart reconciliation, and proof of safe retry must prevent duplicate
   external effects. An evidence lookup failure must not authorize replay.
5. Follow H1-H3 through P1 into P7: unauthorized native method calls fail before
   host use, changed active-host paths trigger CI, missing native subcases block
   promotion, and exact accepted package hashes survive signing and publication.
   Verify the existing tracked frontend lock against the canonical source-lock
   digest and exercise its frozen install in the H2 gate.
6. Follow W2/W7/W4/W5/W6 into P3: stale create intent conflicts, ambiguous starts
   retain the exact retry request, keyset paging preserves timestamp ties,
   terminal cursors stop loading, old query responses cannot overwrite refresh,
   and native/engine lifecycle status remains truthful.
7. Follow R1/R3/R4 into P5: reading is bounded across partial oversized physical
   lines and replacements, and failed mandatory model settlement reaches the
   terminal caller path and blocks unsafe cognitive replay. Automatic redaction
   is not reintroduced under a different finding.
8. Read P7's final pin/lock/artifact and conditional transition gates. Windows,
   real voice, PostgreSQL, owner acceptance and missing archive prerequisites
   remain explicit pending outcomes until actually executed. Retiring source
   invalidates the old candidate and requires new exact acceptance.

Review feedback should cite the owning phase/task and unresolved contract.
Advance design/implementation status only with actual review or execution
evidence. The current package is sufficient to review and select the next work
package; it does not claim product fixes, passing product CI or release readiness.

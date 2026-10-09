# Issue #32 work-package design delivery

Date: 2026-10-09. Status: **DESIGN DOCUMENTED; PRODUCT IMPLEMENTATION OPEN.**

The owner requested Superpowers work-package design with a detailed document
for every phase after the Issue #32 audit and preanalysis. The deliverable is
one [written design](../../superpowers/specs/2026-10-09-issue32-remediation-design.md),
one [coverage/dependency index](../../superpowers/plans/issue32-remediation/index.md),
and eight phase plans P0-P7, subdivided into 26 tasks and 150 unchecked execution
steps. The index is the authoritative finding status
board; phase plans own task interfaces and execution steps.

The package covers 28 original findings: 27 planned repairs and R2 explicitly
superseded by the owner's #40 decision and merged PR #42. No finding is marked
fixed by this documentation delivery. The open backlog points to the package,
and the superseded board records the R2 disposition.

Each phase defines prerequisites, scoped files, interface ownership, failure
behavior, five review risks, regression expectations, commands and exit gates.
Shared Service/App/state edits are sequenced. Eino checks and existing SDK
capabilities bound custom implementation. Final source-bound evidence precedes
DIVA repinning; native acceptance binds the exact retained candidate bytes.

Source inspection also identified a canonical-test prerequisite: the reviewed
DIVA tree tracks package-lock.json but not the pnpm-lock.yaml that
scripts/build-desktop.py reads unconditionally. P1.0 restores the frozen input
from the existing dependency resolution before behavioral tests. This is a
source-confirmed setup gap, not an additional numbered issue finding or a
claim that a native build ran.

Reviewed main identities are VIVY
017ec8cc37970b291e04c619990aed00d5403116 and DIVA
518a33ef09858ee1bb190579dd7529aceaa15dd6. The design records full dependency,
archive and acceptance provenance requirements. These planning identities must
be rechecked when implementation starts.

All durable records are English under repository instructions. Only this
documentation lane writes the repository; independent plan inputs were drafted
in workspace scratch and integrated by the lead. Product code, dependencies,
databases, consumer pins, archive tags and release assets are unchanged. No
push, PR, publication, archive action, or product acceptance occurred. There is
no release.md because this is a planning delivery.

See [verification](verification.md) for actual documentary checks and
[acceptance](acceptance.md) for reviewing the package. Implementation remains
the next scope; the first blocking tranche is P1.0-P1.3 and P2.1-P2.2.

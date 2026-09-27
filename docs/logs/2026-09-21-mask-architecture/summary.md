# Mask architecture design delivery

Detailed design for issue #43 is written against main `5253f77` in
[the specification](../../superpowers/specs/2026-09-21-mask-subsystem-design.md).
The design consolidates both issue addenda and specifies the optional T1 boundary,
backend catalog/selection, atomic admission, immutable prompt capture, native Eino
integration, Markdown ownership, UI contribution, migration and failure semantics.

No product code, generated Assembly, project instructions, Port support state,
GitHub issue or remote branch was changed. This is a design-review deliverable,
not completed implementation. The subsequent planning refinement is recorded below. The local
TODO board links it as P1, awaiting design review. No separate release or rollback
artifact is needed for documentation; release/data compatibility is in the spec.

## Planning refinement

The maintainer subsequently requested a detailed document collection. Added the
[implementation package](../../superpowers/plans/masks/index.md): one index,
shared MASK-C1 contracts, four Story plans and integrated acceptance. The existing
architecture remains the decision source; the package owns detailed execution
contracts and the index owns readiness. All Stories are Planned, not implemented.
Corrected the internal Source Catalog path to `internal/modules/defaults/catalog.go`;
`sdk/internal/catalog_v1.go` is localization parsing, not module registration.

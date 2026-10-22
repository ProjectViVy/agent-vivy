# Acceptance

- Publishing a workflow shows the committed revision without an empty digest label. Subsequent revision reads use workflow/revision/program identity.
- Persona reads and reviews retain existing revision-based saves and expose the actual expected document state without stale hash DTO fields. Existing persona editing/review behavior remains covered by the focused UI suite.
- Session-search demo results carry the real text/source-position fields. Retired approval/digest labels cannot return through the edited type or locale contracts.
- Current deliverable metadata validates against the published closed event schema; a retired sha256 property is rejected.
- SDK-generated UI typechecking and the focused persona/workflow/demo tests pass. The integration lane regenerates assembly from the corrected catalogs and conducts final product acceptance.

Rollback is a focused Git revert. There is no data conversion or tenant-state rollback in this correction.

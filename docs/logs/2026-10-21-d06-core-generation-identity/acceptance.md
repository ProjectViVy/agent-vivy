# Acceptance

On a fresh disposable SQLite or PostgreSQL store, create a Generation through
the existing lifecycle API using its Recipe and optional ID/parent/source ref.
No artifact hash input is needed, and no artifact hash is returned or persisted.
Reopen and verify Generation identity, Recipe, parent and lifecycle state.
Human/evaluation requirements and first-promotion conflict behavior still hold.
In the UI Generation list, ID and phase remain visible; the hash line is absent.

On a quiescent supported baseline035 copy, the final runtime must first refuse
with upgrade_required and leave source metadata and rows unchanged. The finite
ApplyManifest copy-conversion seam reaches037 while retaining Generation,
evaluation, promotion and event history. Verify reopen with the final runtime.
Drifted schemas, retained artifact columns and unlisted interim036 sources must
fail before runtime writes. A failed037 transaction must keep its old target
column/history and omit037 metadata until a successful explicit retry.

Complete upgrade publication, configuration cutover, integrated product checks
and D01/D06 Story acceptance remain with D14/the lead; this record is subsystem
evidence and does not move the shared Story index.

# BML pristine-file reopen after cancelled initialization

Memory warmup now treats an inspector-proven pristine SQLite file as absent
authority. A cancelled first history append can roll back its schema transaction
while leaving the physical file. Previously the read path ignored the
inspector's pristine result and queried `schema_meta`, so composition failed
with a missing-table error on restart.

The existing private `openExistingMutable` owner now returns no store for that
one recognized state. Warmup and list projections perform no initialization,
file deletion or reset. The next normal write initializes through the existing
`writableStore` path. Initialized stores still validate identity and shape;
unknown nonempty stores and schema1 stores still refuse without mutation.

The regression creates a real SQLite schema transaction, cancels and rolls it
back, verifies the pristine state, then exercises repeated warmup/reopen,
byte-identical reads, subsequent initialization and durable history replay.
Separate cases cover unknown nonempty and schema1 refusal on reads and writes.

The change stays within BML Home and its tests. It adds no schema, migration,
Core Storage change, dependency, retry loop, runtime orchestration or old-format
reader. BML schema2/artifact2.0.0 and final SQL target047 remain unchanged. No
production data was accessed. No release was produced; final runtime pin
resealing, fresh independent reviews and integrated App acceptance belong to
the parent delivery.

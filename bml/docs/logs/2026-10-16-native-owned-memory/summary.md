# Native BML producer

Schema 2 adds exact durable ownership and immutable native operation outcomes to
the existing Store.putInner transaction. New bound Home/Store methods preserve
native records, sources, IDs, CAS, FTS ranks and exact-version source export while
legacy reads/writes/L1/GC cannot consume or alter the new owned material. Old
unknown ownership is not promoted.

Fresh/real schema-1 Home upgrade, read-only capability limits, backup validation,
staged restore and rejected bound-state identity migration are documented in
the [native contract](../../owned-memory.md). Record JSON and dependencies stay
unchanged; the unused governed-apply schema is not revived.

This is only the independent library producer. No App, Garden adapter, runtime,
observer, UI, Recipe, generated metadata, Laputa source or dependency pin changed.
Actual source eligibility/C2 grants and both-direction actual App privacy tests
remain prerequisites for later consumer activation.

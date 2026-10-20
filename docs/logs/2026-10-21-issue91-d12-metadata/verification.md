# Verification

Observed RED to GREEN regressions:

- Fresh metadata retained checksum; final columns test now passes.
- Failed pristine bootstrap retained schema/earlier tables; atomic rollback
  now leaves no metadata or application tables.
- Nullable/keyless metadata, views replacing tables, missing session columns
  and missing operation primary key were admitted; read-only guards now reject.
- All old metadata prefixes001..036 classify without mutation; unknown old036
  notebook lineage cannot impersonate D03's private-binding target.

Native migration suite passes after these fixes. Full native SQLite owner run,
independent Linux wiring/kernel/real PostgreSQL, finite fragment/target bridge
and whole-branch review are pending; none are inferred from compilation.

Native conformance reproduction after the D03 merge failed because native Vite
node_modules are absent. The merged artifact temporarily retains prior lane
evidence; regenerate with the actual Linux producer before candidate acceptance.

Historical source normalization/preservation assertions move with the frozen
finite artifact. Runtime Postgres CI requires TestMigrateRefusesV14BeforeWrites
instead of the removed in-place legacy-normalization case; real converter PG
checks remain separately required. No released SQL rewrite or production read.

The first native SQLite owner run exposed two test fixtures that declared final
SQLite metadata with BIGINT rather than the owned INTEGER primary key. The
strict guard correctly refused them. Corrected only those fixture declarations
and registered immediate raw-handle cleanup. Both focused owner cases then pass
(15.352s); the earlier whole native SQLite run otherwise passed until these two
fixture failures. Independent Linux whole-owner verification remains required.

Real PostgreSQL first attempt fails on reopen, exposing a catalog-derived shape
bug: BIGSERIAL identity columns were omitted by the recognizer. The independent
review found the same cause. A deterministic serial-column/key test fails first;
normalizing that owned SQL spelling to PostgreSQL's reported BIGINT fixes it.
Full fresh/reopen PostgreSQL rerun remains required.

After BIGSERIAL correction b4bc941b, real `just test-postgres` passes118 executed
cases with zero skips. That candidate predates the following review fixes, so
it does not accept the later exact candidate.

Independent review reproduced missing unique history/request ownership and the
exported ApplyManifest format-barrier bypass. Both fail first; final exported
entries now share admission, and only package-private test helpers construct
synthetic prefixes. Required UNIQUE constraints and partial indexes derive from
the same embedded SQL and are inspected in both dialects. Exact SQL literals
remain distinct. Native migration suite passes after these corrections.

Linux conformance reproduction exposed one remaining removed-field reference
in the App Work fixture. Replaced its obsolete in-place-upgrade expectation
with final converted-store open/Work coverage and a separate byte-preserving
App refusal case for old23 metadata. No checksum producer is restored.

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

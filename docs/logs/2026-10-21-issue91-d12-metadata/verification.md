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

At a1d42fd8, independent Linux App converted-open and old23 byte-preserving
refusal both pass (0.169s). All23 actual conformance suites pass (77.428s),
regenerating conformance_results.json through its owner. The failed native
attempt and earlier compile failure above are superseded, not erased.

Finite artifact correction is1a8e7506acee43f1ed3afc69f877ebb068f5ccf7 on the
separate artifact branch, never merged into this runtime. Its initial3193abdc
PG suite passed159 cases with zero skips; final shape-review fixes and actual
target bridge are undergoing fresh real-PG execution.

Exact runtime1a4f578e independent Linux gates: wiring16 Node+105 Go passed with
zero skips; kernel vet+1756 executed Go cases passed, minimal Pack/Inspect/boot1
passed separately. The unchanged optional ripgrep case skipped (not passing
evidence). Real PostgreSQL118 executed cases passed with zero skips. During
initial gate capture one stale log-only file remained after mixed index
alignment; it was restored to HEAD before completion, with tested source and
pinned Laputa unchanged. No product-code worktree drift is accepted.

## Final finite fragment and target bridge

Final artifact:8e63682e6bf42e786a4a626c85079ea87f5c8fb8, separate from runtime.
Independent review found incomplete canonical/sealed shape checks; regressions
failed first and frozen SQL-derived columns/types/keys/uniqueness now reject
before writes. Real PG later exposed the same gap in compressed legacy core
shapes; six cases failed first. Frozen V14 core plus only already-recognized
optional feature DDL now validates that class too, preserving missing020.

- Native ordinary module suite: pass114.814s; ordinary/tagged vet pass.
- Real PG source matrix: `go test -tags postgres -run TestPostgresFinite
  -count=1 -timeout=5m`, pass198.866s,78 leaf cases:35 canonical,8 released017,
  1 originalV14,22 compressed/missing020,6 canonical structure,6 legacy partial.
- Changed legacy6 + malformed-sealed3 focused real-PG cases: pass17.399s;
  wrong-type fixture uses version INTEGER/applied_at BIGINT so rejection is
  tested after setup rather than overflowing timestamps. No skipped cases.
- That real-PG run used native Go1.26.4 against its own loopback postgres:16
  container in WSL Docker. The dedicated container was removed. It is separate
  from the independent Linux runtime118-case gate and not a skipped DSN lane.
- Actual final fragment -> target runtime bridge: both SQLite and PostgreSQL
  pass2.167s from the independent Linux checkout. It executes normalized23->035,
  D03's immutable036 owner fragment, metadata seal, actual Apply/open and reopen;
  preserves session content and original applied_at. Temporary modfile/test
  files and task-created PG container were removed. Executed source is archived
  as target-runtime-bridge_test.go.txt; no converter dependency enters runtime.

No complete targetT number, D14 conversion settlement, full-product release,
D09–D11 acceptance or3015 smoke is claimed. The standard split ports currently
belong to D06; an explicit scheduling question remains pending. The earlier
alternative-port BML/MEMRULES behavior evidence remains a scoped observation.


Owner steering (2026-10-21): defer standard3015 smoke and continue to the next
stage. The prior port-scheduling question is answered; no stop/switch is needed.
D06 subsequently completed and stopped its own split servers. No3015 pass or
D12 Story acceptance is inferred from that change.

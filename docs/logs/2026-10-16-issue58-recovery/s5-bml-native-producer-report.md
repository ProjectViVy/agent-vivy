# S5 native BML producer checkpoint

## Immutable source and scope

- Local source SHA: `92048bccda52875a798ade1bccc4e44823a564fb`.
- Tree: `a9532ce41a286334e0462b7401bce786e89e65c0`.
- Parent/baseline: `0baf8d3b6cbde4f9589cbc8f15c4b4d73d3820c6`.
- Branch: `feat/issue-58-recovery-20261016`.
- Author/committer: 大湿 <57738020+mastwet@users.noreply.github.com>.
- Exactly 19 changed files, all under bml/: 14 Go (seven production, seven test) and five documentation files. BML worktree clean; index empty at freeze. Root's unrelated untracked evidence/helper files were not staged or changed.

This delivers only the released independent native library producer. App, private factory, Garden adapter, runtime, observer, UI, Recipe/generated metadata, Laputa source and dependency files were not changed. Source/index were returned to root immediately after the checkpoint. No push, main merge, deployment, consumer activation or actual tenant-data migration occurred. Root owns independent review/publication and any later consumer release.

## Native contract and schema

Tracked contract: `bml/docs/owned-memory.md`; English iteration records: `bml/docs/logs/2026-10-16-native-owned-memory/{summary,verification,acceptance}.md`.

Schema 2 retains legacy Record JSON and native FTS. In the same SQLite file it adds memory_ownership (exact durable record binding) and memory_operations (global operation ID, full binding, caller digest, canonical request fingerprint and immutable native outcome). It does not use the unused governed memory_apply_journal. Supported schema-1 writable upgrade creates the two tables and advances schema_meta atomically, including the real Home.existingStore/openExistingMutable startup path. Old row IDs/revisions/counters/body/source/FTS remain legacy; no ownership or historical outcome is inferred/backfilled. Schema-1 reserved-name collisions are rejected; schema 2 does not silently recreate a missing native receipt table. Unsupported versions/foreign database identities fail closed.

OwnershipBinding is the exact subject/domain/scope-kind/logical-workspace/destination tuple. Record.Scope independently retains native tenant/physical database workspace/session. The trusted host supplies this binding after its own authorization/source qualification; the library neither equates ProfileID to subject nor provides C2 grants. Native DTOs contain no Garden/Vivy root imports.

`Store.MutateBound` feeds the existing putInner transaction. Complete native Record/provenance/evidence and explicit optional CAS shape enter a canonical SHA256 fingerprint, independently of the caller digest. All nested raw Go strings are rejected if invalid UTF-8 before marshaling can normalize them; noncanonical Go enum values are also rejected. Exact validation precedes immutable replay lookup; replay precedes current row/store CAS/capacity. Existing store/record/target guards, supersedes, FTS and counters share Commit with ownership/outcome insertion. The pinned driver's supported immediate transaction mode serializes writers across Store handles.

MutationOutcome retains the full binding/destination, operation/digest, actual native record ID/revision and store revision, target ID and original canonical/index disposition. Tombstone outcome identifies its new native marker/revision plus target; it does not fabricate advancement of the old target row. Receipt bodies do not duplicate record text. Lookup/replay after update, tombstone and reopen reads the immutable stored result, never today's row. Changed source/body/CAS/digest/owner/destination cannot reuse an operation. Native missing result remains absent; schema-1 lookup is explicitly unavailable.

`Store.GetBound/SearchBound` and `Home.ExportBoundSource` use exact stored ownership and visible expected revision; stale/tombstoned/superseded/unknown legacy material is unavailable. Search reuses native FTS escaping/rank, adding ownership before ORDER BY/LIMIT. `Home.ExportSource` is legacy/unbound only. Legacy Store/Home Get/List/Search/CRUD/import/L1 and session GC exclude/reject owned material; bound reads exclude old unknown rows. The same mutation transaction rejects foreign targets/suppression and updates or repeated tombstones against removed targets. Home owns L1 refresh; a postcommit refresh failure leaves the native outcome available for retry.

## Read-only, backup and migration limits

OpenExisting never upgrades native data. Schema-1 read-only handles retain legacy reads with all bound capabilities off. Read-only schema 2 has bound search/export/lookup, with mutation and restore denied. OpenExistingCanonical is the existing explicitly mutable identity-upgrade constructor and is not the read-only OpenExisting API.

Backup remains SQLite VACUUM INTO, covering ownership/outcomes consistently. Restore and identity-migration/rollback backup validation use the read-only path, preserving backup bytes. Restore upgrades a staged copied database and replaces the closed live file only after successful upgrade/close; injected upgrade failure preserves both live database and backup. The receiver still closes on every Restore path. Explicit backup restore rewinds the complete snapshot, not a concurrent merge or a promise to retain operations newer than that snapshot; callers own the maintenance lifecycle exclusively.

Root accepted the narrow identity limit: if ownership OR retained operation state exists, canonical workspace identity migration/rollback rejects before metadata/record/backup/manifest writes. Rewriting physical Record.Scope would invalidate immutable fingerprints. This includes history-only state after records disappear. The existing unbound legacy migration/rollback remains supported/tested; Home's fixed machine identity does not use that migration. No bound-state conversion/re-keying/migration is invented.

## Final executed gates

Go 1.26.4 linux/amd64. Native and root module-list evidence independently confirms SQLite v1.56.0 versus root MVS v1.59.0, with unchanged dependency bytes. Every test/vet/list invocation used the unchanged local-only runner, offline dependencies, credential-name stripping, disposable native fixtures and a non-forwarding external deny proxy. All recorded zero external requests/forwarding. Root prepared exact dependencies separately through official proxy+sumdb; its setup records are copied unchanged.

| Final label (prefix issue58-bml-) | Test pass/fail/skip | Package pass/fail | Exit | Seconds |
| --- | --- | --- | --- | --- |
| checkpoint-native-normal | 155/0/0 | 1/0 | 0 | 1.424 |
| checkpoint-native-race | 155/0/0 | 1/0 | 0 | 7.354 |
| checkpoint-root-normal | 155/0/0 | 1/0 | 0 | 1.730 |
| checkpoint-vet | n/a | n/a | 0 | 0.265 |

Counts include Go parent tests and subtests, with package outcomes separate. No race diagnostic occurred. Gofmt and diff/cached whitespace checks passed. Final execution preceded commit; runner metadata names baseline HEAD. `tested-source-hashes.json` and `source-hashes.json` prove all 14 tested final Go bytes and all 19 committed file bytes equal the fixed checkpoint; dependency bytes equal baseline.

Actual native fixtures cover fresh/real legacy-Home upgrade, readonly snapshots/capabilities, unsupported native shapes, failed-upgrade rollback, native CRUD/CAS/FTS/exact export, full binding and canonical fingerprint conflicts, unknown legacy rejection, privacy before LIMIT/L1, same-operation and distinct-operation concurrency across two handles, one-winner update CAS, actual receipt-insert and deferred-constraint Commit failure, tombstone/reopen outcomes, source visibility, backup/restore and fault preservation, historical identity rejection, GC isolation and Home projection-refresh retry.

## Preserved RED and intermediate evidence

All raw invocations, including failures, are retained beside this report, with exact runner metadata and original Go JSON streams. No assertion/deny rule/runner was weakened. Setup failure is not counted as semantic RED. Missing API declaration build RED was followed by an explicit unavailable shell semantic RED before implementation; that shell is absent in the fixed source. An external schema-step overlay only omitted the two not-yet-declared new bound test files; its exact mapping and empty test file are retained. It did not change an existing assertion or production file.

Five intermediate full-suite failures were old fresh-schema-1 expected values; their original output remains. The tests now assert fresh v2 while separate actual released-v1 fixtures preserve exact old/read-only/fault assertions. Maintenance/fence REDs demonstrated real backup mutation, bound identity rewriting, owned-row GC, deleted-target CAS reuse, foreign suppression and damaged schema recreation. Raw enum REDs include parent/subtest/cascaded effects on the shared test store; their fail counts are not claims of distinct defects. A final explicit read-only Restore RED showed the inherited filesystem route could replace a read-only receiver's DB; its local guard was added and all final gates reran with 155 tests.

### Complete runner invocation ledger

- `issue58-bml-baseline-schema-expectations`: exit 1, 0.967s, counts `{"package_fail": 1, "test_fail": 5, "test_pass": 123}`; `go test -json -p=1 ./... -count=1 -timeout=4m`.
- `issue58-bml-bound-api-red`: exit 1, 0.164s, counts `{"package_fail": 1}`; `go test -json -p=1 ./... -run '^TestIssue58Bound' -count=1 -timeout=3m`.
- `issue58-bml-bound-fences-red`: exit 1, 1.069s, counts `{"package_fail": 1, "test_fail": 5, "test_pass": 2}`; `go test -json -p=1 ./... -run '^TestIssue58Bound(Deleted|Cannot|Distinct|Schema|Restore)' -count=1 -timeout=3m`.
- `issue58-bml-bound-green`: exit 0, 1.085s, counts `{"package_pass": 1, "test_pass": 31}`; `go test -json -p=1 ./... -run '^TestIssue58(Bound|Schema)' -count=1 -timeout=3m`.
- `issue58-bml-bound-red`: exit 1, 0.872s, counts `{"package_fail": 1, "test_fail": 4, "test_pass": 12}`; `go test -json -p=1 ./... -run '^TestIssue58Bound' -count=1 -timeout=3m`.
- `issue58-bml-checkpoint-native-normal`: exit 0, 1.424s, counts `{"package_pass": 1, "test_pass": 155}`; `go test -json -p=1 ./... -count=1 -timeout=4m`.
- `issue58-bml-checkpoint-native-race`: exit 0, 7.354s, counts `{"package_pass": 1, "test_pass": 155}`; `go test -json -race -p=1 ./... -count=1 -timeout=5m`.
- `issue58-bml-checkpoint-root-normal`: exit 0, 1.73s, counts `{"package_pass": 1, "test_pass": 155}`; `go test -json -mod=readonly -p=1 github.com/ProjectViVy/agent-vivy/bml -count=1 -timeout=4m`.
- `issue58-bml-checkpoint-vet`: exit 0, 0.265s, counts `{}`; `go vet -p=1 ./...`.
- `issue58-bml-empty-enum-red`: exit 1, 1.022s, counts `{"package_fail": 1, "test_fail": 5, "test_pass": 6}`; `go test -json -p=1 ./... -run '^TestIssue58Bound(RawEnums|HomeProjection)' -count=1 -timeout=3m`.
- `issue58-bml-fences-green`: exit 0, 0.972s, counts `{"package_pass": 1, "test_pass": 13}`; `go test -json -p=1 ./... -run '^TestIssue58Bound(Deleted|Cannot|Distinct|Schema|Restore|RawEnums)' -count=1 -timeout=3m`.
- `issue58-bml-final-native-normal`: exit 0, 1.626s, counts `{"package_pass": 1, "test_pass": 154}`; `go test -json -p=1 ./... -count=1 -timeout=4m`.
- `issue58-bml-final-native-race`: exit 0, 30.479s, counts `{"package_pass": 1, "test_pass": 154}`; `go test -json -race -p=1 ./... -count=1 -timeout=5m`.
- `issue58-bml-maintenance-green`: exit 0, 0.967s, counts `{"package_pass": 1, "test_pass": 7}`; `go test -json -p=1 ./... -run '^TestIssue58Bound(Backup|Identity|GC)' -count=1 -timeout=3m`.
- `issue58-bml-maintenance-red`: exit 1, 0.918s, counts `{"package_fail": 1, "test_fail": 7}`; `go test -json -p=1 ./... -run '^TestIssue58Bound(Backup|Identity|GC)' -count=1 -timeout=3m`.
- `issue58-bml-native-module`: exit 0, 0.008s, counts `{}`; `go list -m -json modernc.org/sqlite`.
- `issue58-bml-native-normal`: exit 0, 1.574s, counts `{"package_pass": 1, "test_pass": 153}`; `go test -json -p=1 ./... -count=1 -timeout=4m`.
- `issue58-bml-native-vet`: exit 0, 7.941s, counts `{}`; `go vet -p=1 ./...`.
- `issue58-bml-raw-enum-red`: exit 1, 1.022s, counts `{"package_fail": 1, "test_fail": 6}`; `go test -json -p=1 ./... -run '^TestIssue58BoundRawEnums' -count=1 -timeout=3m`.
- `issue58-bml-readonly-restore-red`: exit 1, 0.968s, counts `{"package_fail": 1, "test_fail": 1}`; `go test -json -p=1 ./... -run '^TestIssue58ReadOnlyHandle' -count=1 -timeout=3m`.
- `issue58-bml-root-module`: exit 0, 0.008s, counts `{}`; `go list -m -json modernc.org/sqlite`.
- `issue58-bml-root-mvs-normal`: exit 0, 1.809s, counts `{"package_pass": 1, "test_pass": 154}`; `go test -json -mod=readonly -p=1 github.com/ProjectViVy/agent-vivy/bml -count=1 -timeout=4m`.
- `issue58-bml-schema-green`: exit 0, 0.869s, counts `{"package_pass": 1, "test_pass": 4}`; `go test -json -p=1 -overlay=/tmp/issue58-bml-schema-overlay.json ./... -run '^TestIssue58Schema' -count=1 -timeout=3m`.
- `issue58-bml-schema-red`: exit 1, 0.032s, counts `{"package_fail": 1}`; `go test -json -p=1 ./... -run '^TestIssue58Schema' -count=1 -timeout=3m`.
- `issue58-bml-schema-red2`: exit 1, 15.216s, counts `{"package_fail": 1, "test_fail": 3, "test_pass": 1}`; `go test -json -p=1 -overlay=/tmp/issue58-bml-schema-overlay.json ./... -run '^TestIssue58Schema' -count=1 -timeout=3m`.
- `issue58-bml-v1-shape-red`: exit 1, 0.921s, counts `{"package_fail": 1, "test_fail": 1}`; `go test -json -p=1 ./... -run '^TestIssue58SchemaOneCannot' -count=1 -timeout=3m`.

Each invocation uses `python /tmp/issue58-localonly-runner.py <module-dir> <label>` before the listed command. Native labels use repository bml/; root labels use repository root. `results.json` retains exact argument arrays and metadata; dependency preparation is separate `dependency-setup.{log,json}`. All setup/compile/RED/intermediate records remain; there were no interrupted test attempts.

## Source hashes

SHA256 below matches the fixed committed blob and working bytes. Source manifest also preserves the baseline, fixed parent/tree, dependency hashes and freeze/index status.

| File | SHA256 |
| --- | --- |
| `bml/README.md` | `922efc99ae7625a1fa33c964f4855feb8b4f85cf81cc2dcd28f8dd267787d835` |
| `bml/bound.go` | `9d27547a2ad967c69421ba8cea7830cc0c6ea12f94343fadd034c38b3e985ed3` |
| `bml/bound_failure_test.go` | `14e2a379f3b5c858967486a32d7c0124cac82717e58877b0ebefcba549e83521` |
| `bml/bound_isolation_test.go` | `fdc58ebe2ef47b0c589cc4408701c4482f0f4454239759910e2a05e58cc5cdfd` |
| `bml/bound_maintenance_test.go` | `d0cf59b85f79e7e091309641bce13626ba899a5a5aad031463698af493e1fd35` |
| `bml/bound_mutation_test.go` | `fd73f558aa3c44106bec5ce9275fa2580124153ff5696781634f9a6036b27c80` |
| `bml/bound_read.go` | `80e35047b1b4fa0d29172b9d739b1ce0891f0c268b87024682dc8d49d8bacf3a` |
| `bml/docs/logs/2026-10-16-native-owned-memory/acceptance.md` | `8e0f210120ca0c006f78084b648f881452d0f19754fb42a1759ec69c0fdec3d5` |
| `bml/docs/logs/2026-10-16-native-owned-memory/summary.md` | `ca2d62cd5384c108ed35e052fd805f2e75a17808223ddd8bdc5c4e3674651fe0` |
| `bml/docs/logs/2026-10-16-native-owned-memory/verification.md` | `1367c2359a026e1bbf94d8481ce05418e2f989681c9b3bd4c8311fb68f155111` |
| `bml/docs/owned-memory.md` | `3655dc86ca1255785d1db2f15524ddf81f7f75b26c8dd4c2936c26d0cd45862a` |
| `bml/home.go` | `641fd039cdb1c80332ba1a58463c5effae6af28c4ae925906a36c2d737938c28` |
| `bml/maintenance.go` | `411c99014930f91e49b3be69696e88ffc00a8b00a36691b143337d454fd16a70` |
| `bml/maintenance_test.go` | `a3c520cad365dd899f0cef5d269bf45915c029fb73d304a16304f7c714f36e97` |
| `bml/schema_evolution.go` | `2acbdb209d50a74fc6058a1a319116454af1238772afc01c487ed07b57043e8c` |
| `bml/schema_evolution_test.go` | `d95dbb4e88c15793b7506fd548dd55275d1c7b8df65837b00c106c6097bbf144` |
| `bml/search.go` | `891daaadc37809b861bf1adf09ce7ba887bf4cfbe516c5752c1c3743ad73d6f8` |
| `bml/store.go` | `5216314499130ef49ca56d50ee3ed1840529e135ffb54c9bbe7762b27bc17d16` |
| `bml/store_test.go` | `ff63eea704ca8281e06621add62b4ca0119bcde041827ab84a1ddf15363ab378` |

## Pending acceptance

Actual Windows is **NOT RUN** for this native candidate. Root whole-product CI, Garden backend conformance, actual App-owned lifecycle/factory injection, old raw source qualification, both-direction LIFE/Coding/worker ContextHost isolation, single observer sink, historical core-reader/loaded-pin UI and installed/live-model acceptance remain separate downstream gates. Native storage isolation does not establish those consumer guarantees. No BML adapter/consumer is activated until root independently accepts/publishes this checkpoint and explicitly releases that lane.

The author performed the source pass; independent Astra review is pending and root-owned. Source is frozen after this checkpoint. No additional source/test/compiler work or dependency update is authorized by this report.

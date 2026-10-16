# Independent review: S5 native BML producer

## Verdict and fixed scope

**Changes required: 0 Critical, 2 Important, 0 Minor findings.** Both Important findings have independent executable RED probes. No additional substantiated finding was identified. Native producer acceptance must wait for the consolidated fix and unchanged-probe qualification; this review does not accept consumer activation or whole S5/S6.

- Local candidate: `92048bccda52875a798ade1bccc4e44823a564fb`.
- Exact tree: `a9532ce41a286334e0462b7401bce786e89e65c0`.
- Baseline: `0baf8d3b6cbde4f9589cbc8f15c4b4d73d3820c6`.
- Parent-verified remote tree-equivalent commit: `5770f07ffbe4fd0b5701678846b97d4e6557eb68`. Remote verification is the parent's evidence; this reviewer performed no GitHub operation.
- Isolated clone: `/workspace/scratch/5dda642e718d/agent-vivy-issue58-bml-native-review`.
- Increment: 19 BML-only files: seven production Go files, seven Go test files, five documentation files. All 19 plus six supporting files were SHA-256 checked against committed HEAD blobs. All 25 match; the worktree/index remain clean.

Reviewed the assigned brief, root AGENTS, approved revision-3 C4/C5 and S5 plan/ruling, native owned-memory contract, committed native records, author report, full production increment and its direct native Store/Home/record/maintenance behavior. Receiving/requesting-review and verification-before-completion guidance applied. This is native SQLite persistence; it adds no model orchestration, RAG engine, second memory store or Eino implementation. Root's pinned Eino0.9.20 remains outside this producer execution.

No candidate production/test source, index, active workspace, dependency pin, native Laputa checkout, consumer composition or GitHub state was changed. All three independent probes exist only outside the candidate through test-file overlays. No subagents were launched.

## I1 — Important: an already-open schema-1 reader leaks owned rows after another handle upgrades

**Primary location:** `bml/bound_read.go:22-26` (`legacyPredicate`). The cached schema is populated in `bml/store.go:405-424`; legacy Get/List consume it at `:501-524`, and Search/SearchVisible at `bml/search.go:89-100`.

`OpenExisting` supports a live read-only schema-1 handle. Its `s.schema` remains 1 for the handle's lifetime. A second normal mutable `Open` can perform the supported schema-2 upgrade and commit new bound records. The original read-only handle sees those committed rows through SQLite but still builds the schema-1 predicate `1=1`, omitting ownership exclusion.

**Independent reproduction:**

1. Create the actual released schema-1 fixture, with one legacy row.
2. Open it through public `OpenExisting`; verify legacy reads work and new bound capabilities are unavailable.
3. Keep that handle open. Use a second public `Open` to upgrade, then public `MutateBound` to create `new-private-row` under `issue58Owner`.
4. Use the original read-only handle's legacy Get/List/Search/SearchVisible.

All four legacy reads disclose the newly owned row. The original raw output includes:

```text
schema-1 readonly handle leaked owned row via legacy Get:
id=new-private-row content="private continuity needle"
schema-1 readonly handle leaked owned row via legacy List
schema-1 readonly handle leaked owned FTS via legacy Search
schema-1 readonly handle leaked owned FTS via legacy SearchVisible
```

The FTS queries use `Limit:1`; complete record/provenance bytes are returned. This uses the same current binary, supported constructors and valid migration. It requires neither external database tampering nor a previous binary, backup restore or identity-maintenance concurrency. It breaks both the promised legacy/bound data separation and pre-limit filtering.

**RED:** `probes/reviewer_live_upgrade_test.go`, `TestReviewerBMLReadOnlyHandleAcrossUpgrade`; original output in `raw/issue58-bml-review-live-upgrade.log`.

**Smallest useful correction:** make legacy reads observe/fence schema changes consistently with the data snapshot they query. A schema-1 handle must either remain on an actual schema-1 snapshot, safely adopt the schema-2 exclusion, or explicitly fail closed after an upgrade. Do not select the permissive query from a stale handle field. A separate check followed by an unprotected read would leave a check/read upgrade race; preserve one consistent SQLite read boundary. Keep genuinely unchanged schema-1 read-only files unmodified and legacy-readable.

**Classification:** new schema-evolution/privacy defect. The schema cache and ownership-dependent predicate are introduced by this increment.

## I2 — Important: a single-target tombstone can suppress another updated record without its CAS

**Primary location:** `bml/bound.go:130-134`, where only `Tombstone.TargetRecordID` receives a target-revision guard. `validateWriteOwnership` at `:267-284` checks additional supersedes targets for ownership/visibility, not their revisions. The inherited effect loop at `bml/store.go:738-742` removes FTS for every `Record.Supersedes` target; GetBound treats each as suppressed. The immutable outcome reports only `Tombstone.TargetRecordID` (`bml/bound.go:293-295`).

The new bound API exposes one target and one `ExpectedTargetRevision`, but it accepts a tombstone whose full native record has `Supersedes=[A,B]`. Supplying A's valid revision therefore authorizes an additional suppression of B's latest state without a B revision guard, while the result names only A.

**Independent reproduction:** create A and B under the same exact binding, update B from revision 1 to revision 2, then submit a tombstone with:

```text
TargetRecordID = A
ExpectedTargetRevision = 1  // A's actual revision
Supersedes = [A, B]
ExpectedStoreRevision = nil
```

The mutation succeeds. Its immutable result reports `TargetID=A`; `GetBound(B, expectedRevision=2)` now fails with `native source unavailable for binding`. The request never supplied B's current revision. This is same-binding lost-update/stale-CAS exposure, not a cross-owner authorization bypass or physical erasure of B's canonical row.

**RED:** `probes/reviewer_target_cas_test.go`, `TestReviewerBMLTombstoneCannotBypassSecondTargetCAS`; original output in `raw/issue58-bml-review-target-cas.log`:

```text
single-target CAS tombstone deleted independently updated B without B CAS:
outcome_target=target-a A_revision=1 B_revision=2
error=native source unavailable for binding
```

The root architecture owner confirmed the boundary during review: the new bound operation has exactly one target/CAS/result target, so an additional unguarded target is not an allowed implicit bulk mutation.

**Smallest useful correction:** reject bound tombstones whose supersedes shape contains any target other than the single guarded `TargetRecordID`; retain normal native record validation and the single-target CAS. Preserve existing legacy/unbound multi-supersedes behavior. No bulk-target API expansion is required.

**Classification:** inherited native multi-supersedes storage behavior newly exposed through the bound single-target CAS boundary. The failure concerns the new bound API contract, not a request to redesign legacy semantics.

## Independently verified boundaries

The fixed-candidate focused suite passes 99 tests/subtests. An additional independent cancellation probe passes, for 100 passing test events across two package-pass results. Together with source inspection, the executed cases establish these boundaries outside the findings:

- Record CAS, ownership, supersedes/FTS, counters and the immutable native operation result share the existing `putInner` SQLite transaction. Actual receipt INSERT failure and deferred-constraint Commit failure roll back the operation. No separate CRUD/receipt write or retired apply journal was introduced.
- **Independent cancellation probe:** a registered native SQLite scalar callback in a BEFORE INSERT trigger on `memory_operations` cancels the actual mutation context at receipt insertion. The call reports an error; original lookup stays absent; record/owner/operation/FTS/supersedes rows and counters/store revision remain zero. Removing the trigger and retrying the identical request produces exactly revision 1/store revision 1. This is deterministic local fault injection, not a claim about every possible OS crash point.
- Exact replay returns retained historical outcomes before current CAS/capacity checks, including after updates, tombstones and reopen. Concurrent same-operation writers and distinct-operation/record-CAS cases across actual Store handles pass. The result carries native record/store revisions and marker target semantics; the receipt contains no copy of the record body.
- Full binding/destination and request fingerprints reject altered body/provenance/CAS shape/digest and owner reuse. Nested invalid UTF-8 is rejected before canonical encoding; noncanonical native enums are rejected. Unknown legacy rows are not promoted to owned rows.
- Apart from the live schema transition in I1, tested schema-2 legacy Get/List/Search/Home/L1/import/update/remove/GC and bound read/export paths enforce the intended separation. Ownership filtering is in SQL before ORDER BY/LIMIT. Legacy and cross-binding suppression attempts reject. Deleted targets cannot reuse their old CAS.
- Fresh schema 2 and actual existing Home schema-1 upgrade pass, preserving old rows/revisions/counters/FTS. Unsupported schema shapes and upgrade faults fail closed. Read-only capabilities and rejection of read-only Restore pass.
- Backup/restore tests preserve original backup bytes; staged copied upgrades fail without replacing the live database on the tested injected upgrade failure. Bound records or historical receipts fence canonical identity migration before migration artifacts/writes. Ordinary unbound identity migration/rollback remains functional. These maintenance paths retain their documented exclusive lifecycle requirement.

No further finding is inferred from downstream App/worker qualification being intentionally pending.

## Commands, source integrity and retained evidence

Every test and module-list command ran through the unchanged `/tmp/issue58-localonly-runner.py`, copied verbatim into `probes/localonly-runner.py`. Offline dependencies, credential-name stripping and a non-forwarding deny proxy were active. All five invocations recorded zero external requests and zero forwarding. Fixtures use temporary local SQLite files; no tenant data or external model was accessed.

Runtime: Go1.26.4 linux/amd64. The independently executed module-list log confirms native `modernc.org/sqlite v1.56.0`. Root MVS1.59 is separate qualification. go.mod/go.sum are unchanged and hashed. All tests use `-p 1 -count=1`; no race lane was duplicated by this reviewer.

| Log stem, prefix `issue58-bml-review-` | Test pass/fail/skip | Package pass/fail | Exit | Seconds |
| --- | ---: | ---: | ---: | ---: |
| `live-upgrade` | 0/1/0 | 0/1 | 1 | 1.072 |
| `target-cas` | 0/1/0 | 0/1 | 1 | 1.135 |
| `focused` | 99/0/0 | 1/0 | 0 | 1.334 |
| `cancel` | 1/0/0 | 1/0 | 0 | 1.070 |
| `module` | n/a | n/a | 0 | 0.009 |

Counts are Go JSON terminal events including parent tests/subtests; package outcomes are separate. `results.json` and `raw/*.json` retain exact commands, candidate SHA and runner metadata. `raw/*.log` retain every original output stream, including both REDs. There were no compile/interrupted independent test attempts. `raw/increment.patch` records the exact reviewed source delta.

The original three probe files have not been edited since their first execution. Each has its own external overlay mapping. Reproduction uses the exact recorded runner command; for a fixed candidate root may relocate only overlay target paths, retaining the original probe bytes. `source-hashes.json` records all 25 source/support hashes against HEAD; `raw/source-final.json` records the final clean source freeze. The artifact manifest covers the report, brief, probes, mappings and all raw evidence, excluding the manifest itself.

Author working-source normal/race155+1-package passes and root-MVS execution were read as supporting evidence; they are not this reviewer's execution. Their baseline-HEAD metadata is not relabeled as fixed-candidate execution. The independently executed focused/cancellation results do not erase either RED.

## Handoff and remaining gates

Assign one consolidated author TDD fix for I1 and I2, then have root rerun the unchanged original probes and affected normal/race gates on a newly fixed immutable SHA. This reviewer does not perform a redundant second review of the same fix diff.

Actual Windows native execution, root MVS integration, published-fixed-source qualification, real App/Garden adapter/ObserverHost/C2 authorization, both-direction LIFE/Coding/worker projection, exact core-source navigation, UI and whole S5/S6 remain root-owned downstream gates. No cross-compilation, PostgreSQL, new browser, live model, deployment or installed-product acceptance is claimed here. No native producer finding is manufactured from the separately documented pending consumer work.

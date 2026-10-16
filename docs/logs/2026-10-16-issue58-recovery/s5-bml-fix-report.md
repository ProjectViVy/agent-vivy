# S5 native BML consolidated review fix

## Immutable checkpoint and scope

- Fixed commit: `7993302f2693bd945bdef995fc313eb5d6a52b13`.
- Tree: `2d6fb54b338bd859e50e83531782bd3fdab619b8`.
- Parent: `58943da5cd3a41b5adcd976a402c5c04f43898ed`.
- Author and committer: 大湿 <57738020+mastwet@users.noreply.github.com>.
- Exactly seven BML files: four production Go files, one new regression Go file,
  the native owned-memory contract and one new review-fix record in the existing
  native iteration directory. `staged.patch` records the full source delta.
- Source frozen; BML worktree clean and index empty at handoff. Root received the
  source/index release immediately after commit. No subsequent author source,
  test, dependency, index, build or test mutation occurred.

This is the authorized one-pass correction for independent review I1/I2 only.
Existing root CI fixes and all dependency bytes remain unchanged. No consumer,
App, UI, Garden adapter, native Laputa or public API change is included.

## I1: consistent legacy schema/data snapshot

A schema-1 read-only handle cannot authorize an unrestricted legacy query from
its cached schema. The private legacyRead helper starts a read-only SQLite
transaction and reads actual schema/workspace metadata there. The subsequent
Get/List/Search/SearchVisible/SupersededTargetIDs data query uses that same
transaction. A genuinely schema-1 snapshot stays legacy-readable; an observed
supported upgrade returns typed `schema_changed` and requires reopen. No separate
schema precheck/data-query race, cached-schema mutation or global registry is
introduced. Existing schema-2 handles retain SQL ownership exclusion before
ranking/LIMIT. Empty FTS text still returns no candidates without a data read.

The permanent regression opens the actual schema-1 fixture, verifies ordinary
read-only Get/List/FTS availability and unchanged database bytes, then keeps an
old handle open across supported Open/MutateBound upgrade. All five data-reading
entry points explicitly fence. It checks limit-1 behavior, conservative old
capabilities, reopened schema-2 native/legacy separation and actual Home startup
L1. A separate public-API reader loop overlaps the real writer migration and
commit, remains active until that writer finishes, and checks post-upgrade fence.

## I2: one bound target means one CAS-protected effect

Bound tombstones now require exactly one Supersedes ID equal to the native
Tombstone.TargetRecordID, before fingerprint/persistence. The existing positive
target revision guard remains. No bulk target/CAS API is added. Ordinary unbound
multi-target tombstones continue to use their established behavior.

The permanent regression independently updates B to revision 2, submits a
single-target A tombstone containing extra B, and verifies rejection preserves
all canonical rows, ownership, immutable operations, FTS, supersedes and native
metadata/counters. Lookup stays absent. Retrying the same never-committed
operation ID with only A succeeds, suppresses A and preserves B's corrected
revision/content. A separate legacy multi-target positive control passes.

## RED/GREEN and executed gates

All runs used Go1.26.4 linux/amd64 and the unchanged local-only runner. Native
module SQLite1.56 and root-MVS SQLite1.59 tests are separately executed. Every
run retained raw log plus runner JSON. Credential variables were stripped;
offline dependencies and a non-forwarding deny proxy were active. Every run
records zero rejected external requests and zero forwarding. No tenant data,
external model, public network test, cross-compilation or Windows execution was
used.

| Log suffix (prefix issue58-bml-fix-) | Test pass/fail/skip | Package pass/fail | Exit | Seconds |
| --- | ---: | ---: | ---: | ---: |
| red | 1/3/0 | 0/1 | 1 | 1.176 |
| green | 4/0/0 | 1/0 | 0 | 1.121 |
| original-probes | 3/0/0 | 1/0 | 0 | 0.969 |
| native-normal | 159/0/0 | 1/0 | 0 | 1.476 |
| native-race | 159/0/0 | 1/0 | 0 | 8.311 |
| root-mvs | 159/0/0 | 1/0 | 0 | 1.729 |
| native-vet | n/a | n/a | 0 | 0.215 |

Counts are Go JSON terminal events, including parent tests/subtests; package
results are separate. There were seven runner invocations, one expected semantic
RED, no compile/interrupted attempts and no additional failed gates. The RED
contains the stale-schema disclosures, actual overlapping List disclosure and
extra-target mutation/false receipt/state-change assertions; the legacy positive
control passes. The first focused GREEN preceded the later stronger typed-error
assertion and sustained-overlap test refinements. Final complete normal/race and
root-MVS execution include those final test bytes. No production Go bytes changed
between that first GREEN and final gates.

The three frozen original reviewer Go probes were executed unchanged through
`reviewer-overlay.json`, with only overlay target paths relocated to this
checkout. All three pass, including the deterministic receipt-insert cancellation
rollback/exact-retry positive probe. The original probes, review reports, original
REDs and manifests were not edited. `frozen-review-integrity.log` confirms all
23 frozen review artifacts still match their original manifest.

These are author working-source executions on baseline HEAD58943, as the raw
metadata truthfully records. `source-hashes-before-commit.json` and
`source-final.json` establish that the five changed Go files are byte-identical
to committed fixed source. They are not relabeled fixed-SHA executions. Root owns
clean fixed-commit unchanged-probe and full-gate qualification/publication.

## Exact commands

```text
python /tmp/issue58-localonly-runner.py /workspace/scratch/5dda642e718d/agent-vivy-issue58-recovery/bml issue58-bml-fix-red go test -json -p=1 ./... -run ^TestIssue58(ReadOnlyLegacyHandle|ReadOnlyReadsOverlap|BoundTombstoneHasOnly|LegacyMultipleSupersedes) -count=1 -timeout=3m
```

```text
python /tmp/issue58-localonly-runner.py /workspace/scratch/5dda642e718d/agent-vivy-issue58-recovery/bml issue58-bml-fix-green go test -json -p=1 ./... -run ^TestIssue58(ReadOnlyLegacyHandle|ReadOnlyReadsOverlap|BoundTombstoneHasOnly|LegacyMultipleSupersedes) -count=1 -timeout=3m
```

```text
python /tmp/issue58-localonly-runner.py /workspace/scratch/5dda642e718d/agent-vivy-issue58-recovery/bml issue58-bml-fix-original-probes go test -json -overlay=/workspace/scratch/5dda642e718d/agent-vivy-issue58-recovery/.superpowers/sdd/S5-bml-fix/reviewer-overlay.json -p=1 ./... -run ^TestReviewerBML -count=1 -timeout=3m
```

```text
python /tmp/issue58-localonly-runner.py /workspace/scratch/5dda642e718d/agent-vivy-issue58-recovery/bml issue58-bml-fix-native-normal go test -json -p=1 ./... -count=1 -timeout=4m
```

```text
python /tmp/issue58-localonly-runner.py /workspace/scratch/5dda642e718d/agent-vivy-issue58-recovery/bml issue58-bml-fix-native-race go test -json -race -p=1 ./... -count=1 -timeout=5m
```

```text
python /tmp/issue58-localonly-runner.py /workspace/scratch/5dda642e718d/agent-vivy-issue58-recovery issue58-bml-fix-root-mvs go test -json -mod=readonly -p=1 github.com/ProjectViVy/agent-vivy/bml -count=1 -timeout=4m
```

```text
python /tmp/issue58-localonly-runner.py /workspace/scratch/5dda642e718d/agent-vivy-issue58-recovery/bml issue58-bml-fix-native-vet go vet -p=1 ./...
```

`gofmt -l` on all five changed Go files was empty; `git diff --check` passed.
`precommit-checks.log` preserves those checks. No schema version or dependency
change was made. `source-final.json` records all seven fixed committed source
SHA256 values, four unchanged module/sum hashes, all three original probe hashes
and the unchanged runner hash. `source-hashes.sha256` is the concise seven-file
source manifest. `hash-manifest.sha256` covers this folder's evidence except
itself; its hash is sent in the final handoff.

## Remaining limits

Actual native Windows: **NOT RUN**. No consumer or App integration activation,
whole S5/S6 acceptance, deployment, migration or new dependency pin is claimed.
The explicit stale-handle `schema_changed` fence requires caller close/reopen;
it does not adopt schema-2 capabilities on a schema-1 handle. Root retains
publication and all downstream acceptance decisions. Source and index remain
released/frozen after this checkpoint.

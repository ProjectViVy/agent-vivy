# Native C5 producer checkpoint report

Source is frozen after checkpoint **3c365207d295221c4a3c5667fc71a6a465ee59f8**,
tree **1ca98be9a8cffb18d242bf3c1aa40a613d0bbb7f**, on
`feat/issue-58-recovery-native-20261016`. Native worktree and index are clean.
Author/committer: 大湿 <57738020+mastwet@users.noreply.github.com>.
No source/pin/UI/BML modification was made in Vivy. Root owns fresh independent
review, publication and the new exact dependency pin. This checkpoint is not
consumer activation or installed acceptance.

## History and exact source identity

Checkpoint parent is **1ce1c1e322ff278db258044dfddb640dcf29101c**,
tree **a6d4515158008ffc96dc51f7ee546e13115d3745**. That ordinary documentation merge
preserves ordered parents:

- bb57e93a61cfb41460524d00a557a3a9f2cce1e7 (accepted native recovery producer)
- f44ba00eb6ab5aa6ab9a2f5c09868373e36679f5 (current main policy documentation)

The merge changed 71 documentation/instruction files and zero Go sources. Latest
applicable AGENTS and ADR-0018 were read. Root's already published equivalent merge
is 9b25b0a9d191845a97ca8a114a18619625eaf223 with the same exact tree; source writer
performed no publication.

`native-producer-logs/source-hashes.json` contains SHA-256 for all 12 modified files
and all nine Go files, exact checkpoint/merge SHA, trees and parents. Every recorded
file hash was compared with its committed HEAD blob: all match. Test runner metadata
names merge HEAD because tests ran against its working source increment; hashes bind
that tested source to the committed checkpoint without relabeling the old logs.

## Delivered boundary

Same-owner trusted RecapBinding is installed in ingest before real runtime Start.
Only native ReadRecap success with a validated reachable v3 revision > 0 can close
the incompatible legacy ACTMEM consumer. Original pending/applying/unknown status,
Before, source and effect metadata remain independent in the existing activity_json.
A typed optional legacy_consumer policy proof is not an effect receipt. Already
applied receipts return unchanged before current policy checks. No new store,
scheduler, parser, recovery API or public Backend method was added.

Native owner LookupCaptured validates the actual v3 chain and refuses live unknown
C1 state before using the same capsule parser and exact pair matcher. Actual archive
entries keep their IDs/provenance; archive-only Revision=0 means there is no current
legacy head revision. Garden can rejoin Before.Revision+1 only with the original
persisted structurally valid snapshot and complete exact positive archive pair.
Revision zero is valid for a real first snapshot with its native SHA-256. Missing/
malformed Before, absent/partial/conflicting proof retains uncertainty; IO/malformed
archives stay errors. The old persisted intent is trusted attempt metadata: native
capsules do not preserve pre-head bytes, so arbitrary external DB tampering of an
otherwise well-shaped old hash is not cryptographically reconstructed or claimed.

Start/drain/HighWatermark/ArchiveSession reuse one strict decoder and one semantic
needsLegacyActivity predicate over a closed ordered SQL statement snapshot. Owner IO
never runs with SQL rows open. Earliest unresolved rows fence later watermarks.
Already closed consumers are revalidated against current trusted C1 plus their
reachable historical policy digest. Valid v3 finalization returns an explicit
internal policy no-fold result instead of calling the still-denied FoldSession.
Raw accepted/running/spooled material replay stays independently eligible. No C1
text, revision, digest, coverage or archive evidence is advanced by closure.

Ruling agreed with root: use ordered SQL snapshots + shared strict native decoding
instead of maintaining four subtly different JSON SQL expressions. This is fail
closed for malformed/unknown metadata and preserves conservative snapshot fencing.
ArchiveSession uses the same bounded native projection under existing activityMu
rather than waiting indefinitely on a consumer deliberately omitted from the queue.

## Verification

Unchanged `/tmp/issue58-localonly-runner.py`; Go1.26.4 from the task toolchain.
All command metadata records disabled dependency networking, credentials stripped by
name, a non-forwarding deny proxy and zero external requests/forwarding. No assertions,
refusal logic, dependencies, go.mod/go.sum, CI or justfile were relaxed or changed.

Final working-source checks:

| Log stem | Command selection | Tests/subtests | Packages | Result | Seconds |
|---|---|---:|---:|---|---:|
| final-normal2 | Garden `go test -json -p=1 ./internal/ingest ./internal/runtimecore ./agentapi -count=1 -timeout=4m` | 161 | 3 | PASS; zero fail/skip | 10.136 |
| final-race | Same Garden selection with `-race`, timeout 5m | 161 | 3 | PASS; zero fail/skip/race | 41.897 |
| final-owner | Laputa `go test -json ./actmem -count=1 -timeout=3m` | 120 | 1 | PASS; zero fail/skip | 0.782 |
| final-owner-race | Same native actmem selection with `-race` | 120 | 1 | PASS; zero fail/skip/race | 4.802 |

All stems have prefix `issue58-s5-native-`. Gofmt and git diff --check passed before
commit. Normal and race cover real Start/restart and one-second timer observation,
old applied receipt priority, exact archive positive replay, unknown/missing/partial/
conflicting evidence, legitimate zero Before, invalid owners/metadata, persistence
failure, independent raw material replay, earliest gap and concurrent drain/archive/
reads, unchanged authority/archive bytes, existing v1/v2 behavior and all native v3
legacy writer denials.

## RED and retained attempts

- owner-red: real append/fold/v3/reopen lookup failed recap_unavailable; owner-green
  rejoined the actual entries with explicit archive-only revision zero.
- runtime-red and runtime-red2: fixture setup failures (state directory, then old
  capture binding) preserved; runtime-red3 is the actual symptom RED: real Open's
  worker never closed the valid-v3 pending activity in three seconds. runtime-green
  proves one durable closure across two restarts and observation beyond retry time.
- policy-red: actual archive directory IO was wrongly swallowed as success;
  policy-green preserves the error and original unknown intent.
- archive-red initially passed a broad error assertion for the wrong (partial pair)
  reason. archive-red2 strengthens it to the actual malformed native proof error and
  fails; archive-green returns that error through the one parser's strict v3 scan.
- Full original .log/.json files, intermediate GREENs and regression outputs are in
  native-producer-logs. results.json enumerates every outcome without discarding the
  failing or diagnostic attempts. The incidental apply_patch transport interruption
  was retried before a test ran and did not change a test result.

## Remaining gates and next source boundary

Actual Windows native execution is NOT RUN locally. A Windows-specific missing-head/
orphan zero-view test is committed; platform-neutral positive Start/restart also runs
on Windows. Cross-compilation is not claimed as execution. Root owns actual Windows
native qualification, full native module parsing/dependency qualification, fresh
Astra review, any one consolidated author TDD fix, unchanged probes, remote native
publication and the exact new module/lock/sum pin. No main integration, deployment or
tenant migration was performed. The source writer stops here until root releases a
further lane after native acceptance/pin.

Later consumer acceptance must reuse accepted S4 full-App ToolEvidence/HTTP/seal/
reopen fixtures, and trace real already-persisted old raw material under selected
BML. An old raw/ordinary Coding/private/unknown capture must not gain LIFE durable
ownership merely from legacy ProfileID or selected backend. This producer deliberately
keeps raw canonical ingestion intact and does not decide those source grants. No
Vivy source, BML schema/receipts, new backend injection, UI or consumer path is included.

## Modified files

- `docs/logs/2026-10-16-issue58-legacy-consumer/acceptance.md`
- `docs/logs/2026-10-16-issue58-legacy-consumer/summary.md`
- `docs/logs/2026-10-16-issue58-legacy-consumer/verification.md`
- `garden/internal/ingest/capture_activity.go`
- `garden/internal/ingest/legacy_policy_test.go`
- `garden/internal/ingest/legacy_policy_windows_test.go`
- `garden/internal/ingest/service.go`
- `garden/internal/runtimecore/legacy_consumer_test.go`
- `garden/internal/runtimecore/runtime.go`
- `laputa/actmem/capture_pair.go`
- `laputa/actmem/capture_v3_test.go`
- `laputa/actmem/ops.go`

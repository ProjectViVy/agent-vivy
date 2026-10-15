# S5 native producer consolidated-review fix

## Frozen source identity

- Fixed local checkpoint: `a99c25feea48e151c847cea81015bec0fdfe80bf`.
- Tree: `9fff8cf9d5ca174a1af90e43a0e718281a24635e`.
- Parent: `3c365207d295221c4a3c5667fc71a6a465ee59f8` (reviewed candidate).
- Preserved ordinary merge: `1ce1c1e322ff278db258044dfddb640dcf29101c`.
- Ordered merge parents: `bb57e93a61cfb41460524d00a557a3a9f2cce1e7`, `f44ba00eb6ab5aa6ab9a2f5c09868373e36679f5`.
- Branch: `feat/issue-58-recovery-native-20261016`.
- Author and committer: 大湿 <57738020+mastwet@users.noreply.github.com>.
- Native worktree/index clean; source is frozen. No GitHub operation or Vivy pin/source/BML change was made.

Root owns unchanged reviewer probes, fixed affected gates, exact remote publication and dependency qualification. This report does not replace or edit the earlier 3c/1edf report, source hashes, review artifacts or original RED output. The four read-only consumer seam notes were appended to prep.md before this fix release and then paused.

## Findings and minimal changes

I1: An applying/unknown persisted attempt must carry an explicit, non-null original Before object and `before.revision`. Value decoding previously turned absent/null into zero, allowing true archive IDs to be combined with an invented revision and clearing uncertain source/error. The ingestion decoder now rejects that incomplete metadata before any projection/write. Its original activity_json remains byte-for-byte unchanged, including whitespace, source, error and incomplete Before. Explicit zero remains valid and recovers actual native revision 1. The complete nonzero original revision 2 recovers the actual native revision 3. No `Revision > 0` guard or C1-to-legacy revision substitution was introduced. Existing explicit unusable snapshot hashes retain their previous independent policy-closure behavior.

I2: A local `json.Valid` whole-document guard precedes the existing shared strict decoder. Extra closing delimiters and second JSON values are rejected. Unknown fields and duplicate keys remain rejected by the existing strict decoder. The shared evolution helper and all unrelated consumers remain unchanged. All six ingestion projection/recovery/fence entry points use the same local decoder; no native C1 parser, public API, store or scheduler was added.

New native fixtures construct actual append/fold/archive/v3 evidence and cover unknown/applying incomplete metadata, explicit zero/nonzero controls, malformed full documents and retained unknown/duplicate rejection. Every failing entry point (ActivityProjection, projectActivity, drainActivity, real Start, HighWatermark, ArchiveSession) asserts unchanged original DB JSON. The fixtures also assert exact authority/archive bytes and the same C1 head, including revision/coverage. All existing worker/restart, legacy/header and Windows test source was retained unchanged.

## Modified files and SHA256

- `garden/internal/ingest/capture_activity.go`: `32feb6532576467e8bc057d598f5808db34cc6fc8b0decfe55e22a85df20318e`
- `garden/internal/ingest/legacy_metadata_test.go`: `7fadfb123e23b5a7ed68f31e705c01e45aa94fa1329efacf112ae057b339a7cf`
- `docs/logs/2026-10-16-issue58-legacy-consumer/review-fix.md`: `6c78d39c22e57974cee658d9129e9e31145b47a358d9cee2efe61a6404cc4007`

`native-producer-fix-logs/source-hashes.json` additionally hashes all 14 files in the complete producer increment relative to its merge baseline, verifies each working file against its fixed committed blob, and proves the precommit tested fix bytes equal fixed HEAD. The fix has one production file, one new test file and one new English record; no prior native record was rewritten.

## RED/GREEN and verification

The only test invocations in this fix pass are recorded below; all original streams and runner metadata are preserved in `native-producer-fix-logs`. There were no compile-error, interrupted or unreported intermediate test attempts. The author did not rerun or alter the independent overlay probes; root will run their original bytes against the fixed immutable candidate.

| Label (prefix issue58-s5-fix-) | Test pass/fail/skip | Package pass/fail | Exit | Seconds |
| --- | --- | --- | --- | --- |
| metadata-red | 8/12/0 | 0/1 | 1 | 1.573 |
| metadata-green | 20/0/0 | 1/0 | 0 | 1.372 |
| garden-normal | 181/0/0 | 3/0 | 0 | 10.365 |
| garden-race | 181/0/0 | 3/0 | 0 | 42.326 |
| actmem-normal | 120/0/0 | 1/0 | 0 | 0.365 |
| actmem-race | 120/0/0 | 1/0 | 0 | 3.282 |

Counts are Go JSON terminal events including parent tests/subtests, with package results separate. Metadata RED demonstrated the original wrong mutation for eight incomplete snapshot forms and acceptance of both trailing delimiters; the zero/nonzero and existing second-value/unknown/duplicate controls passed. The unchanged new assertions then passed GREEN. Focused normal/race totals are each 301 tests/subtests plus four packages, with no failures/skips or race diagnostics. Gofmt and git diff/cached diff whitespace checks passed before commit.

Every test command used the unchanged `/tmp/issue58-localonly-runner.py`. Go was 1.26.4 linux/amd64; dependencies were offline, credential-bearing environment names stripped and a non-forwarding deny proxy installed. All six runs observed zero external requests and zero forwarding. Runner metadata names baseline HEAD because the tested fix was uncommitted at execution; the source hashes bind those exact working bytes to the fixed checkpoint. No assertion, runner rejection rule or native dependency was weakened.

Commands (runner prefix `python /tmp/issue58-localonly-runner.py <module-dir> <label>`):

- `python /tmp/issue58-localonly-runner.py /workspace/scratch/5dda642e718d/laputa/laputa issue58-s5-fix-actmem-normal go test -json -p=1 ./actmem -count=1 -timeout=3m`
- `python /tmp/issue58-localonly-runner.py /workspace/scratch/5dda642e718d/laputa/laputa issue58-s5-fix-actmem-race go test -json -race -p=1 ./actmem -count=1 -timeout=3m`
- `python /tmp/issue58-localonly-runner.py /workspace/scratch/5dda642e718d/laputa/garden issue58-s5-fix-garden-normal go test -json -p=1 ./internal/ingest ./internal/runtimecore ./agentapi -count=1 -timeout=4m`
- `python /tmp/issue58-localonly-runner.py /workspace/scratch/5dda642e718d/laputa/garden issue58-s5-fix-garden-race go test -json -race -p=1 ./internal/ingest ./internal/runtimecore ./agentapi -count=1 -timeout=5m`
- `python /tmp/issue58-localonly-runner.py /workspace/scratch/5dda642e718d/laputa/garden issue58-s5-fix-metadata-green go test -json -p=1 ./internal/ingest -run '^TestIssue58Legacy(SnapshotRequiresOriginalRevision|ClosureRequiresCompleteJSONDocument)$' -count=1 -timeout=3m`
- `python /tmp/issue58-localonly-runner.py /workspace/scratch/5dda642e718d/laputa/garden issue58-s5-fix-metadata-red go test -json -p=1 ./internal/ingest -run '^TestIssue58Legacy(SnapshotRequiresOriginalRevision|ClosureRequiresCompleteJSONDocument)$' -count=1 -timeout=3m`

## Limits and handoff

Actual Windows execution is **NOT RUN**. This Linux focused native evidence does not claim Windows, full native module CI, PostgreSQL, live-model quality, published-pin/module-resolution qualification, BML consumer acceptance or whole S5/S6. Root retains those gates and the publication/pin lane. Vivy remains on bb57 until root explicitly accepts/publishes the fixed producer and updates the exact dependency closure. Source remains frozen after this checkpoint; there is no further author fix/review pass or consumer work in this delivery.

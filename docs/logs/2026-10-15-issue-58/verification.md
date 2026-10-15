# Verification

All results below were produced in this workspace, on this machine. Historical
evidence recorded under `docs/logs/2026-10-15-issue58-*` was produced in a
different (Linux) workspace and is **not** treated as validation of this checkout.

Environment

| Fact | Value |
|---|---|
| OS | Microsoft Windows 11, version 10.0.26200, build 26200 |
| Go | `go1.26.4 windows/amd64` (`CGO_ENABLED=0`, `CC=gcc` not installed) |
| node / just | `v24.19.0` / `just 1.46.0` |
| Vivy lane | `feat/issue-58-life-continuity`, branched from `66fabfd2`; `main` unchanged at `5715950d` |
| Laputa pin | `bb57e93a61cfb41460524d00a557a3a9f2cce1e7`, parent `4c7c6b20859375d0d764d3ba2a0188926cf73c9e`, tree clean |

## Scene protection

Before any write: Vivy root `git status --porcelain` empty, no stash, one worktree;
Laputa sibling clean and detached at `4b2bec2c`. Nothing was reset, cleaned or
overwritten. `git switch -c feat/issue-58-life-continuity 66fabfd2` left `main` at
`5715950d`. The only pre-existing dirt after branching is
`.agents/skills/oil-frontend`, where the submodule working tree already sits at
`main`'s newer pointer `07a3c730` while this lane's tree records `772968c3`; it is
not a local edit and it is deliberately left unstaged. T2's merge reconciles it.

## Pinned source recovery (no rebuild)

`just ensure-laputa` → exit 0. It read `laputa-source.lock.json`, fetched
`bb57e93` from `https://github.com/ProjectViVy/laputa.git` and detached-checked it
out. Verified: HEAD `bb57e93…`, parent `4c7c6b2…`, clean tree, and
`laputa/actmem/{recap,recap_format,recap_history}.go`,
`laputa/actmem/{recap_test,recap_crash_test,recap_recovery_test,recap_reconcile_test}.go`,
`garden/agentapi/{recap,recap_test}.go` all present. The `reconcile` transition is
present in the writer (`recap.go`), the strict decoder (`recap_format.go`) and the
reachable-chain validator (`recap_history.go`).

`just bootstrap-test` → **11 pass / 0 fail / 0 skip** (`ensure-laputa` 9 cases,
`dependency-closure` 2 cases).

## The finding: the recovered baseline did not pass locally

`go test ./internal/storage/... ./internal/app ./internal/rpc -run '^TestIssue58' -count=1`
→ **exit 1**. SQLite: 3 pass / 15 fail. App: 5 fail. Every failure was
`storage: subject binding mismatch`:

```
FAIL TestIssue58SubjectSealAcrossHandles, TestIssue58SealedSourceTamperRejectedOnRetry,
     TestIssue58ExactEvidenceRequiredBeforeSeal, TestIssue58InteractionOrderAndDedup,
     TestIssue58SubjectIsolation, TestIssue58AdmissionBoundaries,
     TestIssue58ContinuityReplayBinding, TestIssue58FormattedGrantReplay,
     TestIssue58SubjectRetention, TestIssue58MissingEvidenceBlocksNewSeal,
     TestIssue58SubjectContextSnapshot, TestIssue58SubjectContextScope,
     TestIssue58SubjectContextCopyScope, TestIssue58SubjectContextSnapshotRace,
     TestIssue58SubjectContextInvalidMapping
FAIL agent-vivy/internal/app: TestIssue58ContinuityOnlyCompositionUsesSelectedAdmission,
     TestIssue58SingletonAdmission, TestIssue58ExactTextInteractionSealsThroughServiceRun,
     TestIssue58RetainedDeleteDoesNotTombstoneSingleton, TestIssue58FormattedGrantAuthorizedRead
```

Root cause (single site): `internal/storage/conformance/subject_history.go` wrote
`"owner_path":"/trusted/owner"`. `domain.SubjectIdentity.Valid()` requires
`filepath.IsAbs(OwnerPath)`; on Windows that literal is not absolute, so
`subjectsql.identity` returned `ErrSubjectBinding` for every admission. Confirmed
by inspection: `/trusted/owner` appeared exactly once in the tree, and every failing
suite consumes `conformance.Issue58Identity`.

Fix applied: the fixture builds the identity via `domain.SubjectIdentity` with
`filepath.Join(t.TempDir(), "actmem")`. The validator was not changed.

### After the fix

| Command | Result |
|---|---|
| `go test ./internal/storage/... ./internal/app ./internal/rpc -run '^TestIssue58' -count=1 -json` | exit 0 — **pass=62 fail=0 skip=16** |
| `go test -mod=readonly ./internal/modules/diva-cognitive ./internal/cognitivecontract -count=1 -json` | exit 0 — **pass=41 fail=0 skip=1** |
| `go test -p 1 -timeout 35m ./internal/runtime ./internal/storage/... -count=1` | exit 0 — runtime ok 342.5s, storage ok 2.6s, migrations ok 4.9s, postgres ok 0.1s, sqlite ok 103.5s |
| `go vet ./...` | exit 0 |
| `go build ./...` | exit 0 |
| `git diff --check` | exit 0 (no output) |
| `just ci` `fmt-check` equivalent over tracked Go files | exit 0 (no unformatted files) |

Focused skips, listed by name and **not** counted as passes: 13 PostgreSQL-server
`TestIssue58` suites (`AdmissionBoundaries`, `ContinuityReplayBinding`,
`CopySources`, `FormattedGrantReplay`, `InteractionOrderAndDedup`,
`MissingEvidenceBlocksNewSeal`, `SubjectContextCopyScope`,
`SubjectContextInvalidMapping`, `SubjectContextScope`, `SubjectContextSnapshot`,
`SubjectContextSnapshotRace`, `SubjectIsolation`, `SubjectRetention`) because
`VIVY_POSTGRES_TEST_DSN` is unset. Real PostgreSQL execution is therefore
unverified.

## Laputa on Windows

`go test ./...` from `../laputa/laputa` at the pristine pin → **exit 1**:
`actmem` fails 7 top-level / 22 subtest cases; `evolution`, `evolution/diva`,
`evolution/inofy` and `persona` pass. Every failure is
`read <root>\.owner.lock: The process cannot access the file because another
process has locked a portion of the file` at an `issue58Files` call site.
`recap_recovery_test.go:167` also emits one expected `t.Skip` (a dedicated
platform test covers the Windows missing-head split); that is not a failure.

Cause: `recap_owner.go` holds `.owner.lock` with `gofrs/flock` for the owner's whole
lifetime. Windows `flock` uses an exclusive byte-range lock, so a second handle
cannot read it. `issue58Files` walks the owner root and `os.ReadFile`s every entry,
including the lock. The lock carries no content contract and is never read by
production code — only created, locked and closed.

Isolation experiment (reversible): the helper was patched locally to skip
`.owner.lock`, the suites were rerun, and the pristine file was then restored.
This is **not** a landed change to the Laputa repository.

| Command (Laputa, patched locally, unpublished) | Result |
|---|---|
| `go test ./actmem -run '^TestIssue58' -count=1 -v` | **pass=29 fail=0 skip=1** |
| `go test ./actmem -count=1` | ok 4.773s |

Restoration verified: restored file SHA-256 equals the backup
(`55005D26…D3D18A`), Laputa `git status --porcelain` empty, HEAD still
`bb57e93…`, and `just ensure-laputa` exits 0 again.

Prepared upstream patch (exact, unpublished):

```diff
--- a/laputa/actmem/recap_test.go
+++ b/laputa/actmem/recap_test.go
@@ -306,6 +306,11 @@ func TestIssue58LegacyWriterRejectsV3(t *testing.T) {
 	}
 }
 
+// issue58Files snapshots the owner-visible ACTMem content that must not change.
+// The lifetime owner lock is ownership metadata without a content contract:
+// Windows holds it under an exclusive byte-range lock for the whole owner
+// lifetime, so reading it is neither possible nor meaningful. Excluding it keeps
+// the durable-content assertion identical on every platform.
 func issue58Files(t *testing.T, root string) map[string]string {
 	t.Helper()
 	out := map[string]string{}
@@ -314,6 +319,9 @@ func issue58Files(t *testing.T, root string) map[string]string {
 			return err
 		}
 		if !d.IsDir() {
+			if d.Name() == ".owner.lock" {
+				return nil
+			}
 			raw, e := os.ReadFile(path)
 			if e != nil {
 				return e
```

## Unavailable gates

| Gate | Evidence it cannot run here |
|---|---|
| `go test -race ./...` (Laputa and Vivy) | `go: -race requires cgo`. With `CGO_ENABLED=1 CC=clang`: `clang: error: unsupported option '-mthreads' for target 'x86_64-pc-windows-msvc'`. With `CGO_ENABLED=1 CC="zig cc"`: `lld-link: error: undefined symbol: WakeByAddressSingle` / `WakeByAddressAll`. No `gcc.exe` exists anywhere under the usual toolchain roots. |
| Garden `go test ./agentapi ./internal/runtimecore/...` | build failure: `github.com/coder/hnsw@v0.6.1/encode.go:304:23: undefined: renameio.TempFile` — upstream `renameio` ships only `!windows` files; Vivy carries a vendored replacement in `go.mod` only, so Garden cannot build natively as its own module. Pre-existing, unrelated to #58. |
| Real PostgreSQL parity / snapshot isolation | No `VIVY_POSTGRES_TEST_DSN`, no `psql`, no Docker. |
| `just ci` full recipe | Not run in this step; the focused, broad, vet, build and fmt gates above were run instead. |
| UI gates | Not touched in this step (no UI change yet); T2 runs them with the `main` merge. |

## Not verified here

`just ci` at a frozen candidate, native Windows race/durability for the pinned
Laputa tree, real PostgreSQL migration and isolation parity, real-model quality,
installed-artifact behaviour, DIVA/Mentle acceptance, the `#66` sidebar producer,
and all of S3–S6. No product acceptance is claimed by this step.

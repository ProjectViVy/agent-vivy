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
| Laputa pin | `bb57e93a61cfb41460524d00a557a3a9f2cce1e7` at the T1 baseline; **advanced to `8c5328daa7a4e9cdd089d78858de087a36fb0843` in T1b** (`v0.0.0-20261015195414-8c5328daa7a4`, `refs/heads/feat/issue-58-recap`), tree clean |

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
| `go test ./internal/storage/... ./internal/app ./internal/rpc -run '^TestIssue58' -count=1 -json` | exit 0 — **pass=62 fail=0 skip=16 event totals** |
| `go test -mod=readonly ./internal/modules/diva-cognitive ./internal/cognitivecontract -count=1 -json` | exit 0 — **pass=41 fail=0 skip=1 event totals** |
| `go test -p 1 -timeout 35m ./internal/runtime ./internal/storage/... -count=1` | exit 0 — runtime ok 342.5s, storage ok 2.6s, migrations ok 4.9s, postgres ok 0.1s, sqlite ok 103.5s |
| `go vet ./...` | exit 0 |
| `go build ./...` | exit 0 |
| `git diff --check` | exit 0 (no output) |
| `just ci` `fmt-check` equivalent over tracked Go files | exit 0 (no unformatted files) |

**Counting method (corrected in T1b).** `go test -json` emits both test-level and
package-level events, so a raw `"Action":"pass"`/`"Action":"skip"` count overstates
the number of tests. For the focused run the two readings are:

| Reading | pass | fail | skip |
|---|---|---|---|
| Event totals (as tabulated above) | 62 | 0 | 16 |
| Test-level (top-level + subtest) | 56 | 0 | 13 |
| Package-level | 6 | 0 | 3 |

The 3 package-level skips are `internal/storage/bundlesql`,
`internal/storage/conformance` and `internal/storage/subjectsql`, which match no
`^TestIssue58` test (`no tests to run`). The 6 package-level passes are
`internal/storage`, `postgres`, `migrations`, `rpc`, `sqlite` and `app`, all `ok`.
Both readings reproduce identically before and after the T1b pin advance, so the pin
change caused no test-outcome change. The earlier T1 record quoted the event totals
without this distinction; that was imprecise, not a different result.

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

| Command (Laputa, patched locally, unpublished) | Result |
|---|---|
| `go test ./actmem -run '^TestIssue58' -count=1 -v` | **pass=29 fail=0 skip=1** |
| `go test ./actmem -count=1` | ok 4.773s |

Restoration verified: restored file SHA-256 equals the backup
(`55005D26…D3D18A`), Laputa `git status --porcelain` empty, HEAD still
`bb57e93…`, and `just ensure-laputa` exits 0 again.

The exact patch that was later published (unchanged from the isolated experiment):

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

## T1b — publication and re-pin verification

The owner authorized publishing this patch, so the "unpublished local experiment"
above became a real upstream commit followed by a coordinated re-pin. Sequence and
evidence:

1. **Commit.** In the Laputa checkout, local branch `feat/issue-58-recap` was created
   at the previously pinned `bb57e93`. The helper was patched and committed as
   `8c5328daa7a4e9cdd089d78858de087a36fb0843` (author `mastwet`, no AI attribution,
   8 insertions in one file, `git diff --stat` = 1 file changed).
2. **Verify before pushing.** Native Windows, at that exact commit:
   `go test ./actmem -run '^TestIssue58' -count=1 -v` → **pass=29 fail=0 skip=1**;
   `go test ./...` for the `laputa` module → `actmem ok 5.038s`, `evolution`,
   `evolution/diva`, `evolution/inofy`, `persona` all `ok`, exit 0.
3. **Push.** Direct GitHub connections on this machine are intermittent
   (`Failed to connect to github.com port 443 after 21s`, three consecutive direct
   attempts). The push succeeded through the machine's local proxy:
   `git -c http.proxy=http://127.0.0.1:7892 push origin 8c5328d:refs/heads/feat/issue-58-recap`
   → `bb57e93..8c5328d` (a fast-forward; no force, no history rewrite).
   Remote verified: `git ls-remote --heads ... feat/issue-58-recap` →
   `8c5328daa7a4e9cdd089d78858de087a36fb0843`.
4. **Pseudo-version.** Derived from the commit, not invented:
   commit time `2026-10-16T03:54:14+08:00` = epoch `1792094054` =
   `2026-10-15T19:54:14Z` → `v0.0.0-20261015195414-8c5328daa7a4`.
5. **Re-pin.** `go.mod` (three `require` lines), `laputa-source.lock.json`
   (`commit`), and `go.sum` six new lines.
6. **go.sum cross-check.** The hashes were not derived by hand. A throwaway module
   (`.workspace/tmp/laputa-sum`, no `replace`, `GONOSUMDB=github.com/ProjectViVy/*`)
   downloaded all three modules at the published version and printed the canonical
   `Sum`/`GoModSum`. The lines written into this repository's `go.sum` are identical
   to that output:

   ```text
   garden  h1:TzPbwAjK7l82kUUyOn617cpoyNpqI0NcXqrvVU9q2yo=  go.mod h1:t+6aPD6GSpEPu455CqChrFYzsHnPuflsjme5ralof/I=
   laputa  h1:VK7gAt1R4LvAiDgNAw6VqKoGkvyjv/jJGsgQMinHDmg=  go.mod h1:nctqXPUme+YhoV1SMwVqnrIkTKoHaJNl+0yvZn/7ZaE=
   mentle  h1:emgqDUul4zJ7TUZgtFOB9TUyvA2OAJR6bHbsciCcxs0=  go.mod h1:0CHmst4cXuP9bCslJ1ZQMrHo4ZS3ivVg92YGM3vtjsA=
   ```

   The three `go.mod` hashes are unchanged from the previous pin (expected: no
   `go.mod` was edited); the three module hashes changed (expected: a module zip's
   hash covers `<module>@<version>/...` paths, so the version participates).
7. **Pin consistency in this repository.**
   `just ensure-laputa` → exit 0 (lock and local checkout agree, tree clean).
   `just bootstrap-test` → 11 pass / 0 fail, which includes the
   `dependency-closure` test asserting all three `go.mod` revisions match the lock.
8. **No regression from the pin advance.** Focused `TestIssue58` on the new pin:
   pass=62 fail=0 skip=16 (event totals, identical to T1);
   `go vet ./...` exit 0; `go build ./...` exit 0; `git diff --check` exit 0.

Scope note: this makes the pinned owner tests **runnable** on Windows. It is not a
durability result in itself — Windows durability still rests on the owner's own
crash, recovery and reconcile suites, and a snapshot helper that skips an unreadable
ownership artifact adds no coverage.

## Unavailable gates

| Gate | Evidence it cannot run here |
|---|---|
| Garden `go test ./agentapi ./internal/runtimecore/...` | build failure: `github.com/coder/hnsw@v0.6.1/encode.go:304:23: undefined: renameio.TempFile` — upstream `renameio` ships only `!windows` files; Vivy carries a vendored replacement in `go.mod` only, so Garden cannot build natively as its own module. Pre-existing, unrelated to #58. |
| Real PostgreSQL parity / snapshot isolation | No `VIVY_POSTGRES_TEST_DSN`, no `psql`, no Docker. |
| `just ci` full recipe | Not run in this step; the focused, broad, vet, build and fmt gates above were run instead. |
| UI gates | Not touched in this step (no UI change yet); T2 runs them with the `main` merge. |

## T1c — `-race` resolved locally and executed

`-race` was previously unavailable; it is now executable on this machine and the
relevant gates have been run.

What failed before (recorded so the constraint is not re-discovered):

- `go test -race` → `go: -race requires cgo`.
- `CGO_ENABLED=1 CC=clang` → `clang: error: unsupported option '-mthreads' for target 'x86_64-pc-windows-msvc'`.
- `CGO_ENABLED=1 CC="zig cc"` → `lld-link: error: undefined symbol: WakeByAddressSingle` / `WakeByAddressAll`.
- `choco install mingw -y` → **failed**, this shell is not elevated
  (`IsInRole(Administrator)` = `False`): `Unable to obtain lock file access on
  'C:\ProgramData\chocolatey\lib\...'`, `Access to the path
  'C:\ProgramData\chocolatey\lib-bad' is denied`, `Chocolatey installed 0/0 packages`.

Resolution (no elevation, no repository change):

```powershell
winget install --id BrechtSanders.WinLibs.POSIX.UCRT --accept-package-agreements --accept-source-agreements --disable-interactivity
# -> gcc.exe (MinGW-W64 x86_64-ucrt-posix-seh, built by Brecht Sanders, r2) 16.2.0
$mg = "$env:LOCALAPPDATA\Microsoft\WinGet\Packages\BrechtSanders.WinLibs.POSIX.UCRT_Microsoft.Winget.Source_8wekyb3d8bbwe\mingw64\bin"
$env:PATH = "$mg;$env:PATH"; $env:CGO_ENABLED = '1'; $env:CC = "$mg\x86_64-w64-mingw32-gcc.exe"
```

Race results with that toolchain:

| Command | Result |
|---|---|
| `go test -race -count=1 ./internal/domain` (toolchain smoke test) | exit 0, `ok 3.621s` |
| `go test -race -p 1 -count=1 -run '^TestIssue58' ./internal/storage/... ./internal/app ./internal/rpc` | exit 0 — **0 occurrences of `WARNING: DATA RACE`**; storage ok 1.184s, migrations ok 8.129s, postgres ok 2.400s, sqlite ok 77.776s, app ok 42.649s, rpc ok 6.858s |
| `go test -race -count=1 ./actmem` (Laputa, pinned `8c5328da`) | exit 0 — `ok 15.278s`, **0 occurrences of `WARNING: DATA RACE`** |

Limitations of this evidence, stated plainly:

- The toolchain is a per-command `PATH`/`CC` environment, **not wired into the
  `just` recipes**. A plain `just ci` still runs without `-race`; a future agent must
  set the same environment to reproduce these runs.
- These are the C2 concurrency suites and the owner's own `actmem` suite. The
  S3/S4 recap-context and compaction race gates do not exist yet, so nothing here
  substitutes for them.
- `-race` covers detected data races only. It is not durability or crash evidence.

## Not verified here

`just ci` at a frozen candidate, real PostgreSQL migration and isolation parity,
real-model quality, installed-artifact behaviour, DIVA/Mentle acceptance, the `#66`
sidebar producer, and all of S3–S6. No product acceptance is claimed by this step.

Windows durability for the pinned Laputa owner is now **runnable and passing**
(`actmem` Issue58 pass=29 fail=0 skip=1 at `8c5328daa7a4`), including under `-race`,
but see the scope note in T1b: a helper that skips an unreadable ownership artifact
enables the suite, it does not add durability coverage.

---

## T2 — merge of `main` into `feat/issue-58-life-continuity`

Candidate: merge commit `0c6bdc74c024cc06d2bbbb0d17d27e4eeeeb9278`
(parents `a3c8ee25` = lane, `5715950d` = `main`). Working tree clean before each
gate; Laputa clean at `8c5328daa7a4`.

### Merge surface

```text
git merge main --no-edit            # conflicts: docs/TODO.md, ui/src/routes/_layout.tsx
git diff --cached --shortstat       # 59 files changed, 3106 insertions(+), 268 deletions(-)
git diff --cached --check           # exit 0
git status --porcelain --diff-filter=U   # empty after staging both resolutions
git grep -nE '^(<<<<<<< |=======$|>>>>>>> )'   # no matches
```

### Conflict resolutions

- `docs/TODO.md`: kept main's five `PERSONA-*` / `SKILL-OIL-AUTO-SYNC` rows, dropped
  main's `EINO-TOOLSNODE-ERR-RACE` open row. Verified the id exists exactly once
  afterwards, as a DONE row in the Eino remediation completion record.
- `ui/src/routes/_layout.tsx`: main's body (`useCompanionAvatar`,
  `ShellOnboardingSlot`) + this lane's deletion of `ConversationModePlaceholder`.

### Run 1 — `just ci`, default `TEMP` (FAILED, environment)

```text
just ensure-laputa                  ok
just bootstrap-test                 11 pass / 0 fail
fmt-check                           exit 0
ui-ci -> ui-core -> pnpm typecheck  ok
ui-ci -> ui-core -> pnpm test       92 files, 692 tests, all passed
ui-ci -> ui-core -> pnpm build      FAILED
```

```text
✗ Build failed in 17.11s
[vite:esbuild-transpile] remove C:\Users\com01\AppData\Local\Temp\esbuild-147f2226…: Access is denied.
error: Recipe `ui-core` failed on line 66 with exit code 1
JUST_CI_EXIT=1
```

The build had already transformed 2596 modules. Reproduction and isolation:

```text
pnpm build                              # same failure, twice, new temp name each run
ESBUILD_BINARY_PATH=<node_modules esbuild.exe> pnpm build   # same failure
ESBUILD_WORKER_THREADS=0 pnpm build     # same failure
Remove-Item $env:TEMP\esbuild-*         # pwsh CAN delete these files (full control ACL)
```

A stale `esbuild.exe` service (started 2026-10-15 20:07) was still running and five
orphaned `esbuild-*` temp binaries from 2026-10-15 remained, so the unlink failure is
a standing property of this machine's `%TEMP%`, not of the merged source.

### Run 2 — `just ci` with `TEMP`/`TMP` redirected to a workspace scratch dir

```text
just ensure-laputa                  ok   (all Laputa Go revisions match the source lock)
just bootstrap-test                 11 pass / 0 fail  (65.6s)
fmt-check                           exit 0
ui-ci -> ui-core                    ok   (pnpm install --frozen-lockfile, tsc --noEmit,
                                          692 vitest tests / 92 files, vite build ok)
i18n checks                         ok
go vet ./...                        exit 0
go test -p 1 -timeout 35m -skip '^TestMemoryLoop' ./...   FAILED (see below)
```

Go packages that passed in that run, with the gate's own timings:

```text
internal/runtime            ok 416.950s
internal/runtime/compaction ok   1.652s
internal/rpc                ok 137.347s
internal/storage            ok   3.660s
internal/storage/sqlite     ok 134.017s
internal/storage/migrations ok   6.235s
internal/storage/postgres   ok   0.171s
internal/modules/diva-cognitive ok 8.487s
internal/provider           ok   7.520s
ui                          ok   2.456s
```

Failures in that run, both traced to causes outside the merge:

1. Symlink capability — 34 occurrences of `A required privilege is not held by the
   client`, all inside the workspace tree:
   - `internal/app/subject_binding_test.go:85` (`os.Symlink` for the owner alias);
   - `sdk/internal/assembly/failure_matrix_execution_test.go:63` (`catalog-symlink-escape`);
   - `sdk/internal/frontend_v1_test.go` (`link checked-in Web UI dependencies into
     isolated build boundary`), several subtests of
     `TestGenerationFailureMatrixExecutesEveryCase`,
     `TestPackAndInspectEveryRecipe`, `TestV1PackAndInspectProveRecipeRemoval`.
   - One additional `open C:\Users\com01\AppData\Local\go-build\33\33ec…-d: Access is
     denied`, the same permission class on the Go build cache.
2. Contract conflict — `internal/app/codeface_modes_test.go:213`, from main:
   `app: cognitive factory: diva-cognitive: open garden owner: recap binding:
   storage: subject binding mismatch` (see below).

### Root cause of the symlink failure (host token, not source)

A throwaway module under the gitignored scratch dir (since removed) called
`CreateSymbolicLinkW` directly with an explicit flag matrix, and verified each
attempt with `os.Lstat` instead of trusting the return value:

```text
raw 0x1 (privileged + directory)     r1=1280 GetLastError=1314 err=<nil> | Lstat: file not found, created=false
raw 0x2 (unprivileged only)          r1=1280 GetLastError=1314 err=<nil> | Lstat: file not found, created=false
raw 0x3 (unprivileged + directory)   r1=1280 GetLastError=1314 err=<nil> | Lstat: file not found, created=false
syscall.CreateSymbolicLink 0x3       err=A required privilege is not held by the client. | created=false
os.Symlink                           err=… A required privilege is not held by the client. | created=false
```

The Win32 call itself fails with `ERROR_PRIVILEGE_NOT_HELD` (1314) for **every**
flag combination, including `SYMBOLIC_LINK_FLAG_ALLOW_UNPRIVILEGED_CREATE`. Go's
generated wrapper applies the documented failure test `r1&0xff == 0`
(`$GOROOT/src/syscall/zsyscall_windows.go:552-558`) and correctly reports the
error; `os.Symlink` then retries without the flag
(`$GOROOT/src/os/file_windows.go:371-384`) and surfaces the same errno.

Token state of the gate shell:

```text
whoami                     -> mastwet-think\com01, Mandatory Label\Medium Mandatory Level
BUILTIN\Administrators     -> "Group used for deny only"      (UAC-filtered admin token)
whoami /priv               -> 5 privileges; SeCreateSymbolicLinkPrivilege absent,
                              SeBackupPrivilege absent
HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\AppModelUnlock
  AllowDevelopmentWithoutDevLicense = 1
HKLM\SOFTWARE\Policies\Microsoft\Windows\Appx\AllowDevelopmentWithoutDevLicense
                           -> unset (no policy override)
```

Cause: the process runs with a UAC-filtered administrator token that lacks
`SeCreateSymbolicLinkPrivilege`, and this host does not honour the Developer-Mode
unprivileged-create bypass despite that registry value. No source change can fix
it. Options are host-side: run the gate from an **elevated** shell (that token
carries the privilege), repair/refresh Developer Mode, or run these suites on a
host or CI where link creation is permitted.

**Correction to the first version of this section:** it claimed that a raw
`CreateSymbolicLinkW` succeeded while Go's `os.Symlink` was refused, and inferred
a Go-side or sandbox anomaly. That was a measurement error — the raw probe treated
a nonzero `r1` as success instead of applying the documented `r1&0xff == 0` test,
and never checked that a link existed. Re-measured with `Lstat`, every variant
fails identically.

Consequence: `internal/app/subject_binding_test.go` now probes host symlink support
and skips instead of failing. The `sdk/internal*` suites were **not** modified — they
remain red on this machine and their failure is not attributed to the merge.

### Contract conflict fixed (`TestCodeFaceJSONModeContinuesNewestSession`)

```text
go test ./internal/app -run 'TestCodeFaceJSONModeContinuesNewestSession' -count=1
# before: codeface_modes_test.go:213: continue run: status="" err=app: cognitive
#         factory: diva-cognitive: open garden owner: recap binding: storage:
#         subject binding mismatch
```

Root cause: the test gave the second invocation `Storage.DataDir = t.TempDir()` while
both shared one `SQLite.Path`. `Config.DataDirectory()` (`internal/config/config.go:1219`)
returns `Storage.DataDir` when set, else the journal's directory, else the user data
root, so an explicit override is the only way to diverge — and design revision 3
(`docs/superpowers/specs/2026-10-15-issue-58-unified-context-design.md:143-151`)
requires exactly that divergence to fail closed. The test now uses one data root for
both invocations, keeping its assertion that `--continue` reuses the newest journal
session:

```text
go test ./internal/app -run 'TestCodeFace' -count=1 -v
--- PASS: TestCodeFacePrintModeStreamsAssistantText
--- PASS: TestCodeFaceJSONModeEmitsOrderedRecords
--- PASS: TestCodeFacePrintModeCancelsBlockedRunLoudly
--- PASS: TestCodeFaceJSONModeContinuesNewestSession
--- PASS: TestCodeFaceRPCModeGoldenTranscript
--- PASS: TestCodeFaceRPCModeQueuesFollowUpWhileRunning
```

No production code was changed for this.

### Post-fix focused evidence

```text
go build ./...                                                        exit 0
gofmt -l internal/app/codeface_modes_test.go internal/app/subject_binding_test.go   (no output)
go test ./internal/storage/... ./internal/app ./internal/rpc -run '^TestIssue58' -count=1 -v
  exit 0   pass=30 (top level)  fail=0  skip=14
  --- SKIP: TestIssue58SubjectOwnerPathMismatch  (host does not permit symbolic links …)
  ok  internal/storage, storage/migrations, storage/postgres, storage/sqlite,
      internal/app, internal/rpc
```

The 13 other skips are the PostgreSQL-server suites that need
`VIVY_POSTGRES_TEST_DSN`; skips are never counted as passes.

### Not verified in this step

A full green `just ci` on this machine (blocked by the symlink defect above), the
`scripts/test-memory-loop.ps1` second pass, `diva-recall-test`, `headless-compile`,
`plugin-ci`, and the UI end-to-end browser smoke — `just test` aborted at the symlink
and build-cache failures before those recipes ran. Real PostgreSQL parity,
`-race` on the merged tree, real-model quality, and all of S3–S6 also remain
unverified at this candidate.

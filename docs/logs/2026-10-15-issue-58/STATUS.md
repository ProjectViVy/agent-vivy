# Issue #58 status board

Living progress record for the Issue #58 takeover lane. One entry per completed
step. Every entry names the exact commit it was produced at; a green row from a
different SHA is not evidence for the current candidate.

Plan of record: [Issue #58 takeover plan](../../superpowers/plans/issue-58/index.md)
(plan package lives on this lane) with the authoritative design at
[unified subject continuity design](../../superpowers/specs/2026-10-15-issue-58-unified-context-design.md).
Entry point policy from the takeover inventory: LIFE is one persistent subject
main conversation above Sessions; ordinary Coding boundaries stay unchanged.

Lane: `feat/issue-58-life-continuity`, branched from `66fabfd2`
(`origin/feat/issue-58-implementation`, 8 commits ahead of `main` `5715950d`).
Lane work happens in the root checkout; `main` is never moved. The only push in
this lane is the authorized Laputa test-helper fix recorded in T1b; nothing from
this repository has been pushed.

Current Laputa pin: `8c5328daa7a4e9cdd089d78858de087a36fb0843`
(`v0.0.0-20261015195414-8c5328daa7a4`, `refs/heads/feat/issue-58-recap`).

---

## T1 — Laputa pinned source recovered; local Windows baseline established

HEAD: T1 content commit `3777cf59d114df34f71125e6dd21120e9b7bc35f`
(lane parent before it: `66fabfd2`; board updated in the following docs commit)
Laputa pin: `bb57e93a61cfb41460524d00a557a3a9f2cce1e7` (parent `4c7c6b2…`), tree clean

Done:
- Recorded the pre-write scene: Vivy root clean at `5715950d`, no stash, single
  worktree; Laputa sibling clean and detached at `4b2bec2c`.
- Recovered the reviewed upstream owner from the lock: `just ensure-laputa`
  fetched and detached-checked-out `bb57e93`; verified parent, clean tree, all
  C1 owner files and the `reconcile` transition in writer, decoder and chain
  validator. No rebuild was needed and none is claimed.
- Re-ran the baseline gates on this machine. The first local run **failed**, which
  is the reason this step exists: 15 of 18 SQLite C2 suites and 5 app suites
  failed with `storage: subject binding mismatch`, and the pinned Laputa Windows
  suite failed on the owner lock file. See `verification.md` for the full
  before/after evidence.
- Fixed one real Windows portability defect in this repository: the shared C2
  conformance fixture hard-coded the POSIX owner path `"/trusted/owner"`, which
  `domain.SubjectIdentity.Valid` rejects on Windows because `filepath.IsAbs` is
  false there. The fixture now registers a native canonical path. The production
  validator was **not** weakened.
- Prepared (but did not publish) one upstream Laputa test-helper patch; recorded
  it as a blocker instead of silently editing another repository's pin.

Tests: pass=62 fail=0 skip=16 **event** totals for the focused `TestIssue58`
(storage/app/rpc) run — of which 56 pass and 13 skip are test-level and 6 pass
plus 3 skip are package-level (see T1b for the recount); pass=41 fail=0 skip=1
event totals for cognitive (`diva-cognitive`/`cognitivecontract`); broad `./internal/runtime ./internal/storage/...`
ok; `go vet ./...`, `go build ./...`, `git diff --check` and the `just ci`
`fmt-check` equivalent all exit 0; `just bootstrap-test` 11 pass / 0 fail.
Skipped (never counted as pass): 13 PostgreSQL-server `TestIssue58` suites without
`VIVY_POSTGRES_TEST_DSN`, plus existing external-GitHub and PostgreSQL migration
skips.

Remaining risk:
- `-race` is not executable in this environment (no mingw-w64 C toolchain); the
  concurrency/durability race gates are unverified locally.
- Native Windows durability evidence for the pinned Laputa owner exists only with
  an unpublished test-helper patch; the pinned tree alone still fails those tests
  on Windows. Blocked on authorization to publish an upstream Laputa commit.
- Garden's own module cannot build on Windows (`renameio` is `!windows` upstream);
  Garden evidence stays on a Linux/toolchain configuration.
- Real PostgreSQL execution and `just ci` on a frozen candidate are still open.
- The lane base is behind `main`; the merge has not happened yet (T2).

Next: T2 — merge `main` into this lane, resolve the 8 overlapping files while
preserving both `main`'s three fixes and this lane's C1/C2 increments, then rerun
the affected gates.

---

## T1b — Laputa Windows test-helper fix published; pin advanced

HEAD: T1b content commit `1266de6996582e37fee02e7e1774376992a10cf6`
Laputa pin: `8c5328daa7a4e9cdd089d78858de087a36fb0843` (parent `bb57e93…`),
published on `refs/heads/feat/issue-58-recap`, tree clean

The owner authorized publishing the upstream test-helper patch and re-pinning, and
authorized a local mingw-w64 install for the race lane. Both T1 blockers were then
resolved or advanced.

Done:
- Committed the one-helper fix in the Laputa checkout as `8c5328daa7a4` on branch
  `feat/issue-58-recap`, on top of the previously pinned `bb57e93`. Test-only change
  in `laputa/actmem/recap_test.go`; no production path, no authority contract and no
  `AGENTS.md` non-negotiable is touched.
- Pushed it as a fast-forward (`bb57e93..8c5328d`) and verified the remote ref by
  `ls-remote`. Direct GitHub connections are intermittent on this machine; the push
  succeeded through the machine's local proxy (`http://127.0.0.1:7892`).
- Advanced the coordinated pin in `go.mod` (three requires),
  `go.sum` (six new lines) and `laputa-source.lock.json` to
  `v0.0.0-20261015195414-8c5328daa7a4`. The `go.sum` hashes were not hand-derived:
  they were produced by downloading the three modules at the published version in a
  throwaway module without `replace`, then cross-checked line-for-line against the
  values recorded in this repository.
- Corrected an evidence-accuracy defect in the T1 record: the focused figures
  `pass=62 / fail=0 / skip=16` are **event** totals, not test totals. Of those
  events, 56 pass and 13 skip are test-level (top-level plus subtest) and 6 pass plus
  3 skip are package-level (the 3 are packages with no matching test). Both readings
  reproduce, and neither changed with the new pin, so nothing regressed.

Tests: Laputa native Windows at the published commit — actmem Issue58 pass=29
fail=0 skip=1, `go test ./...` for the `laputa` module ok; `just ensure-laputa`
exit 0; `just bootstrap-test` 11 pass / 0 fail; Vivy focused `TestIssue58` on the new
pin pass=62 fail=0 skip=16 (event totals); `go vet ./...`, `go build ./...`,
`git diff --check` all exit 0.

Remaining risk:
- The Windows test now depends on the lock being excluded from a snapshot helper.
  That is a test-visibility decision, not a durability claim: it removes an
  unreadable ownership artifact from a content assertion and adds no coverage. Real
  Windows durability still rests on the owner's own crash/recovery suites.
- `-race` remains unverified. `choco install mingw` was attempted and **failed**:
  this shell is not elevated and cannot write `C:\ProgramData\chocolatey\lib`.
  A user-space toolchain or a Linux lane is still needed.
- PostgreSQL, Garden-on-Windows and `just ci` on a frozen candidate remain open, as
  in T1.

Next: T2 — merge `main` into this lane, resolve the 8 overlapping files, rerun the
affected gates.

---

## Blockers awaiting an owner decision

1. ~~**Upstream Laputa test-helper patch.**~~ **RESOLVED in T1b.** The owner
   authorized publication; the fix is now Laputa `8c5328daa7a4` on
   `feat/issue-58-recap` and all three pins were advanced together. Native Windows
   durability evidence is no longer blocked on this.
2. ~~**`-race` toolchain.**~~ **RESOLVED in T1c.** `choco install mingw` fails
   without elevation, so the portable, admin-free WinLibs UCRT mingw-w64 was
   installed with `winget` instead. `-race` now runs on this machine and both
   relevant gates were executed with zero data races.

## T1c — `-race` resolved locally; concurrency gates executed

HEAD: T1c content commit — this docs commit. The toolchain change is outside this
repository, so this entry is the record; its SHA is written at the start of the next
step, since a commit cannot name itself.

Done:
- Diagnosed why every earlier attempt failed: `-race requires cgo`; `clang` rejects
  Go's `-mthreads`; `zig cc` links with `undefined symbol: WakeByAddressSingle`; and
  `choco install mingw -y` fails because this shell is not elevated and cannot write
  `C:\ProgramData\chocolatey\lib`.
- Installed the portable, admin-free WinLibs mingw-w64 (GCC 16.2.0, UCRT) with
  `winget install BrechtSanders.WinLibs.POSIX.UCRT`. No repository or recipe change.
- Ran the gates: focused `TestIssue58` under `-race` (storage/app/rpc, `-p 1`) and
  the pinned Laputa `actmem` suite under `-race`. Both exit 0 with **zero
  `WARNING: DATA RACE`**.

Tests: `go test -race -count=1 ./internal/domain` ok 3.621s (toolchain smoke);
`go test -race -p 1 -count=1 -run '^TestIssue58' ./internal/storage/... ./internal/app ./internal/rpc`
exit 0, 0 races (sqlite 77.8s, app 42.6s); `go test -race -count=1 ./actmem`
(Laputa) exit 0, ok 15.278s, 0 races.

Remaining risk:
- The toolchain is a per-command `PATH`/`CC` environment and is **not wired into the
  `just` recipes**, so a plain `just ci` still runs without `-race`. The S3/S4
  recap-context and compaction race gates do not exist yet, so they remain owed.
- `-race` detects data races only; it is not durability or crash evidence.

---

## T2 — `main` merged into the lane, gates rerun, environment limits recorded

HEAD: merge commit `0c6bdc74c024cc06d2bbbb0d17d27e4eeeeb9278`
(parents `a3c8ee25` lane `feat/issue-58-life-continuity`, `5715950d` `main`).
Laputa pin unchanged: `8c5328daa7a4e9cdd089d78858de087a36fb0843`.

Done:
- Merged `main` **into** the lane, preserving all 8 lane commits (no rebase, no
  push, `main` not moved). 59 files changed, +3106/-268.
- The prediction of "8 overlapping files" was wrong: only **2** files conflicted,
  the rest auto-merged. Both kept main's newer behaviour where it mattered:
  - `ui/src/routes/_layout.tsx` — kept main's `useCompanionAvatar` and
    `ShellOnboardingSlot`; dropped the `ConversationModePlaceholder` import whose
    component this lane already deleted, so the old Coding/LIFE placeholder stays
    removed.
  - `docs/TODO.md` — took main's five new `PERSONA-*`/`SKILL-OIL-AUTO-SYNC` rows and
    **dropped** main's `EINO-TOOLSNODE-ERR-RACE` open row, because the lane had
    already moved that id into the Eino remediation completion record (fixed by the
    pinned v0.9.20). No conflict marker remains anywhere in the tree.
- `.agents/skills/oil-frontend` takes main's newer gitlink
  `07a3c73001b061b7b90caba42767194205f917a8`, matching the on-disk submodule
  checkout and unblocking `just ci`.
- Found and fixed one genuine **cross-branch semantic conflict** that neither parent
  could see alone: main's `TestCodeFaceJSONModeContinuesNewestSession` gave the
  second `vivy-code` invocation a private `Storage.DataDir` while sharing the
  journal. Revision 3 pins one canonical ACTMEM owner directory to the subject and
  requires a binding mismatch when a changed path would open a second stream, so
  that premise now fails closed (`app: cognitive factory: diva-cognitive: open
  garden owner: recap binding: storage: subject binding mismatch`). The test keeps
  its real assertion — `--continue` reuses the newest session from the shared
  journal — with one shared data root, which is what `Config.DataDirectory()`
  already derives from the journal location when `Storage.DataDir` is unset. No
  production code changed for this.
- Made the lane's `TestIssue58SubjectOwnerPathMismatch` probe the host for symbolic
  link support and `t.Skip` when it is absent, instead of failing on an OS
  capability. The canonical-alias assertion still runs wherever links are
  permitted, and a skip is never counted as pass.

Tests: `go build ./...` and `go vet ./...` exit 0. Focused `TestIssue58`
(storage/app/rpc) exit 0 with 30 top-level pass / 0 fail / 14 skip (13
PostgreSQL-server suites plus the new symlink-capability skip); all six code-face
mode tests pass. Inside the full `just ci` run the Go half passed for
`internal/runtime` (416.9s), `internal/storage` (3.7s), `sqlite` (134.0s),
`migrations` (6.2s), `rpc` (137.3s), `postgres` (0.2s) and, after the fix,
`internal/app`. UI: `pnpm install --frozen-lockfile`, `tsc --noEmit`, **692 vitest
tests across 92 files**, `vite build`, and the three i18n checks all passed.
`just ensure-laputa` and `just bootstrap-test` (11 pass / 0 fail) passed.

Remaining risk:
- **A full `just ci` is not green on this machine, and the merge is not the cause.**
  The gate shell runs with a UAC-filtered administrator token at Medium integrity
  (`BUILTIN\Administrators` is deny-only) that lacks
  `SeCreateSymbolicLinkPrivilege`, and this host does not honour the Developer-Mode
  unprivileged-create bypass, so `CreateSymbolicLinkW` fails with
  `ERROR_PRIVILEGE_NOT_HELD` (1314) for **every** flag combination. Verified by
  calling the API directly and confirming with `Lstat` that no link is created.
  Every symlink-dependent suite therefore fails here: `internal/app` (now an
  explicit skip) and the `sdk/internal*` pack/UI-link suites. The fix is host-side:
  run the gate from an elevated shell, repair Developer Mode, or use a host/CI where
  link creation is permitted.
- `vite build` fails when esbuild unlinks its own temp file under the system `%TEMP%`
  (`Access is denied`), and succeeds when `TEMP`/`TMP` point at a workspace-local
  scratch directory. Both `just ci` attempts are recorded in `verification.md`.
- **Correction to T1:** the T1 figure "focused `TestIssue58` exit 0 including
  `internal/app`" is not reproducible on this machine — the symlink test fails
  before any product code runs. It is recorded here as unsupported rather than as
  evidence.

Next: T3 — S3 Task 1 (`prepareRecapContext`, `RecapContext`, host receipt codec),
with LIFE activation still closed until Task 2 lands and both actual-wire gates pass.

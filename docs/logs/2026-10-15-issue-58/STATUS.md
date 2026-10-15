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

HEAD: T1b content commit (see below)
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
2. **`-race` toolchain.** Still blocked. Not executable here: `-race requires cgo`;
   `clang` rejects Go's `-mthreads`; `zig cc` links with
   `undefined symbol: WakeByAddressSingle`. The owner authorized a local mingw-w64
   install, but `choco install mingw` fails because this shell is not elevated
   (`Unable to obtain lock file access on C:\ProgramData\chocolatey\lib\...`,
   `Access to C:\ProgramData\chocolatey\lib-bad is denied`). Needs an elevated
   install, a portable user-space toolchain, or a Linux/WSL race lane.

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
Lane work happens in the root checkout, `main` is never moved, nothing is pushed.

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

Tests: pass=62 fail=0 skip=16 focused `TestIssue58` (storage/app/rpc);
pass=41 fail=0 skip=1 cognitive; broad `./internal/runtime ./internal/storage/...`
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

## Blockers awaiting an owner decision

1. **Upstream Laputa test-helper patch.** The pinned owner holds `.owner.lock`
   under an exclusive byte-range lock for its whole lifetime; on Windows the
   `issue58Files` snapshot helper cannot read it, so 7 top-level and 22 subtest
   Windows cases fail on a pristine checkout. The exact minimal patch is in
   `verification.md`. Landing it needs a published commit in
   `ProjectViVy/laputa` plus a coordinated re-pin of `garden`/`laputa`/`mentle`
   in `go.mod`, `go.sum` and `laputa-source.lock.json`. No push is authorized.
2. **`-race` toolchain.** Not executable here: `-race requires cgo`;
   `clang` rejects Go's `-mthreads`; `zig cc` links with
   `undefined symbol: WakeByAddressSingle`. Needs a mingw-w64 gcc, or a
   Linux/WSL environment for the race lane.

# Acceptance

This step is infrastructure recovery, not a product feature. There is nothing new
to click, and no LIFE behaviour is enabled. A human can confirm the step in five
minutes with the checks below.

## What a human should be able to confirm

1. **The reviewed owner source is real, pinned and not rebuilt.**

   ```powershell
   just ensure-laputa
   git -C ../laputa rev-parse HEAD
   git -C ../laputa log -1 --format=%P
   git -C ../laputa status --porcelain
   ```

   Expect `bb57e93a61cfb41460524d00a557a3a9f2cce1e7`, parent
   `4c7c6b20859375d0d764d3ba2a0188926cf73c9e`, and empty status. The owner files
   `../laputa/laputa/actmem/recap.go`, `recap_format.go` and `recap_history.go`
   exist, and `recap_history.go` contains a reachable `"reconcile"` transition.

2. **The repository baseline passes on this machine, at this SHA.**

   ```powershell
   go test ./internal/storage/... ./internal/app ./internal/rpc -run '^TestIssue58' -count=1
   go test -mod=readonly ./internal/modules/diva-cognitive ./internal/cognitivecontract -count=1
   go vet ./...
   ```

   Expect exit 0 for all three. Before this step the first command failed in 20
   suites with `storage: subject binding mismatch`; a human can see the fix by
   reverting `internal/storage/conformance/subject_history.go` to the POSIX
   `"owner_path":"/trusted/owner"` literal and observing the same failures return.

3. **Nothing was lost or rewritten.**

   ```powershell
   git rev-parse main
   git log --oneline 66fabfd2 -3
   git stash list
   ```

   Expect `main` still at `5715950d`, the eight Issue #58 commits still present
   with their original SHAs, and an empty stash list. The Laputa sibling is clean
   at its pin — the local experiment there was reverted and is not part of any
   commit.

## What a human should *not* expect yet

- No LIFE entry appears above Sessions, and no Coding/LIFE mode button has been
  removed in this step: the button removal is part of the lane's earlier commit
  `536809c9` and only becomes visible in the app after T2 merges `main` and the UI
  is built.
- No Recap is written, no compaction runs, no memory UI exists. `S3`–`S6` are
  still unimplemented.
- Windows durability of the pinned owner is not yet an accepted result. The
  pristine pinned tree still fails its Windows suite on the owner-lock read; the
  passing numbers in `verification.md` come from an unpublished local patch.
- PostgreSQL and `-race` claims are absent by design, not by omission of a
  passing run.

## Decisions this step needs from the owner

1. Whether to publish the prepared one-helper Laputa patch and re-pin
   `garden`/`laputa`/`mentle` (no push is currently authorized). Until then,
   native Windows durability evidence stays `Pending`.
2. Whether to provide a mingw-w64 toolchain, a WSL/Linux lane, or accept `-race`
   as a documented gap. The concurrency gates in `S3`–`S4` depend on it.
3. Whether a PostgreSQL DSN can be supplied for the migration-parity rows.

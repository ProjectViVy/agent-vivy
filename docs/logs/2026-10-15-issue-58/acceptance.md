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

   Expect `8c5328daa7a4e9cdd089d78858de087a36fb0843`, parent
   `bb57e93a61cfb41460524d00a557a3a9f2cce1e7`, and empty status. The owner files
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
- PostgreSQL and `-race` claims are absent by design, not by omission of a
  passing run.

## What changed after the owner's authorizations

The owner authorized publishing the Laputa test-helper patch and a local
mingw-w64 install. The publication is done; the mingw install is not.

- **Windows durability is now a runnable, reproduced result.** A human can confirm
  it directly:

  ```powershell
  git -C ../laputa rev-parse HEAD          # expect 8c5328daa7a4...
  git -C ../laputa status --porcelain      # expect empty
  cd ../laputa/laputa; go test ./actmem -run '^TestIssue58' -count=1
  # expect ok, 29 pass / 0 fail / 1 skip
  ```

  And from this repository:

  ```powershell
  just ensure-laputa      # exit 0: lock and checkout agree
  just bootstrap-test     # 11 pass / 0 fail, includes the pin-vs-lock assertion
  ```

  Scope honesty: the fix excludes the lifetime owner lock from a test snapshot
  helper, because Windows holds it under an exclusive byte-range lock. That makes the
  suite runnable; it does not add durability coverage.

- **The mingw install succeeded through a different route.** `choco install mingw`
  fails without elevation, so the portable WinLibs mingw-w64 was installed with
  `winget install BrechtSanders.WinLibs.POSIX.UCRT` (GCC 16.2.0, UCRT). A human can
  confirm the race gates directly:

  ```powershell
  $mg = "$env:LOCALAPPDATA\Microsoft\WinGet\Packages\BrechtSanders.WinLibs.POSIX.UCRT_Microsoft.Winget.Source_8wekyb3d8bbwe\mingw64\bin"
  $env:PATH = "$mg;$env:PATH"; $env:CGO_ENABLED = '1'; $env:CC = "$mg\x86_64-w64-mingw32-gcc.exe"
  go test -race -count=1 ./internal/domain        # expect ok
  go test -race -p 1 -count=1 -run '^TestIssue58' ./internal/storage/... ./internal/app ./internal/rpc
  # expect exit 0 and no "WARNING: DATA RACE"
  ```

  Caveat: this environment is per-command. `just ci` does not set it, so a plain
  `just ci` still runs without `-race`.

## Decisions this step still needs from the owner

1. Whether the `-race` environment should be wired into the `just` recipes (and with
   which toolchain expectation), or stay a documented manual step. The concurrency
   gates in `S3`–`S4` depend on the answer for reproducible CI.
2. Whether a PostgreSQL DSN can be supplied for the migration-parity rows.

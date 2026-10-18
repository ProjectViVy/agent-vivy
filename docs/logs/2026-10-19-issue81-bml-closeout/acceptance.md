# Acceptance — issue #81 E1-BML closeout

This batch is evidence, not new behavior. A reviewer accepts it by re-running
four focused suites on the lane branch and confirming that each one measures the
real boundary it names.

## One-time lane setup

```bash
cd .worktrees/issue81-bml/agent-vivy
git log --oneline -4        # cf8f9bc6, 25915b32, 64cd48bf, 88200c8b
git diff --stat e9609296..HEAD   # only new test files
```

Required environment on this macOS host:

```bash
export GOTOOLCHAIN=go1.26.4 GOCACHE=/tmp/dsh-gocache
export XDG_CACHE_HOME=/tmp/dsh-xdg/cache XDG_DATA_HOME=/tmp/dsh-xdg/data
export XDG_STATE_HOME=/tmp/dsh-xdg/state XDG_CONFIG_HOME=/tmp/dsh-xdg/config
export COREPACK_HOME=/tmp/dsh-corepack
pnpm --dir ui install --frozen-lockfile   # once
pnpm --dir ui build                       # provides ui/dist for the Go embed
```

## Four commands, four claims

```bash
# R1 — public long-term state survives closing every owning handle.
go test ./internal/modules/memory -run '^TestBMLCloseoutPublicDiskLifecycle$' -count=1 -v

# R4 — stable EventID redelivery and a durable tombstone across reopen.
go test ./internal/modules/memory -run '^TestBMLCloseoutStableEventReopenReplay$' -count=1 -v
(cd bml && go test ./... -run '^TestBMLCloseoutTombstoneReplayAcrossReopen$' -count=1 -v)

# R2/R3 — a real default-App model request consumes the durable state.
go test ./internal/app -run '^TestBMLCloseoutActualModelConsumption$' -count=1 -v

# R5 — the durable memory page against a packed backend at the split origin.
cd ui
VIVY_BML_E2E_UI_PORT=3017 pnpm exec playwright test --config=playwright.memory.config.ts
```

Expected: every named test passes; the Playwright run prints the packed binary's
SHA-256 and `1 passed`. On a host where `:3015` is free, omit
`VIVY_BML_E2E_UI_PORT`; the config defaults to `:3015` and backend `:8806`.

## What a human should see in the UI run

Against a fresh `.workspace/issue81-bml-e2e/run`:

1. `/memory` starts with `No memories yet`.
2. Adding `bmluiprobe … alpha` shows the row, its id, and `Revision 1` both in
   the page and in an independent `vivy.memory.get` read.
3. Killing and restarting the backend, then reloading, still shows the same id
   and `Revision 1`.
4. Searching `alpha` finds the row; a nonsense query shows
   `No matching memories`.
5. With two tabs open on the same revision, the first tab saves `… beta`; the
   second tab's save keeps its dialog open with
   `This record changed since you opened it…`, keeps its draft, and the backend
   still holds the first tab's body at revision 2. The dialog's `Refresh`
   rebases it to that body.
6. Deleting requires a reason. After confirming, the row is gone, `get` returns
   `memory_not_found`, a reload stays empty, and so does a restart.
7. Stopping the backend and triggering a read shows
   `Failed to load memories`, never an empty list that looks successful.

## Boundaries a reviewer must not over-read

- These tests certify the default-Generation path on this candidate. They do not
  certify Windows `.exe` behavior, Linux acceptance, live-provider answer
  quality, or the repository-wide `just ci` gate.
- The deterministic provider in Story 02 proves that the composed ContextHost
  plumbing reaches the model request with the right record and revision. It does
  not prove that a real model would use the context well.
- Replay evidence is bounded: an identical stable `EventID` is idempotent across
  reopen, and a supported same-record replay cannot defeat a tombstone. There is
  no operator-visible history delete, no `memory_add` operation ID/exactly-once
  write, and no physical-erasure claim.
- The 39 pre-existing UI unit failures recorded in `verification.md` are outside
  this diff and outside this acceptance.

## Reviewer checklist

- [ ] Four Story commits are present on `feat/issue81-bml`, and
      `git diff --stat e9609296..HEAD` shows only new test and log files.
- [ ] The four commands above pass on a supported host (or the recorded macOS
      results are accepted with the `just ci` debt carried by the integrator).
- [ ] No new memory API, schema, or history-delete capability is claimed.
- [ ] The lane is not pushed or merged without separate authorization.

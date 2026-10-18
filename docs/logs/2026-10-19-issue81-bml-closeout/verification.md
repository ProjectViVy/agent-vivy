# Verification — issue #81 E1-BML closeout

Host: macOS (arm64), lane `.worktrees/issue81-bml/agent-vivy`, branch
`feat/issue81-bml` @ `88200c8b` (Story commits `cf8f9bc6`, `25915b32`,
`64cd48bf`, `88200c8b`). Date: 2026-10-19.

## Environment adjustments (and why)

`just` is not installed on this host and macOS is not a documented acceptance
venue (the contract names Windows and WSL2/Linux), so `just ci` was not run and
the gates below were run by hand.

The system Go (`1.27.1`) cannot build this module: `go test` fails with
`package internal/sysinfo is not in std`, and `go run ./sdk` fails with
`undefined: http2.TrailerPrefix`. Every Go command used
`GOTOOLCHAIN=go1.26.4` (downloaded through `goproxy.cn`).

Other environment variables required by the host sandbox:
`GOCACHE=/tmp/dsh-gocache` (the default build cache is not writable),
`XDG_CACHE_HOME/XDG_DATA_HOME/XDG_STATE_HOME/XDG_CONFIG_HOME=/tmp/dsh-xdg/*`,
`COREPACK_HOME=/tmp/dsh-corepack` (pnpm self-switching otherwise fails with
`ERR_PNPM_LOCKFILE_WRITE_FILE`). Pinned pnpm: `10.33.0`.

`go vet ./internal/app` initially failed with
`ui/embed.go:12:12: pattern all:dist: no matching files found`; a fresh lane
checkout has no `ui/dist`. Running `pnpm --dir ui build` in the lane produced it
and the vet/test gates then ran. That build is also what `go run ./sdk pack`
embeds, so it is a prerequisite, not a workaround.

## Story E1-BML-01 — public durable disk lifecycle

```console
$ GOTOOLCHAIN=go1.26.4 GOCACHE=/tmp/dsh-gocache \
  go test ./internal/modules/memory -run '^TestBMLCloseoutPublicDiskLifecycle$' -count=1 -v
--- PASS: TestBMLCloseoutPublicDiskLifecycle (0.01s)
ok  	agent-vivy/internal/modules/memory	0.329s

$ GOTOOLCHAIN=go1.26.4 GOCACHE=/tmp/dsh-gocache go test ./internal/modules/memory -count=1
ok  	agent-vivy/internal/modules/memory	0.308s

$ GOTOOLCHAIN=go1.26.4 GOCACHE=/tmp/dsh-gocache go test -race ./internal/modules/memory \
  -run '^TestBMLCloseoutPublicDiskLifecycle$' -count=1
ok  	agent-vivy/internal/modules/memory
```

The test opens the same SQLite root through `memory.Open`, writes long-term state
through the module action plane, closes every owning handle (`memory.Close`),
reopens the root, and observes the same record identity, content, revision, list
ordering, search hit, and exclusion after removal. It also asserts a stale
`base_revision` correction fails and leaves the stored body untouched.

## Story E1-BML-03 — replay boundaries

```console
$ GOTOOLCHAIN=go1.26.4 GOCACHE=/tmp/dsh-gocache \
  go test ./internal/modules/memory -run '^TestBMLCloseoutStableEventReopenReplay$' -count=1 -v
--- PASS: TestBMLCloseoutStableEventReopenReplay (0.00s)

$ (cd bml && GOTOOLCHAIN=go1.26.4 GOCACHE=/tmp/dsh-gocache \
  go test ./... -run '^TestBMLCloseoutTombstoneReplayAcrossReopen$' -count=1 -v)
--- PASS: TestBMLCloseoutTombstoneReplayAcrossReopen (0.01s)
ok  	github.com/ProjectViVy/agent-vivy/bml	0.339s

$ GOTOOLCHAIN=go1.26.4 GOCACHE=/tmp/dsh-gocache go test ./internal/modules/memory -count=1
ok  	agent-vivy/internal/modules/memory
$ (cd bml && ... go test ./... -count=1)
ok  	github.com/ProjectViVy/agent-vivy/bml
$ GOTOOLCHAIN=go1.26.4 GOCACHE=/tmp/dsh-gocache go test -race ./internal/modules/memory \
  -run '^TestBMLCloseoutStableEventReopenReplay$' -count=1
$ (cd bml && ... go test -race ./... -run '^TestBMLCloseoutTombstoneReplayAcrossReopen$' -count=1)
both ok
```

The provider half redelivers the identical committed `EventID&&Seq` object after
a full service close/reopen (twice) and requires a byte-identical
`observer.DeliveryReceipt`, exactly one durable history candidate with unchanged
content/version/evidence, and no long-term listing of the history record.

The library half proves a superseding tombstone survives reopen: `SearchVisible`
returns no hit, the tombstone row and original row are both retained by raw
`List`/`Get`, an insert-only replay at base revision 0 fails with
`ErrRecordRevisionConflict`, and an explicit rewrite at the observed revision is
allowed. These are exactly the guarantees the plan allows; nothing here claims
a history delete or an exactly-once state-machine property.

## Story E1-BML-02 — actual model request consumption

Fixture: a loopback `httptest` server speaking `chat.completions`. The handler
parses the raw request bytes, finds `[context: vivy.memory.bml/...` in a user-role
text part, and answers with the block body verbatim; it returns `BML_UNKNOWN`
when no block exists. It never receives the expected answer, the record content,
or database access.

```console
$ GOTOOLCHAIN=go1.26.4 GOCACHE=/tmp/dsh-gocache \
  go test ./internal/app -run '^TestBMLCloseoutActualModelConsumption$' -count=1 -v
--- PASS: TestBMLCloseoutActualModelConsumption (0.62s)
    --- PASS: .../absent
    --- PASS: .../present
    --- PASS: .../corrected
    --- PASS: .../removed

$ ... go test -race ./internal/app -run '^TestBMLCloseoutActualModelConsumption$' -count=1
ok  	agent-vivy/internal/app	6.067s

$ ... go test ./internal/app -run \
  '^(TestEinoImportsQuarantined|TestDefaultGenerationComposesEstablishedOptionalHosts|TestLoopbackControlCompletesApprovedConversation)$' -count=1
ok  	agent-vivy/internal/app

$ ... go vet ./internal/app   # with ui/dist built
(no output)
```

Each subtest owns a disposable data/SQLite/workspace root and one scenario:
absent, present, corrected (add revision 1, CAS update to revision 2), removed
(add, update, tombstone with reason). Compose the ordinary App, prepare state
through `module.action.invoke` on `vivy/memory-bml` under `full_auto`, close the
control peer and the App, compose a second App on the same root, open a fresh
session, start one turn, and wait for `completed` through `run/get`.

Assertions read only what the provider received and the assistant projection:
the outbound `model` is the raw `deepseek-flash`, the current body and
`version=<revision>` framing appear in a user message, the superseded body never
appears after correction, system messages carry no BML material, absent/removed
produce no BML block and `BML_UNKNOWN`, and the assistant text equals the
request-derived body (or control value).

## Story E1-BML-04 — real split-origin UI

```console
$ cd ui
$ VIVY_BML_E2E_UI_PORT=3017 pnpm exec playwright test --config=playwright.memory.config.ts
memory e2e: reusing packed binary .../pack/vivy
memory e2e: packed binary sha256=fab3459ab7f69199cbdb68b8c247855fe7d9c258f574246ac0af26a7d811ac6d
  ✓  1 [chromium] › e2e/memory.spec.ts:67:1 › memory UI keeps every claim the browser can make true across a restart (6.8s)
  1 passed (8.6s)
```

Pack command (run once by the config; rerun with `VIVY_BML_E2E_REPACK=1`):

```console
$ go run ./sdk pack --recipe recipes/default.vivy.yml \
    --output .workspace/issue81-bml-e2e/run/pack
```

Flow exercised at `http://127.0.0.1:3017/memory` (same-origin Vite `/rpc` proxy
to the packed backend on `127.0.0.1:8806`, no embedded `:8787`):

1. Empty state, add through the real dialog, verify the row and
   `Revision 1`, and read the same id/content/revision over
   `vivy.memory.list`/`get` through a separate control WebSocket.
2. Stop and restart the backend process, reload the page, and re-read the same
   id and `Revision 1` in the detail pane and through the action plane.
3. Search hits the record; a nonsense query shows `No matching memories` and no
   fabricated row.
4. Two tabs open the same revision; the first saves revision 2, the second
   gets the in-dialog conflict alert, keeps its draft, and an independent read
   still returns the first tab's body at revision 2. Refresh rebases the dialog
   to the authoritative body.
5. Delete requires a reason before enabling the destructive button; after the
   tombstone the row disappears, `get` returns
   `{status:"failed", reason:"memory_not_found"}`, a reload stays empty, and a
   backend restart stays empty.
6. Stop the backend, trigger a read, and the page shows its error alert
   (`Failed to load memories`) instead of a successful empty list.

UI gates:

```console
$ pnpm --dir ui build          # pass, 1.77s
$ pnpm --dir ui typecheck     # pass
$ pnpm --dir ui test          # 663 passed, 39 failed in 9 files
```

The 39 unit failures are **pre-existing on this host and unrelated to this
change**. They live in `src/**` / plugin test files that this branch never
touches (`git diff --stat e9609296..HEAD -- ui/src plugins` is empty), and the
same failure reproduces in the untouched shared root checkout, e.g.
`pnpm exec vitest run src/components/chat/ConversationSidebar.test.tsx` fails 6/6
there with the same React `act` / localStorage errors. They are not used as
evidence for this batch.

## What was not run, and why

- **`just ci`** — `just` is absent and macOS is not a documented CI venue. The
  equivalent individual gates above were run instead; the integrator still owes
  the full `just ci` on Windows or WSL2/Linux.
- **Full `internal/app` package run** — started, then abandoned after it hung for
  ~25 minutes while two unrelated lanes were simultaneously running
  `go test ./...` / `go test ./internal/app` in this worktree root. The hang was
  not attributed to the new test, and the targeted app tests, `-race` run,
  quarantine check and composition check all pass. A clean, unloaded full run is
  left to the integrator.
- **Broad `go test ./...`** — not run, for the same reason and because the change
  only adds test files in three already-covered packages.
- **Windows `.exe` / Linux acceptance** — not available from this host.
- **Live provider calls** — none; every model response is deterministic and
  loopback.

## Deviation from the plan's suggested defaults

- The plan suggested Vite `:3015`; an unrelated Vite process already owns that
  port on this host, so the spec ran with `VIVY_BML_E2E_UI_PORT=3017` and
  `strictPort`. Defaults are left at `:3015`/`:8806` in the config, overridable
  with `VIVY_BML_E2E_UI_PORT` / `VIVY_BML_E2E_BACKEND_ADDR`.
- `just` recipes were replaced by the explicit commands recorded above.

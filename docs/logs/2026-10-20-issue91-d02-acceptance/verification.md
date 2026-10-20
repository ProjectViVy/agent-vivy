# Verification

Final acceptance uses the owner-authorized WSL/scoped route recorded in
`../2026-10-21-wsl-kernel-ci/verification.md`. Wiring, kernel, D02 Faces/Web,
strict conformance reproduction, generation failure/removal/rollback, real
PostgreSQL and race checks pass. The optional ripgrep skip is disclosed there.
The older pending Windows-full-CI notes below are historical and superseded
by that explicit route; no full-product CI success is claimed.

## Reproduced failures and focused corrections

- Native codeclient tests failed when Windows attempted to execute bash
  shebang files. Both correlation/error tests pass with a native helper.
- Shutdown tests reproduced locked `garden.db` files. The new listener-failure
  regression also demonstrated that storage remained readable after Run
  returned its listen error. Shared teardown passes both regressions and the
  existing admission/channel-drain, bounded shutdown and CloseContext cases.
- Prefix/spill tests reproduced missing external `echo` and unsupported
  embedded-printf precision. Their portable fixtures pass unchanged behavioral
  assertions for prefix isolation, exact output size, truncation and spilling.
- Real CODE subprocess suite passes with isolated Persona initialization.
- Conformance update guards were RED before their helpers existed and now
  prove failed, missing and foreign-source results leave old evidence intact.
  An empty selector or skipped test cannot count as execution evidence.
- `just ui-ci` passes: 83 files, 654 tests, typecheck, Vite production build,
  i18n completeness and cross-Face semantics.
- Actual SDK UI pack/Inspect first reproduced missing Windows symlink
  privilege. With the ancestor-resolved scratch directory it passes outside
  the restricted process sandbox (50.747s). The restricted child Node process
  returned Windows loader status `0xc0000142`; no product failure is hidden by
  accepting that run.

## Independent review correction

The independent read-only reviewer found one P1: calling the shared Close
owner synchronously also inherited its unbounded MCP close wait, whereas Run
previously bounded that wait. A real loopback fixture opens two legacy HTTP
MCP sessions and stalls their DELETE responses. It first failed beyond the
shutdown grace window. The shared owner now bounds its MCP wait by the same
shutdown context, continues storage teardown and preserves the timeout cause.
The correction and existing lifecycle subset pass (9.514s). No other
substantiated issue was reported by the reviewer; no duplicate review was run.

The earlier complete producer pass (23 suites, 196.864s) precedes this last
internal-source correction, so it cannot attest the final identity. The
associated full CI attempt was stopped during vet and is diagnostic only.
Final producer regeneration and full CI run sequentially after the correction.

## Environment

The real PostgreSQL gate exposed an invalid shared CN-21 fixture: it wrote a
forked-from marker for `sess-fk` without creating that session. PostgreSQL's
foreign key correctly rejected it. Adding the missing fixture session makes
the same CN-21 case pass against PostgreSQL (0.994s); no production storage
behavior or released migration changed. The complete `just test-postgres` gate
then passed on WSL PostgreSQL (76.259s), with `GOFLAGS=-p=2 -count=1` and a real
disposable DSN. It did not skip. Source conformance is regenerated after this
correction.

Docker is available inside WSL Debian (`26.1.5+dfsg1`), accessed through
`wsl -d Debian -- docker ...`. Windows' default Docker pipe alone is not a
valid daemon availability check. A task-owned PostgreSQL 16 container bound
to WSL's private interface on port 55404 executes the complete gate. Earlier
attempts on a portable Windows PostgreSQL 16.15 cluster on `127.0.0.1:55403`
failed on intermittent TCP dials; those remain failures and did not waive the
gate. That native cluster was stopped after the WSL gate passed. No production
volumes or existing containers are used.

Go is added to process PATH only. GOCACHE is scoped to this checkout's ignored
task scratch. The initial CI used checkout-local TEMP/TMP and exposed three
environment-sensitive fixture failures after 1514.236s of runtime tests:
instruction discovery found the enclosing repository, and ripgrep inherited
the ignored scratch ancestor. All three exact cases pass unchanged (0.186s)
with task-owned TEMP/TMP outside Git. That failed CI was stopped during the SDK
matrix; only the subsequent complete external-temp run can accept the cut.
The final TEMP/TMP path is
`C:/Users/Administrator/AppData/Local/Temp/vivy-issue91-d02-check`.
`CI=true` enables noninteractive pnpm, and
`npm_config_store_dir` selects its already-populated store. No global settings,
production Journals, real-provider credentials or dependency pins are changed.

## Required final checks

- Final real producer update: passed, all 23 suites, 424.940s. Its ordinary
  strict comparison gate remains part of the pending complete CI command.
- Real PostgreSQL backend gate: passed, 76.259s, on disposable WSL Docker.
- Generation failure/removal/rollback matrix: pending.
- Final `just ci`: pending.
- Independent review: complete; the one P1 received its RED/GREEN correction.
- Exact `git diff --check`: passed; rerun after final documentation updates.

The producer update command is:

```text
go test ./sdk/internal/conformance -run '^TestCheckedInProviderConformanceMatchesExecutedSuites$' -count=1 -timeout 20m -v -args -update-provider-conformance
```

Ordinary execution omits the update flag and never modifies evidence. The
historical D01/D02 failure inventories remain historical records.

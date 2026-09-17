# Console start guard — honest start results and lease-aware restart

Date: 2026-09-17
Scope: Vivy Studio overlay (`studio/dsh-vivy-console`, the first-party console
plugin), plus an environment repair and one dev-pair bring-up script in Studio
scratch. No Vivy kernel change.

## Why

The user reported that the Vivy Console could not start the frontend. Two
independent causes were found, both surfacing as the same product defect: the
console reported a start that never happened.

1. **Environment**: `ui/node_modules` was a half-linked pnpm install — the
   `.bin/` directory and `.modules.yaml` were gone and the `vite` package
   directory itself was missing, so `pnpm dev` exited within milliseconds with
   `'vite' is not recognized as an internal or external command`. Six
   consecutive console starts each answered `ok:true` with a PID.
2. **Console contract**: `startBackend` / `startFrontend` returned success
   immediately after `spawn` without waiting for the child and without reading
   its log. Any early exit — including the kernel's `organism lease held` on a
   restart inside the 30s lease TTL — was reported as "已启动".

## What changed

`studio/dsh-vivy-console/index.js` (the only product file):

- **`awaitReady(proc, port, timeoutMs)`** — polls until the child either opens
  its port or exits (`READY_TIMEOUT_MS = 30000`). `spawn` failures that arrive
  asynchronously as an `error` event are treated as an exit.
- **`startFailureDetail` + `freshLogTail` + `freshTailOf`** — a failed start now
  returns `ok:false` with the exit status and the lines the child wrote since
  the start began, instead of a bare failure or a fake success.
- **`leaseWaitMs`** — a hard-stopped backend keeps its organism lease for up to
  `internal/storage/sqlite:leaseTTL` (30s), so an immediate restart of the same
  Journal fails with `storage: organism lease held`. When the console itself
  just stopped that backend (`lastBackendStopMs`) and the fresh log says the
  lease is held, the start now waits out the remaining TTL and retries once.
  A lease held by anything else is still reported, never waited on.
- **`exitText`** — negative exit codes (libuv spawn failures such as `-4058`
  = ENOENT) are reported as launcher errors, not as child exit statuses.
- **`readLogText` / `readLogLines`** — the log-tail reader was extracted from
  `readLogs` so the status feed and the failure reports share one implementation.

Not changed: the kernel lease semantics, the hard-kill stop path (Node cannot
deliver a graceful console signal to another process on Windows), and the
client half (`.vc-msg` already renders `white-space:pre-wrap`, so multi-line
failure text displays as-is).

## Environment repair (not a repository change)

`pnpm install --frozen-lockfile` in `ui/` relinked 374 packages entirely from
the local pnpm store (`downloaded 0`, 16.6s), restoring `.bin/` and
`.modules.yaml`. `data/`, including the profile install and the two smoke
scripts, is ignored scratch.

## Explicitly not done

- No Go/kernel edit: the 30s lease is respected, not weakened, and the console
  never touches lease rows itself.
- No graceful stop for the backend: the console waits out the lease instead of
  bypassing it. A real fix (graceful `vivy` shutdown from the console on
  Windows) is a kernel/console lifecycle change that is not attempted here.
- The `studio/` submodule gitlink in the host repository was not staged, so the
  recorded pointer still differs from the submodule HEAD exactly as before this
  change.

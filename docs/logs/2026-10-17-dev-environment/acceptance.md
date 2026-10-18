# Acceptance

## Daily use stays untouched

1. Run `just doctor` from the repository root. Expect 28 ok checks, no
   failures, and (on a Windows machine without Docker) one warning that the
   Docker CLI is not on PATH.
2. Start the split loop with `just dev` (or `.\dev.ps1`). Open
   `http://127.0.0.1:3016`. The VIVY page and first-run wizard should render.
3. Confirm the backend reports `http://127.0.0.1:8788` and the data root
   `.workspace/dev/home`; the Journal is `.workspace/dev/home/vivy.db`, not
   `data/vivy.db`.
4. Stop the loop with Ctrl+C. `netstat -ano | grep -E ':8788|:3016'` should
   show no LISTEN sockets. The daily `~/.vivy` home and the embedded `:8787`
   instance were never opened.
5. To use another port or overlay, set `VIVY_DEV_BACKEND_PORT`,
   `VIVY_DEV_UI_PORT`, or `VIVY_DEV_CONFIG`. An exported `VIVY_CONFIG` is
   ignored by the dev entries.

## Proxy behavior

- `just setup` and `just dev` never write `go env -w`. Set `VIVY_GOPROXY` to
  override the mirror for one project.
- If the mirror returns a non-zero result (including 502), setup retries
  `go mod download` once with `GOPROXY=direct` and then fails with the
  original error. It does not disable checksum verification or retry forever.

## Parallel lanes

1. Run `just lane-new demo-lane`.
2. Expect `.worktrees/demo-lane/agent-vivy` on branch `feat/demo-lane` and a
   pinned `.worktrees/demo-lane/laputa`.
3. Run `just doctor` and `just dev` inside the lane with a port override. The
   root tree and its Laputa checkout remain untouched.

## Docker packaging

On a machine where Docker runs (WSL/Linux), from an empty parent directory:

```bash
git clone <candidate> agent-vivy && cd agent-vivy
git clone <laputa> ../laputa && git -C ../laputa checkout <lock-commit>
node scripts/ensure-laputa.mjs --quiet
VIVY_GOPROXY=<reachable-proxy> VIVY_NPM_REGISTRY=<reachable-registry> \
  node scripts/docker-build.mjs --tag vivy:acceptance --smoke
```

A passing run prints `smoke passed; container healthy on an empty /data`.
The 2026-10-17 candidate completed this step through two runs of the same
image: an offline `--network=none` build from host-seeded BuildKit caches,
and `node scripts/docker-build.mjs --tag vivy:acceptance --smoke`. A registry
timeout or an unreachable proxy is still not acceptance; on a host with
working registries, run the command above from a cold cache.

## CI

The aggregate CI job requires all lanes: Windows backend, Windows UI, Windows
browser, Linux backend, Linux kernel race, and real Postgres. A skipped
Postgres lane fails the `postgres` job because `VIVY_POSTGRES_TEST_DSN` must be
set. The Linux race lane currently fails on the tracked upstream
`EINO-TOOLSNODE-ERR-RACE` (pinned Eino v0.9.13 `tool_node.go:1253`); this is a
real gate result, not a venue problem, and it must not be skipped to force a
green run. Local quick work uses the affected packages plus `just check-fast`;
the full gate is `just ci` on a supported Node version, with the race and
Postgres lanes run separately or by CI.

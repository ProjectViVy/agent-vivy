# Development environment

Status: current operating contract (2026-10-17). This page describes how the
three venues divide work, how to diagnose them, and what each gate proves.

## Venue split

| Venue | Use | Not responsible for |
|---|---|---|
| Windows (native) | Editor, browser, daily `vivy.exe`, Windows path/file-lock/symlink/exit-cleanup acceptance | Primary compile/test loop |
| WSL2 / Linux (code on the Linux filesystem) | Go/Node/pnpm builds, unit tests, race tests, the main development line | Native `.exe` behavior |
| Docker (daemon in WSL) | Postgres, packaged image startup acceptance | Inner-loop compilation |

Keep Linux checkouts under the Linux filesystem (for example `~/work/`), not
under `/mnt/c`. Cross-filesystem I/O and file watching are the main source of
slow or flaky WSL builds. A Windows checkout may exist separately for `.exe`
acceptance, but it must use the same commit SHA and dependency versions; do
not share `node_modules`, a running database, or `.workspace/` between the two
checkouts.

On this host `.wslconfig` caps WSL at 8 vCPU, 8 GB memory, and 4 GB swap. The
Go gates already run with `-p 1`, so one suite at a time fits; do not raise the
package parallelism to shorten a gate.

A non-login WSL shell does not inherit nvm, so `node` can resolve to Debian's
Node 20 and `just doctor` reports `Go/UI tooling requires Node 22+`. Put the
supported Node 24 bin directory first in the shell that runs `just ci`, for
example `export PATH="$HOME/.nvm/versions/node/v24.21.0/bin:$PATH"`. The same
applies to `just` and `go` when they were installed under `~/.local`.

## Diagnose first

```text
just doctor
```

`scripts/doctor.mjs` is read-only. It checks tool paths and versions, the Go
version against `go.mod`, the Laputa lock and checkout, local `replace`
targets, the effective proxy and its reachability, dev/daily ports, write
access, UI dependencies/artifacts, and the dev versus daily data roots. It
never runs `go env -w`, changes ports, installs tools, or deletes files.

Exit code 1 means a failure that blocks startup. Warnings are non-fatal but
should be read before a full gate.

## Dev instance versus daily Vivy

`just dev` is deliberately not the daily organism:

| Fact | Dev (`just dev` / `just run`) | Daily / embedded |
|---|---|---|
| Backend | `http://127.0.0.1:8788` | `http://127.0.0.1:8787` |
| Vite | `http://127.0.0.1:3016` | `:3015` when run manually |
| Data root | `.workspace/dev/home` (`VIVY_USER_HOME`) | `~/.vivy` |
| Config | `scripts/dev.config.yaml` (empty overlay) | `config.yaml` / `VIVY_CONFIG` |
| Process | `go build -tags vivy_headless` + Vite | packaged embedded UI |

Overrides:

- `VIVY_DEV_BACKEND_PORT`, `VIVY_DEV_UI_PORT` — use different ports for each
  concurrent lane.
- `VIVY_DEV_CONFIG` — use a different dev overlay. Without it, the dev entries
  point `VIVY_CONFIG` at `scripts/dev.config.yaml`, so an exported
  `VIVY_CONFIG` and the daily `config.yaml` are both ignored.
- `VIVY_GOPROXY` — project-level proxy override. Precedence is
  `VIVY_GOPROXY` → existing `GOPROXY` → `https://goproxy.cn,direct`.
- `VIVY_NPM_REGISTRY` — npm registry override for Docker image builds;
  the compose build args and `scripts/docker-build.mjs` both read it.

Recipes use a process-local `GOPROXY`. `just setup` does not persist a mirror
with `go env -w`, so one project cannot change another project's proxy.

Go's comma fallback only advances on 404/410. A 502 from a mirror does not
fall through to `direct`; `scripts/setup.mjs` retries the download once with
`GOPROXY=direct` and then fails with the original error. It never disables
checksum verification and never retries indefinitely.

## Parallel lanes

The root checkout is one write lane. A second lane gets its own parent
directory with a nested worktree and a matching pinned Laputa checkout:

```text
just lane-new diva-recall
# .worktrees/diva-recall/agent-vivy   git worktree, branch feat/diva-recall
# .worktrees/diva-recall/laputa       its own checkout at the lock commit
```

`../laputa` is resolved relative to the checkout, so a nested lane never
shares a mutable Laputa with the root or another lane. A flat
`.worktrees/<lane>` layout shares `.worktrees/laputa` and is not supported
for new lanes; `just doctor` warns when it sees one.

Each worktree has its own ignored `.workspace/`, `data/`, and `ui/node_modules`.
Run `just doctor` and `just dev` inside the lane. Use the port overrides above
when more than one lane runs at once.

## Docker

The image build needs the repository plus the pinned Laputa source, which is
outside the main build context. The build therefore uses a named BuildKit
context:

```text
just docker-build          # verifies the clean pin, passes --build-context laputa=../laputa
just docker-build --smoke  # also starts the image on an empty /data and waits for /healthz
just docker-up             # compose; additional_contexts supplies the same laputa context
```

`scripts/docker-build.mjs` fails closed when the Laputa checkout is not the
clean commit in `laputa-source.lock.json`, when `go.mod` has a local
`replace` target whose `go.mod` is not copied before `go mod download`, or
when `ui/package.json` has a `file:` dependency whose directory is not copied
before `pnpm install`. `go mod download` resolves the whole module graph, so
copying only the root `go.mod`/`go.sum` is insufficient.

The build stage carries both Node and Go: `pnpm build` runs the UI projection
step, which invokes `go run ./sdk stage-ui` against the Recipe. Keeping one
source copy for both toolchains avoids a second UI stage that cannot generate
its own inputs. `node:*-slim` has no system CA bundle, so the build copies the
matching bundle from the Go image for Go's module verification. Corepack also
fetches pnpm itself; `COREPACK_NPM_REGISTRY` is bound to `NPM_REGISTRY` so the
whole build uses one reachable registry. Every layer that invokes `pnpm`
mounts the corepack cache, because the shim resolves the prepared pnpm
through that cache; without the mount a later layer redownloads pnpm even
after `corepack prepare`. The build stage has no VCS client, so `GOPROXY`
must name a reachable proxy rather than `direct`. The runtime stage also
copies the CA bundle instead of running apt, and its healthcheck probes
`/healthz` over bash `/dev/tcp`; the base already carries the
`Asia/Shanghai` zoneinfo, so no package index is needed at runtime.

Acceptance for a packaging change is not "it built on this machine". Use an
empty parent directory containing exactly the repository at the candidate SHA
and a Laputa checkout at the lock commit:

```bash
mkdir -p .workspace/docker-acceptance && cd .workspace/docker-acceptance
git clone https://github.com/ProjectViVy/agent-vivy.git
cd agent-vivy
git checkout <candidate-sha>
git clone https://github.com/ProjectViVy/laputa.git ../laputa
git -C ../laputa checkout <lock-commit>
node scripts/ensure-laputa.mjs --quiet
node scripts/docker-build.mjs --tag vivy:acceptance --smoke
```

The smoke uses a tmpfs `/data`, so it proves first-start schema creation and
`/healthz` without inheriting any existing Journal. Record the candidate SHA,
Laputa SHA, Docker version, and any skipped check. The build needs a reachable
Go module proxy and npm registry; `VIVY_GOPROXY` / `VIVY_NPM_REGISTRY` override
the image defaults. A registry timeout is an inconclusive environment blocker,
not a passing build.

The pnpm store, corepack cache, Go module cache, and Go build cache are
BuildKit cache mounts, so a host that already holds them can run the build
with `--network=none`. Use that on a host whose WSL DNS returns proxy fake
IPs or whose proxy listens only on Windows loopback: seed the mounts from the
host caches, then run the acceptance. That proves the Dockerfile inputs are
complete, not that the public registries are reachable.

## Test layering

| When | Run | Counts as evidence for |
|---|---|---|
| Inner loop | Affected Go packages, `go build ./...`, UI typecheck/unit, `just check-fast` | The changed area only |
| Before PR | `just ci` (Windows or Linux); Linux kernel race lane; UI/E2E where the surface changed | Repository gate on that OS |
| PR CI | Windows backend/UI/browser, Linux backend, Linux kernel race, real Postgres | Merge gate; every lane must succeed |
| Windows acceptance | Real packaged `.exe`, native paths/file locks/symlinks/exit cleanup | Windows platform behavior |
| Release | One fixed candidate SHA; record Laputa SHA, tool versions, skipped items | Release candidate |

Do not start every full suite for a one-button change. Do not count a skipped
Postgres run as a pass: `just test-postgres` exits non-zero when
`VIVY_POSTGRES_TEST_DSN` is unset, and CI supplies a real `postgres:16`
service.

The Linux `race-ci` recipe covers `./internal/...`; `test-race` remains the
full-tree option. The split exists to keep PR feedback bounded, not to weaken
the kernel's concurrency evidence.

The lane is currently red on the pinned Eino v0.9.13 race in
`compose/tool_node.go:1253` (`EINO-TOOLSNODE-ERR-RACE`). Report that failure;
do not skip the affected tests or describe the lane as passing. A first-party
race found by the same lane (`sdk/tui/face/rpc.go` `sessionID`) is fixed.
Adopting the upstream fix is a separate, authorized dependency change.

## Reproducibility boundary and deferrals

This cut makes the existing three-venue setup reproducible with fixed tools
and pins. It does **not** introduce a Dev Container, a remote runner, a
dependency cache shared between worktrees, or a second configuration source.
Those are deferred until the current lanes are shown stable.

The SDK Assembly `undefined` field, a missing `vivy.exe`, and a Windows
owner-test failure are separate defects. They belong to the generation
contract, artifact preparation, and platform diagnosis respectively; a
green Linux lane does not close them.

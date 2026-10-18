# Development environment split

## What changed

The repository now has one Windows daily/acceptance line and one Linux/WSL
development line, with Docker for dependency services and packaged startup
acceptance. The split is implemented by thin Node helpers plus `just`, not by a
second build framework.

- `justfile` keeps its command names but delegates portable work to
  `scripts/*.mjs`; Windows-only operations stay in `dev.ps1` and
  `scripts/ensure-studio.ps1`. Build outputs and recipe commands resolve per
  platform.
- `scripts/doctor.mjs` is a read-only diagnostic for tools and versions,
  Laputa pins, local replaces, proxy reachability, ports, write access, UI
  dependencies/artifacts, and the dev/daily data roots. It never runs
  `go env -w` or changes machine settings.
- `just dev` (Windows `dev.ps1`, Linux `scripts/dev.sh`) runs the backend on
  `127.0.0.1:8788` and Vite on `127.0.0.1:3016`. `scripts/run-dev.mjs` is the
  backend-only entry. Both use `.workspace/dev/home` as `VIVY_USER_HOME` and
  load `scripts/dev.config.yaml`, an intentionally empty overlay, so neither
  the repo `config.yaml` nor an inherited `VIVY_CONFIG` redirects dev writes to
  `data/`. `VIVY_DEV_CONFIG` selects another overlay.
- Proxy configuration is process-local. `just setup` and the dev entries use
  `VIVY_GOPROXY` → `GOPROXY` → `https://goproxy.cn,direct`; nothing calls
  `go env -w`. `scripts/setup.mjs` retries the download once with
  `GOPROXY=direct` after a non-zero mirror result, because Go's comma fallback
  only advances on 404/410, not 5xx or timeouts. Verification and checksum
  behavior are unchanged.
- Parallel lanes use `just lane-new <slug>`, which creates
  `.worktrees/<slug>/agent-vivy` plus a nested, pinned
  `.worktrees/<slug>/laputa`. A flat `.worktrees/<lane>` layout shares one
  mutable Laputa and is warned about by the doctor.
- Docker build inputs now match the dependency graph. The UI/backend build is
  one stage with both Node and Go because `pnpm build` invokes
  `go run ./sdk stage-ui`; `sdk/ui` is copied before `pnpm install` for the
  `file:../sdk/ui` dependency; every local Go replace manifest is copied
  before `go mod download`; the pinned Laputa source arrives through the named
  `laputa` build context. `scripts/docker-build.mjs` fails closed on a dirty or
  wrong Laputa checkout and on missing Go replace or UI file-dependency
  inputs. The external dockerfile frontend image was dropped so named contexts
  work with the built-in BuildKit frontend. The node slim base has no system
  CA bundle, so the build copies it from the Go image; corepack's own pnpm
  download is bound to `NPM_REGISTRY`; the build stage is proxy-only for Go
  because it carries no VCS client. The runtime stage copies the CA bundle and
  uses a bash `/dev/tcp` health probe instead of apt-installing
  `ca-certificates tzdata wget`, so a blocked Debian mirror cannot fail the
  final image.
- CI is split by cost: Windows backend/UI/browser, Linux backend, Linux kernel
  race, and a real-Postgres lane, with an aggregate job that requires every
  lane. `scripts/test-postgres.mjs` fails closed when
  `VIVY_POSTGRES_TEST_DSN` is unset, so a skipped database run cannot pass.
- `docs/dev/DEVELOPMENT-ENVIRONMENT.md`, `README.md`, `AGENTS.md`, and the UI
  testing/kernel-CI skills describe the venues, ports, overrides, lane layout,
  Docker acceptance, and test layering.

## Scope boundaries

- No `Service.Run`, Journal, policy, provider, or Eino orchestration path was
  changed. Eino capability check: nothing in the agent loop, model/tool
  orchestration, prompting, streaming, context, RAG, MCP, or multi-agent path
  was added or replaced, so no Eino API was inspected or substituted.
- Windows acceptance remains mandatory. A Linux/WSL green run does not close a
  missing artifact, an SDK Assembly contract defect, or a Windows owner-test
  failure; see `DEV-ENV-WIN-TRIAGE` in `docs/TODO.md`.
- The WSL Linux line is installed and exercised: Go 1.26.4, Node 24.21.0,
  pnpm 10.33.0, just 1.40.0, PostgreSQL 16.15, and Docker 26.1.5. `just ci`,
  a real-Postgres run, and `just race-ci` were executed locally on the Linux
  filesystem.
- `just race-ci` is not green. It found and fixed a first-party
  `sdk/tui/face` `sessionID` race, then reproduced the pre-existing pinned
  Eino `EINO-TOOLSNODE-ERR-RACE` (`tool_node.go:1253`) in `internal/runtime`.
  All other kernel packages passed under race. The Eino pin was not changed;
  the upstream fix (present in v0.9.21) remains a separate authorized change,
  and the lane must stay visibly red rather than skipping the affected test.
- The Docker packaging path is accepted. The build stage gained a corepack
  cache mount on every layer that invokes `pnpm`; without it the shim
  redownloads pnpm and needs the registry even after `corepack prepare`. The
  pnpm store, corepack cache, Go module cache, and Go build cache are BuildKit
  cache mounts. The empty-directory acceptance ran twice on the same
  candidate: once with `--network=none`, which reused 409/409 packages with 0
  downloads and completed `go mod download`, `pnpm build`, and `go build`;
  and once through `node scripts/docker-build.mjs --tag vivy:acceptance
  --smoke`, which produced `sha256:ff2a1f24…` (180,313,796 bytes) and a
  healthy container on a tmpfs `/data`. Seeding those caches in the
  acceptance copy is an offline-host workaround (fake-IP DNS; proxy bound to
  Windows loopback), not a claim that public mirrors were reachable; a
  cold-cache networked rebuild remains the release-path check.
- No Dev Container, remote runner, dependency cache shared between worktrees,
  or shared Docker socket was introduced. `DEV-VENUE-1` remains deferred.

## Record

- Iteration evidence: `docs/logs/2026-10-17-dev-environment/verification.md`
- Human acceptance: `docs/logs/2026-10-17-dev-environment/acceptance.md`
- Resume point: `docs/logs/2026-10-17-dev-environment/checkpoint.md`

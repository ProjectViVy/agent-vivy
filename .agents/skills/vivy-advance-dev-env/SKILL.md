---
name: vivy-advance-dev-env
description: Analyze, establish, or repair the Vivy development environment across Windows, WSL2, Linux, and Docker, and decide which daily verification gates the current host can actually satisfy. Use when onboarding a machine; when just doctor or just dev fails; when deciding whether to install or use WSL2 or Docker; when a Docker build fails on missing inputs; before claiming just ci, race, real Postgres, Docker smoke, or Windows .exe acceptance; or when the user says 开发环境 / 环境配置 / 能不能跑 just ci / WSL / Docker.
---

# Vivy Advance Dev Env

## Restored baseline

Restored from `7b42b292` onto stable on 2026-10-21. The owner then authorized
portable WSL verification wiring and dedicated minimal-kernel CI. Use the
current `justfile` and these actual entry points:

- `node scripts/ensure-laputa.mjs` prepares the pinned sibling through one
  portable owner. The PowerShell entry only forwards. `just doctor` validates
  the local toolchain and clean source pin; readiness is not test acceptance.
- `just wiring-ci` runs bootstrap/evidence guards and kernel compiler/source/
  runtime Assembly wiring. `just kernel-ci` vets/tests kernel owners and packs,
  inspects and boots only the nine-module usable minimal Recipe, without installing
  or building UI. It proves protected read/list and provider composition; the separate D13 browser gate certifies usable chat. It does not claim default parity.
- `just d02-ci` covers affected runtime/RPC/Face contracts, the causal App probe
  separately, shutdown and native CODE fixtures. `just d02-web-ci` separately
  stages the actual UI, typechecks it and tests affected Web/selected-UI paths.
- `just race-ci` is the Linux/WSL kernel race lane. `just test-postgres` requires
  a real disposable DSN and actual required cases; an unset DSN fails closed.
- Run those scoped gates from a separate Linux-filesystem WSL/Linux checkout,
  with its own pinned Laputa and dependencies. Windows retains focused native
  acceptance. Do not replace the owner's scope with Windows full-CI reruns.
- Full-product `just ci`, the dev launcher and Docker packaging are still
  legacy platform recipes. Do not advertise them as portable scoped gates.
  `scripts/setup.mjs`, isolated `:8788/:3016` launcher and release-conformance
  are not restored. The full `--apply` path still needs that setup owner.
- The manual split remains backend `127.0.0.1:8787` and Vite
  `127.0.0.1:3015`; use an explicit disposable home. Never use tenant Journals.

The older reference tables retain their original environment-tooling context;
this boundary and the current recipes determine applicability.
## Host model

Vivy uses three venues. Keep the split; do not collapse it into one host.

| Venue | Role | Owns |
| --- | --- | --- |
| Windows native | editor, browser, daily use | split UI at `:3015`, native acceptance of the packaged `.exe` |
| WSL2 on the Linux filesystem | main Go/Node/pnpm line | `just wiring-ci`, `just kernel-ci`, `just race-ci`, Docker when the daemon lives there, UI served to the Windows browser |
| Linux (or macOS) | the WSL line without the Windows prefix | backend/UI/race; Windows acceptance is external |
| Docker | dependency services and packaging | real Postgres, image build, empty-`/data` startup smoke; not the inner loop |

The split is a resource decision, not a correctness escape hatch. A Windows-only host may run `just ci` but must not claim the Linux race or a Windows acceptance it did not run. A WSL-only host must not claim native `.exe` acceptance.

## Procedure

1. Analyze before changing anything.

   ```bash
   node .agents/skills/vivy-advance-dev-env/scripts/advance-dev-env.mjs --check
   ```

   Use `--json` for a machine-readable plan and `--repo <path>` from outside a checkout. The script delegates repo state to `scripts/doctor.mjs` and adds the Windows/WSL/Docker/gate decisions doctor intentionally omits. It defaults Go traffic to direct (`VIVY_GOPROXY=direct`, process-local); `--proxy <value>` overrides that run only. It never runs `go env -w` or changes global proxy settings.

2. Read the readiness matrix. Treat `BLOCKED` as a reason to fix or degrade, `EXTERNAL` as a gate that must be run elsewhere, and `OPTIONAL` as not required. Do not turn an `UNKNOWN` into a pass.

3. Decide the venue and the degradation.

   - WSL present and suitable: keep the Go/Node/race checkout on the Linux filesystem; keep Windows for the browser and `.exe` acceptance.
   - WSL absent or unsuitable: develop on Windows, keep Linux/race/Postgres/Docker gates in CI, and label them as not run locally. Do not force an install.
   - Docker absent or unsuitable: use native PostgreSQL or CI for the real-Postgres lane, and CI for the Docker smoke. Do not install a second daemon just to fill a table.
   - No Windows host: record Windows `.exe` acceptance as external; never claim it from WSL or Linux.

4. Install only when the host is suitable and the user explicitly approves.

   ```bash
   node .agents/skills/vivy-advance-dev-env/scripts/advance-dev-env.mjs --install wsl --dry-run
   node .agents/skills/vivy-advance-dev-env/scripts/advance-dev-env.mjs --install docker --dry-run
   ```

   The script refuses unsuitable hosts, never elevates, and starts Docker Desktop only when it is already installed. `--install wsl` requires Windows 10 build 19041+ or Windows 11, x64/arm64, enabled virtualization, and at least 15 GiB free. `--install docker` uses winget on Windows and brew on macOS; on Linux/WSL it refuses to run root installs and points at the official Docker documentation. A reboot or first launch may follow. Never add a runtime dependency only because it is available.

5. Configure the checkout using its actual portable entry.

   ```bash
   node scripts/ensure-laputa.mjs
   just doctor
   ```

   Install UI dependencies only for an affected UI lane. The minimal lane has
   no UI dependency installation. Keep Go proxy changes process-local.

6. Select the scoped verification ladder above in proportion to the change.
   Run one heavy lane at a time. Record every skip and its owner. Optional
   modules, full-product packaging and native Windows acceptance remain
   distinct from a passing minimal kernel. A zero-test selector never passes.
7. Record evidence outside the chat.

   Write `docs/logs/YYYY-MM-DD-slug/{summary,verification,acceptance}.md`, or a focused commit for a minor agent-instruction change. Distinguish implemented, verified, and unverified. A skipped race, Postgres, Docker, or Windows lane is not green. Keep a transferable patch, bundle, or commit so the result does not depend on this conversation.

## Installation and isolation rules

- User-level installs first. WSL Node comes from nvm; Go can live in `~/.local/go`; pnpm comes from corepack; just is a single binary. Do not use administrator execution by default.
- Never share `node_modules`, a running database, or `.workspace/` between the Windows and WSL checkouts. They are separate acceptance environments with the same SHA and dependency versions.
- Parallel work uses `just lane-new <slug>`: `.worktrees/<slug>/agent-vivy` plus its own `.worktrees/<slug>/laputa`. Never share a mutable Laputa across lanes.
- Do not read or write the tenant Journal: `data/vivy.db`, `data/demo/`, `data/workspaces/`. Studio scratch is `data/studio-home/`.
- Do not expose the Docker socket to every agent. Membership in the `docker` group is effectively root; prefer WSL or Docker Desktop integration.
- Config holds `env_key` names only. Never put tokens in files, logs, events, or screenshots.
- Prefer direct downloads. Keep proxy choice process-local (`VIVY_GOPROXY` or `--proxy`); never run `go env -w`; never disable checksum verification. A mirror failure gets one direct retry, not a loop.

## Common failure signatures

The full table lives in `references/verification-layers.md`. The highest-frequency ones:

- `just dev` reads the wrong config or home: `VIVY_CONFIG` must resolve to `scripts/dev.config.yaml`, and `VIVY_USER_HOME` to `.workspace/dev/home`.
- Docker build reaches the registry during an "offline" check: every pnpm layer needs the corepack cache mount, not only `corepack prepare`.
- Docker build fails on CA or apt: copy the CA bundle from the Go image, keep the runtime free of apt, and use a bash `/dev/tcp` healthcheck.
- WSL or container DNS returns `198.18.x.x` while the Windows proxy listens on `127.0.0.1:7890`: that proxy is unreachable from WSL/containers. Seed BuildKit caches and build with `--network=none`; that proves input completeness, not registry reachability.
- WSL resolves an old Node or loses nvm after `bash -lc`: the shell rc guard skips nvm for non-interactive shells. Use an interactive shell or source nvm explicitly.
- A failed `docker run <image> version` hangs: the Vivy binary ignores `version` and starts the server. Smoke through the healthcheck or the product script.

## Resources

- `references/host-matrix.md`: per-host conditions, install choices, WSL/network/resource notes, and the independent Windows acceptance checklist.
- `references/verification-layers.md`: gate requirements, evidence format, claim rules, and known failure signatures.

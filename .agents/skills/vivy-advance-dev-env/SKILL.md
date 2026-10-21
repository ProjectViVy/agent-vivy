---
name: vivy-advance-dev-env
description: Diagnose or prepare a Vivy host when onboarding, a development command fails, or an affected check needs unavailable WSL/Linux, Docker, Postgres or Windows support. Reuse established readiness for routine work; mentioning CI or editing instructions does not trigger environment setup.
---

# Vivy Advance Dev Env

## Restored baseline

Root AGENTS.md owns validation scope; this skill guides venue and readiness.
Use the current justfile and dependency declarations. The restored 2026-10-21
baseline and old command tables are historical context in
`references/verification-layers.md`, not requirements to recreate a former host.
Check that a referenced command/script exists before using it. In particular,
do not run an old full `--apply` route whose setup owner is absent.

`node scripts/ensure-laputa.mjs` prepares the pinned sibling; `just doctor`
checks readiness rather than acceptance. Prepare only dependencies needed by the
affected lane. Kernel-only work does not need UI installation or browser startup.
Browser work uses disposable backend `:8787` and Vite `:3015` homes.
## Host model

Choose the venue for the affected check; this table does not require every venue.

| Venue | Role | Owns |
| --- | --- | --- |
| Windows native | editor, browser, daily use | split UI at `:3015`, native acceptance of the packaged `.exe` |
| WSL2 on the Linux filesystem | main Go/Node/pnpm line | `just wiring-ci`, `just kernel-ci`, `just race-ci`, Docker when the daemon lives there, UI served to the Windows browser |
| Linux (or macOS) | the WSL line without the Windows prefix | backend/UI/race; Windows acceptance is external |
| Docker | dependency services and packaging | real Postgres, image build, empty-`/data` startup smoke; not the inner loop |

Report only the acceptance actually exercised. WSL/Linux cannot establish native
Windows `.exe` acceptance; host readiness cannot establish a product test pass.

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

6. Return to the affected checks under root validation policy. Run one heavy
   lane at a time. Record missing acceptance only when required by this task;
   do not invent a checklist of unrelated unrun platforms. Use root delivery rules.

## Installation and isolation rules

- User-level installs first. WSL Node comes from nvm; Go can live in `~/.local/go`; pnpm comes from corepack; just is a single binary. Do not use administrator execution by default.
- Never share `node_modules`, a running database, or `.workspace/` between the Windows and WSL checkouts. They are separate acceptance environments with the same SHA and dependency versions.
- Parallel work uses `just lane-new <slug>`: `.worktrees/<slug>/agent-vivy` plus its own `.worktrees/<slug>/laputa`. Never share a mutable Laputa across lanes.
- Do not read or write the tenant Journal: `data/vivy.db`, `data/demo/`, `data/workspaces/`. Studio scratch is `data/studio-home/`.
- Do not expose the Docker socket to every agent. Membership in the `docker` group is effectively root; prefer WSL or Docker Desktop integration.
- Config holds `env_key` names only. Never put tokens in files, logs, events, or screenshots.
- Prefer direct downloads. Keep proxy choice process-local (`VIVY_GOPROXY` or `--proxy`); never run `go env -w`; never disable checksum verification. A mirror failure gets one direct retry, not a loop.

## Common failure signatures

Consult `references/verification-layers.md` only for a matching observed symptom.
Confirm that its original host/version conditions still apply; old network,
temporary-directory or upstream failures do not mandate permanent workarounds.

## Resources

- `references/host-matrix.md`: per-host conditions, install choices, WSL/network/resource notes, and the independent Windows acceptance checklist.
- `references/verification-layers.md`: gate requirements, evidence format, claim rules, and known failure signatures.

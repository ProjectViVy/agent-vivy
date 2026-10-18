# vivy-advance-dev-env skill

## What changed

Added `.agents/skills/vivy-advance-dev-env/`:

- `SKILL.md` — host model, analysis-to-verification procedure, opt-in installation rules, isolation/safety rules, and the claim rules for daily gates.
- `references/host-matrix.md` — Windows/WSL2/Linux/macOS roles, WSL/Docker suitability, toolchain baseline, network/offline behavior, resources, and the independent Windows acceptance checklist.
- `references/verification-layers.md` — gate requirements, evidence format, claim rules, and the failure signatures learned during the dev-environment work.
- `scripts/advance-dev-env.mjs` — read-only readiness analysis by default; `--apply` for repo bootstrap; `--install wsl|docker` with a suitability gate, never elevating; `--json` for machine-readable output.

The script delegates repository checks to `scripts/doctor.mjs` instead of duplicating them. It adds the Windows/WSL/Docker cross-host analysis that doctor intentionally omits. Its own subprocesses default Go traffic to `VIVY_GOPROXY=direct` (process-local); `--proxy` overrides one run. It never runs `go env -w`, starts servers, or writes to `data/`.

## Scope

- Covers environment establishment (Windows, WSL2, Linux, Docker) and whether the current host can satisfy each verification layer.
- Installs WSL or Docker only when the host is suitable and the user explicitly invokes `--install`; unsuitable hosts are refused. Linux/WSL Docker Engine installs are never run as root.
- Does not add a new runtime dependency, change the repo `justfile`, or change `doctor.mjs`.
- Does not run a full `just ci` for this change: the deliverable is an agent skill with no product behavior change. The script and the packaged skill are exercised directly.

## Packaging

The package is `.workspace/skill-dist/vivy-advance-dev-env.skill` (gitignored, transferable). The source of truth remains `.agents/skills/vivy-advance-dev-env/`.

## Not done

- No commit or push.
- No automatic Go/Node/just installation in the skill script; installation guidance for those lives in the host matrix.
- No Windows Docker Desktop install on this machine because a usable daemon already runs in WSL; the assessment reports that shape instead of adding a second daemon.

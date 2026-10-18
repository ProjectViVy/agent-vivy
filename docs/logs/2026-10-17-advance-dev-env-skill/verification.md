# Verification

Date: 2026-10-17. Host: Windows 10.0.26100 (x64, 24 CPU, 23.7 GiB RAM), WSL2 Debian 13, Docker 26.1.5 inside WSL.

## Static checks

- `node --check .agents/skills/vivy-advance-dev-env/scripts/advance-dev-env.mjs` — passed.
- `quick_validate.py .agents/skills/vivy-advance-dev-env` — `Skill is valid!`.
- `package_skill.py .agents/skills/vivy-advance-dev-env .workspace/skill-dist` — packaged all four files successfully.

## Windows runs

| Command | Result |
| --- | --- |
| `advance-dev-env.mjs --check` | exit 0; doctor `28 ok, 2 warnings, 0 failures`; WSL present (Debian); Docker daemon in WSL Debian 26.1.5; Go proxy direct (process only); Postgres `BLOCKED`; every other gate `READY` |
| `advance-dev-env.mjs --check --json` | schema `vivy.advance-dev-env/v1`; `host.os=windows`; `host.wsl.distros=["Debian"]`; `host.docker.viaWsl.version=26.1.5+dfsg1`; 9 gates |
| `advance-dev-env.mjs --apply --offline` | exit 0; setup skipped network work; doctor `28 ok` |
| `advance-dev-env.mjs --apply --proxy direct` | exit 0; setup printed `GOPROXY=direct`; module download succeeded; doctor reported `project default GOPROXY=direct`; no HTTP proxy probe; `28 ok, 2 warnings` |

The two Windows warnings are expected: no native Docker CLI on PATH (the daemon lives in WSL), and a global `go env GOPROXY=https://goproxy.cn,direct` that this run intentionally bypasses.

## WSL run

`node .agents/skills/vivy-advance-dev-env/scripts/advance-dev-env.mjs --check --repo /home/mastwet/work/agent-vivy`:

- exit 0; `wsl/x64`, 8 CPU, 7.8 GiB RAM, Docker 26.1.5, `cc 14.2`.
- doctor `28 ok, 1 warning`.
- `windows-acceptance` reported `EXTERNAL`; `studio` reported `OPTIONAL`; `test-postgres` reported `BLOCKED`.
- Confirms a WSL-only checkout does not claim native `.exe` acceptance.

## Suitability and failure paths

Direct probes of the exported `assessWsl`, `assessDocker`, and `parseDsn` covered:

- Windows build 17763, 5 GiB free disk, and disabled virtualization each make WSL unsuitable with a stated reason.
- A suitable Windows host yields `wsl.exe --install -d Debian --no-launch`.
- WSL-on-WSL is `applicable: false` ("already running inside WSL").
- Windows without winget is unsuitable for Docker; Linux refuses root installation; daemon-in-WSL is recognized.
- DSN parsing works for URL and keyword forms and keeps only host/port/database.

Script exit codes, captured through a WSL script file (an inline `bash -c` probe swallowed codes, including for `node -e 'process.exit(7)'`):

- `--install wsl --dry-run` inside WSL → 1, with the refusal message.
- `--help` → 0.
- `--bogus` → 2.

## Packaged skill

Repackaged after the final `SKILL.md` edit, extracted to `.workspace/skill-dist/unpacked/`, and run standalone:

`node .../advance-dev-env/scripts/advance-dev-env.mjs --check --no-doctor --repo . --json`

→ schema `vivy.advance-dev-env/v1`, `host.os=windows`, 9 gates, exit 0.

## Not run

- Full `just ci` for this change: the deliverable is an agent skill plus its own script and docs, with no kernel, UI, or product-contract change. No product behavior is claimed.
- `just test-postgres`, race, Docker smoke, and Windows `.exe` acceptance: the skill only reports their readiness; this iteration did not produce a product candidate to run them against. They remain unverified for this change.
- No installation was performed. The WSL/Docker install paths were exercised with `--dry-run` and synthetic suitability probes only.

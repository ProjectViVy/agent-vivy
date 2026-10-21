> Current validation policy: follow root AGENTS.md and vivy-kernel-ci. The
> commands and records below describe their original host context; they do not
> require full-product CI during ordinary development.

# Verification layers

Read the [restored-baseline applicability boundary](../SKILL.md#restored-baseline) before using these original environment-tooling commands or recorded versions.

## Gate table

| Layer | Command | Requires | Primary evidence | Cannot substitute for |
| --- | --- | --- | --- | --- |
| Environment | `just doctor` | Go, Node, pnpm, just, pinned Laputa, writable checkout | zero failures; warnings triaged | a build or test result |
| Inner loop | `just dev` | doctor clean, ports 8788/3016 free | split loop ready; exercised UI at `:3016` | `just ci` |
| Fast checkpoint | `just check-fast` | Go, pinned Laputa | fmt-check + vet + build exit 0 | tests or UI behavior |
| Platform CI | `just ci` | Go, Node 22+, pnpm, just, pinned Laputa | fmt, UI/i18n, vet, backend tests, memory loop, recall, headless, plugins | race, real Postgres, Docker, Windows `.exe` |
| Linux race | `just race-ci` | Linux/WSL, C toolchain, UI build | `go test -race ./internal/...` exit 0, or a recorded upstream finding | platform CI |
| Real Postgres | `just test-postgres` | `VIVY_POSTGRES_TEST_DSN`, reachable PostgreSQL 16 | postgres package tests against the real DSN | package tests that skipped Postgres |
| UI contract | `just ui-ci`, `just ui-e2e` | Node, pnpm, browser for E2E | typecheck, unit, build, i18n, browser path | backend tests |
| Docker packaging | `just docker-build --smoke` | Docker daemon, pinned Laputa | image builds, container healthy on empty `/data` | registry reachability or a source test |
| Windows acceptance | platform lane + native `.exe` | independent Windows checkout, same SHA and Laputa | `.exe` paths, locks, symlinks, exit cleanup, owner tests | WSL or Linux results |
| Release candidate | `just release-conformance` | fixed candidate SHA, pinned Laputa, tool versions | conformance evidence with every skip listed | an ordinary green CI run |

## What counts as a pass

- A gate passes only when the command ran on this candidate and exited 0, or a documented equivalent real path ran. A skipped gate is not a pass, even if the workflow is green.
- `just test-postgres` fails closed when `VIVY_POSTGRES_TEST_DSN` is unset. Never set the DSN to a mock or report the package tests as a real-Postgres pass.
- `just race-ci` can be red on the pinned upstream Eino. Record the exact upstream location and version, and state that the local change was not implicated. Do not delete the lane or call it green.
- `just docker-build --smoke` must start healthy on an empty `/data`; a Dockerfile that builds only when a registry answers is not proven complete. An offline `--network=none` build with seeded caches proves input completeness, not registry reachability.
- Windows owner-test failures are triaged individually. A failure on one owner test is not "Windows is bad"; an elevated-only skip is a skip that CI or a lower-privilege run must cover.
- Browser behavior is exercised at the split Vite origin (`:3016` for `just dev`, `:3015` for the manual pair), not only a screenshot and not the embedded UI on `:8787`.

## Evidence format

Record, at minimum:

- Candidate identity: `HEAD`, dirty entry count, tracked diff digest, copied-tree digest or patch path.
- Dependency identity: pinned Laputa SHA and clean/dirty state; Go, Node, pnpm, just, Docker, and PostgreSQL versions.
- Commands and outcomes for every layer that ran.
- Every lane that did not run, with the reason. Skipped Postgres/race/Docker/Windows lanes stay visible.
- Docker image id, size, creation time, and the smoke message when the packaging gate ran.
- A transferable artifact (commit, patch, or bundle) so the result does not depend on chat history.

Store the record under `docs/logs/YYYY-MM-DD-slug/` with `summary.md`, `verification.md`, and `acceptance.md`. A minor agent-instruction change can use a focused commit instead.

## Failure signatures

| Symptom | Cause | Action |
| --- | --- | --- |
| `just dev` reads the daily home or a repo `config.yaml` | `VIVY_CONFIG` / `VIVY_USER_HOME` not set | point them at `scripts/dev.config.yaml` and `.workspace/dev/home`; keep `data/` and `~/.vivy` untouched |
| Docker build calls `go run ./sdk stage-ui` but Go is absent | Node-only build stage while `pnpm build` needs Go | use one stage with both Node and Go, or stage the UI before the Node build |
| Docker build cannot find `sdk/ui` | UI workspace depends on a local `file:` package not copied | copy `sdk/ui` (and other local replace manifests) before `pnpm install` |
| `getaddrinfo EAI_AGAIN` on the install/build layer | corepack cache mount exists only on the `corepack prepare` layer | add the corepack cache mount to every pnpm-invoking layer |
| `x509: certificate signed by unknown authority` | slim Node image has no system CA bundle | copy `ca-certificates.crt` from the Go image into the build stage |
| Runtime image fails on `wget`/apt | runtime tried to install packages from an unreachable mirror | keep the runtime package-free; copy the CA bundle and use a bash `/dev/tcp` healthcheck |
| WSL/container DNS returns `198.18.x.x`, proxy is `127.0.0.1:7890` | fake-IP DNS and a loopback-only Windows proxy | make the proxy reachable from the Linux network, or seed BuildKit caches and build `--network=none` |
| WSL Node is old or nvm is missing after `bash -lc` | non-interactive shell returns before the nvm block | use `bash -ic` or source `nvm.sh` explicitly; check for later `export PATH=` lines |
| `memory-loop` fails from a subdirectory | script assumes the repository root | run it from the checkout root or via `just test` |
| Sharing-lock test skips only in an elevated shell | the test guards an elevation-only sharing case | rely on the non-elevated CI lane; record the skip |
| `go test -race` fails in `compose/tool_node.go` | pinned upstream Eino v0.9.13 race, fixed later upstream | record `EINO-TOOLSNODE-ERR-RACE`; do not claim a local race pass; treat a pin upgrade as its own change |
| `docker run <image> version` hangs | the Vivy binary ignores `version` and starts the server | use the healthcheck or the product smoke script, and stop accidental containers |
| `studio/` is empty | submodule not initialized | `just ensure-studio`; do not re-vendor the studio tree into the host |
| Custom provider cannot run a model | only embedded vendors execute runs; a registry entry alone is not enough | diagnose as a provider capability issue, not an environment failure |

## Claim rules

- Distinguish **implemented**, **verified**, and **unverified** in the summary. A compile or a screenshot is not a substitute for the gate that actually exercises the behavior.
- Do not promote a warning to a pass. Doctor warnings are non-fatal but must be read; a missing tool that a gate needs remains a blocked lane.
- Do not report local runs as CI runs, and do not report CI runs as a Windows acceptance. State the venue.
- Keep the result reproducible: same candidate, same Laputa pin, same tool versions, same commands. If the environment changed between runs, record both states.
- Air gap: never read or write `data/vivy.db`, `data/demo/`, or `data/workspaces/` to make a check pass. Secrets stay out of fixtures, logs, events, patches, and screenshots.

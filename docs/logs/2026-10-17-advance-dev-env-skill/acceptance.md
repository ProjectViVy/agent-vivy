# Acceptance

## Skill behavior

1. On the Windows host, from the repository root:

   ```bash
   node .agents/skills/vivy-advance-dev-env/scripts/advance-dev-env.mjs --check
   ```

   A human should see the host summary (Windows build, CPU/RAM/disk, virtualization), WSL and Docker placement, `repo doctor: 28 ok, 2 warnings, 0 failures`, a readiness matrix, and next actions. The run is read-only, exits 0, and prints `Go proxy: direct (process only; no go env -w)`.

2. The same command in the WSL checkout reports `wsl/x64`, the WSL resource cap, the Docker version in WSL, `EXTERNAL` for Windows `.exe` acceptance, and `OPTIONAL` for Studio. It must not claim Windows acceptance.

3. `--json` is parseable and carries schema `vivy.advance-dev-env/v1`, host facts, per-gate readiness, install assessments, and next actions. DSN passwords and `DOCKER_HOST` values are not emitted.

4. On this machine, `test-postgres` is `BLOCKED` until `VIVY_POSTGRES_TEST_DSN` points at a reachable server. With no DSN the script must never report it ready.

## Installation guardrails

- `--install wsl --dry-run` on WSL exits 1 with "already running inside WSL" and changes nothing.
- `--install wsl` on an unsuitable Windows host (old build, low disk, virtualization off) is refused with reasons; on a suitable host it prints the user-level install command and the UAC/reboot note.
- `--install docker` recognizes an existing daemon, offers to start Docker Desktop when installed, and refuses Linux/WSL root installation. It never elevates.
- Running the script does not run `go env -w`, does not change proxy environment outside its own subprocesses, does not start backend/Vite, and does not write under `data/`.

## Daily use

- The skill is discoverable by its description for onboarding, `just doctor`/`just dev` failures, WSL/Docker decisions, Docker input failures, and gate claims. Its references load only when needed.
- `--apply` prepares Laputa and dependencies, then delegates final state to `scripts/doctor.mjs`; `--offline` reuses caches without network access.
- The packaged artifact `.workspace/skill-dist/vivy-advance-dev-env.skill` can be moved to another machine and run without the repository's missing files; the repository copy remains the source of truth.

## Human-visible proof

After the change, a new contributor can ask "can this machine run `just ci`, race, real Postgres, Docker smoke, and Windows acceptance?" and receive an explicit `READY` / `BLOCKED` / `EXTERNAL` / `OPTIONAL` answer with reasons and next actions, without any gate being silently treated as passed.

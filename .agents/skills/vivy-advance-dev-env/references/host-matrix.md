# Host matrix

Read the [restored-baseline applicability boundary](../SKILL.md#restored-baseline) before using these original environment-tooling commands or recorded versions.

## Roles and gates

| Host | Inner loop | `just ci` | `just race-ci` | Real Postgres | Docker smoke | Windows `.exe` |
| --- | --- | --- | --- | --- | --- | --- |
| Windows native | yes, with Go/Node/pnpm/just and Laputa | yes | only with a C toolchain; not the canonical lane | with a real DSN | if a daemon is reachable, often via WSL | yes |
| WSL2, code on ext4 | yes | yes | yes, canonical | yes | yes | no; external Windows host |
| Linux | yes | yes | yes | yes | yes | no; external |
| macOS | yes | yes | yes, with Xcode command-line tools | yes | Docker Desktop/Colima | no; external |

`just dev` is the inner loop everywhere. The embedded UI on `:8787` is the release/CI smoke path, not the edit loop.

## Windows native

- Start with `just doctor`, then `just dev` (or `.\dev.ps1`). The split loop uses backend `127.0.0.1:8788` and Vite `127.0.0.1:3016`, with data under `.workspace/dev/home`.
- PowerShell owns Windows process management; build logic that needs branching lives in cross-platform Node helpers. Do not add a second Windows-only build framework.
- The Windows checkout is independent from the WSL checkout. Give it its own `node_modules`, `.workspace/`, and built binaries. Do not point it at a running WSL database.
- Windows acceptance is a distinct gate: build the native `.exe` from the same SHA and pinned Laputa, then exercise paths with spaces, file locks, symlinks, and exit cleanup. Use a non-elevated shell unless the check itself requires elevation; an elevated-only skip is a recorded skip, not a pass.

### WSL2 on Windows

Install only when all of these hold:

- Windows 10 build 19041+ or Windows 11.
- x64 or arm64.
- Virtualization enabled in firmware and a hypervisor available.
- At least 15 GiB free on the target volume.
- The user explicitly approves the install.

The scripted command is `wsl.exe --install -d Debian --no-launch`. It does not elevate; approve any UAC prompt yourself. A reboot and first-launch user setup may follow. After that, keep the repository on the Linux filesystem (for example `~/work/agent-vivy`), not on `/mnt/c`.

If the host is unsuitable, do not install WSL. Develop on Windows and send the Linux, race, Postgres, and Docker lanes to CI. Record them as not run locally.

### Docker on Windows

There are two valid shapes. Choose one; do not run both by accident.

1. **Docker Engine in WSL.** This is the verified shape on the current machine: Windows has no `docker` on PATH, while WSL Debian runs Docker 26.1.5. Use the WSL checkout for `just docker-build` and `just docker-up`. From Windows, `wsl -d Debian -- docker ...` reaches the same daemon.
2. **Docker Desktop.** Use it when Windows-side Docker is required. It needs virtualization and roughly 20 GiB free, and it adds a second daemon beside a WSL Engine. Install only with explicit approval via `winget install --id Docker.DockerDesktop -e`; start it manually or let the script start an already-installed desktop.

If Docker Desktop is installed but the daemon is stopped, start Docker Desktop. If neither shape is available, keep packaging and the real-Postgres lane in CI or use a native PostgreSQL 16; do not install a second daemon just to fill a readiness row.

## WSL2

- Code and `node_modules` live under the Linux filesystem. `/mnt/c` adds cross-filesystem I/O and file-watch overhead; doctor warns when it sees this.
- The verified toolchain is Go 1.26.4, Node 24.21.0 via nvm, pnpm 10.33.0, just 1.40.0, Docker 26.1.5, and GCC 14. Node 22+ is the repository floor; the pinned pnpm comes from `ui/package.json`.
- `~/.bashrc` may return early for non-interactive shells, so `bash -lc` can skip nvm. Use an interactive shell (`bash -ic`) or source nvm explicitly before diagnosing a "missing" Node. If `appendWindowsPath=false` is set in `/etc/wsl.conf`, Windows tools are intentionally absent from the Linux PATH.
- Resource caps are real: the current host exposes 8 vCPU, about 8 GiB RAM, and a 4 GiB swap to WSL. Run one heavy lane at a time. Full CI plus a Docker build will contend; cap them in `.wslconfig` if needed.
- The Windows browser can normally reach WSL Vite through localhost forwarding. If it cannot, use `VIVY_UI_PORT` and the WSL address, and verify the backend binds to the interface the browser uses. A blocked proxy or disabled localhost forwarding is a network configuration issue, not a reason to move the code to `/mnt/c`.

## Linux and macOS

- Linux is the WSL line without Windows-specific process management. The race lane is canonical here. Windows `.exe` acceptance stays external.
- macOS can run the backend, UI, race, and Docker gates. Windows acceptance stays external. Docker needs Docker Desktop, Colima, or another daemon; the script offers brew only when brew is present.

## Toolchain baseline

| Tool | Repository expectation | User-level source |
| --- | --- | --- |
| Go | `go.mod` (currently 1.26.x) | official tarball, `~/.local/go`, or the platform installer |
| Node | 22+; 24.x verified | nvm on WSL/Linux; installer on Windows |
| pnpm | `ui/package.json` pins 10.33.0 | `corepack prepare pnpm@10.33.0 --activate` |
| just | any recent 1.x; 1.40/1.49 verified | single binary on PATH |
| PostgreSQL | 16 for the real-Postgres lane | container or native install; a skipped DSN is not a pass |
| Docker | daemon reachable from the checkout that builds | WSL Engine or Docker Desktop |

Do not install a tool merely because it is missing from one venue if another venue or CI already covers the gate. Install only when the host is suitable and the user approves.

## Network and offline behavior

- Prefer direct downloads. The skill script sets `VIVY_GOPROXY=direct` for its own subprocesses unless `VIVY_GOPROXY` is already set or `--proxy` is passed. This is process-local; never run `go env -w`.
- A mirror failure gets one direct retry (`scripts/setup.mjs`). Do not loop mirrors, and do not disable checksum verification.
- On hosts where DNS returns fake-IP addresses in `198.18.0.0/16` and the Windows proxy listens only on `127.0.0.1:7890`, WSL and containers cannot use that proxy. Two valid responses: bind the proxy so WSL can reach it, or prove the build inputs offline.
- Offline Docker acceptance: seed the BuildKit cache mounts for the pnpm store, corepack, Go modules, and Go build cache, then run `docker build --network=none`. This proves the Dockerfile inputs are complete; it does not prove registry reachability. A registry timeout is not an acceptance pass.
- BuildKit cache mounts are per cache, not inherited. The corepack cache must be mounted on every layer that invokes pnpm, including install and build, not only on `corepack prepare`.

## Isolation, resources, and security

- Parallel work uses `just lane-new <slug>`: `.worktrees/<slug>/agent-vivy` and `.worktrees/<slug>/laputa`. Each lane has its own checkout and pinned Laputa. Never share a mutable Laputa.
- Windows and WSL checkouts must not share `node_modules`, a running database, `.workspace/`, or built binaries. Same SHA and dependency versions, separate state.
- Do not read or write `data/vivy.db`, `data/demo/`, or `data/workspaces/`. The Studio scratch home is `data/studio-home/`.
- Do not expose the Docker socket to all agents. Membership in the `docker` group is effectively root. Prefer WSL or Docker Desktop integration, and restrict a world-writable socket before running untrusted agents.
- Keep secrets out of config, logs, events, patches, and screenshots. Config stores `env_key` names only.
- Resource guidance is based on the actual machine: read the script's CPU/RAM/free-disk line before starting CI and containers together. A low-memory host should run one lane at a time rather than raising WSL caps beyond its physical memory.

## Independent Windows acceptance checklist

1. Cut or copy a Windows checkout at the same candidate SHA. Record the Laputa SHA and tool versions.
2. Use a separate `node_modules`, `.workspace/`, and database. Do not reuse the WSL state.
3. Build the native `.exe` and run it from a non-elevated shell.
4. Exercise paths with spaces, file locks, symlink handling, restart behavior, and exit cleanup.
5. Run the platform test lane and triage each owner-test failure individually. A failure is not evidence that "Windows is bad"; an elevated-only skip is a skip.
6. Record the commands, results, and skips in the iteration log. Do not claim a lane that was not executed.

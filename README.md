# agent-vivy

AGENT-VIVY V0 runtime — a personal gateway Agent assembled on the Eino
execution framework with a Vivy-owned thin application shell.

This repository is the implementation home of AGENT-VIVY. The design dossier
(PRD v0.5, assembly options, reference index, go/no-go preflight) lives in the
parent directory `../` (`diva-go/`) and is the contract this code must honor.

## Status

V0/V1 species runtime is assembled (see `docs/TODO.md`). **Vivy Studio
is the first-party daily development IDE and the independent owner of the
distribution lifecycle.** Other authorized developer tools may work directly
in this repository with their own native capabilities; work does not need to
be transferred into Studio (`docs/architecture/VIVY-STUDIO.md`).

Do not add a “open Studio” door to `vivy.exe`. The species-side Studio
card is not Studio.

## Requirements

- Go 1.26+ (installed at `C:\Program Files\Go\bin` on this machine; if `go`
  is not on PATH, use the full path or add it).
- `GOPROXY` must be `https://goproxy.cn,direct` (proxy.golang.org is
  unreachable from this network). Already persisted via `go env -w`.
- Node.js 22+ and pnpm for the embedded React UI.
- Optional: [just](https://github.com/casey/just) for the task recipes in
  `justfile`. Without it, run the underlying `go` commands directly.

## Quick start

```powershell
just setup         # go mod download
just ensure-studio # git submodule update --init studio/ (ProjectViVy/vivy-studio)
just build         # go build ./...
just test          # go test ./...
just ci            # Go fmt/vet/test + UI install/typecheck/unit/build
just run           # run the vivy process (health endpoint on :8787)
just dev           # one-click split loop: backend :8787 + Vite :3015
just build-split   # build a headless backend plus standalone ui under dist/
just docker-up     # one-container image, SQLite on a volume, host 127.0.0.1:8787
.\launch-vivy-studio.ps1  # Studio IDE on :3090 (auto-ensures studio submodule)
```

Prefer `git clone --recurse-submodules` so `studio/` is present immediately.
`just ensure-studio` / `launch-vivy-studio.ps1` will init it on first use if not.

Docker is a packaging of the same organism (embedded UI, one process,
replica=1). `docker compose up --build` publishes **only**
`127.0.0.1:8787:8787` and stores the Journal on the named volume
`vivy-data` (`/data/vivy.db` in the container). Do not scale the service
and do not bind `8787` on all host interfaces. Provider keys stay in the
environment (`OPENAI_API_KEY` / `ANTHROPIC_API_KEY`). Open
`http://127.0.0.1:8787` after the container is healthy. The image does
not ship `go` / `git` / `rg`; `execute` / `commandline` stay gated and
unavailable until those binaries are present.

Optional Postgres is a second Journal engine, not a replica set. Set
`storage.backend: postgres` and `storage.postgres.dsn_env` (the
environment variable *name*); put the DSN in that variable. A second
process on the same DSN fails to start. `just docker-up-postgres` is the
compose overlay; default `just docker-up` stays SQLite on a volume.

For day-to-day Vivy feature work, run `just dev` (or `.\dev.ps1` /
`.\dev.cmd`). That starts the backend on `:8787` and the Vite UI on
`:3015` together. Or use two terminals: `just run`, then `cd ui; pnpm
dev`. Open `http://127.0.0.1:3015`. The Vite server proxies `/rpc` to
the backend, so UI and Go changes can be iterated independently. The
embedded UI is the default release/CI path; use `just build-split` when
you need the packaged headless backend and standalone static UI.

## Layout

```text
cmd/vivy/          species entrypoint (daily gateway + worker + `tui` client)
sdk/               vivy-sdk binary (verify/pack); invoked by Studio, not by vivy.exe
sdk/plugin/        author import window
internal/app/      composition and lifecycle
internal/config/   config loading and validation (secret boundary)
internal/domain/   Vivy-owned Session/Message/Run/RunEvent/... contract types
internal/runtime/  Run service + Eino adapter (event mapping, interrupt)
internal/provider/ openai-compatible / anthropic; YAML bundles
internal/tools/    ToolSpec registry + approval policy
internal/storage/  Journal/SnapshotStore/BlobStore/LeaseStore + SQLite backend
internal/events/   event fan-out and after_seq replay cursor
internal/rpc/      UI-facing command/query/event JSON-RPC control plane
internal/tui/      TTY face: `--demo` Crush-style offline shell; `--plain` RPC REPL
schemas/           RunEvent JSON Schema, provider bundle schema
fixtures/          provider / event / recovery fixtures
ui/                only browser UI (React + Vite + Zustand + TanStack Router)
studio/            git submodule → ProjectViVy/vivy-studio (Studio shell + plugins)
cmd/vivy-studio/   Studio lifecycle CLI (pack/eval/release); stays in this repo
docs/              implementation plan + project TODO board
```

The Studio **shell** (DSH profile bundles, first-party skin/console, plugin hub
fork, community plugin snapshots) is maintained in
[`ProjectViVy/vivy-studio`](https://github.com/ProjectViVy/vivy-studio) and
mounted here at `studio/`. Bump the submodule gitlink after shell changes;
do not re-vendor those trees into this host repo.

The default `vivy.exe` remains a single-file, embedded-UI application. For a
same-machine split deployment, `just build-split` produces
`dist/vivy-backend.exe` and `dist/vivy-ui/`. Set `server.allowed_origins` in
the backend config to the exact loopback origin serving the static UI, then
set `controlPlaneUrl` in `dist/vivy-ui/vivy-config.json` to the backend URL.
The split build is intentionally loopback-only; it is not a remote or
multi-user deployment mode.

## Hard rules (from the dossier)

- Eino and reference-project types never leak past `internal/runtime` /
  `internal/provider` into the UI contract (PRD D-007).
- Single Go module (D-006). No multi-module workspace.
- Secrets are never persisted (D-010).
- Exactly one terminal event per run (D-008).
- No `agent-diva-*` code import or schema inheritance (D-005, D-021).
  `.workspace/eino` in the parent dossier is a read-only API reference only;
  this module consumes Eino exclusively as online dependencies.

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
just ci            # Go fmt/vet/test + UI install/typecheck/unit/build + I18N completeness
just run           # run the vivy process (health endpoint on :8787)
just tui           # build and run an independent VIVY CODE TUI
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

## Interface language

English (`en`) is the product default. Exactly `en` and `zh` are supported;
browser or operating-system language does not select another locale.
For development or a new pack, set this non-secret input in the local root
`.env` (which remains gitignored; never commit it):

```dotenv
VIVY_DEFAULT_LOCALE=en
```

An already-set process `VIVY_DEFAULT_LOCALE` takes precedence over `.env`,
including validation: an empty or unsupported process value fails rather than
falling through to the file. If neither source supplies a value, use `en`.
Only this setting is read by the locale loader; unrelated `.env` values are
not logged or copied into presentation metadata.

`vivy-sdk pack` embeds the resolved default in the executable and in
`generation.json` at `recipe.settings.locale`. A sealed Generation keeps that
default even if the process environment or source-tree `.env` later changes.
An unsealed development body resolves the process/file default at startup.
This existing core locale embedding does not imply that the current SDK
implements the v1 plugin platform.

Settings → Language writes `locale` in the global VIVY workspace
`settings.yaml`, not the current project's settings or browser storage.
That user override wins over the Generation/development default and is shared
by Web, TUI, and VIVY CODE. An absent or empty workspace locale means no
override. Sealing fixes the default, not the user's presentation preference.

Both faces use backend authority: `settings/get` returns `locale`,
`generation_locale`, `workspace_locale`, and `locale_read_only`;
`settings/locale` accepts `{ "locale": "en" }` or `{ "locale": "zh" }`
and returns the same locale view. The narrow write preserves unrelated
settings, rejects unsupported locales and read-only deployments, and remains
allowed when provider settings are frozen. Web applies the successful backend
response; localStorage is only a startup cache and hydration supersedes it.
TUI hydrates through the same settings RPC for in-process and `--live` use;
if unavailable, it retains the embedded default with a sanitized warning.
This is shared persistence and hydration, not cross-process live push.
User/model/tool text, Journal content, and protocol identifiers remain data.

From the repository root, `node scripts/check-i18n-completeness.js` validates
Web catalog key/placeholder parity and maintained runtime copy. `just ci`
runs it after UI dependency installation without replacing the existing Go,
headless, plugin-module, or UI gates. See the
[I18N implementation status](docs/superpowers/specs/2026-09-09-i18n-design.md#final-implementation-status)
for fresh verification evidence and the deliberately deferred plugin catalog
proposal; Go/TUI execution and the complete `just ci` gate remain unverified
in the 2026-09-09 Task 9 environment.

## Layout

```text
cmd/vivy/          species entrypoint (daily gateway + worker + fullscreen `tui` command)
cmd/vivy-code/     independent VIVY CODE binary; shared config, private Journal per launch
sdk/               vivy-sdk binary (verify/pack); invoked by Studio, not by vivy.exe
sdk/plugin/        author import window
sdk/tui/           canonical fullscreen TUI view, controller, protocol projection, and tests
internal/app/      composition and lifecycle
internal/config/   config loading and validation (secret boundary)
internal/domain/   Vivy-owned Session/Message/Run/RunEvent/... contract types
internal/runtime/  Run service + Eino adapter (event mapping, interrupt)
internal/provider/ openai-compatible / anthropic; YAML bundles
internal/tools/    ToolSpec registry + approval policy
internal/storage/  Journal/SnapshotStore/BlobStore/LeaseStore + SQLite backend
internal/events/   event fan-out and after_seq replay cursor
internal/rpc/      UI-facing command/query/event JSON-RPC control plane
internal/tui/      remote WebSocket transport for `vivy tui --live`
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

`vivy-code.exe` is a first-party terminal product built from the same kernel,
not a forked runtime. It reads the same `config.yaml` / `VIVY_CONFIG`, provider
environment variables, shared `settings.yaml`, and skills root as `vivy.exe`.
Each launch allocates its own SQLite Journal and runtime/log directory under
`<shared-data-root>/code-instances/`, so multiple TUI processes and the web
process can run concurrently without sharing sessions, messages, approvals,
runs, or checkpoints. `just tui` builds the headless-tagged binary and starts
it in the current project.

## Hard rules (from the dossier)

- Eino and reference-project types never leak past `internal/runtime` /
  `internal/provider` into the UI contract (PRD D-007).
- Single Go module (D-006). No multi-module workspace.
- Secrets are never persisted (D-010).
- Exactly one terminal event per run (D-008).
- No `agent-diva-*` code import or schema inheritance (D-005, D-021).
  `.workspace/eino` in the parent dossier is a read-only API reference only;
  this module consumes Eino exclusively as online dependencies.

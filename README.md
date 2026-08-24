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
just setup      # go mod download
just build      # go build ./...
just test       # go test ./...
just ci         # Go fmt/vet/test + UI install/typecheck/unit/build
just run        # run the vivy process (health endpoint on :8787)
```

## Layout

```text
cmd/vivy/          species entrypoint (daily gateway + worker)
sdk/               vivy-sdk binary (verify/pack); invoked by Studio, not by vivy.exe
sdk/plugin/        author import window
internal/app/      composition and lifecycle
internal/config/   config loading and validation (secret boundary)
internal/domain/   Vivy-owned Session/Message/Run/RunEvent/... contract types
internal/runtime/  Run service + Eino adapter (event mapping, interrupt)
internal/provider/ openai-compatible / anthropic / mock; YAML bundles
internal/tools/    ToolSpec registry + approval policy
internal/storage/  Journal/SnapshotStore/BlobStore/LeaseStore + SQLite backend
internal/events/   event fan-out and after_seq replay cursor
internal/rpc/      UI-facing command/query/event JSON-RPC control plane
schemas/           RunEvent JSON Schema, provider bundle schema
fixtures/          provider / event / recovery fixtures
ui/                only browser UI (React + Vite + Zustand + TanStack Router)
docs/              implementation plan + project TODO board
```

## Hard rules (from the dossier)

- Eino and reference-project types never leak past `internal/runtime` /
  `internal/provider` into the UI contract (PRD D-007).
- Single Go module (D-006). No multi-module workspace.
- Secrets are never persisted (D-010).
- Exactly one terminal event per run (D-008).
- No `agent-diva-*` code import or schema inheritance (D-005, D-021).
  `.workspace/eino` in the parent dossier is a read-only API reference only;
  this module consumes Eino exclusively as online dependencies.

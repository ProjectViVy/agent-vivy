# Verification

## Commands

From the repository root:

```text
go build ./...
go test ./internal/app ./internal/provider ./internal/storage/sqlite ./internal/runtime
just ci
```

## Results

- `go build ./...` **green** (was failing on `ModelResolver` / `NewResolvingChatModel` / `TakeOrganismLease`).
- Targeted tests **green**: `internal/app` 4.449s, `internal/provider` 0.170s, `internal/storage/sqlite` cached, `internal/runtime` 24.760s.
- `just ci` **green**: `fmt-check` / `vet` / `go test ./...` / `headless-compile` / `ui-ci` (typecheck + 162 vitest + `pnpm build`).

## Browser smoke

Skipped: this iteration restores compile-time wiring only. No UI, RPC method, or user-visible path changed. The product gate is `just ci`.

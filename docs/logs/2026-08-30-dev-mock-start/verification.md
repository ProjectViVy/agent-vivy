# Verification

## Commands

From the repository root:

```text
go test ./internal/app ./internal/provider
just ci
```

Isolated start (does not touch `data/vivy.db`):

```text
VIVY_CONFIG=config.dev.yaml
VIVY_USER_HOME=<temp>
VIVY_ADDR=127.0.0.1:18787
go run ./cmd/vivy
```

## Results

- `go test ./internal/app ./internal/provider` **green** (includes `TestResolverUsesRuntimeMock` and `TestResolvingChatModelMock` / `TestCatalogMockRef`).
- Isolated `config.dev.yaml` start logged `vivy starting` on `127.0.0.1:18787` (process held until the probe timeout; not a crash).
- `just ci` **green**: `fmt-check` / `vet` / `go test ./...` / `headless-compile` / `ui-ci` (typecheck + 162 vitest + `pnpm build`).

## Browser smoke

Not walked on `:3015` in this iteration: a leftover Vite already listens on `127.0.0.1:3015` with no backend on `:8787`, so `just dev` would refuse to start. The product-path proof is the isolated mock start plus `just ci`. Kill the stray Vite, then `just dev`, to open `http://127.0.0.1:3015`.

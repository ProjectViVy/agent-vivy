# Verification

## Commands

- `go test ./internal/provider ./internal/config ./internal/app ./internal/rpc ./internal/eval ./internal/storage/sqlite` — pass
- `just ci` — pass (`fmt-check` / `vet` / `go test ./...` / headless compile / `pnpm typecheck` / 154 UI tests / `pnpm build`)

## Notes

- Unit tests do not call live provider networks.
- e2e uses `ui/e2e/openai-stub.mjs` on `127.0.0.1:8800` plus `VIVY_USER_HOME=ui/.e2e-workdir/home`.
- `just dev` points at `data/dev-home`, not the operator `~/.vivy`.

# Verification · 前端可恢复错误

## Commands

| Command | Result |
|---|---|
| `pnpm test -- src/lib/failure.test.ts src/lib/rpc.test.ts src/lib/runtime-config.test.ts src/lib/store.test.ts src/i18n/index.test.ts` (from `ui/`) | pass (28 tests) |
| `pnpm typecheck` (from `ui/`) | pass |
| `pnpm test` (from `ui/`) | pass (175 tests) |
| `just ci` (repo root) | pass: fmt-check, vet, `go test ./...`, headless compile, `pnpm typecheck`, vitest 175/175, `pnpm build` |
| `GET http://127.0.0.1:3015/` | 200 |
| `GET http://127.0.0.1:3015/rpc/bootstrap` | 200 `vivy.rpc.v1` |

## Notes

- Air gap: did not read or write `data/vivy.db`, `data/demo/`, or `data/workspaces/`.
- Live split pair already running: Vite `:3015` + Studio-managed `vivy-backend.exe` on `:8787`. Did not start a second `just run`.
- Control-plane-down path was verified by unit tests (`Failed to fetch` / HTML bootstrap / retryInitialize). The live `:3015` probe hit a healthy backend.

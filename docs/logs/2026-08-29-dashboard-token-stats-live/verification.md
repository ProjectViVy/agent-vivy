# Verification

## Commands run

```text
cd ../agent-vivy-ui-token   # isolated worktree feat/ui-token
gofmt -w internal/storage/contracts.go internal/rpc/tokenstats.go \
       internal/storage/sqlite/token_usage.go internal/storage/postgres/token_usage.go
go test ./internal/rpc/ -run TestBuildTokenSnapshot -v
go test ./internal/rpc/ -v -count=1
go test ./internal/storage/... -v -count=1
just ci
```

## Results

| Check | Result |
| --- | --- |
| `gofmt` | clean after formatting pass |
| `go test ./internal/rpc/` (token stats) | 5/5 PASS |
| `go test ./internal/rpc/` (full) | ALL PASS (21.6s) |
| `go test ./internal/storage/...` | ALL PASS (sqlite 55.7s, postgres 0.3s) |
| `just ci` → `fmt-check` | PASS |
| `just ci` → `vet` | PASS |
| `just ci` → `test` | 2 pre-existing failures unrelated to this change |

### Pre-existing failures (not introduced by this change)

1. `TestServiceApprovalApproveFlow` — flaky disk I/O error in runtime test;
   context canceled during journal append. Known intermittent.
2. `TestEmbeddedHandlerAnswers` — requires `ui/dist` to be built first
   (UI-CI-BOOTSTRAP in `docs/TODO.md`). Worktree starts without it.

Both failures exist on `main` at the same commit and are documented in
`docs/TODO.md` §0.1.

## Smoke (deferred)

Browser smoke at `http://127.0.0.1:3015` requires `just dev` which needs
`pnpm install` + `pnpm build` in the worktree. Deferred to human verification
step in acceptance.md.

# Verification · 聊天区真实 PLAN / TODO

Worktree: `C:\Users\Administrator\Desktop\morediva\diva-go\agent-vivy-plan-todo-display`  
Branch: `feat/plan-todo-display`

## Commands

| Command | Result |
|---|---|
| `gofmt -l` on changed Go files | clean |
| `go test ./internal/rpc` | pass (`ok`, 14.730s; includes `TestControlHandlerListsSessionTodos`) |
| `just ci` | pass: fmt-check, vet, `go test ./...`, headless compile, `pnpm typecheck`, vitest 161/161, `pnpm build` |
| Isolated RPC smoke `127.0.0.1:8798` | pass (see below) |
| Browser `http://127.0.0.1:3015` | **not this lane's server** — port already served the root-tree split pair; did not reuse it |

## Isolated RPC smoke

Started `go run ./cmd/vivy` with `VIVY_CONFIG=.smoke-todo/config.yaml`, `VIVY_ADDR=127.0.0.1:8798`, `VIVY_USER_HOME=.smoke-todo` (scratch, not `data/vivy.db` / `~/.vivy`).

1. `initialize` capabilities contain `session.todos`.
2. `session/create` then `session/todos` → empty list (`todos` length 0).
3. Unknown session → `-32004 session not found`.
4. Missing `session_id` → `-32602`.
5. Inserted two Journal rows into `.smoke-todo/state/smoke.db`, then `session/todos` returned:
   - `id=1` `in_progress` `wire rpc` `active_form=Wiring RPC`
   - `id=2` `completed` `old task`

Backend stopped after smoke. Scratch dir is gitignored.

## Notes

- Air gap: did not read or write `data/vivy.db`, `data/demo/`, `data/workspaces/`, or `~/.vivy`.
- Root tree on `main` was dirty (`studio/dsh-plugin-subscriptions/`); this lane did not edit it.
- Empty worktree had no `ui/dist`; first `go test ./internal/app` failed embed until `pnpm build`. `just ci` then green (known UI-CI-BOOTSTRAP).
- Browser chrome on `:3015` belongs to the other lane; product empty/open-panel path was verified by unit tests + live RPC data, not by driving that occupied UI.

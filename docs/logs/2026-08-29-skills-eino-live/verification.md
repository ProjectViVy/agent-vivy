# Verification · Skill 真正可用

## This concern

| Command | Result |
|---|---|
| `gofmt` on this delivery's Go files | clean |
| `go test ./internal/runtime -run TestEngineInjectsSkillMiddlewareWithoutKeyword\|TestEngineOmitsSkillMiddlewareWhenBackendNil\|TestEinoSkillBackendListGetAndView\|TestNewEngineRejectsNilModel` | pass |
| `go test ./internal/rpc -run TestControlHandlerSkillsCatalog\|TestControlHandlerUnknownMethodAndInvalidParams` | pass (after a one-line `settings.IsZero` compile unblock; see below) |
| `go test ./internal/app -run TestSkillsCatalogRPCSmoke\|TestRPCBootstrapRoutePrecedesUIShell` | pass — composed app lists/gets a real `SKILL.md` over JSON-RPC/WebSocket |
| `cd ui; pnpm typecheck` | pass |
| `cd ui; pnpm test` | pass, 161/161 including `api.test.ts` skills methods and evolution demo tests |

App-level RPC smoke (`TestSkillsCatalogRPCSmoke`) composes `app.New` with a temp `skills_root`, dials `/rpc`, and asserts:

1. `skills/list` returns `demo-skill` with a content hash.
2. `skills/get` without path returns SKILL.md body + `supporting_files: ["references/guide.md"]`.
3. `skills/get` with that path returns `reference content`.

## Full `just ci`

Ran from the dirty root tree. **Failed**, not because of this Skills wiring:

- `internal/provider.TestNoHardcodedKeyLiterals` — `internal/domain/sandbox.go` comment `ask-on-effect` matches `sk-…` (other lane).
- `internal/runtime` filesystem sandbox tests — path-escape failures in sandbox manager (other lane).
- Later `go test ./internal/rpc` also failed to **build** until `Settings.IsZero` stopped comparing `SandboxSettings` (contains `*bool`) with `==`. One-line field check in `internal/app/settings/settings.go` unblocked compilation so this delivery's RPC tests could run. That is not a product claim of this Skills work.

## Browser

`http://127.0.0.1:3015` is already bound by Studio's managed `vivy-backend` on `:8787` (PID 14292, under `data/studio-home/vivy-console/`). This lane did not kill that process or reuse its Journal. Product path for list/get is the composed-app RPC smoke above, plus Vite typecheck/vitest on the new Skills page.

## Air gap

Did not read or write `data/vivy.db`, `data/demo/`, `data/workspaces/`, or `~/.vivy`.

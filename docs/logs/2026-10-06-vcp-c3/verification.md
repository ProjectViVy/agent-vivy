# VCP C3 — verification

## Commands run

```text
go build ./...                                            # clean
go run ./sdk verify plugins/coding/session-tree           # ok vivy/session-tree
go run ./sdk stage-ui --recipe recipes/default.vivy.yml --out ui/src/generated   # 7 extensions
cd ui && pnpm exec tsc --noEmit                           # clean
cd ui && pnpm exec vitest run                             # 75 files, 584 tests green
go test ./internal/rpc ./internal/runtime ./internal/app  # green
go test ./sdk/internal/conformance/                       # green after digest re-pin
```

## Coverage added

- `internal/rpc/exports_test.go`
  - `TestExportsReadRoundtrip` — write→export→read returns matching digest/name/base64; `exports.read` capability advertised.
  - `TestExportsReadGuards` — separator/`.`/`..`/bad-char names → `InvalidParams`; missing file → `CodeNotFound`; wrong `expected_digest` → `CodeNotFound`; unwired dir → `MethodNotFound`.
- `plugins/coding/session-tree/module_test.go` — descriptor pins exactly one `std/ui-extension@v1` provider with zero grants; `vivy-module.yaml` declaration matches the descriptor.
- `plugins/coding/session-tree/ui/session-tree/src/tree.test.ts` — DFS layout order/depth on a fork graph + cycle survival.
- `internal/app/codeface_rpc_test.go` — golden transcript updated: `get_tree` now asserts a real single-node tree (it was previously asserted as unimplemented).
- `ui/src/lib/store.test.ts` — `getQueueState` mock typed to `QueueState` (fixed a latent B3 typecheck break surfaced by `tsc --noEmit`).

## Acceptance checklist (plan)

- [x] Tree page shows real fork graph (nodes + fork edges from `session/tree`).
- [x] Click-to-switch sessions (select + navigate; double-click opens the session).
- [x] Clone/import/export actions wired to `session/clone|import|export`.
- [x] Export downloads a byte-verified HTML (digest-bound `exports/read` + client SHA-256 check before Blob).
- [x] Page contributed via `std/ui-extension@v1` Module — no shell edits.

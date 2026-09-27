# Verification: MEM-1A Vivy runtime wiring for BML

Environment: branch `feat/memory`, 2026-09-26; HEAD at closeout `93c89c3d`.

## 1 — Per-slice gates (run during each task's review)

```text
gofmt -l internal/modules/memory            # clean
go vet ./internal/modules/memory ./internal/modules/defaults ./internal/app   # clean
go test ./internal/modules/...              # green
cd plugins/vivy-memory/ui/vivy-memory && pnpm test   # green (view.test.tsx mocks module.action.invoke)
```

Coverage: `module_test.go` (Open/Active/Close lifecycle, unavailable outcome),
`actions_test.go` (all 9 actions incl. `conflict`/`failed` paths),
`provider_test.go` (recall candidate fields, duplicate EventID writes once and
returns identical receipt, failed store → `failed` receipt),
`view.test.tsx` (rpc-mocked list/search rendering).

## 2 — Manifest sealing

Generated `ContextSources` / `RunObservers` / `Actions` match the declared
PortRef IDs exactly (`vivy.memory.bml` on both sync ports; the nine action IDs
on `vivy/memory-bml`); `Requires` on the sync record names the consuming host
ports so used-provider marking holds.

## 3 — Final gate

```text
just ci    # controller-run, all green
```

## 4 — Closeout is docs-only

This commit adds `docs/logs/2026-09-26-memory-1a-wiring/` and tracker rows;
no Go/TS source changes.

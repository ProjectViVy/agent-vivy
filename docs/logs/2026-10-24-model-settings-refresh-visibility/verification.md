# Verification

All commands ran in the worktree
`.worktrees/model-settings-refresh` (branch `feat/model-settings-refresh`,
based on `stable` 83029dc0).

## Environment notes (pre-existing, not caused by this change)

- **Go toolchain**: the installed `go` is 1.27.1 while `go.mod` pins `go
  1.26.4`. On 1.27 the vendored `google.golang.org/grpc v1.64.1` fails to
  compile against `golang.org/x/net v0.54.0` (`http2.TrailerPrefix` is only
  built with the legacy http2 implementation, excluded on Go 1.27). The root
  checkout fails the same way. Every Go command below therefore ran with
  `GOTOOLCHAIN=go1.26.4`.
- **Node/happy-dom**: Node 24.18/26 defines an experimental global
  `localStorage` that is unusable without `--localstorage-file`; Vitest skips
  populating happy-dom's `localStorage` over it, so every UI test file fails in
  `beforeEach` at `localStorage.clear()` — in the root checkout too. UI test
  commands below ran with
  `NODE_OPTIONS=--localstorage-file=/tmp/vivy-vitest-ls.json`.
- **laputa replacement**: `replace ... => ../laputa/laputa` does not resolve
  from `.worktrees/<lane>`; a symlink `.worktrees/laputa -> ../../laputa`
  (inside the gitignored `.worktrees/`) restores it.

## Go

| Command | Result |
| --- | --- |
| `gofmt -l internal/rpc/control.go internal/rpc/control_test.go` | no output |
| `GOTOOLCHAIN=go1.26.4 go vet ./internal/rpc/` | pass |
| `GOTOOLCHAIN=go1.26.4 go test ./internal/rpc/ -run 'TestProviderRefreshUsesCatalogOverlay\|TestProvidersCatalogMergesRegistryModelsForCatalogEndpoints' -count=1` | ok |
| `GOTOOLCHAIN=go1.26.4 go test ./internal/rpc/ -run 'Provider\|Settings' -count=1` | ok |
| `GOTOOLCHAIN=go1.26.4 go test ./internal/rpc/ ./internal/app/settings/ ./internal/provider/ -count=1` | ok (rpc 21.1s, settings 1.1s, provider 0.7s) |
| `GOTOOLCHAIN=go1.26.4 go test ./sdk/tui/... -count=1` | ok (the TUI reads the same unioned catalog projection) |

New/updated Go coverage:

- `TestProvidersCatalogMergesRegistryModelsForCatalogEndpoints` — an overlay
  with `upstream-x` + a duplicate embedded id makes the catalog endpoint serve
  `[upstream-x, deepseek-flash, deepseek-pro]`; `thinking_models` stays
  embedded-only; the overlay still appears under `entries`.
- `TestProviderRefreshUsesCatalogOverlay` — refreshing a catalog endpoint by
  legacy `bundle: "openai"` + exact `base_url` persists
  `catalog-gateway-openai-completions` (not `custom-*`), uses the vendor display
  name, updates the catalog row to overlay-first order, survives a second
  refresh with a manual id, and never creates a duplicate entry.
- Existing `TestProviderRefreshRPC` / catalog / model-select tests stay green.

## UI

| Command | Result |
| --- | --- |
| `GOTOOLCHAIN=go1.26.4 pnpm typecheck` | pass |
| `pnpm vitest run src/components/settings/ModelSettingsCard.test.tsx` (with the Node localStorage workaround) | 9/9 pass |
| `pnpm test` (with the workaround) | 86 files / 689 tests pass |
| `GOTOOLCHAIN=go1.26.4 pnpm build` | pass (only the pre-existing >500 kB chunk warning) |
| `node scripts/check-i18n-completeness.js` | PASS, en/zh 1580 keys, runtime copy audit clean |
| `node --test scripts/check-i18n-cross-face.test.js` | pass |
| `node scripts/check-i18n-cross-face.js` | PASS, 13 shared semantic units |

New component tests:

- catalog row + manual add → `saveProvider({ id: 'catalog-deepseek-openai-completions',
  models: [embedded…, new] })` (no `api_key`), then `saveSettings` with the new
  model;
- id already listed → no `saveProvider`, only the selection write;
- existing overlay → models appended to the overlay list;
- `saveProvider` rejects → input and draft stay open, no selection write;
- the refresh note survives a later `providers` update.

## Browser smoke (real Vite + Go backend)

Split pair on disposable state, with the connected Chrome:

```text
VIVY_ADDR=127.0.0.1:8797 VIVY_USER_HOME=/tmp/vivy-models-smoke GOTOOLCHAIN=go1.26.4 go run ./cmd/vivy
cd ui && VIVY_BACKEND_ADDR=http://127.0.0.1:8797 pnpm exec vite --port 3016 --host 127.0.0.1 --strictPort
```

Exercised at `http://127.0.0.1:3016/settings?tab=model`:

1. DeepSeek (openai-completions) row → Add model `deepseek-smoke-x` → the row
   immediately lists it and it is marked current.
2. Reload → still listed and current. `/tmp/vivy-models-smoke/settings.yaml`
   holds one `catalog-deepseek-openai-completions` entry whose models include
   `deepseek-smoke-x`, and the top-level selection points at it.
3. Add custom provider `Smoke Gateway` (`http://127.0.0.1:9821`, key
   `sk-smoke-secret`) against a loopback `/models` mock returning
   `smoke-up-a`/`smoke-up-b`.
4. Refresh → "Synced 2 models from upstream" appears and both model rows
   appear; the mock log records exactly one request with
   `Authorization: Bearer sk-smoke-secret`.
5. Add model `my-local-model` → listed and current; refresh again →
   "Synced 3 models from upstream", both upstream ids and the manual id remain,
   and the provider list still has exactly one Smoke Gateway row (no duplicate).
6. Reload → all three models still listed; the settings document holds one
   `custom-<uuid>` entry with those models; `sk-smoke-secret` never appears in
   the page text.

Scratch state (mock script/log, disposable home) was removed afterwards. The
pre-existing dev pair on :8787/:3015 was left untouched.

## Not run

- `just ci` — the recipe's `fmt-check`, `ui-core`, `i18n-check` steps call
  Windows PowerShell, and no independent Linux-filesystem checkout was
  available; the equivalent macOS commands above were run instead. Full
  `just ci` remains a Windows release gate.
- `ui/e2e/model-refresh.spec.ts` — needs a Playwright browser download; the
  same paths (custom-provider refresh, manual id retention, reload
  persistence, no secret on screen) were covered by the browser smoke above.

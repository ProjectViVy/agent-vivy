# Verification record — 2026-08-30 catalog-provider API Key entry

Repository root: `C:\Users\Administrator\Desktop\morediva\diva-go\agent-vivy`.

## Unit tests and build (root `just ci`, all green, exit 0)

```
just ci
# fmt-check ✓  vet ✓  go test ./... ✓  headless-compile ✓
# ui-ci: pnpm install --frozen-lockfile ✓
#        pnpm typecheck (tsc --noEmit) ✓
#        pnpm test → 21 files / 174 tests all passed
#          (custom-providers.test.ts 19 tests, including 4 new
#           catalog-provider key overlay cases)
#        pnpm build (vite build) ✓
```

New test assertions:
- `catalogOverlayId('deepseek')` → `catalog-deepseek`; `isCatalogOverlayEntry`
  recognizes the prefix;
- `allProviderEntries` hides `catalog-*` overlay entries and retains ordinary custom
  entries (clones);
- `providerEntryByEndpoint` matches by `(bundle, base_url)` (the same semantics as
  backend `ActiveKey`);
- `customApiKeySetFor` returns true when a catalog endpoint hits a registry key
  override, and false with no override or no configured key.

## Browser smoke (real path, Dev split pair)

Prerequisite: the repository's split pair was already running (Vite
`127.0.0.1:3015` → `/rpc` proxy to the Studio console backend at
`127.0.0.1:8787`). The smoke script was **read-only**: it entered no key, triggered
no write, and only clicked a catalog row to assert the field state.

```
$env:NODE_PATH = "<repo>\ui\node_modules\.pnpm\node_modules"
node .workspace/smoke/catalog-key-smoke.cjs
```

Result (Playwright headless Chromium, `http://127.0.0.1:3015/settings?tab=model`):

```json
{
  "catalog": { "disabled": false, "placeholder": "sk-… (empty = clear the configured key when applied)",
               "hint": "Written to the local user workspace (~/.vivy/settings.yaml); the value is not returned to the UI or written to logs. It takes effect on the next message." },
  "mock":    { "disabled": true,  "placeholder": "The built-in Mock provider does not need an API Key.", "hint": "The built-in Mock provider does not need an API Key." },
  "consoleErrors": []
}
```

- Catalog provider (DeepSeek) row: the field is **editable**, with the entry copy for
  editable providers;
- Mock row: the field remains **disabled**, with Mock-specific copy;
- The page has no console errors.

WebSocket-level write-path validation was not run (to avoid writing a test key to the
Studio console `settings.yaml` currently in use by the user). The write path is
covered by `custom-providers` pure-function unit tests plus `just ci`; the key
security boundary is the same as for custom providers (`settings/providers/upsert`,
0600 write-only, D-010).

## Explicitly skipped

- `ui/e2e`: not run (it requires an independent backend fixture, while this iteration
  changes UI state/copy; vitest plus the DOM smoke above provides coverage).

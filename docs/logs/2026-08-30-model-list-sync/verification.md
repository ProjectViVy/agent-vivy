# verification.md — 2026-08-30 model-list-sync

Environment: `agent-vivy-model-sync` worktree (`feat/model-list-sync`, independent of
`feat/tui-live-client` and root-tree main). The other session was not changed; the tenant
Journal (`data/vivy.db`, `data/demo/`, `data/workspaces/`) was untouched; all smoke data
was in local temporary worktree directories (`.smoke/`, `ui/.e2e-workdir/`, cleaned up).

## Commands and results

### Go (unit + integration, real HTTP round trips through httptest)

```text
go vet ./internal/provider ./internal/rpc          # PASS
go test ./internal/provider ./internal/rpc -run 'TestModelList|TestProviderRefreshRPC|TestProviderRegistryRPC' -count=1
# ok  agent-vivy/internal/provider   0.667s
# ok  agent-vivy/internal/rpc        3.468s (including wire regression assertions)
```

Coverage:

- `discover_test.go`: 200 successful parses + Bearer-header validation; trailing
  base_url slash → `/v1/models`; dedup/trim/empty id; HTTP 401 errors without the key;
  invalid JSON; connection refusal (neither URL nor key appears in errors).
- `control_test.go TestProviderRefreshRPC`: refresh by id (union [upstream-a,
  upstream-b, manual-a], `api_key_set` preserved, `ApiKey` unchanged in settings.yaml,
  key absent from the response body); **empty model-list wire format is always `[]`
  (regression: `models:null` makes frontend validation discard the entry; see summary)**;
  upstream 500 does not persist or notify a change; catalog provider without a registry
  row clones to a new entry (no key means no Authorization header); unknown id →
  not_found; Anthropic (by bundle and by entry id) → invalid_params; read-only →
  conflict; Frozen → conflict; `initialize` capabilities include
  `settings.providers.refresh`.

Note: top-level `go build ./...` requires `ui/dist` (the existing UI-CI-BOOTSTRAP issue,
see `docs/TODO.md` §0.1); headless compilation is covered by `just ci`'s
`headless-compile` (`-tags vivy_headless`).

### UI (typecheck + unit tests)

```text
cd ui; pnpm install --frozen-lockfile; pnpm typecheck   # PASS (tsc --noEmit)
pnpm test                                                # 162/162 passed (including new assertions in api.test.ts)
```

### Real browser path (Playwright e2e, real rendering + self-started backend at :8799)

```text
cd ui; pnpm exec playwright test e2e/model-refresh.spec.ts   # 1 passed (9.0s)
```

All green: add a custom provider (local /models) → empty-list prompt → click "Refresh"
to show gpt-4o/gpt-4o-mini + "Synced 2 models from upstream"; upstream receives
`Authorization: Bearer sk-e2e-secret` (test-only key); manually add my-local-model →
refresh again → sync 3 models while retaining the manual entry; after reload all three
remain and the key is not echoed.

> Note: the normal development addresses `http://127.0.0.1:3015`/`:8787` were occupied
> by another session's split pair, so this iteration did not restart the dev server
> there (to avoid interfering with it); the e2e-isolated `127.0.0.1:8799` was used for a
> real browser + real backend + real HTTP upstream, equivalently covering the page-click
> path.

### Standalone RPC smoke (real `go run ./cmd/vivy` + real WS protocol + isolated data_dir)

PowerShell-driven temporary smoke (`.smoke/`, deleted afterward): `initialize`
capabilities include `settings.providers.refresh`; `settings/providers/upsert` persists
without returning the key; `settings/providers/refresh` returns [smoke-alpha, smoke-beta,
manual-local] (union + key preserved); `settings/providers` reads back consistently;
unknown id → not_found (code=-32004). Persisted `settings.yaml` keeps `api_key` verbatim
and `models` as the union.

### Full gate

```text
just ci    # fmt-check + vet + test + headless-compile + ui-ci(typecheck/test/build)
```

Result: all green (see the acceptance record).

## Unverified items and reasons

- Manual visual walk-through on the `:3015` split dev server: the port was occupied by
  another session and was not restarted; the e2e browser path equivalently covers page
  behavior.
- Online refresh for native Anthropic endpoints: unsupported by the protocol and
  therefore expected (button hidden + RPC rejected).
- The preset e2e specs `runtime.spec.ts` / `welcome-wizard.spec.ts` contain existing
  stale assertions (UI-E2E-STALE / UI-E2E-DRAW), outside this iteration's scope.

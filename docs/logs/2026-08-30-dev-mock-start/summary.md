# Restore runtime.mock for offline start (2026-08-30)

## Changes

After the compile fix restored `ModelResolver` / `NewResolvingChatModel` to mainline,
the product path no longer used `config.Runtime.Mock`, and the Catalog also rejected
`mock`. When no API key was present, `just dev` / `dev.ps1` still switched to
`config.dev.yaml` (`runtime.mock: true`), so the process listened on its port but the
resolved model had `Ready=false` and chat failed immediately (welcome wizard / "no model
configured").

This iteration reconnects the offline mock to the resolver and Catalog, without changing
the UI or removing the `runtime.mock` config.

### Core

- `ModelResolver`: when `runtime.mock=true`, resolve a ready `mock` /
  `mock:<scenario>` and do not freeze the ENV session (so a leftover `OPENAI_API_KEY`
  cannot override `just dev`).
- `Catalog.For("mock")` returns the mock Ref again (config/test paths only; operator
  settings still reject mock).
- `defaultModelFor` restores mock / mock_scenario fallback.

## Explicitly not done

- Do not delete `config.Runtime.Mock` (`just dev` and e2e still depend on it).
- Do not change welcome-wizard or Settings-page copy.
- Do not address Vite `:3015` already occupied on this machine (an environment issue,
  not a code gap in this iteration).

## Changed files

- `internal/app/model.go`, `internal/app/model_test.go`, `internal/app/app.go`
- `internal/provider/{catalog.go,doc.go,mockref.go,provider_test.go}`

# Remove runtime mock provider

## What changed

- Removed the production `runtime.mock`/`mock_scenario` configuration path,
  Mock provider implementation, catalog ref, and the generated UI Mock provider
  entry.
- `ModelResolver` now resolves only frozen environment settings or persisted
  OpenAI/Anthropic settings. A missing selection returns a typed
  `ErrModelNotConfigured` error instead of manufacturing a reply.
- Provider key, connection, and unconfigured-model failures now publish the
  stable user message `无法连接！请检查供应商配置！`; transport and key details
  remain out of the run payload.
- Replaced runtime-test provider usage with an isolated deterministic echo model
  under `internal/testsupport`, outside the provider catalog.
- Updated model settings, saved-model fixtures, provider docs/schema, and
  browser checks. Tests that require model execution are gated on a real
  provider credential; the no-credential path asserts the connection error and
  absence of `mock reply`.

## Explicitly not changed

The UI's non-chat demo data and test-framework mocks remain test/demo fixtures;
they are not runtime model providers and cannot generate a chat reply.

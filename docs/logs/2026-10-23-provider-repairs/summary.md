# Provider repair batch

Implemented on `codex/provider-repairs`, based on stable `7496f35f4c2e54cb95d037433d2d3cae1fcc863b`.

- Custom providers marked `UserDefined` no longer write credentials into an embedded vendor environment variable when their label collides. Existing non-secret base URL overlays and built-in credential overlays remain in place.
- Frozen environment selection now requires an explicit vendor, a uniquely matching adapter, a matching configured vendor, or a sole candidate, in that order. Invalid/ambiguous overrides fall through to configured selection, as approved. Unresolved ambiguity preserves settings and produces one startup warning containing credential variable names and vendor names only.
- Settings load and provider upsert share an idempotent reconciliation helper that includes a non-empty default in the model list without changing caller-owned slices. Load repairs in memory; the next successful write persists it. RPC retains its existing projection and schemas.
- Discovery skips blank IDs and trimmed IDs above 256 bytes; more than 2,000 accepted unique IDs fails without partial results or a settings write. Existing refresh merging, 15-second timeout, and 4 MiB response bounds remain unchanged.

Relevant functions and tests were selectively adapted from `feat/remake-provider` (`e9bdf7bf`). The Stable `UserDefined` representation, provider data, and custom model construction path remain unchanged. No schema migration, dependency, protocol, Studio, provider-catalog, or frontend product change was made. Existing vendor-overlay test cleanup now restores the environment it touches.

## Eino capability check

Inspected repository-pinned Eino `v0.9.21` (`components/model`: BaseModel, ChatModel, ToolCallingChatModel) and EinoExt `agenticopenai v0.2.4` (`NewChatModel`, `ChatConfig.APIKey`, `ChatConfig.BaseURL`, `Generate`, `Stream`). The existing `internal/provider/openai.go` constructor still uses `agenticopenai.NewChatModel`. These upstream APIs construct and execute models; they do not own Vivy's settings registry, environment attribution, or discovery acceptance policy. This batch hardens existing Vivy configuration boundaries and adds no substitute adapter/runtime or Eino import. There is no new upstream capability to defer or migrate.

## Review and delivery

A separate read-only reviewer found no actionable issues in the implementation or new regression tests. The branch is retained for integration; no merge, push, deployment, or tenant installation was performed. Root checkout edits from the concurrent core-fixes lane were untouched.

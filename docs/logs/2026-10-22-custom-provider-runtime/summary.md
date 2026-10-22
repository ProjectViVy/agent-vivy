# Custom provider runtime

Date: 2026-10-22.

## Change

User-defined provider entries in Settings now resolve as executable selections when no embedded vendor owns their `(adapter, base_url)` endpoint. The runtime carries that identity through the existing provider boundary and constructs the model with `Catalog.Adapter`, which accepts only a sealed protocol adapter. The provider entry remains the source for the raw model id, base URL, and API key.

The implementation reuses the pinned EinoExt `agenticopenai.NewChatModel` component (`github.com/cloudwego/eino-ext/components/model/agenticopenai` v0.2.4) through the existing `openai-completions` adapter in `internal/provider/openai.go`. No new transport, Eino wrapper, or model metadata source was added. User-defined entries declare no capability metadata, so the runtime keeps thinking parameters conservative.

## Scope

- Fixed `resolveStoredSelection` so a matching user registry entry is not converted into a nonexistent embedded vendor.
- Routed marked user-defined selections through the existing sealed adapter and kept the selection type in the model cache identity.
- Added selection, resolver, real HTTP request, raw model-id, authorization, and missing-key regression coverage.
- No database schema, provider catalog, UI, or dependency changes.

The browser acceptance against the rebuilt isolated backend remains pending. The process currently serving the manual page still runs the pre-fix binary.

No release artifact was created; this iteration fixes source behavior and leaves the D14 manual acceptance for the updated backend.

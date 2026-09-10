# Plugin v1 Provider and Eino Capability Matrix

> Decision date: 2026-09-10
> Repository pins: Eino `v0.9.13`, EinoExt OpenAI `v0.1.13`, EinoExt
> Claude `v0.1.25`
> Scope: PLG-P5, `std/provider-profile@v1`, and the internal ModelHost

## Decision rule

Provider Profiles are declarative data. They may select only a build-owned
adapter family verified in the exact pinned module-cache source. Public Modules
cannot carry a constructor, callback, Eino value, transport, credential
resolver, or OAuth implementation. A capability without a suitable pinned
adapter and a compatible Vivy Secret boundary is `DEFERRED-INDEFINITE`; no
placeholder or parallel provider stack is permitted.

## Verified pinned surfaces

| Profile family | Pinned package/API inspected | Vivy decision | Boundary evidence |
|---|---|---|---|
| OpenAI-compatible chat | `github.com/cloudwego/eino-ext/components/model/openai@v0.1.13`: `NewChatModel`, `ChatModelConfig{APIKey, BaseURL, Model}`, `ChatModel.WithTools`, `Generate`, `Stream` | `ADAPT` | The existing T1 adapter constructs the component in `internal/provider`; raw model IDs and scoped Secret values are supplied by ModelHost resolution and Eino types remain quarantined. |
| Anthropic Messages chat | `github.com/cloudwego/eino-ext/components/model/claude@v0.1.25`: `NewChatModel`, `Config{APIKey, BaseURL, Model, MaxTokens}`, `ChatModel.WithTools`, `Generate`, `Stream` | `ADAPT` | The existing T1 adapter constructs the component in `internal/provider`; the direct Anthropic endpoint uses the configured raw model ID and scoped Secret value. |
| Anthropic extended thinking | `github.com/cloudwego/eino-ext/components/model/claude@v0.1.25`: `WithThinking` and `WithThinkingConfig` | `ADAPT` | ModelHost keeps profile selection declarative; the existing provider adapter emits the pinned per-call option only for model metadata that declares thinking support. |
| OpenAI-compatible gateway profiles | Same OpenAI `NewChatModel` surface with explicit `BaseURL` | `ADAPT` | Static and operator-defined endpoints reuse the one OpenAI adapter family. A gateway model ID such as `anthropic/claude-sonnet-4` is already a raw gateway ID and is preserved byte-for-byte. |
| Native OpenAI endpoint | Same OpenAI surface | `ADAPT` | Native IDs such as `gpt-4o` are never rewritten to `openai/gpt-4o`. |
| Native Anthropic endpoint | Same Claude surface | `ADAPT` | Native IDs such as `claude-sonnet-4-5` are never rewritten to `anthropic/claude-sonnet-4-5`. |
| Azure OpenAI | OpenAI `ChatModelConfig.ByAzure`, `APIVersion`, and `AzureModelMapperFunc` | `DEFERRED-INDEFINITE` | The upstream component exists, but PLG-P5 has no approved declarative deployment/API-version contract or governed mapping function. A profile cannot carry executable mapping code. |
| Anthropic on Amazon Bedrock | Claude `Config.ByBedrock`, AWS region/profile/static credential fields | `DEFERRED-INDEFINITE` | The upstream component exists, but the current Vivy Secret reference contract does not define the multi-value AWS/ADC lifecycle or its provenance. |
| Anthropic on Google Vertex AI | Claude `Config.ByVertex`, project/region/service-account/ADC fields | `DEFERRED-INDEFINITE` | The upstream component exists, but the current profile contract does not govern project, region, service-account JSON, or ambient ADC authority. |
| Provider OAuth lifecycle | No general OAuth authorization, refresh, revocation, or token-store API exists in the two pinned model component packages | `DEFERRED-INDEFINITE` | Static `APIKey`/`AuthToken` constructor fields and Vertex ADC are credentials accepted by a model client; they are not a Vivy-governed OAuth lifecycle. No custom OAuth stack is added. |
| Native Gemini, Groq, xAI, Mistral, or other vendor SDK | No corresponding provider component is pinned in `go.mod` | `DEFERRED-INDEFINITE` | These endpoints may run only when their selected profile truthfully declares the already-supported OpenAI-compatible adapter family. Native protocol execution is absent. |

## Requested profile inventory

The shipped UI catalog requests two execution families, not one executable
family per marketing vendor:

- `openai-compatible`: OpenAI plus configured compatible gateways and local
  servers. The profile owns endpoint and raw model IDs; execution reuses the
  pinned EinoExt OpenAI adapter.
- `anthropic`: the direct Anthropic Messages API. Execution reuses the pinned
  EinoExt Claude adapter.

Catalog entries whose API compatibility has not been selected by a compiled
Profile are display candidates only. P5 exposes capability status so a
`DEFERRED-INDEFINITE` family cannot become executable by inventing an endpoint.

## Invariants accepted for `ADAPT`

1. Construction is side-effect free with respect to the network; network I/O
   begins only on `Generate` or `Stream`.
2. Eino's `ToolCallingChatModel.WithTools` returns a request-scoped model and
   keeps tool binding out of the public SDK.
3. Context cancellation and stream errors flow through the pinned component
   and preserve their cause chain through the provider and ModelHost layers.
4. Model IDs are passed exactly as configured. Vivy adds no provider prefix.
5. Secret values are resolved at call time, never stored in Profile,
   Generation Manifest, status, error, or log output.
6. No public executable model-provider Port is introduced.

# Summary — VC-2 Anthropic 后端接线（eino-ext/claude）

## What changed

The Anthropic Messages API is now wired through the online eino-ext Claude
component (research `crush-parity-code-agent-research-2026-08-31.md` §8.5,
落地清单 6 items closed here), replacing the abandoned self-owned adapter
milestone. Blast radius is `internal/provider` only — the Ref seam, runtime,
model adapter, ADK and mocks are untouched.

1. **`claudeRef`** (`internal/provider/claude.go`, modeled on `openaiRef`):
   maps `ModelSpec{ID, APIKey, BaseURL}` onto the component `Config`,
   falls back to the bundle `default_model` / `default_api_base`.
2. **Backend id `eino-ext/claude`**: new constant, wired into the bundle
   validation switch, `Catalog.For` dispatch, and the product contract
   `schemas/providers.bundle.schema.json` backend enum (D-022..D-025). The
   never-wired self-owned `vivy/anthropic` backend was removed outright —
   no production bundle could have used it (it always returned "not wired
   yet"). Fixture `fixtures/provider/anthropic.yaml` migrated.
3. **Stale records corrected**: the "no official Eino Anthropic component
   exists" rationale in `bundle.go`, the fixture provenance note,
   `catalog.go` and `doc.go` now reflect the component reality (§8.5 item 2).
4. **D-010 protection**: `claudeRef` refuses an empty spec key with
   `KeyMissingError` *before* any SDK construction, making the component's
   `ANTHROPIC_API_KEY` / `ANTHROPIC_MODEL` environment fallback paths
   unreachable. A real-protocol outbound test (httptest server shaped like
   `/v1/messages`) poisons both environment variables and asserts the
   request still carries the spec's key in `x-api-key` and the spec's model
   id in the outbound body (the AGENTS.md outbound-model assertion rule).
5. **MaxTokens floor**: the Anthropic protocol requires `max_tokens` (no
   "0 = API decides" like OpenAI); the ref supplies `claudeDefaultMaxTokens
   = 8192`, overridable per call via `model.WithMaxTokens` (the auto-title
   chain uses 40). Outbound test asserts the floor rides the request.
6. **Supply-chain audit** (§8.5 item 5): see `verification.md` — the
   component unconditionally imports the Bedrock/Vertex branches, so
   aws-sdk-go-v2 and Google auth deps enter `go.sum` as predicted; every
   new runtime module's LICENSE was read and is MIT / Apache-2.0 / BSD-3
   clean.
7. **Prompt caching** (§8.5 item 6 + acceptance "caching 生效"): the bundle
   flag `supports_prompt_caching` gets its first consumer —
   `claudeRef` maps it to the component's `AutoCacheControl`, which places
   ephemeral breakpoints on system + tools + last message (SDK-default 5m
   TTL). Crush's system + last-2-messages placement is a known, accepted
   difference (both within Anthropic's 4-breakpoint best practice). The
   outbound test asserts the ephemeral breakpoint actually appears on the
   wire.
8. **Model metadata (D9)**: `knownAnthropicModels` reference table
   (context window, published USD-per-MTok prices, image support) so
   anthropic providers participate in cost accounting. Unknown ids
   (e.g. the fixture's `claude-opus-4-6`, not verifiable from here) stay
   zero-valued — unpriced is never read as free.

## Explicitly not done

- No eino-ext deepseek/qwen/gemini components (per §8.5 "明确不做").
- No thinking/reasoning parameter surface yet (the component supports it;
  no Vivy config maps onto it) — noted for a later slice if wanted.
- Bedrock/Vertex construction paths are not exposed by any Vivy bundle
  field; only direct-API construction is reachable from the ref.
- Real-network smoke against api.anthropic.com was not possible (no key);
  the httptest protocol test is the real-path substitute (recorded in
  verification.md).

## Constraints honored

- Crush is FSL-1.1-MIT: behavior/protocol alignment only, zero code copied.
- No features beyond the Crush-mapped surface were added: caching follows
  the §8.5 decision record; the metadata table extends the already-shipped
  D9 cost feature to the new provider, not a new product surface.

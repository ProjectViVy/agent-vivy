// Package provider implements the ProviderRef boundary and the pre-baked
// provider catalog. V0 ships exactly two provider bundles:
//
//   - openai: OpenAI-compatible, backed by
//     github.com/cloudwego/eino-ext/components/model/openai;
//   - anthropic: Vivy-owned thin adapter over the Anthropic Messages API
//     (no official Eino component exists yet).
//
// Bundles are YAML documents adapted from the Diva providers.yaml schema
// (D-022..D-025), rewritten into Vivy's owned schema with a provenance
// field. Committed config holds env_key names only. Runtime credentials
// arrive as ModelSpec values from the user workspace or a frozen ENV
// session, never by reading process environment inside this package (D-010).
//
// A deterministic mock remains as a test helper (NewMock) and is not a
// product catalog entry. The Vivy-owned Anthropic Messages API adapter
// lands in a later milestone; A2 bundles ship under fixtures/provider/.
package provider

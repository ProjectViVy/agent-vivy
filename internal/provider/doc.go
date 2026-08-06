// Package provider implements the ProviderRef boundary and the pre-baked
// provider catalog. V0 ships exactly two provider bundles plus a mock:
//
//   - openai: OpenAI-compatible, backed by
//     github.com/cloudwego/eino-ext/components/model/openai;
//   - anthropic: Vivy-owned thin adapter over the Anthropic Messages API
//     (no official Eino component exists yet);
//   - mock: deterministic provider for reproducible tests (FR-3, NFR).
//
// Bundles are YAML documents adapted from the Diva providers.yaml schema
// (D-022..D-025), rewritten into Vivy's owned schema with a provenance
// field. Secrets are read from env at request time, never persisted (D-010).
//
// Skeleton stage: empty. Implemented in tasks C1/C2 and A2.
package provider

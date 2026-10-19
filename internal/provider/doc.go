// Package provider implements the ProviderRef boundary and the embedded
// provider catalog.
//
// The sealed capability set is two protocol adapters (adapters.go), both on
// Eino's agentic generation:
//
//   - openai-completions: POST {base}/chat/completions, backed by
//     github.com/cloudwego/eino-ext/components/model/agenticopenai;
//   - anthropic-messages: POST {base}/messages, backed by
//     github.com/cloudwego/eino-ext/components/model/agenticclaude.
//
// The former openai-responses family was retired with the agentic migration:
// it is not sealed, so an endpoint naming it fails closed as ErrAdapterUnknown
// rather than becoming a visible-but-unusable capability.
//
// Everything else about a provider — which vendor, which address, which
// models, which environment variable holds the credential — is data in
// data/vendors.yaml, embedded into the binary at build time and validated at
// startup. There is no runtime path to that data, no directory setting, and no
// working-directory dependency (PROV-P1, decisions D1 and D3).
//
// Committed data and config hold env_key names only. Runtime credentials
// arrive as ModelSpec values from the user workspace or a frozen ENV session,
// never by reading process environment inside this package (D-010).
package provider

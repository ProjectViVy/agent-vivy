# schemas

Vivy-owned public contracts (JSON Schema / OpenAPI).

Planned contents:

- `api.openapi.yaml` — the UI facing command/query/event surface.

Landed:

- `events/run-event.schema.json` + `events/payloads/*.json` — the journal
  `RunEvent` vocabulary, including parent-child worker lifecycle events. This
  is the single contract that runtime, the JSON-RPC control plane, and the UI
  speak; the vocabulary mirrors
  `internal/domain` EventTypes (B3).
- Provider data schema — **moved in PROV-P1** to
  `../internal/provider/data/provider.schema.json`, beside the data it
  describes. The data itself (`internal/provider/data/vendors.yaml`) is
  embedded in the binary, so there is no `fixtures/provider/` directory and no
  `providers.bundle_dir` setting. Secrets are `env_key` names there too
  (D-010, D-022..D-025).

Nothing in here may reference Eino or reference-project types (D-007).

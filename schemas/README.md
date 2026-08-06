# schemas

Vivy-owned public contracts (JSON Schema / OpenAPI).

Planned contents (task A3):

- `runevent.schema.json` — the ten `RunEvent` types (PRD FR-5). This is the
  single contract that both the Eino event stream (via `internal/runtime`)
  and the UI event stream (via `internal/httpapi`) speak.
- `api.openapi.yaml` — the UI-facing command/query/event surface.

Landed:

- `providers.bundle.schema.json` — the provider YAML bundle shape
  (D-022..D-025), with the mandatory `provenance` field (task A2, done).
  Instances live in `../fixtures/provider/` (`openai.yaml`, `anthropic.yaml`).

Nothing in here may reference Eino or reference-project types (D-007).

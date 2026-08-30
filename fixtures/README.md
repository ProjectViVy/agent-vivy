# fixtures

Deterministic fixtures for tests and offline development.

Planned contents:

- `provider/` — provider bundle documents used by the real OpenAI-compatible
  and Anthropic routes.
- `events/` — golden `RunEvent` streams per acceptance scenario (AS-1..AS-9).
- `recovery/` — mid-run SQLite snapshots used by restart-recovery tests
  (AS-6, FR-8).

Fixtures must be real recorded data, never mocks standing in for domain
records in production paths (PRD §6.2).

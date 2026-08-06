# fixtures

Deterministic fixtures for tests and offline development.

Planned contents:

- `provider/` — canned mock-provider responses producing reproducible runs
  so unit tests assert exact event sequences (NFR deterministic test mode,
  AS-8).
- `events/` — golden `RunEvent` streams per acceptance scenario (AS-1..AS-9).
- `recovery/` — mid-run SQLite snapshots used by restart-recovery tests
  (AS-6, FR-8).

Fixtures must be real recorded data, never mocks standing in for domain
records in production paths (PRD §6.2).

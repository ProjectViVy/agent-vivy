# R1 verification

Backend: sqlite + real Postgres (`VIVY_POSTGRES_TEST_DSN`, vivy-pg:55432).

- `go test ./internal/runtime ./internal/storage/sqlite ./internal/storage/postgres -run 'TestReport(Period|Collect|Settings|Feedback)'` — PASS
  (period: daily/weekly-Monday/leap-month/DST/manual-as-of; settings ensure,
  replay, CAS + stale conflict; generation insert/get/dup/list; bounded
  collection excludes ingest-protected rows and hidden-purpose sessions;
  feedback snapshots carry digests — both dialects, no skips).
- `go test ./internal/runtime ./internal/storage/sqlite ./internal/storage/postgres -run 'TestReport(Narrat|Render|Fallback|Empty|Cancel|Publication|Generation|Persist)'` — PASS
  (one model call, zero tools; fallback on malformed + unknown-source refs;
  empty skips the model; publication promoted/replay/candidate/head-untouched;
  cancel leaves no generation receipt).
- `TestReportWorkflowEndToEnd` — admission → sealed INOFY program → atomic
  publication → `GetReport` provenance, all on real sqlite.
- `internal/app`, `internal/modules/reports`, `internal/actionhost` — PASS.
- `just ci` — PASS (see acceptance.md; includes pack + Inspect evidence).

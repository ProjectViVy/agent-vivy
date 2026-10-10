# R0 Rollback

The R0 lane is additive and inert when unselected:

- **Recipe-level removal** — `vivy/reports` is optional; omit it from a
  recipe (or omit `core/report-service@v1` consumers) and the generated
  assembly physically lacks the factory, actions, and module. The default
  recipe does not select it, so default builds are unchanged.
- **Code-level revert** — revert the commit
  `feat(reports): add trusted root workflow admission`. Everything below is
  inside it: `internal/reportcontract/`, `internal/modules/reports/`,
  `internal/app/assembly_reports.go`, `internal/runtime/report_*.go`,
  migration 038 (both dialects), `recipes/reports-backend.vivy.yml`,
  observers/app/ports/catalog/source/generator edits, and the new tests.
- **Schema** — migration 038 adds `runs.purpose`, `sessions.purpose`, and
  the `workflow_revisions` columns/UNIQUE swap. To roll back an applied
  database, restore from the pre-038 backup or rebuild from N1 schema;
  `purpose` values are ignored by pre-R0 code paths (columns are additive).
  Postgres additionally drops `workflow_revisions_parent_run_id_fkey`;
  re-adding it restores the old constraint shape.
- **Hidden sessions** — reverting leaves any committed `report-control`
  Sessions hidden from `ListSessions` (still enumerated by
  `ListSessionsForRecovery`); remove rows manually if abandoning the lane.
- **Generated bindings** — re-run `go generate ./internal/generated/assembly`
  after any revert; never hand-edit `zz_default.go` or
  `conformance_results.json` (re-pin the internal `sourceSha256` rows with
  `go run ./sdk/internal/cmd/source-hash internal ""`).

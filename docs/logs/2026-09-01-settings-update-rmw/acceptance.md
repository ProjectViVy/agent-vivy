# Acceptance: how to verify manually

- The behavior is invisible to a single user—this is a concurrency-correctness
  fix. Observable evidence:
  - `go test ./internal/app/settings/ -run TestUpdateConcurrentUpserts -race`
    is green: 8 concurrent writers each write one provider entry, and all 8
    survive. Restoring the Load→modify→Save implementation makes the same test
    lose entries (it can be verified by stashing locally and rerunning).
- All Settings UI flows continue to work (regression surface): model/provider
  registry create/update/delete and "Refresh model list", MCP create/delete,
  tool toggles, channel controls, and sandbox/compaction/network preference
  saves are covered green by `just ui-e2e`.
- The e2e `model-refresh` spec (the real UI path that triggers the two-stage
  refresh) passes, confirming that the refactor did not change user-visible
  refresh behavior.

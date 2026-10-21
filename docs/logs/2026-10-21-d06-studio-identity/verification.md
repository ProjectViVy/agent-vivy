# Verification

Baseline: `e2c1518e`, isolated branch `codex/issue91-d06-studio`.
Independent Linux checkout:
`/home/mastwet/work/vivy-issue91-d06-studio/agent-vivy`, with separate Laputa at
`ff3936f44ff8cf08c12af2cf698c194cfe474fd3`; Go 1.26.4.

## Executed

- Ledger format tests RED: original implementation retained the artifact hash
  column and admitted legacy/unknown shapes. GREEN after marked final schema
  and read-only format refusal.
- `TestReleaseRejectsEvalForAnotherCandidate` RED on unrelated eval admission;
  GREEN after exact candidate linkage validation.
- Converter RED on missing implementation; GREEN on custom artifact mapping,
  non-file refs, all history tables, event allocator, copied snapshots, actual
  converted-service rollback, existing-target refusal, unknown schema,
  deterministic retry, aliased source-home refusal and injected copy/SQL/
  verification/publication failures. No Linux skips.
- `go test ./internal/studiocore -count=1 -timeout 8m`: PASS before coordinated
  SDK changes were copied. The health-probe candidate uses `vivy_headless`;
  it does not require frontend build assets.
- `go test ./internal/studiocore ./internal/studio ./internal/transition/studioledger/... ./cmd/vivy-studio -count=1 -timeout 8m`: PASS against read-copied
  main-lane SDK/module-source changes and local-only planned app companion edits
  (new manifest API, removed LiveView policy hash assignment and unused snapshot
  variable). No shared companion files are included in this lane's commit.
- `node --test studio/dsh-species-workbench/species.test.mjs studio/dsh-species-workbench/confirm.test.mjs studio/dsh-species-workbench/recipe.test.mjs`:
  PASS, 12 tests, no skips.
- `node --check studio/dsh-species-workbench/client.js`: PASS.
- `go vet ./internal/studiocore ./internal/studio ./internal/transition/studioledger/... ./cmd/vivy-studio`: PASS.
- Go formatting and `git diff --check`: PASS.

## Limits

The candidate executable initially failed without UI assets; the lifecycle
health-probe helper now explicitly builds the supported headless variant.
A combined run during SDK-only copying failed because old module source literals
still used removed fields; copying coordinated source projections resolved it.
Final candidate checks must rerun after actual branch integration.

This subsystem lane does not claim `just wiring-ci`, `just kernel-ci`, complete
Recipe matrix, full product CI, browser walkthrough, native Windows acceptance,
Studio product build or D01 acceptance. The lead integration lane owns those
checks. Core schema/D03 and D14 finite-artifact release boundaries stay open.

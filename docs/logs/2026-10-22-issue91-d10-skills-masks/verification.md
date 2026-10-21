# Verification

Independent Linux clone: /home/mastwet/work/vivy-issue91-d10-sm/agent-vivy with its own clean pinned sibling Laputa. Go/Node PATH and GOFLAGS=-p=2 follow the project environment skill. No production Journal was accessed.

- PASS: go test ./internal/maskcontract ./internal/modules/masks ./internal/tools ./internal/storage/sqlite -count=1. Full SQLite suite executed, including existing mask CAS, operation replay, capture and store conformance.
- PASS: go test ./internal/storage/migrations -run TestD10 -count=1 -v. Finite upgrade preserves actual inputs/status/history, fences unknown old inputs, drops retired columns, reopens, and rolls back each migration on injected failure.
- PASS: temporary focused source bundle of real skills_backend.go, marketplace.go, project_instructions.go, always_skills.go and their dedicated backend/marketplace tests, with unchanged local helper functions extracted from their existing owners. 37 named cases/subcases passed. This isolates unavailable unrelated orchestration APIs; it is not full-runtime acceptance. Tests cover same-length stale frontmatter, large content, missing versus empty, explicit raw reads, read-only projects, rejection, restart, full-directory delete/rollback and symlink-parent denial.
- Observed RED then GREEN: missing-file and deleted-directory rollback proposals initially failed against the existing NOT NULL before_payload column. Persisting empty bytes with the explicit existence value fixed both regression cases. Initial runtime regression execution was blocked before tests by unrelated pinned cognitive/INOFY API adaptation.
- PASS: pnpm --dir ui exec vitest run ../plugins/vivy-masks-ui/ui/vivy-masks/src. Six files, 26 tests. Uses authoritative plugin source; covers stale drafts, CAS retries and selected UI actions.
- PASS: offline frozen pnpm installation in the independent Linux checkout.
- PASS: git diff --check and gofmt on changed Go owner files.
- COMPILE ONLY / SKIPPED: go test ./internal/storage/postgres -run TestMask -count=1 -v. All three real PostgreSQL mask cases skipped because VIVY_POSTGRES_TEST_DSN was unset; no PostgreSQL runtime acceptance is claimed.
- BLOCKED: full go test ./internal/storage/migrations -count=1 has three shared latest-head assertions still expecting 41 (manifest_test.go and runner_test.go). They must follow final integrated head after D11. All D10-specific cases pass.
- BLOCKED: go test ./internal/runtime and normal pnpm --dir ui test (SDK stage-ui) stop at cognitive StrategyDigest and INOFY DefinitionDigest/ArtifactDigest/UsedCatalogDigest consumers being adapted in other lanes. No stubs were added.
- PENDING INTEGRATION: shared RPC/API/demo/type adapters, final generated UI/typecheck, split-origin browser smoke, wiring-ci/kernel-ci/affected Face gates, real PostgreSQL/race and broader plan commands. This focused owner commit does not claim D10 end-to-end acceptance.

Temporary source bundles, transfer files and test scripts were kept only in per-checkout ignored scratch and removed from the implementation checkout before commit. The parent can reproduce the committed tests through the normal runtime suite once shared compilation is restored.

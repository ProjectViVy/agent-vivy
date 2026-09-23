# Session Continuity implementation verification

## T0 baseline refresh — 2026-09-22

This log records observed planning evidence only. No Session Continuity product code, migration, runtime, RPC, UI, recipe, generated assembly, issue, or PR state was changed by T0.

### Observed baseline facts

- Code baseline: `a8d361b0244a1c40be513622bbdaebb5c9d40014`.
- PR #45 is merged through `680ef78`; its centralization implementation is `760ac1c`.
- Central migration owner: `internal/storage/migrations`, with paired embedded SQLite/PostgreSQL SQL and `manifest.go`/`runner.go`.
- Observed migration files: `internal/storage/migrations/sqlite/023_workspace_path.sql` and `internal/storage/migrations/postgres/023_workspace_path.sql`; `024` is the next available logical number. T0 did not reserve or create it.
- `origin/feat/issue-47-goal-plan-foundation` is identical to current main and contains no `session_work_events` implementation or migration; Issue #47 remains design-only.

### Message-writer audit scope

Command:

```sh
rg -n 'INSERT.*messages|AppendMessage|AppendMessageIfAbsent' internal/storage
```

Observed implementation writers: `internal/storage/sqlite/messages.go`, `internal/storage/postgres/messages.go`, `internal/storage/sqlite/history_mutations.go`, and `internal/storage/postgres/history_mutations.go`. The audit also returned the shared contract plus conformance, upgrade, registry, SQLite, and file-context tests. The planned future position allocation must cover ordinary `AppendMessage`, deterministic `AppendMessageIfAbsent`, history mutations, Journal projections, and fork writers; no product code was changed.

### Available checks

| Command | Result |
| --- | --- |
| `node --test scripts/check-plugin-v1-fixtures.test.mjs` | PASS — 5 tests passed, 0 failed. |
| `node scripts/check-plugin-v1-fixtures.mjs` | PASS — fixture corpus reported 13 cases (2 accept, 11 reject). |
| `node --test scripts/check-i18n-cross-face.test.js` | PASS — 8 tests passed, 0 failed. |

### Unverified checks and environment skips

| Command | Result |
| --- | --- |
| `just ci` | UNVERIFIED — exit 127: `/bin/bash: just: command not found`. This is not a passing CI result. |

`node` and `pnpm` are available. `go`, `just`, PowerShell (`pwsh`/`powershell`), Docker, PostgreSQL client (`psql`), Chromium, and Playwright are unavailable; `ui/node_modules` is absent. Accordingly, Go tests, `just ci`, PostgreSQL checks, HTTP/UI browser smoke, and Playwright/browser-cache checks are unverified, not passes.

### T0 conclusion

T0 refreshes the SC-D4/SC-P5 documentation and records evidence only. T2 and T4 are Planned because the centralized migration owner is present, but no product Story is Ready: each still requires implemented and accepted predecessor evidence.

## Phase 0 toolchain burn-down — 2026-09-23

`go` (go1.26.4 windows/amd64) and `just` (1.46.0) became available on the host, lifting the exit-127 block recorded above. This section re-runs the T1–T3 focused verification debt and records observed results. PostgreSQL remains unavailable (`VIVY_POSTGRES_TEST_DSN` unset): every Postgres-gated case stays SKIP — not a pass. `pnpm`/`node` are available; `ui/node_modules` is installed by the ci `ui-ci` step.

### Findings and fix-forward classification

The first-ever execution of the T1–T3 test surface exposed four never-run defects. All four are stale artifacts of stories accepted without execution evidence; the production contracts themselves are intact, so each was fix-forwarded per the Phase 0 rule and the owning story rows stay Accepted with this annotation:

| Finding | Class | Fix commit |
| --- | --- | --- |
| `TestEventVocabulary` golden 38, actual 40: T1 intentionally added `context.reference_attached` and `deliverables.presented` but never updated the golden | Stale T1 test golden | `466c7cfa` |
| `TestContinuityCanonicalHistoryScopeHash` golden `6d9aef6d…` was a placeholder; the deterministic implementation computes `a88372eb…`, independently confirmed via `sha256sum` of the canonical JSON | Stale T1 test golden | `466c7cfa` |
| `TestMessagesPersistImageAttachments` appended the reborn message before creating session `s-att2`; T2's fail-closed position allocation requires the session row first. Reordered the fixture; assertion intent unchanged | Stale pre-T2 fixture vs legitimate T2 contract | `d8a7b873` |
| `internal/runtime` test binary did not compile: T3's `history_service.go` redeclared `truncateUTF8` (existing: `tooladapter.go`) and `containsString` (existing: `search_backend_test.go`). Additionally `TestHistoryTraceRejectsMissingTrustedSession` failed: the `query == nil` unavailable disclosure preceded the missing-authority check in Search/Read/Trace, letting unauthorized callers probe capability state | T3 implementation defects | `28b01b67` |
| 14 tracked `.go` files from T1–T3 commits were never gofmt-ed (fmt-check had never observed them) | Formatting debt | `b5c1d65a` (plus the three commits above) |

Fixes reused existing helpers (`takePrefixUTF8`, `slices.Contains`) per single-source-of-truth; the duplicate `truncateUTF8` in T3 also returned the full text on a zero budget, while the retained `takePrefixUTF8` is fail-closed.

### Observed results after fixes

| Command | Result |
| --- | --- |
| `go test ./internal/domain/... -count=1` | PASS |
| `go test ./internal/storage/... -count=1` | PASS (sqlite 45.0s; postgres package PASS via DSN-gated SKIP — unverified, not a pass) |
| `go test ./internal/runtime ./internal/tools ./internal/rpc -run 'TestHistory' -count=1` | PASS (runtime 7.0s, tools 0.1s, rpc 0.2s) |
| `gofmt -l` over all tracked `*.go` | clean (0 files) |
| `just ci` | PENDING — recorded in the addendum below |

### Phase 0 addendum

Full-suite burn-down (2026-09-23, after `go`/`just` became available):

| Command | Result |
| --- | --- |
| `go test ./internal/runtime ./internal/app ./internal/rpc -count=1` | PASS after fixture fix `29908309` (73 stale fixtures across 21 test files called `Run`/`AppendMessage`/`CreateRun` without the session row that T2's fail-closed position allocation requires; test-only change, no assertions modified). Before the fix: 66 runtime + 6 app + 1 rpc FAIL, all `storage: not found`. |
| `go test ./internal/runtime ./internal/rpc ./internal/app ./internal/tools ./internal/storage/... -count=1` | PASS after `d3f4878b` (runtime 82.2s, rpc 51.6s, app 30.7s, tools 0.6s, storage/sqlite 51.9s, storage/migrations 1.4s, storage/postgres PASS via DSN-gated SKIP — unverified, not a pass). |
| `go test ./sdk/tui/live ./sdk/tui/stream -count=1` | PASS after `d3f4878b`. |
| `cd faces/headless && go test ./... -count=1` | PASS (separate module) after `a320f55f`. |
| `cd ui && pnpm vitest run src/components/chat/ChatView.test.tsx src/lib/run-rows.test.ts` | PASS — 14/14 after `d3f4878b`. |
| `gofmt -l internal sdk` | clean (0 files). |
| `go vet` (runtime, sdk/tui/live, app, rpc, provider) | clean. |
| `just ci` | PASS on `41e67904` (successor lane, Linux + pwsh 7.6.6, go1.26.4, just 1.58.0, node 24.21.0, pnpm 11.19.0): fmt-check clean; ui-ci green (tsc, vitest 392/392 across 48 files, vite build, i18n completeness + cross-face checks); `go vet` clean; `go test -timeout 20m ./...` all packages ok including internal/runtime 22.7s, internal/rpc 8.3s, sdk/internal 220.9s, sdk/internal/conformance 65.3s; headless-compile ok; plugin-ci all `plugins/*` and `faces/*` modules ok. Two stale conformance digests surfaced and were fixed first in `cd30a886`/`41e67904` (internal-rooted suites now record `478b59c8`; `vivy/headless` records the `948c24ec` self-referential fixed point). ci-generated `ui/src/generated/assembly.ts` drift was `git restore`d, not committed. |

Contract change landed during Phase 0/T3 closure (user-directed): `a320f55f` removed the legacy v1 `model.completed` read path everywhere (v2 hash-commit is the only accepted shape; v0/v1 fail closed). `d3f4878b` fixed the two re-review blockers: reconcile skips pre-v2 legacy runs via the `errUnsupportedCompletedVersion` sentinel so old journals stay readable (run-completion path still fails closed), and the remaining v1-shaped fixtures/readers in `sdk/tui/live` and `ui/` were moved to v2.

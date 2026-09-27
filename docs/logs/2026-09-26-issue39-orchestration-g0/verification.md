# Verification — Issue 39 ORCH-01 G0 attempt

## Environment and source

- Branch: `feat/issue39-recovery-boundary`
- Baseline / current `HEAD`: `bbcbd10`
- Go: `go1.26.4 linux/amd64`
- Node: `v24.19.0`; pnpm: `11.25.0`
- `just`: unavailable; PowerShell (`pwsh`): unavailable
- `VIVY_POSTGRES_TEST_DSN`: unset (only presence was checked; no secret value was read or recorded)
- The Go toolchain was added to the process `PATH`; `GOFLAGS=-buildvcs=false` was set because SDK tests build temporary source overlays outside the repository.

## Go tests and static checks

- `PATH="/root/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.4.linux-amd64/bin:$PATH" GOFLAGS=-buildvcs=false go test -timeout 20m ./...` — **PASS before the final broker-policy fix**. All packages completed, including `sdk/internal` (353.730s), `sdk/internal/assembly` (16.130s), and `sdk/internal/conformance` (88.751s). After the policy fix, a first full rerun found the expected stale internal source digest; the five checked-in copies were updated to the test-computed digest. A subsequent full-suite rerun was blocked by automatic review when a test attempted a request to `api.deepseek.com` using unapproved test data. Whether data was sent was not confirmed. No retry was made, so a full-suite result for the final code is **unverified**.
- `PATH="/root/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.4.linux-amd64/bin:$PATH" GOFLAGS=-buildvcs=false go vet ./...` — **PASS after the final broker-policy fix**.
- `PATH="/root/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.4.linux-amd64/bin:$PATH" GOFLAGS=-buildvcs=false go test ./internal/app ./internal/runtime -count=1 -timeout=240s` — **PASS after the final broker-policy fix**.
- `go test ./internal/storage/sqlite ./internal/storage/postgres ./internal/storage/migrations ./internal/domain -count=1 -timeout=180s` — **PASS**. The PostgreSQL package did not connect to a real server; this is not PostgreSQL DSN-backed conformance.
- `go test ./sdk/internal/conformance -run '^TestCheckedInProviderConformanceMatchesExecutedSuites$' -count=1 -timeout=5m -v` — **PASS after source digest refresh**, 23 checked-in provider suites (71.954s).
- `go test ./internal/runtime -run '^TestToolOperationCrashMatrix$' -count=1 -timeout=90s -v` — **PASS**, four separate-process crash boundaries (details below).
- `TestBrokerApprovalCannotOverridePolicyDeny` — the RED run reproduced both failures (initial approved call under `Deny` executed; completed operation replay after policy changed to `Deny` returned success). After the fix, `go test ./internal/runtime -run '^TestBrokerApprovalCannotOverridePolicyDeny$' -count=1 -v` — **PASS** for both cases with `ErrPolicyDenied`; replay does not invoke the tool again.
- Focused approval/security coverage — **PASS**: `TestOrchestrationApprovalResumesThroughService`, `TestOrchestrationApprovalCancellationDoesNotClaimNode`, `TestOrchestrationCancellationStopsApprovalSiblingAndLeavesNoPendingApproval`, `TestNativeOrchestrationApprovalBindsRunNodeAndCheckpointTarget`, `TestToolApprovalFailsClosedWhenMiddlewareRewriteDriftsOnResume`, `TestToolApprovalBindingCannotBeBypassedWhenResumeBecomesAllowed`, `TestToolApprovalJournalRedactsMiddlewareInjectedSecret`, `TestProductionMCPGovernancePathUsesToolHostOrder`, and `TestProductionMCPTransportGovernancePathUsesToolHostOrder`.
- `gofmt -l` over modified and untracked Go files — no output. `git diff --check` — **PASS**.

## Separate-process crash matrix

The external effect is written to an append-only file outside the Journal/operation-row transaction. Recovery opens a fresh store, Service, Engine and Eino Workflow. The test emitted the following identifiers and transition evidence:

| Crash boundary | Run / operation / checkpoint | Journal operation-event sequences | External effects after recovery | Parent-visible recovery |
| --- | --- | --- | ---: | --- |
| Admitted before claim | `run-d15-crash` / `d15-crash-effect-1` / `ckpt-run-d15-crash` | `[2, 3, 4]` | 1 | Stored result returned; no error |
| Claimed before invocation | same IDs | `[2, 3]` | 0 | `ErrToolOperationUnknown`; automatic replay blocked |
| Effect before completion | same IDs | `[2, 3]` | 1 | `ErrToolOperationUnknown`; automatic replay blocked |
| Completion before graph checkpoint | same IDs | `[2, 3, 4]` | 1 | Stored result returned; Eino continued |

The approval/resume integration test also verifies that node A remains admitted but unclaimed at approval, node B completes while A is pending, B's three lifecycle events are not repeated on resume, and the same workflow Run completes with its explicit joined outputs.

## UI checks

- `pnpm install --frozen-lockfile` — **PASS**.
- `pnpm typecheck` — **PASS**.
- `pnpm test` — **PASS**, 49 files / 400 tests.
- `pnpm build` — **PASS**; Vite reported the existing large-chunk advisory (>500 kB).
- `node scripts/check-i18n-completeness.js`, `node --test scripts/check-i18n-cross-face.test.js`, and `node scripts/check-i18n-cross-face.js` — **PASS**.

## Unmet G0 gates

- `just ci` was attempted and exited 127 because `just` is not installed. PowerShell is also absent, and the repository recipe requires it; the established CI recipe therefore remains unverified.
- `VIVY_POSTGRES_TEST_DSN` is unset. Real PostgreSQL migration and conformance execution remains unverified and mandatory for schema acceptance.

**Disposition:** G0 remains **BLOCKED**. The final full Go suite is unverified because automatic review blocked a test's unapproved outbound request. `just ci` and real PostgreSQL conformance are also unavailable. The block does not indicate a reproduced native incompatibility and does not authorize downstream ORCH-02–08 work or product release.

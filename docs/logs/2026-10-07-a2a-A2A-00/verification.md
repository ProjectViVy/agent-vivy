# A2A-00 verification

Environment: Go 1.26.8, just 1.43.1, disposable PG14
(`postgres://vivy:vivy@localhost:5432/vivy_test`), laputa sibling pinned at
`ff3936f44ff8cf08c12af2cf698c194cfe474fd3`.

## A2A-00.1 probe

```text
cd sdk/testdata/a2a-probe && go test ./... -count=1 -v
PASS: 26 checks across TestA2ACustomHandlerOfficialClient (9 subtests),
      TestA2AJSONRPCWireCompatibility (6 subtests),
      TestA2AEventSurfaceContract — 0 SKIP.
Commit 4550625d, human identity 大湿.
```

Key recorded findings: v1.0 JSON-RPC methods are PascalCase
(`SendMessage`, not `message/send`); interceptors on a custom handler
require direct `a2asrv.InterceptedHandler` construction;
`CallContext.Tenant()` is only populated through InterceptedHandler (bare
handlers read `req.Tenant`); the official client silently downgrades
streaming when the card omits `Capabilities.Streaming`; no durable resume
cursor exists on the v1.0 event surface; SSE read cap is 10 MB.

## A2A-00.2 bounded experiment

```text
go test ./internal/runtime -run 'TestEnsureForAdmission|TestDiscardNewPrivateAdmission|TestAdmissionAllocator' -count=1 -v
--- PASS: TestEnsureForAdmissionPrivateLifecycle
--- PASS: TestDiscardNewPrivateAdmissionPreservesNonEmptyDir
--- PASS: TestAdmissionAllocatorNeverRemovesUserDirectories
```

Negative finding (explicit): no `Delete`/`Discard`/`Remove` exists in pinned
laputa `garden/internal/personactx` or `garden/agentapi` — candidate
`frozen_core_sessions` rows cannot be cleaned without a new seam.

## A2A-00.3 documentation reconciliation

- `git diff --check` clean on the adopting commit.
- Relative links updated for renamed section 1.1 anchor (10 touched files,
  scripted check: 0 broken).
- `just ci` (official gate, no `VIVY_POSTGRES_TEST_DSN`): RED with exactly
  one failure — `sdk/codeclient TestClientAgainstRealVivyCode`
  ("persona is not initialized" on post-settle steer). Proven
  **pre-existing on clean `origin/main` (202aac38)**: identical failure
  reproduced in a detached main worktree; introduced by main commit
  `cc54ee27` (persona gate wired into the default headless generation,
  2026-10-06). Everything else green: 81 packages ok, gofmt/vet clean,
  ui typecheck + 598 vitest + i18n audit + vite build + lockfile policy.
- With `VIVY_POSTGRES_TEST_DSN` exported (opt-in, not part of `just ci`):
  one additional failure — `internal/storage/postgres
  TestBackendConformance/CN-21_session_truncation_markers` (FK
  `session_truncations_session_id_fkey` on `sess-fk`, which has no
  sessions row; sqlite schema has no such FK, so the same suite passes
  there). Reproduced on a freshly created database — also pre-existing
  main code (`564777b1`, 2026-10-06), unrelated to this branch.
  `sdk/host/v1 TestExternalModuleConsumesPublicAPI` failed once on a
  goproxy.cn GOAWAY network flake and passed on retry.
- G0 outcomes propagated: design sections 1.1, 8, 10, 10.1, 13, 14;
  index status table and execution rules; A2A-05 config contract
  (single `principal`, no TLS fields); CH-C9 header/identity/layering/
  prohibitions/risk; VIVY-CHANNEL-PACK decision rows and A2A note.

Owner-reportable regressions (pre-existing on main, NOT this branch):
1. `TestClientAgainstRealVivyCode` — headless vivy-code steer now blocked
   by the persona-initialized gate (`cc54ee27`).
2. `TestBackendConformance/CN-21` on PostgreSQL — forked-from marker
   recorded for a session with no sessions row violates the pg-only FK
   (`564777b1`).

# HITL Release Closure Acceptance

Date: 2026-08-12

| Acceptance item | Evidence | Result |
|---|---|---|
| Global queue discovers pending approval | `ui/e2e/smoke.spec.ts`, real `vivy.exe` | PASS |
| Approval executes through server-side decision | Playwright approval scenario; runtime approval tests | PASS |
| Question remains distinct from approval | Playwright question scenario; runtime question tests | PASS |
| Answer enables only after input and survives rerender | Review Center UI plus Playwright answer flow | PASS |
| Narrow viewport remains actionable | 390px Playwright Review Center assertion | PASS |
| Keyboard focus and traversal | Review Center title focus and queue traversal | PASS |
| Expiry, restart, cancel, and stale outcomes | `go test -race ./...` runtime/storage suites | PASS |
| First-writer-wins race safety | SQLite conformance and runtime race gate | PASS |
| Redacted review arguments | SQLite ReviewItem projection tests | PASS |
| Browser Use remains excluded; GraphTool remains test-only | tool exclusion/conformance tests | PASS |

No known P0 acceptance item remains open. The P1 items listed in `docs/TODO.md`
are intentionally outside this release closure.

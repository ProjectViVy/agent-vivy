# Verification

## Gates

- `just ci` — passed (exit 0): Go fmt/vet/test, headless compile, six plugin-ci
  modules, and UI install + `tsc --noEmit` + `vitest run` + `vite build` all
  green (`reviewBusyIds: string[]` refactor, per-ID `respondReview` single-flight,
  and all ApprovalsView/_layout references pass type and lint checks).
- `just ui-e2e` — passed (10 passed / 1 skipped): real browser + real control
  plane; `runtime.spec.ts` covers the main Review path (open the Review sheet +
  response stream), and the busy-lock refactor did not regress existing
  assertions.

## Smoke notes

- There is no component-specific spec for the browser assertion of two-record
  concurrent responses (select B while A responds, then respond after B
  unlocks); following the CH-C1-N3 precedent, the full e2e suite is the smoke
  substitute, and the manual observation path is in acceptance.md.
- WebSocket RPC transport (`/rpc/bootstrap` → WS upgrade) has no curl smoke
  path.

## Static review evidence

- `ui/src/lib/store.ts` `respondReview`: `includes(id)` provides single-flight
  (same-ID repeats early-return); the busy set adds/removes by ID (`finally`
  filters it); optimistic state mapping is by ID, and `loadReviews()` is an
  idempotent, concurrency-safe GET.
- No `reviewBusyId` (singular) remains anywhere in the repository (grep review);
  same-named local variables in cron/mcp and other components are unaffected.
- `ui/src/routes/_layout.tsx`: the sheet close guard
  `reviewBusyIds.length > 0` keeps the sheet closed while any response is in
  flight (same semantics as the original `!!reviewBusyId`).

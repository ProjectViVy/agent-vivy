# Verification

## Gates

- `just ci` — passed (exit 0): Go fmt/vet/test, headless compile, six plugin-ci
  modules, and UI `pnpm install --frozen-lockfile` + `typecheck` (`tsc
  --noEmit`) + `vitest run` + `vite build` all green. After the two dashboard
  cases in demo-api.test.ts were rewritten to use the memory layer, the full
  suite passed.
- `just ui-e2e` — passed (exit 0): 10 passed / 1 skipped (the cron long-path spec
  is skipped under the existing baseline). The suite first runs `pnpm build`
  (including this change), then Playwright drives the real browser against the
  real control plane (`runtime.spec.ts` uses real session/Review/Settings paths).

## Smoke notes

- This slice changes user-visible behavior; browser-level evidence is the full
  `just ui-e2e` suite (real browser + real control plane + build artifacts
  containing this change), following the CH-C1-N3 precedent that uses the full
  e2e suite as the smoke substitute when there is no component-specific spec.
- There is no dashboard-specific spec, so the three overview cards and exact
  numbers lack assertion-level coverage. `session/list` and `review/list` are
  exercised by the real backend through existing application paths (session list
  / Review Center); `background/list` is the same family of read-only RPCs.
  Conclusion: type checks + unit tests + the full e2e suite cover wiring
  correctness; exact-number assertions can wait for a dashboard spec (not
  blocking this item—the audit goal of connecting real RPCs is established by
  the code).
- RPC transport is WebSocket (`/rpc/bootstrap` → WS upgrade), with no direct
  curl path; no manual curl smoke was run.

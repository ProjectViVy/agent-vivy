# Verification

| Command | Result |
|---|---|
| `pnpm typecheck` (ui/) | passed |
| `pnpm test -- --run` (ui/) | passed — 24 files / 196 tests |
| `just ui-e2e` (pnpm build rebuilds the embedded bundle + full e2e, including new lifecycle-readonly.spec.ts) | passed — 13 passed + 1 skipped (runtime.spec skipped with no provider), including `lifecycle-readonly.spec.ts:5 992ms` |
| `just ci` (fmt-check + vet + go test + headless + plugin-ci + UI: typecheck/test/build) | see below |

Key points:
- New spec `lifecycle-readonly.spec.ts` is provider-independent: Settings → Vivy features →
  "Open lifecycle" → current Species + authoritative note visible; Create/start evaluation/
  confirm promotion/Reject buttons all count 0; Generations/Evals/Promotions tabs are
  switchable and have no write buttons.
- The read-only list empty state is simply an empty list (the spec does not assert specific
  entries, avoiding dependence on backend state).

# Verification

- `pnpm vitest run src/components/settings/channel-store.test.ts` — 12/12
  (the legacy test previously asserted non-migration; now asserts deletion
  and zero writes).
- `just ci` — exit 0 (fmt-check, vet, go test, headless compile,
  plugin-ci, UI tsc/eslint/vitest/vite build).

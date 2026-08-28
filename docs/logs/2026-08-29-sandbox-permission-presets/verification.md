# Verification

## Commands

- `go test ./internal/domain ./internal/runtime ./internal/rpc ./internal/app ./internal/app/settings` — pass
- `pnpm test -- --run src/components/settings/diva-preview-data.test.ts src/lib/api.test.ts src/i18n` in `ui/` — 12 tests pass
- `just ci` — pass (`fmt-check`, `vet`, `go test ./...`, UI typecheck / vitest 161, `vite build`)

## Notes

- Command-backend tests still construct `danger_full_access`; without a session context they now fall back to the manager default mode.
- Settings overlay empty `allowed_domains` is normalized to nil so YAML round-trips stay stable.
- Sandbox path checks compare against the symlink-resolved workspace root (Windows short-path / junction).
- Playwright `ui/e2e/sandbox-setting.spec.ts` is added; live `:3015` smoke is recorded separately if the split pair is running.
- `npx playwright test e2e/sandbox-setting.spec.ts` against the split pair — pass (settings → 沙箱, no preview badge, persist 谨慎 then restore 智能)

# Verification

- `gofmt -l internal/channelhost internal/rpc` — clean.
- `go vet ./internal/channelhost/... ./internal/rpc/...` — pass.
- `go test -race ./internal/channelhost/ ./internal/rpc/` — ok (rpc needed
  the accesslog race fix first, committed separately as 9c86023).
- `pnpm vitest run src/components/settings/channel-store.test.ts` — 12/12.
- `just ci` — exit 0 (fmt-check, vet, go test, headless compile,
  plugin-ci, UI tsc/eslint/vitest/vite build).
- `just ui-e2e` — exit 0, 10 passed / 1 skipped (Playwright suite covers
  settings navigation; no channel-specific spec exists).

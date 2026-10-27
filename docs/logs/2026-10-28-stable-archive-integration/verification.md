# Verification

## UI merge checkpoint

- `git diff --check` and staged diff check: passed before dependency verification.
- Initial UI staging with PATH Go 1.27.1 failed compiling grpc v1.64.1 against x/net v0.54.0 (`http2.TrailerPrefix`). `go.mod` and `go.sum` are identical to origin/stable. Repeating the minimal grpc compilation with the repository's Go 1.26.4 succeeds. No dependency update was made; subsequent Go commands use `GOTOOLCHAIN=go1.26.4`.
- `GOTOOLCHAIN=go1.26.4 pnpm --dir ui typecheck`: passed after resolving the removed radio-group import.
- `GOTOOLCHAIN=go1.26.4 pnpm --dir ui exec vitest run src/components/chat/ChatInput.queue.test.tsx src/components/settings/ComposerSettingsCard.test.tsx src/lib/store.test.ts src/components/chat/ChatView.test.tsx`: 4 files, 95 tests passed.

This is a merge checkpoint, not final release acceptance. Kernel/full UI gates, browser smoke, remote SHA verification and deletion readiness have not yet been certified here. Native Windows, real Postgres and Docker acceptance are not implied by Mac results.

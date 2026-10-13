# Verification

All checks ran on Windows from the isolated `fix/smoke-blockers` worktree. No production Journal or provider credentials were used.

Focused regression checks passed:

- `go test ./internal/rpc -run '^(TestControlBindsPeerOnlyFromValidatedSessionAndRun|TestControlBindsTerminalRunToSessionOnly|TestControlRunBindingIgnoresBrowserSessionClaim)$' -count=1` — passed.
- `go test ./internal/app -run '^(TestToolkitPreviewAfterViewingTerminalRun|TestToolkitLifecycle|TestToolkitOwnerOmissionIsReflectedByApp|TestToolkitHumanOriginAdmitsDirectManagementWithoutWeakeningPolicy)$' -count=1 -timeout 3m` — passed.
- `go test ./internal/runtime -run '^(TestBundleHookDispatcherRequiresCapturedArmingAndCurrentConsent|TestScriptHookScopedExecutionUsesExplicitCwdAndEnvironment|TestBashBackendQuotedRedirectsAndHeredocsWriteWorkspaceFiles|TestShellPrefixBashAndCommandline)$' -count=1 -timeout 2m` — passed.
- `go test ./internal/bundles -run '^(TestSafeOriginLocalPaths|TestSafeOriginHTTP|TestSafeOriginRejectsUnsafeSources|TestGitAcquisitionHasNoExecution)$' -count=1 -timeout 3m` — passed.
- `pnpm exec playwright test e2e/toolkits.spec.ts` — 1 test passed.

The real split-browser smoke used the split Vite UI at `http://127.0.0.1:3015`, the local backend, an isolated `VIVY_USER_HOME`, and a disposable OpenAI-compatible mock. Persona initialization and chat were completed in the UI; the assistant answered `Smoke complete`; reloading preserved the conversation; opening Toolkits and previewing a local Toolkit after that terminal Run succeeded; the browser reported no page errors. The active-Run denial path is also covered by `TestToolkitPreviewAfterViewingTerminalRun`.

`just ci` completed with exit code 0. It passed dependency/bootstrap checks, `go vet ./...`, all Go packages, the DIVA Recipe Memory Loop suite, DIVA recall checks, headless compile, and vet/test for all independent plugin and face modules. The UI gate passed TypeScript checking, 86 Vitest files / 639 tests, production build, and i18n checks. In the Go suite, `internal/runtime` passed in 759.803s, `internal/app` in 124.862s, `internal/rpc` in 193.533s, `internal/storage/sqlite` in 244.392s, `sdk/internal` in 634.627s, and `sdk/internal/conformance` in 191.710s. The DIVA Recipe pass reported all six Memory Loop tests passing, and the recall script reported all six recall checks passing.

During `pnpm install --frozen-lockfile`, pnpm printed a TLS failure while checking package-manager update metadata from the configured registry mirror. The lockfile was already current, installation completed from the local store, and typecheck, tests, build, and the full `just ci` command all passed.

`git diff --check` passed. CI-generated changes to `ui/src/generated/assembly.ts` and `ui/src/routeTree.gen.ts` were restored. Cleanup of four untracked local browser-debug scripts under `ui/.smoke-*.mjs` was attempted per-file, but the execution policy rejected `Remove-Item` as blocked; the scripts remain only in the repair worktree and are excluded from the commit.

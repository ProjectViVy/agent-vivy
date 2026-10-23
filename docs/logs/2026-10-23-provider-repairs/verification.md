# Verification

## Test-first evidence

`go test -tags=vivy_headless ./internal/app ./internal/app/settings ./internal/provider ./internal/rpc -run TestProviderRepair -count=1`

Before implementation, all four areas failed with the expected symptoms: custom credential pollution, arbitrary environment selection, missing default models, and accepted discovery overflow that changed settings. After implementation, all four packages passed this selector on Windows. Tests cover synthetic shared credential names; vendor/adapter/configured/sole-candidate selection; preserved settings and secret-free diagnostics; caller backing-array ownership; load without write; default reconciliation round trips; multibyte 256/257-byte boundaries; accepted-ID counts at 2,000/2,001; and byte-identical settings after failed refresh.

## Independent Linux acceptance

Checkout: `/home/mastwet/work/vivy-provider-repairs/agent-vivy`, its own Laputa checkout at `9acc999301d5d575da2cea902a1c6d213763f381`. Go `1.26.4`, Node `24.21.0`. Source started at stable `7496f35f` plus the isolated provider-repairs diff and four new test files; no Windows dependencies, runtime state, or node_modules were shared. Temporary transfer duplicates were removed from Windows before delivery.

| Command | Result |
| --- | --- |
| `just wiring-ci` | PASS: 111 executed Go tests; bootstrap/evidence checks passed. |
| `just kernel-ci` | PASS: vet; 1,975 executed tests passed, 1 skipped; minimal Recipe pack, Inspect, and boot test passed. |
| `just d02-ci` | PASS: 83 runtime/RPC tests, 175 Face/client tests, 1 causal app test, 13 shutdown/headless tests, and 9 headless-module tests; no skips. |
| `go test -tags=vivy_headless ./internal/app ./internal/app/settings ./internal/provider ./internal/rpc -count=1` | PASS: app 15.562s, settings 0.594s, provider 0.267s, RPC 23.983s. Includes existing custom-provider outbound credential/raw-model assertions. |
| `pnpm --dir ui install --offline --frozen-lockfile` | PASS with independent Linux dependencies. |
| `pnpm --dir ui build` | PASS; existing large-chunk warning only. No frontend source changes. |
| `git diff --check` | PASS. |

The kernel skip is `TestBackendGrepRipgrepHonorsGitignore`: Linux ripgrep was unavailable. It is unrelated to the provider changes and is not reported as executed acceptance. The gate itself returned zero. Full-product `just ci`, race, PostgreSQL, Docker and packaged native Windows executable acceptance were not run; they are separate platform/release lanes outside this batch's acceptance scope.

## Browser smoke

The owner approved temporary split ports `8788/3016`, because another lane owns `8787/3015`. Candidate backend: `VIVY_ADDR=127.0.0.1:8788 VIVY_USER_HOME=/home/mastwet/work/vivy-provider-repairs/smoke-home go run -tags=vivy_headless ./cmd/vivy`. Candidate UI: `VIVY_BACKEND_ADDR=http://127.0.0.1:8788 pnpm --dir ui dev --host 127.0.0.1 --port 3016`. This is the split Vite path, not embedded UI. All data was disposable; tenant Journals were not accessed.

Through the real browser at `http://127.0.0.1:3016/settings`:

1. Created custom provider named `deepseek` using a loopback-only mock at `http://127.0.0.1:18437/v1`, default `preferred-model`, and manually supplied list containing only `manual-model`.
2. Confirmed both models appeared, selected `preferred-model`, and observed it in the header and saved quick-switch list.
3. Refreshed successfully from the mock and observed `discovered-0`, `discovered-1`, `manual-model`, and `preferred-model`.
4. Switched the mock to 2,001 unique IDs and refreshed. The UI displayed `upstream advertised more than the 2000 model id limit`, retaining the previous list and active default.
5. `sha256sum --check` confirmed the disposable settings file remained byte-for-byte unchanged after the failed refresh.

Evidence: `browser-smoke.png`. No real provider keys or live upstream network were used.

## Windows limitations and baseline failures

Native baseline `go test ./internal/provider ./internal/app/settings` passed (4.439s / 6.708s). App baseline initially could not compile without UI dist, so regression/native checks used the supported `vivy_headless` tag.

The pre-fix full native RPC baseline failed `TestAttachmentPathsFlowAndMessageDTOConsistency`: a run did not become terminal and cleanup emitted database-closed errors. This is outside the changed provider paths; the complete Linux RPC package passed. The failure is recorded in `docs/TODO.md` for separate investigation.

A subsequent full native app/settings/provider invocation was stopped after making no progress for several minutes, once the complete Linux affected-package tests had passed. No full native package-suite pass is claimed.

Native UI build and dev dependency optimization failed with Windows esbuild access-denied errors, including when TEMP/TMP pointed to worktree-local scratch. This matches existing backlog item `CI-VITE-ESBUILD-TEMP`; Linux UI build and browser smoke passed instead. No unrelated host/toolchain workaround was committed.

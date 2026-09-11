# PR #20 integration verification

All commands ran from the repository root on Windows unless a directory is
shown. Tenant Journal paths were not read or written; browser verification used
an explicit `.workspace/smoke-pr20` configuration and SQLite database.

## Focused regression checks

```text
go test ./sdk/internal/assembly -count=1 -timeout 5m
ok

cd ui && pnpm typecheck
ok

cd ui && pnpm test && pnpm build
35 files / 316 tests passed; Vite build passed

cd sdk/ui && pnpm typecheck && pnpm test
2 files / 27 tests passed

go test ./internal/actionhost -run '^TestActionHostTimeoutAndCancellation$' -count=100 -timeout 2m
ok

go test ./internal/actionhost -count=1 -timeout 5m
ok
```

The first full gate exposed a React peer-type resolution failure and two UI SDK
compatibility assertions. After those fixes, a later run exposed a timing race
where a provider's `context.Canceled` could win over the Host deadline. The
focused timeout test failed before the correction and passed 100 consecutive
runs afterward. One already-running Pack matrix also rejected an intentionally
changing source tree while the fix was written; the final untouched-tree gate
below passed.

## Product gate

```text
just --set go 'C:\Program Files\Go\bin\go.exe' --set gofmt 'C:\Program Files\Go\bin\gofmt.exe' ci
exit 0
```

This passed formatting, UI typecheck/tests/build, localization checks, vet,
the repository-wide Go suite, headless compilation, and each independent
plugin/face module.

## Pack, Inspect, and browser smoke

The selected `fixture/full-ui` Generation was packed and inspected through the
real SDK path. Inspect reported:

- Generation ID `79c968d0d0f5b699b06b9fd721e0c70665a08078cd80973cd5733854b669ed2c`;
- root `fixture.full-ui.root` and extension `fixture.full-ui.extension`;
- typed action `fixture.full-ui.echo`;
- source digest `c106d108fd1510eba19351b3eed7d87338fe777188d75ed942db61443d72b8cc`;
- dependency-lock digest `b5d2e923eda55334ec67587ef2b5e75a998c2a9b62b061c43d753fe4295b64c7`;
- final UI asset digest `d0cf727071c78addcc0cdf8ed756f29f0924ead98b120027d14a68030fb64041`.

The packed backend ran at `127.0.0.1:8787`; Vite ran at
`http://127.0.0.1:3015` with the generated selected Assembly. Playwright used
the installed Chrome executable because the separately downloaded Playwright
browser was absent.

```text
pnpm exec playwright test e2e/plugin-full-ui.spec.ts --config .smoke-pr20.playwright.config.ts
2 passed

pnpm exec playwright test e2e/.smoke-pr20-console.spec.ts --config .smoke-pr20.playwright.config.ts
1 passed
```

The scenario verified default omission, selected root/extension composition,
live store state, route/style/theme registration, typed action RPC, rejection
of forged browser authority, absence of UI Grant prompts, and cleanup. No app
console error or failed application request remained; the exact browser-only
favicon 404 was excluded and filed as `UI-FAVICON-404`. The backend and Vite
processes were stopped by verified PID, and ports 8787, 3015, and 8799 were
clear afterward.

Independent final review reported no actionable findings and recommended
approval.

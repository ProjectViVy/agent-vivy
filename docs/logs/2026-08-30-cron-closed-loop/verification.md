# Verification

## Commands

From worktree `../agent-vivy-cron` (branch `feat/cron-closed-loop`, based on main
`ee2ba80`):

```text
just ci          # fmt-check / vet / go test ./... / headless-compile / ui-ci
```

Layered unit tests (run early during development and all included in `go test ./...`):

```text
go test ./internal/runtime/  -run "TestCron|TestNextCronAfter|TestValidateCronSchedule" -count=1
go test ./internal/storage/... -count=1
go test ./internal/rpc/      -run "TestControlCron" -count=1
go test ./internal/config/   -run TestCronConfigDefaults -count=1
```

UI:

```text
cd ui; pnpm typecheck; pnpm test        # 176 vitest all green (including new cron assertions in api.test.ts and zh/en i18n leaf parity)
pnpm exec playwright test cron-tasks    # new e2e
pnpm exec playwright test               # full e2e regression
```

Real-path smoke (split pair; 8787/3015 were occupied by the root-worktree lane, so an
equivalent split was used: dedicated backend 8791 + Vite 3016,
`VIVY_BACKEND_ADDR=http://127.0.0.1:8791 pnpm exec vite --port 3016`):

```text
# Playwright real-browser script: create "smoke scheduled task" at /cron-tasks → Run now
# → wait for "Completed" → view session → verify no vivy.demo.* keys
node smoke-scratch.mjs   # output SMOKE PASS; screenshots 1-cron-page/2-completed/3-session.png (see acceptance.md)
```

## Results

- `just ci` all green: fmt-check / vet / `go test ./...` (including all new cron unit
  tests) / headless-compile / ui-ci (typecheck + 176 vitest + `pnpm build`).
- New e2e `ui/e2e/cron-tasks.spec.ts` passed (real `go run ./cmd/vivy` backend + mock
  model): create task (backend calculates next run) → Run now → terminal state writes
  back "Completed" → dedicated-session button works → reload persists → delete; no
  `vivy.demo.*` localStorage keys or DemoBanner on the page.
- Full e2e regression: 6 passed, 2 failed — `runtime.spec.ts` (asserts the chat
  "Drawing" button) and `welcome-wizard.spec.ts` (asserts stale key copy). **The same
  failures reproduce on the main root worktree**, so they are pre-existing stale specs
  (UI-E2E-DRAW / UI-E2E-STALE are already recorded in §0.1), not introduced by this
  iteration and not fixed here.
- Smoke-script output: `demo banner count: 0 / created: ok / trigger -> completed: ok / session jump: ok / demo keys: [] / SMOKE PASS`.
- Temporary processes and the scratch script were cleaned up after smoke; the
  `ui/dist` placeholder file was not committed (pre-existing UI-CI-BOOTSTRAP issue;
  `pnpm build` produces the real artifact).

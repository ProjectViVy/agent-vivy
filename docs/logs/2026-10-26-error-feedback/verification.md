# Verification

All commands run in the lane worktree (`/Users/mastwet-macmini/pjvv/agent-vivy/.worktrees/ui-overhaul/agent-vivy`)
with the pinned toolchain (`GOTOOLCHAIN=go1.26.4`) on macOS.

## Tests

```
cd ui
GOTOOLCHAIN=go1.26.4 NODE_OPTIONS=--localstorage-file=/tmp/vivy-vitest.db npx vitest run \
  src/lib/toasts.test.tsx src/lib/failure.test.ts src/components/feedback
```

```
✓ src/lib/failure.test.ts (22 tests)
✓ src/lib/toasts.test.tsx (8 tests)
✓ src/components/feedback/FailureNotice.test.tsx (5 tests)
✓ src/components/feedback/ToastViewport.test.tsx (5 tests)
Test Files 4 passed (4) / Tests 39 passed (39)
```

`src/lib/toasts.test.tsx` uses `createRoot`/`act` rather than
`renderToStaticMarkup`: zustand's server snapshot is the initial state, so a
static render could never observe a toast. Plugin tests run from the
authoritative plugin sources per `vitest.config.ts`; masks, memory, persona and
session-tree passed 54/54 after the migration.

Full suite:

```
GOTOOLCHAIN=go1.26.4 NODE_OPTIONS=--localstorage-file=/tmp/vivy-vitest.db pnpm test
Test Files 1 failed | 90 passed (91)
     Tests 1 failed | 744 passed (745)
```

The single failure is pre-existing: `src/lib/store.test.ts > keeps the run.failed
payload as the conversation error` expects `errors.providerDetail` while
`runFailedMessage` has always preferred a non-empty `payload.message` (the same
preference asserted in `src/lib/failure.test.ts`). Reproduced with
`git stash push -- ui/src/lib/failure.ts` (classifier at HEAD) and the same test
still failed; this lane does not touch the `run.failed` handler.

## Typecheck and i18n

```
GOTOOLCHAIN=go1.26.4 pnpm typecheck
# restages ui/src/generated, then `tsc --noEmit` with no output
```

`just i18n-check` cannot run on macOS here: the recipe calls a PowerShell
`ui-core` body (`Set-Location … if ($LASTEXITCODE) { … }`) that `sh` rejects, so
the two gates it wraps were run directly:

```
node scripts/check-i18n-completeness.js
PASS: en=1613 keys / 162 placeholders; zh=1613 keys / 162 placeholders; runtime copy audit clean.

node scripts/check-i18n-cross-face.js
PASS: 13 shared semantic units; Web and TUI en/zh projections and arguments conform.
```

Two pre-existing gate failures surfaced while adding the keys and were fixed
minimally: the uncatalogued Han allowlist for `ui/src/lib/failure.ts`, and a
`model.switch.action` unit pointing at a removed projection (repointed to
`modelSwitcher.chooseModel`, which has no placeholders).

## Audits

```
pnpm audit:contrast
contrast: 7 families, 182 pairs, all >= AA

pnpm run audit -- --base-url http://127.0.0.1:3016 --widths 390,1280 --out .audit/full
audit: 168 screenshots, 0 failures, 0 ink pairs below AA (8834 text nodes measured)
```

The split dev pair was already running (backend `:8787`, worktree Vite `:3016`).
`ui/.audit/` is gitignored; the toast surface is part of `SURFACES` so future
audits keep measuring it.

## Browser smoke (`http://127.0.0.1:3016`)

Seeded through the live module (`await import('/src/lib/toasts.ts')`) at light and
dark, 1280 and 390:

```
light/1280: stackZ=300  pause=held resume=dismissed  escape 3->2  toast-above-menu=yes
light/390:  stackZ=300  pause=held resume=dismissed  escape 3->2
dark/1280:  stackZ=300  pause=held resume=dismissed  escape 3->2  toast-above-menu=yes
dark/390:   stackZ=300  pause=held resume=dismissed  escape 3->2
```

`toast-above-menu` opens the header model dropdown (Radix, `z-50`) and asserts
`elementFromPoint` at the toast centre resolves inside `[role="alert"]`. The
details panel was also expanded once: `<pre>` holds the raw `rpc error …` text and
the copy button reads `复制诊断`.

Screenshots: `shots/toast-{light,dark}-{1280,390}.png`,
`shots/toast-over-menu-{light,dark}-1280.png`,
`shots/toast-details-light-1280.png`.

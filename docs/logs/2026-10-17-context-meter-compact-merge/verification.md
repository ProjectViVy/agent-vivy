# Verification: the context meter is the compact control

Branch `feat/context-compact-merge`, worktree
`C:\Users\com01\Desktop\agent-vivy-context-compact-merge` (base `70ed55ef`).
All commands ran from the worktree unless noted.

## Focused unit tests

```text
ui: pnpm exec vitest run src/components/chat/ChatInput.compact.test.tsx
    -> 5 passed (5): disabled/passive meter, meter-inside-trigger, instructions,
       blank instructions, run-active busy error
ui: pnpm typecheck      -> clean (tsc --noEmit)
ui: pnpm test           -> 92 files / 693 tests passed (full suite, run before
                           the review pass; the file under change did not move
                           afterwards)
```

## i18n gates

`just ui-ci` reaches these through `i18n-check: ui-core`, which is blocked by the
build failure below, so they were run directly:

```text
node scripts/check-i18n-completeness.js        -> exit 0, en=zh=1571 keys, runtime copy audit clean
node --test scripts/check-i18n-cross-face.test.js -> pass 8 / fail 0
node scripts/check-i18n-cross-face.js           -> exit 0, 14 shared semantic units conform
```

## Real-browser check

The merged control was exercised in Chromium against the real backend. Because
Playwright's pinned browser build is not installed on this machine
(`@playwright/test` 1.62.1 wants `chromium-1234`; only `chromium_headless_shell-1243`
exists) and its expectation `chromium_headless_shell-1234` is missing, the run
reused the installed headless shell through a temporary config's
`launchOptions.executablePath`, and the `ui/dist` fed to the embedded backend was
produced with `vite build --minify false --target esnext` (why: below).

```text
ui: pnpm exec vite build --minify false --target esnext
    -> built in 20.74s, dist/assets/index-*.js 4,367.53 kB (2596 modules)
ui: <headless shell 1243> playwright test --config playwright.local.config.ts   (temporary config)
    -> 1 passed: the usage control is a button named "Context usage n%"
       containing the ring svg + compact affordance, no separate compact button
       exists, clicking it opens the window (立即压缩 + instructions field) and
       Escape closes it. 2.9s, against the Go backend on 127.0.0.1:8799 with a
       disposable VIVY_USER_HOME.
```

Skipped: the split Vite venue at `http://127.0.0.1:3015`. Two environment facts,
neither caused by this change, block it:

- `:3015` is held by another lane's Vite dev server (pid owning the port), and
  `:8787` by its backend; starting a second dev server for this worktree on
  another port still fails.
- A Vite dev server cannot start in this worktree at all:

  ```text
  ui: pnpm exec vite --port 3016 --strictPort
      VITE v7.3.6 ready in 2794 ms  then:
      X [ERROR] Failed to write to output file:
        open ...\ui\node_modules\.vite\deps_temp_25d3aca4\lucide-react.js: Access is denied.
      -> node exits 1 during dependency pre-bundling
  ```

  A direct multi-entry `esbuild.buildSync` into `node_modules/.vite/deps_temp_probe`
  and into a worktree scratch dir both succeed, and the same dev-server command
  fails identically with this change stashed, so the denial lives in the
  dep-optimizer path of this environment, not in the diff. Reusing the other
  checkout's warm deps cache is rejected by Vite ("Re-optimizing dependencies
  because vite config has changed") because the resolved config differs per
  checkout path.

## Gates: what passed and what is environment-blocked

```text
just ui-ci -> pnpm install OK, i18n OK, tsc OK, 693/693 tests OK, then
              vite build FAILED:
              [vite:esbuild-transpile] remove C:\Users\...\Temp\esbuild-<hash>: Access is denied.
```

That failure reproduces **without this change**: with the three edited files
stashed at `70ed55ef`, `pnpm build` fails with the same esbuild temp-file removal
22–23s in, after "2596 modules transformed". It is a pre-existing
environment-level esbuild/Windows write+remove denial that hits the chunk
transpile step (skipping chunk transpile via `--minify false --target esnext`
builds successfully), so `ui-core` and everything gated behind it (`i18n-check`
via the recipe, and `just ci` itself) cannot go green in this environment while
the non-build parts of `ui-ci` do.

The remaining `just ci` constituents were therefore run directly (results
appended below).

## Pre-existing e2e spec failure (not from this change)

`ui/e2e/runtime.spec.ts` was updated for the merged control, but it cannot pass
in this environment for a reason unrelated to the diff: the spec expects the
Chinese face while a fresh browser profile resolves the UI locale to its default
`en` (`ui/src/i18n/index.ts` reads `localStorage` or falls back to `'en'`), so it
fails at line 9 looking for `输入消息... (Enter 发送)`. With this change stashed,
HEAD fails at exactly the same line. The permanent spec edit is still the correct
update for the progressbar → merged-button contract and was reviewed by
inspection; the temporary locale-agnostic spec above carried the real-browser
evidence instead.

## Repository gates

```text
just ensure-laputa bootstrap-test   -> 11/11 node tests pass (source lock honoured)
just fmt-check                     -> clean (gofmt over tracked *.go)
just vet                           -> clean
just test                          -> FAIL in exactly two packages, both on the esbuild
                                      denial above, every other package ok:
                                        sdk/internal            FAIL 657s
                                          TestPackAndInspectEveryShippedRecipe,
                                          TestPackBuildsSelectedUIIntoFinalArtifact,
                                          TestSCXCandidateRollbackRestoresPrior…, +9
                                        sdk/internal/conformance FAIL 258s
                                      failure text in every case:
                                        sdk: build Web UI with generated Assembly: exit status 1
                                        [vite:esbuild-transpile] remove C:\Users\...\Temp\esbuild-<hash>:
                                        Access is denied.
just headless-compile              -> exit 0 (cmd/vivy, cmd/vivy-code, ui compile headless)
just diva-recall-test              -> exit 0 (6 PASS TestMemoryLoop* on the SDK DIVA assembly)
just plugin-ci                     -> exit 0 (go vet + go test for every plugins/* and faces/* module)
```

`just ci` as a single command therefore still stops at `ui-ci`'s build in this
environment; every constituent that does not invoke Vite's chunk transpile
passes. The two failing `sdk/internal` suites are artifact-packing tests whose
only failure is the same esbuild child denial (their log line is identical to
the HEAD-reproduced `pnpm build` failure), not the composer change.

Diagnostics narrowing the denial: plain `node` (and `esbuild.buildSync`, which
runs in an in-process worker thread) can create and delete files in `%TEMP%` and
in this worktree, while the esbuild **service binary** that the async
`transform`/`build` APIs spawn is refused; pointing `TEMP` into the worktree does
not change it. No repository file is responsible, and no repository-side fix is
attempted here.

## Merge

Merged into the root checkout's branch `feat/issue-58-unified-context`
(fast-forward by patch when unchanged, `--no-ff --autostash` otherwise). The
root tree already hosted another lane's uncommitted work
(`ui/src/components/chat/ChatInput.tsx`, `ui/src/i18n/*`, `internal/runtime/*`,
`scripts/i18n-cross-face-contract.json`, `ui/src/generated/assembly.ts`), so both
trees were saved as patches (`my-change.patch`, `root-pre-merge.patch`) before
touching anything and both lanes' edits were verified present afterwards.
`ui/src/generated/assembly.ts` and `ui/src/routeTree.gen.ts` were never staged:
they are regenerated by `pnpm typecheck`/`vite` and dirty only by line endings.

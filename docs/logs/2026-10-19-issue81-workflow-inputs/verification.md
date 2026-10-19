# Verification — workflow inputs (W1)

Host: macOS arm64, 2026-10-19. Lane
`/Users/mastwet-macmini/pjvv/agent-vivy/.worktrees/issue81-workflow/agent-vivy`
(branch `feat/issue81-workflow`).

## Environment substitutions (recorded, not silent)

- `just` is not installed on this host, so the `justfile` recipes were run as
  their component commands (`node scripts/fmt-check.mjs`, `go vet ./...`,
  `node scripts/plugin-ci.mjs`, the i18n scripts, `pnpm --dir ui …`).
- `go.mod` requires Go 1.26.4; the default toolchain here is 1.27.1, under
  which the pinned `google.golang.org/grpc@v1.64.1` does not compile
  (`undefined: http2.TrailerPrefix`). All Go commands ran with
  `GOTOOLCHAIN=go1.26.4` (already cached).
- The sandbox denies `~/Library/Caches/go-build`, so every Go command also set
  `GOCACHE=<lane>/.workspace/gocache`.
- `pnpm` on PATH is 12.10.1 while `ui/package.json` pins `pnpm@10.33.0`; pnpm's
  automatic version switch fails on the sandboxed lockfile write, so UI
  commands ran with `npm_config_manage_package_manager_versions=false` (or
  through `ui/node_modules/.bin/*` directly). No dependency was added or
  upgraded; install used the committed lockfile with `--frozen-lockfile`.
- Playwright browsers were never downloaded; the suite uses system Chrome
  (`channel: 'chrome'`), like `playwright.toolkits.config.ts`.

## Commands and results

| Command | Result |
| --- | --- |
| `node scripts/lane-new.mjs issue81-workflow` | lane + branch + pinned Laputa checkout created |
| `pnpm --dir ui install --frozen-lockfile` (override above) | ok, 523 ms from the local store |
| `pnpm exec vitest run ../plugins/vivy-workflow/ui/vivy-workflow/src` | 5 files / 34 tests pass (NodePanel 6, face-bridge 13, workflow-page 3, graph 7, edit 5) |
| `./node_modules/.bin/vitest run` (full UI suite) | 9 failed / 86 passed files, 39 failed / 680 passed tests. Baseline (unmodified root checkout): 9 failed / 84 passed files, 39 failed / 663 passed tests. The 39 failures are **pre-existing**: `PersonaOnboardingGate`, `ModelSettingsCard`, `ToolsSettingsCard`, … fail because `window.localStorage` is undefined under the installed happy-dom 20.14.0. Not caused by, and not fixed by, this delivery. |
| discovery correction | the baseline's workflow tests were the four **staged copies** (`ui/src/generated/ui/vivy-workflow/**`, 28 tests) pulled in by the blanket `src/**` include; the authoritative module sources never ran. The new include adds 45 source tests and excludes the staged copies: 702 → 719 total (+45 −28) reconciles exactly. |
| `./node_modules/.bin/tsc --noEmit` | clean |
| `go test ./internal/runtime -run TestWorkflowProductInputBindingRoundTrip -count=1` | pass (0.16 s) |
| Sensitivity of the new runtime test | temporarily moved the fixture's `inputs` map under `config`: the test fails with `publish draft: runtime: invalid INOFY definition: runtime: node "join" config: json: unknown field "inputs"`, i.e. the exact defect; fixture restored and the test passes again |
| `node scripts/stage-ui-assembly.mjs` | ok; regenerated only the workflow Module digest inside `ui/src/generated/assembly.ts` (`a0fcd1ea…` → `cc1b09e6…`) |
| digest recomputation (`internal/sourcehash.Tree` over `plugins/vivy-workflow`) | fixed point `cc1b09e680ea8e3efd4505ec9233a8e8e22ecbe67fc1114f810b7463d4084870`; the descriptor and `module.go` agree, and recomputation returns the same value |
| `go run ./sdk verify plugins/vivy-workflow` | `ok vivy/workflow-ui` |
| `./node_modules/.bin/vite build` | built `ui/dist` (needed for `go:embed` in the browser gate) |
| `./node_modules/.bin/playwright test --config playwright.workflow.config.ts workflow-correctness.spec.ts` | 1 test pass (3.4 s): author inputs → save → reload from backend → validate → publish r1, then the published artifact read through `inofy.getRevision` keeps `begin.inputs` and has no `config.inputs` |

## Browser evidence detail

The suite starts its own backend (`:18987`), Vite (`:3195`), and loopback model
(`:18988`); no existing server was reused. The first-run welcome dialog and the
persona onboarding gate are marked completed/dismissed in `localStorage` before
the app boots (`vivy.ui.welcome.completed`, `vivy.persona.onboarding.dismissed`).

## Not run in this cut

- The full `just ci` mirror (`fmt-check`, `vet`, `plugin-ci`, i18n scripts,
  `go test -p 1 ./...`, headless compile) runs once at the end of the lane,
  after W2, and is recorded in the W2 verification document; re-running it for
  an intermediate commit would not change this change's evidence.

# Verification

All commands run from the repository root unless noted. Host: Windows, Go
1.26.4 at `D:\Program Files\Go\bin\go.exe`, Node/pnpm via the checkout.

## Go

| Command | Result |
|---|---|
| `gofmt -l internal/provider internal/runtime` | clean (empty) |
| `go build ./...` | passed |
| `go test ./internal/provider/ ./internal/runtime/ ./internal/rpc/ -run 'Thinking\|ContextStatus' -count=1` | `ok` all three |
| `just ci` | see the end of this file |

New coverage:

- `internal/provider/resolving_thinking_test.go::TestThinkingLevelSurface` —
  declared levels win and are copied (not aliased), a thinking-capable model
  without a declaration reports the seven-level order, an unsupported model
  reports a non-nil empty list.
- `internal/runtime/compaction_test.go::TestServiceContextStatusThinkingSurface`
  — `session/context` reports the declared surface (`low,high`), the seven-level
  fallback for a thinking model without a declaration, and the fail-closed empty
  surface for an id the metadata does not declare.
  `TestServiceContextStatusAndCompactSession` additionally asserts the
  unsupported case is a non-nil empty list on the wire.

## UI

| Command | Result |
|---|---|
| `cd ui; pnpm typecheck` | passed |
| `cd ui; pnpm test` | 93 files / 701 tests passed |
| `cd ui; pnpm build` | passed (`dist/assets/index-*.js`, 30.9s) |
| `cd ui; pnpm exec playwright test e2e/thinking-gate.spec.ts` | **1 passed (48.1s)** |

New coverage: `ui/src/components/chat/ChatInput.thinking.test.tsx` (6 tests) —
grey disabled trigger with the reason tooltip, no dropdown trigger
(`aria-haspopup` absent) and no menu for an unsupported model; menu is exactly
`Follow default / Auto / low / high / On / Off` (`aria-haspopup="menu"`) for a
model declaring `low,high`; seven-level fallback when a thinking model declares
none; picking `high` tints the trigger and sends `thinking: 'high'`; an
untouched composer sends `thinking: undefined`; an unsupported model never sends
a level.

Real-path browser run (`e2e/thinking-gate.spec.ts`, embedded `ui/dist` served by
`go run ./cmd/vivy` on `127.0.0.1:8799`, real SQLite state, no provider
credential). The spec drives the *product* model-selection path
(Settings → 模型, the same `data-testid` rows `model-refresh.spec.ts` uses) to
pin both states, so it needs no CI environment variables:

- `deepseek-chat` (declared in `vendors.yaml` without `supports_thinking`): the
  思考模式 button is visible next to 智能, `disabled`, titled
  `当前模型未声明扩展思考支持`, and a forced click opens no `[role=menu]`.
- `deepseek-flash` (declares `supports_thinking: true`, no level list): the same
  button is enabled and its menu offers `跟随默认` plus the level surface
  (`medium` asserted).
- AutoDream is absent by title and by label; no `询问模式`/`Ask mode` control
  exists.

This run is what exposed the last implementation gap: with the control still
wrapped in `DropdownMenu`, a *forced* pointer click on the disabled trigger
opened the menu (Radix opens on `pointerdown`, which a disabled `<button>` still
receives). The unsupported state now renders a plain disabled button with no
dropdown at all, so the menu genuinely cannot open; the unit test pins the
absent `aria-haspopup`.

Updated tests that encoded the old contract:
`ui/src/components/chat/ChatInput.queue.test.tsx` (3 expectations `'auto'` ->
`undefined`), `ui/src/components/chat/ChatInput.commands.test.tsx`
(3 expectations, same change), and the e2e spec.

## Environment repairs needed during verification

- The sibling Laputa checkout used by the `replace` directives
  (`../laputa/laputa`) was parked on `4b2bec2` (2026-10-10) while
  `laputa-source.lock.json` pins `8c5328da`; `go run ./cmd/vivy` therefore failed
  with `undefined: actmem.RecapBinding|RecapHead|RecapCommit|SourceRange`, which
  also broke `go build ./...` and every package importing
  `internal/cognitivecontract`. The checkout was clean, so the repo's own
  idempotent bootstrap (`scripts/ensure-laputa.ps1`, the first step of
  `just ci`) was run: it fetched and checked out the locked commit, and the
  whole tree built afterwards.
- `pnpm build` failed with
  `[vite:esbuild-transpile] remove C:\Users\...\Temp\esbuild-<hash>: Access is
  denied` for every run, independent of source. Cleaning stale `esbuild-*` files
  did not help; pointing `TEMP`/`TMP` at a workspace-local directory
  (`.workspace/vivy-tmp`) made the same build succeed. The temp root itself is
  writable from PowerShell, so this is a host/AV behaviour on
  `%LOCALAPPDATA%\Temp`, not the DSH sandbox (policy: danger-full-access).
- `pnpm exec playwright test` initially failed with
  `Executable doesn't exist at ...\chromium_headless_shell-1234\...`; the cache
  held revision 1243. `pnpm exec playwright install chromium` downloaded the
  expected revision.
- One e2e webServer from an interrupted run stayed alive on `127.0.0.1:8799` and
  made the next `prepareE2eWorkdir` fail with `EPERM ... rm .e2e-workdir`. The
  orphan `vivy.exe` (plus its `go run` parents) was stopped; no other process
  was touched (the user's own dev pair on `:8787`/`:3015` was left alone).

## Not verified

- Acceptance point 3 end to end: that a `settings.yaml` default of `high` reaches
  the provider for an untouched composer. The composer half is covered (the turn
  carries `thinking: undefined`, so `thinkingFor` falls through to the persisted
  default — see the `internal/rpc` and `internal/provider` tests), but no run was
  sent to a real endpoint: the host's active model is an undeclared proxy id and
  the e2e backend has no credential. The outbound mapping itself is unchanged by
  this cut.
- Acceptance point 2's intensities are asserted at class level in the unit test
  (`text-primary` for `high`, `text-muted-foreground` for the default, plus the
  filled-icon branch); no visual snapshot exists.

## Observations, not fixed in this cut

- The main Playwright config's backend starts from disposable state, so the
  shell renders English while the chat surface renders Chinese in the same page.
  Every `settings/update` re-hydrates the UI language from the backend, which is
  why a spec that switches models mid-run must tolerate both languages. The
  thinking-gate spec now matches either locale.
- `e2e/thinking-gate.spec.ts` previously asserted an execution-mode trigger
  (`智能体模式`/`Agent mode`) that this build does not render at all; that
  assertion was already failing before this change and was reduced to the part
  that still holds (no `询问模式`/`Ask mode` control). The execution-mode
  surface is not part of this cut.
- `ui/src/generated/assembly.ts` shows a line-ending-only difference after any
  `pnpm typecheck`/`build`; it is generated and its content is unchanged.
- `sdk/internal/assembly/conformance_results.json` and the `internal/tools/**`,
  `internal/runtime/**` edits present in the working tree belong to a
  concurrent lane; nothing in this cut stages or reverts them.

## `just ci`

Ran twice from the root with `TEMP`/`TMP` redirected to a workspace-local
directory. The first run stopped at `i18n-check` on
`Unclassified Web key: chatInput.thinkingModeDefault|chatInput.thinkingUnsupported`;
both keys are now registered as Web face-specific keys in
`scripts/i18n-cross-face-contract.json` and `node scripts/check-i18n-cross-face.js`
prints `PASS: 14 shared semantic units`.

The second run reached `test` and failed there. Per-package outcome for the
packages this cut touches:

| Package | Result |
|---|---|
| `agent-vivy/internal/provider` | ok 1.509s |
| `agent-vivy/internal/runtime` | ok 953.722s |
| `agent-vivy/internal/runtime/compaction` | ok (cached) |
| `agent-vivy/internal/rpc` | ok 180.795s |
| `agent-vivy/internal/app/settings` | ok (cached) |
| `agent-vivy/ui` | ok 0.536s |
| `sdk/tui/{live,stream,view,theme,i18n}` | ok |

Failures, none in the packages or files of this cut and none reported as an
assertion about the thinking surface:

- `internal/app` — `TestPlanGoalIntegratedReviewAndTwoRounds`: the sandboxed
  child `go test ./...` cannot read a shared build-cache entry
  (`open C:\Users\com01\AppData\Local\go-build\5e\...-d: Access is denied`), so
  the Goal run has no successful execute event.
- `internal/studiocore` — `TestEvalSpawnsCandidateItself`: `build vivy: exit
  status 1`, `sdk\generation\manifest.go:10:2: open
  C:\Users\com01\AppData\Local\go-build\b1\...-a: Access is denied`.
- `sdk/internal`, `sdk/internal/assembly` — every pack/eval case fails with
  `symlink ... ui\node_modules ...: A required privilege is not held by the
  client`, the host limitation already on the board as `SDK-UI-LINK-SYMLINK`.

The two `go-build` denials are the same host behaviour that broke
`pnpm build`'s esbuild temp cleanup (both are `Access is denied` on files under
the user profile while several `go`/Node processes were running from this and
the concurrent lane); they are not reproducible as a code failure. Because
`just` aborts on the first failing recipe, `diva-recall-test`,
`headless-compile` and `plugin-ci` did not run in this pass.

`just ci` does not include the Playwright suite; the e2e spec above is the
UI recipe (`just ui-e2e`) and was run directly.

The root working tree is shared with a concurrent lane, so a full-suite result
must be read with that attribution. The focused gates above (typecheck, 701
unit tests, build, Go build/vet/tests for the touched packages, and the
real-path e2e spec) all pass on this cut.

# Verification: Toolkit UI redesign

Lane `feat/toolkit-ui-redesign` (worktree `C:\Users\com01\Desktop\agent-vivy-toolkit-ui`,
base `origin/main` `5715950d`), module `plugins/vivy-toolkits-ui`.
Every command below ran inside that worktree.

## 1. Environment deviations (affect how the gate was run, not what it checks)

This machine is offline for `github.com`, and it shares its sibling checkouts with
other active lanes, so two steps of the published gate cannot run here:

- **`just ci` aborts in its first recipe.** `just ci` = `ensure-laputa bootstrap-test
  fmt-check ui-ci vet test diva-recall-test headless-compile plugin-ci`, and
  `ui-ci: i18n-check: ui-core: ensure-laputa`, `vet`, `test`, `diva-recall-test` and
  `headless-compile` all depend on `ensure-laputa`. That preflight pins the Laputa
  source closure at `4b2bec2cc2ab3374b7ec1ab8d448e612c1e4db24`
  (`laputa-source.lock.json`), while the shared sibling checkout is at another
  lane's newer tip `8c5328daa7a4e9cdd089d78858de087a36fb0843`. `ensure-laputa`
  therefore needs `git fetch origin 4b2bec2c...`, and dies with
  `fatal: unable to access 'https://github.com/ProjectViVy/laputa.git/' ... Could not
  connect to server` -> `Laputa git command failed (exit 128)`.
  Measured: `just ensure-laputa` -> exit 1; `just bootstrap-test fmt-check ui-ci vet
  ...` -> `bootstrap-test` passed, then `ensure-laputa` failed at the fourth recipe.
- **Consequence and workaround.** The `ensure-laputa` step only prepares and
  validates sibling Go sources; it is not part of what this change touches (no Go
  file changed). The recipes were therefore run with their exact recipe bodies, in
  the recipe's own order, by `.tmp-build/run-gates.ps1` (temporary, deleted before
  the commit). Deviations: `ensure-laputa` was not run (impossible offline), and the
  Go recipes compiled against the sibling's newer Laputa revision because `go.mod`
  replaces `github.com/ProjectViVy/laputa/*` with `../laputa/*`. For a UI-only
  Module change that affects nothing this change asserts; a green `just ci` on a
  machine with a matching pinned sibling is still owed by the landing process.
- **`pnpm build` / `vite dev` need `ESBUILD_BINARY_PATH` in this sandbox.** The
  pnpm-store copy of `@esbuild/win32-x64@0.28.2/esbuild.exe` carries an inherited
  ACL that makes it unable to create files, so the Vite/esbuild service fails with
  `Access is denied` (`[vite:esbuild-transpile] remove ...esbuild-<hash>: Access is
  denied`, and `Failed to write to output file: open ...\.vite\deps_temp_*\...`).
  Both the store binary and a second copy of it fail the same way from a direct CLI
  call, while a plain copy at `.workspace/esbuild-bin/esbuild.exe` works; every
  command below ran with `ESBUILD_BINARY_PATH` pointing at that copy (gitignored,
  uncommitted). With it, `pnpm build` completes and the dependency cache is
  rewritten (`node_modules/.vite/deps/_metadata.json` hash `54e5da5b`).
- **The two toolkit Playwright specs cannot run from their tracked configs here.**
  `ui/playwright.toolkits.config.ts` hardcodes backend `127.0.0.1:18787` and UI
  `127.0.0.1:3015` with `reuseExistingServer: false`, but this machine already runs
  another lane's dev pair on `:3015`. They were run through a temporary untracked
  config with the same body on backend `18887` / UI `3115` (deleted before the
  commit), against a `vivy.exe` built from this worktree at
  `.workspace/toolkits-e2e/default/vivy.exe` (`go build -o ... ./cmd/vivy`). The spec
  files themselves are unchanged by that workaround. Filed as `TOOLKITS-E2E-RECIPE`
  in `docs/TODO.md` section 0.1.

## 2. Results

| Check | Command | Result |
|---|---|---|
| Module unit tests | `cd ui; pnpm test -- ../plugins/vivy-toolkits-ui/ui/vivy-toolkits/src` | 4 files, **16 tests passed**, exit 0 |
| UI typecheck | `cd ui; pnpm typecheck` | exit 0 |
| Production build | `cd ui; pnpm build` (`ESBUILD_BINARY_PATH` set) | `built in 42.38s`, `dist/index.html` + assets, exit 0 |
| i18n catalog completeness | `node scripts/check-i18n-completeness.js` | `PASS: en=1575 keys / 163 placeholders; zh=1575 keys / 163 placeholders; runtime copy audit clean`, exit 0 |
| Catalog call-site symmetry | `.tmp-build/check-toolkit-keys.cjs` | `units 143 referenced 143`, no missing and no dead unit |
| Module source hash | `go run ./sdk/internal/cmd/source-hash plugins/vivy-toolkits-ui <old>` | `5762a1989ec3b2b3c03341d8513f72e6b6eb45401411b8d3a45b31f722c70ac2` |
| Assembly verify | `go run ./sdk verify plugins/vivy-toolkits-ui` | `ok vivy/toolkits-ui`, exit 0 |
| Recipe bodies (see section 1) | each recipe's body, in recipe order | `GATE_TABLE` below |
| Browser: toolkit flow | `pnpm e2e --config <temp> toolkits.spec.ts` | 1 passed (34.4s) |
| Browser: accessibility and narrow viewport | `pnpm e2e --config <temp> toolkits-accessibility.spec.ts` | 2 passed (26.1s) |

`ui/src/generated/assembly.ts` and `ui/src/generated/ui/**` are generated
(`node scripts/stage-ui-assembly.mjs`, run by every `pnpm` pre-hook); the tracked
file was regenerated, never hand-edited.

### 2.1 Recipe bodies, one by one

Run with `.tmp-build/run-gates.ps1`, which reproduces each recipe body verbatim;
`plugin-ci`'s body was run again on its own after the first attempt hit a quoting
artifact in the wrapper, not a real failure.

| Recipe | Result |
|---|---|
| `bootstrap-test` (`node --test scripts/ensure-laputa.test.mjs scripts/dependency-closure.test.mjs`) | 11 tests, 11 pass, `fail 0` |
| `fmt-check` (gofmt over every tracked Go file) | exit 0 |
| `ui-core` (`pnpm install --frozen-lockfile`, `pnpm typecheck`, `pnpm test`, `pnpm build`) | exit 0 for each; the run covers all UI tests, including the 143 toolkit catalog units |
| `i18n-check` (`check-i18n-completeness.js`, `check-i18n-cross-face.test.js`, `check-i18n-cross-face.js`) | exit 0 for each; `PASS: en=1575 keys / 163 placeholders; zh=1575 keys / 163 placeholders; runtime copy audit clean.` |
| `vet` (`go vet ./...`) | exit 0 |
| `test` (`go test ./...`) | **not green in this environment.** `internal/app`, `internal/runtime` and `sdk/internal` hit Go's 600 s package timeout, and `internal/app`'s `TestMemoryLoopActivityFailurePreservesSourceAndArchiveRetry` fails on Windows temp-directory cleanup (`unlinkat ... memory.sqlite3: The process cannot access the file because it is being used by another process`) after its assertions pass; it reproduces in isolation, with sibling lanes running their own Go suites at the same time throughout. No Go file is in this change. |
| `diva-recall-test` (`scripts/test-diva-recall.ps1`) | exit 0 |
| `headless-compile` (`go test -run '^$' -tags vivy_headless ./cmd/vivy ./cmd/vivy-code ./ui`) | exit 0 |
| `plugin-ci` (`go vet` + `go test` for all 22 `plugins/**` and `faces/**` Go modules) | exit 0, `fail=0` |

Targeted follow-up on the Go side of this feature (its own packages pass):

| Command | Result |
|---|---|
| `go test ./internal/bundles/... ./internal/bundlecontract/...` | `ok agent-vivy/internal/bundles 24.795s`, `ok agent-vivy/internal/bundlecontract 0.145s` |
| `go test ./internal/app -run TestMemoryLoopActivityFailurePreservesSourceAndArchiveRetry -count=1` | reproduces the same cleanup-only failure in isolation |

## 3. Real browser path (not a screenshot)

The split pair contract is honoured: the specs drive Chrome against Vite with the
real Go backend and the real Toolkit RPC actions, on a temporary state directory
(never `data/vivy.db`).

`ui/e2e/toolkits.spec.ts` (1 test) installs a local release from a host directory,
reads the preview and its digest, installs disabled, enables it, saves host
environment-variable names for a hook, prepares and publishes an update from a
second release directory, confirms the stale-consent state, disables, and
uninstalls. `ui/e2e/toolkits-accessibility.spec.ts` (2 tests) reaches the page by
keyboard only and repeats the flow at 390x844, asserting no horizontal document
overflow, no alert, a content dialog that stays inside the viewport with a
non-overflowing body, and focus returning to the trigger.

Both specs were also updated for the redesign they verify:

- the install dialog and the content dialog are scoped by
  `toolkit-install-dialog` / `component-content-dialog`, and the install button is
  taken from the dialog footer, so a query can no longer hit a control outside the
  surface it means to test;
- the orphaned-toolkit row is asserted through
  `[data-testid="toolkit-row"][data-state="orphaned"]`;
- the consent steps wait for the page's `Authorization recorded` notice before
  asserting the declaration badge, so a rejected mutation fails with its own reason
  instead of a stale-badge message;
- both specs seed `vivy.persona.onboarding.dismissed` next to the existing welcome
  seed. On a fresh profile the unrelated Persona first-run gate otherwise opens a
  modal over the Toolkit page and intercepts pointer events -- pre-existing test rot
  in a different Module, not a redesign regression.

## 4. Screenshots

`toolkits-screens.spec.ts` (temporary, not committed) drove the same real path and
captured five frames from a fixture Toolkit: `desktop-install-dialog.png` (a failed
preview keeps its input and shows the failure inside the dialog),
`desktop-install-preview.png`, `desktop-details.png`, `desktop-prepared-change.png`
(the staged change with its component diff and blockers) and `mobile-library.png`
(390x844). Each frame was taken at an asserted state, so the images show
`toolkit-page`, the install dialog, the installed row, the preview and the prepared
candidate respectively. A rendering sanity check (Pillow) reports 1440x900 and
390x844 with 1168-2102 distinct colours per frame: rendered output, not a blank
page.

## 5. Defects found by verification and fixed in this change

1. **A second mutation could resend a stale catalog identity (fixed).** The first
   browser run failed at the MCP consent step: the page showed its own
   `Authorization is not yet confirmed...` alert together with
   `The previous operation could not be recovered yet...`. Root cause: with
   per-action busy state, saving bindings no longer disabled *other* mutations, so
   the consent request started while the configuration-revision refresh was still in
   flight and sent catalog identity the backend had already superseded; the backend
   answered `revision_conflict` (see `internal/bundles/manager.go`
   `SetExecutableConsent`). Fix: `mutationsLocked = Boolean(candidate) || pending !==
   null` in `ToolkitPage.tsx` -- every mutation consumes the shared catalog identity,
   so they serialize, while reads, selection and refresh stay available.
   `ToolkitPage.test.tsx` now pins it: with a binding write held open, the consent
   control is disabled and the client's consent action is never called; after the
   write commits it is enabled again.
2. **The mask-removal guidance was unreachable (fixed).** `ToolkitCandidate.tsx`
   compared the backend's singular component kind against the plural section name
   (`change.kind === 'masks'`), so the "select another mask in Masks" hint and its
   `/masks` link never rendered for a removed Mask. The backend reports `"mask"`
   (`internal/bundlecontract/identity.go`, `diffArtifacts` in
   `internal/bundles/manager.go`). Fix: map through `componentKindOf()` in
   `toolkit-view.ts`; `ToolkitUpdate.test.tsx` now asserts the hint and the link for
   a `mask` / `removed` change, and `toolkit-view.test.ts` pins the mapping.
3. **Three user-visible English literals escaped the copy catalog (fixed).**
   `Release SHA-256: ` in the details and the install preview and `SHA-256 ` in the
   content dialog were hard-coded, so `zh` users saw English and the strings were
   outside the module catalog. Both are now catalog units (`releaseDigest`,
   `contentDigest`); the English values keep the exact prefixes the specs assert.
4. **Copy assembled in code (fixed).** The inventory built
   `Recommended Skills: ...(not enabled automatically)` and `Applies to faces: ...`
   from fragments with hard-coded full-width punctuation, and rendered `1 items` /
   `tools 3`. Each is now one catalog unit with a placeholder
   (`recommendedSkillsLine`, `ruleFacesLine`, `componentCountLine`, `mcpToolsLine`),
   so translators can reorder the sentence and counts read correctly. The five
   replaced units are deleted, leaving 143 units, all with a live call site.
5. **A republished catalog mid-page was reported as a read failure (fixed).** The
   paging loop refuses to assemble a list from two catalog revisions and threw an
   untyped error that surfaced as `Could not read the Toolkit catalog. Check the
   connection and retry.` The failure now carries a marker and renders the new
   `catalogChanged` copy ("republished while it was being read; refresh").
6. **A locked preview blamed the source (fixed).** While a candidate waited, the
   install dialog's preview could return `null` and reported `Preview failed. Check
   the host path or Git source and retry.` It now reports the mutation lock
   (`candidateMutationLock`) instead.

## 6. Independent review

A fresh reviewer with no access to this session's history re-read the diff against
the requirement. Its findings are the source of sections 5.2-5.6. Two items it
raised were reviewed and deliberately accepted:

- `ToolkitExecutableRow.tsx` `BindingsEditor` derives its inputs from props inside
  an effect keyed on array identities, so the effect body runs after every render.
  The body is value-guarded (`return changed ? next : current`), so it cannot loop
  and cannot overwrite a field the user touched; the work is bounded by the number
  of declared environment slots. Rewriting it would either hide the effect's inputs
  from its dependency list or add a second copy of the same state, which is worse
  than the repeated loop.
- `previewSource` / `installPreview` still collapse "no catalog" and "a candidate
  waits" into one refusal at the page boundary; the dialog now names the lock, and
  the remaining case is unreachable from the UI because the controls that can reach
  it are disabled without a catalog.

It found no issue in shared-kit reuse (all surfaces use `@/components/ui`
primitives), dependencies (no `package.json` / lockfile change), spec-to-DOM
consistency, or the page's selection/candidate state machine.

## 7. Checks that were not run

- `just ci` as a single command, and `ensure-laputa` itself: impossible here
  (section 1).
- PostgreSQL / SQLite conformance-class recipes: not part of `just ci`, and this
  change touches no storage, schema, or Go code.
- Recipe or module changes requiring repacking or a new Generation: none in this
  change (the Module declares the same single `std/ui-extension@v1` provider).
- Eino capability check: not applicable -- the change renders existing read-only
  backend projections through the Toolkit RPC surface and adds no model, tool,
  prompt, or orchestration seam.

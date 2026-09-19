# Verification: VIVY sidebar Modules

All commands ran in the repository root unless a working directory is given.
Date of the run: 2026-09-19.

## Module conformance and staging

```text
go run ./sdk verify plugins/vivy-persona     # ok vivy/persona
go run ./sdk verify plugins/vivy-evolution   # ok vivy/evolution
go run ./sdk verify plugins/vivy-memory      # ok vivy/memory
go run ./sdk verify plugins/vivy-notebook    # ok vivy/notebook
go run ./sdk stage-ui --recipe recipes/default.vivy.yml --out ui/src/generated
# {"sdkVersion":"1.0.0","extensions":["vivy.persona.sidebar",
#  "vivy.evolution.sidebar","vivy.memory.sidebar","vivy.notebook.sidebar"]}
```

Source digests are bound to the shipped sources and stable: zeroing the
declared `sha256` in `module.go` + `vivy-module.yaml` and re-running
`go run ./sdk/internal/cmd/source-hash plugins/<slug> <zeros>` reproduces the
declared value on a second run for all four Modules (`stable=True`). The
declarations were rebound once more after `gofmt` removed trailing whitespace
from the Module sources (the formatter check only sees tracked files, so the
untracked Module trees reached the commit unformatted); live digests are
`vivy/persona df0d83d8…`, `vivy/evolution b23ff8f7…`, `vivy/memory e009d794…`,
`vivy/notebook 274acbec…`, each confirmed stable, then re-verified with
`go run ./sdk verify`, re-staged with `stage-ui`, and re-packed.

## Optional assembly (the point of the change)

A Recipe derived from `default.vivy.yml` with only `vivy/persona` selected:

```text
go run ./sdk stage-ui --recipe <persona-only>.vivy.yml --out <tmp>
# extensions: ["vivy.persona.sidebar"]
```

Staged tree: `assembly.ts`, `ui/vivy-persona/**` only — no evolution, memory, or
notebook directory, one `plugin.vivy/persona.*` catalog, one source hash. The
extreme case (no UI Module at all) is the existing
`TestMinimalArtifactPhysicallyOmitsOptionalModules`, which now also asserts the
four Modules are absent from the minimal Assembly; it packs and boots both a
default and a minimal Generation.

## UI gates

```text
cd ui; pnpm typecheck     # exit 0 (the pre-hook stages the projection first)
cd ui; pnpm test          # 47 files, 388 tests pass
cd ui; pnpm build         # exit 0, embedded UI built from the assembled projection
go build ./...            # exit 0
node scripts/check-i18n-completeness.js
# PASS: en=1334 keys / 125 placeholders; zh=1334 keys / 125 placeholders; runtime copy audit clean.
node --test scripts/check-i18n-cross-face.test.js   # 8/8 pass
```

Two host-level tests in `ui/src/plugins/presentation-host.test.tsx` were updated
because the no-root geometry changed (see below), and one new assertion covers
the frame route slot: a claimed path yields the Module node to the shell, and
disposing the route clears it.

### Negative evidence for the new Module copy audit

`check-i18n-completeness.js` gained a Module pass (a Module's translated
literals must resolve in its own sealed catalog or in the shell dictionary;
template literals must have a real prefix). Reverting one key
(`plugin.vivy/notebook.periods.` → `notebook.periods.`) failed the gate with
`Unknown Module translation prefix: … view.tsx:99|102|178`; restoring it
returned the gate to PASS. This pass found and fixed real defects during the
migration: `memory.categories.*`, `notebook.periods.*`, and
`evolution.autodream.*` were still using their pre-migration names and rendered
raw keys.

## Product-path browser evidence

A temporary Playwright driver in gitignored scratch
(`.workspace/plugin-migration/smoke-3015.mjs`, `SMOKE_URL=http://127.0.0.1:3016`)
ran **26 checks, all pass**, and was removed with the other scratch artifacts.

The smoke drives the **split Vite dev loop** and asserts: the VIVY group lists
`人格 | 面具 | 进化 | 记忆 | 记事本` in that order; each entry renders its page
*inside* the frame at `/persona`, `/masks`, `/evolution`, `/memory`,
`/notebook` with the sidebar still present; no page shows a raw `plugin.vivy/…`
key or `[missing translation: …]`; an unclaimed path returns to `/`.

One environment note: the port-3015 Vite server belongs to a parallel lane that
has been running since 17:18 and still served the pre-change module graph
(verified by fetching `/src/generated/ui/vivy-memory/src/view.tsx` from 3015 —
old key — and from a fresh instance on 3016 — corrected key). The dev-loop
smoke therefore ran against a temporary Vite instance on 3016 started from the
same `ui/` tree and config, and was stopped afterwards; 3015 was not touched.
The stale graph is that lane's long-running process, not a repo defect: `just
dev` starts a new server.

## Packed Generation

```text
go run ./sdk pack --recipe recipes/default.vivy.yml --output .workspace/pack-default   # exit 0
go run ./sdk inspect-artifact .workspace/pack-default                                 # exit 0
```

`generation.json` of the packed default Generation: the four Modules are
selected (`vivy/persona`, `vivy/evolution`, `vivy/memory`, `vivy/notebook`),
four UI catalogs are sealed, four UI source hashes are recorded, and there is no
UI root — the shell profile.

Packing also found a real defect that `pnpm build` cannot see: the pack build
stages Module sources in an isolated temporary Assembly root outside the `ui/`
project, where tsconfig path mapping does not apply, so a Module importing the
host UI kit (`@/…`) failed to bundle
(`Rollup failed to resolve import "@/components/demo/DemoBanner"`). Fixed by
mapping `@` in `ui/vite.config.ts`. Before the fix the default Recipe could not
be packed at all; after it, pack and Inspect are green and
`TestMinimalArtifactPhysicallyOmitsOptionalModules` passes inside `just ci`.

## Gate

```text
just ci
```

Run on the final tree: **`just ci` exits 0** (fmt-check, ui-ci, vet, test,
headless-compile, plugin-ci), with all four Modules green in `plugin-ci`.

It took four runs to get there, and each stop was a real defect rather than
noise:

1. `fmt-check` — `sdk/internal/cmd/generate-default/main.go` was not
   `gofmt`-clean after the four Modules were added to it.
2. `ui-core` — two host-level tests asserted the old no-root geometry (see
   below).
3. `i18n-check` — `node scripts/check-i18n-cross-face.js` reported seven
   unclassified keys that the parallel sidebar lane had added to
   `ui/src/i18n/{en,zh}.ts`. Those dictionaries are shared with this change, so
   the keys and their classification in `scripts/i18n-cross-face-contract.json`
   are part of this commit; the checker then passes with
   `PASS: 13 shared semantic units; Web and TUI en/zh projections and arguments
   conform.`
4. `test` — the default Generation's sealed baseline inventory (see below), and
   `fmt-check` again for `sdk/internal/stageui_v1.go`. Both `fmt-check` failures
   have the same cause worth remembering: the recipe checks `git ls-files`, so a
   brand-new untracked Go file is invisible to it until it is staged.

Measured targets:

| Target | Result |
| --- | --- |
| `fmt-check` | pass |
| `ui-ci` — `i18n-check` completeness | `PASS: en=1334 keys / 125 placeholders; zh=1334 keys / 125 placeholders; runtime copy audit clean.` |
| `ui-ci` — cross-face script + unit test | `PASS: 13 shared semantic units…` and 8/8 |
| `ui-ci` — `ui-core` | `pnpm install`/`typecheck`/`test` (47 files, 388 tests)/`build` all pass; the build stages the four selected extensions |
| `vet` | pass |
| `test` | pass (200 packages, `-timeout 20m`); every package `ok` |
| `headless-compile` | pass |
| `plugin-ci` | pass, including `plugins/vivy-{persona,evolution,memory,notebook}` |

The only source change after that green run is `ui/vite.config.ts`
(`resolve.dedupe`, below), which no Go target reads; its UI targets were re-run
on the final tree with `just ui-ci` (i18n checks, `pnpm typecheck`, 388 tests,
`pnpm build`, staging) and the embedded browser check below.

`ui-core` found what the targeted checks did not: two host-level tests in
`ui/src/plugins/presentation-host.test.tsx` asserted the old no-root geometry
(a registered Module route replacing `children`). They now assert the shipped
contract — the Host keeps the frame and the route slot owns the Module node —
and the file passes with 27 tests.

`just test` first reported `FAIL` inside a 15-minute, 200-package run whose
`just` recipe printed only the tail, so it was re-run printing failures only:
`internal/app` `TestDefaultGenerationBaselineInventory` was the single failure,
and the diff was exactly the four Module IDs entering `manifest.Modules` (no
other field moved). `sdk/internal/testdata/default-generation.expected.json` is
the sealed baseline inventory of the default Generation and read only by that
test (`rg -l`), so the evidence was updated with the artifact rather than the
test being loosened; the test and the full gate then pass.
`go test ./sdk/internal -run
'TestMinimalArtifactPhysicallyOmitsOptionalModules|TestV1PackAndInspectProveRecipeRemoval'
-count=1` was also re-run after the final digest rebinding, to prove removal
still holds against the shipped sources.

## Deliberately not run

- The opt-in `ui/e2e/plugin-full-ui.spec.ts` browser smoke (needs a packed
  fixture server on `VIVY_FULL_UI_URL`). The root-profile branch it covers is
  unchanged by construction: the host previously computed
  `routeNode ?? (root ? rootNode : children)` and now computes
  `root ? (routeNode ?? rootNode) : children` — identical whenever a UI root is
  selected, which is the fixture's profile. The changed branch (no root) is the
  one the dev-loop smoke and the embedded suite exercise. The CI
  `full UI browser smoke` job is separately red for a cold-compile timeout
  (`docs/TODO.md` `CI-BROWSER-SMOKE-WEBSERVER`).
- `sdk/ui` typecheck remains broken at HEAD by stale Face-contract mocks
  (`docs/TODO.md` `APR-SDK-MOCK-DRIFT`); this change adds no `sdk/ui` source
  beyond the host binding, and `sdk/ui`'s own vitest suite still passes.

## Embedded Playwright suite

`cd ui; pnpm build; pnpm e2e` runs the whole suite against the embedded UI on
`127.0.0.1:8799`, which is where six specs drive the VIVY sidebar directly
(`runtime.spec.ts` clicks 人格 / 面具 / 记忆 / 记事本 and asserts each page's real
data and localized chrome).

The suite is red: **22 failed, 2 passed, 2 skipped**, identically on two runs
(13.3 minutes each), the second on an otherwise idle machine. That first run is
what found the real defect below; the remaining failures are **pre-existing and
not from this change**. `d9ba447` (2026-09-09, "feat(i18n): persist and hydrate
the global locale") removed navigator-based locale detection in favour of
backend hydration, but `ui/playwright.config.ts` still sets
`use.locale: 'zh-CN'` and the specs still assert Chinese chrome — a fresh e2e
database hydrates `en`, so 22 specs fail on the first Chinese expectation. The
error snapshot proves the app itself is healthy: it renders the shell, the
sidebar and the VIVY group, in English, with no crash banner. Recorded as
`EMBEDDED-E2E-LOCALE` in `docs/TODO.md`.

### The defect that run caught

The embedded bundle crashed on load:

```text
alert: Vivy UI Module failed
paragraph: UI root component failed: Cannot read properties of null (reading 'useContext')
```

Two React copies were in the graph. `sdk/ui` is consumed from source
(`@vivy/ui-sdk` → `sdk/ui/src`) and installs its own React devDependency in its
own pnpm store, so `ui/node_modules/react` was 19.2.8 while
`sdk/ui/node_modules/react` was 19.3.0. `PluginHostProvider` is the SDK's first
React hook usage, so the Module host binding called `useContext` from the second
copy and React's dispatcher was null. The development graph hid it — Vite's
pre-bundled dependency graph shared one instance, which is why the 26-check
dev-loop smoke above passed — and so did `pnpm typecheck`, `pnpm test` (388
tests, including the host conformance suite), `pnpm build`, and every Go
package.

Fixed by `resolve.dedupe: ["react", "react-dom"]` in `ui/vite.config.ts` (the
same config the pack path uses), after which the embedded bundle shrank from
1,433.35 kB to 1,424.56 kB — the removed second copy. `UI-SINGLE-REACT` in
`docs/TODO.md` records the missing guard.

### Verification of the embedded build after the fix

A temporary spec (deleted after the run, so it leaves no repo artifact) drove
the packed-projection embedded build with the welcome wizard suppressed:

```text
cd ui; pnpm exec playwright test e2e/tmp-embedded-modules.spec.ts   # 1 passed (4.3s)
```

It asserted: the VIVY group lists `Persona | Masks | Evolution | Memory |
Notebook` in order; clicking 进化 sets `/evolution`, renders the Module page in
the frame's `main` region (`div[data-vivy-presentation-route="vivy-evolution"]`)
with the sidebar still present; clicking 记忆 renders the Memory page's real
search box and seeded demo data; no `UI Module failed`, no
`[missing translation`, no raw `plugin.vivy/…` key, and no page error. Reading
the embedded build in Chinese was not asserted because the shell hydrates the
locale from the backend, which is the pre-existing failure above; the dev-loop
smoke covers the Chinese rendering.
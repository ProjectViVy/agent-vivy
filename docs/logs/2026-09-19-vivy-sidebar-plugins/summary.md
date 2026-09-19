# VIVY sidebar Modules: persona, evolution, memory, notebook

## What changed

The VIVY sidebar group no longer contains four hand-wired pages. Four v1 UI
Modules now provide them, and the default Recipe selects all four:

| Module | Module ID | Entry (Recipe) | Order |
|---|---|---|---|
| `plugins/vivy-persona` | `vivy/persona` | `vivy.persona.sidebar` | 10 |
| masks (core, unchanged) | — | — | 20 |
| `plugins/vivy-evolution` | `vivy/evolution` | `vivy.evolution.sidebar` | 30 |
| `plugins/vivy-memory` | `vivy/memory` | `vivy.memory.sidebar` | 40 |
| `plugins/vivy-notebook` | `vivy/notebook` | `vivy.notebook.sidebar` | 50 |

Each Module owns its page, its grouped sidebar entry, its catalog
(`plugin.<module-id>.*`), its Go Provider on `std/ui-extension@v1`, and a bound
source digest. The shell keeps only the assembly mechanism plus the entries it
always renders: chat, toolbox, dashboard, and 面具 inside the VIVY group (which
is why the group is always present and only its assembled entries come and go).

Assembly path: `recipes/default.vivy.yml` (modules, `order`, `ui.extensions`)
→ `go run ./sdk stage-ui` → `ui/src/generated/assembly.ts` (committed) plus the
staged Module sources (generated, gitignored) → `ui/dev`, `ui/build`,
`ui/typecheck`, `ui/test`, and `sdk pack` all read that same projection.

The default Generation's sealed baseline inventory
(`sdk/internal/testdata/default-generation.expected.json`, read by
`internal/app/default_generation_test.go`) gained the four Module IDs: the
artifact really does assemble them, so the baseline evidence moves with it.

### Host changes the move required

- **A frame route slot.** A Generation without an exclusive UI root keeps the
  shell frame; `routes/_layout.$.tsx` renders the assembled Module route inside
  it (`useActivePresentationRoute`), so `/persona` shows the page *beside* the
  sidebar instead of replacing the window. A Generation that does select a root
  still renders its routes over that root, unchanged.
- **Grouped navigation.** A navigation item may declare `group`; the shell
  projects one group's items as a single ordered surface
  (`ui/src/plugins/grouped-navigation.tsx`), which is how four independent
  Modules land in the VIVY group in Recipe order with core-first tie-breaking.
- **One host binding.** `@vivy/ui-sdk` now exports `PluginHostProvider`,
  `usePluginHost`, and `usePluginTranslation`; the shell mounts the provider
  once (`ui/src/routes/__root.tsx`) and Modules translate through it with the
  core dictionary as the fallback.
- **The `@/` host-kit alias is explicit.** `ui/vite.config.ts` maps `@` in the
  bundler, not only through tsconfig paths, because a packed Generation builds
  Module sources from a temporary Assembly root outside the `ui/` project.
- **One React in the assembled UI.** The same config dedupes `react` and
  `react-dom`. `sdk/ui` installs its own React devDependency, so the SDK's first
  hook usage (`PluginHostProvider`) resolved a second copy: the development graph
  shared one instance, while the embedded/production bundle crashed with
  `Cannot read properties of null (reading 'useContext')`. Found by the embedded
  browser run, not by typecheck, tests, or build.

### Removed / moved

Deleted from the shell: the four `_layout.<page>.tsx` routes, their views
(`PersonaMemoryView`, `EvolutionView`, `MemoryDemoView`, `NotebookView`), the
`useEvolution` hook, the four i18n sections, and the dead `nav.*` keys. The
shared demo layer (`ui/src/lib/demo-api.ts`) stayed core, so its six domain
errors moved to a core `demo.evolution.errors.*` namespace instead of shipping
twice.

## Scope boundaries (explicitly not done)

- 面具 stays a core entry, as requested; it is not a Module.
- No UI Module runtime loading, no new Port, no Grant/approval prompt.
- The demo data layer and its seeded English/localized content are untouched.
- `sdk/ui` typecheck breakage is not repaired here (it is another lane's Face
  mock drift; see `verification.md`).
- No new CI job: the UI lane (`just ui-ci`) now needs Go because the projection
  is staged by the SDK; `.github/workflows/ci.yml` installs it there.

## Shared files (one root tree, two write lanes)

The sidebar host file is one file, not two: `ConversationSidebar.tsx` carries
this change's VIVY-group assembly *and* the parallel sidebar lane's session-list
header/search/view-mode work, and it imports that lane's new
`session-list-view.ts` and `WorkspaceFolderDialog.tsx`. The two lanes were
active in the same root tree, which the repository's parallel-lane rule
forbids; the user chose to land them as one change rather than ship a commit
that cannot build. This commit therefore also carries that lane's sidebar files
(`ConversationSidebar.{tsx,test.tsx}`, `session-list-view.{ts,test.ts}`,
`WorkspaceFolderDialog.tsx`, `WorkspaceSelector.tsx`, `routes/_layout.tsx`), its
seven new web i18n keys plus their cross-face classification, and the
`sdk/ui/src/module.ts` contract they need. That lane's unrelated work stays
uncommitted: `internal/workflow/`, `internal/domain/workflow_test_support.go`,
`docs/plans/provider-registry/MIGRATION.md`, the `studio` submodule pointer, and
its own `docs/logs/` directories.

## Verification summary

Gate: `just ci` (see `verification.md` for every command and result). Product
evidence: a 26-check browser smoke of the group at the split Vite dev loop, the
embedded Playwright suite, `sdk pack` + `inspect-artifact` of the default
Generation, plus a staged persona-only Recipe proving unselected Modules
physically disappear.
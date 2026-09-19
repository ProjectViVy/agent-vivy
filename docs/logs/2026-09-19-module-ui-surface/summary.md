# Module UI standards: host icon names and the host-owned page surface

Date: 2026-09-19
Follows: `docs/logs/2026-09-19-vivy-sidebar-plugins/` (commit `1418c2c`)

## What changed

The first cut of the assembled VIVY sidebar worked mechanically but looked
wrong in use, for two reasons that both came from leaving presentation decisions
to each Module:

1. **Every entry invented its own icon.** A Module passed a `lucide-react`
   component through the navigation port, so the icon's implementation, size,
   and identity lived in Module code and the SDK contract could not describe an
   icon at all.
2. **Every page invented its own frame.** A Module page rendered its own demo
   banner, its own `<h1>` block, and its own `h-full` wrapper. Two of the four
   pages had no heading at all, and because the shell's route slot was a bare
   `<div>` with no height, a page's `h-full` collapsed to its content height: the
   plugin page occupied 269–530 px of an 844 px frame while the core `/masks`
   page filled it.

This iteration makes both things the host's decision.

### 1. Icon standard: a name, not an implementation

- `sdk/ui/src/module.ts` publishes `HOST_ICON_NAMES` (a closed list of 13
  names), `HostIconName`, `isHostIconName()`, and `UINavigationItem.icon` is
  now `HostIconName` instead of a React component.
- `ui/src/plugins/host-icons.ts` maps each name to the host's own
  `lucide-react` icon and resolves unknown names to the group fallback instead
  of rendering an entry with no icon.
- `ui/src/plugins/host-icons.test.ts` fails if the SDK's list and the host's
  table drift, so a Module can never name an icon the host does not implement.
- The sidebar's own entries (dashboard, toolbox, 面具, settings) use the same
  names, so core and Module entries are one visual system. The sidebar no
  longer imports an icon component per entry.

### 2. Page standard: the host renders the frame

- `sdk/ui/src/module.ts` adds `UIRouteItem` and `defineUIRoute({ path,
  titleKey, subtitleKey, demo, render })`: a page declares its presentation
  once, next to the route it answers.
- `ModulePageSurface` (in `ui/src/plugins/presentation-host.tsx`) renders every
  claimed route, in both geometry paths (exclusive root and shell slot), as:
  the demo banner when the page is local demo data, a header carrying the
  entry's icon plus the page title and subtitle, and a content region with a
  definite full height (`min-h-0 flex-1 overflow-y-auto`).
- A Module page therefore returns **content only**. All four `page.tsx` files
  shrank to a single view import, and the four Modules dropped their
  `DemoBanner` and `usePluginTranslation` imports.
- The header icon is read from the page's own navigation contribution — one
  icon declaration per Module, used by the sidebar entry and the page header.
- `memory` and `notebook` gained the `title`/`subtitle` catalog units they never
  had; their pages previously rendered with no heading.

### 3. One SDK resolution for dev, build, test, and typecheck

While adding `defineUIRoute` the tests kept seeing an SDK without it.
`ui/node_modules/@vivy/ui-sdk` is a pnpm copy of `sdk/ui` created at install
time, and `ui/vitest.config.ts` / `ui/tsconfig.json` resolved through it while
`ui/vite.config.ts` resolved the SDK source (the app the developer actually
runs). A stale copy could therefore disagree with the running app. Tests and
typecheck now resolve `@vivy/ui-sdk` to `../sdk/ui/src` exactly like the app,
with `resolve.dedupe: ['react', 'react-dom']` so the aliased source cannot load
a second React copy.

## Measured result

Playwright audit of the embedded build at 1440×900, `main` = 844 px:

| page | slot height before | slot height after | headings before | headings after |
| --- | --- | --- | --- | --- |
| `/persona` | 345 | 844 | 1 | 1 |
| `/evolution` | 530 | 844 | 1 | 1 |
| `/memory` | 320 | 844 | 0 | 1 |
| `/notebook` | 269 | 844 | 0 | 1 |
| `/masks` (core) | 844 | 844 | 1 | 1 |

## Scope

Done: the icon vocabulary and resolution, the page surface, the four Module
pages, catalog copy for the two pages that lacked it, the test/typecheck SDK
resolution, the two Module digests, and the regenerated Assembly.

Not done: no core page was converted to the surface (core pages own their own
frame and keep it), the SDK still declares its own React devDependency (dedupe
covers dev, build, and test), Module page *content* still uses host UI kit
imports through `@/`, and `pnpm e2e`'s locale expectations remain a separate
pre-existing gap recorded in `docs/TODO.md` §0.1.
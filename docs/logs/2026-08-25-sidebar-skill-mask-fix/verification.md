# Verification

## Automation

- Repository-root `just ci`: **exit 0**, all stages passed (fmt-check / `go vet` /
  `go test ./...` all ok / headless-compile / `ui-ci`: `pnpm typecheck` +
  `vitest run` **40/40 passed** + `vite build` succeeded).
- i18n consistency: `nav.evolution` / `nav.evolutionPending` /
  `nav.evolutionUnavailable` were added to both zh/en, and the zh/en leaf-key
  consistency assertion in `ui/src/i18n/index.test.ts` passed.

## Browser run (http://127.0.0.1:3015, split Vite)

Both desktop (1280×720) and the mobile drawer (390×844) were verified:

1. The Vivy group contains `button "Evolution Planned"`: icon + copy + Planned
   Badge; it is a `<button>`, not a `<Link>` (the DOM snapshot has no `/url`).
2. Clicking “Evolution Planned” shows “Evolution is not implemented yet”; the URL
   remains on the current page (`/skills`) and does not navigate.
3. Highlight check under `/skills`: only the Skill `<a>` has the `active` class;
   the Evolution button has no active style; all other navigation items are also
   inactive. The double highlight is gone.
4. Click each other navigation item (Chat / Dashboard / Cron Tasks / Persona /
   Masks / Memory / Notebook / MCP) and use `dom_cua.get_visible_dom()` each time
   to verify that the Skill item’s bounding rect has no overlapping element.

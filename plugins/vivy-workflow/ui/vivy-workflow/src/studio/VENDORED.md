# Vendored INOFY definition/model layer

Contents of this directory are vendored from the pinned INOFY engine
repository, `github.com/ProjectViVy/inofy` at commit `4def2ae6185f`
(the same commit `go.mod` pins for the Go engine).

Copied verbatim, byte-identical from `studio/src/`:

- `schema.ts` — `inofy.workflow/v1` wire types (artifact/graph/binding/run).
- `transport.ts` — `TransportError` + `EventPage`/`EventSubscription` seam
  types (the editor-side `StudioTransport` interface is vendored for its
  types only; the VIVY client implements the calls directly).
- `edit.ts` — artifact mutation helpers (add/patch/remove node, exits,
  output binding, id allocation).
- `graph.ts` — artifact ↔ React Flow canvas projection with layout
  write-back semantics.
- `edit.test.ts`, `graph.test.ts` — upstream unit tests for the two.

Deliberately not vendored (VIVY owns its own UI — the upstream studio
components are rejected by product direction):

- `App.tsx`, `Editor.tsx`, `pages/*`, `components/*`, `styles.css`,
  `i18n.ts`, `labels.ts`, `router.ts`, `seed.ts` — the INOFY-native editor
  UI. The VIVY module implements its own React components in `../`
  (`WorkflowPage`, `editor/*`, `panes/*`) using the host UI kit and the
  module i18n catalog.
- `main.tsx`, `app-transport.ts` — standalone shell entry and INOFY App
  HTTP transport; the module uses the `inofy.*` RPC bridge in
  `../face-bridge.ts`.
- `vivy-transport.ts`, `flows.test.tsx`, `Editor.test.tsx` — upstream's
  editor transport adapter and RTL tests; the VIVY typed client lives in
  `../client.ts` and page coverage in `../workflow-page.test.tsx`.

To re-vendor a newer INOFY pin, recopy `schema.ts`, `transport.ts`,
`edit.ts`, `graph.ts` and their tests from `<inofy>/studio/src`.

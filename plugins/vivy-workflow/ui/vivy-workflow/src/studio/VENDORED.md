# Vendored INOFY Studio editor

Contents of this directory are vendored from the pinned INOFY engine
repository, `github.com/ProjectViVy/inofy` at commit `4def2ae6185f`
(the same commit `go.mod` pins for the Go engine).

Copied verbatim, byte-identical:

- `schema.ts`, `transport.ts`, `graph.ts`, `graph.test.ts`, `edit.ts`,
  `edit.test.ts`, `labels.ts`, `i18n.ts`, `seed.ts`, `router.ts`,
  `styles.css`, `Editor.tsx`, `App.tsx`
- `components/{Ui,InofyNode,NodeProperties,Panes,ConnsPane,Shell,Login}.tsx`
- `pages/{WorkflowsPage,EditorPage,RunsPage,SettingsPage}.tsx`

Deliberately not vendored:

- `main.tsx` (the standalone Studio shell entry — VIVY mounts `App` inside a
  UI Module page instead)
- `app-transport.ts` (the INOFY App HTTP transport — the VIVY module uses the
  `inofy.*` RPC bridge in `../face-bridge.ts`)
- `flows.test.tsx`, `Editor.test.tsx` (they depend on
  `@testing-library/react`/`jest-dom`, which the VIVY UI toolchain does not
  use; equivalent coverage lives in `../face-bridge.test.ts` and
  `../workflow-page.test.tsx`)

Narrow host adaptation (permitted by plan 07-ui.md for the bridge seam):

- `vivy-transport.ts`: the `call` helper maps the RPC `error.data.code` onto
  the App §11.4 HTTP statuses the editor branch-checks (`not_found` and
  `revision_conflict` → 412, validation codes → 422,
  `unsupported_feature` → 501, `unavailable` → 503, `idempotency_conflict`
  → 409, `unauthenticated` → 401). Without it every RPC error surfaced with
  status 0 and the "no draft yet" / CAS-conflict editor paths could not
  trigger.

To re-vendor a newer INOFY pin, recopy the same file set from
`<inofy>/studio/src` and reapply the `statusFor` mapping in
`vivy-transport.ts`.

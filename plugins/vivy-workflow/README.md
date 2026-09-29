# vivy/workflow-ui

VIVY UI Module hosting the INOFY workflow editor (the vendored `studio`
package pinned to the `github.com/ProjectViVy/inofy` engine commit).

- Provides only `std/ui-extension@v1` (`vivy.workflow-ui.sidebar`): the
  `/workflows` route plus its VIVY-group sidebar entry and
  `plugin.vivy/workflow-ui.*` en/zh catalog.
- All editor traffic goes through the host `inofy.*` RPC surface bound to the
  active session (`face-bridge.ts`); authorization, budget, journal, and
  storage authority stay in the VIVY backend. Editor state is data, never
  policy or tool authority.
- `src/studio/` is a byte-verbatim vendor of `INOFY/studio/src` at the Go pin
  (`6acfcc6b1a51`), except the narrow status mapping documented in
  `src/studio/VENDORED.md`.
- Known gap: editor chrome inside the vendored package is upstream zh-only;
  the Module's own route/nav labels are localized through the catalog.

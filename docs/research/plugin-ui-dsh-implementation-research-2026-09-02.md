# Deep dive into DSH plugin UI implementation (2026-09-02)

> Trigger: maintainer request—"Future plugins need to include UI; look at how DSH implements it."
> Upstream document: `docs/research/plugin-model-dsh-vs-vivy-research-2026-09-02.md` (port standard proposal).
> This document answers two questions: **the complete mechanism of DSH plugin UI**; **form options for porting it to the Vivy port model**.

---

## 1. One-sentence conclusion

DSH's web client **is itself a Cordis plugin host inside the browser**. A plugin package has two halves:
`.` (the Node host half: services/tools) and `./client` (the browser half: React UI), declared by the
`dsh.client` block in package.json. The host Node process scans declarations, assembles a boot graph injected into
`window.__DSH_BOOT__`, and serves prebuilt bundles through a custom combo route; the browser side implements a
"lazy CJS table" to materialize plugins, and its `apply(ctx)` registers React components in a typed slot tree where
**declaration means ownership**. No module federation, no import map, no iframe—full trust and one React tree.

---

## 2. Physical delivery chain (how the bundle reaches the browser)

Source: `<D>/docs/subsystems/client-modules.md`, `<D>/packages/client/modules/README.md`.

1. **Declaration**: add `dsh.client { platform: "web", inject?, immediately?,
   external? }` to package.json and attach the build artifact at `exports["./client"]`. Example:
   `packages/extensions/ui-cordis/package.json:16-47`.
2. **Host half** `ctx.clientModules` (ClientModuleRegistry) combines four responsibilities: scan host
   loader entries for packages declaring `dsh.client`; assemble the boot graph; serve versioned combo scripts; answer boot protocol requests.

   ```ts
   interface WebBootEntry {
     id: string            // == package name
     url: string           // versioned combo endpoint (for HMR)
     rev: string           // opaque revision, cache-busting
     inject?: string[]     // package-name dependency edge
     immediately?: boolean // first-level prefetch marker
     external?: string[]   // non-baseline module request
   }
   ```

3. **Bundle routing**: `GET /plugins/??<a>/client.js,<b>/client.js&rev=<rev>` (3KiB URL
   prefix partitioning, immutable cache, Indexed SourceMap v3). Unknown resources or expired revs always return 404;
   **never** let SPA fallback emit HTML as JS (client-modules.md:85).
4. **Injection**: the host takes over index rendering and injects a module-load-queue facade, advisory
   preload, blocking boot combo, and boot graph into `<head>` (`<` is escaped so plugin strings cannot escape the script tag).
5. **Browser half** `ctx.modules`: a **lazy CJS table**—executing the bundle only registers factories; all module-body side effects
   (including CSS injection) live in the factory closure and run only when materialized (`factory(require)`), with the result memoized in
   `loadCache`. Externals resolve to **frozen platform seeds** (`packages/client/web/src/platform.ts:8-13`):
   `react / react-dom / @deepseek-ai/cordis / dsh-client-store / dsh-client-ui-slots /
   dsh-client-ui-primitives`—one instance for the entire browser; plugins may not ship copies.
6. **Boot**: `dsh-client-web` is a framework-free boot core (it draws its own loading/failure pages so diagnostics remain available when
   the React tree crashes). After all fibers are ACTIVE, it hydrates through `ctx.uiRenderer` and calls
   `renderSlot('root')` exactly once to draw the whole tree. HMR: poll bundle stat → broadcast rev over SSE.

## 3. Slot system (the UI contract layer)

Source: `<D>/docs/subsystems/slots.md`, `packages/client/ui-slots/README.md`.

- **Type registration**: `SlotMap` is populated at compile time through TS declaration merging:

  ```ts
  declare module '@deepseek-ai/dsh-client-ui-slots' {
    interface SlotMap {
      'tool.view.cordis': { kind: 'keyed'; scope: 'session';
                            owner: CordisToolViewOwnerProps }
    }
  }
  ```

- **Declaration means ownership**: the entry that registers a declared slot becomes the sole renderer for that key; registering
  to an undeclared slot **throws at load time**. `root` is the only built-in declaration; components that own render positions declare all other child slots.
- **Cardinality × scope**: `single` (priority winner) / `list` (by order + registration order) / `keyed`
  (owner dispatches entryKey and the matching cell renders) / `chain` (each entry is a pure `select(owner)` function,
  the first non-null wins and receives matched); scopes `root / session-maybe / session`.
  `priority` is the masking order; dynamic packages automatically receive the "within-page masking rank, later registration comes first" order.
- **Component inputs**: the shared intersection of four sources (runtime hooks / authorized child-slot renderers / store
  selector+actions / the return value of the inject factory at registration) + locale `t`. **Components never receive ctx**—
  the capability surface is cleanly separated, and UI components can see only what is explicitly injected.
- **Shipped slot tree** (slots.md:110-163): `root → sidebar (brand/footer.action/
  workspaces/settings.*) → conversation (session/view/chat.node/tool.call.toolview →
  tool.view.cordis/composer/**/input overlays) → details → shell.overlay`.
  The `gen-client-catalog` generator produces the machine contract, and the live tree can be inspected at runtime with `cordis_inspect what:"client"`.
- **Real example** (`packages/extensions/ui-cordis/src/client/index.ts`): one package contributes
  a global panel (`sidebar.footer.action` list slot), four keyed tool cards
  (`cordis_define/run/stop/undefine` mounted at `tool.call.toolview`), a child slot for third parties
  `tool.view.cordis`, and an `@pluginId` completion source for the input box.

## 4. One package, two halves (host/client split)

- Entry points: `.` → Node half (services/tools), `./client` → browser half (slot registration + components).
  Each declares externals independently: the Node side externalizes production dependencies, while the browser side externalizes the platform baseline plus
  exact additions in `dsh.client.external` (no alias protocol).
- Mounting: **each of the two processes runs its own Cordis loader**. Dynamic and static packages on the browser side use the same
  activation gates, fiber-effect cleanup, and state projection.
- **The bridge between the halves is Remote**: the client calls generated `ctx.remote.<service>.<method>(...)`, which loops back to
  the host controller. Discipline: "cross-package behavior uses injected Cordis services; cross-package UI uses slots"—feature plugins
  never runtime-import values from another feature plugin (`packages/client/AGENTS.md:36`).
- Constraints: React 18, CSS Modules + clsx, no component library/Tailwind; global styles are allowed only for ui-theme and
  must install style tags through `ctx.effect()` (automatically reclaimed on uninstall/HMR); semantic tokens `--dsw-alias-*`.

## 5. Dynamic plugins (the model writes its own UI)

Source: `packages/extensions/{tool-cordis,cordis-client-runner,ui-cordis}`.

- Flow: `cordis_define` (validation + syntax preflight only; do not run and do not require approval) → `cordis_run`;
  a package with a browser half returns `awaiting-approval` and executes in the browser only after **manual approval** via the full-frame
  `cordis/request-run` round trip (optional override of a future version; first response wins).
- Code form: **plain JS, no JSX/TS/import**, evaluated as an async function **inside the page** (not an iframe).
  The parameter allowlist closure is `['React','console','styles','host','harness',...traps,'process','Buffer']`
  (`evaluator.ts:173`)—browser globals such as `fetch`/`setTimeout` are unreachable;
  the stylesheet from `styles.insert(css)` is automatically removed when the package is uninstalled; `host.call(method,args)` reaches only the package's
  own host half.
- Guard: an allowlisted ctx facade—exposes only lifecycle verbs and declared services and rejects returned Context values;
  slots are automatically assigned masking ranks and accounted for; the theme override source is pinned to the package ID. Rendering crashes report to the host:
  slot name, whether it abdicated, and an author-visible message; the model learns this through a steering message or
  `cordis_inspect_self`. guard.ts:12-13 states: "**This is API discipline, not a security boundary.**"

## 6. Trust and isolation

- Full trust: no iframe sandbox; everything renders in **the same React tree**.
- The isolation that actually exists is engineering isolation: lifecycle isolation (all contributions, including style tags, unload with the fiber);
  rendering shell (error boundary or abdication retirement, with crashes attributed to the package by identity); CSS-convention isolation
  (CSS Modules hashed classes + token discipline, feature packages prohibited from global styles); transport hardening (boot-graph escaping,
  combo/rev 404 semantics). Known gaps acknowledged by the system: slot admission has no carrier (there is nowhere to attach a per-deployment allow/deny list);
  the guard allowlist and host sandbox facade are duplicated manual mirrors.

## 7. theme / layout are plugins too

- `ctx.theme`: the light/dark, font-size, and `--dsw-*` token table belongs to ui-theme; third-party themes register **alias-token overrides**
  through `ctx.theme`, folded into the activation snapshot in registration order; the host embeds the resolved theme in the index response, so the first render is themed.
- `ctx.layout`: the three-column AppFrame **is itself a plugin**—one `register()` registers AppFrame in the `root` slot, declares four child slots in the same operation (sidebar/conversation/details/shell.overlay),
  joins the layout store, and exposes panel-action services through `ctx.layout`. Geometry is transient (reset on refresh).

---

## 8. Porting to Vivy: form options (pending decision, not scheduled)

Vivy reality: plugins are Go source (`vivy-sdk pack`, compile-time generations), and the UI is an independent React application
(`ui/`, Vite, RPC/WS same-origin contract). DSH's "plugin-shipped client bundle" cannot be copied verbatim because Vivy plugin authors currently write only Go. Three options:

- **Option A: declarative UI port (recommended starting point)**. Port family `ui/slot`: Go plugins declare
  slot intent (slot name + keyed/list + data projection + action callback), while **rendering primitives are built into the host UI**
  (a limited set such as cards/forms/panels/lists). Zero JS and zero supply-chain burden for plugins; the UI side needs only a generic
  PluginSurface renderer + RPC projection endpoint. Expressiveness is approximately DSH keyed tool-card level,
  covering most needs for "giving a plugin a settings page/result card/panel." Audit-friendly: UI intent enters generation.json.
- **Option B: plugin JS half (full DSH alignment)**. Add a prebuilt `client/` bundle to the plugin repository and collect it in pack;
  the gateway adopts DSH: boot graph + `/plugins` combo route + lazy CJS table + platform seeds
  (the host supplies the React singleton). Expressiveness = DSH; cost: plugin authors must maintain a JS bundling chain,
  bundle hashes must enter generation.json, and this introduces a new trust surface of "plugin UI code"
  (DSH itself acknowledges that it is full trust + API discipline).
- **Option C: hybrid, phased**. Establish A as the v1 standard first; defer B as the advanced `ui/canvas` port,
  approving it only if A's expressiveness is genuinely insufficient.

Regardless of the option, the parts of DSH worth copying verbatim (independent of host language) are:

1. **Slot contract**: declaration means ownership (loading an undeclared slot is rejected), four cardinalities single/list/keyed/chain,
   priority masking order, and components do not receive ctx (the capability surface is explicitly injected).
2. **Lifecycle is UI**: every UI contribution (including styles) is attached to the plugin lifecycle and disappears on uninstall.
3. **Attribute rendering crashes**: error boundary + attribute the crash to the plugin + abdicate retirement.
4. **Platform seed singleton**: React/component libraries are supplied as a single host instance; plugins do not ship copies.
5. **Transport discipline**: escape the boot graph and return 404 rather than HTML for unknown bundles.

Governance red lines (Vivy-specific, non-negotiable): plugin UI receives only **projected data**; all action callbacks go through RPC and pass
approval/policy; plugins never connect directly to Journal; UI ports belong to web-face (FACE-0) assembly and do not enter the physical kernel.

---

## 9. Conclusion

DSH plugin UI = **a second plugin host on the browser side + a declaration-means-ownership slot tree + a lazy CJS bundle
pipeline**, with full trust and no iframe; engineering isolation relies on lifecycle/error boundaries/CSS conventions. The implication for Vivy:
the slot contract and lifecycle model can be ported as a whole; whether to port the bundle pipeline depends on whether plugin authors should write
JS—v1 should use a declarative UI port (Options A/C), include `ui/slot` in the port standard,
and reserve `ui/canvas` (JS half) as a later advanced port. This has been added to the PLG-1 decision scope.

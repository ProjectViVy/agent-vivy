# Pluggable frontend research: DSH comparison and Vivy implementation approach

Date: 2026-08-30. Nature: research document (not an implementation commitment).

## 0. Problem definition

> How should we implement a pluggable frontend design like DSH (for example, if I do not install a plugin, that plugin should not display any UI)?

Break this into four subquestions:

1. For the question "is a given plugin installed," where is the state stored and who is authoritative?
2. In what form does a plugin's UI exist—data or code?
3. How does the frontend know "which plugins exist and what UI each contributes"?
4. How does the UI disappear after uninstall or generation change?

**One-sentence conclusion**: In Vivy, "installation" is already a compile-time event (`vivy-sdk pack` statically links plugins into a new-generation EXE), so "no installation means no UI" does not require inventing a new install/uninstall mechanism. Only two things are missing: the kernel must report "the plugins compiled into this generation and their declared UI descriptions" through RPC; the Web UI must change its currently hard-coded navigation/settings pages/tool cards into slots rendered "from the kernel report." DSH's value is that it demonstrates this end to end, provides three reusable layers of mechanisms, and offers one trust-boundary lesson.

## 1. How DSH does it

Source: `.workspace/deepseek-harness/deepseek-harness/` (local working clone, this research's ground truth).

### 1.1 Overall form

- TypeScript/Node monorepo, **everything is a plugin** (vendored Cordis framework): the agent loop, tool registry, session log, and model adapter are all plugins. `docs/architecture.md` says: "Every part of the product is a plugin … so each is replaceable from configuration."
- `dsh` starts a **profile** = ordered bundle patch (`cordis.patch.yml` fields: `id/name/config/disabled`) + user overlay; the `dsh plugin` command is pnpm forwarding plus compositional coordination of `dsh.profile.bundles` (`apps/cli/src/plugin.ts`).
- **The frontend is a second Cordis context in the browser** (`packages/client/web/src/boot.ts`). The four frontends—web / headless / sdk (stdio JSON-RPC) / acp—are selected **purely by composition** over the same engine kernel.

### 1.2 Mechanism A: client module system — mounting determines delivery

- The server (`packages/client/modules/src/index.ts`) scans Loader entries for packages declaring `dsh.client`, builds the `window.__DSH_BOOT__` entry graph, and **serves browser bundles only for mounted plugins**.
- The browser side (`system.ts`) loads according to the graph and registers factories through `window.__ModuleLoader__.load({id, factory})`.
- Effect (in the cookbook's words): "the plugin appears on the page as soon as a `cordis.yml` mounts it — **no rebuild of the web application**." Not mounted → bundle is never delivered → `apply()` does not run → **the UI never exists**, rather than being hidden.

### 1.3 Mechanism B: slot registry — reversible in-page registration

- A plugin's `ctx.slots.register({name, key, order, …}, Component)` registers a React component in a typed slot (`single | list | keyed | chain`, `packages/client/ui-slots`, `ui-renderer/registry.ts`).
- Registration itself is a `ctx.effect` (a reversible Cordis effect)—**uninstalling the plugin automatically removes the component**, with no hand-written cleanup.
- Framework slots: `root / sidebar / conversation / details / shell.overlay / settings tabs / tool.call.<name>`; an unclaimed key falls back to the generic renderer, while a claimed key replaces it.

### 1.4 Mechanism C: data-only descriptors + generic rendering

- Slash commands: the host registers `ctx.commands`, and the composer fetches the directory per session through `command.list` (`ui-commands/directory.ts`) → the menu contains only commands from mounted plugins.
- Tool cards: a tool declares a **presenter intent** (`card: 'terminal' | 'diff' | 'read' | …`) plus persisted `presentationMeta`; the UI derives the card from raw run events + intent; a key without a dedicated card falls back to a generic row.
- This tier puts **zero plugin code into the browser** while covering the highest-frequency "plugin visibility" scenario.

### 1.5 Trust model (lesson)

- There is **no in-page sandbox**: plugin UI receives full-page permissions; isolation consists only of a build-time bundle purity gate (prohibiting cross-plugin value imports) + a network trust fence (preventing DNS rebinding, explicitly "not an auth layer").
- Dynamic packages written by the agent at runtime: fixed globals (React/console/styles/host) + manual approval each time, and the documentation acknowledges that this is "not a security boundary; treat it like bash."
- Conclusion: DSH trades flexibility for "trusted installation sources + build-time discipline." This tradeoff works for developer tools but must be reconsidered for a tenant-facing daily product (Vivy's `vivy.exe`)—`docs/architecture/VIVY-STUDIO.md` §9.1 already says no to "arbitrary CSS in settings" ("the trust model is wrong").

### 1.6 Inventory page

- `packages/host/plugin-inventory`: `PluginInventoryGateway.list()` reads `ctx.loader.entries()` and returns `{entryId, moduleName, enabled, fiberPhase}` for each entry, rendered by a read-only settings page. "What did I install?" is itself a first-class UI.

## 2. Vivy current state

### 2.1 Plugin lifecycle is compile-time

- Manifest `vivy.plugin/v0`: `apiVersion/name/version/seam(tool|tool-world|provider)/module/grants(fs.read|fs.write)/tools[]` (`sdk/internal/manifest.go:21-36`). **There are no UI fields.**
- `vivy-sdk pack`: verify → generate registration file → `go build -overlay` overlays `internal/generated/plugins/zz_register.go` → statically link a new generation at `dist/<gen_id>/{vivy.exe, generation.json}`. generation.json freezes `Recipe{Loop, World, Plugins[]}` and `Tools[]` (`sdk/internal/pack.go`).
- **Uninstall = remove one line from the recipe and pack again** (`docs/architecture/VIVY-PLUGIN-SPEC.md` §7/§8: "plugin crash = this generation's EXE crash. Isolation is not in the process, but in the next generation"). There is no runtime enable/disable, plugin registry, or hot loading.

### 2.2 Kernel runtime exposure of plugins (the gap)

- `species/inspect` (`internal/studio/inspect.go`) reports `generation_id`, `recipe.plugins` (names only), tools, and grants. This is the only method that dynamically reflects "what is installed in this generation."
- `initialize`/`capabilities` (`internal/rpc/control.go:345-360`) is a **hard-coded static string table**, unrelated to plugins.
- The UI stores `capabilities` in Zustand (`ui/src/lib/store.ts:33,211`) but **does not consume it at all**—the channel exists but spins idle.
- There is no `plugins/list` and no UI descriptor.

### 2.3 The Web UI is a closed single bundle

- One bundle via `go:embed ui/dist` (`ui/embed.go`); navigation uses three hard-coded arrays (`NAV_ITEMS/VIVY_ITEMS/TOOL_ITEMS` in `ui/src/components/chat/ConversationSidebar.tsx:7-9`); the settings-page tab set is static; the i18n dual dictionaries are closed (zh authoritative, en structurally mirrored, consistency enforced by tests).
- Existing examples of "backend report → conditional rendering": the `settings.read_only` gate for save buttons, `settings/providers`, `settings/mcp`, and the "backend list → render cards" pattern in `skills/list`. This is the prototype of option one.

### 2.4 DSH mechanisms already run in Studio (an existing model outside the kernel)

- `studio/dsh-vivy-studio/index.js`: server plugin `webServer.tapIndex` injects `<style data-plugin>`/`<script data-plugin>` into index.html; reads files at startup, takes effect after restart, and has no hot loading.
- `studio/dsh-vivy-console`: client side `window.__ModuleLoader__.load({id, factory})` + `dsh.client.inject`, registering a `conversation.view` tab.
- `studio/dsh-better-sidebar`: `ctx.slots.inject('conversation.chat.turnTail', () => ctx.slots.register(…))`.
- In other words, DSH mechanisms A/B already run in the Studio shell. What is missing is the counterpart in Vivy's **own web UI (tenant product)**—exactly where the "pluggable frontend" should land.

### 2.5 Related proposal: `seam: face` in VIVY-FACE-PACK

- `docs/architecture/VIVY-FACE-PACK.md` (proposal, not implemented): Face = a compilable organ in the recipe, one primary face per generation; `RegisterFace()` pack overlay; "a live EXE does not dynamically load any face code."
- Granularity distinction: a face is **the entire shell** (web|tui|headless), while the plugin UI contribution discussed here is **panels/cards/commands/tabs inside the shell**. They are orthogonal: the face changes the shell, and plugin UI fills the shell. The design must not conflict with FACE-PACK—plugin UI descriptors are meaningful only in the `faces/web` face.

## 3. Options: three-tier trust spectrum

### Option one (recommended first): UI as data — descriptor channel

Principle: **plugins never send code to the browser, only data**; the renderer is a generic component built once in the kernel (tier C).

- **Manifest**: `vivy.plugin/v1` adds an optional `ui` block with a very small allowlist and inline i18n (aligned with the zh-authoritative/en-twin convention):

  ```json
  "ui": {
    "nav": [{ "path": "/hello-fs", "title": { "zh": "File Probe", "en": "File Probe" }, "icon": "folder" }],
    "settings": { "schema": { "type": "object", "properties": { "root": { "type": "string" } } } },
    "toolCards": [{ "tool": "hello_stat", "card": "kv" }]
  }
  ```

- **Pack**: the new mechanism follows the existing pattern—while generating the `zz_register.go` overlay file, pack also generates `zz_ui.go` (`func UI() map[string]json.RawMessage`) and freezes each plugin manifest's `ui` block verbatim into the EXE. **The manifest remains the single source of truth, the SDK's Go API window does not change, and plugin authors have zero extra burden**; runtime also does not depend on whether `generation.json`/`install.json` is present beside it.
- **Kernel**: add read-only RPC `plugins/list` (additive; do not change `initialize` handshake semantics, which would affect all clients such as TUI/worker), with data source = `genplugins.Register()` + `genplugins.UI()`. Add no ledger tables (NG-28 safety: this is not a product-semantics expansion of generation/eval/promotion).
- **UI**: on startup, the store fetches `plugins/list` → renders a "Plugins" sidebar group (next to `TOOL_ITEMS`), plugin cards on the settings page (schema → form, reusing the MCP/providers card pattern), and tool cards from the `toolCards` table. **Not installed → empty list → nothing is rendered.**
- **Result**: strictly satisfies "not installed means not displayed"; zero browser code execution; plugin authors still touch only `plugins/<name>/` (tier A); one-time kernel change (tier C, no further growth with each plugin).
- **Limitation**: UI expressiveness = the renderer vocabulary.

### Option two: schema-driven rich components + presenter intents

Extend option one with a widget vocabulary (forms/tables/KV/diff/badges/links), and adopt the DSH tool-card idea: tools declare presenter intent, and the UI derives cards from run events + intent; **undeclared keys fall back to a generic row**. Still zero plugin code enters the browser. Vocabulary fields need parse/validate tests (aligned with the convention that "new configuration fields need tests").

### Option three: plugins ship frontend code (Vivy version of DSH mechanisms A/B) — proceed slowly, or limit to first-party

- Pack additionally collects prebuilt assets from the plugin's `ui/` (ESM/lazy CJS bundle + CSS), served by the kernel with the generation; Vivy UI adds a module loader + slot registry.
- Use **generations** instead of DSH hot-uninstall for disappearance semantics: a generation change (reinstall/restart) replaces the UI, with no need for Cordis reversible effects—simpler and consistent with the established position that "a live EXE does not dynamically load code."
- Prerequisite decisions (discuss after options one/two): sandbox choice (iframe/Web Components vs. DSH-style unsandboxed + build-time purity gate), CSP and asset integrity, and whether the §9.1 first-party sealed skin and tenant plugin UI should coexist as two trust levels long term.
- Highest cost and heaviest governance. If approved, expose it only to first-party plugins or make it a separate track after FACE-PACK.

### Three-tier comparison

| | Option one descriptor | Option two schema cards | Option three bundled code |
|---|---|---|---|
| Browser executes plugin code | None | None | Yes |
| Expressiveness | Low (vocabulary) | Medium (expanded vocabulary) | High (arbitrary React) |
| Kernel/UI changes | Small (one-time) | Medium (vocabulary evolution) | Large (asset serving + loader + slots) |
| Trust model | No addition | No addition | New decision required (§9.1 level) |
| DSH counterpart | Mechanism C | Mechanism C+ | Mechanisms A+B |
| Fit with existing governance | Fully compliant | Fully compliant | Requires exemption/new rules |

## 4. The complete Vivy chain for "not installed means not displayed" (option-one view)

```
pack without the plugin → it does not enter zz_register.go/zz_ui.go → it is absent from this generation's EXE
→ plugins/list reports nothing → the UI slot has no data → nothing renders
```

No step requires a "delete UI" action—**UI existence is determined entirely by generation contents**. This is the compile-time version of DSH mechanism A (mounting determines delivery), and is even more complete: the browser bundle never exists at all.

Two small supporting items:

- **Generation awareness**: the UI currently fetches state only once at startup. Include `generation_id` in `plugins/list` (or `species/inspect`); at startup, the UI compares it with the local record and prompts for refresh on mismatch. Do not hot-switch—consistent with the repository-wide established position of "no hot loading."
- **Plugin inventory page**: the counterpart to DSH's read-only PluginInventory. Vivy already has all the data from `species/inspect`; rendering a read-only "plugins in this generation" page naturally belongs in the first delivery of this design.

## 5. Open questions

1. Manifest version strategy: compatibility between `vivy.plugin/v1` and v0 (dual-read in verify? Can v0 plugins continue to be packed?).
2. `plugins/list` as an independent method vs. making capabilities dynamic: additive is recommended first; dynamic capabilities would change the handshake semantics for all clients (TUI/worker).
3. Data-source boundary allowed by the settings-card schema: may a plugin query only its own tools, or may it reference general stats? This determines whether the widget vocabulary needs authorization semantics.
4. If option three is approved: sandbox choice, CSP, and the relationship between the two trust levels for tenant web UI and the Studio shell.
5. Should TUI (`internal/tui`) consume the same descriptors (text slots) to fulfill "multiple frontends, one plugin"?

## 6. Reference paths

DSH: `packages/client/modules/` (mechanism A), `packages/client/ui-slots` + `ui-renderer` (mechanism B), `packages/interaction/commands` + `ui-commands` (command descriptors), `packages/client/ui-tool` (presenter intents), `packages/host/plugin-inventory` (inventory page), `docs/architecture.md`, `docs/cookbook/adding-a-settings-card.md`.

Vivy：`sdk/internal/manifest.go`、`sdk/internal/pack.go`、`internal/generated/plugins/zz_register.go`、`internal/pluginhost/host.go`、`internal/rpc/control.go:345-360`、`internal/studio/inspect.go`、`ui/src/components/chat/ConversationSidebar.tsx:7-9`、`ui/src/lib/store.ts:211`、`studio/dsh-vivy-studio/index.js`、`studio/dsh-vivy-console/`、`docs/architecture/VIVY-PLUGIN-SPEC.md`、`docs/architecture/VIVY-FACE-PACK.md`、`docs/architecture/VIVY-STUDIO.md` §9.1。

## 7. Research path

### 7.1 This research process (reproducible)

```
AGENTS.md entry point (DSH ground-truth location + governance constraints)
→ DSH source issue list (.workspace/deepseek-harness/deepseek-harness/)
→ Vivy current-state issue list (sdk/ internal/ ui/ studio/ docs/architecture/)
→ cross-validation across both lines (dsh-* plugins in Studio = live samples of DSH mechanisms)
→ manual spot-check of key references (grep/sed review of 5 locations)
→ synthesis of the three-tier options
```

**Entry point and basis**: `AGENTS.md` designates `.workspace/deepseek-harness/` (`deepseek-harness/` working clone + `upstream/` mirror) as the source of truth for DSH behavior and requires using that tree rather than build artifacts in `node_modules`. The research confirmed that the working clone is complete and that `upstream/` was not used. On the Vivy side, the repository itself and product contracts in `docs/architecture/` are authoritative. The exploration was completed through two parallel read-only subtasks (DSH route / Vivy route); conclusions were written into the body only after spot checks.

**DSH-route question list and search anchors**: product form and package layout (→ `AGENTS.md` repository layout, `docs/architecture.md`); frontend/backend separation (→ `packages/host/webserver`, `packages/client/web/src/boot.ts`); plugin manifest, discovery, and loading (→ `docs/cordis-primer.md`, `apps/cli/src/plugin.ts`, `packages/boot/app-boot`); **whether UI renders dynamically with installation**—the key question of this research (→ the `dsh.client` declaration and `__DSH_BOOT__` graph in `packages/client/modules/`, `slots.register` in `packages/client/ui-slots`, `command.list` in `packages/interaction/commands`, presenter intents in `packages/client/ui-tool`, and the cookbook's adding-a-settings-card / adding-a-tool); trust model (→ `scripts/client-bundle-purity.spec.ts`, `packages/client/connection/src/api-request-trust.ts`, `packages/extensions/cordis-client-runner`); inventory page (→ `packages/host/plugin-inventory`); theoretical background (→ `paper.txt`, formalization of Cordis reversible effects).

**Vivy-route question list and search anchors**: the full plugin-to-EXE chain (→ `sdk/internal/manifest.go`, `sdk/internal/pack.go`, `internal/generated/plugins/zz_register.go`, `internal/pluginhost/host.go`, `internal/studiocore/service.go`); runtime exposure and RPC surface (→ method switch in `internal/rpc/control.go`, `internal/studio/inspect.go`); UI structure and existing conditional-rendering precedents (→ `ui/src/components/chat/ConversationSidebar.tsx`, `ui/src/lib/store.ts`, the providers/MCP card pattern in `ui/src/components/settings/`, `ui/src/i18n/`); Studio examples (→ `studio/dsh-vivy-studio/index.js`, `studio/dsh-vivy-console/`, `studio/dsh-better-sidebar/`); TUI (→ `cmd/vivy/tui.go`, `internal/tui/`, `docs/architecture/VIVY-FACE-PACK.md`); governance constraints (→ tier A/C and NG-* decisions in `VIVY-PLUGIN-SPEC.md`, `VIVY-STUDIO.md`).

**Cross-validation**: two independent lines of evidence corroborate each other—the `dsh-*` plugins in Studio submodules are live samples of DSH mechanisms A/B (`webServer.tapIndex` injection, `window.__ModuleLoader__` loading, `ctx.slots.inject` registration), consistent with the DSH source description. Five key references written into the body were separately spot-checked manually and all were accurate: `zz_register.go` is generated by pack and returns nil; three hard-coded navigation arrays (`ConversationSidebar.tsx:7-9`); the static `capabilities` table (`control.go:345-360`); manifest has no UI fields (`manifest.go:21-36`); UI stores capabilities without consuming them (`store.ts:33,211`).

**Discipline and boundaries**: read-only throughout; `.workspace/` was not changed; `data/vivy.db`, `data/demo/`, and `data/workspaces/` were not touched (air gap); all DSH conclusions came from local source, without relying on network materials.

### 7.2 Limitations of this research

- DSH was read statically; bundle-delivery behavior was not verified by running it.
- Only plugin source in the Studio submodules was read; their runtime was not observed.
- Option three's sandbox choice (iframe vs. build-time purity gate) was not prototyped and remains an open question.
- The ordering details for mechanism A's composition follow the modules' own documentation and cookbook; the `packages/client/modules` source was not checked line by line.

### 7.3 Follow-up research path (if approved, in order)

1. **Finalize option-one contract**: the manifest v1 `ui` block allowlist vocabulary—use the rendering capabilities of existing `settings/mcp` cards and provider forms as the widget-vocabulary baseline; first produce a field draft + parse/validate test design.
2. **Pack spike**: verify whether the overlay mechanism can generate `zz_ui.go` in parallel with `zz_register.go` (the same `go build -overlay`, without changing the SDK Go API window).
3. **RPC contract impact**: `plugins/list` payload shape vs. dynamic `capabilities`—read `internal/tui/client.go` and worker-client handshake dependencies to confirm the additive path.
4. **i18n strategy**: compatibility plan for inline zh/en in descriptors and the twin-enforcement tests in `ui/src/i18n/index.test.ts`.
5. **Generation awareness**: minimal implementation location for `generation_id` comparison and the "prompt for refresh" interaction (`store.initialize` vs. route guard).
6. **Prerequisite research for option three** (proceed slowly): inventory the current CSP in `ui/index.html` and response headers; experiment with an iframe sandbox prototype vs. DSH-style build-time purity gate; record the relationship with the sealed skin in `VIVY-STUDIO.md` §9.1 in the product contract.

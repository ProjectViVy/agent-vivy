# Vivy UI

- **Development loop:** `just dev` (or `.\dev.ps1`) starts the split pair
  in one shot. Or by hand: `just run` from the repo root (`127.0.0.1:8787`)
  and `pnpm dev` in `ui/` (`127.0.0.1:3015`). Open `http://127.0.0.1:3015`.
  Vite proxies `/rpc` HTTP and WebSocket to the backend. Do not develop
  against the embedded UI on `:8787`, Docker, or a `just build-split`
  static tree.
- The default empty `vivy-config.json` is correct for Vite development because
  the proxy preserves the same-origin browser contract. Edit it only when
  serving a static build against a separately addressed headless backend.
- The embedded UI is packaged by the normal Vivy build and exercised at product
  closeout under root validation policy; it is not the development server.

- `src/lib/rpc.ts` is the sole JSON-RPC WebSocket transport implementation.
- `src/lib/api.ts` defines the backend-authoritative wire types and typed API (including the `settings/providers*` registry RPCs).
- `src/lib/store.ts` is the sole source of truth for Session, Run, Review, Settings, Provider registry, and lifecycle state.
- Real features must not import `src/lib/demo-api.ts`.
- `demo-api.ts` serves only the Notebook, Persona, Cron, Skills, and planning pages marked as “demo / local mock,” and may use only `vivy.demo.*` localStorage keys.

- **The shell is assembled, not hand-wired.** Which sidebar entries and pages
  exist is decided by the selected Recipe (`recipes/default.vivy.yml` when
  unset); the SDK's Assembler writes
  `src/generated/assembly.ts` plus the staged Module sources under
  `src/generated/ui/`. Every `dev`, `build`, `typecheck` and `test` run stages
  that projection first (`ui/scripts/stage-ui-assembly.mjs` → `go run ./sdk
  stage-ui`). A composed validation session stages its chosen projection once
  and runs the applicable direct checks. Never
  hand-edit `src/generated/**`; change the Recipe or the Module. Staged Module
  sources are generated copies and are gitignored — edit
  `plugins/<module>/ui/<module>/src/**` instead, and re-run `pnpm run stage:ui`
  to refresh (the pre-hooks already do).
- A UI Module owns its page, its grouped sidebar entry, and its
  `plugin.<module-id>.*` catalog; the shell owns only the assembly mechanism
  plus the entries it always renders (chat and toolbox; the VIVY group is
  contributed entirely by Modules, including 面具 when vivy/masks-ui is
  selected). Module code reaches the host through `@vivy/ui-sdk`
  (`usePluginTranslation`, `usePluginHost`, `defineNavigationItem`) and the
  host UI kit through the `@/` alias; it never imports the shell's state store
  or another Module's internals.
- Provider secrets are injected by the runtime by default (config `env_key`); the custom provider registry under “Settings → Models” is **persisted by the backend** (`settings/providers` series RPCs → `data/agent-home/
  settings.yaml`; secrets are written to disk write-only with mode 0600 and synced to environment variables afterward), and the UI no longer stores a localStorage copy. When `settings/update` selects a model, it does not carry the secret (the backend resolves it through the registry); values are never written to logs or sent back to the control plane; `vivy.demo.*` still disables secret fields.
- Follow root AGENTS.md for scoped CI and proportional records. Exercise changed
  browser behavior at the isolated split Vite origin. `VIVY_UI_RECIPE` selects an
  explicit development Recipe; unset preserves the current default. A small CI
  stages once then runs direct checks in the same session; ordinary pnpm
  pre-hooks still prepare their inputs. Never hand-edit generated sources.

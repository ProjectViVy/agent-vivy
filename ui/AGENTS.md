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
- The embedded UI is validated by `just ci` and packaged by the normal Vivy
  build; it is the release/smoke path, not the development server.

- `src/lib/rpc.ts` is the sole JSON-RPC WebSocket transport implementation.
- `src/lib/api.ts` defines the backend-authoritative wire types and typed API (including the `settings/providers*` registry RPCs).
- `src/lib/store.ts` is the sole source of truth for Session, Run, Review, Settings, Provider registry, and lifecycle state.
- Real features must not import `src/lib/demo-api.ts`.
- `demo-api.ts` serves only the Notebook, Persona, Cron, Skills, and planning pages marked as “demo / local mock,” and may use only `vivy.demo.*` localStorage keys.
- Provider secrets are injected by the runtime by default (config `env_key`); the custom provider registry under “Settings → Models” is **persisted by the backend** (`settings/providers` series RPCs → `data/agent-home/
  settings.yaml`; secrets are written to disk write-only with mode 0600 and synced to environment variables afterward), and the UI no longer stores a localStorage copy. When `settings/update` selects a model, it does not carry the secret (the backend resolves it through the registry); values are never written to logs or sent back to the control plane; `vivy.demo.*` still disables secret fields.
- After completing UI changes, run `just ci` from the repository root. User-visible behavior must also be exercised end to end at
  `http://127.0.0.1:3015`, and the root `AGENTS.md` requires writing
  `docs/logs/YYYY-MM-DD-slug/` (`summary.md` / `verification.md` /
  `acceptance.md`). Record unfinished gaps in `docs/TODO.md` §0.1.

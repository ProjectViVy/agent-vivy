# Vivy UI

Vivy’s only browser UI, built with React, Vite, TanStack Router, and Zustand. The production build is embedded by Go and runs same-origin with the Vivy control plane by default; it can also be served as a static directory connected to a headless backend.

## Local development

Start the Vivy backend from the repository root first (by default at `127.0.0.1:8787`), then start the UI:

```powershell
just run
cd ui
pnpm install
pnpm dev
```

Open `http://localhost:3015`. Vite proxies `/rpc` HTTP and WebSocket requests to `http://127.0.0.1:8787`.

For a split deployment, edit `vivy-config.json` in the build output:

```json
{ "controlPlaneUrl": "http://127.0.0.1:8787" }
```

The backend’s `server.allowed_origins` must include the static site’s exact loopback origin.

## Data boundaries

- Session, Run, Review, Settings, and lifecycle data come from Vivy JSON-RPC and are not written to localStorage.
- The current session ID is stored in `vivy.ui.activeSession`.
- Notebook, Persona, Cron, Skills, and the planning sidebar do not yet have backend APIs; they are explicitly marked local demos, and their data may use only `vivy.demo.*` keys.
- Provider secrets never pass through the UI; Settings manages only the provider, default model, and base URL.

## Verification

```powershell
pnpm typecheck
pnpm test
pnpm build
```

For repository-level verification, use `just ci` from the root directory.

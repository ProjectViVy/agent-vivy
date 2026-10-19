# T0: reproducible Toolkit browser gate

## What changed

- Added `scripts/toolkits-e2e.mjs` and the `just toolkits-e2e` recipe. The launcher checks ports, builds the embedded UI, packs `recipes/default.vivy.yml` into a fresh lane-local `.workspace/toolkits-e2e/default`, inspects that artifact, prints its generation/recipe/UI digest and binary SHA-256, then runs the lifecycle and accessibility Playwright suites sequentially.
- Ports are validated before launch: `VIVY_TOOLKITS_E2E_BACKEND_PORT` / `VIVY_TOOLKITS_E2E_UI_PORT` default to 18787/3015, must be integers 1–65535, must differ, and must be free. The Vite UI port is passed through `VIVY_UI_PORT` and the backend URL through `VIVY_BACKEND_ADDR`.
- Added `ui/e2e/e2e-ports.mjs` as the single resolver used by both Toolkit and Notebook browser configs. The Notebook suite received `VIVY_NB_E2E_BACKEND_PORT` / `VIVY_NB_E2E_UI_PORT` with its existing 8797/3015 defaults.
- The Toolkit config now uses the platform binary name, passes the UI port, logs the fresh data root, and fails explicitly when the binary is absent. The launcher refuses `VIVY_TOOLKITS_E2E_EXTERNAL_BACKEND=1`; that manual mode is not the reproducible gate. The native gate owns its backend and Vite servers with `reuseExistingServer:false`.
- An explicit `VIVY_TOOLKITS_E2E_BINARY` now requires `VIVY_TOOLKITS_E2E_ARTIFACT_DIR`; the launcher inspects that pack output and refuses a binary whose SHA-256 does not match the artifact binary. The printed identity is the evidence.
- Suite assertions and Toolkit UI were not changed.

Not done: Linux and native Windows venue runs, TK-13/E4 matrix completion, or a claim that all of #81 is closed.

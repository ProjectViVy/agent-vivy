# T0 acceptance

1. From the repository root run `just toolkits-e2e`. The log names the backend/UI ports, artifact root, generation id, recipe digest, UI dist digest, binary path and SHA-256, then shows one lifecycle test and both accessibility cases passing.
2. Run beside another lane with `VIVY_TOOLKITS_E2E_BACKEND_PORT=18889 VIVY_TOOLKITS_E2E_UI_PORT=3117 just toolkits-e2e` (PowerShell: set both environment variables first). No other lane's server is reused; the backend and Vite logs name the isolated ports and a fresh temporary data root.
3. Invalid (`abc`, `0`), equal, or occupied ports fail before Playwright starts and name the variable to change.
4. A binary at a path containing spaces works. An explicit binary override is accepted only with its inspected `VIVY_TOOLKITS_E2E_ARTIFACT_DIR`, and the log binds the binary hash to that generation.
5. The gate cleans only `.workspace/toolkits-e2e/` (or the explicit external artifact) and leaves no backend/Vite process listening after success or failure.

# Verification

- `pnpm install --frozen-lockfile` (`ui/`): passed.
- `pnpm build` (`ui/`): passed.
- `just ci`: passed.
  - Go vet/test passed.
  - UI typecheck passed.
  - UI Vitest: 175 tests passed.
  - UI Vite build passed.
- Split smoke: `just run` and `pnpm dev --host 127.0.0.1` both started successfully.
- `curl.exe http://127.0.0.1:8787/healthz`: HTTP 200.
- `curl.exe http://127.0.0.1:3015/`: HTTP 200.
- Browser runtime: unavailable (no usable browser instance); no screenshot or interaction assertions were performed.

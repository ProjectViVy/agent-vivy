# Verification

- `pnpm install --frozen-lockfile`（`ui/`）：通过。
- `pnpm build`（`ui/`）：通过。
- `just ci`：通过。
  - Go vet/test 通过。
  - UI typecheck 通过。
  - UI Vitest：175 tests passed。
  - UI Vite build 通过。
- Split smoke：`just run` 与 `pnpm dev --host 127.0.0.1` 均成功启动。
- `curl.exe http://127.0.0.1:8787/healthz`：HTTP 200。
- `curl.exe http://127.0.0.1:3015/`：HTTP 200。
- Browser runtime：不可用（无可用浏览器实例），未进行截图/交互断言。

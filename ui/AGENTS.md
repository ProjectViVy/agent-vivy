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

- `src/lib/rpc.ts` 是唯一 JSON-RPC WebSocket 传输实现。
- `src/lib/api.ts` 定义后端权威的 wire types 和 typed API。
- `src/lib/store.ts` 是 Session、Run、Review、Settings 与生命周期状态的唯一来源。
- 真实功能不得导入 `src/lib/demo-api.ts`。
- `demo-api.ts` 只服务带“演示 / 本地模拟”标识的 Notebook、Persona、Cron、Skills 和计划页面，并且只能使用 `vivy.demo.*` localStorage key。
- Provider secrets 不属于 UI Settings。
- UI 修改完成后在仓库根目录运行 `just ci`。用户可见行为还要在
  `http://127.0.0.1:3015` 实走过一遍，并按根目录 `AGENTS.md` 写
  `docs/logs/YYYY-MM-DD-slug/`（`summary.md` / `verification.md` /
  `acceptance.md`）。未修完的缺口记入 `docs/TODO.md` §0.1。

# Vivy UI

Vivy 的唯一浏览器 UI，使用 React、Vite、TanStack Router 和 Zustand。生产构建由 Go 嵌入并与 Vivy control plane 同源运行。

## 本地开发

先在仓库根目录启动 Vivy 后端（默认 `127.0.0.1:8787`），再启动 UI：

```powershell
just run
cd ui
pnpm install
pnpm dev
```

打开 `http://localhost:3015`。Vite 会把 `/rpc` HTTP 与 WebSocket 请求代理到 `http://127.0.0.1:8787`。

## 数据边界

- Session、Run、Review、Settings 与生命周期数据来自 Vivy JSON-RPC，不写入 localStorage。
- 当前会话 ID 使用 `vivy.ui.activeSession` 保存。
- Notebook、Persona、Cron、Skills 和计划侧栏尚无后端 API，是明确标记的本地演示，数据只能使用 `vivy.demo.*` key。
- Provider 密钥永远不经过 UI；Settings 只管理 provider、默认模型与 base URL。

## 验证

```powershell
pnpm typecheck
pnpm test
pnpm build
```

仓库级验证使用根目录的 `just ci`。

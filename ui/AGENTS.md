# Vivy UI

- `src/lib/rpc.ts` 是唯一 JSON-RPC WebSocket 传输实现。
- `src/lib/api.ts` 定义后端权威的 wire types 和 typed API。
- `src/lib/store.ts` 是 Session、Run、Review、Settings 与生命周期状态的唯一来源。
- 真实功能不得导入 `src/lib/demo-api.ts`。
- `demo-api.ts` 只服务带“演示 / 本地模拟”标识的 Notebook、Persona、Cron、Skills 和计划页面，并且只能使用 `vivy.demo.*` localStorage key。
- Provider secrets 不属于 UI Settings。
- UI 修改完成后在仓库根目录运行 `just ci`。

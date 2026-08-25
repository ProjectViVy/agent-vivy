# MCP 面板重做（oil-frontend 规范）

## 变更内容

按 oil-frontend 规范重做 `/mcp` 面板，替换此前残缺的实现。

### 删除

- `ui/src/components/demo/mcp.css`（353 行私有粉色设计系统：渐变背景、点阵、悬浮爱心装饰、独立 Token），不再把兜底值扩张成新设计系统。
- 装饰性元素：三颗悬浮 `Heart`、hero 图标、渐变背景。
- 伪操作与死状态：只重读 localStorage 的“刷新”按钮；永远不可达的 `degraded` 状态；与 `enabled` 完全重复的 `status` 字段；状态码原始文本（`connected` mono 标签）；与侧栏导航重复的面包屑返回按钮。
- 死代码：`getMcpConnectionStatus` 与 `McpConnectionStatusDto`（无任何调用方）。
- 四张彩色统计卡（与筛选器、行内开关重复的信息）。

### 重建

- 数据模型（`ui/src/lib/types.ts`、`ui/src/lib/demo-api.ts`）：
  - `DemoMcpServer` 增加 `command`（stdio 启动命令）与 `url`（http 服务地址），对象具备可识别的连接目标。
  - 新增 `updateDemoMcpServer`、`removeDemoMcpServer`；`addDemoMcpServer` 改为带校验的输入（名称必填；http 必须为绝对 http(s) URL；stdio 必须有启动命令；重名拒绝）。
  - 导入/导出 JSON 往返 `command` / `url`。
- 视图（`ui/src/components/demo/McpDemoView.tsx`）：对齐 `CronTaskManagementView` 的既有模式 —— `Card` 列表 + 行内 `Switch`/编辑/删除、`Dialog` 添加编辑表单（传输方式联动命令/地址字段）、`AlertDialog` 删除确认、`Skeleton` 加载态、空态与筛选空态、忙碌范围只锁当前行。全部使用项目 shadcn 组件与设计 Token，无页面私有 CSS。
- e2e（`ui/e2e/runtime.spec.ts`）：断言更新为新契约（标题“MCP 服务”、开关 checked 状态），不再断言已删除的原始状态码文本。

### 明确不做

- 不新增后端 MCP 管理 RPC：内核的 MCP 仍由 `config.yaml` 的 `runtime.mcp_servers` 驱动；该面板保持演示/本地模拟边界（`vivy.demo.*` localStorage），由 DemoBanner 明示。
- 不改动其他演示页面与共享组件。

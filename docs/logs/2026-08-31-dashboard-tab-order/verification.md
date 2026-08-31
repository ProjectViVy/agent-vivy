# 验证记录

## 命令与结果

- `just ci` — 通过。包含：Go `fmt-check` / `vet` / `test ./...` / `headless-compile`；
  UI `pnpm install --frozen-lockfile` / `typecheck` / `test`（21 个文件 175 个用例全过）/ `build`（vite 生产构建成功）。

## 真实路径冒烟（split Vite）

环境：已在运行的 split 开发对（Vite `http://127.0.0.1:3015` + 控制面 `127.0.0.1:8787`），Vite 热加载了本次改动。

浏览器（ZCode 内置浏览器）访问 `http://127.0.0.1:3015/dashboard`：

- tablist 顺序确认为：`tab "Token" [selected]` → `tab "轨迹"` → `tab "会话"`（DOM rect x 坐标 332/396/448，左到右一致）。
- 默认进入即选中 Token，Token 统计面板正常渲染（总 Token 3.9K、模型分布、使用趋势、会话明细）。
- 点击「轨迹」→ `[selected]`，轨迹面板与工具栏正常渲染。
- 点击「会话」→ `[selected]`，面板显示原概览内容（运行状态：会话 12 / 活跃运行 2 / 待处理 Review 1；近期活动列表）。

## 覆盖说明

- 未跑 `just ui-e2e`（不在 `just ci` 门禁内；本次为选项卡重排，浏览器冒烟已覆盖交互路径）。

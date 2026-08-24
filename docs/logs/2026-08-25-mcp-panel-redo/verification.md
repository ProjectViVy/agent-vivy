# 验证

## 命令与结果

- `pnpm typecheck`（ui/）：通过。
- `pnpm test`（ui/，vitest）：7 个文件 22 个用例全部通过，含新增的
  `updates, removes and validates MCP servers`（编辑、删除、URL/命令/重名校验的失败路径）
  与导入/导出往返 `command`/`url` 的断言。
- `just ci`（fmt-check + vet + go test ./... + headless-compile + ui-ci）：全部通过。

## 真机冒烟（http://127.0.0.1:3015/mcp，split Vite）

浏览器逐步操作并核对：

1. 标题“MCP 服务”、演示横幅、导入/导出/添加按钮、服务列表卡（“2 个服务 · 1 个启用”）。
2. 行内展示连接目标（`npx -y @modelcontextprotocol/server-filesystem .`、`http://127.0.0.1:9123/mcp`）与工具数。
3. 切换 Browser Tools 开关：请求期间行操作禁用（忙碌范围正确），完成后开关 checked、计数变为“2 个启用”。
4. 添加服务弹窗：选 HTTP 后字段联动为“服务地址”；留空提交显示“请输入 HTTP 服务地址”，弹窗保留已输入内容；填入 `http://127.0.0.1:9999/mcp` 提交后列表按名称排序出现新服务。
5. 删除：确认对话框出现，确认后行消失、计数回落。
6. 搜索无结果：出现“没有符合条件的 MCP 服务”与“清除筛选”，清除后恢复。
7. console 无 error/warn。截图：`ui/test-results/mcp-panel-redo.png`（test-results 为 gitignored 临时产物）。

冒烟结束后已将演示数据恢复为初始状态（Browser Tools 停用）。

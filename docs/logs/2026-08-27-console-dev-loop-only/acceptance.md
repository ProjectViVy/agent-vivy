# Acceptance — Vivy Console dev-loop-only

用户视角：Studio 里的「Vivy 控制台」能用，而且只做开发回路。

## 验收步骤

1. 打开 `http://127.0.0.1:3090`，会话视图环里有「Vivy 控制台」标签。
2. 控制台内只有 网关 / VIVY WEB / 日志 三个页签，**没有「生命周期」**。
3. 网关页点「▶ 启动」：网关变为运行中（mock 模式），日志页能尾随
   `vivy starting` 等 stdout/stderr；点「■ 停止」后状态回到已停止。
4. VIVY WEB 页：内嵌 iframe 能显示网关 UI；控制台/RPC 捕获区能收到
   页面 console 与 WebSocket 流量；求值框可执行 `document.title`。
5. 打包/发布不再出现在 Studio UI：`pack/eval/release/install/rollback`
   只通过 `vivy-sdk` / `vivy-studio.exe` 命令行执行。
6. 数据隔离不变：网关数据只在 `data/studio-home/vivy-console/`；
   `data/vivy.db`、`data/demo/`、`data/workspaces/` 未被触碰。

## 失败判据

- 控制台点启动后网关立即退出（日志仍报
  `field allowed_origins not found in type config.Server`）→ 未修复。
- 「生命周期」页签仍出现，或 `/vivy-console/api/lifecycle/*` 仍返回
  200 → 未修复。

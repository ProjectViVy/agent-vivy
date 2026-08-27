# Acceptance — 2026-08-26 studio-launch-fix

用户视角：Vivy Studio 能正常启动并打开。

## 验收步骤

1. 运行 `.\launch-vivy-studio.ps1`（或原有的启动入口），不再报错退出。
2. 启动日志出现 `dsh web: http://127.0.0.1:3090`。
3. 浏览器打开 `http://127.0.0.1:3090`，Studio 主界面正常加载（非 502/白屏）。
4. 右上/右下调试浮标（◈，dsh-vivy-debugger）可用，点开能看到网关状态/日志。
5. 插件市场（dsh-plugin / Plugin Hub）入口可用，`/dsh-plugin-hub/settings`
   返回正常数据。

## 失败判据

启动仍抛 `ERR_MODULE_NOT_FOUND` / `Cannot find package 'dsh-plugin'`，或
`:3090` 无响应，即视为未修复。

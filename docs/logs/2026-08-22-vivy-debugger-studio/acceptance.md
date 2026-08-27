# Acceptance — Vivy Debugger Studio bundle

Date: 2026-08-22

## How a human can tell it worked

1. Open Vivy Studio at `http://127.0.0.1:3090` and refresh.
2. A floating **◈** button appears at the **bottom-right corner of the main
   page** (not in the sidebar).
3. Click it → a "Vivy 网关调试器" panel opens showing 状态 / PID / 监听 /
   启动于 / 运行时长 / EXE / 配置 / 日志.
4. Click **▶ 启动** → the panel shows "已启动 (PID …)…（mock 模式，数据隔离）",
   the status dot turns green, and the log pane streams JSONL lines ending in
   `"msg":"vivy starting"`.
5. Click **■ 停止** → status returns to 已停止; **⟳ 重启** restarts.
6. The gateway's data lives only under
   `data/studio-home/vivy-debugger/` — production `data/vivy.db`,
   `data/demo/`, `data/workspaces/` are untouched.
7. Restart Vivy Studio itself: the debugger is still there (it is now part of
   the Studio profile, not a session plugin).

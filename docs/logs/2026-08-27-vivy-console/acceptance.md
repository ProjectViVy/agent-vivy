# Acceptance — Vivy Console (dsh-vivy-console)

Date: 2026-08-27

## How a human can tell it worked

1. Open Vivy Studio at `http://127.0.0.1:3090`, open any session: the
   conversation view ring shows **Chat / Trajectory / Context / Vivy 控制台**
   (轨迹 / 上下文 / Vivy 控制台), and the floating **◈** button in the
   bottom-right corner is **gone**.
2. Click **Vivy 控制台** → four sub-views: 网关 / VIVY WEB / 日志 / 生命周期.
3. **网关**: click ▶ 启动 → status turns 运行中, PID/监听 appear, and the
   gateway runs in mock mode with data isolated under
   `data/studio-home/vivy-console/`.
4. **VIVY WEB**: the embedded VIVY WEB UI renders in an iframe; as the app
   talks to the gateway, the capture pane streams `[rpc]` rows (send/recv,
   method names, `initialize` and friends) and `[console]` rows; the
   evaluate box returns e.g. `"Agent Diva 前端演示"` for `document.title`;
   reload and 新标签打开 work.
5. **日志**: gateway stdout/stderr tail streams (lines ending in
   `"msg":"vivy starting"`).
6. **生命周期**: the ledger table renders generations/evals/releases/
   installs/events; **发布 (release) is disabled/refused unless the
   human-confirm checkbox is checked**; pack → eval round-trips update the
   ledger; install/rollback targets outside the source tree are accepted,
   inside are refused by the tool.
7. Restart Vivy Studio: the console is still there (part of the profile),
   and the old bottom-right debugger never reappears.

## Acceptance result

⏳ pending — recorded after the Studio restart + browser smoke (next turn).

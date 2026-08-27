# Acceptance — 2026-08-27 console 总控台

How a human can tell the change worked.

1. Open Vivy Studio at `http://127.0.0.1:3090` and refresh the browser
   (no Studio restart needed), then open the **「Vivy 控制台」** tab.
2. The tab has two sections: **总控台 / 日志** — there is no separate
   后端/前端 tab division any more.
3. 总控台 shows:
   - a 总览 line (未运行 → 后端运行中 · 前端未运行 → … → 全部运行中);
   - **▶ 一键启动** — starts backend (auto-compiles the pure-API
     `vivy_headless` build when not overridden) then frontend; the message
     line reports the combined result;
   - **■ 一键停止** and **⟳ 一键重启** for both together;
   - **后端状态** card (状态/PID/监听/EXE/形态/编译 + individual buttons +
     EXE override) and **前端状态** card (状态/PID/入口/命令/代理 + individual
     buttons + 打开 button) side by side.
4. 日志 section still shows one unified timeline of 后端 + 前端 lines with
   source chips, pause, and clear.
5. One-click flow on a clean state: click ▶ 一键启动 → both cards flip to
   运行中; open `http://127.0.0.1:3015` (the dev server is the app);
   click ■ 一键停止 → both cards flip to 已停止 and ports free up.
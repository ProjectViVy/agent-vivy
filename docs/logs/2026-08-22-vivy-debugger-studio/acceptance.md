# Acceptance — Vivy Debugger Studio bundle

Date: 2026-08-22

## How a human can tell it worked

1. Open Vivy Studio at `http://127.0.0.1:3090` and refresh.
2. A floating **◈** button appears at the **bottom-right corner of the main
   page** (not in the sidebar).
3. Click it → a "Vivy Gateway Debugger" panel opens showing Status / PID / Listening /
   Started at / Uptime / EXE / Configuration / Logs.
4. Click **▶ Start** → the panel shows "Started (PID …)… (mock mode, data isolated)",
   the status dot turns green, and the log pane streams JSONL lines ending in
   `"msg":"vivy starting"`.
5. Click **■ Stop** → status returns to Stopped; **⟳ Restart** restarts.
6. The gateway's data lives only under
   `data/studio-home/vivy-debugger/` — production `data/vivy.db`,
   `data/demo/`, `data/workspaces/` are untouched.
7. Restart Vivy Studio itself: the debugger is still there (it is now part of
   the Studio profile, not a session plugin).

# Acceptance — Vivy Console (dsh-vivy-console)

Date: 2026-08-27

## How a human can tell it worked

1. Open Vivy Studio at `http://127.0.0.1:3090`, open any session: the
   conversation view ring shows **Chat / Trajectory / Context / Vivy Console**
   (Trajectory / Context / Vivy Console), and the floating **◈** button in the
   bottom-right corner is **gone**.
2. Click **Vivy Console** → four sub-views: Gateway / VIVY WEB / Logs / Lifecycle.
3. **Gateway**: click ▶ Start → status turns Running, PID/Listening appear, and the
   gateway runs in mock mode with data isolated under
   `data/studio-home/vivy-console/`.
4. **VIVY WEB**: the embedded VIVY WEB UI renders in an iframe; as the app
   talks to the gateway, the capture pane streams `[rpc]` rows (send/recv,
   method names, `initialize` and friends) and `[console]` rows; the
   evaluate box returns e.g. `"Agent Diva frontend demo"` for `document.title`;
   reload and Open in new tab work.
5. **Logs**: gateway stdout/stderr tail streams (lines ending in
   `"msg":"vivy starting"`).
6. **Lifecycle**: the ledger table renders generations/evals/releases/
   installs/events; **the literal Release action is disabled/refused unless the
   human-confirm checkbox is checked**; pack → eval round-trips update the
   ledger; install/rollback targets outside the source tree are accepted,
   inside are refused by the tool.
7. Restart Vivy Studio: the console is still there (part of the profile),
   and the old bottom-right debugger never reappears.

## Acceptance result

⏳ pending — recorded after the Studio restart + browser smoke (next turn).

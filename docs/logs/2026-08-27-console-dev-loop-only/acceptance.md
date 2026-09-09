# Acceptance — Vivy Console dev-loop-only

User perspective: Studio's 「Vivy Console」 works and is limited to the development loop.

## Acceptance steps

1. Open `http://127.0.0.1:3090`; the session-view ring contains a 「Vivy Console」 tab.
2. The console contains only three tabs—Gateway / VIVY WEB / Logs—and **no 「Lifecycle」**.
3. Click 「▶ Start」 on the Gateway page: the gateway becomes running (mock mode), and
   the Logs page tails `vivy starting` and other stdout/stderr; click 「■ Stop」 and the
   status returns to stopped.
4. On the VIVY WEB page, the embedded iframe displays the gateway UI; the console/RPC
   capture area receives page console and WebSocket traffic; the evaluation box can run
   `document.title`.
5. Packaging/release no longer appears in the Studio UI: `pack/eval/release/install/rollback`
   run only through the `vivy-sdk` / `vivy-studio.exe` command line.
6. Data isolation is unchanged: gateway data lives only in
   `data/studio-home/vivy-console/`; `data/vivy.db`, `data/demo/`, and
   `data/workspaces/` are untouched.

## Failure criteria

- The gateway exits immediately after clicking Start in the console (the log still reports
  `field allowed_origins not found in type config.Server`) → not fixed.
- The 「Lifecycle」 tab still appears, or `/vivy-console/api/lifecycle/*` still returns
  `200` → not fixed.

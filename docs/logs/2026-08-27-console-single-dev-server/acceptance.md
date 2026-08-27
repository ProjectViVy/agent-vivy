# Acceptance — 2026-08-27 console single-dev-server

How a human can tell the change worked.

## User-visible behavior

1. Open Vivy Studio at `http://127.0.0.1:3090`, go to a conversation, and
   open the **「Vivy 控制台」** tab (ring beside Trajectory / Context).
2. The tab has exactly three sections: **后端 / 前端 / 日志** — there is
   **no「VIVY WEB」section** and no 内嵌代理/开发直连 target selector
   anywhere.
3. In「后端」: start the backend. The status card says 形态
   「纯 API 后端（vivy_headless 编译，无内嵌前端）」, and the EXE row shows
   `data/studio-home/vivy-console/vivy-backend.exe`. Opening
   `http://127.0.0.1:8787/` in a browser shows a plain **404** — the backend
   serves API only, no UI. `http://127.0.0.1:8787/healthz` answers
   `{"status":"ok",...}`.
4. In「前端」: start the DEV dev server. The 入口 row reads
   `127.0.0.1:3015（Vite DEV 开发服务器，唯一前端入口）` and the 代理 row
   points `/rpc → http://127.0.0.1:8787` (the live backend port). The
   「打开 127.0.0.1:3015」 button opens the app — this is the only UI.
5. In「日志」: one unified timeline shows both 后端 (gateway startup JSON,
   including the `go build -tags vivy_headless` compile lines on start) and
   前端 (Vite) lines, filterable by source.

## No longer present

- No「VIVY WEB」page/tab debugging in the console (no standalone-tab capture,
  no evaluate box, no `/vivy-web/` proxy).
- The backend never serves its own HTML: `vivy.exe` as "嵌入式版本" is not
  what the console starts — it compiles the `vivy_headless` pure-API binary.
- No stale gateway from an old profile copy can linger: starting the backend
  refuses while an external `vivy.exe` is running, and the managed process is
  the auto-compiled headless build.

## Regression checks

- `just ci` green (see verification.md).
- The standard CLI dev loop is untouched: `just dev` / `just run` +
  `cd ui; pnpm dev` still serve `http://127.0.0.1:3015` with `/rpc` proxied
  to `127.0.0.1:8787` (the `VIVY_BACKEND_ADDR` default).
- The tenant Journal (`data/vivy.db`, `data/demo/`, `data/workspaces/`) is
  never read or written; all backend data stays under
  `data/studio-home/vivy-console/`.
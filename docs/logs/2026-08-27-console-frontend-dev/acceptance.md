# Acceptance — Vivy Console frontend dev + unified logs + standalone WEB

User perspective: the console manages the backend and frontend together, logs use one
timeline, and VIVY WEB is a standalone page.

## Acceptance steps

1. Open `http://127.0.0.1:3090`; the 「Vivy Console」 tab contains four sections:
   Gateway / Frontend / VIVY WEB / Logs (no 「Lifecycle」).
2. Click 「▶ Start」 on the 「Gateway」 page: the mock gateway runs and listens on
   127.0.0.1:8787.
3. Click 「▶ Start」 on the 「Frontend」 page: the Vite dev server runs and listens on
   127.0.0.1:3015; clicking 「■ Stop」 again stops it.
4. On the 「Logs」 page, logs from both sources, `[Backend]` and `[Frontend]`, appear in
   one timeline; All/Backend/Frontend chips filter the view; pause/clear work.
5. Click 「Open standalone page」 on the 「VIVY WEB」 page: VIVY WEB opens in a new browser
   tab (proxy mode `/vivy-web/`); page console and RPC traffic return to the console's
   capture area; the evaluation box can execute `document.title`.
6. Data isolation is unchanged: `data/vivy.db`, `data/demo/`, and `data/workspaces/` are
   untouched; gateway data exists only in `data/studio-home/vivy-console/`.

## Failure criteria

- 「Frontend」 fails to start or Vite cannot come up, or the Logs page cannot show the
  frontend source → not fixed.
- VIVY WEB cannot open in a new tab, or standalone-page capture does not return → not fixed.

# Acceptance — 2026-08-27 console 打包与版本 page

How a human can tell the change worked.

1. Open Vivy Studio at `http://127.0.0.1:3090` and refresh the browser.
   The 「Vivy 控制台」 tab now shows **three sections: 总控台 / 打包与版本 /
   日志**.
2. 总控台 is unchanged: the dev loop (pure-API backend + Vite dev server,
   one-click start/stop/restart, backend/frontend status cards).
3. 打包与版本 page:
   - header states it is the distribution lifecycle via `vivy-studio.exe`,
     **separate from 总控台's dev mode — no dev process is started or
     stopped here**;
   - ledger chips (generations / evals / releases / installs / events /
     worktrees) load real rows from the Studio ledger (a JSON table with
     刷新);
   - the action form (pack / eval / release / reject / install / rollback /
     inspect inputs) and buttons run as **one concurrent job** with a
     streamed output viewer;
   - 发布 (release) is refused unless「我确认发布：人工操作」is checked
     (NG-25); install/rollback targets the tool refuses source tree/`data/`.
4. Separation is structural: switching between 总控台 and 打包与版本 never
   changes the other page's state; starting a backend in 总控台 is
   unaffected by the packaging page and vice versa.
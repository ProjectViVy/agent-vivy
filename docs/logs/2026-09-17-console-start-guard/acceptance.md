# Acceptance — console start guard (2026-09-17)

## How a human can tell it worked

After the Studio server restarts (the plugin has no hot reload), hard-refresh
`http://127.0.0.1:3090` and open the **Vivy Console** tab.

1. **The reported bug is gone.** The 一键启动 / 前端启动 action reports
   success only when Vite is really listening on `:3015`: the status rows show
   `前端 运行中（本控制台管理）` and `监听 127.0.0.1:3015`. The app opens at
   `http://127.0.0.1:3015`.

2. **A failure now explains itself.** If the frontend cannot start, the message
   line shows the cause and the last lines the child printed, for example:

   ```
   进程在监听 3015 前已退出（exit 1）；日志尾部：
   ELIFECYCLE  Command failed with exit code 1.
   'vite' is not recognized as an internal or external command,
   operable program or batch file.
   ```

   …instead of the previous `已启动 (PID 1234)` followed by a silently stopped
   pair. The same is true for the backend (a lease conflict, a compile failure,
   or a spawn error is named, not hidden).

3. **一键重启 works.** Stopping the backend and starting it again completes by
   itself; the message adds `（已等待上一次运行的 workspace 租约过期）` once the
   console has waited out the kernel's 30-second organism lease. Expect that
   single restart to take roughly 30–35 seconds.

4. **Repeat-start protection.** Pressing 前端启动 twice answers
   `前端 dev server 已在运行（本控制台管理）` rather than starting a second
   Vite.

## Known limitations to expect

- A restart right after 一键停止 takes ~30s because the kernel lease
  (`internal/storage/sqlite:leaseTTL = 30s`) outlives the hard-stopped backend.
  This is a wait, not a failure.
- If the console did **not** stop the backend itself (for example after a
  Studio restart, or another organism on the same Journal), the start fails
  fast with the real `organism lease held` message instead of waiting.
- The console never deletes or shortens a lease; the kernel stays authoritative.
- The frontend still fails if `ui/node_modules` is broken again (an interrupted
  `pnpm install`). The console now says so; the repair remains
  `pnpm install --frozen-lockfile` in `ui/`.

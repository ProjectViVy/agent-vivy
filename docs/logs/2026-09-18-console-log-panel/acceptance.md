# Acceptance — 日志 console (2026-09-18)

Human view. After this iteration, `studio/dsh-vivy-console`'s 日志 page is the
thing a person opens to find out what the two dev processes just did.

## Walkthrough

1. **Refresh the Studio page.** The restart drops the GUI; a hard refresh
   (Ctrl+Shift+R) of `http://127.0.0.1:3090` brings it back. Open the
   **「Vivy 控制台」** tab and click **日志**.
2. **The status row must not say 「主机未重启：整段刷新」.** If it does, the host
   half is still the old one and the restart did not land.
3. **Look at the panel.** Each line now shows its wall-clock time with
   milliseconds, a level (INFO/WARN/ERROR/DEBUG), a source (后端/前端) with a
   small source marker, and the message; slog attributes follow the message as
   `key=value`. Backend lines are readable text, not raw JSON; the ANSI colour
   codes Vite writes are gone. Errors carry a red level and a red row marker,
   warnings amber, debug lines are dimmed.
4. **Change the size.** 字号 小/中/大 switches 13/14/16px immediately, and the
   choice survives a page reload.
5. **Filter.** 级别 错误 shows only errors and the count on each pill is the
   count in the view; 来源 前端 shows only Vite; typing in the search box
   filters on message, attributes, caller and the raw line. 「显示 N / M 条」
   tells you how much of the buffer you are looking at.
6. **Read a line in full.** Click a line's message: it expands to the raw line
   as written, the caller (`channelhost.Host.StartAll`), `file:line`, the full
   timestamp and the record kind, with 复制本行. Clicking again collapses it.
   「复制」 copies the whole visible view as text.
7. **Follow / pause.** While following, new lines keep the view at the bottom.
   Scroll up and a 「↓ 跳到最新 (n)」 button appears with the number of lines
   that arrived meanwhile. 暂停 stops polling (the view stops moving); ▶ 继续
   brings in exactly what arrived — nothing is lost or duplicated.
8. **清空** empties the view and stays empty while new lines keep arriving.
9. **Stop the frontend** from 总控台 and watch the 日志 page: the tail of its
   run, including any error the process printed, appears with a severity colour
   instead of a plain grey line; the empty state (after 清空) names which side
   is stopped.
10. **Rotate or delete a log file** under
    `data/studio-home/vivy-console/` while the page is open: the status row
    says 「日志已轮转，视图已刷新」 and the view restarts from the new file
    instead of showing fragments.

## What "it worked" means

- The panel reads as a console: aligned columns, mono type at a readable size,
  colour that means severity rather than decoration, and a line you can open,
  copy and search.
- No line is invented, lost or duplicated across polls, pauses and clears; a
  half-written line appears once, then completes in place.
- The 总控台/打包与版本 pages look exactly as before apart from the job-output
  box now sharing the console's mono type.
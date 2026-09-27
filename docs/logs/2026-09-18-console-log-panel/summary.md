# 日志 panel: a structured, incremental log console

Iteration: 2026-09-18. Scope: the **Vivy Studio overlay plugin**
`studio/dsh-vivy-console` (submodule `ProjectViVy/vivy-studio`), host half and
client half. Requested by the product owner:

> agentvivy的vivy studio的vivy开发插件，日志版块优化一下。前端后端都要优化显示，
> 首先先改成一个专业的显示面板，现在太简陋。要能够支持显示彩色，字体也要改，
> 大小目前来说太小了。现在就纯业余日志面板，搞专业点。

## What was wrong

The 日志 page joined both processes into one `<pre>` at `font-size:11px`:

- lines were plain text (`[前端] …`), so severity, timestamps, callers and
  attributes were invisible;
- every 2 s poll re-rendered the whole 300-line tail, so 清空 was undone by the
  next poll, and a paused view could not be resumed without losing or
  duplicating lines;
- the ANSI escapes that Vite and pnpm write were shown raw;
- there was no filtering beyond source, no search, no row detail, no copy.

## What changed

### Host half — `GET /vivy-console/api/logs` is now an incremental structured feed

- New pure module **`logs.js`** owns every parsing rule: ANSI stripping, slog
  JSON extraction, level normalisation and inference, caller/file shortening,
  continuation chaining, byte-accurate line splitting, timeline ordering and
  cursor parsing/formatting. It does no I/O, so it is unit tested
  (`logs.test.mjs`, 14 cases).
- A record is
  `{id, src, stream, kind, level, ts, msg, caller, file, line, fields, text, partial}`.
  `id` is `<b|f>:<out|err>:<line byte offset>`, so a line still being written
  (`partial:true`, cursor held at its start) comes back complete under the same
  id and the client upserts it instead of duplicating it.
- `?cursor=<4 byte offsets>` (backend.out / backend.err / frontend.out /
  frontend.err) makes a poll return only what was appended. `mode` is
  `tail|delta`, `reset` reports a rotated/truncated/rewritten file, `omitted`
  reports a window that skips earlier content (tail clipping, per-stream tail
  line cap, backlog above the delta byte cap). A malformed cursor is ignored
  rather than guessed at.
- Levels are inferred once, on the host: the slog `level` attribute; otherwise
  error wording, an ANSI red/amber SGR hint, warning wording, and finally the
  stream default (stderr is at least a warning). A line that looks like a
  continuation (indented, Go stack frame, tool error block) after a structured
  line inherits that line's level as `kind:"cont"`; any other plain line is its
  own record with an inferred level.
- Ordering merges all four streams by timestamp; a record without one inherits
  the last timestamp of its own source, and a source with no timestamps at all
  (today: Vite) sorts after the timestamped records in read order.
- The response stays **additive**: `lines[].src` and `lines[].text` are kept, so
  a client half that predates the cursor still renders the feed.
- `index.js` reads each stream as a byte window (`readRange`, `readLogWindow`)
  and only snaps to a line boundary where a window can actually start mid-line
  (a tail read, or a delta that had to jump past the delta byte cap) — a cursor
  is a line start by contract, which is what keeps `partial` records working.
  The four log paths, the lifecycle surface and the air gap are unchanged.

### Client half — a real console

- `LogsPane` renders a mono code surface (`--ds-font-family-code`, code-block
  token surface) at **13/14/16px** (小/中/大, default 中) with a fixed
  `时间 · 级别 · 来源 · 消息` grid, `tabular-nums` timestamps, severity colour on
  the level plus a row marker for error/warn, neutral message text, and dimmed
  debug/continuation lines.
- Toolbar: level filters (全部/错误/警告/信息/调试) with counts, source filters,
  a search box over message/field/caller/raw text, 暂停/继续 (cursor-backed, so
  resuming loses nothing), 清空 (clears the view and stays cleared), 换行/不换行,
  复制 (the visible view as text), and 跳到最新 with a pending count when the
  view is not following.
- Rows expand on click to the raw line, caller, `file:line`, full timestamp,
  record kind and a per-row 复制. The view buffer is capped at 1000 records and
  says how many older records left the view.
- States are explicit and mutually exclusive: 读取中…, （暂无日志）+ a hint that
  names whichever side is stopped, no-matches-for-this-filter, and a read
  failure (inline in the status row, keeping the records already on screen, with
  a body-level error plus 重试 when nothing has been read yet).
- View shape (字号, 换行) persists in `localStorage` under
  `dsh-vivy-console.logs.view`; filters and search deliberately do not.
- Toolbar controls use the DSH **ui-primitives** atoms (`Button` sm/ghost,
  `Pill`, `Input`) resolved from the browser module table, with a local
  `.vc-btn`/`.vc-chip`/`.vc-input` fallback if a shell does not seed them.
- `.vc-pre` (the 打包与版本 job-output viewer) moves onto the same code font at
  13px/20px so the two console surfaces agree.
- A client half talking to a host half that has not been restarted says
  「主机未重启：整段刷新」 and disables 清空, because whole-tail refresh would
  undo it.

## Explicitly not done

- No server-side filtering, paging, download or export route; no virtualization
  (the 1000-record view cap and the 300-line-per-stream host tail are the
  bounds).
- 总控台 and 打包与版本 are unchanged apart from `.vc-pre` typography. Their
  local `.vc-btn`/`.vc-chip` controls stay as they are; migrating them to the
  same ui-primitives atoms is recorded as a follow-up in `docs/TODO.md` §0.1
  rather than done silently here.
- No kernel, `internal/logging`, `docs/architecture/LOGGING.md` or Eino surface
  is touched: this iteration reads the four log files the console already
  writes, and the log contract itself is unchanged. (No agent loop, model,
  tool, prompt, stream, context, RAG or MCP capability is involved, so no Eino
  capability check applies.)
- No new dependency, no build step, no locale seam: the plugin copy stays
  Chinese and hardcoded, as before.
- The submodule gitlink is not staged in the host repository (the submodule
  lands on its own branch), following the 2026-09-17 console lane.
- Air gap (ST-2) respected: `data/studio-home/` only; `data/vivy.db`,
  `data/demo/` and `data/workspaces/` were never read or written.

## Deliverables

- `studio/dsh-vivy-console/logs.js` (new), `logs.test.mjs` (new)
- `studio/dsh-vivy-console/index.js`, `client.js`, `package.json`, `README.md`
- Submodule commit `4483d84` on `feat/hub-delete-source-autocommit`
- This record
# Acceptance — one context-usage control in the composer

Human steps (split pair: `just run` + `cd ui; pnpm dev`, open
http://127.0.0.1:3015, open a session):

1. The composer's bottom-left shows a single control: usage ring + percentage +
   a small compact icon. Hovering it shows the full usage tooltip (percent,
   tokens, bytes, compaction status).
2. Click it: the "压缩上下文" window opens anchored above the meter, with the
   optional instructions field and `立即压缩`. `Esc` or a click outside closes
   it and nothing is left behind.
3. Compaction still works end to end: with an executable provider, type an
   instruction and click `立即压缩`; the window closes and the composer notice
   shows `已压缩 X → Y tokens` (or `暂无可压缩内容`). While a run is in flight
   the window explains that compaction waits for the run, and a server busy
   rejection still appears in the same notice area.
4. There is no longer a separate compress button next to the meter, and no
   duplicate compaction entry anywhere in the composer.
5. Settings → 通用 → turn auto compaction off: the meter becomes a plain usage
   display again — it does not pretend to be clickable.
6. Keyboard / assistive tech: Tab reaches the control, its accessible name is
   "上下文占用 <n>%", and Enter/Space opens the same window.

How to tell it failed: the meter still shows a separate `Shrink` button, or the
meter is clickable with nothing happening, or the old `progressbar` role and
the new button both exist at once.

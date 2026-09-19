# Acceptance — chat transcript rows (Phase 1)

Open the dev UI at `http://127.0.0.1:3015` and open a session that has used
tools (the console's own `你好` session works).

1. **Every tool call is one compact row.** Instead of a wide `工具结果` card per
   call you see a 24 px line: a small family icon, a title (`读取` / `写入` /
   `修改` / `搜索` / `命令` / `网络` / `任务` / `提问` / `工具调用`), a dot, and a
   single-line summary taken from the arguments (a path, a command description, a
   search pattern…). Unfinished calls are not shown as failures; failed ones turn
   red and show the error's first line. File mutations show `+N -M`.
2. **Clicking a row expands its body.** One card only: a unified diff for a file
   mutation, otherwise `输入`/`输出` sections; long output is folded to 8 lines
   with `展开剩余 N 行`. Shell output shows `退出码 N` when the command reported
   one.
3. **Thinking is a real row and it stays.** A collapsed `思考过程` row precedes
   the work of a turn: while the model is streaming, the summary follows the
   newest line; after the run ends the row is still there and still expandable,
   showing the full reasoning text.
4. **The answer is still the answer.** Assistant replies remain bubbles with the
   usual copy / regenerate / rewind / fork actions, and a run whose events are not
   loaded renders exactly as before (no regression on old history).
5. **Nothing leaked.** No duplicate rendering: a run is rendered either from its
   events or from its projected messages, never both.

Not covered yet (see `docs/TODO.md` §0.1 `TOOL-UI-ROWS`): folding a completed
turn behind one `已思考 · N 次工具调用` row, per-turn usage/timing in a turn tail,
opening a file or jumping to the trajectory panel from a tool row, reasoning for
runs outside the last three, code highlighting in chat, and CJK bold after
punctuation.

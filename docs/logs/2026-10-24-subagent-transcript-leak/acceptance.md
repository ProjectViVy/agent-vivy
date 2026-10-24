# One-shot sub-agent transcript leak — acceptance

A human can tell this worked without reading code:

1. **Open the reported conversation.** In the split pair at `http://127.0.0.1:3015`, open the session
   titled 测试一下你的子agent (`sess_fed3d415787a9794`). The last message is the parent's summary
   (子 agent 三项测试全部完成，结论如下。). Nothing appears after it; there is no extra "round" and no
   unattached binary-tree answer.

2. **The child answer still exists, in the right place.** In the parent transcript, the third
   `agent` tool-call row expands to the same binary-tree analysis. It is a tool result of the parent
   run, not a conversation message.

3. **Run Inspector → Children still lists every sub-agent.** Settings → Vivy 功能 → Run Inspector,
   select `run_d5abbb64ecc2bba4`, open **子 Run 3**. All three children are listed as `completed`, and
   selecting one shows its final result text. A failed child would show its error here.

4. **The model no longer sees child transcripts.** On the next turn in that session, the context
   contains only the parent's own messages; `messages` has 20 rows for the session and none carry a
   `child_*` run id (verify with
   `sqlite3 "file:.workspace/dev/home/vivy.db?mode=ro" "select count(*) from messages where session_id='sess_fed3d415787a9794' and run_id like 'child_%';"`).

5. **Fresh and upgraded databases both behave.** A new database starts at migration 48; an existing
   dev database is repaired at startup — the pre-existing leaked rows are deleted, and the known
   message count drops accordingly.

6. **Continuable children are unaffected.** A continuable child keeps its own child session and its
   history still appears when that session is opened.

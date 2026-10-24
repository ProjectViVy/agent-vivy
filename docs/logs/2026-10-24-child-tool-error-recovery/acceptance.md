# Acceptance — child tool-error recovery

A human can reproduce the fixed behavior against a build of this branch.

1. Start the split pair from the branch checkout (fresh user home, provider
   configured in Settings → Model), open `http://127.0.0.1:3015`.
2. Write a session workspace that contains a skill with a `references/`
   directory (or use this repository as the session workspace, which carries
   `.agents/skills/vivy-plugin`).
3. Ask the parent to delegate a bad supporting-file read, for example:

   ```text
   用子 agent 读取 vivy-plugin 技能的 references/does-not-exist.md 并报告结果
   ```

   Expected: the parent receives a normal tool result from the child — the
   text `skills: supporting file … is unavailable: file does not exist` — and
   answers with the child's report. It must **not** answer
   `sub-agent failed: The child task could not be completed. Please retry.`

4. Repeat with an out-of-allowlist path (`docs/x.md`, `../SKILL.md`) and with
   `http_request` on a relative URL. Each child answers with
   `tool argument "path"/"url": …` and finishes; the parent turn completes.
5. Ask a child to inspect the current goal
   (`用子 agent 查看当前会话的 goal`). The child's tool list has no `get_goal`;
   it cannot call it and the run completes. The parent itself can still call
   `get_goal`.
6. Negative control: a normal read-only delegation (for example "read
   `vivy-plugin/SKILL.md` and summarize it") still completes.

In the journal (`runs`, `run_events`), each child run above ends `completed`,
and the offending `tool.finished` event carries `outcome: "recoverable"` with
`reason: "not_found"` or `"invalid_arguments"` instead of failing the run.

Still fatal by design: an unknown host, a sandbox network denial, a disabled
skill, an unreadable file, a malformed skill and backend/storage faults.

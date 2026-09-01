# Acceptance — TT-2 resume mounts

## How a human can tell it worked

1. Start a chat and ask the agent something that routes through a skill
   which declares tools in its SKILL.md frontmatter (e.g. a "writer" skill
   declaring `echo_info`-class tools).
2. The agent calls `skill_view` on that skill mid-run — its declared tools
   become callable for the rest of the run (existing TT-1 behavior).
3. The run hits an interruption that suspends it — a `write_note`-class
   approval request or an `ask_user` question appears in the Review Center.
4. Decide it (approve, or answer the question).
5. After the resume, the agent can still call the skill-mounted tool. Before
   this fix the call was rejected with
   `runtime: tool "<name>" is not selected for this request`, the turn
   errored, and the run could not continue the scripted flow. Now the
   tool executes and the run completes normally.

## What does NOT change

- Mounts are still run-scoped and memory-only: a process restart mid-suspension
  recovers the run with an empty mount registry (the model must re-view the
  skill to re-mount). That boundary is TT-3 (journal visibility) / TT-1.
- Tools mounted by a skill are still not advertised to a *different* run or
  session; nothing about the selected-tool surface contract changed.
